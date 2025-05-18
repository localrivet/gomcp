package test

import (
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSamplingConfig_Creation tests the construction of the SamplingConfig
func TestSamplingConfig_Creation(t *testing.T) {
	// Test the default construction
	config := client.NewSamplingConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 100, config.MaxTokens, "Default max tokens should be 100")
	assert.Equal(t, 1000, config.MaxSystemPrompt, "Default max system prompt should be 1000")
	assert.True(t, config.SupportedContentTypes["text"], "Text content should be supported by default")
	assert.Equal(t, time.Duration(30*time.Second), config.RequestTimeout, "Default timeout should be 30s")
	assert.False(t, config.StreamingSupported, "Streaming should not be supported by default")
}

// TestSamplingConfig_ForVersion tests version-specific configurations
func TestSamplingConfig_ForVersion(t *testing.T) {
	tests := []struct {
		name            string
		version         string
		expectedTokens  int
		expectedPrompt  int
		expectedContent map[string]bool
		expectError     bool
	}{
		{
			name:           "draft version",
			version:        "draft",
			expectedTokens: 2048,
			expectedPrompt: 10000,
			expectedContent: map[string]bool{
				"text":  true,
				"image": true,
				"audio": true,
			},
			expectError: false,
		},
		{
			name:           "2024-11-05 version",
			version:        "2024-11-05",
			expectedTokens: 4096,
			expectedPrompt: 10000,
			expectedContent: map[string]bool{
				"text":  true,
				"image": true,
			},
			expectError: false,
		},
		{
			name:           "2025-03-26 version",
			version:        "2025-03-26",
			expectedTokens: 8192,
			expectedPrompt: 16000,
			expectedContent: map[string]bool{
				"text":  true,
				"image": true,
				"audio": true,
			},
			expectError: false,
		},
		{
			name:        "invalid version",
			version:     "2023-invalid",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := client.NewSamplingConfig()
			result, err := config.ForVersion(tc.version)

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedTokens, result.MaxTokens)
			assert.Equal(t, tc.expectedPrompt, result.MaxSystemPrompt)

			for contentType, expected := range tc.expectedContent {
				actual, exists := result.SupportedContentTypes[contentType]
				assert.True(t, exists, "Content type %s should exist", contentType)
				assert.Equal(t, expected, actual, "Content type %s should be %v", contentType, expected)
			}

			assert.Equal(t, tc.version, result.ProtocolVersion)
		})
	}
}

// TestSamplingConfig_FluentMethods tests the fluent API methods
func TestSamplingConfig_FluentMethods(t *testing.T) {
	// Test all the fluent configuration methods
	config := client.NewSamplingConfig().
		WithMaxTokens(500).
		WithMaxSystemPrompt(2000).
		WithSupportedContentType("image", true).
		WithRequestTimeout(60*time.Second).
		WithStreamingSupport(true).
		WithChunkSizeRange(20, 200, 50).
		WithModelNameMaxLength(150)

	assert.Equal(t, 500, config.MaxTokens)
	assert.Equal(t, 2000, config.MaxSystemPrompt)
	assert.True(t, config.SupportedContentTypes["image"])
	assert.Equal(t, time.Duration(60*time.Second), config.RequestTimeout)
	assert.True(t, config.StreamingSupported)
	assert.Equal(t, 20, config.MinChunkSize)
	assert.Equal(t, 200, config.MaxChunkSize)
	assert.Equal(t, 50, config.DefaultChunkSize)
	assert.Equal(t, 150, config.ModelNameMaxLength)

	// Test RetryConfig setting
	customRetry := client.RetryConfig{
		MaxRetries:      5,
		RetryInterval:   100 * time.Millisecond,
		RetryMultiplier: 2.0,
		MaxInterval:     5 * time.Second,
	}

	config = config.WithRetryConfig(customRetry)
	assert.Equal(t, 5, config.RetryConfig.MaxRetries)
	assert.Equal(t, time.Duration(100*time.Millisecond), config.RetryConfig.RetryInterval)
	assert.Equal(t, 2.0, config.RetryConfig.RetryMultiplier)
	assert.Equal(t, time.Duration(5*time.Second), config.RetryConfig.MaxInterval)
}

// TestSamplingConfig_OptimizeMethods tests the optimization methods
func TestSamplingConfig_OptimizeMethods(t *testing.T) {
	t.Run("OptimizeForCompletion", func(t *testing.T) {
		config := client.NewSamplingConfig().
			OptimizeForCompletion()

		assert.Equal(t, time.Duration(20*time.Second), config.RequestTimeout)
		assert.Equal(t, 2, config.RetryConfig.MaxRetries)
		assert.Equal(t, time.Duration(500*time.Millisecond), config.RetryConfig.RetryInterval)
		assert.Equal(t, 1.2, config.RetryConfig.RetryMultiplier)
	})

	t.Run("OptimizeForStreamingChat", func(t *testing.T) {
		// Test with 2025-03-26 (supports streaming)
		config := client.NewSamplingConfig()
		config, err := config.ForVersion("2025-03-26")
		require.NoError(t, err)

		config = config.OptimizeForStreamingChat()
		assert.True(t, config.StreamingSupported)
		assert.Equal(t, 20, config.DefaultChunkSize)
		assert.Equal(t, 5, config.RetryConfig.MaxRetries)

		// Test with 2024-11-05 (doesn't support streaming)
		config = client.NewSamplingConfig()
		config, err = config.ForVersion("2024-11-05")
		require.NoError(t, err)

		config = config.OptimizeForStreamingChat()
		assert.False(t, config.StreamingSupported)
	})

	t.Run("OptimizeForImageGeneration", func(t *testing.T) {
		// Test with valid image support
		config := client.NewSamplingConfig()
		config, err := config.ForVersion("2024-11-05")
		require.NoError(t, err)

		config, err = config.OptimizeForImageGeneration()
		require.NoError(t, err)
		assert.Equal(t, time.Duration(60*time.Second), config.RequestTimeout)
		assert.Equal(t, 3, config.RetryConfig.MaxRetries)
		assert.Equal(t, time.Duration(1*time.Second), config.RetryConfig.RetryInterval)

		// Test token doubling within limits
		assert.Equal(t, 4096, config.MaxTokens) // Limited by version max

		// Test without image support
		config = client.NewSamplingConfig()
		config.SupportedContentTypes = map[string]bool{"text": true}
		config.ProtocolVersion = "custom" // Just for testing

		_, err = config.OptimizeForImageGeneration()
		assert.Error(t, err)
	})
}

// TestSamplingConfig_Validation tests the configuration validation
func TestSamplingConfig_Validation(t *testing.T) {
	t.Run("ValidateForVersion valid", func(t *testing.T) {
		config := client.NewSamplingConfig().
			WithMaxTokens(4000).
			WithMaxSystemPrompt(9000).
			WithSupportedContentType("text", true).
			WithSupportedContentType("image", true)

		err := config.ValidateForVersion("2024-11-05")
		assert.NoError(t, err)
	})

	t.Run("ValidateForVersion invalid token count", func(t *testing.T) {
		config := client.NewSamplingConfig().
			WithMaxTokens(10000) // Exceeds all version limits

		err := config.ValidateForVersion("2024-11-05")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max tokens")
	})

	t.Run("ValidateForVersion invalid content type", func(t *testing.T) {
		config := client.NewSamplingConfig().
			WithSupportedContentType("audio", true)

		err := config.ValidateForVersion("2024-11-05")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "content type")
	})

	t.Run("ValidateForVersion invalid streaming", func(t *testing.T) {
		config := client.NewSamplingConfig().
			WithStreamingSupport(true)

		err := config.ValidateForVersion("2024-11-05")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "streaming not supported")
	})
}

// TestSamplingConfig_Apply tests applying configuration to requests
func TestSamplingConfig_Apply(t *testing.T) {
	t.Run("Apply to request", func(t *testing.T) {
		config := client.NewSamplingConfig().
			WithMaxTokens(500).
			WithMaxSystemPrompt(5000).
			WithRequestTimeout(15*time.Second).
			WithSupportedContentType("text", true).
			WithSupportedContentType("image", true)

		req := client.NewSamplingRequest(
			[]client.SamplingMessage{
				client.CreateTextSamplingMessage("user", "Test message"),
			},
			client.SamplingModelPreferences{},
		).WithProtocolVersion("2024-11-05")

		err := config.Apply(req)
		assert.NoError(t, err)
		assert.Equal(t, 500, req.MaxTokens)
		assert.Equal(t, time.Duration(15*time.Second), req.Timeout)
	})

	t.Run("Apply with version mismatch", func(t *testing.T) {
		config := client.NewSamplingConfig()
		config, err := config.ForVersion("2025-03-26")
		require.NoError(t, err)

		req := client.NewSamplingRequest(
			[]client.SamplingMessage{
				client.CreateTextSamplingMessage("user", "Test message"),
			},
			client.SamplingModelPreferences{},
		).WithProtocolVersion("2024-11-05")

		err = config.Apply(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be applied to request with version")
	})
}

// TestSamplingHelperFunctions tests the helper functions for creating sampling requests
func TestSamplingHelperFunctions(t *testing.T) {
	t.Run("CreateTextCompletionRequest", func(t *testing.T) {
		req, err := client.CreateTextCompletionRequest("Test prompt", "2024-11-05")
		require.NoError(t, err)
		assert.Equal(t, "2024-11-05", req.ProtocolVersion)
		assert.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Equal(t, "text", req.Messages[0].Content.Type)
		assert.Equal(t, "Test prompt", req.Messages[0].Content.Text)
	})

	t.Run("CreateChatRequest", func(t *testing.T) {
		messages := []client.SamplingMessage{
			client.CreateTextSamplingMessage("user", "Hello"),
			client.CreateTextSamplingMessage("assistant", "Hi there"),
		}

		req, err := client.CreateChatRequest(messages, "Be helpful", "2024-11-05")
		require.NoError(t, err)
		assert.Equal(t, "2024-11-05", req.ProtocolVersion)
		assert.Len(t, req.Messages, 2)
		assert.Equal(t, "Be helpful", req.SystemPrompt)
	})

	t.Run("CreateStreamingChatRequest", func(t *testing.T) {
		messages := []client.SamplingMessage{
			client.CreateTextSamplingMessage("user", "Tell me a story"),
		}

		// Should fail for non-streaming version
		_, err := client.CreateStreamingChatRequest(messages, "Be creative", "2024-11-05")
		assert.Error(t, err)

		// Should work for streaming-enabled version
		req, err := client.CreateStreamingChatRequest(messages, "Be creative", "2025-03-26")
		require.NoError(t, err)
		assert.Equal(t, "2025-03-26", req.ProtocolVersion)
		assert.True(t, req.ChunkSize > 0)
	})

	t.Run("CreateImageGenerationRequest", func(t *testing.T) {
		req, err := client.CreateImageGenerationRequest("A beautiful sunset", "2024-11-05")
		require.NoError(t, err)
		assert.Equal(t, "2024-11-05", req.ProtocolVersion)
		assert.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Equal(t, "text", req.Messages[0].Content.Type)
		assert.Equal(t, "A beautiful sunset", req.Messages[0].Content.Text)

		// Should have image-capable hint
		assert.Len(t, req.ModelPreferences.Hints, 1)
		assert.Equal(t, "image-capable", req.ModelPreferences.Hints[0].Name)
	})
}
