package v20250326

import (
	"testing"

	"github.com/localrivet/gomcp/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamingSamplingConfig tests the streaming capabilities of the v20250326 protocol
func TestStreamingSamplingConfig(t *testing.T) {
	// Test applying streaming configuration to a streaming request
	t.Run("ApplyToStreaming", func(t *testing.T) {
		config := client.NewSamplingConfig()
		config, err := config.ForVersion("2025-03-26")
		require.NoError(t, err)

		config = config.WithStreamingSupport(true).
			WithChunkSizeRange(30, 300, 150)

		// Create the messages
		messages := []client.SamplingMessage{
			client.CreateTextSamplingMessage("user", "Test message"),
		}
		prefs := client.SamplingModelPreferences{}

		// Create a streaming request - use the correct type
		streamingReq := client.NewStreamingSamplingRequest(messages, prefs)
		// Directly set the protocol version field
		streamingReq.ProtocolVersion = "2025-03-26"
		// Set the initial chunk size to 0 to ensure it gets set by the config
		streamingReq.ChunkSize = 0

		// Apply the config
		err = config.ApplyToStreaming(streamingReq)
		assert.NoError(t, err)
		assert.Equal(t, 150, streamingReq.ChunkSize, "Expected chunk size to be 150, got %d", streamingReq.ChunkSize)
	})

	// Test the streaming chat request creation helper
	t.Run("CreateStreamingChatRequest", func(t *testing.T) {
		messages := []client.SamplingMessage{
			client.CreateTextSamplingMessage("user", "Tell me a story"),
		}

		// Should work for streaming-enabled version
		req, err := client.CreateStreamingChatRequest(messages, "Be creative", "2025-03-26")
		require.NoError(t, err)
		assert.Equal(t, "2025-03-26", req.ProtocolVersion)
		assert.True(t, req.ChunkSize > 0)
	})
}
