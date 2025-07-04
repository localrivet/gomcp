// Package test provides test utilities for the client package.
package test

import (
	"github.com/localrivet/gomcp/client"
)

// SetSamplingHandler is a helper function to set a sampling handler on a client
func SetSamplingHandler(c client.Client, handler client.SamplingHandler) client.Client {
	// WithSamplingHandler is part of the Client interface
	return c.WithSamplingHandler(handler)
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

// CreateTextSamplingMessage creates a text sampling message for testing
func CreateTextSamplingMessage(role, text string) client.SamplingMessage {
	return client.CreateTextMessage(role, text)
}

// CreateImageSamplingMessage creates an image sampling message for testing
func CreateImageSamplingMessage(role, imageData, mimeType string) client.SamplingMessage {
	return client.CreateImageMessage(role, imageData, mimeType)
}

// CreateAudioSamplingMessage creates an audio sampling message for testing
func CreateAudioSamplingMessage(role, audioData, mimeType string) client.SamplingMessage {
	return client.CreateAudioMessage(role, audioData, mimeType)
}

// NewSamplingRequest creates a new sampling options for testing
func NewSamplingRequest(messages []client.SamplingMessage, prefs client.SamplingModelPreferences) *client.SamplingOptions {
	return client.NewSamplingOptions(messages, prefs)
}

// NewSamplingConfig creates a new sampling model preferences for testing
func NewSamplingConfig() client.SamplingModelPreferences {
	return client.SamplingModelPreferences{}
}

// CreateStreamingChatRequest creates a streaming sampling request for testing
func CreateStreamingChatRequest(messages []client.SamplingMessage, systemPrompt, version string) (*client.SamplingOptions, error) {
	opts := client.NewSamplingOptions(messages, client.SamplingModelPreferences{}).
		WithSystemPrompt(systemPrompt)
	opts.ProtocolVersion = version
	opts.Streaming = true
	return opts, nil
}

// NewStreamingSamplingRequest creates a new streaming sampling options for testing
func NewStreamingSamplingRequest(messages []client.SamplingMessage, prefs client.SamplingModelPreferences) *client.SamplingOptions {
	opts := client.NewSamplingOptions(messages, prefs)
	opts.Streaming = true
	return opts
}

// IsStreamingSupportedForVersion checks if streaming is supported for a version
func IsStreamingSupportedForVersion(version string) bool {
	// Streaming is supported in 2025-03-26, 2025-06-18, and draft
	return version == "2025-03-26" || version == "2025-06-18" || version == "draft"
}
