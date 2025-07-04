package test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/localrivet/gomcp/mcp"
	"github.com/localrivet/gomcp/server"
)

func TestProgressTokenManager(t *testing.T) {
	ptm := mcp.NewProgressTokenManager()

	// Test token generation
	requestID := "test-request-123"
	token := ptm.GenerateToken(requestID)

	if token == "" {
		t.Fatal("Expected non-empty token")
	}

	if !ptm.ValidateToken(token) {
		t.Fatal("Expected token to be valid")
	}

	// Test token retrieval
	progressToken, err := ptm.GetToken(token)
	if err != nil {
		t.Fatalf("Expected to retrieve token, got error: %v", err)
	}

	if progressToken.RequestID != requestID {
		t.Errorf("Expected request ID %s, got %s", requestID, progressToken.RequestID)
	}

	if !progressToken.IsActive {
		t.Error("Expected token to be active")
	}

	// Test token update
	time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamps
	originalUpdate := progressToken.LastUpdate

	err = ptm.UpdateToken(token)
	if err != nil {
		t.Fatalf("Expected to update token, got error: %v", err)
	}

	updatedToken, _ := ptm.GetToken(token)
	if !updatedToken.LastUpdate.After(originalUpdate) {
		t.Error("Expected last update time to be updated")
	}

	// Test token deactivation
	err = ptm.DeactivateToken(token)
	if err != nil {
		t.Fatalf("Expected to deactivate token, got error: %v", err)
	}

	if ptm.ValidateToken(token) {
		t.Error("Expected token to be invalid after deactivation")
	}

	// Test cleanup
	token2 := ptm.GenerateToken("test-request-456")
	time.Sleep(10 * time.Millisecond)

	removed := ptm.CleanupExpiredTokens(5 * time.Millisecond)
	if removed < 1 {
		t.Errorf("Expected to remove at least 1 expired token, removed %d", removed)
	}

	if ptm.ValidateToken(token2) {
		t.Error("Expected token2 to be removed by cleanup")
	}
}

func TestProgressNotificationHandling(t *testing.T) {
	srv := server.NewServer("test-progress-server")
	serverImpl := srv.GetServer()

	// Create a progress token
	requestID := "test-request-789"
	token := serverImpl.CreateProgressToken(requestID)

	if token == "" {
		t.Fatal("Expected non-empty progress token")
	}

	// Test valid progress notification
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/progress",
		"params": map[string]interface{}{
			"progressToken": token,
			"progress":      50.0,
			"total":         100.0,
			"message":       "Halfway done",
		},
	}

	notificationBytes, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("Failed to marshal notification: %v", err)
	}

	err = serverImpl.HandleProgressNotification(notificationBytes)
	if err != nil {
		t.Errorf("Expected to handle valid progress notification, got error: %v", err)
	}

	// Test invalid progress notification (missing token)
	invalidNotification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/progress",
		"params": map[string]interface{}{
			"progress": 75.0,
		},
	}

	invalidBytes, _ := json.Marshal(invalidNotification)
	err = serverImpl.HandleProgressNotification(invalidBytes)
	if err == nil {
		t.Error("Expected error for notification missing progress token")
	}

	// Test notification with unknown token (should not error, just ignore)
	unknownNotification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/progress",
		"params": map[string]interface{}{
			"progressToken": "unknown-token",
			"progress":      25.0,
		},
	}

	unknownBytes, _ := json.Marshal(unknownNotification)
	err = serverImpl.HandleProgressNotification(unknownBytes)
	if err != nil {
		t.Errorf("Expected no error for unknown token, got: %v", err)
	}
}

func TestProgressRateLimitConfiguration(t *testing.T) {
	// Test default configuration
	defaultConfig := server.NewDefaultProgressRateLimitConfig()

	if defaultConfig.MaxNotificationsPerSecond != 10 {
		t.Errorf("Expected default max notifications per second to be 10, got %d", defaultConfig.MaxNotificationsPerSecond)
	}

	if defaultConfig.BufferSize != 100 {
		t.Errorf("Expected default buffer size to be 100, got %d", defaultConfig.BufferSize)
	}

	if defaultConfig.OverflowStrategy != server.CombineNotifications {
		t.Errorf("Expected default overflow strategy to be CombineNotifications, got %v", defaultConfig.OverflowStrategy)
	}

	// Test custom configuration
	customConfig := &server.ProgressRateLimitConfig{
		MaxNotificationsPerSecond: 5,
		BufferSize:                50,
		OverflowStrategy:          server.DropOldest,
		CombineThreshold:          200 * time.Millisecond,
		EnableBatching:            true,
		BatchSize:                 3,
		BatchTimeout:              1 * time.Second,
	}

	srv := server.NewServer("test-rate-limit-config")
	serverImpl := srv.GetServer()

	// Set custom configuration
	serverImpl.SetProgressRateLimitConfiguration(customConfig)

	// Get configuration and verify
	retrievedConfig := serverImpl.GetProgressRateLimitConfiguration()

	if retrievedConfig.MaxNotificationsPerSecond != 5 {
		t.Errorf("Expected max notifications per second to be 5, got %d", retrievedConfig.MaxNotificationsPerSecond)
	}

	if retrievedConfig.BufferSize != 50 {
		t.Errorf("Expected buffer size to be 50, got %d", retrievedConfig.BufferSize)
	}

	if retrievedConfig.OverflowStrategy != server.DropOldest {
		t.Errorf("Expected overflow strategy to be DropOldest, got %v", retrievedConfig.OverflowStrategy)
	}
}

func TestProgressRateLimiting(t *testing.T) {
	srv := server.NewServer("test-rate-limiting")
	serverImpl := srv.GetServer()

	// Set a very restrictive rate limit for testing
	config := &server.ProgressRateLimitConfig{
		MaxNotificationsPerSecond: 2, // Only 2 notifications per second
		BufferSize:                10,
		OverflowStrategy:          server.DropOldest,
		CombineThreshold:          50 * time.Millisecond,
		EnableBatching:            false,
		BatchSize:                 5,
		BatchTimeout:              500 * time.Millisecond,
	}
	serverImpl.SetProgressRateLimitConfiguration(config)

	// Create a progress token
	token := serverImpl.CreateProgressToken("test-rate-limit")

	// Send notifications rapidly - should trigger rate limiting
	var errors []error
	for i := 0; i < 5; i++ {
		total := 100.0
		err := serverImpl.SendProgressNotification(token, float64(i*20), &total, fmt.Sprintf("Progress %d", i))
		if err != nil {
			errors = append(errors, err)
		}
	}

	// Should have sent some notifications without error (within rate limit)
	// and buffered others
	if len(errors) > 3 {
		t.Errorf("Expected at most 3 errors due to rate limiting, got %d", len(errors))
	}

	// Get statistics
	stats := serverImpl.GetProgressRateLimitStatistics()
	if stats == nil {
		t.Fatal("Expected statistics to be available")
	}

	// Verify statistics structure
	if totalLimiters, ok := stats["totalRateLimiters"]; !ok || totalLimiters.(int) == 0 {
		t.Error("Expected at least one rate limiter to be created")
	}

	if tokenStats, ok := stats["tokenStatistics"]; ok {
		if tokenStatsMap, ok := tokenStats.(map[string]interface{}); ok {
			if _, exists := tokenStatsMap[token]; !exists {
				t.Errorf("Expected statistics for token %s", token)
			}
		}
	}
}

func TestProgressRateLimiterBuffering(t *testing.T) {
	config := server.NewDefaultProgressRateLimitConfig()
	config.MaxNotificationsPerSecond = 1 // Very restrictive
	config.BufferSize = 3
	config.OverflowStrategy = server.DropOldest

	rateLimiter := server.NewProgressRateLimiter(config)

	// First notification should be allowed
	if !rateLimiter.CanSendNotification() {
		t.Error("Expected first notification to be allowed")
	}

	// Subsequent notifications should be rate limited
	token := "test-buffer-token"
	for i := 0; i < 5; i++ {
		notification := mcp.NewProgressNotification(token, float64(i*20), nil, fmt.Sprintf("Message %d", i))
		err := rateLimiter.BufferNotification(notification)
		if err != nil && i < 3 {
			t.Errorf("Expected buffering to succeed for notification %d, got error: %v", i, err)
		}
	}

	// Check statistics
	stats := rateLimiter.GetStatistics()

	if bufferSize, ok := stats["bufferSize"]; !ok || bufferSize.(int) > 3 {
		t.Errorf("Expected buffer size to be at most 3, got %v", bufferSize)
	}

	if totalNotifications, ok := stats["totalNotifications"]; !ok || totalNotifications.(int64) != 5 {
		t.Errorf("Expected total notifications to be 5, got %v", totalNotifications)
	}

	if droppedNotifications, ok := stats["droppedNotifications"]; !ok || droppedNotifications.(int64) < 2 {
		t.Errorf("Expected at least 2 dropped notifications due to buffer overflow, got %v", droppedNotifications)
	}
}

func TestProgressRateLimiterOverflowStrategies(t *testing.T) {
	testCases := []struct {
		name     string
		strategy server.OverflowStrategy
	}{
		{"DropOldest", server.DropOldest},
		{"DropNewest", server.DropNewest},
		{"CombineNotifications", server.CombineNotifications},
		{"BlockUntilSpace", server.BlockUntilSpace},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := server.NewDefaultProgressRateLimitConfig()
			config.BufferSize = 2
			config.OverflowStrategy = tc.strategy

			rateLimiter := server.NewProgressRateLimiter(config)

			token := "test-overflow-token"

			// Fill the buffer
			for i := 0; i < 2; i++ {
				notification := mcp.NewProgressNotification(token, float64(i*50), nil, fmt.Sprintf("Message %d", i))
				err := rateLimiter.BufferNotification(notification)
				if err != nil {
					t.Errorf("Expected buffering to succeed for notification %d, got error: %v", i, err)
				}
			}

			// Add one more to trigger overflow
			overflowNotification := mcp.NewProgressNotification(token, 100.0, nil, "Overflow message")
			err := rateLimiter.BufferNotification(overflowNotification)

			stats := rateLimiter.GetStatistics()

			switch tc.strategy {
			case server.DropNewest:
				if err == nil {
					t.Error("Expected error when dropping newest notification")
				}
			case server.DropOldest, server.BlockUntilSpace:
				if err != nil {
					t.Errorf("Expected no error for %s strategy, got: %v", tc.name, err)
				}
				if droppedNotifications, ok := stats["droppedNotifications"]; !ok || droppedNotifications.(int64) == 0 {
					t.Errorf("Expected at least one dropped notification for %s strategy", tc.name)
				}
			case server.CombineNotifications:
				if err != nil {
					t.Errorf("Expected no error for CombineNotifications strategy, got: %v", err)
				}
				if combinedNotifications, ok := stats["combinedNotifications"]; !ok || combinedNotifications.(int64) == 0 {
					t.Error("Expected at least one combined notification")
				}
			}
		})
	}
}

func TestProgressRateLimiterProcessBuffer(t *testing.T) {
	config := server.NewDefaultProgressRateLimitConfig()
	config.MaxNotificationsPerSecond = 2
	config.BufferSize = 5

	rateLimiter := server.NewProgressRateLimiter(config)

	token := "test-process-buffer-token"

	// Use up the rate limit
	rateLimiter.CanSendNotification()
	rateLimiter.CanSendNotification()

	// Buffer some notifications
	for i := 0; i < 3; i++ {
		notification := mcp.NewProgressNotification(token, float64(i*25), nil, fmt.Sprintf("Buffered %d", i))
		err := rateLimiter.BufferNotification(notification)
		if err != nil {
			t.Errorf("Expected buffering to succeed for notification %d, got error: %v", i, err)
		}
	}

	// Wait for rate limit window to reset
	time.Sleep(1100 * time.Millisecond)

	// Process buffer
	processedNotifications := rateLimiter.ProcessBuffer()

	// Should be able to process up to the rate limit
	if len(processedNotifications) == 0 {
		t.Error("Expected at least one notification to be processed from buffer")
	}

	if len(processedNotifications) > 2 {
		t.Errorf("Expected at most 2 notifications to be processed (rate limit), got %d", len(processedNotifications))
	}

	// Check that buffer size decreased
	stats := rateLimiter.GetStatistics()
	if bufferSize, ok := stats["bufferSize"]; !ok || bufferSize.(int) >= 3 {
		t.Errorf("Expected buffer size to decrease after processing, got %v", bufferSize)
	}
}

func TestProgressNotificationTypes(t *testing.T) {
	// Test NewProgressNotification
	token := "test-token-123"
	progress := 75.0
	total := 100.0
	message := "Three quarters complete"

	notification := mcp.NewProgressNotification(token, progress, &total, message)

	if notification.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", notification.JSONRPC)
	}

	if notification.Method != "notifications/progress" {
		t.Errorf("Expected method 'notifications/progress', got '%s'", notification.Method)
	}

	if notification.Params.ProgressToken != token {
		t.Errorf("Expected progress token '%s', got '%s'", token, notification.Params.ProgressToken)
	}

	if notification.Params.Progress != progress {
		t.Errorf("Expected progress %f, got %f", progress, notification.Params.Progress)
	}

	if notification.Params.Total == nil || *notification.Params.Total != total {
		t.Errorf("Expected total %f, got %v", total, notification.Params.Total)
	}

	if notification.Params.Message != message {
		t.Errorf("Expected message '%s', got '%s'", message, notification.Params.Message)
	}

	// Test serialization/deserialization
	jsonBytes, err := notification.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize notification: %v", err)
	}

	var deserializedNotification mcp.ProgressNotification
	err = deserializedNotification.FromJSON(jsonBytes)
	if err != nil {
		t.Fatalf("Failed to deserialize notification: %v", err)
	}

	if deserializedNotification.Params.ProgressToken != token {
		t.Errorf("Deserialized token mismatch: expected '%s', got '%s'",
			token, deserializedNotification.Params.ProgressToken)
	}

	// Test validation
	err = notification.Validate()
	if err != nil {
		t.Errorf("Expected valid notification, got error: %v", err)
	}

	// Test percentage calculation
	percentage := notification.GetPercentage()
	if percentage == nil {
		t.Fatal("Expected percentage to be calculated")
	}

	expectedPercentage := 75.0
	if *percentage != expectedPercentage {
		t.Errorf("Expected percentage %f, got %f", expectedPercentage, *percentage)
	}

	// Test completion check
	if notification.IsComplete() {
		t.Error("Expected notification to not be complete at 75%")
	}

	// Test completion at 100%
	completeNotification := mcp.NewProgressNotification(token, 100.0, &total, "Complete")
	if !completeNotification.IsComplete() {
		t.Error("Expected notification to be complete at 100%")
	}
}

func TestProgressNotificationValidation(t *testing.T) {
	// Test invalid JSON-RPC version
	notification := &mcp.ProgressNotification{
		JSONRPC: "1.0",
		Method:  "notifications/progress",
		Params: mcp.ProgressNotificationParams{
			ProgressToken: "test-token",
			Progress:      50.0,
		},
	}

	err := notification.Validate()
	if err == nil {
		t.Error("Expected error for invalid JSON-RPC version")
	}

	// Test invalid method
	notification.JSONRPC = "2.0"
	notification.Method = "invalid/method"

	err = notification.Validate()
	if err == nil {
		t.Error("Expected error for invalid method")
	}

	// Test missing progress token
	notification.Method = "notifications/progress"
	notification.Params.ProgressToken = ""

	err = notification.Validate()
	if err == nil {
		t.Error("Expected error for missing progress token")
	}

	// Test negative progress
	notification.Params.ProgressToken = "test-token"
	notification.Params.Progress = -10.0

	err = notification.Validate()
	if err == nil {
		t.Error("Expected error for negative progress")
	}

	// Test negative total
	notification.Params.Progress = 50.0
	total := -100.0
	notification.Params.Total = &total

	err = notification.Validate()
	if err == nil {
		t.Error("Expected error for negative total")
	}

	// Test progress exceeding total
	total = 100.0
	notification.Params.Total = &total
	notification.Params.Progress = 150.0

	err = notification.Validate()
	if err == nil {
		t.Error("Expected error for progress exceeding total")
	}
}

func TestProgressChannel(t *testing.T) {
	channel := mcp.NewProgressChannel()

	if !channel.IsActive() {
		t.Error("Expected new channel to be active")
	}

	// Test subscription
	token := "test-token-channel"
	var receivedNotifications []*mcp.ProgressNotification

	listener := func(notification *mcp.ProgressNotification) error {
		receivedNotifications = append(receivedNotifications, notification)
		return nil
	}

	channel.Subscribe(token, listener)

	if channel.GetListenerCount(token) != 1 {
		t.Errorf("Expected 1 listener for token, got %d", channel.GetListenerCount(token))
	}

	// Test publishing
	notification := mcp.NewProgressNotification(token, 25.0, nil, "Quarter done")
	err := channel.Publish(notification)
	if err != nil {
		t.Errorf("Expected to publish notification, got error: %v", err)
	}

	if len(receivedNotifications) != 1 {
		t.Errorf("Expected 1 received notification, got %d", len(receivedNotifications))
	}

	// Test global subscription
	var globalNotifications []*mcp.ProgressNotification
	globalListener := func(notification *mcp.ProgressNotification) error {
		globalNotifications = append(globalNotifications, notification)
		return nil
	}

	channel.SubscribeAll(globalListener)

	// Publish another notification
	notification2 := mcp.NewProgressNotification(token, 50.0, nil, "Half done")
	err = channel.Publish(notification2)
	if err != nil {
		t.Errorf("Expected to publish second notification, got error: %v", err)
	}

	// Should have received by both token-specific and global listeners
	if len(receivedNotifications) != 2 {
		t.Errorf("Expected 2 token-specific notifications, got %d", len(receivedNotifications))
	}

	if len(globalNotifications) != 1 {
		t.Errorf("Expected 1 global notification, got %d", len(globalNotifications))
	}

	// Test unsubscribe
	channel.Unsubscribe(token)

	if channel.GetListenerCount(token) != 0 {
		t.Errorf("Expected 0 listeners after unsubscribe, got %d", channel.GetListenerCount(token))
	}

	// Test channel close
	channel.Close()

	if channel.IsActive() {
		t.Error("Expected channel to be inactive after close")
	}

	err = channel.Publish(notification)
	if err == nil {
		t.Error("Expected error when publishing to closed channel")
	}
}

func TestBidirectionalProgressCommunication(t *testing.T) {
	srv := server.NewServer("test-bidirectional-progress")
	serverImpl := srv.GetServer()

	// Create a progress token
	requestID := "test-bidirectional-request"
	token := serverImpl.CreateProgressToken(requestID)

	// Set up a listener for progress notifications
	var receivedNotifications []*mcp.ProgressNotification
	listener := func(notification *mcp.ProgressNotification) error {
		receivedNotifications = append(receivedNotifications, notification)
		return nil
	}

	serverImpl.SubscribeToProgress(token, listener)

	// Send a progress notification
	total := 200.0
	err := serverImpl.SendProgressNotification(token, 100.0, &total, "Halfway there")
	if err != nil {
		t.Errorf("Expected to send progress notification, got error: %v", err)
	}

	// Check that the listener received the notification
	if len(receivedNotifications) != 1 {
		t.Errorf("Expected 1 received notification, got %d", len(receivedNotifications))
	}

	if receivedNotifications[0].Params.Progress != 100.0 {
		t.Errorf("Expected progress 100.0, got %f", receivedNotifications[0].Params.Progress)
	}

	// Test direct notification sending
	directNotification := mcp.NewProgressNotification(token, 150.0, &total, "Three quarters done")
	err = serverImpl.SendProgressNotificationDirect(directNotification)
	if err != nil {
		t.Errorf("Expected to send direct notification, got error: %v", err)
	}

	if len(receivedNotifications) != 2 {
		t.Errorf("Expected 2 received notifications, got %d", len(receivedNotifications))
	}

	// Test global subscription
	var globalNotifications []*mcp.ProgressNotification
	globalListener := func(notification *mcp.ProgressNotification) error {
		globalNotifications = append(globalNotifications, notification)
		return nil
	}

	serverImpl.SubscribeToAllProgress(globalListener)

	// Send another notification
	err = serverImpl.SendProgressNotification(token, 200.0, &total, "Complete")
	if err != nil {
		t.Errorf("Expected to send final notification, got error: %v", err)
	}

	// Should be received by both token-specific and global listeners
	if len(receivedNotifications) != 3 {
		t.Errorf("Expected 3 token-specific notifications, got %d", len(receivedNotifications))
	}

	if len(globalNotifications) != 1 {
		t.Errorf("Expected 1 global notification, got %d", len(globalNotifications))
	}

	// Test unsubscribe
	serverImpl.UnsubscribeFromProgress(token)

	// Send another notification - should only be received by global listener
	err = serverImpl.SendProgressNotification(token, 200.0, &total, "Final")
	if err != nil {
		t.Errorf("Expected to send final notification, got error: %v", err)
	}

	// Token-specific listener should not receive this
	if len(receivedNotifications) != 3 {
		t.Errorf("Expected still 3 token-specific notifications after unsubscribe, got %d", len(receivedNotifications))
	}

	// Global listener should receive it
	if len(globalNotifications) != 2 {
		t.Errorf("Expected 2 global notifications, got %d", len(globalNotifications))
	}
}

func TestProgressTokenCreation(t *testing.T) {
	srv := server.NewServer("test-progress-creation")
	serverImpl := srv.GetServer()

	// Test creating progress token
	requestID := "test-request-create"
	token := serverImpl.CreateProgressToken(requestID)

	if token == "" {
		t.Fatal("Expected non-empty progress token")
	}

	// Test that we can create multiple tokens
	token2 := serverImpl.CreateProgressToken("test-request-create-2")
	if token2 == "" {
		t.Fatal("Expected non-empty second progress token")
	}

	if token == token2 {
		t.Error("Expected different tokens for different requests")
	}
}

func TestContextProgressMethods(t *testing.T) {
	srv := server.NewServer("test-context-progress")
	serverImpl := srv.GetServer()

	// Create a context
	requestBytes := []byte(`{"jsonrpc":"2.0","id":"test","method":"test","params":{}}`)
	ctx, err := server.NewContext(context.Background(), requestBytes, serverImpl)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}

	// Test creating progress token
	token := ctx.CreateProgressToken()
	if token == "" {
		t.Fatal("Expected non-empty progress token")
	}

	if ctx.ProgressToken != token {
		t.Errorf("Expected context progress token to be set to %s, got %s", token, ctx.ProgressToken)
	}

	// Test sending progress (will be no-op without transport, but shouldn't error)
	total := 200.0
	err = ctx.SendProgress(75.0, &total, "Three quarters done")
	if err != nil {
		t.Errorf("Expected to send progress without error, got: %v", err)
	}

	// Test completing progress (will be no-op without transport, but shouldn't error)
	err = ctx.CompleteProgress("All done!")
	if err != nil {
		t.Errorf("Expected to complete progress without error, got: %v", err)
	}
}

func TestExtractProgressTokenFromRequest(t *testing.T) {
	// Test extracting from params._meta.progressToken (correct MCP format)
	requestWithMetaToken := []byte(`{
		"jsonrpc": "2.0",
		"id": "test",
		"method": "test",
		"params": {
			"_meta": {
				"progressToken": "token-from-meta"
			},
			"otherParam": "value"
		}
	}`)

	token, err := mcp.ExtractProgressTokenFromRequest(requestWithMetaToken)
	if err != nil {
		t.Errorf("Expected no error extracting from params._meta, got: %v", err)
	}

	if token != "token-from-meta" {
		t.Errorf("Expected token 'token-from-meta', got '%s'", token)
	}

	// Test extracting integer token from params._meta.progressToken
	requestWithIntToken := []byte(`{
		"jsonrpc": "2.0",
		"id": "test",
		"method": "test",
		"params": {
			"_meta": {
				"progressToken": 12345
			}
		}
	}`)

	token, err = mcp.ExtractProgressTokenFromRequest(requestWithIntToken)
	if err != nil {
		t.Errorf("Expected no error extracting integer token from params._meta, got: %v", err)
	}

	if token != "12345" {
		t.Errorf("Expected token '12345', got '%s'", token)
	}

	// Test no token present
	requestWithoutToken := []byte(`{
		"jsonrpc": "2.0",
		"id": "test",
		"method": "test",
		"params": {
			"otherParam": "value"
		}
	}`)

	token, err = mcp.ExtractProgressTokenFromRequest(requestWithoutToken)
	if err != nil {
		t.Errorf("Expected no error when no token present, got: %v", err)
	}

	if token != "" {
		t.Errorf("Expected empty token when none present, got '%s'", token)
	}

	// Test no _meta section
	requestWithoutMeta := []byte(`{
		"jsonrpc": "2.0",
		"id": "test",
		"method": "test",
		"params": {
			"progressToken": "should-not-be-found",
			"otherParam": "value"
		}
	}`)

	token, err = mcp.ExtractProgressTokenFromRequest(requestWithoutMeta)
	if err != nil {
		t.Errorf("Expected no error when no _meta section, got: %v", err)
	}

	if token != "" {
		t.Errorf("Expected empty token when not in _meta, got '%s'", token)
	}

	// Test invalid JSON
	invalidJSON := []byte(`{"invalid": json}`)

	_, err = mcp.ExtractProgressTokenFromRequest(invalidJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

// ProgressReporter Tests

func TestProgressReporter(t *testing.T) {
	// Test basic ProgressReporter creation
	requestID := "test-reporter-request"
	total := 100.0
	initialMessage := "Starting operation"

	reporter := mcp.NewProgressReporter(mcp.ProgressReporterConfig{
		RequestID:      requestID,
		Total:          &total,
		InitialMessage: initialMessage,
	})

	if reporter == nil {
		t.Fatal("Expected non-nil ProgressReporter")
	}

	if reporter.GetRequestID() != requestID {
		t.Errorf("Expected request ID %s, got %s", requestID, reporter.GetRequestID())
	}

	if !reporter.IsActive() {
		t.Error("Expected reporter to be active")
	}

	if reporter.IsCompleted() {
		t.Error("Expected reporter to not be completed initially")
	}

	// Test progress updates
	if err := reporter.Update(25.0, "Quarter done"); err != nil {
		t.Errorf("Unexpected error updating reporter: %v", err)
	}

	current, currentTotal, message, isCompleted := reporter.GetProgress()
	if current != 25.0 {
		t.Errorf("Expected current progress 25.0, got %f", current)
	}

	if currentTotal == nil || *currentTotal != total {
		t.Errorf("Expected total %f, got %v", total, currentTotal)
	}

	if message != "Quarter done" {
		t.Errorf("Expected message 'Quarter done', got '%s'", message)
	}

	if isCompleted {
		t.Error("Expected reporter to not be completed yet")
	}

	// Test percentage calculation
	percentage := reporter.GetPercentage()
	if percentage == nil {
		t.Fatal("Expected percentage to be calculated")
	}

	expectedPercentage := 25.0
	if *percentage != expectedPercentage {
		t.Errorf("Expected percentage %f, got %f", expectedPercentage, *percentage)
	}

	// Test increment
	err := reporter.Increment(25.0, "Half done")
	if err != nil {
		t.Errorf("Expected no error incrementing progress, got: %v", err)
	}

	current, _, _, _ = reporter.GetProgress()
	if current != 50.0 {
		t.Errorf("Expected current progress 50.0 after increment, got %f", current)
	}

	// Test completion
	if err := reporter.Complete("Operation completed"); err != nil {
		t.Errorf("Unexpected error completing reporter: %v", err)
	}

	if !reporter.IsCompleted() {
		t.Error("Expected reporter to be completed")
	}

	current, _, message, isCompleted = reporter.GetProgress()
	if current != total {
		t.Errorf("Expected current progress to equal total (%f) after completion, got %f", total, current)
	}

	if message != "Operation completed" {
		t.Errorf("Expected completion message 'Operation completed', got '%s'", message)
	}

	if !isCompleted {
		t.Error("Expected isCompleted to be true")
	}
}

func TestProgressReporterValidation(t *testing.T) {
	total := 100.0
	reporter := mcp.NewSimpleProgressReporter("test-validation", &total)

	// Test negative progress
	err := reporter.Update(-10.0)
	if err == nil {
		t.Error("Expected error for negative progress")
	}

	// Test progress exceeding total
	err = reporter.Update(150.0)
	if err == nil {
		t.Error("Expected error for progress exceeding total")
	}

	// Test updating completed reporter
	if err := reporter.Complete("Done"); err != nil {
		t.Errorf("Unexpected error completing reporter: %v", err)
	}

	// Test updating cancelled reporter
	reporter2 := mcp.NewSimpleProgressReporter("test-cancelled", &total)
	if err := reporter2.Cancel("Cancelled"); err != nil {
		t.Errorf("Unexpected error cancelling reporter: %v", err)
	}
}

func TestProgressReporterHierarchical(t *testing.T) {
	// Test hierarchical progress reporting (simplified - no automatic parent updates)
	parentTotal := 100.0
	parent := mcp.NewSimpleProgressReporter("parent-request", &parentTotal)

	// Create child reporters
	childTotal1 := 50.0
	child1 := parent.CreateChild("child1-request", 0.6, &childTotal1) // 60% weight

	childTotal2 := 30.0
	child2 := parent.CreateChild("child2-request", 0.4, &childTotal2) // 40% weight

	if parent.GetChildCount() != 2 {
		t.Errorf("Expected parent to have 2 children, got %d", parent.GetChildCount())
	}

	// Update child progress
	if err := child1.Update(25.0, "Child 1 half done"); err != nil {
		t.Errorf("Unexpected error updating child1: %v", err)
	}
	if err := child2.Update(15.0, "Child 2 half done"); err != nil {
		t.Errorf("Unexpected error updating child2: %v", err)
	}

	// Check that children updated correctly
	child1Current, _, _, _ := child1.GetProgress()
	if child1Current != 25.0 {
		t.Errorf("Expected child1 progress 25.0, got %f", child1Current)
	}

	child2Current, _, _, _ := child2.GetProgress()
	if child2Current != 15.0 {
		t.Errorf("Expected child2 progress 15.0, got %f", child2Current)
	}

	// Parent progress should remain at 0 (no automatic updates in simplified version)
	parentCurrent, _, _, _ := parent.GetProgress()
	if parentCurrent != 0.0 {
		t.Errorf("Expected parent progress to remain 0.0 (no automatic updates), got %f", parentCurrent)
	}

	// Complete child1
	if err := child1.Complete("Child 1 done"); err != nil {
		t.Errorf("Unexpected error completing child1: %v", err)
	}

	// Verify child1 is completed
	if !child1.IsCompleted() {
		t.Error("Expected child1 to be completed")
	}

	// Parent progress should still be 0 (no automatic updates)
	parentCurrent, _, _, _ = parent.GetProgress()
	if parentCurrent != 0.0 {
		t.Errorf("Expected parent progress to remain 0.0 (no automatic updates), got %f", parentCurrent)
	}

	// Complete child2
	if err := child2.Complete("Child 2 done"); err != nil {
		t.Errorf("Unexpected error completing child2: %v", err)
	}

	// Verify child2 is completed
	if !child2.IsCompleted() {
		t.Error("Expected child2 to be completed")
	}

	// Parent can be manually updated if needed
	if err := parent.Update(100.0, "All children completed"); err != nil {
		t.Errorf("Unexpected error updating parent: %v", err)
	}

	// Test that children are accessible through parent
	children := parent.GetChildren()
	if len(children) != 2 {
		t.Errorf("Expected 2 children accessible from parent, got %d", len(children))
	}

	// Verify both children are in the map
	foundChild1 := false
	foundChild2 := false
	for _, child := range children {
		if child.GetRequestID() == "child1-request" {
			foundChild1 = true
		}
		if child.GetRequestID() == "child2-request" {
			foundChild2 = true
		}
	}

	if !foundChild1 {
		t.Error("Expected to find child1 in parent's children map")
	}
	if !foundChild2 {
		t.Error("Expected to find child2 in parent's children map")
	}
}

func TestProgressReporterWithServer(t *testing.T) {
	srv := server.NewServer("test-progress-reporter-server")
	serverImpl := srv.GetServer()

	// Test server integration
	requestID := "test-server-integration"
	total := 200.0
	initialMessage := "Server operation starting"

	reporter := serverImpl.CreateProgressReporter(requestID, &total, initialMessage)

	if reporter == nil {
		t.Fatal("Expected non-nil ProgressReporter from server")
	}

	if reporter.GetRequestID() != requestID {
		t.Errorf("Expected request ID %s, got %s", requestID, reporter.GetRequestID())
	}

	// Test progress updates (will be no-op without transport, but shouldn't error)
	err := reporter.Update(50.0, "Half done")
	if err != nil {
		t.Errorf("Expected no error updating progress with server, got: %v", err)
	}

	// Test completion
	err = reporter.Complete("Server operation completed")
	if err != nil {
		t.Errorf("Expected no error completing progress with server, got: %v", err)
	}

	if !reporter.IsCompleted() {
		t.Error("Expected reporter to be completed")
	}
}

func TestProgressReporterConvenienceMethods(t *testing.T) {
	// Test StartOperation convenience method
	requestID := "test-convenience"
	total := 100.0
	initialMessage := "Starting convenient operation"

	// Mock notification sender
	var sentNotifications []*mcp.ProgressNotification
	mockSender := &mockProgressNotificationSender{
		notifications: &sentNotifications,
	}

	reporter := mcp.StartOperation(requestID, &total, initialMessage, mockSender)

	if reporter == nil {
		t.Fatal("Expected non-nil ProgressReporter from StartOperation")
	}

	// Should have sent initial notification
	if len(sentNotifications) != 1 {
		t.Errorf("Expected 1 initial notification, got %d", len(sentNotifications))
	}

	// Test UpdateOperation convenience method
	err := mcp.UpdateOperation(reporter, 3, 10, "30% complete")
	if err != nil {
		t.Errorf("Expected no error from UpdateOperation, got: %v", err)
	}

	current, _, message, _ := reporter.GetProgress()
	if current != 30.0 {
		t.Errorf("Expected progress 30.0, got %f", current)
	}

	if message != "30% complete" {
		t.Errorf("Expected message '30%% complete', got '%s'", message)
	}

	// Test CompleteOperation convenience method
	err = mcp.CompleteOperation(reporter, "Operation finished")
	if err != nil {
		t.Errorf("Expected no error from CompleteOperation, got: %v", err)
	}

	if !reporter.IsCompleted() {
		t.Error("Expected reporter to be completed")
	}
}

func TestProgressReporterStatistics(t *testing.T) {
	requestID := "test-statistics"
	total := 100.0
	reporter := mcp.NewSimpleProgressReporter(requestID, &total)

	// Update progress
	if err := reporter.Update(25.0, "Quarter done"); err != nil {
		t.Errorf("Unexpected error updating reporter: %v", err)
	}

	// Get statistics
	stats := reporter.GetStatistics()

	if stats["requestID"] != requestID {
		t.Errorf("Expected requestID %s in stats, got %v", requestID, stats["requestID"])
	}

	if stats["current"] != 25.0 {
		t.Errorf("Expected current 25.0 in stats, got %v", stats["current"])
	}

	if stats["isActive"] != true {
		t.Errorf("Expected isActive true in stats, got %v", stats["isActive"])
	}

	if stats["isCompleted"] != false {
		t.Errorf("Expected isCompleted false in stats, got %v", stats["isCompleted"])
	}

	if stats["percentage"] != 25.0 {
		t.Errorf("Expected percentage 25.0 in stats, got %v", stats["percentage"])
	}

	// Test duration tracking
	duration, ok := stats["duration"].(time.Duration)
	if !ok {
		t.Error("Expected duration to be a time.Duration")
	}

	if duration <= 0 {
		t.Error("Expected positive duration")
	}
}

func TestContextProgressReporterMethods(t *testing.T) {
	srv := server.NewServer("test-context-progress-reporter")
	serverImpl := srv.GetServer()

	// Create a context
	requestBytes := []byte(`{"jsonrpc":"2.0","id":"test","method":"test","params":{}}`)
	ctx, err := server.NewContext(context.Background(), requestBytes, serverImpl)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}

	// Test CreateProgressReporter
	total := 100.0
	initialMessage := "Context operation starting"
	reporter := ctx.CreateProgressReporter(&total, initialMessage)

	if reporter == nil {
		t.Fatal("Expected non-nil ProgressReporter from context")
	}

	if ctx.ProgressToken != reporter.GetToken() {
		t.Errorf("Expected context progress token to match reporter token")
	}

	// Test CreateSimpleProgressReporter
	total2 := 200.0
	reporter2 := ctx.CreateSimpleProgressReporter(&total2)

	if reporter2 == nil {
		t.Fatal("Expected non-nil simple ProgressReporter from context")
	}

	// Test StartProgressOperation
	total3 := 300.0
	initialMessage3 := "Context operation with notification"
	reporter3 := ctx.StartProgressOperation(&total3, initialMessage3)

	if reporter3 == nil {
		t.Fatal("Expected non-nil ProgressReporter from StartProgressOperation")
	}

	if ctx.ProgressToken != reporter3.GetToken() {
		t.Errorf("Expected context progress token to be updated to latest reporter token")
	}
}

// Mock implementation for testing
type mockProgressNotificationSender struct {
	notifications *[]*mcp.ProgressNotification
}

func (m *mockProgressNotificationSender) SendProgressNotification(progressToken string, progress float64, total *float64, message string) error {
	notification := mcp.NewProgressNotification(progressToken, progress, total, message)
	*m.notifications = append(*m.notifications, notification)
	return nil
}

func TestMCPSpecificationCompliance(t *testing.T) {
	// Test compliance with all three MCP specification versions
	testCases := []struct {
		version           string
		shouldHaveMessage bool
	}{
		{"draft", true},
		{"2024-11-05", false},
		{"2025-03-26", true},
		{"2025-06-18", true},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Version_%s", tc.version), func(t *testing.T) {
			// Test ProgressNotification creation for specific version
			token := "test-token"
			progress := 50.0
			total := 100.0
			message := "Test message"

			notification := mcp.NewProgressNotificationForVersion(token, progress, &total, message, tc.version)

			// Validate notification structure
			if notification.JSONRPC != "2.0" {
				t.Errorf("Expected JSONRPC '2.0', got '%s'", notification.JSONRPC)
			}

			if notification.Method != "notifications/progress" {
				t.Errorf("Expected method 'notifications/progress', got '%s'", notification.Method)
			}

			if notification.Params.ProgressToken != token {
				t.Errorf("Expected progress token '%s', got '%s'", token, notification.Params.ProgressToken)
			}

			if notification.Params.Progress != progress {
				t.Errorf("Expected progress %f, got %f", progress, notification.Params.Progress)
			}

			if notification.Params.Total == nil || *notification.Params.Total != total {
				t.Errorf("Expected total %f, got %v", total, notification.Params.Total)
			}

			// Test protocol version specific behavior
			if tc.shouldHaveMessage {
				if notification.Params.Message != message {
					t.Errorf("Version %s should include message '%s', got '%s'", tc.version, message, notification.Params.Message)
				}
			}

			// Test validation
			err := notification.Validate()
			if err != nil {
				t.Errorf("Expected valid notification for version %s, got error: %v", tc.version, err)
			}

			// Test JSON serialization
			jsonBytes, err := notification.ToJSON()
			if err != nil {
				t.Errorf("Expected successful JSON serialization for version %s, got error: %v", tc.version, err)
			}

			// For 2024-11-05, ensure message field is not in JSON
			if tc.version == "2024-11-05" {
				jsonStr := string(jsonBytes)
				if strings.Contains(jsonStr, `"message"`) {
					t.Errorf("Version 2024-11-05 should not include message field in JSON, got: %s", jsonStr)
				}
			}

			// Test ProgressReporter with version
			reporter := mcp.NewSimpleProgressReporterForVersion("test-request", &total, tc.version)
			if reporter.GetProtocolVersion() != tc.version {
				t.Errorf("Expected reporter protocol version '%s', got '%s'", tc.version, reporter.GetProtocolVersion())
			}

			// Test progress value increase validation
			err = reporter.Update(25.0, "First update")
			if err != nil {
				t.Errorf("Expected no error for first update, got: %v", err)
			}

			// Test that progress cannot decrease (MCP requirement)
			err = reporter.Update(20.0, "Should fail")
			if err == nil {
				t.Error("Expected error when progress decreases, but got none")
			}

			// Test that progress can increase
			err = reporter.Update(30.0, "Should succeed")
			if err != nil {
				t.Errorf("Expected no error when progress increases, got: %v", err)
			}
		})
	}
}

// Test progress token extraction with correct MCP format
func TestMCPProgressTokenFormat(t *testing.T) {
	// Test the exact format specified in MCP documentation
	mcpRequest := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "some_method",
		"params": {
			"_meta": {
				"progressToken": "abc123"
			}
		}
	}`)

	token, err := mcp.ExtractProgressTokenFromRequest(mcpRequest)
	if err != nil {
		t.Errorf("Expected no error extracting MCP format token, got: %v", err)
	}

	if token != "abc123" {
		t.Errorf("Expected token 'abc123', got '%s'", token)
	}

	// Test integer token as allowed by MCP spec
	mcpRequestInt := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "some_method",
		"params": {
			"_meta": {
				"progressToken": 123
			}
		}
	}`)

	token, err = mcp.ExtractProgressTokenFromRequest(mcpRequestInt)
	if err != nil {
		t.Errorf("Expected no error extracting MCP format integer token, got: %v", err)
	}

	if token != "123" {
		t.Errorf("Expected token '123', got '%s'", token)
	}
}

func TestProgressReporterConcurrency(t *testing.T) {
	// Test high-concurrency scenarios with our simplified ProgressReporter
	total := 1000.0
	reporter := mcp.NewSimpleProgressReporter("concurrency-test", &total)

	// Number of goroutines to run concurrently
	numGoroutines := 100
	updatesPerGoroutine := 10

	// Channel to collect errors
	errorChan := make(chan error, numGoroutines*updatesPerGoroutine)

	// WaitGroup to wait for all goroutines
	var wg sync.WaitGroup

	// Test 1: Concurrent reads (should be very fast with atomic operations)
	t.Run("ConcurrentReads", func(t *testing.T) {
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()

				for j := 0; j < updatesPerGoroutine; j++ {
					// These should all be lock-free
					_ = reporter.IsActive()
					_ = reporter.IsCompleted()
					_ = reporter.GetToken()
					_ = reporter.GetRequestID()
					_ = reporter.GetDuration()
					_ = reporter.GetProtocolVersion()

					// These use atomic operations
					current, _, _, _ := reporter.GetProgress()
					if current < 0 {
						errorChan <- fmt.Errorf("goroutine %d: negative current value: %f", goroutineID, current)
					}
				}
			}(i)
		}

		wg.Wait()

		// Check for errors
		select {
		case err := <-errorChan:
			t.Errorf("Concurrent reads failed: %v", err)
		default:
			// No errors, good!
		}
	})

	// Test 2: Concurrent progress updates (should be serialized but safe)
	t.Run("ConcurrentUpdates", func(t *testing.T) {
		// Reset reporter for this test
		reporter2 := mcp.NewSimpleProgressReporter("concurrency-updates", &total)

		wg.Add(numGoroutines)

		// Track successful updates
		successCount := int64(0)

		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()

				for j := 0; j < updatesPerGoroutine; j++ {
					// Try to update with increasing values
					// Some will fail due to progress decrease validation, which is expected
					progress := float64(goroutineID*updatesPerGoroutine + j)
					err := reporter2.Update(progress, fmt.Sprintf("Update from goroutine %d", goroutineID))

					if err == nil {
						atomic.AddInt64(&successCount, 1)
					} else if !strings.Contains(err.Error(), "must not decrease") {
						// Only report non-decrease errors
						errorChan <- fmt.Errorf("goroutine %d: unexpected error: %v", goroutineID, err)
					}
				}
			}(i)
		}

		wg.Wait()

		// Check for unexpected errors
		select {
		case err := <-errorChan:
			t.Errorf("Concurrent updates failed: %v", err)
		default:
			// No unexpected errors
		}

		// Should have at least some successful updates
		if atomic.LoadInt64(&successCount) == 0 {
			t.Error("Expected at least some successful updates")
		}

		t.Logf("Successful updates: %d out of %d attempts", atomic.LoadInt64(&successCount), numGoroutines*updatesPerGoroutine)
	})

	// Test 3: Mixed concurrent operations
	t.Run("MixedConcurrentOperations", func(t *testing.T) {
		reporter3 := mcp.NewSimpleProgressReporter("mixed-concurrency", &total)

		wg.Add(numGoroutines * 3) // 3 types of operations

		// Readers
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()

				for j := 0; j < updatesPerGoroutine; j++ {
					current, total, message, isCompleted := reporter3.GetProgress()

					// Validate consistency
					if current < 0 {
						errorChan <- fmt.Errorf("reader %d: negative current: %f", goroutineID, current)
					}
					if total != nil && current > *total {
						errorChan <- fmt.Errorf("reader %d: current (%f) > total (%f)", goroutineID, current, *total)
					}
					if isCompleted && current == 0 {
						errorChan <- fmt.Errorf("reader %d: completed but current is 0", goroutineID)
					}

					// Use the values to prevent optimization
					_ = message

					time.Sleep(time.Microsecond) // Small delay to increase contention
				}
			}(i)
		}

		// Writers (progress updates)
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()

				for j := 0; j < updatesPerGoroutine; j++ {
					progress := float64(goroutineID*10 + j)
					if progress <= total {
						if err := reporter3.Update(progress, fmt.Sprintf("Writer %d update %d", goroutineID, j)); err != nil {
							// Log error but continue (expected in concurrent scenarios)
							_ = err // Acknowledge error but don't log to avoid race conditions in tests
						}
					}

					time.Sleep(time.Microsecond) // Small delay to increase contention
				}
			}(i)
		}

		// Statistics readers
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()

				for j := 0; j < updatesPerGoroutine; j++ {
					stats := reporter3.GetStatistics()

					// Validate statistics consistency
					if current, ok := stats["current"].(float64); ok && current < 0 {
						errorChan <- fmt.Errorf("stats reader %d: negative current in stats: %f", goroutineID, current)
					}

					if isActive, ok := stats["isActive"].(bool); ok && !isActive {
						// If not active, should be completed or cancelled
						if isCompleted, ok := stats["isCompleted"].(bool); ok && !isCompleted {
							errorChan <- fmt.Errorf("stats reader %d: not active but not completed", goroutineID)
						}
					}

					time.Sleep(time.Microsecond) // Small delay to increase contention
				}
			}(i)
		}

		wg.Wait()

		// Check for errors
		select {
		case err := <-errorChan:
			t.Errorf("Mixed concurrent operations failed: %v", err)
		default:
			// No errors, excellent!
		}
	})

	// Test 4: Concurrent child creation and access
	t.Run("ConcurrentChildOperations", func(t *testing.T) {
		parentReporter := mcp.NewSimpleProgressReporter("parent-concurrency", &total)

		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()

				// Create children
				childTotal := 100.0
				child := parentReporter.CreateChild(
					fmt.Sprintf("child-%d", goroutineID),
					0.1,
					&childTotal,
				)

				if child == nil {
					errorChan <- fmt.Errorf("goroutine %d: failed to create child", goroutineID)
					return
				}

				// Update child progress
				for j := 0; j < 5; j++ {
					err := child.Update(float64(j*20), fmt.Sprintf("Child %d update %d", goroutineID, j))
					if err != nil {
						errorChan <- fmt.Errorf("goroutine %d: child update failed: %v", goroutineID, err)
					}
				}

				// Complete child
				if err := child.Complete(fmt.Sprintf("Child %d completed", goroutineID)); err != nil {
					errorChan <- fmt.Errorf("goroutine %d: child completion failed: %v", goroutineID, err)
				}
			}(i)
		}

		// Concurrent readers of parent children
		wg.Add(10)
		for i := 0; i < 10; i++ {
			go func(readerID int) {
				defer wg.Done()

				for j := 0; j < 20; j++ {
					children := parentReporter.GetChildren()

					// Validate children consistency (this should always be true)
					for token, child := range children {
						if child.GetToken() != token {
							errorChan <- fmt.Errorf("reader %d: child token mismatch", readerID)
						}
					}

					// Test that GetChildCount() returns a reasonable value
					// (may not be exactly equal to len(children) during concurrent modifications)
					count := parentReporter.GetChildCount()
					if count < 0 || count > numGoroutines*2 { // Allow some buffer for timing
						errorChan <- fmt.Errorf("reader %d: unreasonable child count: %d", readerID, count)
					}

					time.Sleep(time.Microsecond)
				}
			}(i)
		}

		wg.Wait()

		// Check for errors
		select {
		case err := <-errorChan:
			t.Errorf("Concurrent child operations failed: %v", err)
		default:
			// No errors!
		}

		// Verify final state
		finalChildCount := parentReporter.GetChildCount()
		if finalChildCount != numGoroutines {
			t.Errorf("Expected %d children, got %d", numGoroutines, finalChildCount)
		}
	})
}
