// Package v20250326 contains tests specifically for the 2025-03-26 version of the MCP specification
package v20250326

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestResourceFormatV20250326 tests that resource responses are formatted according to the 2025-03-26 spec
func TestResourceFormatV20250326(t *testing.T) {
	// Create a server
	srv := server.NewServer("spec-test-server-2025-03-26")

	// Add a simple string resource (like /hello)
	srv.Resource("/hello", "String resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "Hello, World!", nil
	})

	// Add a simple text resource
	srv.Resource("/text", "Text resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "This is a simple text resource", nil
	})

	// Add a blob resource
	srv.Resource("/blob", "Blob resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"uri": "/blob",
					"content": []map[string]interface{}{
						{
							"type":     "blob",
							"blob":     "SGVsbG8sIFdvcmxkIQ==", // Base64 encoded "Hello, World!"
							"mimeType": "text/plain",
						},
					},
				},
			},
		}, nil
	})

	// Add a mixed content resource
	srv.Resource("/mixed", "Mixed content resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"uri": "/mixed",
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": "This is text content",
						},
						{
							"type":     "image",
							"imageUrl": "https://example.com/image.jpg",
							"altText":  "Example image",
						},
						{
							"type":  "link",
							"url":   "https://example.com",
							"title": "Example link",
						},
					},
				},
			},
		}, nil
	})

	// Test string resource format (like /hello)
	t.Run("StringResource", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "resources/read",
			"params": {
				"uri": "/hello"
			}
		}`)

		responseBytes, err := server.HandleMessageWithVersion(srv.GetServer(), requestJSON, "2025-03-26")
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

		// According to the 2025-03-26 spec, the result should have a contents array
		contents, ok := response.Result["contents"].([]interface{})
		if !ok {
			t.Fatalf("Expected contents to be an array, got %T", response.Result["contents"])
		}

		if len(contents) == 0 {
			t.Fatalf("Expected at least one content object")
		}

		// Each contents item should be an object with uri and content fields
		contentObj, ok := contents[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected contents item to be a map, got %T", contents[0])
		}

		// Check for uri field
		uri, ok := contentObj["uri"].(string)
		if !ok || uri != "/hello" {
			t.Fatalf("Expected uri field to be '/hello', got: %v", uri)
		}

		// Check for content array
		content, ok := contentObj["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", contentObj["content"])
		}

		if len(content) == 0 {
			t.Fatalf("Expected at least one content item")
		}

		// Each content item should have required fields
		contentItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected content item to be a map, got %T", content[0])
		}

		// Validate type field
		contentType, ok := contentItem["type"].(string)
		if !ok || contentType != "text" {
			t.Fatalf("Expected type to be 'text', got %v", contentType)
		}

		// Text content must have text field
		text, ok := contentItem["text"].(string)
		if !ok {
			t.Fatalf("Expected text field to be present and of string type")
		}

		if text != "Hello, World!" {
			t.Fatalf("Expected text to be 'Hello, World!', got '%s'", text)
		}
	})

	// Test text resource format
	t.Run("TextResource", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "resources/read",
			"params": {
				"uri": "/text"
			}
		}`)

		responseBytes, err := server.HandleMessageWithVersion(srv.GetServer(), requestJSON, "2025-03-26")
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

		// According to the 2025-03-26 spec, the result should have a contents array
		contents, ok := response.Result["contents"].([]interface{})
		if !ok {
			t.Fatalf("Expected contents to be an array, got %T", response.Result["contents"])
		}

		if len(contents) == 0 {
			t.Fatalf("Expected at least one content object")
		}

		// Each contents item should be an object with uri and content fields
		contentObj, ok := contents[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected contents item to be a map, got %T", contents[0])
		}

		// Check for uri field
		uri, ok := contentObj["uri"].(string)
		if !ok || uri != "/text" {
			t.Fatalf("Expected uri field to be '/text', got: %v", uri)
		}

		// Check for content array
		content, ok := contentObj["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", contentObj["content"])
		}

		if len(content) == 0 {
			t.Fatalf("Expected at least one content item")
		}

		// Each content item should have required fields
		contentItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected content item to be a map, got %T", content[0])
		}

		// Validate type field
		contentType, ok := contentItem["type"].(string)
		if !ok || contentType != "text" {
			t.Fatalf("Expected type to be 'text', got %v", contentType)
		}

		// Text content must have text field
		text, ok := contentItem["text"].(string)
		if !ok || text == "" {
			t.Fatalf("Expected text field to be present and non-empty")
		}
	})

	// Test blob resource format
	t.Run("BlobResource", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "resources/read",
			"params": {
				"uri": "/blob"
			}
		}`)

		responseBytes, err := server.HandleMessageWithVersion(srv.GetServer(), requestJSON, "2025-03-26")
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

		// According to the 2025-03-26 spec, the result should have a contents array
		contents, ok := response.Result["contents"].([]interface{})
		if !ok {
			t.Fatalf("Expected contents to be an array, got %T", response.Result["contents"])
		}

		if len(contents) == 0 {
			t.Fatalf("Expected at least one content object")
		}

		// Each contents item should be an object with uri and content fields
		contentObj, ok := contents[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected contents item to be a map, got %T", contents[0])
		}

		// Check for uri field
		uri, ok := contentObj["uri"].(string)
		if !ok || uri != "/blob" {
			t.Fatalf("Expected uri field to be '/blob', got: %v", uri)
		}

		// Check for content array
		content, ok := contentObj["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", contentObj["content"])
		}

		if len(content) == 0 {
			t.Fatalf("Expected at least one content item")
		}

		// Each content item should have required fields
		contentItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected content item to be a map, got %T", content[0])
		}

		// Validate type field
		contentType, ok := contentItem["type"].(string)
		if !ok || contentType != "blob" {
			t.Fatalf("Expected type to be 'blob', got %v", contentType)
		}

		// Blob content must have blob field
		blob, ok := contentItem["blob"].(string)
		if !ok || blob == "" {
			t.Fatalf("Expected blob field to be present and non-empty")
		}

		// Blob should have mimeType
		mimeType, ok := contentItem["mimeType"].(string)
		if !ok || mimeType == "" {
			t.Fatalf("Expected mimeType field to be present and non-empty")
		}
	})

	// Test mixed content resource
	t.Run("MixedResource", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "resources/read",
			"params": {
				"uri": "/mixed"
			}
		}`)

		responseBytes, err := server.HandleMessageWithVersion(srv.GetServer(), requestJSON, "2025-03-26")
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

		// According to the 2025-03-26 spec, the result should have a contents array
		contents, ok := response.Result["contents"].([]interface{})
		if !ok {
			t.Fatalf("Expected contents to be an array, got %T", response.Result["contents"])
		}

		if len(contents) == 0 {
			t.Fatalf("Expected at least one content object")
		}

		// Each contents item should be an object with uri and content fields
		contentObj, ok := contents[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected contents item to be a map, got %T", contents[0])
		}

		// Check for uri field
		uri, ok := contentObj["uri"].(string)
		if !ok || uri != "/mixed" {
			t.Fatalf("Expected uri field to be '/mixed', got: %v", uri)
		}

		// Check for content array
		content, ok := contentObj["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", contentObj["content"])
		}

		if len(content) != 3 {
			t.Fatalf("Expected 3 content items, got %d", len(content))
		}

		// Check each content type has required fields
		for i, item := range content {
			contentItem, ok := item.(map[string]interface{})
			if !ok {
				t.Fatalf("Content item %d is not a map: %T", i, item)
			}

			contentType, ok := contentItem["type"].(string)
			if !ok {
				t.Fatalf("Content item %d missing type field", i)
			}

			switch contentType {
			case "text":
				if _, ok := contentItem["text"].(string); !ok {
					t.Fatalf("Text content item missing text field")
				}
			case "image":
				if _, ok := contentItem["imageUrl"].(string); !ok {
					t.Fatalf("Image content item missing imageUrl field")
				}
			case "link":
				if _, ok := contentItem["url"].(string); !ok {
					t.Fatalf("Link content item missing url field")
				}
				if _, ok := contentItem["title"].(string); !ok {
					t.Fatalf("Link content item missing title field")
				}
			case "blob":
				if _, ok := contentItem["blob"].(string); !ok {
					t.Fatalf("Blob content item missing blob field")
				}
				if _, ok := contentItem["mimeType"].(string); !ok {
					t.Fatalf("Blob content item missing mimeType field")
				}
			}
		}
	})
}
