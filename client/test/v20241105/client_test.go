package v20241105

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/client/test"
	"github.com/localrivet/gomcp/mcp"
)

// No need to define MockTransport here as we're using the shared one from client/test/mocktransport.go

// setupTest creates a client with a mock transport and ensures it's initialized
func setupTest(t *testing.T) (client.Client, *test.MockTransport) {
	mockTransport := test.SetupMockTransport("2024-11-05")

	// Create a new client with the mock transport
	c, err := client.NewClient("test://server",
		client.WithTransport(mockTransport),
		client.WithVersionDetector(mcp.NewVersionDetector()),
	)
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Verify the correct protocol version was negotiated
	if c.Version() != "2024-11-05" {
		t.Fatalf("Expected protocol version 2024-11-05, got %s", c.Version())
	}

	// Reset the mock transport's response queue
	mockTransport.ClearResponses()

	return c, mockTransport
}

func TestClientInitialization_v20241105(t *testing.T) {
	mockTransport := test.SetupMockTransport("2024-11-05")

	// Create a new client with the mock transport
	c, err := client.NewClient("test://server",
		client.WithTransport(mockTransport),
		client.WithVersionDetector(mcp.NewVersionDetector()),
	)
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// The Connect method will be automatically called when needed
	// Just verify the client is in the expected state
	if !mockTransport.ConnectCalled {
		// Manually call a method that should trigger a connection
		_, err := c.GetResource("/")
		if err != nil {
			t.Fatalf("Failed to trigger connection: %v", err)
		}

		if !mockTransport.ConnectCalled {
			t.Error("Connect was not called on the transport")
		}
	}

	// Verify the client is connected
	if !c.IsConnected() {
		t.Error("Client should be connected")
	}

	// Verify the client is initialized
	if !c.IsInitialized() {
		t.Error("Client should be initialized")
	}

	// Verify the correct protocol version was negotiated
	if c.Version() != "2024-11-05" {
		t.Errorf("Expected protocol version 2024-11-05, got %s", c.Version())
	}

	// Test closing the client
	if err := c.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Verify the transport was disconnected
	if !mockTransport.DisconnectCalled {
		t.Error("Disconnect was not called on the transport")
	}

	// Verify the client is no longer connected
	if c.IsConnected() {
		t.Error("Client should not be connected after close")
	}
}

func TestGetResource_v20241105(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Set up the mock response for the resource request
	// Note: In 2024-11-05, the resource response has a different structure from 2025-03-26
	resourceResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"result": map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Hello, World!",
				},
			},
		},
	}

	responseJSON, _ := json.Marshal(resourceResponse)
	mockTransport.QueueResponse(responseJSON, nil)

	// Call GetResource
	result, err := c.GetResource("/test/resource")
	if err != nil {
		t.Fatalf("GetResource failed: %v", err)
	}

	// Verify the result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", result)
	}

	content, ok := resultMap["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("Expected result to have content array, got %v", resultMap)
	}

	// Parse the sent request to verify it matches the 2024-11-05 spec
	var sentRequest map[string]interface{}
	if err := json.Unmarshal(mockTransport.LastSentMessage, &sentRequest); err != nil {
		t.Fatalf("Failed to parse sent request: %v", err)
	}

	params, ok := sentRequest["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params in sent request, got %v", sentRequest)
	}

	path, ok := params["path"].(string)
	if !ok || path != "/test/resource" {
		t.Errorf("Expected path parameter to be /test/resource, got %v", params)
	}

	method, ok := sentRequest["method"].(string)
	if !ok || method != "resource/get" {
		t.Errorf("Expected method to be resource/get, got %v", sentRequest)
	}
}

func TestCallTool_v20241105(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Set up the mock response for the tool request
	toolResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"result": map[string]interface{}{
			"output": "Tool executed successfully",
		},
	}

	responseJSON, _ := json.Marshal(toolResponse)
	mockTransport.QueueResponse(responseJSON, nil)

	// Call the tool
	result, err := c.CallTool("test-tool", map[string]interface{}{
		"param1": "value1",
		"param2": 42,
	})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	// Verify the result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", result)
	}

	output, ok := resultMap["output"].(string)
	if !ok || output != "Tool executed successfully" {
		t.Errorf("Expected output to be 'Tool executed successfully', got %v", resultMap)
	}

	// Parse the sent request to verify it matches the 2024-11-05 spec
	var sentRequest map[string]interface{}
	if err := json.Unmarshal(mockTransport.LastSentMessage, &sentRequest); err != nil {
		t.Fatalf("Failed to parse sent request: %v", err)
	}

	params, ok := sentRequest["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params in sent request, got %v", sentRequest)
	}

	name, ok := params["name"].(string)
	if !ok || name != "test-tool" {
		t.Errorf("Expected name parameter to be test-tool, got %v", params)
	}

	args, ok := params["arguments"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected arguments parameter, got %v", params)
	}

	if args["param1"] != "value1" || args["param2"] != float64(42) {
		t.Errorf("Expected argument values to match, got %v", args)
	}

	method, ok := sentRequest["method"].(string)
	if !ok || method != "tool/execute" {
		t.Errorf("Expected method to be tool/execute, got %v", sentRequest)
	}
}

func TestGetPrompt_v20241105(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Set up mock response
	mockResponse := test.CreatePromptResponse("Hello {{name}}", "Hello Test User")
	mockTransport.QueueResponse(mockResponse, nil)

	// Call the method
	result, err := c.GetPrompt("test-prompt", map[string]interface{}{
		"name": "Test User",
	})
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}

	// Verify the result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}

	rendered, ok := resultMap["rendered"].(string)
	if !ok || rendered != "Hello Test User" {
		t.Errorf("Expected rendered text 'Hello Test User', got %v", resultMap)
	}
}

// TestRoots_v20241105 tests root management operations in the 2024-11-05 version
func TestRoots_v20241105(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Test add root
	addRootResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result":  map[string]interface{}{},
	}
	addRootJSON, _ := json.Marshal(addRootResponse)
	mockTransport.QueueResponse(addRootJSON, nil)

	err := c.AddRoot("/test/2024-11-05/root", "2024-11-05 Test Root")
	if err != nil {
		t.Fatalf("AddRoot failed: %v", err)
	}

	// Verify the add request format
	var addRequest map[string]interface{}
	if err := json.Unmarshal(mockTransport.LastSentMessage, &addRequest); err != nil {
		t.Fatalf("Failed to parse add request: %v", err)
	}

	if addRequest["method"] != "roots/add" {
		t.Errorf("Expected method roots/add, got %v", addRequest["method"])
	}

	addParams, ok := addRequest["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params in add request, got %v", addRequest)
	}

	if addParams["uri"] != "/test/2024-11-05/root" || addParams["name"] != "2024-11-05 Test Root" {
		t.Errorf("Add root params not as expected: %v", addParams)
	}

	// Test get roots
	getRootsResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"result": map[string]interface{}{
			"roots": []interface{}{
				map[string]interface{}{
					"uri":  "/test/2024-11-05/root",
					"name": "2024-11-05 Test Root",
				},
			},
		},
	}
	getRootsJSON, _ := json.Marshal(getRootsResponse)
	mockTransport.QueueResponse(getRootsJSON, nil)

	roots, err := c.GetRoots()
	if err != nil {
		t.Fatalf("GetRoots failed: %v", err)
	}

	if len(roots) != 1 {
		t.Fatalf("Expected 1 root, got %d", len(roots))
	}

	if roots[0].URI != "/test/2024-11-05/root" || roots[0].Name != "2024-11-05 Test Root" {
		t.Errorf("Root doesn't match expected: %+v", roots[0])
	}

	// Verify the get roots request format
	var getRootsRequest map[string]interface{}
	if err := json.Unmarshal(mockTransport.LastSentMessage, &getRootsRequest); err != nil {
		t.Fatalf("Failed to parse get roots request: %v", err)
	}

	if getRootsRequest["method"] != "roots/list" {
		t.Errorf("Expected method roots/list, got %v", getRootsRequest["method"])
	}

	// Test remove root
	removeRootResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"result":  map[string]interface{}{},
	}
	removeRootJSON, _ := json.Marshal(removeRootResponse)
	mockTransport.QueueResponse(removeRootJSON, nil)

	err = c.RemoveRoot("/test/2024-11-05/root")
	if err != nil {
		t.Fatalf("RemoveRoot failed: %v", err)
	}

	// Verify the remove request format
	var removeRequest map[string]interface{}
	if err := json.Unmarshal(mockTransport.LastSentMessage, &removeRequest); err != nil {
		t.Fatalf("Failed to parse remove request: %v", err)
	}

	if removeRequest["method"] != "roots/remove" {
		t.Errorf("Expected method roots/remove, got %v", removeRequest["method"])
	}

	removeParams, ok := removeRequest["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params in remove request, got %v", removeRequest)
	}

	if removeParams["uri"] != "/test/2024-11-05/root" {
		t.Errorf("Remove root params not as expected: %v", removeParams)
	}
}
