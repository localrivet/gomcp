// Package draft contains tests specifically for the draft version of the MCP specification
package draft

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestPromptDraft tests prompt functionality against draft specification
func TestPromptDraft(t *testing.T) {
	// Create a server
	s := server.NewServer("test-server-prompt-draft")

	// Register a prompt with variables for testing
	s.Prompt("test-prompt-draft", "A test prompt for draft spec",
		server.System("You are a helpful assistant."),
		server.User("Explain the concept of {{topic}} in simple terms."),
		server.Assistant("I'll explain {{topic}} simply."),
	)

	// Create test requests
	testCases := []struct {
		name           string
		method         string
		params         json.RawMessage
		expectedStatus int
		validateResult func(t *testing.T, result map[string]interface{})
	}{
		{
			name:           "list prompts",
			method:         "prompts/list",
			params:         json.RawMessage(`{}`),
			expectedStatus: 0,
			validateResult: func(t *testing.T, result map[string]interface{}) {
				// Validate the prompts list structure
				prompts, ok := result["prompts"].([]interface{})
				if !ok {
					t.Fatalf("Expected prompts to be a slice, got %T", result["prompts"])
				}

				// Check that we have at least our test prompt
				found := false
				for _, p := range prompts {
					prompt, ok := p.(map[string]interface{})
					if !ok {
						continue
					}
					if prompt["name"] == "test-prompt-draft" {
						found = true
						// Check arguments - draft spec requires arguments
						args, ok := prompt["arguments"].([]interface{})
						if !ok {
							t.Errorf("Expected arguments to be a slice, got %T", prompt["arguments"])
						} else if len(args) == 0 {
							t.Errorf("Expected at least one argument, got none")
						}
						break
					}
				}
				if !found {
					t.Errorf("Test prompt not found in prompts list")
				}
			},
		},
		{
			name:           "get prompt with arguments",
			method:         "prompts/get",
			params:         json.RawMessage(`{"name":"test-prompt-draft","variables":{"topic":"neural networks"}}`),
			expectedStatus: 0,
			validateResult: func(t *testing.T, result map[string]interface{}) {
				// Validate prompt structure according to draft spec
				description, ok := result["description"].(string)
				if !ok || description != "A test prompt for draft spec" {
					t.Errorf("Expected description 'A test prompt for draft spec', got %v", description)
				}

				// Check the messages
				messages, ok := result["messages"].([]interface{})
				if !ok {
					t.Fatalf("Expected messages to be a slice, got %T", result["messages"])
				}
				if len(messages) != 3 {
					t.Errorf("Expected 3 messages, got %d", len(messages))
				}

				// Check second message (user)
				if len(messages) > 1 {
					msg, ok := messages[1].(map[string]interface{})
					if !ok {
						t.Fatalf("Expected message to be a map, got %T", messages[1])
					}

					// Check role
					role, ok := msg["role"].(string)
					if !ok || role != "user" {
						t.Errorf("Expected role 'user', got %v", role)
					}

					// Check content format - draft spec requires content object
					content, ok := msg["content"].(map[string]interface{})
					if !ok {
						t.Fatalf("Expected content to be a map, got %T", msg["content"])
					}

					// Content should have type
					contentType, ok := content["type"].(string)
					if !ok || contentType != "text" {
						t.Errorf("Expected content type 'text', got %v", contentType)
					}

					// Content should have text with variables substituted
					text, ok := content["text"].(string)
					if !ok || text != "Explain the concept of neural networks in simple terms." {
						t.Errorf("Expected text with substituted variable, got %v", text)
					}
				}
			},
		},
		{
			name:           "get prompt with missing argument",
			method:         "prompts/get",
			params:         json.RawMessage(`{"name":"test-prompt-draft","variables":{}}`),
			expectedStatus: -32602, // Invalid params
			validateResult: nil,
		},
		{
			name:           "get nonexistent prompt",
			method:         "prompts/get",
			params:         json.RawMessage(`{"name":"nonexistent-prompt","variables":{}}`),
			expectedStatus: -32602, // Invalid params
			validateResult: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a request
			request := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "1",
				"method":  tc.method,
			}
			if tc.params != nil {
				var params interface{}
				if err := json.Unmarshal(tc.params, &params); err != nil {
					t.Fatalf("Failed to unmarshal params: %v", err)
				}
				request["params"] = params
			}

			// Convert to JSON
			requestJSON, err := json.Marshal(request)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			// Process the request using s.GetServer()
			responseBytes, err := server.HandleMessage(s.GetServer(), requestJSON)
			if err != nil {
				t.Fatalf("Failed to process message: %v", err)
			}

			// Parse the response
			var response map[string]interface{}
			if err := json.Unmarshal(responseBytes, &response); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			// Check for errors
			if tc.expectedStatus != 0 {
				error, ok := response["error"].(map[string]interface{})
				if !ok {
					t.Fatalf("Expected error response, got %v", response)
				}
				code, ok := error["code"].(float64)
				if !ok || int(code) != tc.expectedStatus {
					t.Errorf("Expected error code %d, got %v", tc.expectedStatus, code)
				}
				return
			}

			// Check for success
			result, ok := response["result"].(map[string]interface{})
			if !ok {
				t.Fatalf("Expected result map, got %v", response["result"])
			}

			// Validate the result
			if tc.validateResult != nil {
				tc.validateResult(t, result)
			}
		})
	}
}
