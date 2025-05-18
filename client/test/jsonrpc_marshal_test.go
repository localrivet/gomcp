package test

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/client/test/jsonrpc"
)

func TestJSONRPCMarshaling(t *testing.T) {
	// Create a tool request
	toolReq := jsonrpc.NewToolRequest(1, "calculator", map[string]interface{}{
		"operation": "add",
		"x":         5,
		"y":         3,
	})

	// Marshal to JSON
	toolReqJSON, err := json.Marshal(toolReq)
	if err != nil {
		t.Fatalf("Failed to marshal tool request: %v", err)
	}

	// Parse as a map to check structure
	var reqMap map[string]interface{}
	if err := json.Unmarshal(toolReqJSON, &reqMap); err != nil {
		t.Fatalf("Failed to parse request JSON: %v", err)
	}

	// Check basic fields
	if reqMap["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %v", reqMap["jsonrpc"])
	}

	if reqMap["method"] != "tool/execute" {
		t.Errorf("Expected method tool/execute, got %v", reqMap["method"])
	}

	if reqMap["id"] != float64(1) {
		t.Errorf("Expected id 1, got %v", reqMap["id"])
	}

	// Test unmarshaling back to struct
	var parsedReq jsonrpc.JSONRPC
	if err := json.Unmarshal(toolReqJSON, &parsedReq); err != nil {
		t.Fatalf("Failed to unmarshal request back to struct: %v", err)
	}

	// Verify struct fields
	if parsedReq.Version != "2.0" {
		t.Errorf("Expected Version 2.0, got %v", parsedReq.Version)
	}

	if parsedReq.Method != "tool/execute" {
		t.Errorf("Expected Method tool/execute, got %v", parsedReq.Method)
	}

	if parsedReq.ID != float64(1) {
		t.Errorf("Expected ID 1, got %v", parsedReq.ID)
	}

	// Test error response
	errorResp := jsonrpc.NewErrorResponse(2, 404, "Not Found", nil)
	errorJSON, err := json.Marshal(errorResp)
	if err != nil {
		t.Fatalf("Failed to marshal error response: %v", err)
	}

	var parsedError jsonrpc.JSONRPC
	if err := json.Unmarshal(errorJSON, &parsedError); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	if parsedError.Error == nil {
		t.Fatal("Expected Error to be present")
	}

	if parsedError.Error.Code != 404 {
		t.Errorf("Expected error code 404, got %d", parsedError.Error.Code)
	}

	if parsedError.Error.Message != "Not Found" {
		t.Errorf("Expected error message 'Not Found', got %s", parsedError.Error.Message)
	}
}
