package jsonrpc

import (
	"encoding/json"
	"testing"
)

func TestJSONRPCTypes(t *testing.T) {
	// Test basic JSONRPC struct
	message := &JSONRPC{
		Version: "2.0",
		ID:      1,
		Method:  "test/method",
		Params:  map[string]interface{}{"key": "value"},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("Failed to marshal JSONRPC: %v", err)
	}

	// Unmarshal back
	var parsed JSONRPC
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSONRPC: %v", err)
	}

	// Verify fields
	if parsed.Version != "2.0" {
		t.Errorf("Expected Version 2.0, got %s", parsed.Version)
	}
	if parsed.ID != float64(1) {
		t.Errorf("Expected ID 1, got %v", parsed.ID)
	}
	if parsed.Method != "test/method" {
		t.Errorf("Expected Method 'test/method', got %s", parsed.Method)
	}
}

func TestErrorType(t *testing.T) {
	// Test RPCError struct
	errorObj := &RPCError{
		Code:    404,
		Message: "Not Found",
		Data:    map[string]interface{}{"path": "/missing"},
	}

	// Create message with error
	message := &JSONRPC{
		Version: "2.0",
		ID:      1,
		Error:   errorObj,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("Failed to marshal error message: %v", err)
	}

	// Unmarshal back
	var parsed JSONRPC
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal error message: %v", err)
	}

	// Verify error fields
	if parsed.Error == nil {
		t.Fatal("Expected Error to be present")
	}
	if parsed.Error.Code != 404 {
		t.Errorf("Expected Error.Code 404, got %d", parsed.Error.Code)
	}
	if parsed.Error.Message != "Not Found" {
		t.Errorf("Expected Error.Message 'Not Found', got %s", parsed.Error.Message)
	}
}

func TestBuilderFunctions(t *testing.T) {
	// Test NewToolRequest
	toolReq := NewToolRequest(1, "calculator", map[string]interface{}{
		"operation": "add",
		"x":         5,
		"y":         3,
	})

	if toolReq.Method != "tool/execute" {
		t.Errorf("Expected Method 'tool/execute', got %s", toolReq.Method)
	}

	// Test NewErrorResponse
	errorResp := NewErrorResponse(2, 400, "Bad Request", nil)
	if errorResp.Error == nil {
		t.Fatal("Expected Error to be present")
	}
	if errorResp.Error.Code != 400 {
		t.Errorf("Expected Error.Code 400, got %d", errorResp.Error.Code)
	}

	// Test NewResourceRequest
	resReq := NewResourceRequest(3, "/test/path")
	if resReq.Method != "resource/get" {
		t.Errorf("Expected Method 'resource/get', got %s", resReq.Method)
	}

	// Marshal and validate JSON structure
	jsonData, err := json.Marshal(resReq)
	if err != nil {
		t.Fatalf("Failed to marshal resource request: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	params, ok := parsed["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params to be a map, got %T", parsed["params"])
	}

	if path, ok := params["path"].(string); !ok || path != "/test/path" {
		t.Errorf("Expected path '/test/path', got %v", params["path"])
	}
}
