package test

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/localrivet/gomcp/server"
	"github.com/stretchr/testify/assert"
)

func TestSamplingConfig(t *testing.T) {
	// Test default configuration
	config := server.NewDefaultSamplingConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 120, config.MaxRequestsPerMinute)
	assert.Equal(t, 10, config.MaxConcurrentRequests)
	assert.Equal(t, 30*time.Second, config.DefaultTimeout)
	assert.Equal(t, true, config.GracefulDegradation)

	// Test protocol specific configurations
	draftConfig := config.ProtocolDefaults["draft"]
	assert.NotNil(t, draftConfig)
	assert.Equal(t, 2048, draftConfig.MaxTokens)
	assert.True(t, draftConfig.SupportedContentTypes["audio"])

	v2024Config := config.ProtocolDefaults["2024-11-05"]
	assert.NotNil(t, v2024Config)
	assert.Equal(t, 4096, v2024Config.MaxTokens)
	assert.True(t, v2024Config.SupportedContentTypes["text"])
	assert.True(t, v2024Config.SupportedContentTypes["image"])
	assert.False(t, v2024Config.SupportedContentTypes["audio"])

	v2025Config := config.ProtocolDefaults["2025-03-26"]
	assert.NotNil(t, v2025Config)
	assert.Equal(t, 8192, v2025Config.MaxTokens)
	assert.True(t, v2025Config.SupportedContentTypes["audio"])
	assert.True(t, v2025Config.StreamingSupported)
}

func TestSamplingController(t *testing.T) {
	// Create a logger for testing
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Create config and controller
	config := server.NewDefaultSamplingConfig()
	controller := server.NewSamplingController(config, logger)
	defer controller.Stop() // Clean up resources

	// Test rate limiting
	sessionID := server.SessionID("test-session")

	// Should initially be allowed
	assert.True(t, controller.CanProcessRequest(sessionID))

	// Record a request
	controller.RecordRequest(sessionID)

	// Should still be allowed since we're below the limit
	assert.True(t, controller.CanProcessRequest(sessionID))

	// Set a very low limit for testing
	config.MaxConcurrentRequests = 1

	// At this point, we have 1 recorded request and the limit is now 1,
	// so we're at the limit. No need to record another request.

	// Should be blocked due to concurrent limit
	assert.False(t, controller.CanProcessRequest(sessionID))

	// Complete a request
	controller.CompleteRequest(sessionID)

	// Should be allowed again
	assert.True(t, controller.CanProcessRequest(sessionID))

	// Test options based on priority
	lowPriorityOptions := controller.GetRequestOptions(1)
	highPriorityOptions := controller.GetRequestOptions(10)

	// Higher priority should get more generous timeout
	assert.Greater(t, highPriorityOptions.Timeout, lowPriorityOptions.Timeout)
}

func TestSamplingValidation(t *testing.T) {
	// Create a logger for testing
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Create config and controller
	config := server.NewDefaultSamplingConfig()
	controller := server.NewSamplingController(config, logger)
	defer controller.Stop()

	// Test protocol validation
	messages := []server.SamplingMessage{
		server.CreateTextSamplingMessage("user", "This is a test message"),
	}

	// Valid token count should pass
	err := controller.ValidateForProtocol("2024-11-05", messages, 2000)
	assert.NoError(t, err)

	// Exceeding token limit should fail
	err = controller.ValidateForProtocol("2024-11-05", messages, 5000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maxTokens exceeds maximum value")

	// Create messages with different content types
	textMessage := server.CreateTextSamplingMessage("user", "Text content")
	imageMessage := server.CreateImageSamplingMessage("user", "base64data", "image/png")
	audioMessage := server.CreateAudioSamplingMessage("user", "base64data", "audio/mp3")

	// 2024-11-05 should reject audio
	mixedMessages := []server.SamplingMessage{textMessage, audioMessage}
	err = controller.ValidateForProtocol("2024-11-05", mixedMessages, 1000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported in protocol version")

	// 2025-03-26 should accept all types
	allMessages := []server.SamplingMessage{textMessage, imageMessage, audioMessage}
	err = controller.ValidateForProtocol("2025-03-26", allMessages, 1000)
	assert.NoError(t, err)
}
