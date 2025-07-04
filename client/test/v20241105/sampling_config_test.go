package v20241105

import (
	"testing"

	"github.com/localrivet/gomcp/client"
	clienttest "github.com/localrivet/gomcp/client/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamingSamplingConfig_NotSupported tests that the 2024-11-05 protocol
// correctly rejects streaming operations
func TestStreamingSamplingConfig_NotSupported(t *testing.T) {
	t.Run("CreateStreamingChatRequest", func(t *testing.T) {
		messages := []client.SamplingMessage{
			clienttest.CreateTextSamplingMessage("user", "Tell me a story"),
		}

		// Should succeed but validation should fail for non-streaming version
		opts, err := clienttest.CreateStreamingChatRequest(messages, "Be creative", "2024-11-05")
		require.NoError(t, err)

		// Validation should fail because streaming is not supported in 2024-11-05
		err = opts.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "streaming is only supported in protocol versions 2025-03-26 and 2025-06-18")
	})

	t.Run("StreamingSupport", func(t *testing.T) {
		// Test that streaming is not supported in 2024-11-05
		supported := clienttest.IsStreamingSupportedForVersion("2024-11-05")
		assert.False(t, supported)

		// But it is supported in 2025-03-26 and 2025-06-18
		supported = clienttest.IsStreamingSupportedForVersion("2025-03-26")
		assert.True(t, supported)

		supported = clienttest.IsStreamingSupportedForVersion("2025-06-18")
		assert.True(t, supported)
	})

	t.Run("SamplingOptions", func(t *testing.T) {
		messages := []client.SamplingMessage{
			clienttest.CreateTextSamplingMessage("user", "Hello"),
		}
		prefs := clienttest.NewSamplingConfig()

		opts := clienttest.NewSamplingRequest(messages, prefs)
		opts.ProtocolVersion = "2024-11-05"

		// Non-streaming should work
		err := opts.Validate()
		require.NoError(t, err)

		// But streaming should fail
		opts.Streaming = true
		err = opts.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "streaming is only supported in protocol versions 2025-03-26 and 2025-06-18")
	})
}
