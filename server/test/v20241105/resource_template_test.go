// Package v20241105 contains tests specifically for the 2024-11-05 version of the MCP specification
package v20241105

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestResourceTemplatesV20241105 tests resource template handling against 2024-11-05 specification
func TestResourceTemplatesV20241105(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-template-server-2024-11-05")

	// Register regular resources
	srv.Resource("/static", "A static resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "Static content", nil
	})

	// Register template resources with parameters
	srv.Resource("/users/{id}", "Get user by ID", func(ctx *server.Context, args interface{}) (interface{}, error) {
		params := args.(map[string]interface{})
		userId := params["id"].(string)
		return "User ID: " + userId, nil
	})

	srv.Resource("/files/{path*}", "Access files by path", func(ctx *server.Context, args interface{}) (interface{}, error) {
		params := args.(map[string]interface{})
		path := params["path"].(string)
		return "File path: " + path, nil
	})

	// Test resource templates listing
	t.Run("ResourceTemplatesList", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "resources/templates/list"
		}`)

		// Use HandleMessageWithVersion to force 2024-11-05 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2024-11-05")
		if err != nil {
			t.Fatalf("Failed to process message: %v", err)
		}

		var response struct {
			JSONRPC string `json:"jsonrpc"`
			ID      int    `json:"id"`
			Result  struct {
				ResourceTemplates []map[string]interface{} `json:"resourceTemplates"`
			} `json:"result"`
		}
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Verify the resource templates
		if len(response.Result.ResourceTemplates) != 2 {
			t.Fatalf("Expected 2 resource templates, got %d", len(response.Result.ResourceTemplates))
		}

		// Verify each template has the required fields
		for _, template := range response.Result.ResourceTemplates {
			if template["uriTemplate"] == nil {
				t.Errorf("Missing 'uriTemplate' in template: %v", template)
			}
			if template["name"] == nil {
				t.Errorf("Missing 'name' in template: %v", template)
			}
			if template["description"] == nil {
				t.Errorf("Missing 'description' in template: %v", template)
			}
			if template["mimeType"] == nil {
				t.Errorf("Missing 'mimeType' in template: %v", template)
			}
		}

		// Check for specific templates
		foundUserTemplate := false
		foundFileTemplate := false

		for _, template := range response.Result.ResourceTemplates {
			uriTemplate := template["uriTemplate"].(string)
			switch uriTemplate {
			case "/users/{id}":
				foundUserTemplate = true
			case "/files/{path*}":
				foundFileTemplate = true
			}
		}

		if !foundUserTemplate {
			t.Errorf("Missing '/users/{id}' template")
		}
		if !foundFileTemplate {
			t.Errorf("Missing '/files/{path*}' template")
		}
	})

	// Test that template resources don't appear in the regular resources list
	t.Run("TemplatesExcludedFromResourcesList", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 4,
			"method": "resources/list"
		}`)

		// Use HandleMessageWithVersion to force 2024-11-05 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2024-11-05")
		if err != nil {
			t.Fatalf("Failed to process message: %v", err)
		}

		var response struct {
			JSONRPC string `json:"jsonrpc"`
			ID      int    `json:"id"`
			Result  struct {
				Resources []map[string]interface{} `json:"resources"`
			} `json:"result"`
		}
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// We should only have the regular resource, not the template resources
		if len(response.Result.Resources) != 1 {
			t.Fatalf("Expected 1 resource (only the regular one), got %d", len(response.Result.Resources))
		}

		// Verify the resource has the expected URI
		if response.Result.Resources[0]["uri"] != "/static" {
			t.Errorf("Expected resource URI to be '/static', got %s", response.Result.Resources[0]["uri"])
		}

		// Verify the template resources are not in the list
		for _, resource := range response.Result.Resources {
			uri := resource["uri"].(string)
			if uri == "/users/{id}" || uri == "/files/{path*}" {
				t.Errorf("Template resource found in regular resources list: %s", uri)
			}
		}
	})

	// Test using a template
	t.Run("UseTemplate", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 2,
			"method": "resources/read",
			"params": {
				"uri": "/users/123"
			}
		}`)

		// Use HandleMessageWithVersion to force 2024-11-05 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2024-11-05")
		if err != nil {
			t.Fatalf("Failed to process message: %v", err)
		}

		// Debug: Print out the raw response to see what's actually being returned
		t.Logf("Raw response: %s", string(responseBytes))

		var response struct {
			JSONRPC string                 `json:"jsonrpc"`
			ID      int                    `json:"id"`
			Result  map[string]interface{} `json:"result"`
		}
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// For 2024-11-05, check if content exists and is correctly formatted
		if response.Result["content"] == nil {
			t.Fatalf("Missing 'content' in response result: %+v", response.Result)
		}

		// Content should be a simple array of content items
		contentItems, ok := response.Result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T: %+v", response.Result["content"], response.Result["content"])
		}

		if len(contentItems) == 0 {
			t.Fatalf("Expected at least one content item")
		}

		// Check the first content item
		textItem, ok := contentItems[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected content item to be a map, got %T", contentItems[0])
		}

		if textItem["type"] != "text" {
			t.Errorf("Expected content type to be 'text', got %v", textItem["type"])
		}

		if textItem["text"] != "User ID: 123" {
			t.Errorf("Expected text to be 'User ID: 123', got %v", textItem["text"])
		}
	})

	// Test using a wildcard path template
	t.Run("UseWildcardTemplate", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 3,
			"method": "resources/read",
			"params": {
				"uri": "/files/path/to/file.txt"
			}
		}`)

		// Use HandleMessageWithVersion to force 2024-11-05 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2024-11-05")
		if err != nil {
			t.Fatalf("Failed to process message: %v", err)
		}

		// Debug: Print out the raw response to see what's actually being returned
		t.Logf("Raw response: %s", string(responseBytes))

		var response struct {
			JSONRPC string                 `json:"jsonrpc"`
			ID      int                    `json:"id"`
			Result  map[string]interface{} `json:"result"`
		}
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// For 2024-11-05, check if content exists and is correctly formatted
		if response.Result["content"] == nil {
			t.Fatalf("Missing 'content' in response result: %+v", response.Result)
		}

		// Content should be a simple array of content items
		contentItems, ok := response.Result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T: %+v", response.Result["content"], response.Result["content"])
		}

		if len(contentItems) == 0 {
			t.Fatalf("Expected at least one content item")
		}

		// Check the first content item
		textItem, ok := contentItems[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected content item to be a map, got %T", contentItems[0])
		}

		if textItem["type"] != "text" {
			t.Errorf("Expected content type to be 'text', got %v", textItem["type"])
		}

		expected := "File path: path/to/file.txt"
		if textItem["text"] != expected {
			t.Errorf("Expected text to be '%s', got %v", expected, textItem["text"])
		}
	})
}
