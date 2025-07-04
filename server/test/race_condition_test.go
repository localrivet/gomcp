package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport"
)

// ThreadSafeBuffer is a thread-safe wrapper around bytes.Buffer
type ThreadSafeBuffer struct {
	mu  sync.RWMutex
	buf bytes.Buffer
}

func (b *ThreadSafeBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *ThreadSafeBuffer) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.buf.String()
}

// TestInitializationSequenceCompliance tests that the server follows proper MCP initialization sequence
func TestInitializationSequenceCompliance(t *testing.T) {
	tests := []struct {
		name            string
		protocolVersion string
	}{
		{"draft", "draft"},
		{"v2025-03-26", "2025-03-26"},
		{"v2024-11-05", "2024-11-05"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testInitializationSequence(t, tt.protocolVersion)
		})
	}
}

func testInitializationSequence(t *testing.T, protocolVersion string) {
	// Create server
	srv := server.NewServer("test-init-sequence")

	// Create mock transport to capture all messages
	transport := NewSequenceCapturingTransport()

	// Get the server implementation
	serverImpl := srv.GetServer()

	// IMPORTANT: Set the transport on the server so notifications go through our mock
	serverImpl.SetTransport(transport)

	// Set the transport message handler
	transport.SetHandler(func(message []byte) {
		// Process message through server
		response, err := server.HandleMessage(serverImpl, message)
		if err != nil {
			t.Errorf("Server error processing message: %v", err)
			return
		}

		// If there's a response, add it to transport's response queue
		if response != nil {
			transport.QueueResponse(response)
		}
	})

	// Register tools to trigger notifications AFTER setting up transport
	srv.Tool("test-tool", "Test tool", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "ok", nil
	})
	time.Sleep(50 * time.Millisecond) // Allow time for async notification processing

	// Step 1: Send initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	initBytes, _ := json.Marshal(initRequest)
	transport.SimulateMessage(initBytes)

	// Verify initialization response was sent before any notifications
	responses := transport.GetResponsesInOrder()
	if len(responses) == 0 {
		t.Fatal("Expected initialize response, got none")
	}

	// Parse first response - should be initialize response
	var initResponse map[string]interface{}
	if err := json.Unmarshal(responses[0], &initResponse); err != nil {
		t.Fatalf("Failed to parse initialize response: %v", err)
	}

	// Verify it's the correct response
	if initResponse["id"].(float64) != 1 {
		t.Errorf("Expected initialize response with id=1, got id=%v", initResponse["id"])
	}

	if initResponse["result"] == nil {
		t.Errorf("Expected initialize response to have result, got: %v", initResponse)
	}

	// At this point, there should be NO notifications sent yet
	notifications := transport.GetNotificationsSentBeforeInitialized()
	if len(notifications) > 0 {
		t.Errorf("Server sent %d notifications before receiving notifications/initialized: %v",
			len(notifications), notifications)
	}

	// Step 2: Send notifications/initialized
	initNotification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	notifBytes, _ := json.Marshal(initNotification)
	transport.SimulateMessage(notifBytes)

	// Give server time to process and send pending notifications
	time.Sleep(100 * time.Millisecond)

	// Now notifications should be sent
	notificationsAfter := transport.GetNotificationsSentAfterInitialized()
	if len(notificationsAfter) == 0 {
		t.Error("Expected server to send notifications after receiving notifications/initialized")
	}

	// Verify tools/list_changed notification was sent
	foundToolsChanged := false
	for _, notif := range notificationsAfter {
		var parsed map[string]interface{}
		if json.Unmarshal(notif, &parsed) == nil {
			if method, ok := parsed["method"].(string); ok && method == "notifications/tools/list_changed" {
				foundToolsChanged = true
				break
			}
		}
	}

	if !foundToolsChanged {
		t.Error("Expected tools/list_changed notification after initialization")
	}
}

// TestConcurrentToolRegistrationRace tests race conditions when tools are registered concurrently
func TestConcurrentToolRegistrationRace(t *testing.T) {
	srv := server.NewServer("test-concurrent-tools")
	transport := NewSequenceCapturingTransport()
	serverImpl := srv.GetServer()

	// IMPORTANT: Set the transport on the server so notifications go through our mock
	serverImpl.SetTransport(transport)

	// Set message handler
	transport.SetHandler(func(message []byte) {
		response, _ := server.HandleMessage(serverImpl, message)
		if response != nil {
			transport.QueueResponse(response)
		}
	})

	// Start concurrent tool registration
	var wg sync.WaitGroup
	numGoroutines := 10
	toolsPerGoroutine := 5

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < toolsPerGoroutine; j++ {
				toolName := fmt.Sprintf("tool-%d-%d", routineID, j)
				srv.Tool(toolName, fmt.Sprintf("Tool %s", toolName),
					func(ctx *server.Context, args interface{}) (interface{}, error) {
						return "ok", nil
					})
				// Small delay to increase chance of race conditions
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	// Wait for all tools to be registered
	wg.Wait()

	// Send initialize + initialized sequence
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "draft",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	initBytes, _ := json.Marshal(initRequest)
	transport.SimulateMessage(initBytes)

	initNotification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	notifBytes, _ := json.Marshal(initNotification)
	transport.SimulateMessage(notifBytes)

	time.Sleep(200 * time.Millisecond)

	// Verify all tools are present in tools/list response
	toolsListRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}

	listBytes, _ := json.Marshal(toolsListRequest)
	transport.SimulateMessage(listBytes)

	responses := transport.GetResponsesInOrder()
	var toolsListResponse map[string]interface{}
	found := false
	for _, resp := range responses {
		var parsed map[string]interface{}
		if json.Unmarshal(resp, &parsed) == nil {
			if id, ok := parsed["id"].(float64); ok && id == 2 {
				toolsListResponse = parsed
				found = true
				break
			}
		}
	}

	if !found {
		t.Fatal("Did not receive tools/list response")
	}

	result, ok := toolsListResponse["result"].(map[string]interface{})
	if !ok {
		t.Fatal("tools/list response missing result")
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatal("tools/list result missing tools array")
	}

	expectedToolCount := numGoroutines * toolsPerGoroutine
	if len(tools) != expectedToolCount {
		t.Errorf("Expected %d tools, got %d", expectedToolCount, len(tools))
	}
}

// TestStdioTransportInitializationRace tests stdio-specific race conditions
func TestStdioTransportInitializationRace(t *testing.T) {
	// Create a buffer to capture stdout
	var stdout bytes.Buffer

	// Use a thread-safe buffer for stderr to avoid race conditions
	var stderr ThreadSafeBuffer

	// Create server with stdio transport
	srv := server.NewServer("test-stdio-race")
	srv.Tool("echo", "Echo tool", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return args, nil
	})

	// Create logger that writes to stderr
	logger := slog.New(slog.NewTextHandler(&stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Get server with logger
	serverWithLogger := server.NewServer("test-stdio-race", server.WithLogger(logger))
	serverWithLogger.Tool("echo", "Echo tool", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return args, nil
	})

	// For testing, we'll simulate the stdio behavior directly
	serverImpl := serverWithLogger.GetServer()

	// Test message sequence
	messages := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"draft","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	}

	var responses [][]byte
	var mu sync.Mutex

	// Process messages sequentially to test race conditions
	for i, msg := range messages {
		msgBytes := []byte(msg)

		// Process message
		response, err := server.HandleMessage(serverImpl, msgBytes)
		if err != nil {
			t.Errorf("Message %d failed: %v", i, err)
			continue
		}

		if response != nil {
			mu.Lock()
			responses = append(responses, response)
			mu.Unlock()

			// Write response to stdout (simulating stdio transport)
			stdout.Write(response)
			stdout.Write([]byte("\n"))
		}

		// Small delay to allow for any async operations
		time.Sleep(10 * time.Millisecond)
	}

	// Verify stdout contains only JSON responses, no debug output
	stdoutContent := stdout.String()
	lines := bytes.Split([]byte(stdoutContent), []byte("\n"))

	for i, line := range lines {
		if len(line) == 0 {
			continue
		}

		// Each line should be valid JSON
		var jsonObj map[string]interface{}
		if err := json.Unmarshal(line, &jsonObj); err != nil {
			t.Errorf("Line %d in stdout is not valid JSON: %s", i, string(line))
		}

		// Should be a JSON-RPC response
		if jsonrpc, ok := jsonObj["jsonrpc"].(string); !ok || jsonrpc != "2.0" {
			t.Errorf("Line %d is not a valid JSON-RPC 2.0 message: %s", i, string(line))
		}
	}

	// Wait a bit longer for any pending goroutines to complete their logging
	time.Sleep(50 * time.Millisecond)

	// Verify stderr contains debug output (if any)
	stderrContent := stderr.String()
	if len(stderrContent) > 0 {
		t.Logf("Debug output (correctly sent to stderr): %s", stderrContent)
	}
}

// SequenceCapturingTransport captures messages and responses in order
type SequenceCapturingTransport struct {
	transport.BaseTransport
	mu                      sync.Mutex
	responses               [][]byte
	notifications           [][]byte
	notificationsBeforeInit [][]byte
	notificationsAfterInit  [][]byte
	initializedReceived     bool
	messageHandler          func([]byte)
}

func NewSequenceCapturingTransport() *SequenceCapturingTransport {
	return &SequenceCapturingTransport{
		responses:               [][]byte{},
		notifications:           [][]byte{},
		notificationsBeforeInit: [][]byte{},
		notificationsAfterInit:  [][]byte{},
	}
}

// Transport interface methods
func (t *SequenceCapturingTransport) Initialize() error {
	return nil
}

func (t *SequenceCapturingTransport) Start() error {
	return nil
}

func (t *SequenceCapturingTransport) Stop() error {
	return nil
}

func (t *SequenceCapturingTransport) Receive() ([]byte, error) {
	return nil, nil
}

func (t *SequenceCapturingTransport) Send(message []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if this is a notification (no ID field)
	var jsonMsg map[string]interface{}
	if json.Unmarshal(message, &jsonMsg) == nil {
		if _, hasID := jsonMsg["id"]; !hasID {
			// It's a notification
			t.notifications = append(t.notifications, append([]byte{}, message...))

			// Use the current state of initializedReceived to categorize
			if t.initializedReceived {
				t.notificationsAfterInit = append(t.notificationsAfterInit, append([]byte{}, message...))
			} else {
				t.notificationsBeforeInit = append(t.notificationsBeforeInit, append([]byte{}, message...))
			}
		} else {
			// It's a response, add to responses
			t.responses = append(t.responses, append([]byte{}, message...))
		}
	}

	return nil
}

func (t *SequenceCapturingTransport) SendAsync(ctx context.Context, message []byte) error {
	return t.Send(message)
}

func (t *SequenceCapturingTransport) SetHandler(handler func([]byte)) {
	t.messageHandler = handler
}

func (t *SequenceCapturingTransport) SimulateMessage(message []byte) {
	// Check if this is notifications/initialized
	var jsonMsg map[string]interface{}
	if json.Unmarshal(message, &jsonMsg) == nil {
		if method, ok := jsonMsg["method"].(string); ok && method == "notifications/initialized" {
			t.mu.Lock()
			t.initializedReceived = true
			t.mu.Unlock()
		}
	}

	if t.messageHandler != nil {
		t.messageHandler(message)
	}
}

func (t *SequenceCapturingTransport) QueueResponse(response []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.responses = append(t.responses, append([]byte{}, response...))
}

func (t *SequenceCapturingTransport) GetResponsesInOrder() [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([][]byte, len(t.responses))
	for i, resp := range t.responses {
		result[i] = append([]byte{}, resp...)
	}
	return result
}

func (t *SequenceCapturingTransport) GetNotificationsSentBeforeInitialized() [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([][]byte, len(t.notificationsBeforeInit))
	for i, notif := range t.notificationsBeforeInit {
		result[i] = append([]byte{}, notif...)
	}
	return result
}

func (t *SequenceCapturingTransport) GetNotificationsSentAfterInitialized() [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([][]byte, len(t.notificationsAfterInit))
	for i, notif := range t.notificationsAfterInit {
		result[i] = append([]byte{}, notif...)
	}
	return result
}

// TestNotificationTimingRace tests that notifications are properly timed
func TestNotificationTimingRace(t *testing.T) {
	srv := server.NewServer("test-timing")
	transport := NewSequenceCapturingTransport()
	serverImpl := srv.GetServer()

	// IMPORTANT: Set the transport on the server so notifications go through our mock
	serverImpl.SetTransport(transport)

	// Set message handler
	transport.SetHandler(func(message []byte) {
		response, _ := server.HandleMessage(serverImpl, message)
		if response != nil {
			transport.QueueResponse(response)
		}
	})

	// Register multiple tools at different times
	srv.Tool("tool1", "Tool 1", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "tool1", nil
	})

	// Send initialize
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "draft",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	initBytes, _ := json.Marshal(initRequest)
	transport.SimulateMessage(initBytes)

	// Register another tool after initialize but before initialized notification
	srv.Tool("tool2", "Tool 2", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "tool2", nil
	})

	// Should still have no notifications
	earlyNotifications := transport.GetNotificationsSentBeforeInitialized()
	if len(earlyNotifications) > 0 {
		t.Errorf("Server sent notifications too early: %d notifications", len(earlyNotifications))
	}

	// Send initialized notification
	initNotification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	notifBytes, _ := json.Marshal(initNotification)
	transport.SimulateMessage(notifBytes)

	// Register yet another tool after initialized
	srv.Tool("tool3", "Tool 3", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "tool3", nil
	})

	// Allow time for async notifications
	time.Sleep(150 * time.Millisecond)

	// Now should have notifications
	lateNotifications := transport.GetNotificationsSentAfterInitialized()
	if len(lateNotifications) == 0 {
		t.Error("Expected notifications after initialized, got none")
	}

	// Should have at least one tools/list_changed notification
	toolsChangedCount := 0
	for _, notif := range lateNotifications {
		var parsed map[string]interface{}
		if json.Unmarshal(notif, &parsed) == nil {
			if method, ok := parsed["method"].(string); ok && method == "notifications/tools/list_changed" {
				toolsChangedCount++
			}
		}
	}

	if toolsChangedCount == 0 {
		t.Error("Expected at least one tools/list_changed notification")
	}

	// Verify all tools are available via tools/list
	toolsListRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}

	listBytes, _ := json.Marshal(toolsListRequest)
	transport.SimulateMessage(listBytes)

	responses := transport.GetResponsesInOrder()
	var toolsListResponse map[string]interface{}
	for _, resp := range responses {
		var parsed map[string]interface{}
		if json.Unmarshal(resp, &parsed) == nil {
			if id, ok := parsed["id"].(float64); ok && id == 2 {
				toolsListResponse = parsed
				break
			}
		}
	}

	if toolsListResponse == nil {
		t.Fatal("Did not receive tools/list response")
	}

	result, ok := toolsListResponse["result"].(map[string]interface{})
	if !ok {
		t.Fatal("tools/list response missing result")
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatal("tools/list result missing tools array")
	}

	// Should have all 3 tools
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
	}

	// Verify tool names
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		if toolMap, ok := tool.(map[string]interface{}); ok {
			if name, ok := toolMap["name"].(string); ok {
				toolNames[i] = name
			}
		}
	}

	expectedTools := []string{"tool1", "tool2", "tool3"}
	for _, expected := range expectedTools {
		found := false
		for _, actual := range toolNames {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Tool %s not found in tools list: %v", expected, toolNames)
		}
	}
}
