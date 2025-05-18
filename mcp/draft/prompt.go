package draft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"text/template"
)

// PromptDefinition represents a prompt definition
type PromptDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Template    string          `json:"template"`
	Parameters  interface{}     `json:"parameters"`
	Metadata    interface{}     `json:"metadata,omitempty"`
	ContentType string          `json:"content_type,omitempty"` // New in draft - enables non-text templates
	Format      string          `json:"format,omitempty"`       // New in draft - "text", "markdown", "json", etc.
	Version     string          `json:"version,omitempty"`      // New in draft
	Deprecated  bool            `json:"deprecated,omitempty"`   // New in draft
	Category    string          `json:"category,omitempty"`     // New in draft
	Tags        []string        `json:"tags,omitempty"`         // New in draft
	Examples    []PromptExample `json:"examples,omitempty"`     // New in draft
}

// PromptExample represents an example prompt usage (new in draft)
type PromptExample struct {
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Result      string                 `json:"result,omitempty"`
}

// PromptMetadata represents metadata for a prompt (new in draft)
type PromptMetadata struct {
	Author      string   `json:"author,omitempty"`
	Version     string   `json:"version,omitempty"`
	Created     string   `json:"created,omitempty"`
	Modified    string   `json:"modified,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Category    string   `json:"category,omitempty"`
	License     string   `json:"license,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Repository  string   `json:"repository,omitempty"`
	InlineStyle bool     `json:"inline_style,omitempty"` // Whether styles should be inlined in the output
	MaxTokens   int      `json:"max_tokens,omitempty"`   // Recommended token limit
}

// PromptParameter represents a parameter for a prompt template (new in draft)
type PromptParameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // "string", "number", "boolean", "array", "object"
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"`   // Possible values
	Format      string      `json:"format,omitempty"` // Format hint (e.g., "date", "email", "uri")
}

// RenderPrompt renders a prompt with the given parameters
func RenderPrompt(promptDef PromptDefinition, parameters map[string]interface{}) ([]Content, error) {
	// Create template instance
	tmpl, err := template.New(promptDef.Name).Parse(promptDef.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to parse prompt template: %w", err)
	}

	// Validate parameters
	if err := validatePromptParameters(promptDef.Parameters, parameters); err != nil {
		return nil, err
	}

	// Render template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, parameters); err != nil {
		return nil, fmt.Errorf("failed to render prompt template: %w", err)
	}

	// Convert to content
	content := TextContent{
		Type: "text",
		Text: buf.String(),
	}

	// For JSON format, try to parse the rendered template as JSON and return structured content
	if promptDef.Format == "json" {
		var structuredData interface{}
		if err := json.Unmarshal(buf.Bytes(), &structuredData); err == nil {
			return []Content{
				StructuredContent{
					Type:    "json",
					Content: structuredData,
				},
			}, nil
		}
		// Fallback to text if JSON parsing fails
	} else if promptDef.Format == "markdown" {
		// For markdown, use the markdown content type
		content.Type = "markdown"
	}

	return []Content{content}, nil
}

// validatePromptParameters validates input parameters against the prompt definition
func validatePromptParameters(paramDef interface{}, params map[string]interface{}) error {
	// Skip validation if no parameter definition
	if paramDef == nil {
		return nil
	}

	// Get parameter definition as map
	var paramDefMap map[string]interface{}
	switch v := paramDef.(type) {
	case map[string]interface{}:
		paramDefMap = v
	default:
		// Try to convert to JSON and back to map
		paramJSON, err := json.Marshal(paramDef)
		if err != nil {
			return fmt.Errorf("invalid parameter definition format: %w", err)
		}
		if err := json.Unmarshal(paramJSON, &paramDefMap); err != nil {
			return fmt.Errorf("invalid parameter definition format: %w", err)
		}
	}

	// Check for required parameters
	if properties, ok := paramDefMap["properties"].(map[string]interface{}); ok {
		var requiredFields []string
		if required, ok := paramDefMap["required"].([]interface{}); ok {
			for _, field := range required {
				if fieldStr, ok := field.(string); ok {
					requiredFields = append(requiredFields, fieldStr)
				}
			}
		}

		for field, propDef := range properties {
			// Check if field is required
			isRequired := false
			for _, reqField := range requiredFields {
				if reqField == field {
					isRequired = true
					break
				}
			}

			if isRequired {
				if value, exists := params[field]; !exists || isZeroValue(value) {
					return fmt.Errorf("required parameter missing: %s", field)
				}
			}

			// Type validation for provided parameters
			if value, exists := params[field]; exists {
				propDefMap, ok := propDef.(map[string]interface{})
				if !ok {
					continue
				}

				if typeStr, ok := propDefMap["type"].(string); ok {
					if err := validateParameterType(field, value, typeStr); err != nil {
						return err
					}
				}

				// Enum validation
				if enum, ok := propDefMap["enum"].([]interface{}); ok {
					if err := validateEnum(field, value, enum); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// isZeroValue checks if a value is the zero value for its type
func isZeroValue(value interface{}) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	}
	return false
}

// validateParameterType validates that the parameter is of the expected type
func validateParameterType(field string, value interface{}, expectedType string) error {
	if value == nil {
		return nil // Nil values are handled by required check
	}

	v := reflect.ValueOf(value)

	switch expectedType {
	case "string":
		if v.Kind() != reflect.String {
			return fmt.Errorf("parameter %s must be a string", field)
		}
	case "number":
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
			// These are all numeric types
		default:
			return fmt.Errorf("parameter %s must be a number", field)
		}
	case "integer":
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			// These are all integer types
		default:
			return fmt.Errorf("parameter %s must be an integer", field)
		}
	case "boolean":
		if v.Kind() != reflect.Bool {
			return fmt.Errorf("parameter %s must be a boolean", field)
		}
	case "array":
		if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
			return fmt.Errorf("parameter %s must be an array", field)
		}
	case "object":
		if v.Kind() != reflect.Map && v.Kind() != reflect.Struct {
			return fmt.Errorf("parameter %s must be an object", field)
		}
	}

	return nil
}

// validateEnum validates that the parameter value is one of the allowed enum values
func validateEnum(field string, value interface{}, enum []interface{}) error {
	for _, allowedValue := range enum {
		if reflect.DeepEqual(value, allowedValue) {
			return nil
		}
	}
	return fmt.Errorf("parameter %s must be one of the allowed values", field)
}

// ValidatePromptDefinition validates a prompt definition
func ValidatePromptDefinition(promptDef PromptDefinition) error {
	if promptDef.Name == "" {
		return ErrInvalidPromptDefinition("prompt name is required")
	}
	if promptDef.Description == "" {
		return ErrInvalidPromptDefinition("prompt description is required")
	}
	if promptDef.Template == "" {
		return ErrInvalidPromptDefinition("prompt template is required")
	}

	// Verify template can be parsed
	_, err := template.New(promptDef.Name).Parse(promptDef.Template)
	if err != nil {
		return ErrInvalidPromptDefinition(fmt.Sprintf("invalid template: %v", err))
	}

	// Check for content type and format compatibility
	if promptDef.ContentType != "" && promptDef.Format != "" {
		if !isContentTypeCompatibleWithFormat(promptDef.ContentType, promptDef.Format) {
			return ErrInvalidPromptDefinition(fmt.Sprintf("incompatible content type (%s) and format (%s)",
				promptDef.ContentType, promptDef.Format))
		}
	}

	return nil
}

// isContentTypeCompatibleWithFormat checks if content type and format are compatible (new in draft)
func isContentTypeCompatibleWithFormat(contentType, format string) bool {
	compatibilityMap := map[string][]string{
		"text/plain":       {"text"},
		"text/markdown":    {"markdown", "text"},
		"application/json": {"json"},
		"text/html":        {"html", "text"},
		"application/xml":  {"xml", "text"},
	}

	if allowedFormats, ok := compatibilityMap[contentType]; ok {
		for _, allowedFormat := range allowedFormats {
			if format == allowedFormat {
				return true
			}
		}
		return false
	}

	// If content type is not in our map, we assume it's compatible
	return true
}

// ErrInvalidPromptDefinition represents an error for an invalid prompt definition
type ErrInvalidPromptDefinition string

func (e ErrInvalidPromptDefinition) Error() string {
	return string(e)
}
