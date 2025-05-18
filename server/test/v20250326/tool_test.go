// Package v20250326 contains tests specifically for the 2025-03-26 version of the MCP specification
package v20250326

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestToolExecutionV20250326 tests tool execution against 2025-03-26 specification
func TestToolExecutionV20250326(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-server-2025-03-26")

	// Register a test tool with 2025-03-26 features
	srv.Tool("test-tool", "A simple test tool for 2025-03-26 spec", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// Return a response that's compatible with 2025-03-26 spec
		return "This is a 2025-03-26 tool response", nil
	})

	// Add tool annotations
	srv.WithAnnotations("test-tool", map[string]interface{}{
		"isReadOnly":    true,
		"isDestructive": false,
		"category":      "utility",
		"tags":          []string{"test", "example", "v20250326"},
	})

	// Create a JSON-RPC request for tool call
	requestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "test-tool",
			"arguments": {}
		}
	}`)

	// Process the request using the wrapper's HandleMessage method
	responseBytes, err := server.HandleMessage(srv.GetServer(), requestJSON)
	if err != nil {
		t.Fatalf("Failed to process message: %v", err)
	}

	// Parse the response
	var response struct {
		JSONRPC string                 `json:"jsonrpc"`
		ID      int                    `json:"id"`
		Result  map[string]interface{} `json:"result"`
	}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify the response structure
	resultMap := response.Result
	content, ok := resultMap["content"].([]interface{})
	if !ok {
		t.Fatalf("Expected content to be an array, got %T", resultMap["content"])
	}

	// Should have one content item (text)
	if len(content) != 1 {
		t.Fatalf("Expected 1 content item, got %d", len(content))
	}

	contentItem, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected content item to be a map, got %T", content[0])
	}

	// Verify content item is text
	if contentItem["type"] != "text" {
		t.Errorf("Expected content type to be 'text', got %v", contentItem["type"])
	}

	// Verify content text
	if contentItem["text"] != "This is a 2025-03-26 tool response" {
		t.Errorf("Expected content text to be 'This is a 2025-03-26 tool response', got %v", contentItem["text"])
	}

	// Test annotations in tool listing
	toolListRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "tools/list"
	}`)

	toolListResponseBytes, err := server.HandleMessage(srv.GetServer(), toolListRequestJSON)
	if err != nil {
		t.Fatalf("Failed to process tools/list message: %v", err)
	}

	var toolListResponse struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []map[string]interface{} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(toolListResponseBytes, &toolListResponse); err != nil {
		t.Fatalf("Failed to parse tools/list response: %v", err)
	}

	// Verify we have one tool
	if len(toolListResponse.Result.Tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(toolListResponse.Result.Tools))
	}

	// Check for annotations
	tool := toolListResponse.Result.Tools[0]
	annotations, ok := tool["annotations"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected tool to have annotations, but they were missing")
	}

	// Check for specific annotations
	if category, ok := annotations["category"].(string); ok {
		if category != "utility" {
			t.Errorf("Expected 'category' annotation to be 'utility', got %v", category)
		}
	}
}
