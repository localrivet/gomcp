package test

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/client"
)

// TestFixture defines a reusable test scenario that can be used across multiple tests
type TestFixture struct {
	Name        string                           // Name of the fixture for identification
	Description string                           // Description of what the fixture is for
	Setup       func(*testing.T, *MockTransport) // Setup function to prepare the fixture
	Teardown    func(*testing.T, *MockTransport) // Teardown function to clean up
	Verify      func(*testing.T, client.Client)  // Function to verify the fixture works as expected
	Data        map[string]interface{}           // Data associated with the fixture
	Responses   [][]byte                         // Pre-defined responses
}

// CommonFixtureSetups defines setup functions to avoid circular dependencies
var CommonFixtureSetups = map[string]func(*testing.T, *MockTransport){
	"BasicClient":        setupBasicClient,
	"ResourceServer":     setupResourceServer,
	"ToolServer":         setupToolServer,
	"VersionNegotiation": setupVersionNegotiation,
}

// CommonFixtures contains a set of commonly used test fixtures
var CommonFixtures = map[string]TestFixture{
	"BasicClient": {
		Name:        "BasicClient",
		Description: "Basic client setup with initialization response",
		Setup:       setupBasicClient,
		Verify: func(t *testing.T, c client.Client) {
			// Verify client is initialized
			if !c.IsInitialized() {
				t.Fatal("Client should be initialized")
			}
		},
	},
	"ResourceServer": {
		Name:        "ResourceServer",
		Description: "Server with sample resources for testing resource operations",
		Setup:       setupResourceServer,
		Verify: func(t *testing.T, c client.Client) {
			// Verify we can retrieve a resource
			result, err := c.GetResource("/test/doc1.md")
			if err != nil {
				t.Fatalf("Failed to get resource: %v", err)
			}

			if result == nil {
				t.Fatal("Resource response should not be nil")
			}
		},
		Data: map[string]interface{}{
			"resourcePaths": []string{
				"/test/doc1.md",
				"/test/doc2.md",
				"/test/image.jpg",
				"/test/data.json",
			},
		},
	},
	"ToolServer": {
		Name:        "ToolServer",
		Description: "Server with sample tools for testing tool operations",
		Setup:       setupToolServer,
		Verify: func(t *testing.T, c client.Client) {
			// Verify we can call a tool
			result, err := c.CallTool("calculator", map[string]interface{}{})
			if err != nil {
				t.Fatalf("Failed to call tool: %v", err)
			}

			if result == nil {
				t.Fatal("Tool response should not be nil")
			}
		},
		Data: map[string]interface{}{
			"tools": []string{
				"calculator",
				"echo",
				"list",
				"error",
			},
		},
	},
	"VersionNegotiation": {
		Name:        "VersionNegotiation",
		Description: "Fixture for testing version negotiation and fallback",
		Setup:       setupVersionNegotiation,
		Verify: func(t *testing.T, c client.Client) {
			// Nothing to verify before the actual test
		},
		Data: map[string]interface{}{
			"serverVersions": map[string][]string{
				"newest":     {"draft", "2024-11-05", "2025-03-26"},
				"legacy":     {"draft", "2024-11-05"},
				"draftOnly":  {"draft"},
				"middleOnly": {"2024-11-05"},
				"newestOnly": {"2025-03-26"},
			},
		},
	},
}

// Setup functions for fixtures

func setupBasicClient(t *testing.T, m *MockTransport) {
	// Add a successful initialization response to the queue
	initResp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"serverInfo": map[string]interface{}{
				"name":    "TestServer",
				"version": "1.0.0",
			},
			"capabilities": map[string]interface{}{
				"roots": map[string]interface{}{
					"listChanged": true,
				},
				"enhancedResources": true,
			},
		},
	}

	initJSON, err := json.Marshal(initResp)
	if err != nil {
		t.Fatalf("Failed to marshal init response: %v", err)
	}

	m.QueueResponse(initJSON, nil)
}

func setupResourceServer(t *testing.T, m *MockTransport) {
	// Set up basic client first
	setupBasicClient(t, m)

	// Create some sample resources
	resources := []struct {
		path    string
		content string
	}{
		{"/test/doc1.md", "# Test Document 1\n\nThis is a test document."},
		{"/test/doc2.md", "# Test Document 2\n\nAnother test document."},
		{"/test/image.jpg", "mock-image-data"},
		{"/test/data.json", `{"key":"value","nested":{"array":[1,2,3]}}`},
	}

	// Queue responses for each resource
	for _, res := range resources {
		m.QueueConditionalResponse(
			CreateResourceResponse("2025-03-26", res.content),
			nil,
			func(req []byte) bool {
				var request map[string]interface{}
				if err := json.Unmarshal(req, &request); err != nil {
					return false
				}

				if method, ok := request["method"].(string); !ok || method != "resource/get" {
					return false
				}

				params, ok := request["params"].(map[string]interface{})
				if !ok {
					return false
				}

				path, ok := params["path"].(string)
				return ok && path == res.path
			},
		)
	}
}

func setupToolServer(t *testing.T, m *MockTransport) {
	// Set up basic client first
	setupBasicClient(t, m)

	// Create some sample tools
	tools := []struct {
		name   string
		result interface{}
	}{
		{"calculator", map[string]interface{}{
			"result": 42,
		}},
		{"echo", "Hello, world!"},
		{"list", []string{"item1", "item2", "item3"}},
		{"error", nil}, // Special case for error tool
	}

	// Queue responses for each tool
	for _, tool := range tools {
		if tool.name == "error" {
			// Special case for error tool
			errorResp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"error": map[string]interface{}{
					"code":    400,
					"message": "Bad Request",
					"data": map[string]interface{}{
						"reason": "Invalid arguments",
					},
				},
			}

			errorJSON, _ := json.Marshal(errorResp)

			m.QueueConditionalResponse(
				errorJSON,
				nil,
				func(req []byte) bool {
					return isToolNameInRequest(req, "error")
				},
			)
		} else {
			m.QueueConditionalResponse(
				CreateToolResponse(tool.result),
				nil,
				func(req []byte) bool {
					return isToolNameInRequest(req, tool.name)
				},
			)
		}
	}
}

func setupVersionNegotiation(t *testing.T, m *MockTransport) {
	// Don't set up basic client, this fixture is specifically for
	// version negotiation testing

	// Clear any existing responses
	m.ClearResponses()
}

// isToolNameInRequest checks if the given tool name is in the request
func isToolNameInRequest(req []byte, toolName string) bool {
	var request map[string]interface{}
	if err := json.Unmarshal(req, &request); err != nil {
		return false
	}

	if method, ok := request["method"].(string); !ok || method != "tool/execute" {
		return false
	}

	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return false
	}

	name, ok := params["name"].(string)
	return ok && name == toolName
}

// isRequestMethod checks if a request has the given method
func isRequestMethod(req []byte, method string) bool {
	var request map[string]interface{}
	if err := json.Unmarshal(req, &request); err != nil {
		return false
	}

	reqMethod, ok := request["method"].(string)
	return ok && reqMethod == method
}

// SetupFixture sets up a named fixture and returns a configured client
func SetupFixture(t *testing.T, fixtureName string, version string) (client.Client, *MockTransport) {
	fixture, ok := CommonFixtures[fixtureName]
	if !ok {
		t.Fatalf("Unknown fixture: %s", fixtureName)
	}

	c, m := SetupClientWithMockTransport(t, version)

	fixture.Setup(t, m)

	return c, m
}

// VerifyFixture verifies a named fixture
func VerifyFixture(t *testing.T, fixtureName string, c client.Client) {
	fixture, ok := CommonFixtures[fixtureName]
	if !ok {
		t.Fatalf("Unknown fixture: %s", fixtureName)
	}

	fixture.Verify(t, c)
}

// GetFixtureData gets data from a named fixture
func GetFixtureData(t *testing.T, fixtureName string) map[string]interface{} {
	fixture, ok := CommonFixtures[fixtureName]
	if !ok {
		t.Fatalf("Unknown fixture: %s", fixtureName)
	}

	return fixture.Data
}
