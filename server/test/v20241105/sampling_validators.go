package v20241105

import (
	"testing"

	"github.com/localrivet/gomcp/server"
)

// ValidateSamplingContent validates the sampling content structure for the v20241105 version
// In this version, only "text" and "image" types are supported
func ValidateSamplingContent(t *testing.T, content map[string]interface{}) server.SamplingMessageContent {
	t.Helper()

	contentType, ok := content["type"].(string)
	if !ok {
		t.Fatalf("Missing or invalid type in sampling content: %v", content)
	}

	// v20241105 only supports text and image content types
	if contentType != "text" && contentType != "image" {
		t.Fatalf("v20241105 only supports 'text' and 'image' content types, got: %s", contentType)
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

	case "image":
		data, ok := content["data"].(string)
		if !ok {
			t.Fatalf("Missing or invalid data field for image content: %v", content)
		}
		result.Data = data

		mimeType, ok := content["mimeType"].(string)
		if !ok {
			t.Fatalf("Missing or invalid mimeType field for image content: %v", content)
		}
		result.MimeType = mimeType
	}

	return result
}
