// Package draft contains tests specifically for the draft version of the MCP specification
package draft

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestResourceHandlingDraft tests resource handling against draft specification
func TestResourceHandlingDraft(t *testing.T) {
	// Create a server
	srv := server.NewServer("test-server-draft")

	// Register a test resource with advanced content types (text, image, audio)
	srv.Resource("/test", "A simple test resource for draft spec", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// Return a response with multiple content types for draft spec
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "This is a draft spec resource response",
				},
				{
					"type":     "image",
					"imageUrl": "https://example.com/image.png",
					"altText":  "An example image",
					"mimeType": "image/png",
				},
				{
					"type":     "audio",
					"audioUrl": "https://example.com/audio.mp3",
					"altText":  "An example audio file",
					"mimeType": "audio/mpeg",
				},
			},
			"metadata": map[string]interface{}{
				"version":   "draft",
				"tags":      []string{"test", "example", "draft"},
				"generated": true,
			},
		}, nil
	})

	// Register an image resource
	srv.Resource("/image", "Test image resource", func(ctx *server.Context, args interface{}) (server.ImageResource, error) {
		return server.ImageResource{
			URL:      "https://example.com/image.jpg",
			AltText:  "Test image for draft spec",
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

	// Register a resource with path parameters
	srv.Resource("/users/{id}", "Gets a user by ID", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// Check if we got the path parameter
		if params, ok := args.(map[string]interface{}); ok {
			userId, exists := params["id"]
			if !exists {
				return nil, fmt.Errorf("Missing id parameter")
			}
			return map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "User ID: " + userId.(string),
					},
				},
				"metadata": map[string]interface{}{
					"resourceType": "user",
					"timestamp":    "2025-05-15T12:00:00Z", // Future date for draft spec
				},
			}, nil
		}
		return nil, fmt.Errorf("Internal error: args is not a map")
	})

	// Register an audio resource
	srv.Resource("/audio", "Audio resource test", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return server.AudioResource{
			URL:      "https://example.com/audio.mp3",
			MimeType: "audio/mpeg",
			AltText:  "Test audio for draft spec",
		}, nil
	})

	// Test mixed content resource
	t.Run("MixedContentResource", func(t *testing.T) {
		requestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "resources/read",
			"params": {
				"uri": "/test"
			}
		}`)

		// Use HandleMessageWithVersion to ensure draft version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "draft")
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

		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 1)

		// Verify the response structure according to draft specification
		validateDraftResourceResponse(t, response.Result)

		// Additional specific checks for this test
		resultMap := response.Result

		// Check for metadata (enhanced in draft)
		metadata, ok := resultMap["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected metadata to be a map, got %T", resultMap["metadata"])
		}

		if metadata["version"] != "draft" {
			t.Errorf("Expected metadata version to be 'draft', got %v", metadata["version"])
		}

		// Check for generated flag
		if generated, ok := metadata["generated"].(bool); !ok || !generated {
			t.Errorf("Expected generated flag to be true, got %v", metadata["generated"])
		}

		// Verify the content types (should have text, image, and audio)
		contents := resultMap["contents"].([]interface{})
		contentObj := contents[0].(map[string]interface{})
		content := contentObj["content"].([]interface{})

		// Should have three content items (text, image, audio)
		if len(content) != 3 {
			t.Fatalf("Expected 3 content items, got %d", len(content))
		}

		// Map to track which types we've found
		foundTypes := make(map[string]bool)

		for _, item := range content {
			contentItem, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			contentType, ok := contentItem["type"].(string)
			if !ok {
				continue
			}

			foundTypes[contentType] = true

			// Special check for audio content
			if contentType == "audio" && contentItem["mimeType"] != "audio/mpeg" {
				t.Errorf("Expected audio mimeType to be 'audio/mpeg', got %v", contentItem["mimeType"])
			}
		}

		// Verify we found all required content types
		if !foundTypes["text"] {
			t.Errorf("Missing text content type in response")
		}
		if !foundTypes["image"] {
			t.Errorf("Missing image content type in response")
		}
		if !foundTypes["audio"] {
			t.Errorf("Missing audio content type in response")
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

		// Use HandleMessageWithVersion to ensure draft version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "draft")
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

		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 2)
		validateDraftResourceResponse(t, response.Result)

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

				if contentItem["altText"] != "Test image for draft spec" {
					t.Errorf("Expected altText to be 'Test image for draft spec', got %v", contentItem["altText"])
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

		// Use HandleMessageWithVersion to ensure draft version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "draft")
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

		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 3)
		validateDraftResourceResponse(t, response.Result)

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

		// Use HandleMessageWithVersion to ensure draft version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "draft")
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

		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 4)

		// Verify we have a result
		result := response.Result
		if result == nil {
			t.Fatalf("Expected a result, got nil")
		}

		// Check for contents field - draft uses "contents" (NOT "content")
		if _, hasContent := result["content"]; hasContent {
			t.Errorf("Response has 'content' field which is not part of draft spec, should use 'contents'")
		}

		// Check for contents array (required in draft)
		contents, ok := result["contents"].([]interface{})
		if !ok {
			t.Fatalf("Expected contents to be an array according to draft spec, got %T", result["contents"])
		}

		if len(contents) == 0 {
			t.Fatalf("Expected at least one content item in contents array - even for empty resources")
		}

		// For empty resources, the content array inside the contents item should be empty
		contentItem := contents[0].(map[string]interface{})

		// Empty resource still must have URI and text fields according to spec
		if _, hasURI := contentItem["uri"]; !hasURI {
			t.Errorf("Missing required 'uri' field in contents[0] according to draft spec")
		}

		if _, hasText := contentItem["text"]; !hasText {
			t.Errorf("Missing required 'text' field in contents[0] according to draft spec")
		}

		// But the inner content array should be empty
		innerContent, hasInnerContent := contentItem["content"]
		if !hasInnerContent {
			t.Errorf("Missing required 'content' field in contents[0] according to draft spec")
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

		// Use HandleMessageWithVersion to ensure draft version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "draft")
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

		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 5)
		validateDraftResourceResponse(t, response.Result)

		// Additional checks for path parameter handling
		result := response.Result

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

		// Use HandleMessageWithVersion to ensure draft version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "draft")
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

		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 6)

		// Verify resources list structure according to draft spec
		if len(response.Result.Resources) < 4 {
			t.Fatalf("Expected at least 4 resources, got %d", len(response.Result.Resources))
		}

		// Verify each resource has required fields according to draft spec
		for i, resource := range response.Result.Resources {
			if resource["uri"] == nil {
				t.Errorf("Missing required 'uri' field in resource %d", i)
			}

			if resource["name"] == nil {
				t.Errorf("Missing required 'name' field in resource %d", i)
			}

			// Draft should have kind and description
			if resource["kind"] == nil {
				t.Errorf("Missing required 'kind' field in resource %d", i)
			}

			if resource["description"] == nil {
				t.Errorf("Missing required 'description' field in resource %d", i)
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

		// Use HandleMessageWithVersion to ensure draft version
		responseBytes, err := server.HandleMessageWithVersion(srv, requestJSON, "draft")
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

		validateJSONRPCResponse(t, response.JSONRPC, response.ID, 7)
		validateDraftResourceResponse(t, response.Result)

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

				// Check for required fields in draft format
				if audioUrl, hasAudioUrl := contentItem["audioUrl"].(string); !hasAudioUrl {
					t.Errorf("Missing required 'audioUrl' field for audio content")
				} else if audioUrl != "https://example.com/audio.mp3" {
					t.Errorf("Expected audioUrl to be 'https://example.com/audio.mp3', got %v", audioUrl)
				}

				if mimeType, hasMimeType := contentItem["mimeType"].(string); !hasMimeType {
					t.Errorf("Missing required 'mimeType' field for audio content")
				} else if mimeType != "audio/mpeg" {
					t.Errorf("Expected mimeType to be 'audio/mpeg', got %v", mimeType)
				}

				// data field should not be present in draft format
				if _, hasData := contentItem["data"]; hasData {
					t.Errorf("data field should not be present in draft format")
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

// Helper function to validate draft resource response structure (which follows 2025-03-26 format)
func validateDraftResourceResponse(t *testing.T, result map[string]interface{}) {
	t.Helper()

	// Draft spec requires "contents" field, not "content"
	if _, hasContent := result["content"]; hasContent {
		t.Errorf("Response has 'content' field which is not part of draft spec, should use 'contents'")
	}

	// Check for contents array (required in draft)
	contents, ok := result["contents"].([]interface{})
	if !ok {
		t.Fatalf("Expected contents to be an array according to draft spec, got %T", result["contents"])
	}

	// Each content item in contents should have the required fields
	for i, item := range contents {
		contentItem, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected contents[%d] to be a map, got %T", i, item)
		}

		// URI is required
		if _, hasURI := contentItem["uri"]; !hasURI {
			t.Errorf("Missing required 'uri' field in contents[%d] according to draft spec", i)
		}

		// Text or blob field is required at the top level
		hasText := contentItem["text"] != nil
		hasBlob := contentItem["blob"] != nil

		if !hasText && !hasBlob {
			t.Errorf("Missing required 'text' or 'blob' field in contents[%d] according to draft spec", i)
		}

		// Must contain content array
		innerContent, hasInnerContent := contentItem["content"]
		if !hasInnerContent {
			t.Errorf("Missing required 'content' field in contents[%d] according to draft spec", i)
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
		// In the draft spec, audio uses audioUrl field
		if _, hasAudioUrl := contentItem["audioUrl"].(string); !hasAudioUrl {
			t.Errorf("Missing required 'audioUrl' field for audio-type content in contents[%d].content[%d]", contentIndex, itemIndex)
		}

		if _, hasMimeType := contentItem["mimeType"].(string); !hasMimeType {
			t.Errorf("Missing required 'mimeType' field for audio-type content in contents[%d].content[%d]", contentIndex, itemIndex)
		}
	}
}

// Helper function to get all keys from a map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
