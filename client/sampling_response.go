// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"encoding/json"
	"fmt"
)

// SamplingResponseError represents an error in a sampling response
type SamplingResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// Error returns the error message
func (e *SamplingResponseError) Error() string {
	if e.Data != "" {
		return fmt.Sprintf("%s (code %d): %s", e.Message, e.Code, e.Data)
	}
	return fmt.Sprintf("%s (code %d)", e.Message, e.Code)
}

// parseSamplingResponse parses a JSON-RPC response to a sampling request
func parseSamplingResponse(responseJSON []byte) (*SamplingResponse, error) {
	// Parse the JSON-RPC response envelope
	var response map[string]json.RawMessage
	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return nil, NewSamplingError("parsing", "invalid JSON response", false, err)
	}

	// Check for JSON-RPC error
	if errorJSON, ok := response["error"]; ok {
		var rpcError SamplingResponseError
		if err := json.Unmarshal(errorJSON, &rpcError); err != nil {
			return nil, NewSamplingError("parsing", "invalid error response", false, err)
		}
		return nil, &rpcError
	}

	// Parse the result
	resultJSON, ok := response["result"]
	if !ok {
		return nil, NewSamplingError("validation", "missing result in response", false, nil)
	}

	// Parse the sampling response
	var samplingResponse SamplingResponse
	if err := json.Unmarshal(resultJSON, &samplingResponse); err != nil {
		return nil, NewSamplingError("parsing", "invalid sampling response", false, err)
	}

	// Validate the response
	if err := validateSamplingResponse(&samplingResponse); err != nil {
		return nil, err
	}

	return &samplingResponse, nil
}

// validateSamplingResponse validates a sampling response
func validateSamplingResponse(response *SamplingResponse) error {
	if response.Role == "" {
		return NewSamplingError("validation", "missing role in sampling response", false, nil)
	}

	// Validate the content based on its type
	content := response.Content
	if content.Type == "" {
		return NewSamplingError("validation", "missing content type in sampling response", false, nil)
	}

	switch content.Type {
	case "text":
		if content.Text == "" {
			return NewSamplingError("validation", "missing text in text content", false, nil)
		}
	case "image":
		if content.Data == "" {
			return NewSamplingError("validation", "missing data in image content", false, nil)
		}
		if content.MimeType == "" {
			return NewSamplingError("validation", "missing mimeType in image content", false, nil)
		}
	case "audio":
		if content.Data == "" {
			return NewSamplingError("validation", "missing data in audio content", false, nil)
		}
		if content.MimeType == "" {
			return NewSamplingError("validation", "missing mimeType in audio content", false, nil)
		}
	default:
		return NewSamplingError("validation", fmt.Sprintf("unknown content type: %s", content.Type), false, nil)
	}

	return nil
}

// validateSamplingResponseForVersion validates a sampling response against a specific protocol version
func validateSamplingResponseForVersion(response *SamplingResponse, version string) error {
	// First perform basic validation
	if err := validateSamplingResponse(response); err != nil {
		return err
	}

	// Then validate against the specific protocol version
	if !response.Content.IsValidForVersion(version) {
		return NewSamplingError(
			"validation",
			fmt.Sprintf("response content type '%s' not supported in protocol version '%s'",
				response.Content.Type, version),
			false,
			nil,
		)
	}

	// Protocol-specific validations
	switch version {
	case "draft":
		// No additional validations for draft
	case "2024-11-05":
		// 2024-11-05 does not support audio content
		if response.Content.Type == "audio" {
			return NewSamplingError(
				"validation",
				"audio content type not supported in protocol version 2024-11-05",
				false,
				nil,
			)
		}
		// Validate model field based on protocol
		if response.Model != "" && len(response.Model) > 100 {
			return NewSamplingError(
				"validation",
				"model identifier exceeds maximum length for protocol version 2024-11-05",
				false,
				nil,
			)
		}
	case "2025-03-26":
		// 2025-03-26 supports all content types
		// Validate model field based on protocol
		if response.Model != "" && len(response.Model) > 200 {
			return NewSamplingError(
				"validation",
				"model identifier exceeds maximum length for protocol version 2025-03-26",
				false,
				nil,
			)
		}
	default:
		return NewSamplingError(
			"validation",
			fmt.Sprintf("unknown protocol version: %s", version),
			false,
			nil,
		)
	}

	return nil
}

// StreamingSamplingResponse represents a streaming response from a sampling operation
type StreamingSamplingResponse struct {
	// Base response fields
	Role       string                 `json:"role"`
	Content    SamplingMessageContent `json:"content"`
	Model      string                 `json:"model,omitempty"`
	StopReason string                 `json:"stopReason,omitempty"`

	// Streaming-specific fields
	IsComplete bool   `json:"isComplete"`
	ChunkID    string `json:"chunkId,omitempty"`
}

// parseStreamingSamplingResponse parses a JSON-RPC response to a streaming sampling request
func parseStreamingSamplingResponse(responseJSON []byte) (*StreamingSamplingResponse, error) {
	// Parse the JSON-RPC response envelope
	var response map[string]json.RawMessage
	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return nil, NewSamplingError("parsing", "invalid JSON response", false, err)
	}

	// Check for JSON-RPC error
	if errorJSON, ok := response["error"]; ok {
		var rpcError SamplingResponseError
		if err := json.Unmarshal(errorJSON, &rpcError); err != nil {
			return nil, NewSamplingError("parsing", "invalid error response", false, err)
		}
		return nil, &rpcError
	}

	// Parse the result
	resultJSON, ok := response["result"]
	if !ok {
		return nil, NewSamplingError("validation", "missing result in response", false, nil)
	}

	// Parse the streaming sampling response
	var streamingResponse StreamingSamplingResponse
	if err := json.Unmarshal(resultJSON, &streamingResponse); err != nil {
		return nil, NewSamplingError("parsing", "invalid streaming response", false, err)
	}

	// Validate the response - simplified for streaming
	if streamingResponse.Role == "" {
		return nil, NewSamplingError("validation", "missing role in streaming response", false, nil)
	}

	// For streaming, text may be empty in intermediate chunks
	if streamingResponse.Content.Type == "" {
		return nil, NewSamplingError("validation", "missing content type in streaming response", false, nil)
	}

	return &streamingResponse, nil
}

// validateStreamingSamplingResponseForVersion validates a streaming response against a specific protocol version
func validateStreamingSamplingResponseForVersion(response *StreamingSamplingResponse, version string) error {
	// Validate against the specific protocol version
	if !response.Content.IsValidForVersion(version) {
		return NewSamplingError(
			"validation",
			fmt.Sprintf("streaming response content type '%s' not supported in protocol version '%s'",
				response.Content.Type, version),
			false,
			nil,
		)
	}

	// Protocol-specific validations
	switch version {
	case "draft":
		// Streaming is not explicitly specified in draft version
		return NewSamplingError(
			"validation",
			"streaming responses are not fully supported in protocol version draft",
			false,
			nil,
		)
	case "2024-11-05":
		// Streaming is not supported in 2024-11-05
		return NewSamplingError(
			"validation",
			"streaming responses are not supported in protocol version 2024-11-05",
			false,
			nil,
		)
	case "2025-03-26":
		// 2025-03-26 supports streaming responses
		// Ensure required fields for streaming are present
		if response.ChunkID == "" {
			return NewSamplingError(
				"validation",
				"missing chunkId in streaming response for protocol version 2025-03-26",
				false,
				nil,
			)
		}
	default:
		return NewSamplingError(
			"validation",
			fmt.Sprintf("unknown protocol version: %s", version),
			false,
			nil,
		)
	}

	return nil
}

// StreamingResponseHandler is a function that handles streaming responses
type StreamingResponseHandler func(response *StreamingSamplingResponse) error

// SamplingError represents a sampling-specific error
type SamplingError struct {
	ErrorType        string // Category of error (network, parsing, validation, server)
	Message          string
	RetryRecommended bool
	OriginalError    error
}

// Error returns the error message
func (e *SamplingError) Error() string {
	if e.OriginalError != nil {
		return fmt.Sprintf("%s: %s: %v", e.ErrorType, e.Message, e.OriginalError)
	}
	return fmt.Sprintf("%s: %s", e.ErrorType, e.Message)
}

// NewSamplingError creates a new sampling error
func NewSamplingError(errorType, message string, retry bool, originalErr error) *SamplingError {
	return &SamplingError{
		ErrorType:        errorType,
		Message:          message,
		RetryRecommended: retry,
		OriginalError:    originalErr,
	}
}

// IsRetryable returns whether the error is retryable
func (e *SamplingError) IsRetryable() bool {
	return e.RetryRecommended
}

// GetContent returns the text content of a sampling response
func (r *SamplingResponse) GetContent() string {
	if r.Content.Type == "text" {
		return r.Content.Text
	}
	return ""
}

// GetContentType returns the content type of a sampling response
func (r *SamplingResponse) GetContentType() string {
	return r.Content.Type
}

// GetImageData returns the image data and MIME type if available
func (r *SamplingResponse) GetImageData() (string, string, error) {
	if r.Content.Type != "image" {
		return "", "", fmt.Errorf("content is not an image")
	}
	return r.Content.Data, r.Content.MimeType, nil
}

// GetAudioData returns the audio data and MIME type if available
func (r *SamplingResponse) GetAudioData() (string, string, error) {
	if r.Content.Type != "audio" {
		return "", "", fmt.Errorf("content is not audio")
	}
	return r.Content.Data, r.Content.MimeType, nil
}

// IsProtocolVersionSupported checks if a given protocol version is supported for sampling
func IsProtocolVersionSupported(version string) bool {
	switch version {
	case "draft", "2024-11-05", "2025-03-26":
		return true
	default:
		return false
	}
}
