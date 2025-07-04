package v20250618

import (
	"testing"

	"github.com/localrivet/gomcp/client"
	clienttest "github.com/localrivet/gomcp/client/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test2025_03_26SamplingRequest(t *testing.T) {
	// Create a simple sampling request
	messages := []client.SamplingMessage{
		clienttest.CreateTextSamplingMessage("user", "Hello, world!"),
	}

	prefs := client.SamplingModelPreferences{
		Hints: []client.SamplingModelHint{
			{Name: "test-model"},
		},
	}

	opts := clienttest.NewSamplingRequest(messages, prefs)
	opts.SystemPrompt = "You are a test assistant"
	opts.MaxTokens = 100
	opts.ProtocolVersion = "2025-06-18"

	// Test validation
	err := opts.Validate()
	require.NoError(t, err)

	// Test content types for 2025-06-18 version
	t.Run("TextContent", func(t *testing.T) {
		textMsg := clienttest.CreateTextSamplingMessage("user", "Hello, world!")
		textOpts := clienttest.NewSamplingRequest([]client.SamplingMessage{textMsg}, prefs)
		textOpts.ProtocolVersion = "2025-06-18"

		err := textOpts.Validate()
		require.NoError(t, err)

		// Verify content is valid for version
		assert.True(t, textMsg.Content.IsValidForVersion("2025-06-18"))
	})

	t.Run("ImageContent", func(t *testing.T) {
		imageMsg := clienttest.CreateImageSamplingMessage("user", "test-image-data", "image/png")
		imageOpts := clienttest.NewSamplingRequest([]client.SamplingMessage{imageMsg}, prefs)
		imageOpts.ProtocolVersion = "2025-06-18"

		err := imageOpts.Validate()
		require.NoError(t, err)

		// Verify content is valid for version
		assert.True(t, imageMsg.Content.IsValidForVersion("2025-06-18"))
	})

	t.Run("AudioContent", func(t *testing.T) {
		// Audio is supported in 2025-06-18
		audioMsg := clienttest.CreateAudioSamplingMessage("user", "test-audio-data", "audio/mp3")
		audioOpts := clienttest.NewSamplingRequest([]client.SamplingMessage{audioMsg}, prefs)
		audioOpts.ProtocolVersion = "2025-06-18"

		err := audioOpts.Validate()
		require.NoError(t, err)

		// Verify content is valid for version
		assert.True(t, audioMsg.Content.IsValidForVersion("2025-06-18"))
	})
}

func Test2025_03_26StreamingSamplingRequest(t *testing.T) {
	// Test streaming support in 2025-06-18
	t.Run("StreamingSupported", func(t *testing.T) {
		supported := clienttest.IsStreamingSupportedForVersion("2025-06-18")
		assert.True(t, supported)
	})

	t.Run("StreamingOptions", func(t *testing.T) {
		messages := []client.SamplingMessage{
			clienttest.CreateTextSamplingMessage("user", "Tell me a story"),
		}
		prefs := clienttest.NewSamplingConfig()

		opts := clienttest.NewStreamingSamplingRequest(messages, prefs)
		opts.ProtocolVersion = "2025-06-18"
		opts.StreamHandler = func(*client.SamplingResponse) error { return nil }

		err := opts.Validate()
		require.NoError(t, err)
		assert.True(t, opts.Streaming)
		assert.NotNil(t, opts.StreamHandler)
	})

	t.Run("StreamingChatRequest", func(t *testing.T) {
		messages := []client.SamplingMessage{
			clienttest.CreateTextSamplingMessage("user", "Tell me a story"),
		}

		opts, err := clienttest.CreateStreamingChatRequest(messages, "Be creative", "2025-06-18")
		require.NoError(t, err)
		assert.Equal(t, "2025-06-18", opts.ProtocolVersion)
		assert.Equal(t, "Be creative", opts.SystemPrompt)
		assert.True(t, opts.Streaming)
	})

	t.Run("ChunkSizeValidation", func(t *testing.T) {
		messages := []client.SamplingMessage{
			clienttest.CreateTextSamplingMessage("user", "Test"),
		}
		prefs := clienttest.NewSamplingConfig()

		opts := clienttest.NewSamplingRequest(messages, prefs)
		opts.ProtocolVersion = "2025-06-18"
		opts.Streaming = true
		opts.StreamHandler = func(*client.SamplingResponse) error { return nil }

		// Valid chunk size
		opts.ChunkSize = 100
		err := opts.Validate()
		require.NoError(t, err)

		// Invalid chunk size (too small)
		opts.ChunkSize = 5
		err = opts.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "chunk size must be at least 10 characters")

		// Invalid chunk size (too large)
		opts.ChunkSize = 1001
		err = opts.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "chunk size cannot exceed 1000 characters")
	})
}

func Test2025_03_26ContentValidation(t *testing.T) {
	testCases := []struct {
		name          string
		contentType   string
		shouldBeValid bool
	}{
		{"text content", "text", true},
		{"image content", "image", true},
		{"audio content", "audio", true},
		{"invalid content", "video", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := client.SamplingMessageContent{
				Type: tc.contentType,
			}

			isValid := content.IsValidForVersion("2025-06-18")
			assert.Equal(t, tc.shouldBeValid, isValid)
		})
	}
}
