// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"fmt"
	"time"
)

// SamplingConfig defines a complete configuration for sampling operations,
// allowing users to customize behavior within protocol constraints.
type SamplingConfig struct {
	// Protocol specific parameters
	MaxTokens             int             // Maximum tokens to generate
	MaxSystemPrompt       int             // Maximum allowed system prompt length
	SupportedContentTypes map[string]bool // Map of supported content types
	ProtocolVersion       string          // The protocol version this config is for

	// Timeout and retry settings
	RequestTimeout time.Duration
	RetryConfig    RetryConfig

	// Streaming settings
	StreamingSupported bool
	MinChunkSize       int
	MaxChunkSize       int
	DefaultChunkSize   int

	// Model configuration
	ModelNameMaxLength int
}

// NewSamplingConfig creates a new sampling configuration with sensible defaults.
// This is the ONLY constructor for SamplingConfig - all customization is done
// through fluent methods following this constructor.
func NewSamplingConfig() *SamplingConfig {
	return &SamplingConfig{
		MaxTokens:       100, // Conservative default
		MaxSystemPrompt: 1000,
		SupportedContentTypes: map[string]bool{
			"text": true, // Text is supported in all versions
		},
		RequestTimeout: 30 * time.Second,
		RetryConfig:    DefaultRetryConfig(),

		StreamingSupported: false,
		MinChunkSize:       10,
		MaxChunkSize:       100,
		DefaultChunkSize:   50,

		ModelNameMaxLength: 100,
	}
}

// ForVersion configures the sampling config for a specific protocol version,
// setting all version-specific constraints and capabilities.
func (c *SamplingConfig) ForVersion(version string) (*SamplingConfig, error) {
	if !IsProtocolVersionSupported(version) {
		return nil, fmt.Errorf("unsupported protocol version: %s", version)
	}

	c.ProtocolVersion = version

	switch version {
	case "draft":
		// Default draft configuration
		c.MaxTokens = 2048
		c.MaxSystemPrompt = 10000
		c.SupportedContentTypes = map[string]bool{
			"text":  true,
			"image": true,
			"audio": true,
		}
	case "2024-11-05":
		// 2024-11-05 configuration
		c.MaxTokens = 4096
		c.MaxSystemPrompt = 10000
		c.SupportedContentTypes = map[string]bool{
			"text":  true,
			"image": true,
		}
		c.ModelNameMaxLength = 100
	case "2025-03-26":
		// 2025-03-26 configuration
		c.MaxTokens = 8192
		c.MaxSystemPrompt = 16000
		c.SupportedContentTypes = map[string]bool{
			"text":  true,
			"image": true,
			"audio": true,
		}
		c.StreamingSupported = true
		c.MinChunkSize = 10
		c.MaxChunkSize = 1000
		c.DefaultChunkSize = 100
		c.ModelNameMaxLength = 200
	}

	return c, nil
}

// WithMaxTokens sets the maximum number of tokens for the configuration
func (c *SamplingConfig) WithMaxTokens(maxTokens int) *SamplingConfig {
	c.MaxTokens = maxTokens
	return c
}

// WithMaxSystemPrompt sets the maximum system prompt length
func (c *SamplingConfig) WithMaxSystemPrompt(maxLength int) *SamplingConfig {
	c.MaxSystemPrompt = maxLength
	return c
}

// WithSupportedContentType adds or removes a content type from supported types
func (c *SamplingConfig) WithSupportedContentType(contentType string, supported bool) *SamplingConfig {
	if c.SupportedContentTypes == nil {
		c.SupportedContentTypes = make(map[string]bool)
	}
	c.SupportedContentTypes[contentType] = supported
	return c
}

// WithRequestTimeout sets the request timeout duration
func (c *SamplingConfig) WithRequestTimeout(timeout time.Duration) *SamplingConfig {
	c.RequestTimeout = timeout
	return c
}

// WithRetryConfig sets the retry configuration
func (c *SamplingConfig) WithRetryConfig(retryConfig RetryConfig) *SamplingConfig {
	c.RetryConfig = retryConfig
	return c
}

// WithStreamingSupport enables or disables streaming support
func (c *SamplingConfig) WithStreamingSupport(supported bool) *SamplingConfig {
	c.StreamingSupported = supported
	return c
}

// WithChunkSizeRange sets the min, max, and default chunk sizes for streaming
func (c *SamplingConfig) WithChunkSizeRange(min, max, defaultSize int) *SamplingConfig {
	c.MinChunkSize = min
	c.MaxChunkSize = max
	c.DefaultChunkSize = defaultSize
	return c
}

// WithModelNameMaxLength sets the maximum length for model names
func (c *SamplingConfig) WithModelNameMaxLength(maxLength int) *SamplingConfig {
	c.ModelNameMaxLength = maxLength
	return c
}

// OptimizeForCompletion configures the sampling for text completions
func (c *SamplingConfig) OptimizeForCompletion() *SamplingConfig {
	// Optimize for text completion
	c.RequestTimeout = 20 * time.Second

	// Conservative retry configuration
	c.RetryConfig = RetryConfig{
		MaxRetries:      2,
		RetryInterval:   500 * time.Millisecond,
		RetryMultiplier: 1.2,
		MaxInterval:     3 * time.Second,
	}

	return c
}

// OptimizeForStreamingChat configures the sampling for streaming chat
func (c *SamplingConfig) OptimizeForStreamingChat() *SamplingConfig {
	// Only enable streaming if supported by this version
	if c.ProtocolVersion == "2025-03-26" {
		c.StreamingSupported = true
		c.DefaultChunkSize = 20 // Smaller chunks for more responsive UI
	} else {
		c.StreamingSupported = false
	}

	// More aggressive retry for streaming
	c.RetryConfig = RetryConfig{
		MaxRetries:      5,
		RetryInterval:   200 * time.Millisecond,
		RetryMultiplier: 1.3,
		MaxInterval:     2 * time.Second,
	}

	return c
}

// OptimizeForImageGeneration configures the sampling for image generation
func (c *SamplingConfig) OptimizeForImageGeneration() (*SamplingConfig, error) {
	// Check if image content is supported
	if !c.SupportedContentTypes["image"] {
		return nil, fmt.Errorf("image content not supported in protocol version %s", c.ProtocolVersion)
	}

	// Image generation needs more time and tokens
	c.RequestTimeout = 60 * time.Second

	// Double the tokens (within version limits)
	// We need to know the version-specific max to enforce limits
	var versionMaxTokens int
	switch c.ProtocolVersion {
	case "draft":
		versionMaxTokens = 2048
	case "2024-11-05":
		versionMaxTokens = 4096
	case "2025-03-26":
		versionMaxTokens = 8192
	default:
		// If we don't know the version, use a conservative limit
		versionMaxTokens = c.MaxTokens
	}

	// Set tokens to double or version max, whichever is smaller
	doubled := c.MaxTokens * 2
	if doubled > versionMaxTokens {
		c.MaxTokens = versionMaxTokens
	} else {
		c.MaxTokens = doubled
	}

	// More patient retry settings for image generation
	c.RetryConfig = RetryConfig{
		MaxRetries:      3,
		RetryInterval:   1 * time.Second,
		RetryMultiplier: 2.0,
		MaxInterval:     15 * time.Second,
	}

	return c, nil
}

// Apply applies the configuration to a sampling request
func (c *SamplingConfig) Apply(req *SamplingRequest) error {
	// Ensure protocol version is set
	if req.ProtocolVersion == "" {
		return fmt.Errorf("cannot apply config: protocol version not set in request")
	}

	// If the config has a protocol version set, it must match the request's version
	if c.ProtocolVersion != "" && c.ProtocolVersion != req.ProtocolVersion {
		return fmt.Errorf("configuration for protocol version %s cannot be applied to request with version %s",
			c.ProtocolVersion, req.ProtocolVersion)
	}

	// Validate the configuration against the protocol version
	if err := c.ValidateForVersion(req.ProtocolVersion); err != nil {
		return fmt.Errorf("invalid configuration for protocol version %s: %w", req.ProtocolVersion, err)
	}

	// Apply configuration to the request
	if c.MaxTokens > 0 {
		req.MaxTokens = c.MaxTokens
	}

	if c.MaxSystemPrompt > 0 && len(req.SystemPrompt) > c.MaxSystemPrompt {
		// Truncate system prompt if necessary
		req.SystemPrompt = req.SystemPrompt[:c.MaxSystemPrompt]
	}

	// Apply request timeout
	if c.RequestTimeout > 0 {
		req.Timeout = c.RequestTimeout
	}

	// Apply retry configuration if set
	if c.RetryConfig != (RetryConfig{}) {
		req.RetryConfig = c.RetryConfig
	}

	// Validate all content types
	for i, msg := range req.Messages {
		if !c.SupportedContentTypes[msg.Content.Type] {
			return fmt.Errorf("message %d content type '%s' not supported in configuration for protocol version '%s'",
				i, msg.Content.Type, req.ProtocolVersion)
		}
	}

	return nil
}

// ApplyToStreaming applies the configuration to a streaming sampling request
func (c *SamplingConfig) ApplyToStreaming(req *StreamingSamplingRequest) error {
	// First apply common config
	if err := c.Apply(&req.SamplingRequest); err != nil {
		return err
	}

	// Check if streaming is supported
	if !c.StreamingSupported {
		return fmt.Errorf("streaming not supported in protocol version %s", req.ProtocolVersion)
	}

	// Apply streaming-specific settings
	if req.ChunkSize == 0 {
		req.ChunkSize = c.DefaultChunkSize
	} else if req.ChunkSize < c.MinChunkSize {
		req.ChunkSize = c.MinChunkSize
	} else if req.ChunkSize > c.MaxChunkSize {
		req.ChunkSize = c.MaxChunkSize
	}

	return nil
}

// ValidateForVersion validates the configuration against a protocol version
func (c *SamplingConfig) ValidateForVersion(version string) error {
	// Check if the protocol version is supported
	if !IsProtocolVersionSupported(version) {
		return fmt.Errorf("unsupported protocol version: %s", version)
	}

	// Create a reference config for comparison
	refConfig := NewSamplingConfig()
	_, err := refConfig.ForVersion(version)
	if err != nil {
		return err
	}

	// Check if max tokens exceeds version limit
	if c.MaxTokens > refConfig.MaxTokens {
		return fmt.Errorf("max tokens (%d) exceeds limit (%d) for protocol version %s",
			c.MaxTokens, refConfig.MaxTokens, version)
	}

	// Check if system prompt length exceeds version limit
	if c.MaxSystemPrompt > refConfig.MaxSystemPrompt {
		return fmt.Errorf("max system prompt length (%d) exceeds limit (%d) for protocol version %s",
			c.MaxSystemPrompt, refConfig.MaxSystemPrompt, version)
	}

	// Check streaming settings
	if c.StreamingSupported && !refConfig.StreamingSupported {
		return fmt.Errorf("streaming not supported in protocol version %s", version)
	}

	// If streaming is configured, check streaming-specific settings
	if c.StreamingSupported {
		if c.MinChunkSize < refConfig.MinChunkSize {
			return fmt.Errorf("min chunk size (%d) below limit (%d) for protocol version %s",
				c.MinChunkSize, refConfig.MinChunkSize, version)
		}
		if c.MaxChunkSize > refConfig.MaxChunkSize {
			return fmt.Errorf("max chunk size (%d) exceeds limit (%d) for protocol version %s",
				c.MaxChunkSize, refConfig.MaxChunkSize, version)
		}
	}

	// Check supported content types
	for contentType := range c.SupportedContentTypes {
		if c.SupportedContentTypes[contentType] && !refConfig.SupportedContentTypes[contentType] {
			return fmt.Errorf("content type '%s' not supported in protocol version %s", contentType, version)
		}
	}

	return nil
}

// Helper functions for common sampling operations

// CreateTextCompletionRequest creates a request optimized for text completion
func CreateTextCompletionRequest(prompt string, version string) (*SamplingRequest, error) {
	config := NewSamplingConfig()
	if _, err := config.ForVersion(version); err != nil {
		return nil, err
	}
	config.OptimizeForCompletion()

	// Create user message with the prompt
	messages := []SamplingMessage{
		CreateTextSamplingMessage("user", prompt),
	}

	// Create basic model preferences
	prefs := SamplingModelPreferences{}

	// Create the request
	req := NewSamplingRequest(messages, prefs)
	req.WithProtocolVersion(version)

	// Apply configuration to the request
	if err := config.Apply(req); err != nil {
		return nil, err
	}

	return req, nil
}

// CreateChatRequest creates a request for a chat conversation
func CreateChatRequest(messages []SamplingMessage, systemPrompt string, version string) (*SamplingRequest, error) {
	// Validate messages
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}

	config := NewSamplingConfig()
	if _, err := config.ForVersion(version); err != nil {
		return nil, err
	}

	// Create basic model preferences
	prefs := SamplingModelPreferences{}

	// Create the request
	req := NewSamplingRequest(messages, prefs)
	req.WithProtocolVersion(version)

	// Add system prompt if provided
	if systemPrompt != "" {
		// Truncate if necessary
		if len(systemPrompt) > config.MaxSystemPrompt {
			systemPrompt = systemPrompt[:config.MaxSystemPrompt]
		}
		req.WithSystemPrompt(systemPrompt)
	}

	// Apply configuration to the request
	if err := config.Apply(req); err != nil {
		return nil, err
	}

	return req, nil
}

// CreateStreamingChatRequest creates a request for streaming chat responses
func CreateStreamingChatRequest(messages []SamplingMessage, systemPrompt string, version string) (*StreamingSamplingRequest, error) {
	config := NewSamplingConfig()
	if _, err := config.ForVersion(version); err != nil {
		return nil, err
	}
	config.OptimizeForStreamingChat()

	// Check if streaming is supported
	if !config.StreamingSupported {
		return nil, fmt.Errorf("streaming not supported in protocol version %s", version)
	}

	// Create basic model preferences
	prefs := SamplingModelPreferences{}

	// Create the streaming request
	req := NewStreamingSamplingRequest(messages, prefs)
	req.WithProtocolVersion(version)

	// Add system prompt if provided
	if systemPrompt != "" {
		// Truncate if necessary
		if len(systemPrompt) > config.MaxSystemPrompt {
			systemPrompt = systemPrompt[:config.MaxSystemPrompt]
		}
		req.WithSystemPrompt(systemPrompt)
	}

	// Apply configuration to the request
	if err := config.ApplyToStreaming(req); err != nil {
		return nil, err
	}

	return req, nil
}

// CreateImageGenerationRequest creates a request for image generation
func CreateImageGenerationRequest(prompt string, version string) (*SamplingRequest, error) {
	config := NewSamplingConfig()
	if _, err := config.ForVersion(version); err != nil {
		return nil, err
	}

	// Optimize for image generation
	if _, err := config.OptimizeForImageGeneration(); err != nil {
		return nil, err
	}

	// Create user message with the prompt
	messages := []SamplingMessage{
		CreateTextSamplingMessage("user", prompt),
	}

	// Create model preferences optimized for image generation
	prefs := SamplingModelPreferences{
		Hints: []SamplingModelHint{
			{Name: "image-capable"},
		},
	}

	// Create the request
	req := NewSamplingRequest(messages, prefs)
	req.WithProtocolVersion(version)

	// Apply configuration to the request
	if err := config.Apply(req); err != nil {
		return nil, err
	}

	return req, nil
}
