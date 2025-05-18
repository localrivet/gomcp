package v20241105

import "encoding/json"

// ToolDefinition represents an MCP tool definition
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

// ToolInputSchema represents a JSON Schema for tool input
type ToolInputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]PropertyDetail `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

// PropertyDetail represents a JSON Schema property definition
type PropertyDetail struct {
	Type        string        `json:"type"`
	Description string        `json:"description,omitempty"`
	Enum        []interface{} `json:"enum,omitempty"`
	Format      string        `json:"format,omitempty"`
	Minimum     *float64      `json:"minimum,omitempty"`
	Maximum     *float64      `json:"maximum,omitempty"`
	MinLength   *int          `json:"minLength,omitempty"`
	MaxLength   *int          `json:"maxLength,omitempty"`
	Pattern     string        `json:"pattern,omitempty"`
	Default     interface{}   `json:"default,omitempty"`
}

// ToolMetadata represents metadata for a tool
type ToolMetadata struct {
	Version     string                 `json:"version,omitempty"`
	Author      string                 `json:"author,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// ValidateToolDefinition validates a tool definition
func ValidateToolDefinition(tool ToolDefinition) error {
	if tool.Name == "" {
		return ErrInvalidToolDefinition("tool name is required")
	}
	if tool.Description == "" {
		return ErrInvalidToolDefinition("tool description is required")
	}
	if len(tool.Schema) == 0 {
		return ErrInvalidToolDefinition("tool schema is required")
	}
	return nil
}

// ErrInvalidToolDefinition represents an error for an invalid tool definition
type ErrInvalidToolDefinition string

func (e ErrInvalidToolDefinition) Error() string {
	return string(e)
}
