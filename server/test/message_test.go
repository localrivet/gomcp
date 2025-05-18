package test

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestHandleMessage tests the message routing functionality
func TestHandleMessage(t *testing.T) {
	// Create a server
	s := server.NewServer("test-server")

	// Register a test tool
	s.Tool("test-tool", "Test tool", func(ctx *server.Context, args map[string]interface{}) (interface{}, error) {
		return "tool-executed", nil
	})

	// Register a test resource using wildcards for dynamic paths
	s.Resource("/test-resource/{param}", "Test resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// Simple mock implementation
		return "resource-executed", nil
	})

	// Register a test prompt
	s.Prompt("test-prompt", "Test prompt", "template content")

	// Create test messages
	testCases := []struct {
		name          string
		message       interface{}
		expectedType  string
		expectedError bool
	}{
		{
			name: "tool call message",
			message: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "test-id-1",
				"method":  "tools/call",
				"params": map[string]interface{}{
					"name":      "test-tool",
					"arguments": map[string]interface{}{},
				},
			},
			expectedType:  "2.0",
			expectedError: false,
		},
		{
			name: "resource request message",
			message: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "test-id-2",
				"method":  "resources/read",
				"params": map[string]interface{}{
					"uri": "/test-resource/value",
				},
			},
			expectedType:  "2.0",
			expectedError: false,
		},
		{
			name: "prompt request message",
			message: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "test-id-3",
				"method":  "prompts/get",
				"params": map[string]interface{}{
					"name": "test-prompt",
				},
			},
			expectedType:  "2.0",
			expectedError: false,
		},
		{
			name: "unknown method",
			message: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "test-id-4",
				"method":  "unknown_method",
			},
			expectedType:  "2.0",
			expectedError: true,
		},
		{
			name: "invalid tool call message",
			message: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "test-id-5",
				"method":  "tools/call",
				"params": map[string]interface{}{
					"name":      "nonexistent-tool",
					"arguments": map[string]interface{}{},
				},
			},
			expectedType:  "2.0",
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Marshal the message to JSON
			messageBytes, err := json.Marshal(tc.message)
			if err != nil {
				t.Fatalf("Failed to marshal message: %v", err)
			}

			// Handle the message
			responseBytes, err := server.HandleMessage(s.GetServer(), messageBytes)
			if err != nil {
				t.Fatalf("handleMessage returned error: %v", err)
			}

			// Parse the response
			var response map[string]interface{}
			if err := json.Unmarshal(responseBytes, &response); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			// Check the response jsonrpc version
			jsonrpc, ok := response["jsonrpc"].(string)
			if !ok {
				t.Fatal("Response missing 'jsonrpc' field")
			}

			if jsonrpc != tc.expectedType {
				t.Errorf("Expected jsonrpc version %s, got %s", tc.expectedType, jsonrpc)
			}

			// Check for errors
			if tc.expectedError {
				if _, hasError := response["error"]; !hasError {
					t.Errorf("Expected error response, but got none")
				}
			} else {
				if _, hasError := response["error"]; hasError {
					t.Errorf("Expected successful response, got error: %v", response["error"])
				}
				if _, hasResult := response["result"]; !hasResult {
					t.Errorf("Expected result in response, but found none")
				}
			}
		})
	}
}

// TestPing verifies the ping functionality across all protocol versions
func TestPing(t *testing.T) {
	// Create a server
	s := server.NewServer("test-server")

	// Test ping for all protocol versions
	versions := []string{"2024-11-05", "2025-03-26", "draft"}

	for _, version := range versions {
		t.Run(version, func(t *testing.T) {
			// Create a ping message
			pingMessage := []byte(`{
				"jsonrpc": "2.0",
				"id": 1,
				"method": "ping"
			}`)

			var responseBytes []byte
			var err error

			if version == "" {
				// Test regular message handling without version
				responseBytes, err = server.HandleMessage(s.GetServer(), pingMessage)
			} else {
				// Test with specific version
				responseBytes, err = server.HandleMessageWithVersion(s, pingMessage, version)
			}

			if err != nil {
				t.Fatalf("Failed to handle ping message for version %s: %v", version, err)
			}

			// Parse the response
			var response struct {
				JSONRPC string      `json:"jsonrpc"`
				ID      int         `json:"id"`
				Result  interface{} `json:"result"`
			}
			if err := json.Unmarshal(responseBytes, &response); err != nil {
				t.Fatalf("Failed to parse ping response: %v", err)
			}

			// Verify the response
			if response.JSONRPC != "2.0" {
				t.Errorf("Expected jsonrpc version '2.0', got '%s'", response.JSONRPC)
			}
			if response.ID != 1 {
				t.Errorf("Expected ID 1, got %v", response.ID)
			}

			// According to all protocol specifications, the ping response must be an empty object ({})
			result, ok := response.Result.(map[string]interface{})
			if !ok {
				t.Fatalf("Expected result to be a map, got %T", response.Result)
			}

			// Check if the result has any keys
			if len(result) > 0 {
				t.Errorf("Expected empty result object {}, got %v", result)
			}
		})
	}
}

// TestRootsList tests that the server properly rejects client-side roots/list method
func TestRootsList(t *testing.T) {
	// This test is skipped because roots/list is implemented on the client side, not server side
	t.Skip("Roots/list is implemented in the client, not the server - skipping this test")
}

// TestRootsListVersions verifies roots/list behavior across all protocol versions
func TestRootsListVersions(t *testing.T) {
	// This test is skipped because roots/list is implemented on the client side, not server side
	t.Skip("Roots/list is implemented in the client, not the server - skipping this test")
}
