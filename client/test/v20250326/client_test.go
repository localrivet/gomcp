package v20250326

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/client/test"
)

// No need to define MockTransport here as we're using the shared one from client/test/mocktransport.go

// setupTest creates a client with a mock transport and ensures it's initialized
func setupTest(t *testing.T) (client.Client, *test.MockTransport) {
	return test.SetupClientWithMockTransport(t, "2025-03-26")
}

func TestClientInitialization_v20250326(t *testing.T) {
	// Simply test that the setupTest function works correctly
	c, mockTransport := setupTest(t)

	// Verify that the client has proper version
	if c.Version() != "2025-03-26" {
		t.Errorf("Expected version 2025-03-26, got %s", c.Version())
	}

	// Verify that the transport was connected
	if !mockTransport.ConnectCalled {
		t.Error("Connect was not called on the transport")
	}
}

func TestGetResource_v20250326(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Set up mock response
	mockResponse := test.CreateResourceResponse("2025-03-26", "Hello, World!")
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

	test.AssertMethodEquals(t, mockTransport.LastSentMessage, "resource/get")

	// Verify the resource content was parsed correctly
	if resource == nil {
		t.Fatal("Expected resource to be non-nil")
	}

	// Depending on the client implementation, check that resource contains the expected data
}

func TestCallTool_v20250326(t *testing.T) {
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

func TestGetPrompt_v20250326(t *testing.T) {
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

	test.AssertMethodEquals(t, mockTransport.LastSentMessage, "prompt/get")

	// Verify the parameters
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params to be a map, got %T", request["params"])
	}

	if params["name"] != "test-prompt" {
		t.Errorf("Expected prompt name to be 'test-prompt', got %v", params["name"])
	}

	// Verify the result was parsed correctly
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", result)
	}

	prompt, ok := resultMap["prompt"].(string)
	if !ok || prompt != "Hello {{name}}" {
		t.Errorf("Expected prompt 'Hello {{name}}', got %v", prompt)
	}

	rendered, ok := resultMap["rendered"].(string)
	if !ok || rendered != "Hello World" {
		t.Errorf("Expected rendered 'Hello World', got %v", rendered)
	}
}

// TestRoots_v20250326 tests root management operations in the 2025-03-26 version
func TestRoots_v20250326(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Test add root
	addRootResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result":  map[string]interface{}{},
	}
	addRootJSON, _ := json.Marshal(addRootResponse)
	mockTransport.QueueResponse(addRootJSON, nil)

	err := c.AddRoot("/test/2025-03-26/root", "2025-03-26 Test Root")
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

	if addParams["uri"] != "/test/2025-03-26/root" || addParams["name"] != "2025-03-26 Test Root" {
		t.Errorf("Add root params not as expected: %v", addParams)
	}

	// Test get roots
	getRootsResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"result": map[string]interface{}{
			"roots": []interface{}{
				map[string]interface{}{
					"uri":  "/test/2025-03-26/root",
					"name": "2025-03-26 Test Root",
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

	if roots[0].URI != "/test/2025-03-26/root" || roots[0].Name != "2025-03-26 Test Root" {
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

	err = c.RemoveRoot("/test/2025-03-26/root")
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

	if removeParams["uri"] != "/test/2025-03-26/root" {
		t.Errorf("Remove root params not as expected: %v", removeParams)
	}
}
