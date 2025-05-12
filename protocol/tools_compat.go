// Package protocol defines compatibility structures for older MCP versions.
package protocol

// CallToolResultV2024 defines the result payload for a 'tools/call' response
// according to the 2024-11-05 schema.
// Note: This schema used 'content' (structured) and 'isError' instead of 'output' (raw) and 'error' (structured).
type CallToolResultV2024 struct {
	ToolCallID string       `json:"tool_call_id"`      // Assuming tool_call_id was implicitly present or intended
	Content    []Content    `json:"content"`           // Result content (SLICE of structured parts)
	IsError    bool         `json:"isError,omitempty"` // Indicates if the tool call failed
	Meta       *RequestMeta `json:"_meta,omitempty"`   // Propagate meta if needed
}

// Note: The original 2024-11-05 schema didn't explicitly include tool_call_id in the result,
// but it's essential for matching responses. We include it here for functional parity.
// The schema also didn't detail how errors should be represented in 'content' when 'isError' is true.
// We will likely populate 'content' with a TextContent part containing the error message.
