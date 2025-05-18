// Package v20250326 contains tests specifically for the 2025-03-26 version of the MCP specification
package v20250326

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestResourceHandlingV20250326 tests resource handling against 2025-03-26 specification
func TestResourceHandlingV20250326(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-server-2025-03-26")

	// Register a test resource with 2025-03-26 features
	srv.Resource("/test", "A simple test resource for 2025-03-26", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// Return a response that's compatible with the 2025-03-26 spec
		// Directly use the 2025-03-26 structure with contents array
		return map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"uri":  "/test",
					"text": "This is a 2025-03-26 resource response", // Top-level text field is required
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": "This is a 2025-03-26 resource response",
						},
						{
							"type":     "image",
							"imageUrl": "https://example.com/image.png",
							"altText":  "An example image",
						},
					},
				},
			},
			"metadata": map[string]interface{}{
				"version": "2025-03-26",
				"tags":    []string{"test", "example"},
			},
		}, nil
	})

	// Register an image resource
	srv.Resource("/image", "Test image resource", func(ctx *server.Context, args interface{}) (server.ImageResource, error) {
		return server.ImageResource{
			URL:      "https://example.com/image.jpg",
			AltText:  "Test image for 2025-03-26",
			MimeType: "image/jpeg",
		}, nil
	})

	// Register a link resource
	srv.Resource("/link", "Test link resource", func(ctx *server.Context, args interface{}) (server.LinkResource, error) {
		return server.LinkResource{
			URL:   "https://example.com",
			Title: "Example website",
		}, nil
	})

	// Register an empty resource
	srv.Resource("/empty", "Test empty resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return map[string]interface{}{
			"content": []interface{}{},
		}, nil
	})

	// Register a resource with path parameters and query parameters
	srv.Resource("/users/{id}", "Gets a user by ID", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// Check if we got the path parameter
		if params, ok := args.(map[string]interface{}); ok {
			userId, exists := params["id"]
			if !exists {
				return nil, fmt.Errorf("Missing id parameter")
			}

			// In a real implementation, you might use query parameters too
			return map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "User ID: " + userId.(string),
					},
				},
				"metadata": map[string]interface{}{
					"resourceType": "user",
					"timestamp":    "2025-03-26T12:00:00Z",
				},
			}, nil
		}
		return nil, fmt.Errorf("Internal error: args is not a map")
	})

	// Register an audio resource
	srv.Resource("/audio", "Audio resource test", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return server.AudioResource{
			Data:     "base64encodedaudiodata",
			MimeType: "audio/mpeg",
			AltText:  "Test audio for v20250326",
		}, nil
	})

	// Test text+image (mixed) resource
	t.Run("MixedContentResource", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "resources/read",
			"params": {
				"uri": "/test"
			}
		}`)

		// Always use HandleMessageWithVersion to force 2025-03-26 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2025-03-26")
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

		// Verify basic JSON-RPC structure
		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 1)

		// Verify response strictly adheres to 2025-03-26 specification
		validate2025_03_26ResourceResponse(t, response.Result)

		// Additional specific checks for this test case
		resultMap := response.Result

		// Check for metadata (a 2025-03-26 feature)
		metadata, ok := resultMap["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected metadata to be a map, got %T", resultMap["metadata"])
		}

		if metadata["version"] != "2025-03-26" {
			t.Errorf("Expected metadata version to be '2025-03-26', got %v", metadata["version"])
		}

		// Verify the mixed content types (text and image)
		contents := resultMap["contents"].([]interface{})
		contentObj := contents[0].(map[string]interface{})
		content := contentObj["content"].([]interface{})

		// Should have two content items (text, image)
		if len(content) != 2 {
			t.Fatalf("Expected 2 content items, got %d", len(content))
		}

		// Check for text content
		textContent, ok := content[0].(map[string]interface{})
		if !ok || textContent["type"] != "text" {
			t.Errorf("Expected first content item to be text, got %T or type %v",
				content[0], textContent["type"])
		}

		// Check for image content
		imgContent, ok := content[1].(map[string]interface{})
		if !ok || imgContent["type"] != "image" {
			t.Errorf("Expected second content item to be image, got %T or type %v",
				content[1], imgContent["type"])
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

		// Always use HandleMessageWithVersion to force 2025-03-26 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2025-03-26")
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

		// Verify basic JSON-RPC structure
		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 2)

		// Verify response strictly adheres to 2025-03-26 specification
		validate2025_03_26ResourceResponse(t, response.Result)

		// Additional image-specific checks
		result := response.Result
		contents := result["contents"].([]interface{})
		contentObj := contents[0].(map[string]interface{})
		contentItems := contentObj["content"].([]interface{})

		// Find the image content item
		var foundImage bool
		for _, item := range contentItems {
			contentItem, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			if contentItem["type"] == "image" {
				foundImage = true

				if contentItem["imageUrl"] != "https://example.com/image.jpg" {
					t.Errorf("Expected imageUrl to be 'https://example.com/image.jpg', got %v", contentItem["imageUrl"])
				}

				if contentItem["altText"] != "Test image for 2025-03-26" {
					t.Errorf("Expected altText to be 'Test image for 2025-03-26', got %v", contentItem["altText"])
				}

				break
			}
		}

		if !foundImage {
			t.Errorf("No image content item found in response")
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

		// Always use HandleMessageWithVersion to force 2025-03-26 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2025-03-26")
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

		// Verify basic JSON-RPC structure
		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 3)

		// Verify response strictly adheres to 2025-03-26 specification
		validate2025_03_26ResourceResponse(t, response.Result)

		// Additional link-specific checks
		result := response.Result
		contents := result["contents"].([]interface{})
		contentObj := contents[0].(map[string]interface{})
		contentItems := contentObj["content"].([]interface{})

		// Find the link content item
		var foundLink bool
		for _, item := range contentItems {
			contentItem, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			if contentItem["type"] == "link" {
				foundLink = true

				if contentItem["url"] != "https://example.com" {
					t.Errorf("Expected url to be 'https://example.com', got %v", contentItem["url"])
				}

				if contentItem["title"] != "Example website" {
					t.Errorf("Expected title to be 'Example website', got %v", contentItem["title"])
				}

				break
			}
		}

		if !foundLink {
			t.Errorf("No link content item found in response")
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

		// Always use HandleMessageWithVersion to force 2025-03-26 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2025-03-26")
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

		// Verify basic JSON-RPC structure
		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 4)

		// Verify we have a result
		result := response.Result
		if result == nil {
			t.Fatalf("Expected a result, got nil")
		}

		// Check for contents field - 2025-03-26 uses "contents" (NOT "content")
		if _, hasContent := result["content"]; hasContent {
			t.Errorf("Response has 'content' field which is not part of 2025-03-26 spec, should use 'contents'")
		}

		// Check for contents array (required in 2025-03-26)
		contents, ok := result["contents"].([]interface{})
		if !ok {
			t.Fatalf("Expected contents to be an array according to 2025-03-26 spec, got %T", result["contents"])
		}

		if len(contents) == 0 {
			t.Fatalf("Expected at least one content item in contents array - even for empty resources")
		}

		// For empty resources, the content array inside the contents item should be empty
		contentItem := contents[0].(map[string]interface{})

		// Empty resource still must have URI and text fields according to spec
		if _, hasURI := contentItem["uri"]; !hasURI {
			t.Errorf("Missing required 'uri' field in contents[0] according to 2025-03-26 spec")
		}

		if _, hasText := contentItem["text"]; !hasText {
			t.Errorf("Missing required 'text' field in contents[0] according to 2025-03-26 spec")
		}

		// But the inner content array should be empty
		innerContent, hasInnerContent := contentItem["content"]
		if !hasInnerContent {
			t.Errorf("Missing required 'content' field in contents[0] according to 2025-03-26 spec")
		} else {
			innerContentArr, ok := innerContent.([]interface{})
			if !ok {
				t.Errorf("Expected 'content' field to be an array in contents[0], got %T", innerContent)
			} else if len(innerContentArr) != 0 {
				t.Errorf("Expected empty content array, got array with %d items", len(innerContentArr))
			}
		}
	})

	// Test resource with path parameters
	t.Run("PathParameters", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 5,
			"method": "resources/read",
			"params": {
				"uri": "/users/42",
				"parameters": {
					"id": "42"
				}
			}
		}`)

		// Always use HandleMessageWithVersion to force 2025-03-26 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2025-03-26")
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

		// Verify basic JSON-RPC structure
		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 5)

		// Verify response strictly adheres to 2025-03-26 specification
		validate2025_03_26ResourceResponse(t, response.Result)

		// Additional checks for path parameter handling
		result := response.Result

		// Check for metadata which should contain user ID info
		metadata, ok := result["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected metadata to be a map, got %T", result["metadata"])
		}

		if metadata["resourceType"] != "user" {
			t.Errorf("Expected resourceType to be 'user', got %v", metadata["resourceType"])
		}

		// Check content for the user ID
		contents := result["contents"].([]interface{})
		contentObj := contents[0].(map[string]interface{})
		contentItems := contentObj["content"].([]interface{})

		// Find the text content item with user info
		var foundUserInfo bool
		for _, item := range contentItems {
			contentItem, ok := item.(map[string]interface{})
			if !ok || contentItem["type"] != "text" {
				continue
			}

			text, ok := contentItem["text"].(string)
			if !ok {
				continue
			}

			if text == "User ID: 42" {
				foundUserInfo = true
				break
			}
		}

		if !foundUserInfo {
			t.Errorf("No text content with 'User ID: 42' found in response")
		}
	})

	// Test resource list
	t.Run("ResourceList", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 6,
			"method": "resources/list"
		}`)

		// Always use HandleMessageWithVersion to force 2025-03-26 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2025-03-26")
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

		// Verify basic JSON-RPC structure
		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 6)

		// Verify resources list structure according to 2025-03-26 spec
		if len(response.Result.Resources) < 4 {
			t.Fatalf("Expected at least 4 resources, got %d", len(response.Result.Resources))
		}

		// Verify each resource has required fields according to 2025-03-26 spec
		for i, resource := range response.Result.Resources {
			if resource["uri"] == nil {
				t.Errorf("Missing required 'uri' field in resource %d", i)
			}

			if resource["name"] == nil {
				t.Errorf("Missing required 'name' field in resource %d", i)
			}

			// 2025-03-26 should have mimeType or size or description as optional fields
			hasOptionalField := resource["mimeType"] != nil ||
				resource["size"] != nil ||
				resource["description"] != nil

			if !hasOptionalField {
				t.Logf("Resource %d lacks any optional fields (mimeType, size, description)", i)
			}
		}
	})

	// Test audio resource
	t.Run("AudioResource", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 7,
			"method": "resources/read",
			"params": {
				"uri": "/audio"
			}
		}`)

		// Always use HandleMessageWithVersion to force 2025-03-26 version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "2025-03-26")
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

		// Verify response follows 2025-03-26 spec
		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 7)
		validate2025_03_26ResourceResponse(t, response.Result)

		// Additional audio-specific checks
		result := response.Result
		contents := result["contents"].([]interface{})
		contentObj := contents[0].(map[string]interface{})
		contentItems := contentObj["content"].([]interface{})

		// Find the audio content item
		var foundAudio bool
		for _, item := range contentItems {
			contentItem, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			if contentItem["type"] == "audio" {
				foundAudio = true

				// Check for required fields in v20250326 format
				if _, hasData := contentItem["data"].(string); !hasData {
					t.Errorf("Missing required 'data' field for audio content")
				}

				if mimeType, hasMimeType := contentItem["mimeType"].(string); !hasMimeType {
					t.Errorf("Missing required 'mimeType' field for audio content")
				} else if mimeType != "audio/mpeg" {
					t.Errorf("Expected mimeType to be 'audio/mpeg', got %v", mimeType)
				}

				// audioUrl should not be present in v20250326 format
				if _, hasAudioUrl := contentItem["audioUrl"]; hasAudioUrl {
					t.Errorf("audioUrl field should not be present in v20250326 format")
				}

				break
			}
		}

		if !foundAudio {
			t.Errorf("No audio content item found in response")
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

// Helper function to validate 2025-03-26 resource response structure
func validate2025_03_26ResourceResponse(t *testing.T, result map[string]interface{}) {
	t.Helper()

	// 2025-03-26 spec requires "contents" field, not "content"
	if _, hasContent := result["content"]; hasContent {
		t.Errorf("Response has 'content' field which is not part of 2025-03-26 spec, should use 'contents'")
	}

	// Check for contents array (required in 2025-03-26)
	contents, ok := result["contents"].([]interface{})
	if !ok {
		t.Fatalf("Expected contents to be an array according to 2025-03-26 spec, got %T", result["contents"])
	}

	// Each content item in contents should have the required fields
	for i, item := range contents {
		contentItem, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected contents[%d] to be a map, got %T", i, item)
		}

		// URI is required
		if _, hasURI := contentItem["uri"]; !hasURI {
			t.Errorf("Missing required 'uri' field in contents[%d] according to 2025-03-26 spec", i)
		}

		// Text or blob field is required at the top level
		hasText := contentItem["text"] != nil
		hasBlob := contentItem["blob"] != nil

		if !hasText && !hasBlob {
			t.Errorf("Missing required 'text' or 'blob' field in contents[%d] according to 2025-03-26 spec", i)
		}

		// Must contain content array
		innerContent, hasInnerContent := contentItem["content"]
		if !hasInnerContent {
			t.Errorf("Missing required 'content' field in contents[%d] according to 2025-03-26 spec", i)
			continue
		}

		// The inner content should be an array, possibly empty
		innerContentArr, ok := innerContent.([]interface{})
		if !ok {
			t.Errorf("Expected 'content' field to be an array in contents[%d], got %T", i, innerContent)
			continue
		}

		// Validate each content item if there are any
		for j, contentItem := range innerContentArr {
			validateContentItem(t, contentItem, i, j)
		}
	}
}

// Helper function to validate content items
func validateContentItem(t *testing.T, item interface{}, contentIndex, itemIndex int) {
	t.Helper()

	contentItem, ok := item.(map[string]interface{})
	if !ok {
		t.Errorf("Expected contents[%d].content[%d] to be a map, got %T", contentIndex, itemIndex, item)
		return
	}

	// Verify type field is present
	contentType, hasType := contentItem["type"].(string)
	if !hasType {
		t.Errorf("Missing required 'type' field in contents[%d].content[%d]", contentIndex, itemIndex)
		return
	}

	// Verify required fields based on type
	switch contentType {
	case "text":
		if _, hasText := contentItem["text"].(string); !hasText {
			t.Errorf("Missing required 'text' field for text-type content in contents[%d].content[%d]", contentIndex, itemIndex)
		}
	case "image":
		if _, hasUrl := contentItem["imageUrl"].(string); !hasUrl {
			t.Errorf("Missing required 'imageUrl' field for image-type content in contents[%d].content[%d]", contentIndex, itemIndex)
		}
		if _, hasAlt := contentItem["altText"].(string); !hasAlt {
			t.Errorf("Missing required 'altText' field for image-type content in contents[%d].content[%d]", contentIndex, itemIndex)
		}
	case "link":
		if _, hasUrl := contentItem["url"].(string); !hasUrl {
			t.Errorf("Missing required 'url' field for link-type content in contents[%d].content[%d]", contentIndex, itemIndex)
		}
		if _, hasTitle := contentItem["title"].(string); !hasTitle {
			t.Errorf("Missing required 'title' field for link-type content in contents[%d].content[%d]", contentIndex, itemIndex)
		}
	case "audio":
		if _, hasData := contentItem["data"].(string); !hasData {
			t.Errorf("Missing required 'data' field for audio-type content in contents[%d].content[%d]", contentIndex, itemIndex)
		}
		if _, hasMimeType := contentItem["mimeType"].(string); !hasMimeType {
			t.Errorf("Missing required 'mimeType' field for audio-type content in contents[%d].content[%d]", contentIndex, itemIndex)
		}
	}
}
