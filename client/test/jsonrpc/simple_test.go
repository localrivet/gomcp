package jsonrpc

import (
	"encoding/json"
	"testing"
)

func TestSimpleToolRequest(t *testing.T) {
	// Create a tool request
	req := NewToolRequest(1, "calculator", map[string]interface{}{
		"operation": "add",
		"x":         5,
		"y":         3,
	})

	// Marshal to JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Unmarshal to map to check structure
	var reqMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &reqMap); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	// Check basic structure
	if reqMap["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc field to be '2.0', got %v", reqMap["jsonrpc"])
	}

	if reqMap["method"] != "tool/execute" {
		t.Errorf("Expected method to be 'tool/execute', got %v", reqMap["method"])
	}

	// Check params structure
	params, ok := reqMap["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params to be a map, got %T", reqMap["params"])
	}

	if params["name"] != "calculator" {
		t.Errorf("Expected name to be 'calculator', got %v", params["name"])
	}

	args, ok := params["arguments"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected arguments to be a map, got %T", params["arguments"])
	}

	if args["operation"] != "add" {
		t.Errorf("Expected operation to be 'add', got %v", args["operation"])
	}

	if args["x"] != float64(5) {
		t.Errorf("Expected x to be 5, got %v", args["x"])
	}

	if args["y"] != float64(3) {
		t.Errorf("Expected y to be 3, got %v", args["y"])
	}
}

func TestSimpleErrorResponse(t *testing.T) {
	// Create an error response
	errorResp := NewErrorResponse(2, 404, "Not Found", map[string]interface{}{
		"path": "/missing/resource",
	})

	// Marshal to JSON
	jsonData, err := json.Marshal(errorResp)
	if err != nil {
		t.Fatalf("Failed to marshal error response: %v", err)
	}

	// Unmarshal to map to check structure
	var respMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &respMap); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	// Check error structure
	errorObj, ok := respMap["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected error to be a map, got %T", respMap["error"])
	}

	if errorObj["code"] != float64(404) {
		t.Errorf("Expected error code to be 404, got %v", errorObj["code"])
	}

	if errorObj["message"] != "Not Found" {
		t.Errorf("Expected error message to be 'Not Found', got %v", errorObj["message"])
	}

	data, ok := errorObj["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected error data to be a map, got %T", errorObj["data"])
	}

	if data["path"] != "/missing/resource" {
		t.Errorf("Expected path to be '/missing/resource', got %v", data["path"])
	}
}
