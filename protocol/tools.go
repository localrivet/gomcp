// Package protocol defines the structures and constants for the Model Context Protocol (MCP).
package protocol

import (
	"encoding/json" // Added for UnmarshalJSON
	"fmt"           // Added for UnmarshalJSON
	"log"           // Added for UnmarshalJSON
)

// --- Tooling Structures and Messages ---

// ToolInputSchema defines the expected input structure for a tool (JSON Schema subset).
type ToolInputSchema struct {
	Type       string                    `json:"type"` // Typically "object"
	Properties map[string]PropertyDetail `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}

// PropertyDetail describes a single parameter within a ToolInputSchema.
type PropertyDetail struct {
	Type        string        `json:"type"`
	Description string        `json:"description,omitempty"`
	Enum        []interface{} `json:"enum,omitempty"`   // Possible values for the property
	Format      string        `json:"format,omitempty"` // Specific format (e.g., "date-time", "email")
}

// ToolAnnotations provides optional hints about tool behavior.
type ToolAnnotations struct {
	Title           string `json:"title,omitempty"`           // Optional human-readable title for the tool.
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty"`    // Use pointer for optional boolean
	DestructiveHint *bool  `json:"destructiveHint,omitempty"` // Use pointer for optional boolean
	IdempotentHint  *bool  `json:"idempotentHint,omitempty"`  // Use pointer for optional boolean
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`   // Use pointer for optional boolean
}

// Tool defines a tool offered by the server.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema ToolInputSchema `json:"inputSchema"`
	Annotations ToolAnnotations `json:"annotations,omitempty"`
}

// ListToolsRequestParams defines the parameters for a 'tools/list' request.
type ListToolsRequestParams struct {
	Cursor string `json:"cursor,omitempty"`
}

// ListToolsResult defines the result payload for a successful 'tools/list' response.
type ListToolsResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// CallToolParams defines the parameters for a 'tools/call' request.
type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	Meta      *RequestMeta           `json:"_meta,omitempty"` // Defined in messages.go
}

// CallToolResult defines the result payload for a successful 'tools/call' response.
type CallToolResult struct {
	Content []Content    `json:"content"` // Defined in messages.go
	IsError *bool        `json:"isError,omitempty"`
	Meta    *RequestMeta `json:"_meta,omitempty"` // Defined in messages.go
}

// UnmarshalJSON implements custom unmarshalling for CallToolResult to handle the Content interface slice.
func (r *CallToolResult) UnmarshalJSON(data []byte) error {
	// 1. Define an auxiliary type to prevent recursion
	type Alias CallToolResult
	aux := &struct {
		Content []json.RawMessage `json:"content"` // Unmarshal Content into RawMessage first
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	// 2. Unmarshal into the auxiliary type
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal base CallToolResult: %w", err)
	}

	// 3. Iterate over RawMessages and unmarshal into concrete types
	r.Content = make([]Content, 0, len(aux.Content)) // Initialize the slice
	for _, raw := range aux.Content {
		var typeDetect struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &typeDetect); err != nil {
			return fmt.Errorf("failed to detect content type: %w", err)
		}

		var actualContent Content
		switch typeDetect.Type {
		case "text":
			var tc TextContent // Defined in messages.go
			if err := json.Unmarshal(raw, &tc); err != nil {
				return fmt.Errorf("failed to unmarshal TextContent: %w", err)
			}
			actualContent = tc
		case "image":
			var ic ImageContent // Defined in messages.go
			if err := json.Unmarshal(raw, &ic); err != nil {
				return fmt.Errorf("failed to unmarshal ImageContent: %w", err)
			}
			actualContent = ic
		case "audio":
			var ac AudioContent // Defined in messages.go
			if err := json.Unmarshal(raw, &ac); err != nil {
				return fmt.Errorf("failed to unmarshal AudioContent: %w", err)
			}
			actualContent = ac
		case "resource":
			var erc EmbeddedResourceContent // Defined in messages.go
			if err := json.Unmarshal(raw, &erc); err != nil {
				return fmt.Errorf("failed to unmarshal EmbeddedResourceContent: %w", err)
			}
			actualContent = erc
		default:
			// Handle unknown content types if necessary, maybe return an error or skip
			log.Printf("Warning: Unknown content type '%s' encountered during unmarshalling", typeDetect.Type)
			// Or return fmt.Errorf("unknown content type '%s'", typeDetect.Type)
			continue // Skip unknown types for now
		}
		r.Content = append(r.Content, actualContent)
	}

	return nil
}
