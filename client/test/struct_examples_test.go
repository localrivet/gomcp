package test

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/client/test/jsonrpc"
)

func TestJSONRPCTypes(t *testing.T) {
	// Example: Creating a tool request with structs
	toolReq := jsonrpc.NewToolRequest(1, "calculator", map[string]interface{}{
		"operation": "add",
		"x":         5,
		"y":         3,
	})

	// Serialize to JSON
	toolReqJSON, err := json.Marshal(toolReq)
	if err != nil {
		t.Fatalf("Failed to marshal tool request: %v", err)
	}

	// Create a transport with this request
	transport := NewMockTransport()

	// Set connection to true to prevent "connection closed" errors
	transport.mu.Lock()
	transport.Connected = true
	transport.mu.Unlock()

	// Queue a response for this request
	toolResp := jsonrpc.NewToolResponse(1, 8, map[string]interface{}{
		"duration": 0.0025,
	})

	// Serialize to JSON
	toolRespJSON, err := json.Marshal(toolResp)
	if err != nil {
		t.Fatalf("Failed to marshal tool response: %v", err)
	}

	transport.QueueResponse(toolRespJSON, nil)

	// Send the request
	response, err := transport.Send(toolReqJSON)
	if err != nil {
		t.Fatalf("Error sending request: %v", err)
	}

	// Parse the response
	var respMsg jsonrpc.JSONRPC
	if err := json.Unmarshal(response, &respMsg); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Type assertion for the result
	resultMap, ok := respMsg.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", respMsg.Result)
	}

	// Verify response values
	outputVal := resultMap["output"]
	if outputVal != float64(8) {
		t.Errorf("Expected output 8, got %v", outputVal)
	}
}

func TestResourceRequests(t *testing.T) {
	// Create a resource request
	resourceReq := jsonrpc.NewResourceRequest(2, "/test/resource")

	resourceReqJSON, err := json.Marshal(resourceReq)
	if err != nil {
		t.Fatalf("Failed to marshal resource request: %v", err)
	}

	// Just to avoid the "unused variable" error in this example
	_ = resourceReqJSON

	// Create resource content
	content := []jsonrpc.ContentItem{
		{
			Type: "text",
			Text: "Hello, World!",
		},
	}

	// Create different response types for different protocol versions

	// For 2024-11-05 and draft
	result20241105 := jsonrpc.ResourceResult20241105{
		Content: content,
	}

	// For 2025-03-26
	resource := jsonrpc.ResourceItem{
		URI:     "/test/resource",
		Text:    "Test Resource",
		Content: content,
	}

	result20250326 := jsonrpc.ResourceResult20250326{
		Contents: []jsonrpc.ResourceItem{resource},
	}

	// Create responses for different versions
	resp20241105 := &jsonrpc.JSONRPC{
		Version: "2.0",
		ID:      2,
		Result:  result20241105,
	}

	resp20250326 := &jsonrpc.JSONRPC{
		Version: "2.0",
		ID:      2,
		Result:  result20250326,
	}

	// Marshal them to JSON
	resp20241105JSON, err := json.Marshal(resp20241105)
	if err != nil {
		t.Fatalf("Failed to marshal 2024-11-05 response: %v", err)
	}

	resp20250326JSON, err := json.Marshal(resp20250326)
	if err != nil {
		t.Fatalf("Failed to marshal 2025-03-26 response: %v", err)
	}

	// Verify they match the expected structure
	var parsed20241105 map[string]interface{}
	if err := json.Unmarshal(resp20241105JSON, &parsed20241105); err != nil {
		t.Fatalf("Failed to parse 2024-11-05 response: %v", err)
	}

	result, ok := parsed20241105["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", parsed20241105["result"])
	}

	contentArray, ok := result["content"].([]interface{})
	if !ok || len(contentArray) == 0 {
		t.Errorf("Expected content array in 2024-11-05 response")
	}

	var parsed20250326 map[string]interface{}
	if err := json.Unmarshal(resp20250326JSON, &parsed20250326); err != nil {
		t.Fatalf("Failed to parse 2025-03-26 response: %v", err)
	}

	result, ok = parsed20250326["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", parsed20250326["result"])
	}

	contentsArray, ok := result["contents"].([]interface{})
	if !ok || len(contentsArray) == 0 {
		t.Errorf("Expected contents array in 2025-03-26 response")
	}
}

func TestErrorResponses(t *testing.T) {
	// Create an error response
	errorResp := jsonrpc.NewErrorResponse(3, 404, "Resource not found", map[string]interface{}{
		"path": "/missing/resource",
	})

	// Marshal to JSON
	errorRespJSON, err := json.Marshal(errorResp)
	if err != nil {
		t.Fatalf("Failed to marshal error response: %v", err)
	}

	// Verify structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(errorRespJSON, &parsed); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	errorObj, ok := parsed["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected error to be a map, got %T", parsed["error"])
	}

	if errorObj["code"] != float64(404) {
		t.Errorf("Expected code 404, got %v", errorObj["code"])
	}

	if errorObj["message"] != "Resource not found" {
		t.Errorf("Expected message 'Resource not found', got %v", errorObj["message"])
	}

	data, ok := errorObj["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data to be a map, got %T", errorObj["data"])
	}

	if data["path"] != "/missing/resource" {
		t.Errorf("Expected path '/missing/resource', got %v", data["path"])
	}
}

func TestStructEncoding(t *testing.T) {
	// Create a request using the struct system
	toolRequest := jsonrpc.NewToolRequest(1, "calculator", map[string]interface{}{
		"operation": "add",
		"x":         5,
		"y":         3,
	})

	// Marshal to JSON
	requestJSON, err := json.Marshal(toolRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Check the JSON
	var requestMap map[string]interface{}
	if err := json.Unmarshal(requestJSON, &requestMap); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Check JSONRPC version
	if requestMap["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %v", requestMap["jsonrpc"])
	}

	// Check method
	if requestMap["method"] != "tool/execute" {
		t.Errorf("Expected method tool/execute, got %v", requestMap["method"])
	}

	// Create an error response
	errorResponse := jsonrpc.NewErrorResponse(1, 400, "Bad Request", nil)

	// Marshal to JSON
	errorJSON, err := json.Marshal(errorResponse)
	if err != nil {
		t.Fatalf("Failed to marshal error: %v", err)
	}

	// Parse it back
	var parsedResp jsonrpc.JSONRPC
	if err := json.Unmarshal(errorJSON, &parsedResp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if parsedResp.Error == nil {
		t.Fatal("Expected error to be present")
	}

	if parsedResp.Error.Code != 400 {
		t.Errorf("Expected code 400, got %d", parsedResp.Error.Code)
	}
}
