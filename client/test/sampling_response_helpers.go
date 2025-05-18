// Package test provides test utilities for the client package.
package test

import (
	"encoding/json"
	"fmt"

	"github.com/localrivet/gomcp/client/test/jsonrpc"
)

// CreateSamplingResponse generates a sampling response in the format appropriate for the specified version
func CreateSamplingResponse(id interface{}, version string, text string, options map[string]interface{}) []byte {
	var response *jsonrpc.JSONRPC

	// Create standard content
	content := jsonrpc.SamplingMessageContent{
		Type: "text",
		Text: text,
	}

	// Default role
	role := "assistant"

	// Extract optional parameters
	var model, stopReason string
	if options != nil {
		if m, ok := options["model"].(string); ok {
			model = m
		}
		if s, ok := options["stopReason"].(string); ok {
			stopReason = s
		}
		if r, ok := options["role"].(string); ok {
			role = r
		}
		// Handle content type overrides
		if contentType, ok := options["contentType"].(string); ok {
			content.Type = contentType
		}
		if contentData, ok := options["data"].(string); ok {
			content.Data = contentData
		}
		if mimeType, ok := options["mimeType"].(string); ok {
			content.MimeType = mimeType
		}
	}

	// All versions support the same basic response structure for non-streaming
	response = jsonrpc.NewSamplingResponse(id, role, content, model, stopReason)

	responseJSON, err := json.Marshal(response)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal sampling response: %v", err))
	}
	return responseJSON
}

// CreateSamplingTextResponse is a convenience function to create a text sampling response
func CreateSamplingTextResponse(id interface{}, version string, text string, model string) []byte {
	options := map[string]interface{}{
		"model": model,
	}
	return CreateSamplingResponse(id, version, text, options)
}

// CreateSamplingImageResponse creates an image sampling response
func CreateSamplingImageResponse(id interface{}, version string, data string, mimeType string, model string) []byte {
	options := map[string]interface{}{
		"contentType": "image",
		"data":        data,
		"mimeType":    mimeType,
		"model":       model,
	}
	return CreateSamplingResponse(id, version, "", options) // Empty text as we're using data
}

// CreateSamplingAudioResponse creates an audio sampling response
func CreateSamplingAudioResponse(id interface{}, version string, data string, mimeType string, model string) []byte {
	options := map[string]interface{}{
		"contentType": "audio",
		"data":        data,
		"mimeType":    mimeType,
		"model":       model,
	}
	return CreateSamplingResponse(id, version, "", options) // Empty text as we're using data
}

// CreateStreamingSamplingResponse generates a streaming response in the format appropriate for the specified version
// Note: streaming is only supported in v20241105 and v20250326, not in draft
func CreateStreamingSamplingResponse(id interface{}, version string, options map[string]interface{}) ([]byte, error) {
	// Validate version supports streaming
	if version == "draft" {
		return nil, fmt.Errorf("streaming not supported in draft version")
	}

	var response *jsonrpc.JSONRPC

	// Check if this is a chunk or completion
	if chunk, isChunk := options["chunk"].(bool); isChunk && chunk {
		// Create a chunk response
		chunkID := "chunk-1"
		text := ""
		role := "assistant"
		contentType := "text"

		if id, ok := options["chunkID"].(string); ok {
			chunkID = id
		}
		if t, ok := options["text"].(string); ok {
			text = t
		}
		if r, ok := options["role"].(string); ok {
			role = r
		}
		if ct, ok := options["contentType"].(string); ok {
			contentType = ct
		}

		content := jsonrpc.SamplingMessageContent{
			Type: contentType,
			Text: text,
		}

		// Handle other content types
		if data, ok := options["data"].(string); ok {
			content.Data = data
		}
		if mime, ok := options["mimeType"].(string); ok {
			content.MimeType = mime
		}

		response = jsonrpc.NewSamplingStreamingChunkResponse(id, chunkID, role, content)
	} else {
		// Create a completion response
		model := ""
		stopReason := "stop"

		if m, ok := options["model"].(string); ok {
			model = m
		}
		if sr, ok := options["stopReason"].(string); ok {
			stopReason = sr
		}

		response = jsonrpc.NewSamplingStreamingCompletionResponse(id, model, stopReason)
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal streaming response: %v", err)
	}
	return responseJSON, nil
}

// CreateStreamingSamplingTextChunk creates a streaming text chunk response
func CreateStreamingSamplingTextChunk(id interface{}, version string, chunkID string, text string) ([]byte, error) {
	options := map[string]interface{}{
		"chunk":   true,
		"chunkID": chunkID,
		"text":    text,
	}
	return CreateStreamingSamplingResponse(id, version, options)
}

// CreateStreamingSamplingCompletion creates a streaming completion response
func CreateStreamingSamplingCompletion(id interface{}, version string, model string, stopReason string) ([]byte, error) {
	options := map[string]interface{}{
		"chunk":      false,
		"model":      model,
		"stopReason": stopReason,
	}
	return CreateStreamingSamplingResponse(id, version, options)
}

// SetupSamplingMockTransport configures a mock transport with appropriate responses for sampling tests
func SetupSamplingMockTransport(version string, mockTransport *MockTransport) *MockTransport {
	// Ensure the mock transport is initialized
	if mockTransport == nil {
		mockTransport = NewMockTransport()
	}

	// Configure the mock transport with appropriate responses for the specified version
	switch version {
	case "draft":
		// Draft version doesn't support streaming
		mockTransport.WithDefaultResponse(
			CreateSamplingTextResponse(1, version, "This is a sampling response", "test-model"),
			nil,
		)
	case "2024-11-05", "v20241105":
		// 2024-11-05 supports basic streaming
		mockTransport.WithDefaultResponse(
			CreateSamplingTextResponse(1, version, "This is a sampling response", "test-model"),
			nil,
		)
	case "2025-03-26", "v20250326":
		// 2025-03-26 supports advanced streaming
		mockTransport.WithDefaultResponse(
			CreateSamplingTextResponse(1, version, "This is a sampling response", "test-model"),
			nil,
		)
	default:
		// Default to current version behavior
		mockTransport.WithDefaultResponse(
			CreateSamplingTextResponse(1, version, "This is a sampling response", "test-model"),
			nil,
		)
	}

	return mockTransport
}

// IsStreamingSupportedInVersion returns true if streaming is supported in the specified version
func IsStreamingSupportedInVersion(version string) bool {
	return version != "draft"
}

// SamplingCreateMessageMatcher creates a condition function that matches sampling/createMessage requests
func SamplingCreateMessageMatcher() func([]byte) bool {
	return func(request []byte) bool {
		var req map[string]interface{}
		if err := json.Unmarshal(request, &req); err != nil {
			return false
		}

		method, ok := req["method"].(string)
		return ok && method == "sampling/createMessage"
	}
}

// SamplingCreateStreamingMessageMatcher creates a condition function that matches sampling/createStreamingMessage requests
func SamplingCreateStreamingMessageMatcher() func([]byte) bool {
	return func(request []byte) bool {
		var req map[string]interface{}
		if err := json.Unmarshal(request, &req); err != nil {
			return false
		}

		method, ok := req["method"].(string)
		return ok && method == "sampling/createStreamingMessage"
	}
}
