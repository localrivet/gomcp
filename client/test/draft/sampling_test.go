package draft

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/localrivet/gomcp/client"
	clienttest "github.com/localrivet/gomcp/client/test"
)

func TestDraftSamplingRequest(t *testing.T) {
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
	req.WithProtocolVersion("draft")
	if err := req.Validate(); err != nil {
		t.Errorf("validation failed for valid request with draft version: %v", err)
	}

	// Test content types for draft version
	textMsg := client.CreateTextSamplingMessage("user", "Hello, world!")
	textReq := client.NewSamplingRequest([]client.SamplingMessage{textMsg}, prefs)
	textReq.WithProtocolVersion("draft")
	if err := textReq.Validate(); err != nil {
		t.Errorf("validation failed for text content in draft: %v", err)
	}

	imageMsg := client.CreateImageSamplingMessage("user", "test-image-data", "image/png")
	imageReq := client.NewSamplingRequest([]client.SamplingMessage{imageMsg}, prefs)
	imageReq.WithProtocolVersion("draft")
	if err := imageReq.Validate(); err != nil {
		t.Errorf("validation failed for image content in draft: %v", err)
	}

	audioMsg := client.CreateAudioSamplingMessage("user", "test-audio-data", "audio/mp3")
	audioReq := client.NewSamplingRequest([]client.SamplingMessage{audioMsg}, prefs)
	audioReq.WithProtocolVersion("draft")
	if err := audioReq.Validate(); err != nil {
		t.Errorf("validation failed for audio content in draft: %v", err)
	}

	// Test request building
	req = client.NewSamplingRequest(messages, prefs)
	req.WithSystemPrompt("You are a test assistant")
	req.WithMaxTokens(100)
	req.WithProtocolVersion("draft")

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
	if request["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", request["jsonrpc"])
	}

	if request["id"] != float64(1) {
		t.Errorf("expected id 1, got %v", request["id"])
	}

	if request["method"] != "sampling/createMessage" {
		t.Errorf("expected method sampling/createMessage, got %v", request["method"])
	}

	params, ok := request["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected params object, got %T", request["params"])
	}

	if params["protocolVersion"] != "draft" {
		t.Errorf("expected protocolVersion draft, got %v", params["protocolVersion"])
	}

	// Test streaming is not supported in draft version
	if clienttest.IsStreamingSupportedForVersionForTest("draft") {
		t.Error("streaming should not be supported in draft version")
	}

	// Attempt to create a streaming request (should fail)
	streamingReq := client.NewStreamingSamplingRequest(messages, prefs)
	streamingReq.WithProtocolVersion("draft")
	_, streamingErr := mockStreamingSamplingRequest(streamingReq)
	if streamingErr == nil {
		t.Error("streaming request should fail for draft version")
	}
}

func TestDraftSamplingResponseParsing(t *testing.T) {
	// Test successful response parsing for draft version
	successJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"role": "assistant",
			"content": {
				"type": "text",
				"text": "Hello, I'm a test assistant."
			},
			"model": "test-model"
		}
	}`

	response, err := clienttest.ParseSamplingResponseForTest([]byte(successJSON))
	if err != nil {
		t.Fatalf("failed to parse valid response: %v", err)
	}

	if response.Role != "assistant" {
		t.Errorf("expected role assistant, got %s", response.Role)
	}

	if response.Content.Type != "text" {
		t.Errorf("expected content type text, got %s", response.Content.Type)
	}

	if response.Content.Text != "Hello, I'm a test assistant." {
		t.Errorf("expected text 'Hello, I'm a test assistant.', got %s", response.Content.Text)
	}

	if response.Model != "test-model" {
		t.Errorf("expected model test-model, got %s", response.Model)
	}

	// Test error response parsing
	errorJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"error": {
			"code": -32600,
			"message": "Invalid Request",
			"data": "Invalid parameter format"
		}
	}`

	_, err = clienttest.ParseSamplingResponseForTest([]byte(errorJSON))
	if err == nil {
		t.Fatal("expected error for error response, got nil")
	}

	// Test validation of response against protocol version
	imageResponseJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"role": "assistant",
			"content": {
				"type": "image",
				"data": "base64-encoded-image-data",
				"mimeType": "image/png"
			}
		}
	}`

	imageResponse, err := clienttest.ParseSamplingResponseForTest([]byte(imageResponseJSON))
	if err != nil {
		t.Fatalf("failed to parse valid image response: %v", err)
	}

	// Validate against draft version
	err = clienttest.ValidateSamplingResponseForVersionForTest(imageResponse, "draft")
	if err != nil {
		t.Errorf("image response validation failed for draft version: %v", err)
	}

	// Test audio response (supported in draft)
	audioResponseJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"role": "assistant",
			"content": {
				"type": "audio",
				"data": "base64-encoded-audio-data",
				"mimeType": "audio/mp3"
			}
		}
	}`

	audioResponse, err := clienttest.ParseSamplingResponseForTest([]byte(audioResponseJSON))
	if err != nil {
		t.Fatalf("failed to parse valid audio response: %v", err)
	}

	// Validate against draft version
	err = clienttest.ValidateSamplingResponseForVersionForTest(audioResponse, "draft")
	if err != nil {
		t.Errorf("audio response validation failed for draft version: %v", err)
	}
}

// Mock function to test streaming requests (exported from client package)
func mockStreamingSamplingRequest(req *client.StreamingSamplingRequest) (*client.StreamingSamplingSession, error) {
	if !clienttest.IsStreamingSupportedForVersionForTest(req.ProtocolVersion) {
		return nil, fmt.Errorf("streaming not supported in protocol version %s", req.ProtocolVersion)
	}
	return nil, nil
}
