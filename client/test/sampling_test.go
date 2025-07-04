package test

import (
	"context"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSamplingMessageContent_IsValidForVersion(t *testing.T) {
	testCases := []struct {
		name          string
		content       client.SamplingMessageContent
		version       string
		shouldBeValid bool
	}{
		{
			name: "text content in draft",
			content: client.SamplingMessageContent{
				Type: "text",
				Text: "Hello world",
			},
			version:       "draft",
			shouldBeValid: true,
		},
		{
			name: "text content in 2024-11-05",
			content: client.SamplingMessageContent{
				Type: "text",
				Text: "Hello world",
			},
			version:       "2024-11-05",
			shouldBeValid: true,
		},
		{
			name: "text content in 2025-03-26",
			content: client.SamplingMessageContent{
				Type: "text",
				Text: "Hello world",
			},
			version:       "2025-03-26",
			shouldBeValid: true,
		},
		{
			name: "image content in 2024-11-05",
			content: client.SamplingMessageContent{
				Type:     "image",
				Data:     "base64-image-data",
				MimeType: "image/jpeg",
			},
			version:       "2024-11-05",
			shouldBeValid: true,
		},
		{
			name: "audio content in 2025-03-26",
			content: client.SamplingMessageContent{
				Type:     "audio",
				Data:     "base64-audio-data",
				MimeType: "audio/wav",
			},
			version:       "2025-03-26",
			shouldBeValid: true,
		},
		{
			name: "audio content in 2025-06-18",
			content: client.SamplingMessageContent{
				Type:     "audio",
				Data:     "base64-audio-data",
				MimeType: "audio/wav",
			},
			version:       "2025-06-18",
			shouldBeValid: true,
		},
		{
			name: "audio content in 2024-11-05 (not supported)",
			content: client.SamplingMessageContent{
				Type:     "audio",
				Data:     "base64-audio-data",
				MimeType: "audio/wav",
			},
			version:       "2024-11-05",
			shouldBeValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isValid := tc.content.IsValidForVersion(tc.version)
			assert.Equal(t, tc.shouldBeValid, isValid)
		})
	}
}

func TestSamplingOptions_Validation(t *testing.T) {
	t.Run("valid options", func(t *testing.T) {
		messages := []client.SamplingMessage{
			client.CreateTextMessage("user", "Hello"),
		}
		prefs := client.SamplingModelPreferences{}

		opts := client.NewSamplingOptions(messages, prefs)
		opts.ProtocolVersion = "2025-03-26"

		err := opts.Validate()
		require.NoError(t, err)
	})

	t.Run("no messages", func(t *testing.T) {
		prefs := client.SamplingModelPreferences{}
		opts := client.NewSamplingOptions([]client.SamplingMessage{}, prefs)

		err := opts.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one message is required")
	})

	t.Run("streaming without handler", func(t *testing.T) {
		messages := []client.SamplingMessage{
			client.CreateTextMessage("user", "Hello"),
		}
		prefs := client.SamplingModelPreferences{}

		opts := client.NewSamplingOptions(messages, prefs)
		opts.ProtocolVersion = "2025-03-26"
		opts.Streaming = true
		// No StreamHandler set

		err := opts.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "stream handler is required for streaming mode")
	})

	t.Run("streaming in unsupported version", func(t *testing.T) {
		messages := []client.SamplingMessage{
			client.CreateTextMessage("user", "Hello"),
		}
		prefs := client.SamplingModelPreferences{}

		opts := client.NewSamplingOptions(messages, prefs)
		opts.ProtocolVersion = "2024-11-05"
		opts.Streaming = true
		opts.StreamHandler = func(*client.SamplingResponse) error { return nil }

		err := opts.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "streaming is only supported in protocol versions 2025-03-26 and 2025-06-18")
	})

	t.Run("invalid chunk size", func(t *testing.T) {
		messages := []client.SamplingMessage{
			client.CreateTextMessage("user", "Hello"),
		}
		prefs := client.SamplingModelPreferences{}

		opts := client.NewSamplingOptions(messages, prefs)
		opts.ProtocolVersion = "2025-03-26"
		opts.Streaming = true
		opts.StreamHandler = func(*client.SamplingResponse) error { return nil }
		opts.ChunkSize = 5 // Too small

		err := opts.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "chunk size must be at least 10 characters")
	})
}

func TestSamplingOptions_FluentAPI(t *testing.T) {
	messages := []client.SamplingMessage{
		client.CreateTextMessage("user", "Hello"),
	}
	prefs := client.SamplingModelPreferences{}

	opts := client.NewSamplingOptions(messages, prefs).
		WithSystemPrompt("You are helpful").
		WithMaxTokens(100).
		WithTimeout(30 * time.Second).
		WithContext(context.Background()).
		WithChunkSize(50)

	assert.Equal(t, "You are helpful", opts.SystemPrompt)
	assert.Equal(t, 100, opts.MaxTokens)
	assert.Equal(t, 30*time.Second, opts.Timeout)
	assert.NotNil(t, opts.Context)
	assert.Equal(t, 50, opts.ChunkSize)
}

func TestSamplingOptions_StreamingConfiguration(t *testing.T) {
	messages := []client.SamplingMessage{
		client.CreateTextMessage("user", "Tell me a story"),
	}
	prefs := client.SamplingModelPreferences{}

	var receivedChunks []*client.SamplingResponse
	streamHandler := func(chunk *client.SamplingResponse) error {
		receivedChunks = append(receivedChunks, chunk)
		return nil
	}

	opts := client.NewSamplingOptions(messages, prefs).
		WithStreaming(streamHandler).
		WithChunkSize(100)

	assert.True(t, opts.Streaming)
	assert.NotNil(t, opts.StreamHandler)
	assert.Equal(t, 100, opts.ChunkSize)

	// Test the handler
	testChunk := &client.SamplingResponse{
		Role: "assistant",
		Content: client.SamplingMessageContent{
			Type: "text",
			Text: "Once upon a time...",
		},
		IsComplete: false,
		ChunkIndex: 0,
	}

	err := opts.StreamHandler(testChunk)
	require.NoError(t, err)
	assert.Len(t, receivedChunks, 1)
	assert.Equal(t, testChunk, receivedChunks[0])
}

func TestSamplingHelperFunctions(t *testing.T) {
	t.Run("CreateTextMessage", func(t *testing.T) {
		msg := client.CreateTextMessage("user", "Hello world")
		assert.Equal(t, "user", msg.Role)
		assert.Equal(t, "text", msg.Content.Type)
		assert.Equal(t, "Hello world", msg.Content.Text)
	})

	t.Run("CreateImageMessage", func(t *testing.T) {
		msg := client.CreateImageMessage("user", "base64-data", "image/jpeg")
		assert.Equal(t, "user", msg.Role)
		assert.Equal(t, "image", msg.Content.Type)
		assert.Equal(t, "base64-data", msg.Content.Data)
		assert.Equal(t, "image/jpeg", msg.Content.MimeType)
	})

	t.Run("CreateAudioMessage", func(t *testing.T) {
		msg := client.CreateAudioMessage("user", "base64-audio", "audio/wav")
		assert.Equal(t, "user", msg.Role)
		assert.Equal(t, "audio", msg.Content.Type)
		assert.Equal(t, "base64-audio", msg.Content.Data)
		assert.Equal(t, "audio/wav", msg.Content.MimeType)
	})

	t.Run("NewSamplingOptions", func(t *testing.T) {
		messages := []client.SamplingMessage{
			client.CreateTextMessage("user", "test"),
		}
		prefs := client.SamplingModelPreferences{}

		opts := client.NewSamplingOptions(messages, prefs)
		assert.NotNil(t, opts)
		assert.Equal(t, messages, opts.Messages)
		assert.Equal(t, prefs, opts.ModelPreferences)

		// Check defaults
		assert.Equal(t, 30*time.Second, opts.Timeout)
		assert.Equal(t, 3, opts.MaxRetries)
		assert.Equal(t, 1*time.Second, opts.RetryInterval)
		assert.Equal(t, 2.0, opts.RetryMultiplier)
		assert.Equal(t, 10*time.Second, opts.MaxInterval)
		assert.True(t, opts.StopOnComplete)
	})
}

func TestSamplingModelPreferences(t *testing.T) {
	t.Run("empty preferences", func(t *testing.T) {
		prefs := client.SamplingModelPreferences{}
		assert.Empty(t, prefs.Hints)
		assert.Nil(t, prefs.CostPriority)
		assert.Nil(t, prefs.SpeedPriority)
		assert.Nil(t, prefs.IntelligencePriority)
	})

	t.Run("preferences with hints and priorities", func(t *testing.T) {
		costPriority := 0.3
		speedPriority := 0.8
		intelligencePriority := 0.5

		prefs := client.SamplingModelPreferences{
			Hints: []client.SamplingModelHint{
				{Name: "claude-3-sonnet"},
				{Name: "claude"},
			},
			CostPriority:         &costPriority,
			SpeedPriority:        &speedPriority,
			IntelligencePriority: &intelligencePriority,
		}

		assert.Len(t, prefs.Hints, 2)
		assert.Equal(t, "claude-3-sonnet", prefs.Hints[0].Name)
		assert.Equal(t, "claude", prefs.Hints[1].Name)
		assert.Equal(t, 0.3, *prefs.CostPriority)
		assert.Equal(t, 0.8, *prefs.SpeedPriority)
		assert.Equal(t, 0.5, *prefs.IntelligencePriority)
	})
}

func TestStreamingSupportDetection(t *testing.T) {
	testCases := []struct {
		version   string
		supported bool
	}{
		{"draft", false},
		{"2024-11-05", false},
		{"2025-03-26", true},
		{"2025-06-18", true},
		{"invalid", false},
	}

	for _, tc := range testCases {
		t.Run(tc.version, func(t *testing.T) {
			// Test by trying to validate streaming options
			messages := []client.SamplingMessage{
				client.CreateTextMessage("user", "test"),
			}
			opts := client.NewSamplingOptions(messages, client.SamplingModelPreferences{})
			opts.ProtocolVersion = tc.version
			opts.Streaming = true
			opts.StreamHandler = func(*client.SamplingResponse) error { return nil }

			err := opts.Validate()
			if tc.supported {
				assert.NoError(t, err, "streaming should be supported in %s", tc.version)
			} else {
				assert.Error(t, err, "streaming should not be supported in %s", tc.version)
				if err != nil {
					assert.Contains(t, err.Error(), "streaming is only supported in protocol versions 2025-03-26 and 2025-06-18")
				}
			}
		})
	}
}
