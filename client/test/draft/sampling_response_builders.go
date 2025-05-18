// Package draft provides test utilities specific to the draft protocol version.
package draft

import (
	"encoding/json"

	"github.com/localrivet/gomcp/client/test"
	"github.com/localrivet/gomcp/client/test/jsonrpc"
)

// The draft protocol version
const Version = "draft"

// CreateTextResponse creates a standard text response for the draft protocol version
func CreateTextResponse(id interface{}, text string, model string) []byte {
	return test.CreateSamplingTextResponse(id, Version, text, model)
}

// CreateImageResponse creates an image response for the draft protocol version
func CreateImageResponse(id interface{}, data string, mimeType string, model string) []byte {
	return test.CreateSamplingImageResponse(id, Version, data, mimeType, model)
}

// CreateAudioResponse creates an audio response for the draft protocol version
func CreateAudioResponse(id interface{}, data string, mimeType string, model string) []byte {
	return test.CreateSamplingAudioResponse(id, Version, data, mimeType, model)
}

// CreateResponseWithOptions creates a custom response with specified options
func CreateResponseWithOptions(id interface{}, text string, options map[string]interface{}) []byte {
	return test.CreateSamplingResponse(id, Version, text, options)
}

// SetupMockTransport configures a mock transport for draft version sampling tests
func SetupMockTransport() *test.MockTransport {
	return test.SetupSamplingMockTransport(Version, nil)
}

// IsStreamingSupported returns false as streaming is not supported in draft version
func IsStreamingSupported() bool {
	return false
}

// CreateRequestMatcher creates a matcher function for sampling requests
func CreateRequestMatcher() func([]byte) bool {
	return test.SamplingCreateMessageMatcher()
}

// CreateCreateMessageRequest creates a sampling/createMessage request for draft protocol
func CreateCreateMessageRequest(id interface{}, messages []jsonrpc.SamplingMessage, modelPreferences jsonrpc.SamplingModelPreferences) []byte {
	params := jsonrpc.NewSamplingCreateMessageParams(messages, modelPreferences)
	req := jsonrpc.NewSamplingCreateMessageRequest(id, params)
	jsonData, _ := json.Marshal(req)
	return jsonData
}

// CreateTextMessage creates a text sampling message
func CreateTextMessage(role, text string) jsonrpc.SamplingMessage {
	return jsonrpc.SamplingMessage{
		Role: role,
		Content: jsonrpc.SamplingMessageContent{
			Type: "text",
			Text: text,
		},
	}
}

// CreateImageMessage creates an image sampling message
func CreateImageMessage(role, data, mimeType string) jsonrpc.SamplingMessage {
	return jsonrpc.SamplingMessage{
		Role: role,
		Content: jsonrpc.SamplingMessageContent{
			Type:     "image",
			Data:     data,
			MimeType: mimeType,
		},
	}
}

// CreateAudioMessage creates an audio sampling message
func CreateAudioMessage(role, data, mimeType string) jsonrpc.SamplingMessage {
	return jsonrpc.SamplingMessage{
		Role: role,
		Content: jsonrpc.SamplingMessageContent{
			Type:     "audio",
			Data:     data,
			MimeType: mimeType,
		},
	}
}

// CreateModelPreferences creates sampling model preferences
func CreateModelPreferences(modelNames ...string) jsonrpc.SamplingModelPreferences {
	hints := make([]jsonrpc.SamplingModelHint, 0, len(modelNames))
	for _, name := range modelNames {
		hints = append(hints, jsonrpc.SamplingModelHint{Name: name})
	}
	return jsonrpc.SamplingModelPreferences{
		Hints: hints,
	}
}
