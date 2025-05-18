package v20250326

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/client"
	clienttest "github.com/localrivet/gomcp/client/test"
)

func Test2025_03_26SamplingRequest(t *testing.T) {
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
	req.WithProtocolVersion("2025-03-26")
	if err := req.Validate(); err != nil {
		t.Errorf("validation failed for valid request with 2025-03-26 version: %v", err)
	}

	// Test content types for 2025-03-26 version
	// Text is supported
	textMsg := client.CreateTextSamplingMessage("user", "Hello, world!")
	textReq := client.NewSamplingRequest([]client.SamplingMessage{textMsg}, prefs)
	textReq.WithProtocolVersion("2025-03-26")
	if err := textReq.Validate(); err != nil {
		t.Errorf("validation failed for text content in 2025-03-26: %v", err)
	}

	// Image is supported
	imageMsg := client.CreateImageSamplingMessage("user", "test-image-data", "image/png")
	imageReq := client.NewSamplingRequest([]client.SamplingMessage{imageMsg}, prefs)
	imageReq.WithProtocolVersion("2025-03-26")
	if err := imageReq.Validate(); err != nil {
		t.Errorf("validation failed for image content in 2025-03-26: %v", err)
	}

	// Audio is supported in 2025-03-26
	audioMsg := client.CreateAudioSamplingMessage("user", "test-audio-data", "audio/mp3")
	audioReq := client.NewSamplingRequest([]client.SamplingMessage{audioMsg}, prefs)
	audioReq.WithProtocolVersion("2025-03-26")
	if err := audioReq.Validate(); err != nil {
		t.Errorf("validation failed for audio content in 2025-03-26: %v", err)
	}

	// Test max tokens validation
	// Max tokens is limited to 8192 in 2025-03-26
	maxTokensReq := client.NewSamplingRequest(messages, prefs)
	maxTokensReq.WithProtocolVersion("2025-03-26")
	maxTokensReq.WithMaxTokens(8192) // Maximum allowed
	if err := maxTokensReq.Validate(); err != nil {
		t.Errorf("validation failed for valid max tokens in 2025-03-26: %v", err)
	}

	// Test exceeding max tokens
	maxTokensReq.WithMaxTokens(8193) // Exceeds maximum
	if err := maxTokensReq.Validate(); err == nil {
		t.Error("validation should fail for max tokens > 8192 in 2025-03-26")
	} else {
		t.Logf("Expected validation error for max tokens > 8192 in 2025-03-26: %v", err)
	}

	// Test system prompt length validation
	// System prompt is limited to 16000 chars in 2025-03-26
	longPromptReq := client.NewSamplingRequest(messages, prefs)
	longPromptReq.WithProtocolVersion("2025-03-26")

	// Generate a system prompt just at the limit
	longPrompt := ""
	for i := 0; i < 16000; i++ {
		longPrompt += "x"
	}
	longPromptReq.WithSystemPrompt(longPrompt)

	if err := longPromptReq.Validate(); err != nil {
		t.Errorf("validation failed for valid system prompt length in 2025-03-26: %v", err)
	}

	// Test exceeding system prompt length
	tooLongPrompt := longPrompt + "x" // Exceeds maximum
	longPromptReq.WithSystemPrompt(tooLongPrompt)
	if err := longPromptReq.Validate(); err == nil {
		t.Error("validation should fail for system prompt > 16000 chars in 2025-03-26")
	} else {
		t.Logf("Expected validation error for long system prompt in 2025-03-26: %v", err)
	}

	// Test request building
	req = client.NewSamplingRequest(messages, prefs)
	req.WithSystemPrompt("You are a test assistant")
	req.WithMaxTokens(100)
	req.WithProtocolVersion("2025-03-26")

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

	if params["protocolVersion"] != "2025-03-26" {
		t.Errorf("expected protocolVersion 2025-03-26, got %v", params["protocolVersion"])
	}
}

func Test2025_03_26SamplingResponseParsing(t *testing.T) {
	// Test successful response parsing for 2025-03-26 version
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

	// Validate against 2025-03-26 version
	err = clienttest.ValidateSamplingResponseForVersionForTest(response, "2025-03-26")
	if err != nil {
		t.Errorf("text response validation failed for 2025-03-26 version: %v", err)
	}

	// Test image response (supported in 2025-03-26)
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

	// Validate against 2025-03-26 version
	err = clienttest.ValidateSamplingResponseForVersionForTest(imageResponse, "2025-03-26")
	if err != nil {
		t.Errorf("image response validation failed for 2025-03-26 version: %v", err)
	}

	// Test audio response (supported in 2025-03-26)
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

	// Validate against 2025-03-26 version
	err = clienttest.ValidateSamplingResponseForVersionForTest(audioResponse, "2025-03-26")
	if err != nil {
		t.Errorf("audio response validation failed for 2025-03-26 version: %v", err)
	}

	// Test model identifier length validation (limited to 200 chars in 2025-03-26)
	longModelResponse := clienttest.NewMockSamplingResponse("assistant", "text", "Hello")
	longModelResponse.Model = longModelString(200) // Maximum length allowed

	err = clienttest.ValidateSamplingResponseForVersionForTest(longModelResponse, "2025-03-26")
	if err != nil {
		t.Errorf("valid model length validation failed for 2025-03-26: %v", err)
	}

	// Test with too long model identifier
	longModelResponse.Model = longModelString(201) // Exceeds maximum
	err = clienttest.ValidateSamplingResponseForVersionForTest(longModelResponse, "2025-03-26")
	if err == nil {
		t.Error("validation should fail for model identifier > 200 chars in 2025-03-26")
	} else {
		t.Logf("Expected validation error for long model identifier in 2025-03-26: %v", err)
	}
}

func Test2025_03_26StreamingSamplingRequest(t *testing.T) {
	// Check that streaming is supported in 2025-03-26
	if !clienttest.IsStreamingSupportedForVersionForTest("2025-03-26") {
		t.Error("streaming should be supported in 2025-03-26 version")
	}

	// Create a streaming sampling request
	messages := []client.SamplingMessage{
		client.CreateTextSamplingMessage("user", "Hello, world!"),
	}

	prefs := client.SamplingModelPreferences{
		Hints: []client.SamplingModelHint{
			{Name: "test-model"},
		},
	}

	req := client.NewStreamingSamplingRequest(messages, prefs)
	req.WithSystemPrompt("You are a test assistant")
	req.WithMaxTokens(100)
	req.WithProtocolVersion("2025-03-26")
	req.WithChunkSize(20)
	req.WithMaxChunks(5)
	req.WithStopOnComplete(true)

	// Build the request
	requestJSON, err := req.BuildStreamingCreateMessageRequest(1)
	if err != nil {
		t.Fatalf("failed to build streaming request: %v", err)
	}

	// Parse and verify the request
	var request map[string]interface{}
	if err := json.Unmarshal(requestJSON, &request); err != nil {
		t.Fatalf("failed to parse streaming request JSON: %v", err)
	}

	// Verify the request structure
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected params object, got %T", request["params"])
	}

	if params["streaming"] != true {
		t.Error("expected streaming to be true")
	}

	if params["chunkSize"] != float64(20) {
		t.Errorf("expected chunkSize 20, got %v", params["chunkSize"])
	}

	if params["protocolVersion"] != "2025-03-26" {
		t.Errorf("expected protocolVersion 2025-03-26, got %v", params["protocolVersion"])
	}

	// Test chunk size validation
	// Chunk size has min/max limits in 2025-03-26

	// Test with valid chunk size (minimum)
	minChunkReq := client.NewStreamingSamplingRequest(messages, prefs)
	minChunkReq.WithProtocolVersion("2025-03-26")
	minChunkReq.WithChunkSize(10) // Minimum allowed
	_, err = minChunkReq.BuildStreamingCreateMessageRequest(1)
	if err != nil {
		t.Errorf("failed to build streaming request with min chunk size: %v", err)
	}

	// Test with invalid chunk size (too small)
	tooSmallChunkReq := client.NewStreamingSamplingRequest(messages, prefs)
	tooSmallChunkReq.WithProtocolVersion("2025-03-26")
	tooSmallChunkReq.WithChunkSize(9) // Below minimum
	// We can't validate this with BuildStreamingCreateMessageRequest directly
	// Testing this would require actual transport call which is out of scope for unit tests

	// Test with valid chunk size (maximum)
	maxChunkReq := client.NewStreamingSamplingRequest(messages, prefs)
	maxChunkReq.WithProtocolVersion("2025-03-26")
	maxChunkReq.WithChunkSize(1000) // Maximum allowed
	_, err = maxChunkReq.BuildStreamingCreateMessageRequest(1)
	if err != nil {
		t.Errorf("failed to build streaming request with max chunk size: %v", err)
	}

	// Test with invalid chunk size (too large)
	tooLargeChunkReq := client.NewStreamingSamplingRequest(messages, prefs)
	tooLargeChunkReq.WithProtocolVersion("2025-03-26")
	tooLargeChunkReq.WithChunkSize(1001) // Above maximum
	// We can't validate this with BuildStreamingCreateMessageRequest directly
	// Testing this would require actual transport call which is out of scope for unit tests
}

func Test2025_03_26StreamingSamplingResponseParsing(t *testing.T) {
	// Test successful streaming response parsing for 2025-03-26 version
	streamingJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"role": "assistant",
			"content": {
				"type": "text",
				"text": "Hello,"
			},
			"model": "test-model",
			"isComplete": false,
			"chunkId": "chunk-1"
		}
	}`

	response, err := clienttest.ParseStreamingSamplingResponseForTest([]byte(streamingJSON))
	if err != nil {
		t.Fatalf("failed to parse valid streaming response: %v", err)
	}

	// Validate against 2025-03-26 version
	err = clienttest.ValidateStreamingSamplingResponseForVersionForTest(response, "2025-03-26")
	if err != nil {
		t.Errorf("streaming response validation failed for 2025-03-26 version: %v", err)
	}

	// Check required fields for streaming
	if response.ChunkID != "chunk-1" {
		t.Errorf("expected chunkId to be 'chunk-1', got %s", response.ChunkID)
	}

	if response.IsComplete {
		t.Error("expected isComplete to be false")
	}

	// Test incomplete chunk
	incompleteJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"role": "assistant",
			"content": {
				"type": "text",
				"text": "Hello, "
			},
			"model": "test-model",
			"isComplete": false,
			"chunkId": "chunk-1"
		}
	}`

	incompleteResponse, err := clienttest.ParseStreamingSamplingResponseForTest([]byte(incompleteJSON))
	if err != nil {
		t.Fatalf("failed to parse valid incomplete streaming response: %v", err)
	}

	if incompleteResponse.IsComplete {
		t.Error("expected isComplete to be false for incomplete chunk")
	}

	// Test complete chunk
	completeJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"role": "assistant",
			"content": {
				"type": "text",
				"text": "World!"
			},
			"model": "test-model",
			"stopReason": "endTurn",
			"isComplete": true,
			"chunkId": "chunk-2"
		}
	}`

	completeResponse, err := clienttest.ParseStreamingSamplingResponseForTest([]byte(completeJSON))
	if err != nil {
		t.Fatalf("failed to parse valid complete streaming response: %v", err)
	}

	if !completeResponse.IsComplete {
		t.Error("expected isComplete to be true for complete chunk")
	}

	if completeResponse.StopReason != "endTurn" {
		t.Errorf("expected stopReason to be 'endTurn', got %s", completeResponse.StopReason)
	}

	// Test missing chunkId (required in 2025-03-26)
	missingChunkIDJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"role": "assistant",
			"content": {
				"type": "text",
				"text": "Hello"
			},
			"model": "test-model",
			"isComplete": false
		}
	}`

	missingChunkIDResponse, err := clienttest.ParseStreamingSamplingResponseForTest([]byte(missingChunkIDJSON))
	if err != nil {
		t.Fatalf("failed to parse streaming response with missing chunkId: %v", err)
	}

	// Validation should fail due to missing chunkId
	err = clienttest.ValidateStreamingSamplingResponseForVersionForTest(missingChunkIDResponse, "2025-03-26")
	if err == nil {
		t.Error("validation should fail for streaming response missing chunkId in 2025-03-26")
	} else {
		t.Logf("Expected validation error for missing chunkId in 2025-03-26: %v", err)
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
