// Package test provides test utilities for the client package.
package test

import (
	"github.com/localrivet/gomcp/client"
)

// SetSamplingHandler is a helper function to set a sampling handler on a client
// This is needed because WithSamplingHandler is not part of the Client interface
func SetSamplingHandler(c client.Client, handler client.SamplingHandler) {
	// We need to access the client implementation which has the WithSamplingHandler method
	// This is done through the actual dispatch methods in client/sampling.go
	// Create a response that we know will be valid for the client's version
	resp := client.SamplingResponse{
		Role: "assistant",
		Content: client.SamplingMessageContent{
			Type: "text",
			Text: "This is a sampling response",
		},
	}

	// Register the handler by wrapping it
	client.RegisterSamplingHandler(c, func(params client.SamplingCreateMessageParams) (client.SamplingResponse, error) {
		if handler != nil {
			return handler(params)
		}
		return resp, nil
	})
}

// SamplingTestHelpers provides access to internal client package functions for testing
type SamplingTestHelpers struct{}

// ParseSamplingResponseForTest provides access to the parseSamplingResponse function for testing
func ParseSamplingResponseForTest(data []byte) (*client.SamplingResponse, error) {
	return client.ParseSamplingResponseForTest(data)
}

// ValidateSamplingResponseForVersionForTest provides access to validateSamplingResponseForVersion for testing
func ValidateSamplingResponseForVersionForTest(response *client.SamplingResponse, version string) error {
	return client.ValidateSamplingResponseForVersionForTest(response, version)
}

// ParseStreamingSamplingResponseForTest provides access to parseStreamingSamplingResponse for testing
func ParseStreamingSamplingResponseForTest(data []byte) (*client.StreamingSamplingResponse, error) {
	return client.ParseStreamingSamplingResponseForTest(data)
}

// ValidateStreamingSamplingResponseForVersionForTest provides access to validateStreamingSamplingResponseForVersion for testing
func ValidateStreamingSamplingResponseForVersionForTest(response *client.StreamingSamplingResponse, version string) error {
	return client.ValidateStreamingSamplingResponseForVersionForTest(response, version)
}

// IsStreamingSupportedForVersionForTest provides access to isStreamingSupportedForVersion for testing
func IsStreamingSupportedForVersionForTest(version string) bool {
	return client.IsStreamingSupportedForVersionForTest(version)
}

// NewMockSamplingResponse creates a mock sampling response for testing
func NewMockSamplingResponse(role string, contentType string, contentText string) *client.SamplingResponse {
	return &client.SamplingResponse{
		Role: role,
		Content: client.SamplingMessageContent{
			Type: contentType,
			Text: contentText,
		},
	}
}

// NewMockStreamingSamplingResponse creates a mock streaming sampling response for testing
func NewMockStreamingSamplingResponse(role string, contentType string, contentText string, isComplete bool) *client.StreamingSamplingResponse {
	return &client.StreamingSamplingResponse{
		Role: role,
		Content: client.SamplingMessageContent{
			Type: contentType,
			Text: contentText,
		},
		IsComplete: isComplete,
		ChunkID:    "test-chunk-1",
	}
}
