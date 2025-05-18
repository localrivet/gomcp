package test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/localrivet/gomcp/server"
)

func TestFormatResourceResponse(t *testing.T) {
	testCases := []struct {
		name           string
		uri            string
		result         interface{}
		version        string
		validateFields []string // Fields expected to exist at various levels
	}{
		{
			name:           "String result with 2024-11-05 version",
			uri:            "/test/resource",
			result:         "Hello, World!",
			version:        "2024-11-05",
			validateFields: []string{"content.0.type", "content.0.text"},
		},
		{
			name:           "String result with 2025-03-26 version",
			uri:            "/test/resource",
			result:         "Hello, World!",
			version:        "2025-03-26",
			validateFields: []string{"contents.0.uri", "contents.0.text", "contents.0.content.0.type", "contents.0.content.0.text"},
		},
		{
			name:           "Map result with 2024-11-05 version",
			uri:            "/test/resource",
			result:         map[string]interface{}{"message": "Hello, World!"},
			version:        "2024-11-05",
			validateFields: []string{"content.0.type", "content.0.text"},
		},
		{
			name:           "Map result with 2025-03-26 version",
			uri:            "/test/resource",
			result:         map[string]interface{}{"message": "Hello, World!"},
			version:        "2025-03-26",
			validateFields: []string{"contents.0.uri", "contents.0.text", "contents.0.content.0.type", "contents.0.content.0.text"},
		},
		{
			name:           "Map with contents array for 2025-03-26",
			uri:            "/test/resource",
			result:         map[string]interface{}{"contents": []interface{}{map[string]interface{}{"uri": "/test/resource"}}},
			version:        "2025-03-26",
			validateFields: []string{"contents.0.uri", "contents.0.text"},
		},
		{
			name:           "Text resource for 2025-03-26",
			uri:            "/test/resource",
			result:         server.TextResource{Text: "Text resource content"},
			version:        "2025-03-26",
			validateFields: []string{"contents.0.text"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response := server.FormatResourceResponse(tc.uri, tc.result, tc.version)

			// Convert to JSON and back to ensure we can access nested fields correctly
			jsonData, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("Failed to marshal response: %v", err)
			}

			var parsed map[string]interface{}
			if err := json.Unmarshal(jsonData, &parsed); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			// Print the formatted JSON response for debugging
			t.Logf("Response: %s", string(jsonData))

			// Validate required fields
			for _, field := range tc.validateFields {
				value, exists := getNestedField(parsed, field)
				if !exists {
					t.Errorf("Missing required field: %s", field)
					continue
				}

				// Ensure text fields are never empty
				if strings.HasSuffix(field, ".text") && value == "" {
					t.Errorf("Field %s has empty string value", field)
				}
			}
		})
	}
}

// getNestedField retrieves a value from a nested map by path notation (e.g., "contents.0.text")
func getNestedField(data map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")

	var current interface{} = data

	for i, part := range parts {
		switch c := current.(type) {
		case map[string]interface{}:
			// If this part is an array index, get the array first
			if i < len(parts)-1 && isNumeric(parts[i+1]) {
				var ok bool
				current, ok = c[part]
				if !ok {
					return nil, false
				}
				continue
			}

			// Regular map access
			var ok bool
			current, ok = c[part]
			if !ok {
				return nil, false
			}

		case []interface{}:
			// Array access by index
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(c) {
				return nil, false
			}
			current = c[idx]

		default:
			// We've hit a leaf node too early
			return nil, false
		}
	}

	return current, true
}

// isNumeric checks if a string represents a valid numeric index
func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
