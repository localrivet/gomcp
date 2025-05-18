// Package v20250326 contains tests specifically for the 2025-03-26 version of the MCP specification
package v20250326

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestResourceTemplatesV20250326 tests resource templates handling against 2025-03-26 specification
func TestResourceTemplatesV20250326(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-server-2025-03-26")

	// Register a regular resource (not a template)
	srv.Resource("/regular", "A regular resource (not a template)", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "This is a regular resource", nil
	})

	// Register a resource template with a path parameter
	srv.Resource("/users/{id}", "User template resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		if params, ok := args.(map[string]interface{}); ok {
			id := params["id"].(string)
			return map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "User ID: " + id,
					},
				},
				"metadata": map[string]interface{}{
					"resourceType": "user",
					"timestamp":    "2025-03-26T12:00:00Z",
				},
			}, nil
		}
		return "No ID provided", nil
	})

	// Register a resource template with multiple parameters
	srv.Resource("/repos/{owner}/{repo}/issues/{number}", "GitHub issue resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		if params, ok := args.(map[string]interface{}); ok {
			owner := params["owner"].(string)
			repo := params["repo"].(string)
			number := params["number"].(string)
			return map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Issue Details",
					},
					{
						"type":  "link",
						"url":   "https://github.com/" + owner + "/" + repo + "/issues/" + number,
						"title": "Issue #" + number,
					},
				},
				"metadata": map[string]interface{}{
					"owner":  owner,
					"repo":   repo,
					"number": number,
				},
			}, nil
		}
		return "Invalid parameters", nil
	})

	// Test listing resource templates
	t.Run("ListResourceTemplates", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "resources/templates/list"
		}`)

		responseBytes, err := server.HandleMessage(srv.GetServer(), requestJSON)
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

		// Verify we have the expected number of templates
		if len(response.Result.ResourceTemplates) != 2 {
			t.Fatalf("Expected 2 resource templates, got %d", len(response.Result.ResourceTemplates))
		}

		// Verify each template has the required properties according to the 2025-03-26 spec
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

		// Verify templates have the correct URIs
		foundUserTemplate := false
		foundIssueTemplate := false
		for _, template := range response.Result.ResourceTemplates {
			uri := template["uriTemplate"].(string)
			if uri == "/users/{id}" {
				foundUserTemplate = true
			} else if uri == "/repos/{owner}/{repo}/issues/{number}" {
				foundIssueTemplate = true
			}
		}

		if !foundUserTemplate {
			t.Error("User template not found in resource templates list")
		}
		if !foundIssueTemplate {
			t.Error("Issue template not found in resource templates list")
		}
	})

	// Test that template resources don't appear in the regular resources list
	t.Run("TemplatesExcludedFromResourcesList", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 4,
			"method": "resources/list"
		}`)

		responseBytes, err := server.HandleMessage(srv.GetServer(), requestJSON)
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
		if response.Result.Resources[0]["uri"] != "/regular" {
			t.Errorf("Expected resource URI to be '/regular', got %s", response.Result.Resources[0]["uri"])
		}

		// Verify the template resources are not in the list
		for _, resource := range response.Result.Resources {
			uri := resource["uri"].(string)
			if uri == "/users/{id}" || uri == "/repos/{owner}/{repo}/issues/{number}" {
				t.Errorf("Template resource found in regular resources list: %s", uri)
			}
		}
	})

	// Test using a template (accessing a parameterized resource)
	t.Run("UseResourceTemplate", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 2,
			"method": "resources/read",
			"params": {
				"uri": "/users/123"
			}
		}`)

		responseBytes, err := server.HandleMessage(srv.GetServer(), requestJSON)
		if err != nil {
			t.Fatalf("Failed to process message: %v", err)
		}

		var response struct {
			JSONRPC string                 `json:"jsonrpc"`
			ID      int                    `json:"id"`
			Result  map[string]interface{} `json:"result"`
		}
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Verify the content structure per 2025-03-26 spec (contents array)
		contents, ok := response.Result["contents"].([]interface{})
		if !ok {
			t.Fatalf("Expected contents to be an array, got %T", response.Result["contents"])
		}

		if len(contents) == 0 {
			t.Fatalf("Expected at least one content item in contents array")
		}

		content := contents[0].(map[string]interface{})
		contentItems, ok := content["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", content["content"])
		}

		if len(contentItems) == 0 {
			t.Fatalf("Expected at least one content item")
		}

		textItem, ok := contentItems[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected text content item to be a map, got %T", contentItems[0])
		}

		if textItem["type"] != "text" {
			t.Errorf("Expected content type to be 'text', got %v", textItem["type"])
		}

		if textItem["text"] != "User ID: 123" {
			t.Errorf("Expected text to be 'User ID: 123', got %v", textItem["text"])
		}

		// Verify metadata for 2025-03-26
		metadata, ok := response.Result["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected metadata to be a map for 2025-03-26, got %T", response.Result["metadata"])
		}

		if metadata["resourceType"] != "user" {
			t.Errorf("Expected resourceType to be 'user', got %v", metadata["resourceType"])
		}

		if metadata["timestamp"] != "2025-03-26T12:00:00Z" {
			t.Errorf("Expected timestamp to be '2025-03-26T12:00:00Z', got %v", metadata["timestamp"])
		}
	})

	// Test using a template with multiple parameters
	t.Run("UseMultiParameterTemplate", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 3,
			"method": "resources/read",
			"params": {
				"uri": "/repos/localrivet/gomcp/issues/42"
			}
		}`)

		responseBytes, err := server.HandleMessage(srv.GetServer(), requestJSON)
		if err != nil {
			t.Fatalf("Failed to process message: %v", err)
		}

		var response struct {
			JSONRPC string                 `json:"jsonrpc"`
			ID      int                    `json:"id"`
			Result  map[string]interface{} `json:"result"`
		}
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Verify the content structure per 2025-03-26 spec (contents array)
		contents, ok := response.Result["contents"].([]interface{})
		if !ok {
			t.Fatalf("Expected contents to be an array, got %T", response.Result["contents"])
		}

		if len(contents) == 0 {
			t.Fatalf("Expected at least one content item in contents array")
		}

		content := contents[0].(map[string]interface{})
		contentItems, ok := content["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", content["content"])
		}

		// In 2025-03-26, we expect content array with text AND link items
		if len(contentItems) != 2 {
			t.Fatalf("Expected 2 content items, got %d", len(contentItems))
		}

		// First item should be text
		textItem, ok := contentItems[0].(map[string]interface{})
		if !ok || textItem["type"] != "text" {
			t.Errorf("Expected first content item to be text, got %T or type %v",
				contentItems[0], textItem["type"])
		}

		// Second item should be link
		linkItem, ok := contentItems[1].(map[string]interface{})
		if !ok || linkItem["type"] != "link" {
			t.Errorf("Expected second content item to be link, got %T or type %v",
				contentItems[1], linkItem["type"])
		}

		// Verify link URL contains the parameters
		linkURL := linkItem["url"].(string)
		if !containsAll(linkURL, []string{"localrivet", "gomcp", "42"}) {
			t.Errorf("Expected link URL to contain all parameters, got: %s", linkURL)
		}

		// Verify metadata contains the parameters
		metadata, ok := response.Result["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected metadata to be present")
		}

		if metadata["owner"] != "localrivet" {
			t.Errorf("Expected owner to be 'localrivet', got %v", metadata["owner"])
		}

		if metadata["repo"] != "gomcp" {
			t.Errorf("Expected repo to be 'gomcp', got %v", metadata["repo"])
		}

		if metadata["number"] != "42" {
			t.Errorf("Expected number to be '42', got %v", metadata["number"])
		}
	})
}

// Helper function to check if a string contains all the specified substrings
func containsAll(s string, substrings []string) bool {
	for _, sub := range substrings {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
