package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/localrivet/gomcp/client/test/jsonrpc"
)

func TestCreateToolResponse(t *testing.T) {
	// Test creating a tool response with a string output
	response := CreateToolResponse("Test output")
	var data map[string]interface{}
	err := json.Unmarshal(response, &data)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify response structure
	if data["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc to be 2.0, got %v", data["jsonrpc"])
	}

	if data["id"] != float64(1) {
		t.Errorf("Expected ID to be 1, got %v", data["id"])
	}

	result, ok := data["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", data["result"])
	}

	if result["output"] != "Test output" {
		t.Errorf("Expected output to be 'Test output', got %v", result["output"])
	}

	// Test with a complex object
	complexOutput := map[string]interface{}{
		"status": "success",
		"count":  42,
		"items":  []string{"item1", "item2"},
	}
	response = CreateToolResponse(complexOutput)
	err = json.Unmarshal(response, &data)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	result, ok = data["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", data["result"])
	}

	outputMap, ok := result["output"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected output to be a map, got %T", result["output"])
	}

	if outputMap["status"] != "success" {
		t.Errorf("Expected status to be 'success', got %v", outputMap["status"])
	}

	if outputMap["count"] != float64(42) {
		t.Errorf("Expected count to be 42, got %v", outputMap["count"])
	}
}

func TestCreateResourceResponse(t *testing.T) {
	// Test creating resource responses for different versions
	versions := []string{"2024-11-05", "2025-03-26", "draft"}

	for _, version := range versions {
		t.Run("TextContent-"+version, func(t *testing.T) {
			response := CreateResourceResponse(version, "Hello, World!")
			var data map[string]interface{}
			err := json.Unmarshal(response, &data)
			if err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			// Verify basic structure
			if data["jsonrpc"] != "2.0" {
				t.Errorf("Expected jsonrpc to be 2.0, got %v", data["jsonrpc"])
			}

			result, ok := data["result"].(map[string]interface{})
			if !ok {
				t.Fatalf("Expected result to be a map, got %T", data["result"])
			}

			// Check version-specific format
			switch version {
			case "2025-03-26":
				contents, ok := result["contents"].([]interface{})
				if !ok || len(contents) == 0 {
					t.Fatalf("Expected contents array, got %v", result["contents"])
				}

				content := contents[0].(map[string]interface{})
				if content["uri"] != "/test/resource" {
					t.Errorf("Expected URI to be /test/resource, got %v", content["uri"])
				}
			case "2024-11-05", "draft":
				content, ok := result["content"].([]interface{})
				if !ok || len(content) == 0 {
					t.Fatalf("Expected content array, got %v", result["content"])
				}

				textContent := content[0].(map[string]interface{})
				if textContent["type"] != "text" {
					t.Errorf("Expected content type to be text, got %v", textContent["type"])
				}
			}
		})
	}
}

func TestAssertJSONContains(t *testing.T) {
	// Create a test JSON object
	testJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "test/method",
		"params": {
			"name": "test-param",
			"value": 42
		},
		"result": {
			"status": "success",
			"data": "test-data"
		}
	}`)

	// Test for fields at different levels in the JSON
	// Top level field - should be found
	testContains(t, testJSON, "jsonrpc", "2.0", true)

	// Field in params - should be found
	testContains(t, testJSON, "name", "test-param", true)

	// Field in result - should be found
	testContains(t, testJSON, "status", "success", true)

	// Nonexistent field - should not be found
	testContains(t, testJSON, "nonexistent", "value", false)

	// Wrong value - should not match
	testContains(t, testJSON, "jsonrpc", "1.0", false)
}

// testContains is a helper function to test the AssertJSONContains function
func testContains(t *testing.T, data []byte, key string, expectedValue interface{}, shouldPass bool) {
	// Parse the JSON to verify the field exists with the expected value
	var jsonObj map[string]interface{}
	if err := json.Unmarshal(data, &jsonObj); err != nil {
		t.Fatalf("Invalid JSON for testing: %v", err)
	}

	found := false
	// Check top level
	if value, ok := jsonObj[key]; ok && value == expectedValue {
		found = true
	}

	// Check in params
	if params, ok := jsonObj["params"].(map[string]interface{}); ok {
		if value, ok := params[key]; ok && value == expectedValue {
			found = true
		}
	}

	// Check in result
	if result, ok := jsonObj["result"].(map[string]interface{}); ok {
		if value, ok := result[key]; ok && value == expectedValue {
			found = true
		}
	}

	if shouldPass && !found {
		t.Errorf("Expected to find %s: %v but didn't", key, expectedValue)
	} else if !shouldPass && found {
		t.Errorf("Expected not to find %s: %v but did", key, expectedValue)
	}
}

func TestToolRequest(t *testing.T) {
	args := map[string]interface{}{
		"param1": "value1",
		"param2": 42,
	}

	request := ToolRequest("test-tool", args)

	if request["method"] != "tool/execute" {
		t.Errorf("Expected method to be tool/execute, got %v", request["method"])
	}

	params, ok := request["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params to be a map, got %T", request["params"])
	}

	if params["name"] != "test-tool" {
		t.Errorf("Expected name to be test-tool, got %v", params["name"])
	}

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected arguments to be a map, got %T", params["arguments"])
	}

	if arguments["param1"] != "value1" || arguments["param2"] != 42 {
		t.Errorf("Expected arguments to match input, got %v", arguments)
	}
}

func TestCreateResourceContent(t *testing.T) {
	// Test text content
	textContent := CreateResourceContent("text", "Hello, World!")
	textMap, ok := textContent.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected text content to be a map, got %T", textContent)
	}

	if textMap["type"] != "text" || textMap["text"] != "Hello, World!" {
		t.Errorf("Expected text content to match input, got %v", textMap)
	}

	// Test image content
	imageContent := CreateResourceContent("image", nil)
	imageMap, ok := imageContent.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected image content to be a map, got %T", imageContent)
	}

	if imageMap["type"] != "image" {
		t.Errorf("Expected image type, got %v", imageMap["type"])
	}

	if imageMap["url"] != "https://example.com/image.jpg" {
		t.Errorf("Expected default image URL, got %v", imageMap["url"])
	}

	// Test with custom image data
	customImage := map[string]interface{}{
		"url":     "https://custom.com/image.png",
		"altText": "Custom image",
	}

	imageContent = CreateResourceContent("image", customImage)
	imageMap, ok = imageContent.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected image content to be a map, got %T", imageContent)
	}

	if imageMap["url"] != "https://custom.com/image.png" {
		t.Errorf("Expected custom URL, got %v", imageMap["url"])
	}
}

func TestMockTransport(t *testing.T) {
	// Create a mock transport
	mock := NewMockTransport()

	// Set the Connected property to true so we don't get "connection closed" errors
	mock.mu.Lock()
	mock.Connected = true
	mock.mu.Unlock()

	// Prepare response data
	responseData := []byte(`{"jsonrpc":"2.0","id":1,"result":{"success":true}}`)

	// Queue a response
	mock.QueueResponse(responseData, nil)

	// Send a request
	request := []byte(`{"jsonrpc":"2.0","id":1,"method":"test/method","params":{"key":"value"}}`)
	response, err := mock.Send(request)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if string(response) != string(responseData) {
		t.Errorf("Expected response %s, got %s", responseData, response)
	}

	// Test request recording
	if len(mock.RequestHistory) != 1 {
		t.Fatalf("Expected 1 request in history, got %d", len(mock.RequestHistory))
	}

	record := mock.RequestHistory[0]
	if record.Method != "test/method" {
		t.Errorf("Expected method 'test/method', got '%s'", record.Method)
	}

	// Test conditional responses
	mock.QueueConditionalResponse(
		[]byte(`{"jsonrpc":"2.0","id":2,"result":{"condition":"true"}}`),
		nil,
		IsRequestMethod("conditional/method"),
	)

	conditionalRequest := []byte(`{"jsonrpc":"2.0","id":2,"method":"conditional/method","params":{}}`)
	response, err = mock.Send(conditionalRequest)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var responseObj map[string]interface{}
	if err := json.Unmarshal(response, &responseObj); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	result, ok := responseObj["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", responseObj["result"])
	}

	if result["condition"] != "true" {
		t.Errorf("Expected condition to be 'true', got %v", result["condition"])
	}
}

// Example of struct-based approach for JSONRPC using the types from jsonrpc package
func ExampleJSONRPC() {
	// Create a tool request using structs
	toolRequest := jsonrpc.NewToolRequest(1, "calculator", map[string]interface{}{
		"operation": "add",
		"x":         5,
		"y":         3,
	})

	// Marshal to JSON
	requestJSON, err := json.Marshal(toolRequest)
	if err != nil {
		fmt.Printf("Failed to marshal request: %v\n", err)
		return
	}

	// Create a response
	toolResponse := jsonrpc.NewToolResponse(1, 8, map[string]interface{}{
		"duration": 0.001,
	})

	// Marshal to JSON
	responseJSON, err := json.Marshal(toolResponse)
	if err != nil {
		fmt.Printf("Failed to marshal response: %v\n", err)
		return
	}

	// Example of using with mock transport
	mock := NewMockTransport()
	mock.QueueResponse(responseJSON, nil)

	// Send the request
	response, err := mock.Send(requestJSON)
	if err != nil {
		fmt.Printf("Failed to send request: %v\n", err)
		return
	}

	// Parse the response
	var parsedResponse map[string]interface{}
	if err := json.Unmarshal(response, &parsedResponse); err != nil {
		fmt.Printf("Failed to parse response: %v\n", err)
		return
	}

	// Type assertion for the result
	result, ok := parsedResponse["result"].(map[string]interface{})
	if !ok {
		fmt.Printf("Expected result to be a map, got %T\n", parsedResponse["result"])
		return
	}

	// Use the output
	output := result["output"]
	fmt.Printf("Output: %v\n", output)
}

func TestHelperFunctions(t *testing.T) {
	// Test CreateToolResponse
	toolResponse := CreateToolResponse("test output")
	var resp map[string]interface{}
	if err := json.Unmarshal(toolResponse, &resp); err != nil {
		t.Fatalf("Failed to parse tool response: %v", err)
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", resp["result"])
	}

	if result["output"] != "test output" {
		t.Errorf("Expected output to be 'test output', got %v", result["output"])
	}

	// Test CreateResourceResponse with different versions
	versions := []string{"draft", "2024-11-05", "2025-03-26"}
	for _, version := range versions {
		resourceResponse := CreateResourceResponse(version, "test content")

		var resp map[string]interface{}
		if err := json.Unmarshal(resourceResponse, &resp); err != nil {
			t.Fatalf("Failed to parse resource response for version %s: %v", version, err)
		}

		// Check structure based on version
		result, ok := resp["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result to be a map for version %s, got %T", version, resp["result"])
		}

		if version == "2025-03-26" {
			contents, ok := result["contents"].([]interface{})
			if !ok || len(contents) == 0 {
				t.Errorf("Expected contents array for version %s", version)
			}
		} else {
			content, ok := result["content"].([]interface{})
			if !ok || len(content) == 0 {
				t.Errorf("Expected content array for version %s", version)
			}
		}
	}

	// Test assertion helpers
	jsonData := []byte(`{"jsonrpc":"2.0","method":"test/method","params":{"key":"value"}}`)

	// Test AssertMethodEquals
	AssertMethodEquals(t, jsonData, "test/method")

	// Test AssertJSONContains
	AssertJSONContains(t, jsonData, "jsonrpc", "2.0")

	// Test AssertRequestParams
	AssertRequestParams(t, jsonData, map[string]interface{}{
		"key": "value",
	})

	// Test AssertJSONPath
	jsonWithNesting := []byte(`{"result":{"nested":{"value":42}}}`)
	AssertJSONPath(t, jsonWithNesting, []string{"result", "nested", "value"}, float64(42))
}

func TestFactories(t *testing.T) {
	// Test ToolRequest factory
	toolReq := ToolRequest("test-tool", map[string]interface{}{"arg": "value"})
	if toolReq["method"] != "tool/execute" {
		t.Errorf("Expected method to be 'tool/execute', got %v", toolReq["method"])
	}

	params, ok := toolReq["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params to be a map, got %T", toolReq["params"])
	}

	if params["name"] != "test-tool" {
		t.Errorf("Expected name to be 'test-tool', got %v", params["name"])
	}

	// Test ResourceRequest factory
	resourceReq := ResourceRequest("/test/path")
	if resourceReq["method"] != "resource/get" {
		t.Errorf("Expected method to be 'resource/get', got %v", resourceReq["method"])
	}

	// Test error response factory
	errorResp := ErrorResponse(400, "Bad Request")
	errorObj, ok := errorResp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected error to be a map, got %T", errorResp["error"])
	}

	if errorObj["code"] != 400 {
		t.Errorf("Expected code to be 400, got %v", errorObj["code"])
	}

	if errorObj["message"] != "Bad Request" {
		t.Errorf("Expected message to be 'Bad Request', got %v", errorObj["message"])
	}
}
