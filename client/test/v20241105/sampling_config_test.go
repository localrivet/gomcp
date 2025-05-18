package v20241105

import (
	"testing"

	"github.com/localrivet/gomcp/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamingSamplingConfig_NotSupported tests that the 2024-11-05 protocol
// correctly rejects streaming operations
func TestStreamingSamplingConfig_NotSupported(t *testing.T) {
	t.Run("CreateStreamingChatRequest", func(t *testing.T) {
		messages := []client.SamplingMessage{
			client.CreateTextSamplingMessage("user", "Tell me a story"),
		}

		// Should fail for non-streaming version
		_, err := client.CreateStreamingChatRequest(messages, "Be creative", "2024-11-05")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "streaming not supported")
	})

	t.Run("StreamingConfig", func(t *testing.T) {
		config := client.NewSamplingConfig()
		config, err := config.ForVersion("2024-11-05")
		require.NoError(t, err)

		// Streaming should not be supported
		assert.False(t, config.StreamingSupported)

		// Streaming optimization should leave it as false
		config = config.OptimizeForStreamingChat()
		assert.False(t, config.StreamingSupported)
	})
}
