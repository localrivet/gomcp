// Package protocol defines the structures and constants for the Model Context Protocol (MCP).
package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// --- Tooling Structures and Messages (Schema 2025-03-26) ---

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

// ToolCall defines the structure for a tool call request (mirrors types.ToolCall).
type ToolCall struct {
	ID       string          `json:"id"`
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"input,omitempty"`
}

// CallToolRequestParams defines the parameters for a 'tools/call' request.
// Supports both schema versions:
// - 2024-11-05: { "name": "...", "arguments": { ... } }
// - 2025-03-26: { "tool_call": { "id": "...", "tool_name": "...", "input": { ... } } }
type CallToolRequestParams struct {
	// V2025 format
	ToolCall *ToolCall    `json:"tool_call,omitempty"`
	Meta     *RequestMeta `json:"_meta,omitempty"`

	// V2024 format (for backward compatibility)
	Name      string          `json:"name,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling to handle both V2024 and V2025 formats
func (p *CallToolRequestParams) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a standard struct first
	type Alias CallToolRequestParams
	var standard Alias
	if err := json.Unmarshal(data, &standard); err != nil {
		return err
	}

	// Copy the standard fields
	*p = CallToolRequestParams(standard)

	// Handle conversion between formats based on which fields were set
	if p.ToolCall == nil && p.Name != "" {
		// Convert from V2024 format to V2025 format
		// Generate a random ID if none exists
		id := "auto-" + fmt.Sprintf("%d", time.Now().UnixNano())
		p.ToolCall = &ToolCall{
			ID:       id,
			ToolName: p.Name,
			Input:    p.Arguments,
		}
	} else if p.ToolCall != nil && p.Name == "" {
		// For completeness, we could fill the V2024 fields from V2025 format
		// But this isn't strictly necessary since we'll use ToolCall going forward
		p.Name = p.ToolCall.ToolName
		p.Arguments = p.ToolCall.Input
	}

	return nil
}

// ToolError defines the structure for reporting errors during tool execution (Schema 2025-03-26).
type ToolError struct {
	Code    ErrorCode   `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// CallToolResult defines the result payload for a 'tools/call' response (Schema 2025-03-26).
type CallToolResult struct {
	ToolCallID string          `json:"tool_call_id"`
	Output     json.RawMessage `json:"output,omitempty"`
	Error      *ToolError      `json:"error,omitempty"`
	Meta       *RequestMeta    `json:"_meta,omitempty"`
}
