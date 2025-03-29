// Package mcp provides the core implementation for the Model Context Protocol (MCP)
// in Go. It defines message structures, transport mechanisms (currently stdio),
// and basic client/server logic for establishing connections via the MCP handshake.
package mcp

// --- Core Message Structures ---

// Message represents the base structure for all MCP messages.
// Specific message types typically embed this struct and define their own
// payload structure. The Payload field here is often handled as
// json.RawMessage during transport to allow for type-specific unmarshalling
// after the MessageType is identified.
type Message struct {
	ProtocolVersion string      `json:"protocol_version"` // MCP Protocol Version (e.g., "2025-03-26")
	MessageID       string      `json:"message_id"`       // Unique message identifier (UUID)
	MessageType     string      `json:"message_type"`     // Type of MCP message (e.g., "HandshakeRequest")
	Payload         interface{} `json:"payload"`          // Message-specific data (often json.RawMessage)
}

// ErrorPayload defines the structure for the 'error' object within a JSONRPCError response,
// aligning with the JSON-RPC 2.0 specification used by MCP.
type ErrorPayload struct {
	Code    int         `json:"code"`           // Numeric error code (JSON-RPC standard or implementation-defined)
	Message string      `json:"message"`        // Short error description
	Data    interface{} `json:"data,omitempty"` // Optional additional error details
}

// ErrorMessage represents an MCP Error message, which follows the JSONRPCError structure.
// Note: While MCP conceptually sends an "Error" message type, the underlying JSON-RPC
// format uses a top-level "error" field instead of "payload". This struct helps bridge
// the gap for the current SendMessage implementation but might need adjustment for
// strict JSON-RPC transport layers.
type ErrorMessage struct {
	// We might need to adjust how this is sent/received later to strictly match JSONRPCError format.
	// For now, embedding helps fit the existing SendMessage structure.
	// Ideally, SendMessage would detect ErrorPayload and construct the correct JSONRPCError.
	Message              // Embeds ProtocolVersion, MessageID
	Payload ErrorPayload `json:"error"` // Field name MUST be "error" for JSON-RPC compliance
}

// --- Handshake Messages (To be replaced by Initialize) ---
// Keeping these temporarily for compatibility during refactor, will remove later.

type HandshakeRequestPayload struct {
	SupportedProtocolVersions []string `json:"supported_protocol_versions"`
	ServerName                string   `json:"server_name,omitempty"`
	ClientName                string   `json:"client_name,omitempty"`
}
type HandshakeRequest struct {
	Message
	Payload HandshakeRequestPayload `json:"payload"`
}
type HandshakeResponsePayload struct {
	SelectedProtocolVersion string `json:"selected_protocol_version"`
	ServerName              string `json:"server_name,omitempty"`
	ClientName              string `json:"client_name,omitempty"`
}
type HandshakeResponse struct {
	Message
	Payload HandshakeResponsePayload `json:"payload"`
}

// --- Tool Definition Messages ---

type ToolInputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]PropertyDetail `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}
type PropertyDetail struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	// TODO: Add other JSON schema fields
}
type ToolOutputSchema struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	// TODO: Add other JSON schema fields
}
type ToolDefinition struct {
	Name         string           `json:"name"`
	Description  string           `json:"description,omitempty"`
	InputSchema  ToolInputSchema  `json:"input_schema"`
	OutputSchema ToolOutputSchema `json:"output_schema"`
	// TODO: Add Annotations field later based on 'Tool' schema
}
type ToolDefinitionRequestPayload struct{}
type ToolDefinitionRequest struct { // To be renamed ListToolsRequest
	Message
	Payload ToolDefinitionRequestPayload `json:"payload"`
}
type ToolDefinitionResponsePayload struct { // To be renamed ListToolsResult
	Tools []ToolDefinition `json:"tools"`
}
type ToolDefinitionResponse struct { // To be renamed ListToolsResponse (JSONRPCResponse wrapper)
	Message
	Payload ToolDefinitionResponsePayload `json:"payload"`
}

// --- Tool Usage Messages ---

type UseToolRequestPayload struct { // To be renamed CallToolParams
	ToolName  string                 `json:"tool_name"` // To be renamed 'name'
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}
type UseToolRequest struct { // To be renamed CallToolRequest
	Message
	Payload UseToolRequestPayload `json:"payload"`
}
type UseToolResponsePayload struct { // To be renamed CallToolResult
	Result interface{} `json:"result"` // To be changed to 'content' array + 'isError' bool
}
type UseToolResponse struct { // To be renamed CallToolResponse (JSONRPCResponse wrapper)
	Message
	Payload UseToolResponsePayload `json:"payload"`
}

// --- Constants ---

const (
	// CurrentProtocolVersion defines the MCP version this library implementation supports.
	CurrentProtocolVersion = "2025-03-26" // Updated version

	// --- Message Type Constants ---
	// NOTE: These will change according to the spec (e.g., "initialize", "tools/list", "tools/call")

	// MessageTypeError identifies an Error message (conceptually).
	MessageTypeError = "Error" // This might become irrelevant if errors are handled purely via JSONRPCError structure
	// MessageTypeHandshakeRequest identifies a HandshakeRequest message (to be replaced by "initialize").
	MessageTypeHandshakeRequest = "HandshakeRequest"
	// MessageTypeHandshakeResponse identifies a HandshakeResponse message (to be replaced by JSONRPCResponse for "initialize").
	MessageTypeHandshakeResponse = "HandshakeResponse"
	// MessageTypeToolDefinitionRequest identifies a ToolDefinitionRequest message (to be replaced by "tools/list").
	MessageTypeToolDefinitionRequest = "ToolDefinitionRequest"
	// MessageTypeToolDefinitionResponse identifies a ToolDefinitionResponse message (to be replaced by JSONRPCResponse for "tools/list").
	MessageTypeToolDefinitionResponse = "ToolDefinitionResponse"
	// MessageTypeUseToolRequest identifies a UseToolRequest message (to be replaced by "tools/call").
	MessageTypeUseToolRequest = "UseToolRequest"
	// MessageTypeUseToolResponse identifies a UseToolResponse message (to be replaced by JSONRPCResponse for "tools/call").
	MessageTypeUseToolResponse = "UseToolResponse"
	// TODO: Add other message type constants (ResourceAccess, Notification, etc.)

	// --- Standard JSON-RPC Error Codes ---
	ErrorCodeParseError     = -32700
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternalError  = -32603
	// -32000 to -32099 are reserved for implementation-defined server-errors.

	// --- MCP / Implementation-Defined Error Codes (Example Range) ---
	// Using -32000 range for MCP/implementation specific errors
	ErrorCodeMCPHandshakeFailed            = -32000 // Custom code for handshake phase errors
	ErrorCodeMCPUnsupportedProtocolVersion = -32001 // Custom code for version mismatch
	ErrorCodeMCPInvalidMessage             = -32002 // Custom code for structurally invalid MCP message (before JSON check)
	ErrorCodeMCPInvalidPayload             = -32003 // Custom code for invalid MCP payload structure
	ErrorCodeMCPNotImplemented             = -32004 // Custom code for unimplemented MCP features/methods
	ErrorCodeMCPToolNotFound               = -32010 // Custom code for tool not found
	ErrorCodeMCPInvalidArgument            = -32011 // Custom code for invalid tool arguments
	ErrorCodeMCPToolExecutionError         = -32012 // Custom code for error during tool run
	ErrorCodeMCPAuthenticationFailed       = -32020 // Custom code for auth failure
	ErrorCodeMCPRateLimitExceeded          = -32021 // Custom code for rate limit exceeded
	ErrorCodeMCPSecurityViolation          = -32030 // Custom code for security issues (e.g., sandbox escape)
	ErrorCodeMCPOperationFailed            = -32031 // Custom code for general operation failure (e.g., file IO)
	ErrorCodeMCPResourceNotFound           = -32040 // Placeholder
	ErrorCodeMCPAccessDenied               = -32041 // Placeholder
)
