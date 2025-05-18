package draft

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/client/test"
	"github.com/localrivet/gomcp/mcp"
)

// No need to define MockTransport here as we're using the shared one from client/test/mocktransport.go

func setupDraftMockTransport() *test.MockTransport {
	// Use the test.SetupMockTransport function which properly sets Connected = true
	mockTransport := test.SetupMockTransport("draft")

	// Clear the responses to replace with our custom ones
	mockTransport.ClearResponses()

	// Create custom capabilities for the draft version
	capabilities := map[string]interface{}{
		"roots": map[string]interface{}{
			"listChanged": true,
		},
		"experimental": map[string]interface{}{
			"featureX": true,
		},
	}

	// Queue conditional response for initialize method
	mockTransport.QueueConditionalResponse(
		test.CreateInitializeResponse("draft", capabilities),
		nil,
		test.IsRequestMethod("initialize"),
	)

	// Queue response for resource/get method
	defaultResourceResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2, // Match the ID used by the client
		"result": map[string]interface{}{
			"contents": []interface{}{
				map[string]interface{}{
					"uri":  "/",
					"text": "Root Resource",
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Default resource content",
						},
					},
					"metadata": map[string]interface{}{
						"experimental": true,
						"version":      "draft",
					},
				},
			},
		},
	}
	resourceJSON, _ := json.Marshal(defaultResourceResponse)
	mockTransport.QueueConditionalResponse(
		resourceJSON,
		nil,
		test.IsRequestMethod("resource/get"),
	)

	// Add response for notifications/initialized
	mockTransport.QueueConditionalResponse(
		[]byte(`{"jsonrpc":"2.0","result":null}`),
		nil,
		test.IsRequestMethod("notifications/initialized"),
	)

	// Add default response for tool/call
	defaultToolResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      0, // Will be overridden by actual request ID
		"result": map[string]interface{}{
			"output": "Default tool response",
			"metadata": map[string]interface{}{
				"duration":     10,
				"experimental": true,
			},
		},
	}
	toolJSON, _ := json.Marshal(defaultToolResponse)
	mockTransport.QueueConditionalResponse(
		toolJSON,
		nil,
		test.IsRequestMethod("tool/call"),
	)

	// Set a generic fallback response
	mockTransport.WithDefaultResponse(
		[]byte(`{"jsonrpc":"2.0","id":0,"result":null}`),
		nil,
	)

	return mockTransport
}

// setupTest creates a client with a mock transport and ensures it's initialized
func setupTest(t *testing.T) (client.Client, *test.MockTransport) {
	mockTransport := setupDraftMockTransport()

	// Create a new client with the mock transport
	c, err := client.NewClient("test://server",
		client.WithTransport(mockTransport),
		client.WithVersionDetector(mcp.NewVersionDetector()),
		client.WithExperimentalCapability("featureX", true),
	)
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Verify the correct protocol version was negotiated
	if c.Version() != "draft" {
		t.Fatalf("Expected protocol version draft, got %s", c.Version())
	}

	// Reset the mock transport's response queue
	mockTransport.ClearResponses()

	return c, mockTransport
}

func TestClientInitialization_Draft(t *testing.T) {
	mockTransport := setupDraftMockTransport()

	// Create a new client with the mock transport
	c, err := client.NewClient("test://server",
		client.WithTransport(mockTransport),
		client.WithVersionDetector(mcp.NewVersionDetector()),
		client.WithExperimentalCapability("featureX", true),
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
	if c.Version() != "draft" {
		t.Errorf("Expected protocol version draft, got %s", c.Version())
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

func TestGetResource_Draft(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Set up the mock response for the resource request
	// Use the newest format for draft version
	resourceResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"result": map[string]interface{}{
			"contents": []interface{}{
				map[string]interface{}{
					"uri":  "/test/resource",
					"text": "Test Resource",
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Hello, World!",
						},
					},
					"metadata": map[string]interface{}{
						"experimental": true,
						"version":      "draft",
					},
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

	contents, ok := resultMap["contents"].([]interface{})
	if !ok || len(contents) == 0 {
		t.Fatalf("Expected result to have contents array, got %v", resultMap)
	}

	// Parse the sent request to verify it matches the draft spec
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

func TestCallTool_Draft(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Set up the mock response for the tool request with draft-specific fields
	toolResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"result": map[string]interface{}{
			"output": "Tool executed successfully",
			"metadata": map[string]interface{}{
				"duration":     42,
				"experimental": true,
			},
		},
	}

	responseJSON, _ := json.Marshal(toolResponse)
	mockTransport.QueueResponse(responseJSON, nil)

	// Call the tool
	result, err := c.CallTool("test-tool", map[string]interface{}{
		"param1": "value1",
		"param2": 42,
		"metadata": map[string]interface{}{
			"experimental": true,
		},
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

	// Parse the sent request to verify it matches the draft spec
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

func TestGetPrompt_Draft(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Set up the mock response for the prompt request with draft-specific fields
	promptResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      4,
		"result": map[string]interface{}{
			"prompt":   "Hello, {{name}}! The answer is {{result}}.",
			"rendered": "Hello, World! The answer is 42.",
			"metadata": map[string]interface{}{
				"tokens":       15,
				"experimental": true,
			},
		},
	}

	responseJSON, _ := json.Marshal(promptResponse)
	mockTransport.QueueResponse(responseJSON, nil)

	// Call GetPrompt
	result, err := c.GetPrompt("test-prompt", map[string]interface{}{
		"name":   "World",
		"result": 42,
		"metadata": map[string]interface{}{
			"experimental": true,
		},
	})
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}

	// Verify the result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", result)
	}

	rendered, ok := resultMap["rendered"].(string)
	if !ok || rendered != "Hello, World! The answer is 42." {
		t.Errorf("Expected rendered prompt, got %v", resultMap)
	}

	// Parse the sent request to verify it matches the draft spec
	var sentRequest map[string]interface{}
	if err := json.Unmarshal(mockTransport.LastSentMessage, &sentRequest); err != nil {
		t.Fatalf("Failed to parse sent request: %v", err)
	}

	params, ok := sentRequest["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params in sent request, got %v", sentRequest)
	}

	name, ok := params["name"].(string)
	if !ok || name != "test-prompt" {
		t.Errorf("Expected name parameter to be test-prompt, got %v", params)
	}

	variables, ok := params["variables"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected variables parameter, got %v", params)
	}

	if variables["name"] != "World" || variables["result"] != float64(42) {
		t.Errorf("Expected variable values to match, got %v", variables)
	}

	method, ok := sentRequest["method"].(string)
	if !ok || method != "prompt/get" {
		t.Errorf("Expected method to be prompt/get, got %v", sentRequest)
	}
}

// TestRoots_Draft tests root management operations in the draft version
func TestRoots_Draft(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Test add root
	addRootResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result":  map[string]interface{}{},
	}
	addRootJSON, _ := json.Marshal(addRootResponse)
	mockTransport.QueueResponse(addRootJSON, nil)

	err := c.AddRoot("/test/draft/root", "Draft Test Root")
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

	if addParams["uri"] != "/test/draft/root" || addParams["name"] != "Draft Test Root" {
		t.Errorf("Add root params not as expected: %v", addParams)
	}

	// Test get roots
	getRootsResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"result": map[string]interface{}{
			"roots": []interface{}{
				map[string]interface{}{
					"uri":  "/test/draft/root",
					"name": "Draft Test Root",
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

	if roots[0].URI != "/test/draft/root" || roots[0].Name != "Draft Test Root" {
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

	err = c.RemoveRoot("/test/draft/root")
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

	if removeParams["uri"] != "/test/draft/root" {
		t.Errorf("Remove root params not as expected: %v", removeParams)
	}
}
