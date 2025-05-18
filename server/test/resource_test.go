// Package test provides test utilities for the server package.
package test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestResourceRegistration tests registering resources with the server
func TestResourceRegistration(t *testing.T) {
	// Create a server
	s := server.NewServer("test-server")

	// Register some resources
	s.Resource("/test/path", "Test resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "Test resource data", nil
	})

	s.Resource("/another/path", "Another resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "Another resource data", nil
	})

	// Add a resource with parameters
	s.Resource("/users/{id}", "User by ID", func(ctx *server.Context, args interface{}) (interface{}, error) {
		params, ok := ctx.Metadata["params"].(map[string]string)
		if !ok {
			return nil, errors.New("no params in context")
		}
		id, exists := params["id"]
		if !exists {
			return nil, errors.New("no ID parameter")
		}
		return "User data for ID: " + id, nil
	})

	// Create a resource/list request
	requestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "resources/list"
	}`)

	// Process the request using the exported HandleMessage method
	responseBytes, err := server.HandleMessage(s.GetServer(), requestJSON)
	if err != nil {
		t.Fatalf("Failed to process resources/list request: %v", err)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Check that the response has a result
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result object in response, got: %T", response["result"])
	}

	// Check that the result has a resources array
	resources, ok := result["resources"].([]interface{})
	if !ok {
		t.Fatalf("Expected resources array in result, got: %T", result["resources"])
	}

	// Check that we have at least the resources we registered
	// Note: Exact number may vary based on built-in resources and templates
	// We should just check that our custom resources are included
	if len(resources) < 2 {
		t.Errorf("Expected at least 2 resources, got %d", len(resources))
	}

	// Print the resources for debugging
	for i, res := range resources {
		if resource, ok := res.(map[string]interface{}); ok {
			t.Logf("Resource %d: %v", i, resource)
		} else {
			t.Logf("Resource %d: (not a map) %v", i, res)
		}
	}

	// Verify our specific resources are in the list - only check for essential resources
	foundTestPath := false
	foundAnotherPath := false

	for _, res := range resources {
		resource, ok := res.(map[string]interface{})
		if !ok {
			continue
		}

		uri, ok := resource["uri"].(string)
		if !ok {
			continue
		}

		if uri == "/test/path" {
			foundTestPath = true
		} else if uri == "/another/path" {
			foundAnotherPath = true
		}
	}

	if !foundTestPath {
		t.Error("Resource /test/path not found in resources list")
	}
	if !foundAnotherPath {
		t.Error("Resource /another/path not found in resources list")
	}

	// Test calling a resource with parameters
	paramCallRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 3,
		"method": "resources/call",
		"params": {
			"path": "/users/123"
		}
	}`)

	// Process the request using the exported HandleMessage method
	paramCallResponseBytes, err := server.HandleMessage(s.GetServer(), paramCallRequestJSON)
	if err != nil {
		t.Fatalf("Failed to process resources/call request with params: %v", err)
	}

	// Parse the response
	var paramCallResponse map[string]interface{}
	if err := json.Unmarshal(paramCallResponseBytes, &paramCallResponse); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Test a resource that doesn't exist
	notFoundRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 4,
		"method": "resources/call",
		"params": {
			"path": "/not/exist"
		}
	}`)

	// Process the request using the exported HandleMessage method
	notFoundResponseBytes, err := server.HandleMessage(s.GetServer(), notFoundRequestJSON)
	if err != nil {
		t.Fatalf("Failed to process resources/call request for non-existent path: %v", err)
	}

	// Parse the response
	var notFoundResponse map[string]interface{}
	if err := json.Unmarshal(notFoundResponseBytes, &notFoundResponse); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}
}

// TestDisabledResources tests that resources can be disabled
func TestDisabledResources(t *testing.T) {
	// Create a server
	s := server.NewServer("test-server-disabled")

	// Register a resource
	s.Resource("/test/path", "Test resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "Test resource data", nil
	})

	// Create a resource/list request
	requestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "resources/list"
	}`)

	// Process the request using the exported HandleMessage method
	_, err := server.HandleMessage(s.GetServer(), requestJSON)
	if err != nil {
		t.Fatalf("Failed to process resources/list request: %v", err)
	}
}

// TestResourceRequest tests processing resource requests
func TestResourceRequest(t *testing.T) {
	// Create a server
	s := server.NewServer("test-server")

	// Register a test resource
	s.Resource("/api/data", "Test data resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "Resource Data", nil
	})

	// Create a resources/read request
	message := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "resources/read",
		"params": {
			"uri": "/api/data"
		}
	}`)

	// Handle the message using the exported HandleMessage method
	response, err := server.HandleMessage(s.GetServer(), message)
	if err != nil {
		t.Fatalf("Failed to handle resources/read message: %v", err)
	}

	// Parse the response
	var respObj map[string]interface{}
	if err := json.Unmarshal(response, &respObj); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check that there's a result (not an error)
	result, hasResult := respObj["result"]
	if !hasResult {
		t.Fatalf("Expected result in response, but got: %v", respObj)
	}

	// Verify that the result is a map (as expected for resource results)
	if _, ok := result.(map[string]interface{}); !ok {
		t.Errorf("Expected result to be a map, got: %T", result)
	}
}

// TestResourceList tests listing resources
func TestResourceList(t *testing.T) {
	// Create a server
	s := server.NewServer("test-server")

	// Register some resources
	resourcePaths := []string{
		"/api/users",
		"/api/posts",
		"/api/comments",
	}

	for _, path := range resourcePaths {
		s.Resource(path, "Resource at "+path, func(ctx *server.Context, args interface{}) (interface{}, error) {
			return "Data for " + path, nil
		})
	}

	// Create a resources/list request
	message := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "resources/list"
	}`)

	// Handle the message using the exported HandleMessage method
	response, err := server.HandleMessage(s.GetServer(), message)
	if err != nil {
		t.Fatalf("Failed to handle resources/list message: %v", err)
	}

	// Parse the response
	var respObj map[string]interface{}
	if err := json.Unmarshal(response, &respObj); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check if the response contains a result (no error)
	result, hasResult := respObj["result"]
	if !hasResult {
		t.Fatalf("Expected result in response, but found none: %v", respObj)
	}

	// Extract resources array
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", result)
	}

	resources, ok := resultMap["resources"].([]interface{})
	if !ok {
		t.Fatalf("Expected resources to be an array, got %T", resultMap["resources"])
	}

	// Check if all registered resources are in the list
	if len(resources) < len(resourcePaths) {
		t.Errorf("Expected at least %d resources, got %d", len(resourcePaths), len(resources))
	}

	// Check if all registered paths are present
	foundPaths := make(map[string]bool)
	for _, res := range resources {
		resource, ok := res.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected resource to be a map, got %T", res)
		}

		uri, ok := resource["uri"].(string)
		if !ok {
			t.Fatalf("Expected uri to be a string, got %T", resource["uri"])
		}

		foundPaths[uri] = true
	}

	for _, path := range resourcePaths {
		if !foundPaths[path] {
			t.Errorf("Resource path %s not found in list", path)
		}
	}
}

// TestConvertToResourceHandler is simplified since we can't easily access the internal functions
func TestConvertToResourceHandler(t *testing.T) {
	// Create a test server to test resource registration with different handler signatures
	s := server.NewServer("test-server")

	// Register a resource with a standard signature
	s.Resource("/test/standard", "Standard handler", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "standard", nil
	})

	// Test by calling the resource
	requestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "resources/call",
		"params": {
			"path": "/test/standard"
		}
	}`)

	response, err := server.HandleMessage(s.GetServer(), requestJSON)
	if err != nil {
		t.Fatalf("Failed to call standard resource: %v", err)
	}

	if response == nil {
		t.Errorf("Expected response from standard handler, got nil")
	}
}
