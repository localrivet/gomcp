// Package v20241105 contains tests specifically for the 2024-11-05 version of the MCP specification
package v20241105

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport"
)

// MockTransport implements a simple transport for testing
type MockTransport struct {
	messageHandler transport.MessageHandler
	lastResponse   []byte
}

func (m *MockTransport) Initialize() error {
	return nil
}

func (m *MockTransport) Start() error {
	return nil
}

func (m *MockTransport) Stop() error {
	return nil
}

func (m *MockTransport) Send(message []byte) error {
	return nil
}

func (m *MockTransport) SetMessageHandler(handler transport.MessageHandler) {
	m.messageHandler = handler
}

func (m *MockTransport) ProcessMessage(message []byte) error {
	response, err := m.messageHandler(message)
	if err != nil {
		return err
	}
	m.lastResponse = response
	return nil
}

// TestToolExecutionV20241105 tests tool execution against 2024-11-05 specification
func TestToolExecutionV20241105(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-server-2024-11-05")

	// Register a test tool with 2024-11-05 features
	srv.Tool("test-tool", "A simple test tool for 2024-11-05", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "This is a 2024-11-05 tool response", nil
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
	if response.JSONRPC != "2.0" {
		t.Errorf("Expected jsonrpc to be '2.0', got %s", response.JSONRPC)
	}
	if response.ID != 1 {
		t.Errorf("Expected id to be 1, got %d", response.ID)
	}

	// Verify the result conforms to 2024-11-05 specification
	resultMap := response.Result
	if resultMap == nil {
		t.Fatalf("Expected result to be a map, got nil")
	}

	// Check that the content field exists
	content, ok := resultMap["content"].([]interface{})
	if !ok {
		t.Fatalf("Expected content to be an array, got %T", resultMap["content"])
	}

	// Check that the content field is properly formatted
	if len(content) != 1 {
		t.Fatalf("Expected 1 content item, got %d", len(content))
	}

	contentItem, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected content item to be a map, got %T", content[0])
	}

	if contentItem["type"] != "text" {
		t.Errorf("Expected content type to be 'text', got %v", contentItem["type"])
	}

	if contentItem["text"] != "This is a 2024-11-05 tool response" {
		t.Errorf("Expected content text to be 'This is a 2024-11-05 tool response', got %v", contentItem["text"])
	}

	// Check that the isError field exists and is false
	isError, ok := resultMap["isError"].(bool)
	if !ok {
		t.Fatalf("Expected isError to be a boolean, got %T", resultMap["isError"])
	}
	if isError {
		t.Errorf("Expected isError to be false, got true")
	}

	// Test tool list
	toolListRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "tools/list"
	}`)

	// Process the tool list request
	toolListResponseBytes, err := server.HandleMessage(srv.GetServer(), toolListRequestJSON)
	if err != nil {
		t.Fatalf("Failed to process tools/list message: %v", err)
	}

	// Parse the tool list response
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

	// Verify tool properties
	tool := toolListResponse.Result.Tools[0]
	if tool["name"] != "test-tool" {
		t.Errorf("Expected tool name to be 'test-tool', got %v", tool["name"])
	}
	if tool["description"] != "A simple test tool for 2024-11-05" {
		t.Errorf("Expected tool description to be 'A simple test tool for 2024-11-05', got %v", tool["description"])
	}
}

// Helper function to get tool list from server
func getTools(srv server.Server) ([]interface{}, error) {
	// We are using a simplified approach to get the tools
	// In a real implementation, you would use the server's API

	// For testing, we'll return a mocked tool list
	return []interface{}{
		map[string]interface{}{
			"name":        "test-tool",
			"description": "A simple test tool for 2024-11-05",
			"inputSchema": map[string]interface{}{
				"type": "object",
			},
		},
	}, nil
}
