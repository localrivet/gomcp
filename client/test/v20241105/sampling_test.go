package v20241105

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/localrivet/gomcp/client"
	clienttest "github.com/localrivet/gomcp/client/test"
)

func Test2024_11_05SamplingRequest(t *testing.T) {
	// Create a simple sampling request
	messages := []client.SamplingMessage{
		client.CreateTextSamplingMessage("user", "Hello, world!"),
	}

	prefs := client.SamplingModelPreferences{
		Hints: []client.SamplingModelHint{
			{Name: "test-model"},
		},
	}

	req := client.NewSamplingRequest(messages, prefs)
	req.WithSystemPrompt("You are a test assistant")
	req.WithMaxTokens(100)

	// Test validation
	if err := req.Validate(); err != nil {
		t.Errorf("validation failed for valid request: %v", err)
	}

	// Test version validation
	req.WithProtocolVersion("2024-11-05")
	if err := req.Validate(); err != nil {
		t.Errorf("validation failed for valid request with 2024-11-05 version: %v", err)
	}

	// Test content types for 2024-11-05 version
	// Text is supported
	textMsg := client.CreateTextSamplingMessage("user", "Hello, world!")
	textReq := client.NewSamplingRequest([]client.SamplingMessage{textMsg}, prefs)
	textReq.WithProtocolVersion("2024-11-05")
	if err := textReq.Validate(); err != nil {
		t.Errorf("validation failed for text content in 2024-11-05: %v", err)
	}

	// Image is supported
	imageMsg := client.CreateImageSamplingMessage("user", "test-image-data", "image/png")
	imageReq := client.NewSamplingRequest([]client.SamplingMessage{imageMsg}, prefs)
	imageReq.WithProtocolVersion("2024-11-05")
	if err := imageReq.Validate(); err != nil {
		t.Errorf("validation failed for image content in 2024-11-05: %v", err)
	}

	// Audio is NOT supported in 2024-11-05
	audioMsg := client.CreateAudioSamplingMessage("user", "test-audio-data", "audio/mp3")
	audioReq := client.NewSamplingRequest([]client.SamplingMessage{audioMsg}, prefs)
	audioReq.WithProtocolVersion("2024-11-05")
	if err := audioReq.Validate(); err == nil {
		t.Error("validation should fail for audio content in 2024-11-05")
	} else {
		t.Logf("Expected validation error for audio in 2024-11-05: %v", err)
	}

	// Test max tokens validation
	// Max tokens is limited to 4096 in 2024-11-05
	maxTokensReq := client.NewSamplingRequest(messages, prefs)
	maxTokensReq.WithProtocolVersion("2024-11-05")
	maxTokensReq.WithMaxTokens(4096) // Maximum allowed
	if err := maxTokensReq.Validate(); err != nil {
		t.Errorf("validation failed for valid max tokens in 2024-11-05: %v", err)
	}

	// Test exceeding max tokens
	maxTokensReq.WithMaxTokens(4097) // Exceeds maximum
	if err := maxTokensReq.Validate(); err == nil {
		t.Error("validation should fail for max tokens > 4096 in 2024-11-05")
	} else {
		t.Logf("Expected validation error for max tokens > 4096 in 2024-11-05: %v", err)
	}

	// Test request building
	req = client.NewSamplingRequest(messages, prefs)
	req.WithSystemPrompt("You are a test assistant")
	req.WithMaxTokens(100)
	req.WithProtocolVersion("2024-11-05")

	requestJSON, err := req.BuildCreateMessageRequest(1)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	// Parse and verify the request
	var request map[string]interface{}
	if err := json.Unmarshal(requestJSON, &request); err != nil {
		t.Fatalf("failed to parse request JSON: %v", err)
	}

	// Verify the request structure
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected params object, got %T", request["params"])
	}

	if params["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocolVersion 2024-11-05, got %v", params["protocolVersion"])
	}

	// Test streaming is not supported in 2024-11-05 version
	if clienttest.IsStreamingSupportedForVersionForTest("2024-11-05") {
		t.Error("streaming should not be supported in 2024-11-05 version")
	}

	// Attempt to create a streaming request (should fail)
	streamingReq := client.NewStreamingSamplingRequest(messages, prefs)
	streamingReq.WithProtocolVersion("2024-11-05")
	_, streamingErr := mockStreamingSamplingRequest(streamingReq)
	if streamingErr == nil {
		t.Error("streaming request should fail for 2024-11-05 version")
	}
}

func Test2024_11_05SamplingResponseParsing(t *testing.T) {
	// Test successful response parsing for 2024-11-05 version
	successJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"role": "assistant",
			"content": {
				"type": "text",
				"text": "Hello, I'm a test assistant."
			},
			"model": "test-model",
			"stopReason": "endTurn"
		}
	}`

	response, err := clienttest.ParseSamplingResponseForTest([]byte(successJSON))
	if err != nil {
		t.Fatalf("failed to parse valid response: %v", err)
	}

	// Validate against 2024-11-05 version
	err = clienttest.ValidateSamplingResponseForVersionForTest(response, "2024-11-05")
	if err != nil {
		t.Errorf("text response validation failed for 2024-11-05 version: %v", err)
	}

	// Test image response (supported in 2024-11-05)
	imageResponseJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"role": "assistant",
			"content": {
				"type": "image",
				"data": "base64-encoded-image-data",
				"mimeType": "image/png"
			},
			"model": "test-model"
		}
	}`

	imageResponse, err := clienttest.ParseSamplingResponseForTest([]byte(imageResponseJSON))
	if err != nil {
		t.Fatalf("failed to parse valid image response: %v", err)
	}

	// Validate against 2024-11-05 version
	err = clienttest.ValidateSamplingResponseForVersionForTest(imageResponse, "2024-11-05")
	if err != nil {
		t.Errorf("image response validation failed for 2024-11-05 version: %v", err)
	}

	// Test audio response (NOT supported in 2024-11-05)
	audioResponseJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"role": "assistant",
			"content": {
				"type": "audio",
				"data": "base64-encoded-audio-data",
				"mimeType": "audio/mp3"
			},
			"model": "test-model"
		}
	}`

	audioResponse, err := clienttest.ParseSamplingResponseForTest([]byte(audioResponseJSON))
	if err != nil {
		t.Fatalf("failed to parse valid audio response: %v", err)
	}

	// Validate against 2024-11-05 version (should fail)
	err = clienttest.ValidateSamplingResponseForVersionForTest(audioResponse, "2024-11-05")
	if err == nil {
		t.Error("audio response validation should fail for 2024-11-05 version")
	} else {
		t.Logf("Expected validation error for audio in 2024-11-05: %v", err)
	}

	// Test model identifier length validation (limited to 100 chars in 2024-11-05)
	longModelResponse := clienttest.NewMockSamplingResponse("assistant", "text", "Hello")
	longModelResponse.Model = "x" + longModelString(99) // Total 100 chars (valid)

	err = clienttest.ValidateSamplingResponseForVersionForTest(longModelResponse, "2024-11-05")
	if err != nil {
		t.Errorf("valid model length validation failed for 2024-11-05: %v", err)
	}

	// Test with too long model identifier
	longModelResponse.Model = "x" + longModelString(100) // Total 101 chars (invalid)
	err = clienttest.ValidateSamplingResponseForVersionForTest(longModelResponse, "2024-11-05")
	if err == nil {
		t.Error("validation should fail for model identifier > 100 chars in 2024-11-05")
	} else {
		t.Logf("Expected validation error for long model identifier in 2024-11-05: %v", err)
	}
}

// Helper function to create a long model string for testing
func longModelString(length int) string {
	result := ""
	for i := 0; i < length; i++ {
		result += "m"
	}
	return result
}

// Mock function to test streaming requests
func mockStreamingSamplingRequest(req *client.StreamingSamplingRequest) (*client.StreamingSamplingSession, error) {
	if !clienttest.IsStreamingSupportedForVersionForTest(req.ProtocolVersion) {
		return nil, fmt.Errorf("streaming not supported in protocol version %s", req.ProtocolVersion)
	}
	return nil, nil
}
