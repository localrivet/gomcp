// Package v20241105 contains tests specifically for the 2024-11-05 version of the MCP specification
package v20241105

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestPromptV20241105 tests prompt functionality against 2024-11-05 specification
func TestPromptV20241105(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-server-prompt-2024-11-05")

	// Register a prompt with variables for testing
	srv.Prompt("test-prompt-2024-11-05", "A test prompt for 2024-11-05 spec",
		server.System("You are a helpful assistant."),
		server.User("Explain the concept of {{topic}} in simple terms."),
		server.Assistant("I'll explain {{topic}} simply."),
	)

	// Create JSON-RPC request for listing prompts
	promptListRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "prompts/list",
	}

	promptListRequestJSON, err := json.Marshal(promptListRequest)
	if err != nil {
		t.Fatalf("Failed to marshal prompt list request: %v", err)
	}

	// Send the prompts/list request
	promptListResponseBytes, err := server.HandleMessage(srv.GetServer(), promptListRequestJSON)
	if err != nil {
		t.Fatalf("Failed to process prompt list message: %v", err)
	}

	// Parse response
	var promptListResponse map[string]interface{}
	if err := json.Unmarshal(promptListResponseBytes, &promptListResponse); err != nil {
		t.Fatalf("Failed to unmarshal prompt list response: %v", err)
	}

	// Check for errors
	if error, hasError := promptListResponse["error"]; hasError {
		t.Fatalf("Expected success, got error: %v", error)
	}

	// Validate the prompts list
	result, ok := promptListResponse["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", promptListResponse["result"])
	}

	prompts, ok := result["prompts"].([]interface{})
	if !ok {
		t.Fatalf("Expected prompts to be a slice, got %T", result["prompts"])
	}

	// Check that we have at least one prompt
	if len(prompts) == 0 {
		t.Errorf("Expected at least one prompt, got none")
	}

	// Check that our test prompt is in the list
	var found bool
	for _, p := range prompts {
		prompt, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if prompt["name"] == "test-prompt-2024-11-05" {
			found = true
			// 2024-11-05 spec should have arguments listed
			if args, ok := prompt["arguments"].([]interface{}); !ok || len(args) == 0 {
				t.Errorf("Expected arguments in prompt listing")
			}
			break
		}
	}
	if !found {
		t.Errorf("Test prompt not found in prompts list")
	}

	// Create JSON-RPC request for getting a prompt with variables
	promptGetRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "prompts/get",
		"params": map[string]interface{}{
			"name": "test-prompt-2024-11-05",
			"variables": map[string]interface{}{
				"topic": "databases",
			},
		},
	}

	promptGetRequestJSON, err := json.Marshal(promptGetRequest)
	if err != nil {
		t.Fatalf("Failed to marshal prompt get request: %v", err)
	}

	// Send the prompts/get request
	promptGetResponseBytes, err := server.HandleMessage(srv.GetServer(), promptGetRequestJSON)
	if err != nil {
		t.Fatalf("Failed to process prompt get message: %v", err)
	}

	// Parse response
	var promptGetResponse map[string]interface{}
	if err := json.Unmarshal(promptGetResponseBytes, &promptGetResponse); err != nil {
		t.Fatalf("Failed to unmarshal prompt get response: %v", err)
	}

	// Check for errors
	if error, hasError := promptGetResponse["error"]; hasError {
		t.Fatalf("Expected success, got error: %v", error)
	}

	// Validate the prompt response according to 2024-11-05 specification
	getResult, ok := promptGetResponse["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", promptGetResponse["result"])
	}

	// Should have description
	description, ok := getResult["description"].(string)
	if !ok || description != "A test prompt for 2024-11-05 spec" {
		t.Errorf("Expected description 'A test prompt for 2024-11-05 spec', got %v", description)
	}

	// Should have messages array
	messages, ok := getResult["messages"].([]interface{})
	if !ok || len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %v", messages)
	}

	// Check the user message (second one)
	if len(messages) >= 2 {
		userMsg, ok := messages[1].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected user message to be a map, got %T", messages[1])
		}

		// Check role
		role, ok := userMsg["role"].(string)
		if !ok || role != "user" {
			t.Errorf("Expected role 'user', got %v", role)
		}

		// Check content (should be object with type and text)
		content, ok := userMsg["content"].(map[string]interface{})
		if !ok {
			t.Errorf("Expected content to be a map, got %T", userMsg["content"])
		} else {
			// Check content type
			cType, ok := content["type"].(string)
			if !ok || cType != "text" {
				t.Errorf("Expected content type 'text', got %v", cType)
			}

			// Check text with variable substituted
			text, ok := content["text"].(string)
			if !ok || text != "Explain the concept of databases in simple terms." {
				t.Errorf("Expected text with variable substituted, got %v", text)
			}
		}
	}

	// Test with missing required argument
	promptGetMissingArgRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "prompts/get",
		"params": map[string]interface{}{
			"name":      "test-prompt-2024-11-05",
			"variables": map[string]interface{}{},
		},
	}

	promptGetMissingArgRequestJSON, err := json.Marshal(promptGetMissingArgRequest)
	if err != nil {
		t.Fatalf("Failed to marshal prompt get missing arg request: %v", err)
	}

	// Send the request with missing argument
	promptGetMissingArgResponseBytes, err := server.HandleMessage(srv.GetServer(), promptGetMissingArgRequestJSON)
	if err != nil {
		t.Fatalf("Failed to process prompt get missing arg message: %v", err)
	}

	// Parse response
	var promptGetMissingArgResponse map[string]interface{}
	if err := json.Unmarshal(promptGetMissingArgResponseBytes, &promptGetMissingArgResponse); err != nil {
		t.Fatalf("Failed to unmarshal prompt get missing arg response: %v", err)
	}

	// Should have error with code -32602 (Invalid params)
	errorObj, hasError := promptGetMissingArgResponse["error"].(map[string]interface{})
	if !hasError {
		t.Fatalf("Expected error for missing required argument, got success: %v", promptGetMissingArgResponse)
	}

	errorCode, ok := errorObj["code"].(float64)
	if !ok || int(errorCode) != -32602 {
		t.Errorf("Expected error code -32602, got %v", errorCode)
	}
}
