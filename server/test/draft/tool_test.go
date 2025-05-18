// Package draft contains tests specifically for the draft version of the MCP specification
package draft

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestToolExecutionDraft tests tool execution against draft specification
func TestToolExecutionDraft(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-server-draft")

	// Register a test tool with potential draft features
	srv.Tool("test-tool", "A simple test tool for draft spec", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// Return a response that's compatible with the draft spec
		// This may need to be updated as the draft evolves
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "This is a draft spec tool response",
				},
				{
					"type":     "image",
					"imageUrl": "https://example.com/image.png",
					"altText":  "An example image",
				},
				{
					"type":     "audio",
					"audioUrl": "https://example.com/audio.mp3",
					"altText":  "An example audio file",
				},
			},
			"isError": false,
		}, nil
	})

	// Add tool annotations with potential draft extensions
	srv.WithAnnotations("test-tool", map[string]interface{}{
		"isReadOnly":    true,
		"isDestructive": false,
		"category":      "example",
		"tags":          []string{"test", "example", "draft"},
		// Potential draft annotations
		"compatibility": []string{"2024-11-05", "2025-03-26", "draft"},
		"securityLevel": "high",
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

	// Process the request directly using the exported HandleMessage method
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

	// Should have three content items (text, image, audio)
	if len(content) != 3 {
		t.Fatalf("Expected 3 content items, got %d", len(content))
	}

	// Check for audio content (potentially new in draft)
	audioContent, ok := content[2].(map[string]interface{})
	if !ok || audioContent["type"] != "audio" {
		t.Errorf("Expected third content item to be audio, got %T or type %v",
			content[2], audioContent["type"])
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

	// Check for draft annotations
	tool := toolListResponse.Result.Tools[0]
	annotations, ok := tool["annotations"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected tool to have annotations, but they were missing")
	}

	// Check for potential draft-specific annotations
	if securityLevel, ok := annotations["securityLevel"].(string); ok {
		if securityLevel != "high" {
			t.Errorf("Expected 'securityLevel' annotation to be 'high', got %v", securityLevel)
		}
	}

	if compatibility, ok := annotations["compatibility"].([]interface{}); ok {
		if len(compatibility) != 3 {
			t.Errorf("Expected 'compatibility' annotation to have 3 items, got %d", len(compatibility))
		}
	}
}
