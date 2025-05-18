package v20250326

import (
	"testing"

	"github.com/localrivet/gomcp/server"
)

// ValidateSamplingContent validates the sampling content structure for the v20250326 version
// This version supports "text", "image", and "audio" content types
func ValidateSamplingContent(t *testing.T, content map[string]interface{}) server.SamplingMessageContent {
	t.Helper()

	contentType, ok := content["type"].(string)
	if !ok {
		t.Fatalf("Missing or invalid type in sampling content: %v", content)
	}

	// v20250326 supports text, image, and audio content types
	if contentType != "text" && contentType != "image" && contentType != "audio" {
		t.Fatalf("v20250326 only supports 'text', 'image', and 'audio' content types, got: %s", contentType)
	}

	result := server.SamplingMessageContent{
		Type: contentType,
	}

	switch contentType {
	case "text":
		text, ok := content["text"].(string)
		if !ok {
			t.Fatalf("Missing or invalid text field for text content: %v", content)
		}
		result.Text = text

	case "image", "audio":
		data, ok := content["data"].(string)
		if !ok {
			t.Fatalf("Missing or invalid data field for %s content: %v", contentType, content)
		}
		result.Data = data

		mimeType, ok := content["mimeType"].(string)
		if !ok {
			t.Fatalf("Missing or invalid mimeType field for %s content: %v", contentType, content)
		}
		result.MimeType = mimeType
	}

	return result
}
