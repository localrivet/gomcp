// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// SamplingRequest represents a request to the sampling API.
// It contains all the necessary information to construct a properly
// formatted request for any protocol version.
type SamplingRequest struct {
	// Core request parameters
	Messages         []SamplingMessage
	ModelPreferences SamplingModelPreferences
	SystemPrompt     string
	MaxTokens        int

	// Context and options
	Context              context.Context
	Timeout              time.Duration
	RetryConfig          RetryConfig
	ProtocolVersion      string
	FormatAsNotification bool
}

// RetryConfig defines the retry behavior for sampling requests
type RetryConfig struct {
	MaxRetries      int
	RetryInterval   time.Duration
	RetryMultiplier float64 // For exponential backoff
	MaxInterval     time.Duration
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      3,
		RetryInterval:   500 * time.Millisecond,
		RetryMultiplier: 1.5,
		MaxInterval:     10 * time.Second,
	}
}

// NewSamplingRequest creates a new sampling request with the given parameters
func NewSamplingRequest(messages []SamplingMessage, prefs SamplingModelPreferences) *SamplingRequest {
	return &SamplingRequest{
		Messages:         messages,
		ModelPreferences: prefs,
		Context:          context.Background(),
		Timeout:          30 * time.Second, // Default timeout
		RetryConfig:      DefaultRetryConfig(),
	}
}

// WithContext sets the context for the request
func (req *SamplingRequest) WithContext(ctx context.Context) *SamplingRequest {
	req.Context = ctx
	return req
}

// WithTimeout sets the timeout for the request
func (req *SamplingRequest) WithTimeout(timeout time.Duration) *SamplingRequest {
	req.Timeout = timeout
	return req
}

// WithRetryConfig sets the retry configuration for the request
func (req *SamplingRequest) WithRetryConfig(config RetryConfig) *SamplingRequest {
	req.RetryConfig = config
	return req
}

// WithSystemPrompt sets the system prompt for the request
func (req *SamplingRequest) WithSystemPrompt(prompt string) *SamplingRequest {
	req.SystemPrompt = prompt
	return req
}

// WithMaxTokens sets the maximum number of tokens for the request
func (req *SamplingRequest) WithMaxTokens(maxTokens int) *SamplingRequest {
	req.MaxTokens = maxTokens
	return req
}

// WithProtocolVersion sets the protocol version for the request
// This is normally set automatically when the request is sent
func (req *SamplingRequest) WithProtocolVersion(version string) *SamplingRequest {
	req.ProtocolVersion = version
	return req
}

// AsNotification formats the request as a notification instead of a request
func (req *SamplingRequest) AsNotification() *SamplingRequest {
	req.FormatAsNotification = true
	return req
}

// Validate checks if the request is valid
func (req *SamplingRequest) Validate() error {
	// Check for required fields
	if len(req.Messages) == 0 {
		return fmt.Errorf("messages cannot be empty")
	}

	// Validate each message and its content
	for i, msg := range req.Messages {
		if msg.Role == "" {
			return fmt.Errorf("message %d has empty role", i)
		}

		// If protocol version is set, validate content against it
		if req.ProtocolVersion != "" {
			if !msg.Content.IsValidForVersion(req.ProtocolVersion) {
				return fmt.Errorf("message %d content type '%s' not supported in protocol version '%s'",
					i, msg.Content.Type, req.ProtocolVersion)
			}
		}
	}

	// Validate system prompt length based on protocol version
	if req.ProtocolVersion != "" && req.SystemPrompt != "" {
		// Protocol-specific validations
		switch req.ProtocolVersion {
		case "draft":
			// No specific constraints for system prompt in draft
		case "2024-11-05":
			if len(req.SystemPrompt) > 10000 {
				return fmt.Errorf("system prompt exceeds maximum length (10000 characters) for protocol version %s", req.ProtocolVersion)
			}
		case "2025-03-26":
			if len(req.SystemPrompt) > 16000 {
				return fmt.Errorf("system prompt exceeds maximum length (16000 characters) for protocol version %s", req.ProtocolVersion)
			}
		default:
			return fmt.Errorf("unknown protocol version: %s", req.ProtocolVersion)
		}
	}

	// Validate max tokens based on protocol version
	if req.ProtocolVersion != "" && req.MaxTokens > 0 {
		switch req.ProtocolVersion {
		case "draft":
			// No specific constraints for max tokens in draft
		case "2024-11-05":
			if req.MaxTokens > 4096 {
				return fmt.Errorf("maxTokens exceeds maximum value (4096) for protocol version %s", req.ProtocolVersion)
			}
		case "2025-03-26":
			if req.MaxTokens > 8192 {
				return fmt.Errorf("maxTokens exceeds maximum value (8192) for protocol version %s", req.ProtocolVersion)
			}
		}
	}

	return nil
}

// BuildCreateMessageRequest builds a JSON-RPC request for sampling/createMessage
func (req *SamplingRequest) BuildCreateMessageRequest(id int) ([]byte, error) {
	// Create the parameters object
	params := SamplingCreateMessageParams{
		Messages:         req.Messages,
		ModelPreferences: req.ModelPreferences,
		SystemPrompt:     req.SystemPrompt,
		MaxTokens:        req.MaxTokens,
	}

	// Create the request object
	var request map[string]interface{}

	if req.FormatAsNotification {
		// Format as a notification (no id)
		request = map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "sampling/createMessage",
			"params":  params,
		}
	} else {
		// Format as a request with id
		request = map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  "sampling/createMessage",
			"params":  params,
		}
	}

	// Add protocol version if specified
	if req.ProtocolVersion != "" {
		// Different versions might format this differently
		switch req.ProtocolVersion {
		case "draft", "2024-11-05", "2025-03-26":
			// All current versions use the same format
			// Create a copy of the params with protocol version
			paramsMap := map[string]interface{}{
				"messages":         params.Messages,
				"modelPreferences": params.ModelPreferences,
				"systemPrompt":     params.SystemPrompt,
				"maxTokens":        params.MaxTokens,
				"protocolVersion":  req.ProtocolVersion,
			}
			request["params"] = paramsMap
		}
	}

	// Marshal to JSON
	return json.Marshal(request)
}

// AddTextMessage adds a text message to the request
func (req *SamplingRequest) AddTextMessage(role, text string) *SamplingRequest {
	message := CreateTextSamplingMessage(role, text)
	req.Messages = append(req.Messages, message)
	return req
}

// AddImageMessage adds an image message to the request
func (req *SamplingRequest) AddImageMessage(role, imageData, mimeType string) *SamplingRequest {
	message := CreateImageSamplingMessage(role, imageData, mimeType)
	req.Messages = append(req.Messages, message)
	return req
}

// AddAudioMessage adds an audio message to the request
func (req *SamplingRequest) AddAudioMessage(role, audioData, mimeType string) *SamplingRequest {
	message := CreateAudioSamplingMessage(role, audioData, mimeType)
	req.Messages = append(req.Messages, message)
	return req
}

// RequestSampling sends the sampling request to the server and returns the response
func (c *clientImpl) RequestSampling(req *SamplingRequest) (*SamplingResponse, error) {
	// Set protocol version if not already set
	if req.ProtocolVersion == "" {
		req.ProtocolVersion = c.negotiatedVersion
	}

	// Validate the request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid sampling request: %w", err)
	}

	// Build the request with a new request ID using the existing method
	requestID := c.generateRequestID()
	requestJSON, err := req.BuildCreateMessageRequest(int(requestID))
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	// Create a context with timeout if not already set
	ctx := req.Context
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), req.Timeout)
		defer cancel()
	}

	// Send the request with retry logic
	var responseJSON []byte
	var sendErr error

	for attempt := 0; attempt <= req.RetryConfig.MaxRetries; attempt++ {
		// Wait before retrying (except for the first attempt)
		if attempt > 0 {
			retryInterval := req.RetryConfig.RetryInterval * time.Duration(float64(attempt)*req.RetryConfig.RetryMultiplier)
			if retryInterval > req.RetryConfig.MaxInterval {
				retryInterval = req.RetryConfig.MaxInterval
			}

			c.logger.Debug("retrying sampling request", "attempt", attempt, "retryInterval", retryInterval)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryInterval):
				// Continue with retry
			}
		}

		// Send the request
		responseJSON, sendErr = c.transport.SendWithContext(ctx, requestJSON)

		// If successful or not retryable error, break
		if sendErr == nil || !isRetryableError(sendErr) {
			break
		}

		c.logger.Warn("retryable error in sampling request", "error", sendErr, "attempt", attempt+1, "maxRetries", req.RetryConfig.MaxRetries)
	}

	// Check for errors after all retries
	if sendErr != nil {
		return nil, fmt.Errorf("failed to send sampling request after %d retries: %w", req.RetryConfig.MaxRetries, sendErr)
	}

	// Parse the response
	response, err := parseSamplingResponse(responseJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sampling response: %w", err)
	}

	// Validate response against protocol version
	if !response.Content.IsValidForVersion(req.ProtocolVersion) {
		return nil, fmt.Errorf("response content type '%s' not supported in protocol version '%s'",
			response.Content.Type, req.ProtocolVersion)
	}

	return response, nil
}

// isRetryableError determines if an error should trigger a retry attempt
func isRetryableError(err error) bool {
	// Check for network errors that might be transient
	// This is a simplified check; in a real implementation, you might want to
	// check for specific error types or error messages

	// Check for known SamplingError types
	if samplingErr, ok := err.(*SamplingError); ok {
		return samplingErr.RetryRecommended
	}

	// Context cancellation or deadline exceeded errors should not be retried
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	// Check for network timeouts and connection issues
	errStr := err.Error()
	return contains(errStr, "timeout") ||
		contains(errStr, "temporary") ||
		contains(errStr, "connection") ||
		contains(errStr, "reset") ||
		contains(errStr, "broken pipe") ||
		contains(errStr, "i/o timeout")
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr
}

// CreateSamplingCreateMessageParams creates parameters for a sampling/createMessage request
func CreateSamplingCreateMessageParams(
	messages []SamplingMessage,
	prefs SamplingModelPreferences,
	systemPrompt string,
	maxTokens int,
	protocolVersion string,
) (SamplingCreateMessageParams, error) {
	// Validate based on protocol version
	for i, msg := range messages {
		if !msg.Content.IsValidForVersion(protocolVersion) {
			return SamplingCreateMessageParams{}, fmt.Errorf("message %d content type '%s' not supported in protocol version '%s'",
				i, msg.Content.Type, protocolVersion)
		}
	}

	// Create the parameters
	return SamplingCreateMessageParams{
		Messages:         messages,
		ModelPreferences: prefs,
		SystemPrompt:     systemPrompt,
		MaxTokens:        maxTokens,
	}, nil
}
