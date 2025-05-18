// Package v20241105 contains tests specifically for the 2024-11-05 version of the MCP specification
package v20241105

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestResourceFormatV20241105 tests that resource responses are formatted according to the 2024-11-05 spec
func TestResourceFormatV20241105(t *testing.T) {
	// Create a server
	srv := server.NewServer("spec-test-server-2024-11-05")

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
		// Important: Format for 2024-11-05 spec explicitly
		if ctx.Version == "2024-11-05" {
			return map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type":     "blob",
						"blob":     "SGVsbG8sIFdvcmxkIQ==", // Base64 encoded "Hello, World!"
						"mimeType": "text/plain",
					},
				},
			}, nil
		}
		// For other versions, return in newer format (will be converted)
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
		// Format specifically for 2024-11-05
		if ctx.Version == "2024-11-05" {
			return map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "This is text content",
					},
					map[string]interface{}{
						"type":     "image",
						"imageUrl": "https://example.com/image.jpg",
						"altText":  "Example image",
					},
					map[string]interface{}{
						"type":  "link",
						"url":   "https://example.com",
						"title": "Example link",
					},
				},
			}, nil
		}
		// For other versions, use newer format
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

		responseBytes, err := server.HandleMessageWithVersion(srv.GetServer(), requestJSON, "2024-11-05")
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

		// According to the 2024-11-05 spec, the result should have a content array
		content, ok := response.Result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", response.Result["content"])
		}

		if len(content) == 0 {
			t.Fatalf("Expected at least one content item")
		}

		// Check the content item
		contentItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected content item to be a map, got %T", content[0])
		}

		// Text content must have type and text
		contentType, ok := contentItem["type"].(string)
		if !ok || contentType != "text" {
			t.Fatalf("Expected content type to be 'text', got %v", contentType)
		}

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

		responseBytes, err := server.HandleMessageWithVersion(srv.GetServer(), requestJSON, "2024-11-05")
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

		// According to the 2024-11-05 spec, the result should have a content array
		content, ok := response.Result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", response.Result["content"])
		}

		if len(content) == 0 {
			t.Fatalf("Expected at least one content item")
		}

		// Check the first content item
		contentItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected content item to be a map, got %T", content[0])
		}

		// Text content must have type and text
		contentType, ok := contentItem["type"].(string)
		if !ok || contentType != "text" {
			t.Fatalf("Expected content type to be 'text', got %v", contentType)
		}

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

		responseBytes, err := server.HandleMessageWithVersion(srv.GetServer(), requestJSON, "2024-11-05")
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

		// According to the 2024-11-05 spec, the result should have a content array
		content, ok := response.Result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", response.Result["content"])
		}

		if len(content) == 0 {
			t.Fatalf("Expected at least one content item")
		}

		// Check the first content item
		contentItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected content item to be a map, got %T", content[0])
		}

		// Blob content must have type, blob, and mimeType
		contentType, ok := contentItem["type"].(string)
		if !ok || contentType != "blob" {
			t.Fatalf("Expected content type to be 'blob', got %v", contentType)
		}

		blob, ok := contentItem["blob"].(string)
		if !ok || blob == "" {
			t.Fatalf("Expected blob field to be present and non-empty")
		}

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

		responseBytes, err := server.HandleMessageWithVersion(srv.GetServer(), requestJSON, "2024-11-05")
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

		// According to the 2024-11-05 spec, the result should have a content array
		content, ok := response.Result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", response.Result["content"])
		}

		if len(content) != 3 {
			t.Fatalf("Expected 3 content items, got %d", len(content))
		}

		// Check each content item has the right format
		// First item: text
		textItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected text content item to be a map, got %T", content[0])
		}

		textType, ok := textItem["type"].(string)
		if !ok || textType != "text" {
			t.Fatalf("Expected first item type to be 'text', got %v", textType)
		}

		if _, ok := textItem["text"].(string); !ok {
			t.Fatalf("Expected text field to be present in text content")
		}

		// Second item: image
		imageItem, ok := content[1].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected image content item to be a map, got %T", content[1])
		}

		imageType, ok := imageItem["type"].(string)
		if !ok || imageType != "image" {
			t.Fatalf("Expected second item type to be 'image', got %v", imageType)
		}

		if _, ok := imageItem["imageUrl"].(string); !ok {
			t.Fatalf("Expected imageUrl field to be present in image content")
		}

		// Third item: link
		linkItem, ok := content[2].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected link content item to be a map, got %T", content[2])
		}

		linkType, ok := linkItem["type"].(string)
		if !ok || linkType != "link" {
			t.Fatalf("Expected third item type to be 'link', got %v", linkType)
		}

		if _, ok := linkItem["url"].(string); !ok {
			t.Fatalf("Expected url field to be present in link content")
		}

		if _, ok := linkItem["title"].(string); !ok {
			t.Fatalf("Expected title field to be present in link content")
		}
	})
}
