package v20250618

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/client/test"
)

// No need to define MockTransport here as we're using the shared one from client/test/mocktransport.go

// setupTest creates a client with a mock transport and ensures it's initialized
func setupTest(t *testing.T) (client.Client, *test.MockTransport) {
	return test.SetupClientWithMockTransport(t, "2025-06-18")
}

func TestClientInitialization_v20250618(t *testing.T) {
	// Simply test that the setupTest function works correctly
	c, mockTransport := setupTest(t)

	// Verify that the client has proper version
	if c.Version() != "2025-06-18" {
		t.Errorf("Expected version 2025-06-18, got %s", c.Version())
	}

	// Verify that the transport was connected
	if !mockTransport.ConnectCalled {
		t.Error("Connect was not called on the transport")
	}
}

func TestGetResource_v20250618(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Set up mock response
	mockResponse := test.CreateResourceResponse("2025-06-18", "Hello, World!")
	mockTransport.QueueResponse(mockResponse, nil)

	// Call GetResource
	resource, err := c.GetResource("/test/path")
	if err != nil {
		t.Fatalf("GetResource failed: %v", err)
	}

	// Check the request that was sent
	var request map[string]interface{}
	err = json.Unmarshal(mockTransport.LastSentMessage, &request)
	if err != nil {
		t.Fatalf("Failed to parse request JSON: %v", err)
	}

	test.AssertMethodEquals(t, mockTransport.LastSentMessage, "resources/read")

	// Verify the resource content was parsed correctly
	if resource == nil {
		t.Fatal("Expected resource to be non-nil")
	}

	// Depending on the client implementation, check that resource contains the expected data
}

func TestCallTool_v20250618(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Set up mock response
	mockResponse := test.CreateToolResponse("Tool execution result")
	mockTransport.QueueResponse(mockResponse, nil)

	// Execute a mock tool
	args := map[string]interface{}{
		"param1": "value1",
		"param2": 42,
	}
	result, err := c.CallTool("test-tool", args)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	// Verify result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", result)
	}

	output, ok := resultMap["output"]
	if !ok || output != "Tool execution result" {
		t.Fatalf("Expected output to be 'Tool execution result', got %v", resultMap)
	}
}

func TestGetPrompt_v20250618(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Set up mock response
	mockResponse := test.CreatePromptResponse("Hello {{name}}", "Hello World")
	mockTransport.QueueResponse(mockResponse, nil)

	// Call GetPrompt
	result, err := c.GetPrompt("test-prompt", map[string]interface{}{"name": "World"})
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}

	// Check the request that was sent
	var request map[string]interface{}
	err = json.Unmarshal(mockTransport.LastSentMessage, &request)
	if err != nil {
		t.Fatalf("Failed to parse request JSON: %v", err)
	}

	test.AssertMethodEquals(t, mockTransport.LastSentMessage, "prompts/get")

	// Verify the parameters
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params to be a map, got %T", request["params"])
	}

	if params["name"] != "test-prompt" {
		t.Errorf("Expected prompt name to be 'test-prompt', got %v", params["name"])
	}

	// Verify the result was parsed correctly - now returns concrete PromptResponse type
	if result == nil {
		t.Fatalf("Expected PromptResponse result, got nil")
	}

	if len(result.Messages) == 0 {
		t.Fatalf("Expected at least one message in prompt response")
	}

	// Check that the message was rendered correctly
	firstMessage := result.Messages[0]
	if firstMessage.Content.Text != "Hello World" {
		t.Errorf("Expected rendered text 'Hello World', got %v", firstMessage.Content.Text)
	}
}

// TestRoots_v20250618 tests root management operations in the 2025-06-18 version using correct MCP protocol
func TestRoots_v20250618(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Test add root - should only send notifications/roots/list_changed
	err := c.AddRoot("file:///test/2025-06-18/root", "2025-06-18 Test Root")
	if err != nil {
		t.Fatalf("AddRoot failed: %v", err)
	}

	// Verify that NO roots/add request was sent (this method doesn't exist in MCP)
	addRequests := mockTransport.GetRequestsByMethod("roots/add")
	if len(addRequests) != 0 {
		t.Errorf("Expected 0 roots/add requests (method doesn't exist in MCP), got %d", len(addRequests))
	}

	// Verify that notifications/roots/list_changed was sent (correct MCP behavior)
	// Wait for asynchronous notification to be sent
	if !mockTransport.WaitForNotification("notifications/roots/list_changed", 1*time.Second) {
		t.Fatal("Timeout waiting for roots/list_changed notification")
	}

	notifications := mockTransport.GetRequestsByMethod("notifications/roots/list_changed")
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 roots/list_changed notification, got %d", len(notifications))
	}

	// Test get roots - should read from local cache, no server requests
	roots, err := c.GetRoots()
	if err != nil {
		t.Fatalf("GetRoots failed: %v", err)
	}

	if len(roots) != 1 {
		t.Fatalf("Expected 1 root, got %d", len(roots))
	}

	if roots[0].URI != "file:///test/2025-06-18/root" || roots[0].Name != "2025-06-18 Test Root" {
		t.Errorf("Root doesn't match expected: %+v", roots[0])
	}

	// Verify that NO roots/list request was sent (GetRoots uses local cache)
	listRequests := mockTransport.GetRequestsByMethod("roots/list")
	if len(listRequests) != 0 {
		t.Errorf("Expected 0 roots/list requests (GetRoots uses local cache), got %d", len(listRequests))
	}

	// Test remove root - should only send notifications/roots/list_changed
	err = c.RemoveRoot("file:///test/2025-06-18/root")
	if err != nil {
		t.Fatalf("RemoveRoot failed: %v", err)
	}

	// Verify that NO roots/remove request was sent (this method doesn't exist in MCP)
	removeRequests := mockTransport.GetRequestsByMethod("roots/remove")
	if len(removeRequests) != 0 {
		t.Errorf("Expected 0 roots/remove requests (method doesn't exist in MCP), got %d", len(removeRequests))
	}

	// Wait for the second notification
	if !mockTransport.WaitForNotification("notifications/roots/list_changed", 1*time.Second) {
		t.Fatal("Timeout waiting for second roots/list_changed notification")
	}

	// Verify that a second notifications/roots/list_changed was sent
	notifications = mockTransport.GetRequestsByMethod("notifications/roots/list_changed")
	if len(notifications) != 2 {
		t.Fatalf("Expected 2 roots/list_changed notifications (add + remove), got %d", len(notifications))
	}

	// Verify root was removed from local cache
	roots, err = c.GetRoots()
	if err != nil {
		t.Fatalf("GetRoots failed after remove: %v", err)
	}

	if len(roots) != 0 {
		t.Errorf("Expected 0 roots after removal, got %d", len(roots))
	}
}

// TestClientEvents_v20250618 tests that the Events() method returns the events subject
func TestClientEvents_v20250618(t *testing.T) {
	c, _ := setupTest(t)

	// Test that Events() method returns a non-nil events subject
	events := c.Events()
	if events == nil {
		t.Fatal("Expected Events() to return non-nil events subject")
	}
}
