package v20250618

import (
	"testing"

	"github.com/localrivet/gomcp/client"
	clienttest "github.com/localrivet/gomcp/client/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamingSamplingConfig tests the streaming capabilities of the v20250326 protocol
func TestStreamingSamplingConfig(t *testing.T) {
	// Test streaming support detection
	t.Run("StreamingSupport", func(t *testing.T) {
		// Test that streaming is supported in 2025-06-18
		supported := clienttest.IsStreamingSupportedForVersion("2025-06-18")
		assert.True(t, supported)

		// But not in older versions
		supported = clienttest.IsStreamingSupportedForVersion("2024-11-05")
		assert.False(t, supported)
	})

	// Test creating streaming options
	t.Run("StreamingOptions", func(t *testing.T) {
		messages := []client.SamplingMessage{
			clienttest.CreateTextSamplingMessage("user", "Test message"),
		}
		prefs := clienttest.NewSamplingConfig()

		// Create a streaming request using helper
		opts := clienttest.NewStreamingSamplingRequest(messages, prefs)
		opts.ProtocolVersion = "2025-06-18"
		opts.StreamHandler = func(*client.SamplingResponse) error { return nil }

		// Should validate successfully
		err := opts.Validate()
		require.NoError(t, err)
		assert.True(t, opts.Streaming)
		assert.NotNil(t, opts.StreamHandler)
	})

	// Test the streaming chat request creation helper
	t.Run("CreateStreamingChatRequest", func(t *testing.T) {
		messages := []client.SamplingMessage{
			clienttest.CreateTextSamplingMessage("user", "Tell me a story"),
		}

		// Should work for streaming-enabled version
		opts, err := clienttest.CreateStreamingChatRequest(messages, "Be creative", "2025-06-18")
		require.NoError(t, err)
		assert.Equal(t, "2025-06-18", opts.ProtocolVersion)
		assert.Equal(t, "Be creative", opts.SystemPrompt)
		assert.True(t, opts.Streaming)
	})

	// Test chunk size configuration
	t.Run("ChunkSizeConfiguration", func(t *testing.T) {
		messages := []client.SamplingMessage{
			clienttest.CreateTextSamplingMessage("user", "Test"),
		}
		prefs := clienttest.NewSamplingConfig()

		opts := clienttest.NewSamplingRequest(messages, prefs).
			WithStreaming(func(*client.SamplingResponse) error { return nil }).
			WithChunkSize(100)

		opts.ProtocolVersion = "2025-06-18"

		err := opts.Validate()
		require.NoError(t, err)
		assert.Equal(t, 100, opts.ChunkSize)
	})
}
