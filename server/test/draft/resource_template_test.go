// Package draft contains tests specifically for the draft version of the MCP specification
package draft

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestResourceTemplatesDraft tests resource templates handling against draft specification
func TestResourceTemplatesDraft(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-server-draft")

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

	// Register a more complex resource template with multiple parameters
	// This showcases the draft spec's ability to handle complex parameter types
	srv.Resource("/api/{version}/data/{format}/{year}/{month}/{day}", "Daily data resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		if params, ok := args.(map[string]interface{}); ok {
			version := params["version"].(string)
			format := params["format"].(string)
			year := params["year"].(string)
			month := params["month"].(string)
			day := params["day"].(string)

			return map[string]interface{}{
				"contents": []map[string]interface{}{
					{
						"uri": "/api/" + version + "/data/" + format + "/" + year + "/" + month + "/" + day,
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": "Data for " + year + "-" + month + "-" + day,
							},
							{
								"type":     "file",
								"name":     "data." + format,
								"url":      "https://example.com/api/" + version + "/data/" + format + "/" + year + "/" + month + "/" + day + "/download",
								"mimeType": getMimeType(format),
							},
						},
					},
				},
				"metadata": map[string]interface{}{
					"version":   version,
					"format":    format,
					"date":      year + "-" + month + "-" + day,
					"generated": true,
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

		// Verify each template has the required properties according to the draft spec
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
		foundDataTemplate := false
		for _, template := range response.Result.ResourceTemplates {
			uri := template["uriTemplate"].(string)
			if uri == "/users/{id}" {
				foundUserTemplate = true
			} else if uri == "/api/{version}/data/{format}/{year}/{month}/{day}" {
				foundDataTemplate = true
			}
		}

		if !foundUserTemplate {
			t.Error("User template not found in resource templates list")
		}
		if !foundDataTemplate {
			t.Error("Data template not found in resource templates list")
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
			if containsAll(uri, []string{"{", "}"}) {
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

		// Verify the content structure per draft spec (contents array)
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

		// Ensure the URI is included in the contents array
		uri, hasURI := content["uri"].(string)
		if !hasURI || uri != "/users/123" {
			t.Errorf("Expected URI to be '/users/123', got %v", uri)
		}
	})

	// Test using a template with multiple parameters
	t.Run("UseComplexTemplate", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 3,
			"method": "resources/read",
			"params": {
				"uri": "/api/v2/data/json/2025/03/26"
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

		// Verify the content structure per draft spec (contents array)
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

		// In draft, we expect content array with text AND file items
		if len(contentItems) != 2 {
			t.Fatalf("Expected 2 content items, got %d", len(contentItems))
		}

		// First item should be text
		textItem, ok := contentItems[0].(map[string]interface{})
		if !ok || textItem["type"] != "text" {
			t.Errorf("Expected first content item to be text, got %T or type %v",
				contentItems[0], textItem["type"])
		}

		if !contains(textItem["text"].(string), "2025-03-26") {
			t.Errorf("Expected text to contain date, got %v", textItem["text"])
		}

		// Second item should be file
		fileItem, ok := contentItems[1].(map[string]interface{})
		if !ok || fileItem["type"] != "file" {
			t.Errorf("Expected second content item to be file, got %T or type %v",
				contentItems[1], fileItem["type"])
		}

		// Verify file URL contains the parameters
		fileURL := fileItem["url"].(string)
		if !containsAll(fileURL, []string{"v2", "json", "2025", "03", "26"}) {
			t.Errorf("Expected file URL to contain all parameters, got: %s", fileURL)
		}

		// Verify the file name has the right format
		fileName := fileItem["name"].(string)
		if fileName != "data.json" {
			t.Errorf("Expected file name to be 'data.json', got %s", fileName)
		}

		// Verify MIME type is set correctly
		mimeType := fileItem["mimeType"].(string)
		if mimeType != "application/json" {
			t.Errorf("Expected MIME type to be 'application/json', got %s", mimeType)
		}

		// Verify metadata contains the parameters
		metadata, ok := response.Result["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected metadata to be present")
		}

		if metadata["version"] != "v2" {
			t.Errorf("Expected version to be 'v2', got %v", metadata["version"])
		}

		if metadata["format"] != "json" {
			t.Errorf("Expected format to be 'json', got %v", metadata["format"])
		}

		if metadata["date"] != "2025-03-26" {
			t.Errorf("Expected date to be '2025-03-26', got %v", metadata["date"])
		}

		// Ensure the URI is included in the contents array
		uri, hasURI := content["uri"].(string)
		if !hasURI || uri != "/api/v2/data/json/2025/03/26" {
			t.Errorf("Expected URI to be '/api/v2/data/json/2025/03/26', got %v", uri)
		}
	})
}

// Helper function to get MIME type for a file format
func getMimeType(format string) string {
	switch format {
	case "json":
		return "application/json"
	case "xml":
		return "application/xml"
	case "csv":
		return "text/csv"
	case "pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
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
	return s != "" && s != "null" && s != "{}" && s != "[]" && contains2(s, substr)
}

// Simple string contains implementation
func contains2(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
