package test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/stretchr/testify/assert"
)

// TestListResources tests the resource discovery functionality
func TestListResources(t *testing.T) {
	// Create client with mock transport
	c, m := SetupClientWithMockTransport(t, "2025-03-26")
	defer c.Close()

	// Add resources/list response
	resourcesResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2, // ID 1 is used for initialization
		"result": map[string]interface{}{
			"resources": []interface{}{
				map[string]interface{}{
					"uri":         "/files/readme.txt",
					"name":        "README File",
					"description": "Project documentation",
					"mimeType":    "text/plain",
					"annotations": map[string]interface{}{
						"category": "documentation",
						"size":     1024,
					},
				},
				map[string]interface{}{
					"uri":         "/api/users",
					"name":        "Users API",
					"description": "REST API endpoint for user management",
					"mimeType":    "application/json",
					"annotations": map[string]interface{}{
						"category": "api",
						"methods":  []string{"GET", "POST"},
					},
				},
			},
		},
	}

	resourcesJSON, err := json.Marshal(resourcesResponse)
	if err != nil {
		t.Fatalf("Failed to marshal resources response: %v", err)
	}

	// Queue the response for resources/list request
	m.QueueConditionalResponse(
		resourcesJSON,
		nil,
		func(req []byte) bool {
			return isRequestMethod(req, "resources/list")
		},
	)

	// Test ListResources
	resources, err := c.ListResources()
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	// Verify we got the expected resources
	if len(resources) != 2 {
		t.Fatalf("Expected 2 resources, got %d", len(resources))
	}

	// Check that resources have the expected structure
	expectedUris := []string{"/files/readme.txt", "/api/users"}
	for i, resource := range resources {
		if resource.URI != expectedUris[i] {
			t.Errorf("Expected resource %d to have URI %s, got %s", i, expectedUris[i], resource.URI)
		}
		if resource.URI == "" {
			t.Error("Resource URI should not be empty")
		}
		if resource.Name == "" {
			t.Error("Resource name should not be empty")
		}
		if resource.Annotations == nil {
			t.Error("Resource annotations should not be nil")
		}

		// Check specific fields
		if resource.URI == "/files/readme.txt" {
			if resource.MimeType != "text/plain" {
				t.Errorf("Expected README file to have mimeType 'text/plain', got '%s'", resource.MimeType)
			}
			if resource.Description != "Project documentation" {
				t.Errorf("Expected README file to have description 'Project documentation', got '%s'", resource.Description)
			}
		}
	}

	t.Logf("Successfully retrieved %d resources", len(resources))
	for _, resource := range resources {
		t.Logf("Resource: %s (%s) - %s", resource.Name, resource.URI, resource.Description)
	}
}

// TestListResourcesWithPagination tests resource discovery with pagination
func TestListResourcesWithPagination(t *testing.T) {
	// Create client with mock transport
	c, m := SetupClientWithMockTransport(t, "2025-03-26")
	defer c.Close()

	// Add first page response
	firstPageResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"result": map[string]interface{}{
			"resources": []interface{}{
				map[string]interface{}{
					"uri":         "/resource1",
					"name":        "Resource 1",
					"description": "First resource",
					"mimeType":    "text/plain",
				},
			},
			"nextCursor": "page2",
		},
	}

	// Add second page response
	secondPageResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"result": map[string]interface{}{
			"resources": []interface{}{
				map[string]interface{}{
					"uri":         "/resource2",
					"name":        "Resource 2",
					"description": "Second resource",
					"mimeType":    "application/json",
				},
			},
			// No nextCursor means this is the last page
		},
	}

	firstPageJSON, _ := json.Marshal(firstPageResponse)
	secondPageJSON, _ := json.Marshal(secondPageResponse)

	// Queue responses for pagination
	m.QueueConditionalResponse(
		firstPageJSON,
		nil,
		func(req []byte) bool {
			var request map[string]interface{}
			if err := json.Unmarshal(req, &request); err != nil {
				return false
			}

			if method, ok := request["method"].(string); !ok || method != "resources/list" {
				return false
			}

			// First request should have no cursor or empty cursor
			params, ok := request["params"].(map[string]interface{})
			if !ok {
				return true // No params means first request
			}

			cursor, exists := params["cursor"]
			return !exists || cursor == ""
		},
	)

	m.QueueConditionalResponse(
		secondPageJSON,
		nil,
		func(req []byte) bool {
			var request map[string]interface{}
			if err := json.Unmarshal(req, &request); err != nil {
				return false
			}

			if method, ok := request["method"].(string); !ok || method != "resources/list" {
				return false
			}

			// Second request should have cursor "page2"
			params, ok := request["params"].(map[string]interface{})
			if !ok {
				return false
			}

			cursor, exists := params["cursor"]
			return exists && cursor == "page2"
		},
	)

	// Test ListResources with pagination
	resources, err := c.ListResources()
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	// Verify we got resources from both pages
	if len(resources) != 2 {
		t.Fatalf("Expected 2 resources (1 from each page), got %d", len(resources))
	}

	// Verify the resources are in the expected order
	if resources[0].URI != "/resource1" {
		t.Errorf("Expected first resource to be /resource1, got %s", resources[0].URI)
	}
	if resources[1].URI != "/resource2" {
		t.Errorf("Expected second resource to be /resource2, got %s", resources[1].URI)
	}

	t.Logf("Successfully retrieved %d resources across pages", len(resources))
}

// TestListPrompts tests the prompt discovery functionality
func TestListPrompts(t *testing.T) {
	// Create client with mock transport
	c, m := SetupClientWithMockTransport(t, "2025-03-26")
	defer c.Close()

	// Add prompts/list response
	promptsResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2, // ID 1 is used for initialization
		"result": map[string]interface{}{
			"prompts": []interface{}{
				map[string]interface{}{
					"name":        "greeting",
					"description": "Generate a personalized greeting",
					"arguments": []interface{}{
						map[string]interface{}{
							"name":        "name",
							"description": "The person's name to greet",
							"required":    true,
						},
						map[string]interface{}{
							"name":        "formal",
							"description": "Whether to use formal greeting",
							"required":    false,
						},
					},
				},
				map[string]interface{}{
					"name":        "summary",
					"description": "Summarize the given text",
					"arguments": []interface{}{
						map[string]interface{}{
							"name":        "text",
							"description": "The text to summarize",
							"required":    true,
						},
						map[string]interface{}{
							"name":        "length",
							"description": "Maximum length of summary",
							"required":    false,
						},
					},
				},
			},
		},
	}

	promptsJSON, err := json.Marshal(promptsResponse)
	if err != nil {
		t.Fatalf("Failed to marshal prompts response: %v", err)
	}

	// Queue the response for prompts/list request
	m.QueueConditionalResponse(
		promptsJSON,
		nil,
		func(req []byte) bool {
			return isRequestMethod(req, "prompts/list")
		},
	)

	// Test ListPrompts
	prompts, err := c.ListPrompts()
	if err != nil {
		t.Fatalf("Failed to list prompts: %v", err)
	}

	// Verify we got the expected prompts
	if len(prompts) != 2 {
		t.Fatalf("Expected 2 prompts, got %d", len(prompts))
	}

	// Check that prompts have the expected structure
	expectedNames := []string{"greeting", "summary"}
	for i, prompt := range prompts {
		if prompt.Name != expectedNames[i] {
			t.Errorf("Expected prompt %d to be %s, got %s", i, expectedNames[i], prompt.Name)
		}
		if prompt.Name == "" {
			t.Error("Prompt name should not be empty")
		}
		if prompt.Arguments == nil {
			t.Error("Prompt arguments should not be nil")
		}

		// Check specific prompt details
		if prompt.Name == "greeting" {
			if len(prompt.Arguments) != 2 {
				t.Errorf("Expected greeting prompt to have 2 arguments, got %d", len(prompt.Arguments))
			}
			// Check first argument
			if len(prompt.Arguments) > 0 {
				arg := prompt.Arguments[0]
				if arg.Name != "name" {
					t.Errorf("Expected first argument to be 'name', got '%s'", arg.Name)
				}
				if !arg.Required {
					t.Error("Expected 'name' argument to be required")
				}
			}
			// Check second argument
			if len(prompt.Arguments) > 1 {
				arg := prompt.Arguments[1]
				if arg.Name != "formal" {
					t.Errorf("Expected second argument to be 'formal', got '%s'", arg.Name)
				}
				if arg.Required {
					t.Error("Expected 'formal' argument to be optional")
				}
			}
		}
	}

	t.Logf("Successfully retrieved %d prompts", len(prompts))
	for _, prompt := range prompts {
		t.Logf("Prompt: %s - %s (%d arguments)", prompt.Name, prompt.Description, len(prompt.Arguments))
	}
}

// TestWithResourceParams tests the enhanced resource access with parameters
func TestWithResourceParams(t *testing.T) {
	// Create client with mock transport
	c, m := SetupClientWithMockTransport(t, "2025-03-26")
	defer c.Close()

	// Expected resource parameters
	expectedParams := map[string]interface{}{
		"include_posts": true,
		"limit":         50,
		"format":        "json",
	}

	// Add resource response
	resourceResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3, // Match the actual request ID
		"result": map[string]interface{}{
			"contents": []interface{}{
				map[string]interface{}{
					"uri":  "/api/users",
					"text": "Users API with parameters",
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": `{"users": [{"id": 1, "name": "John", "posts": []}], "total": 1}`,
						},
					},
				},
			},
		},
	}

	resourceJSON, err := json.Marshal(resourceResponse)
	if err != nil {
		t.Fatalf("Failed to marshal resource response: %v", err)
	}

	// Queue the response for resources/read request
	m.QueueConditionalResponse(
		resourceJSON,
		nil,
		func(req []byte) bool {
			return isRequestMethod(req, "resources/read")
		},
	)

	// Test GetResource with parameters
	result, err := c.GetResource("/api/users",
		client.WithResourceParams(expectedParams),
		client.WithRequestTimeoutOption(5*time.Second),
	)

	if err != nil {
		t.Fatalf("Failed to get resource with parameters: %v", err)
	}

	// Verify we got a result
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify the request was sent with correct parameters
	history := m.GetRequestHistory()

	// Filter for only resources/read requests
	var resourceRequests []RequestRecord
	for _, req := range history {
		var request map[string]interface{}
		if err := json.Unmarshal(req.Message, &request); err != nil {
			continue
		}
		if method, ok := request["method"].(string); ok && method == "resources/read" {
			resourceRequests = append(resourceRequests, req)
		}
	}

	// We expect 1 resources/read request:
	// The setup function calls GetResource("/") but then calls ClearHistory(), so only our test request should be counted
	if len(resourceRequests) != 1 {
		t.Fatalf("Expected 1 resources/read request (from test), got %d (total requests: %d)", len(resourceRequests), len(history))
	}

	var request map[string]interface{}
	if err := json.Unmarshal(resourceRequests[0].Message, &request); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	params, ok := request["params"].(map[string]interface{})
	if !ok {
		t.Fatal("Request should have params")
	}

	// Verify all expected parameters are present
	for key, expectedValue := range expectedParams {
		actualValue, exists := params[key]
		if !exists {
			t.Errorf("Expected parameter '%s' not found in request", key)
		} else {
			// Convert values to strings for comparison to handle JSON number conversion
			expectedStr := fmt.Sprintf("%v", expectedValue)
			actualStr := fmt.Sprintf("%v", actualValue)
			if expectedStr != actualStr {
				t.Errorf("Expected parameter '%s' to be %v (type %T), got %v (type %T)",
					key, expectedValue, expectedValue, actualValue, actualValue)
			}
		}
	}

	// Verify URI is present
	if uri, ok := params["uri"].(string); !ok || uri != "/api/users" {
		t.Errorf("Expected URI to be '/api/users', got %v", params["uri"])
	}

	t.Logf("Successfully retrieved resource with parameters: %+v", result)
}

// TestWithResourceParamsBackwardCompatibility tests that GetResource still works without parameters
func TestWithResourceParamsBackwardCompatibility(t *testing.T) {
	// Create client with mock transport
	c, m := SetupClientWithMockTransport(t, "2025-03-26")
	defer c.Close()

	// Add resource response
	resourceResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"result": map[string]interface{}{
			"contents": []interface{}{
				map[string]interface{}{
					"uri":  "/files/readme.txt",
					"text": "README File",
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "# Project README\nThis is a test project.",
						},
					},
				},
			},
		},
	}

	resourceJSON, err := json.Marshal(resourceResponse)
	if err != nil {
		t.Fatalf("Failed to marshal resource response: %v", err)
	}

	// Queue the response and verify only URI parameter is sent
	m.QueueConditionalResponse(
		resourceJSON,
		nil,
		func(req []byte) bool {
			var request map[string]interface{}
			if err := json.Unmarshal(req, &request); err != nil {
				return false
			}

			// Check method
			if method, ok := request["method"].(string); !ok || method != "resources/read" {
				return false
			}

			// Check params
			params, ok := request["params"].(map[string]interface{})
			if !ok {
				return false
			}

			// Should only have URI parameter
			if len(params) != 1 {
				return false
			}

			// Check URI
			if uri, ok := params["uri"].(string); !ok || uri != "/files/readme.txt" {
				return false
			}

			return true
		},
	)

	// Test GetResource without parameters (backward compatibility)
	result, err := c.GetResource("/files/readme.txt")
	if err != nil {
		t.Fatalf("Failed to get resource without parameters: %v", err)
	}

	// Verify we got a result
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	t.Logf("Successfully retrieved resource without parameters (backward compatibility): %+v", result)
}

// TestPing tests the ping functionality
func TestPing(t *testing.T) {
	// Create client with mock transport
	c, m := SetupClientWithMockTransport(t, "2025-03-26")
	defer c.Close()

	// Add ping response (empty result as per MCP spec)
	pingResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"result":  map[string]interface{}{},
	}

	pingJSON, err := json.Marshal(pingResponse)
	if err != nil {
		t.Fatalf("Failed to marshal ping response: %v", err)
	}

	// Queue the response for ping request
	m.QueueConditionalResponse(
		pingJSON,
		nil,
		func(req []byte) bool {
			var request map[string]interface{}
			if err := json.Unmarshal(req, &request); err != nil {
				return false
			}

			// Check method
			if method, ok := request["method"].(string); !ok || method != "ping" {
				return false
			}

			// Ping should have no parameters (or null params)
			params, exists := request["params"]
			if exists && params != nil {
				// If params exist, they should be empty/null
				if paramsMap, ok := params.(map[string]interface{}); ok && len(paramsMap) > 0 {
					return false
				}
			}

			return true
		},
	)

	// Test Ping
	err = c.Ping()
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	t.Log("Ping successful")
}

// Create mock client with capabilities
func TestServerCapabilityDiscovery(t *testing.T) {
	// Create client with mock transport - we'll need to modify the initialize response
	c, _ := SetupClientWithMockTransport(t, "2025-03-26")
	defer c.Close()

	// Since we can't easily modify the initialize response in the existing test setup,
	// let's verify that the methods exist and work when server capabilities are not set
	// For now, test the nil handling behavior until we can set up proper mocking

	// Test that methods return appropriate nil/empty values when no capabilities are set
	caps := c.GetServerCapabilities()
	// Note: This might be nil depending on the mock setup

	info := c.GetServerInfo()
	// Note: This might be nil depending on the mock setup

	instructions := c.GetServerInstructions()
	assert.Equal(t, "", instructions)

	// Test capability checking methods work without crashing
	assert.False(t, c.HasCapability("nonexistent"))

	// These should all return false if capabilities are nil
	toolsSupported := c.HasCapability("tools")
	resourcesSupported := c.HasCapability("resources")
	promptsSupported := c.HasCapability("prompts")
	loggingSupported := c.HasCapability("logging")
	experimentalSupported := c.HasCapability("experimental")

	// Log what we found for debugging
	t.Logf("Server capabilities: %+v", caps)
	t.Logf("Server info: %+v", info)
	t.Logf("Server instructions: %q", instructions)
	t.Logf("Tools supported: %v", toolsSupported)
	t.Logf("Resources supported: %v", resourcesSupported)
	t.Logf("Prompts supported: %v", promptsSupported)
	t.Logf("Logging supported: %v", loggingSupported)
	t.Logf("Experimental supported: %v", experimentalSupported)

	// Test SupportsResourceSubscriptions works without crashing
	resourceSubscriptions := c.SupportsResourceSubscriptions()
	t.Logf("Resource subscriptions supported: %v", resourceSubscriptions)

	// Test SupportsListChangedNotifications works without crashing
	toolsListChanged := c.SupportsListChangedNotifications("tools")
	resourcesListChanged := c.SupportsListChangedNotifications("resources")
	promptsListChanged := c.SupportsListChangedNotifications("prompts")
	nonexistentListChanged := c.SupportsListChangedNotifications("nonexistent")

	t.Logf("Tools list changed notifications: %v", toolsListChanged)
	t.Logf("Resources list changed notifications: %v", resourcesListChanged)
	t.Logf("Prompts list changed notifications: %v", promptsListChanged)
	t.Logf("Nonexistent list changed notifications: %v", nonexistentListChanged)

	// Verify nonexistent capability returns false
	assert.False(t, nonexistentListChanged)
}

func TestServerCapabilityDiscoveryNilHandling(t *testing.T) {
	// This test verifies that all server capability methods handle nil gracefully
	// We use a basic setup and expect nil/empty values
	c, _ := SetupClientWithMockTransport(t, "2025-03-26")
	defer c.Close()

	// All methods should handle nil capabilities gracefully
	caps := c.GetServerCapabilities()
	info := c.GetServerInfo()
	instructions := c.GetServerInstructions()

	// Instructions should always be empty string, not nil
	assert.Equal(t, "", instructions)

	// HasCapability should return false for any capability when nil
	assert.False(t, c.HasCapability("tools"))
	assert.False(t, c.HasCapability("resources"))
	assert.False(t, c.HasCapability("prompts"))
	assert.False(t, c.HasCapability("logging"))
	assert.False(t, c.HasCapability("experimental"))
	assert.False(t, c.HasCapability("nonexistent"))

	// SupportsResourceSubscriptions should return false for nil
	assert.False(t, c.SupportsResourceSubscriptions())

	// SupportsListChangedNotifications should return false for nil
	assert.False(t, c.SupportsListChangedNotifications("tools"))
	assert.False(t, c.SupportsListChangedNotifications("resources"))
	assert.False(t, c.SupportsListChangedNotifications("prompts"))
	assert.False(t, c.SupportsListChangedNotifications("nonexistent"))

	t.Logf("All nil handling tests passed - capabilities: %+v, info: %+v", caps, info)
}
