// Package v20241105 contains tests specifically for the 2024-11-05 version of the MCP specification
package v20241105

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestResourceHandlingV20241105 tests resource handling against 2024-11-05 specification
func TestResourceHandlingV20241105(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-resource-server-2024-11-05")

	// Register resources for each type
	srv.Resource("/text", "Text resource test", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "This is a 2024-11-05 resource response", nil
	})

	srv.Resource("/image", "Image resource test", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return server.ImageResource{
			URL:     "https://example.com/image.jpg",
			AltText: "Test image for 2024-11-05",
		}, nil
	})

	srv.Resource("/link", "Link resource test", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return server.LinkResource{
			URL:   "https://example.com",
			Title: "Example website",
		}, nil
	})

	srv.Resource("/empty", "Empty resource test", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// For v20241105, we need to return a specific format to match the expectation
		return map[string]interface{}{
			"content": []interface{}{},
		}, nil
	})

	srv.Resource("/json", "JSON resource test", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return server.JSONResource{
			Data: map[string]interface{}{
				"key": "value",
				"num": 42,
			},
		}, nil
	})

	srv.Resource("/file", "File resource test", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return server.FileResource{
			Filename: "test.txt",
			URL:      "https://example.com/test.txt",
			MimeType: "text/plain",
		}, nil
	})

	srv.Resource("/custom", "Custom resource test", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "Custom content",
				},
			},
		}, nil
	})

	// Register an audio resource
	srv.Resource("/audio", "Audio resource test", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return server.AudioResource{
			URL:      "https://example.com/audio.mp3",
			MimeType: "audio/mpeg",
			AltText:  "Test audio for v20241105",
		}, nil
	})

	// Test text resource
	t.Run("TextResource", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "resources/read",
			"params": {
				"uri": "/text"
			}
		}`)

		// Use HandleMessageWithVersion to force 2024-11-05 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2024-11-05")
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

		// Verify response follows 2024-11-05 spec
		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 1)

		result := response.Result

		// Check for content field - 2024-11-05 uses "content" (NOT "contents")
		if _, hasContents := result["contents"]; hasContents {
			t.Errorf("Response has 'contents' field which is not part of 2024-11-05 spec, should use 'content'")
		}

		// Check we have content in the correct format
		content, ok := result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", result["content"])
		}

		if len(content) == 0 {
			t.Fatalf("Expected at least one content item")
		}

		// Check the content item directly
		textItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected content item to be a map, got %T", content[0])
		}

		if textItem["type"] != "text" {
			t.Errorf("Expected content type to be 'text', got %v", textItem["type"])
		}

		if textItem["text"] != "This is a 2024-11-05 resource response" {
			t.Errorf("Expected text to be 'This is a 2024-11-05 resource response', got %v", textItem["text"])
		}
	})

	// Test image resource
	t.Run("ImageResource", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 2,
			"method": "resources/read",
			"params": {
				"uri": "/image"
			}
		}`)

		// Always use HandleMessageWithVersion to ensure 2024-11-05 format
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2024-11-05")
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

		// Verify response follows 2024-11-05 spec
		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 2)

		result := response.Result

		// Check for content field - 2024-11-05 uses "content" (NOT "contents")
		if _, hasContents := result["contents"]; hasContents {
			t.Errorf("Response has 'contents' field which is not part of 2024-11-05 spec, should use 'content'")
		}

		// 2024-11-05 schema expects content array
		content, ok := result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", result["content"])
		}

		if len(content) == 0 {
			t.Fatalf("Expected at least one content item in content array")
		}

		// Check image item format
		imageItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected image content item to be a map, got %T", content[0])
		}

		if imageItem["type"] != "image" {
			t.Errorf("Expected content type to be 'image', got %v", imageItem["type"])
		}

		if imageItem["imageUrl"] != "https://example.com/image.jpg" {
			t.Errorf("Expected imageUrl to be 'https://example.com/image.jpg', got %v", imageItem["imageUrl"])
		}

		if imageItem["altText"] != "Test image for 2024-11-05" {
			t.Errorf("Expected altText to be 'Test image for 2024-11-05', got %v", imageItem["altText"])
		}
	})

	// Test link resource
	t.Run("LinkResource", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 3,
			"method": "resources/read",
			"params": {
				"uri": "/link"
			}
		}`)

		// Always use HandleMessageWithVersion to ensure 2024-11-05 format
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2024-11-05")
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

		// Verify response follows 2024-11-05 spec
		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 3)

		result := response.Result

		// Check for content field - 2024-11-05 uses "content" (NOT "contents")
		if _, hasContents := result["contents"]; hasContents {
			t.Errorf("Response has 'contents' field which is not part of 2024-11-05 spec, should use 'content'")
		}

		// 2024-11-05 schema expects content array
		content, ok := result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", result["content"])
		}

		if len(content) == 0 {
			t.Fatalf("Expected at least one content item")
		}

		// Check the link item format
		linkItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected link content item to be a map, got %T", content[0])
		}

		if linkItem["type"] != "link" {
			t.Errorf("Expected content type to be 'link', got %v", linkItem["type"])
		}

		if linkItem["url"] != "https://example.com" {
			t.Errorf("Expected url to be 'https://example.com', got %v", linkItem["url"])
		}

		if linkItem["title"] != "Example website" {
			t.Errorf("Expected title to be 'Example website', got %v", linkItem["title"])
		}
	})

	// Test empty resource
	t.Run("EmptyResource", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 4,
			"method": "resources/read",
			"params": {
				"uri": "/empty"
			}
		}`)

		// Use HandleMessageWithVersion to force 2024-11-05 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2024-11-05")
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

		// Verify response follows 2024-11-05 spec
		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 4)

		// Verify we have a result
		result := response.Result
		if result == nil {
			t.Fatalf("Expected a result, got nil")
		}

		// Check for content field - 2024-11-05 uses "content" (NOT "contents")
		if _, hasContents := result["contents"]; hasContents {
			t.Errorf("Response has 'contents' field which is not part of 2024-11-05 spec, should use 'content'")
		}

		// For empty resources, we should have an empty content array
		content, ok := result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", result["content"])
		}

		if len(content) != 0 {
			t.Fatalf("Expected empty content array, got array with %d items", len(content))
		}
	})

	// Test resource list
	t.Run("ResourceList", func(t *testing.T) {
		resourceListRequestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 6,
			"method": "resources/list"
		}`)

		// Always use HandleMessageWithVersion to ensure 2024-11-05 format
		resourceListResponseBytes, err := server.HandleMessageWithVersion(srv, resourceListRequestJSON, "2024-11-05")
		if err != nil {
			t.Fatalf("Failed to process resources/list message: %v", err)
		}

		var resourceListResponse struct {
			JSONRPC string `json:"jsonrpc"`
			ID      int    `json:"id"`
			Result  struct {
				Resources []map[string]interface{} `json:"resources"`
			} `json:"result"`
		}
		if err := json.Unmarshal(resourceListResponseBytes, &resourceListResponse); err != nil {
			t.Fatalf("Failed to parse resources/list response: %v", err)
		}

		// Verify response follows 2024-11-05 spec
		validateJSONRPCResponse(t, resourceListResponse.JSONRPC, resourceListResponse.ID, 6)

		// Verify we have only non-template resources
		// Note: Template resources are now excluded from regular resources list
		// For v20241105 tests, there should be 8 resources (json, file, image, link, text, empty, custom, audio, etc.)
		if len(resourceListResponse.Result.Resources) != 8 {
			t.Fatalf("Expected 8 resources, got %d", len(resourceListResponse.Result.Resources))
		}

		// Verify each resource has the required properties according to 2024-11-05 spec
		for _, resource := range resourceListResponse.Result.Resources {
			if resource["uri"] == nil {
				t.Errorf("Missing 'uri' in resource: %v", resource)
			}
			if resource["name"] == nil {
				t.Errorf("Missing 'name' in resource: %v", resource)
			}
			if resource["kind"] == nil {
				t.Errorf("Missing 'kind' in resource: %v", resource)
			}
		}
	})

	// Test audio resource (which should be converted to a link for v20241105)
	t.Run("AudioResourceAsLink", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 7,
			"method": "resources/read",
			"params": {
				"uri": "/audio"
			}
		}`)

		// Use HandleMessageWithVersion to force 2024-11-05 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2024-11-05")
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

		// Verify response follows 2024-11-05 spec
		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 7)

		result := response.Result

		// Check for content field - 2024-11-05 uses "content" (NOT "contents")
		if _, hasContents := result["contents"]; hasContents {
			t.Errorf("Response has 'contents' field which is not part of 2024-11-05 spec, should use 'content'")
		}

		// In 2024-11-05 format, content should be a simple flat array
		content, ok := result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", result["content"])
		}

		// Since v20241105 doesn't support audio, it should be converted to a link
		// Find the link that represents the audio
		var foundAudioAsLink bool
		for _, item := range content {
			contentItem, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			// Audio should be converted to a link type in v20241105
			if contentItem["type"] == "link" {
				// Check if this link is the converted audio by checking the URL
				if url, hasURL := contentItem["url"].(string); hasURL && url == "https://example.com/audio.mp3" {
					foundAudioAsLink = true

					// Check that it has a title
					if _, hasTitle := contentItem["title"].(string); !hasTitle {
						t.Errorf("Missing required 'title' field for link content")
					}

					// Audio-specific fields should not be present
					if _, hasAudioUrl := contentItem["audioUrl"]; hasAudioUrl {
						t.Errorf("audioUrl field should not be present in v20241105 format")
					}
					if _, hasMimeType := contentItem["mimeType"]; hasMimeType {
						t.Errorf("mimeType field should not be present in link format for v20241105")
					}

					break
				}
			}
		}

		if !foundAudioAsLink {
			t.Errorf("No link content representing the audio file found in response")
		}
	})
}

// TestResourceRequest tests resource request handling
func TestResourceRequest(t *testing.T) {
	// Create a server with template and regular resources
	srv := server.NewServer("test-server-v20241105")

	// Register resource with output format according to 2024-11-05 spec
	srv.Resource("/test/resource", "Test resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "Test resource response", nil
	})

	t.Run("valid_resource_URI", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "resources/read",
			"params": {
				"uri": "/test/resource"
			}
		}`)

		// Force 2024-11-05 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2024-11-05")
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

		// Verify response follows 2024-11-05 spec
		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 1)

		// Verify the response has content
		result := response.Result

		// Check for content field - 2024-11-05 uses "content" (NOT "contents")
		if _, hasContents := result["contents"]; hasContents {
			t.Errorf("Response has 'contents' field which is not part of 2024-11-05 spec, should use 'content'")
		}

		// In 2024-11-05 format, content should be a simple flat array of content items
		content, ok := result["content"].([]interface{})
		if !ok {
			t.Fatalf("Expected content to be an array, got %T", result["content"])
		}

		// The content should have at least one item
		if len(content) == 0 {
			t.Fatalf("Expected at least one content item")
		}

		// Now we can access the actual content item
		textItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected text content item to be a map, got %T", content[0])
		}

		if textItem["type"] != "text" {
			t.Errorf("Expected content type to be 'text', got %v", textItem["type"])
		}

		if textItem["text"] != "Test resource response" {
			t.Errorf("Expected text to be 'Test resource response', got %v", textItem["text"])
		}
	})
}

// Helper function to validate JSON-RPC response format
func validateJSONRPCResponse(t *testing.T, jsonrpc string, id, expectedId int) {
	t.Helper()

	// Check JSON-RPC version
	if jsonrpc != "2.0" {
		t.Errorf("Expected jsonrpc version '2.0', got '%s'", jsonrpc)
	}

	// Check ID matches
	if id != expectedId {
		t.Errorf("Expected ID %d, got %d", expectedId, id)
	}
}
