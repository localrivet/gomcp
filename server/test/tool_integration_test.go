// Package integration contains integration tests for testing the MCP server
package test

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestFluentToolRegistration tests the fluent API for tool registration during initialization
func TestFluentToolRegistration(t *testing.T) {
	// Create a server using fluent API with tools
	srv := server.NewServer("integration-test").
		Tool("tool1", "First test tool", func(ctx *server.Context, args interface{}) (interface{}, error) {
			return "Tool 1 Response", nil
		}).
		Tool("tool2", "Second test tool", func(ctx *server.Context, args interface{}) (interface{}, error) {
			return "Tool 2 Response", nil
		}).
		WithAnnotations("tool1", map[string]interface{}{
			"isReadOnly": true,
			"category":   "test",
		})

	// Create a JSON-RPC request for tool list
	requestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/list"
	}`)

	// Process the request directly using the exported HandleMessage method
	responseBytes, err := server.HandleMessage(srv.GetServer(), requestJSON)
	if err != nil {
		t.Fatalf("Failed to process message: %v", err)
	}

	// Parse the response
	var response struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []map[string]interface{} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify two tools are registered
	if len(response.Result.Tools) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(response.Result.Tools))
	}

	// Test tool execution for the first tool
	toolCallRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "tools/call",
		"params": {
			"name": "tool1",
			"arguments": {}
		}
	}`)

	toolCallResponseBytes, err := server.HandleMessage(srv.GetServer(), toolCallRequestJSON)
	if err != nil {
		t.Fatalf("Failed to process tools/call message: %v", err)
	}

	// Parse the response
	var toolCallResponse struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(toolCallResponseBytes, &toolCallResponse); err != nil {
		t.Fatalf("Failed to parse tools/call response: %v", err)
	}

	// Log the response for debugging
	t.Logf("Tool1 response: %s", string(toolCallResponse.Result))

	// First, try to unmarshal as a string (direct response)
	var stringResult string
	stringErr := json.Unmarshal(toolCallResponse.Result, &stringResult)

	// Next, try to unmarshal as a content structure (wrapped response)
	var contentResult struct {
		Content []map[string]interface{} `json:"content"`
		IsError bool                     `json:"isError"`
	}
	contentErr := json.Unmarshal(toolCallResponse.Result, &contentResult)

	// Check if either parsing was successful
	if stringErr == nil {
		// Direct string response (matches original tool response)
		if stringResult != "Tool 1 Response" {
			t.Errorf("Expected tool result 'Tool 1 Response', got %q", stringResult)
		}
	} else if contentErr == nil {
		// Content structure response
		if contentResult.IsError {
			t.Errorf("Expected tool execution success, got error")
		}
		if len(contentResult.Content) == 0 {
			t.Errorf("Expected at least one content item, got none")
		} else {
			// Check the first content item if it's text
			firstContent := contentResult.Content[0]
			if contentType, ok := firstContent["type"].(string); ok && contentType == "text" {
				if text, ok := firstContent["text"].(string); ok {
					if text != "Tool 1 Response" {
						t.Errorf("Expected content text 'Tool 1 Response', got %q", text)
					}
				} else {
					t.Errorf("Content type is 'text' but no 'text' field found")
				}
			}
		}
	} else {
		// Neither parsing worked
		t.Errorf("Could not parse tool result as either string or content structure: %v / %v", stringErr, contentErr)
		t.Logf("Raw result: %s", string(toolCallResponse.Result))
	}

	// Register a third tool for testing
	srv.Tool("tool3", "Third test tool", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "Tool 3 Response", nil
	})

	// Create a JSON-RPC request to list tools (should now include all three)
	listToolsCallJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 4,
		"method": "tools/list"
	}`)

	listToolsResponseBytes, err := server.HandleMessage(srv.GetServer(), listToolsCallJSON)
	if err != nil {
		t.Fatalf("Failed to process tools/list message after adding third tool: %v", err)
	}

	// Parse the response
	var listToolsResponse struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []map[string]interface{} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(listToolsResponseBytes, &listToolsResponse); err != nil {
		t.Fatalf("Failed to parse tools/list response: %v", err)
	}

	// Verify all three tools are registered
	if len(listToolsResponse.Result.Tools) != 3 {
		t.Fatalf("Expected 3 tools, got %d", len(listToolsResponse.Result.Tools))
	}

	// Verify that the specific tools exist
	var toolNames []string
	for _, tool := range listToolsResponse.Result.Tools {
		if name, ok := tool["name"].(string); ok {
			toolNames = append(toolNames, name)
		}
	}

	// Check for each expected tool name
	expectedTools := []string{"tool1", "tool2", "tool3"}
	for _, expected := range expectedTools {
		found := false
		for _, actual := range toolNames {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Tool %q not found in tools list. Available tools: %v", expected, toolNames)
		}
	}

	// Verify that the annotation made it through
	for _, tool := range listToolsResponse.Result.Tools {
		if name, ok := tool["name"].(string); ok && name == "tool1" {
			annotations, hasAnnotations := tool["annotations"].(map[string]interface{})
			if !hasAnnotations {
				t.Error("Expected tool1 to have annotations, but none found")
				continue
			}

			isReadOnly, hasReadOnly := annotations["isReadOnly"].(bool)
			if !hasReadOnly || !isReadOnly {
				t.Errorf("Expected tool1 to have isReadOnly=true annotation, got %v", annotations["isReadOnly"])
			}

			category, hasCategory := annotations["category"].(string)
			if !hasCategory || category != "test" {
				t.Errorf("Expected tool1 to have category='test' annotation, got %v", annotations["category"])
			}
		}
	}
}

// Helper function to get the keys of a map as a slice
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
