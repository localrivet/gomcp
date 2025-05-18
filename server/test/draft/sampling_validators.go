package draft

import (
	"testing"

	"github.com/localrivet/gomcp/server"
)

// ValidateSamplingContent validates the sampling content structure for the draft version
func ValidateSamplingContent(t *testing.T, content map[string]interface{}) server.SamplingMessageContent {
	t.Helper()

	contentType, ok := content["type"].(string)
	if !ok {
		t.Fatalf("Missing or invalid type in sampling content: %v", content)
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

	default:
		t.Fatalf("Invalid content type: %s", contentType)
	}

	return result
}
