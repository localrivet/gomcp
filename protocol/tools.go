// Package protocol defines the structures and constants for the Model Context Protocol (MCP).
package protocol

import (
	"encoding/json" // Added for UnmarshalJSON
	// Added for UnmarshalJSON
	// Added for UnmarshalJSON
	// "github.com/localrivet/gomcp/types" // REMOVED to break import cycle
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

// CallToolRequestParams defines the parameters for a 'tools/call' request (Schema 2025-03-26).
type CallToolRequestParams struct {
	ToolCall *ToolCall    `json:"tool_call"` // Use protocol.ToolCall
	Meta     *RequestMeta `json:"_meta,omitempty"`
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
