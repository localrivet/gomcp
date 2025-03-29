// Package mcp provides the core implementation for the Model Context Protocol (MCP)
// in Go. It defines message structures, transport mechanisms (currently stdio),
// and basic client/server logic for establishing connections via the MCP handshake.
package mcp

import "fmt"

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

// MCPError wraps ErrorPayload to implement the error interface.
// Handlers can return this type to provide specific JSON-RPC error details.
type MCPError struct {
	ErrorPayload
}

// Error implements the error interface for MCPError.
func (e *MCPError) Error() string {
	return fmt.Sprintf("MCP Error: Code=%d, Message=%s", e.Code, e.Message)
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

// --- JSON-RPC 2.0 Base Structures ---

// JSONRPCRequest represents a standard JSON-RPC request object.
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`          // MUST be "2.0"
	ID      interface{} `json:"id"`               // Request ID (string, number, or null)
	Method  string      `json:"method"`           // Method name (e.g., "initialize", "tools/call")
	Params  interface{} `json:"params,omitempty"` // Parameters (struct or array)
}

// JSONRPCResponse represents a standard JSON-RPC response object.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`          // MUST be "2.0"
	ID      interface{}   `json:"id"`               // MUST be the same as the request ID (or null if error before ID parsing)
	Result  interface{}   `json:"result,omitempty"` // Result object (on success)
	Error   *ErrorPayload `json:"error,omitempty"`  // Error object (on failure)
}

// JSONRPCNotification represents a standard JSON-RPC notification object.
type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`          // MUST be "2.0"
	Method  string      `json:"method"`           // Method name (e.g., "initialized", "notifications/...")
	Params  interface{} `json:"params,omitempty"` // Parameters (struct or array)
	// Note: Notifications MUST NOT have an 'id' field.
}

// --- Initialization Sequence Structures ---

// Implementation describes the name and version of an MCP implementation (client or server).
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities describes features the client supports.
// NOTE: This is a basic structure; real implementations might add more specific fields
// based on the capabilities they actually support (e.g., roots, sampling).
type ClientCapabilities struct {
	// Experimental capabilities can be added here.
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	// Add other known capability fields as needed, e.g.:
	Roots *struct { // Add Roots capability field
		ListChanged bool `json:"listChanged,omitempty"` // Client supports notifications/roots/list_changed
	} `json:"roots,omitempty"`
	Sampling *struct{} `json:"sampling,omitempty"` // Add Sampling capability field
}

// ServerCapabilities describes features the server supports.
// NOTE: This is a basic structure; real implementations might add more specific fields.
type ServerCapabilities struct {
	// Experimental capabilities can be added here.
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	// Add other known capability fields as needed, e.g.:
	Logging *struct{} `json:"logging,omitempty"` // Add Logging capability field
	// Completions *struct{} `json:"completions,omitempty"`
	Prompts *struct { // Add Prompts capability field
		ListChanged bool `json:"listChanged,omitempty"` // Server supports notifications/prompts/list_changed
	} `json:"prompts,omitempty"`
	Resources *struct { // Add Resources capability field
		Subscribe   bool `json:"subscribe,omitempty"`   // Server supports resources/subscribe
		ListChanged bool `json:"listChanged,omitempty"` // Server supports notifications/resources/list_changed
	} `json:"resources,omitempty"`
	Tools *struct {
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"tools,omitempty"` // Add Tools capability field
}

// InitializeRequestParams defines the parameters for the 'initialize' request.
type InitializeRequestParams struct {
	ProtocolVersion string             `json:"protocolVersion"` // Note camelCase from schema
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
	// TODO: Add other optional fields like workspaceFolders, trace, etc. if needed
}

// InitializeRequest is sent by the client to start the connection.
// Replaces the old HandshakeRequest.
type InitializeRequest struct {
	Message                         // Embeds ProtocolVersion, MessageID, MessageType="initialize"
	Payload InitializeRequestParams `json:"params"` // JSON-RPC uses "params"
}

// InitializeResult defines the result payload for a successful 'initialize' response.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
}

// InitializeResponse represents the successful server response to an InitializeRequest.
// This is conceptually similar to the old HandshakeResponse but aligns with JSONRPCResponse structure.
// Note: For strict JSON-RPC, this shouldn't embed Message, but have top-level id, jsonrpc, result.
// We'll keep embedding for now to fit the current transport, but use the correct payload structure.
type InitializeResponse struct {
	Message                  // Embeds ProtocolVersion, MessageID, MessageType="initializeResponse" (conceptual)
	Payload InitializeResult `json:"result"` // JSON-RPC uses "result"
}

// InitializedNotificationParams is the payload for the 'initialized' notification (empty).
type InitializedNotificationParams struct{}

// InitializedNotification is sent by the client after receiving InitializeResult.
type InitializedNotification struct {
	Message                               // Embeds ProtocolVersion, MessageID, MessageType="initialized"
	Payload InitializedNotificationParams `json:"params"` // JSON-RPC uses "params"
}

// --- Tooling Structures and Messages ---

// ToolInputSchema defines the expected input structure for a tool (JSON Schema subset).
type ToolInputSchema struct {
	Type       string                    `json:"type"` // Typically "object"
	Properties map[string]PropertyDetail `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}

// PropertyDetail describes a single parameter within a ToolInputSchema.
type PropertyDetail struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	// TODO: Add other JSON schema fields (enum, format, etc.)
}

// ToolAnnotations provides optional hints about tool behavior.
type ToolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty"`    // Use pointer for optional boolean
	DestructiveHint *bool  `json:"destructiveHint,omitempty"` // Use pointer for optional boolean
	IdempotentHint  *bool  `json:"idempotentHint,omitempty"`  // Use pointer for optional boolean
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`   // Use pointer for optional boolean
}

// Tool defines a tool offered by the server (replaces ToolDefinition).
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema ToolInputSchema `json:"inputSchema"` // Note camelCase
	Annotations ToolAnnotations `json:"annotations,omitempty"`
	// Note: OutputSchema is removed from Tool definition in 2025-03-26 spec,
	// the output structure is defined by CallToolResult.
}

// ListToolsRequestParams defines the parameters for a 'tools/list' request (includes pagination).
type ListToolsRequestParams struct {
	Cursor string `json:"cursor,omitempty"` // Opaque pagination cursor
}

// ListToolsRequest asks the server for its available tools.
type ListToolsRequest struct {
	Message                        // Embeds ProtocolVersion, MessageID, MessageType="tools/list"
	Payload ListToolsRequestParams `json:"params"` // JSON-RPC uses "params"
}

// ListToolsResult defines the result payload for a successful 'tools/list' response.
type ListToolsResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"` // Opaque pagination cursor
}

// ListToolsResponse represents the successful server response to a ListToolsRequest.
type ListToolsResponse struct {
	Message                 // Embeds ProtocolVersion, MessageID, MessageType="tools/listResponse" (conceptual)
	Payload ListToolsResult `json:"result"` // JSON-RPC uses "result"
}

// CallToolParams defines the parameters for a 'tools/call' request.
type CallToolParams struct {
	Name      string                 `json:"name"` // Renamed from tool_name
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	// TODO: Add _meta field if progressToken support is added
}

// CallToolRequest asks the server to execute a specific tool.
type CallToolRequest struct {
	Message                // Embeds ProtocolVersion, MessageID, MessageType="tools/call"
	Payload CallToolParams `json:"params"` // JSON-RPC uses "params"
}

// Content defines the interface for different types of content in results/prompts.
// Using an interface requires type assertions or switches when processing.
// Alternatively, use a struct with one field per type and 'omitempty'.
type Content interface {
	GetType() string
}

// TextContent represents textual content.
type TextContent struct {
	Type string `json:"type"` // Should always be "text"
	Text string `json:"text"`
	// TODO: Add Annotations
}

func (tc TextContent) GetType() string { return tc.Type }

// TODO: Define ImageContent, AudioContent, EmbeddedResource structs as needed.

// CallToolResult defines the result payload for a successful 'tools/call' response.
type CallToolResult struct {
	Content []Content `json:"content"`           // Array of content parts (e.g., TextContent)
	IsError *bool     `json:"isError,omitempty"` // Pointer to boolean for optional field
	// TODO: Add _meta field
}

// CallToolResponse represents the successful server response to a CallToolRequest.
type CallToolResponse struct {
	Message                // Embeds ProtocolVersion, MessageID, MessageType="tools/callResponse" (conceptual)
	Payload CallToolResult `json:"result"` // JSON-RPC uses "result"
}

// --- Resource Access Structures ---

// Resource represents a piece of context available from the server.
type Resource struct {
	URI         string                 `json:"uri"`                   // Unique identifier (e.g., "file:///path/to/file", "git://...?rev=...")
	Kind        string                 `json:"kind,omitempty"`        // e.g., "file", "git_commit", "api_spec"
	Title       string                 `json:"title,omitempty"`       // Human-readable title
	Description string                 `json:"description,omitempty"` // Longer description
	Version     string                 `json:"version,omitempty"`     // Opaque version string (changes when content changes)
	Metadata    map[string]interface{} `json:"metadata,omitempty"`    // Additional arbitrary metadata
}

// ResourceContents defines the interface for different types of resource content.
type ResourceContents interface {
	GetContentType() string
}

// TextResourceContents holds text-based resource content.
type TextResourceContents struct {
	ContentType string `json:"contentType"` // e.g., "text/plain", "application/json"
	Content     string `json:"content"`
}

func (trc TextResourceContents) GetContentType() string { return trc.ContentType }

// BlobResourceContents holds binary resource content (base64 encoded).
type BlobResourceContents struct {
	ContentType string `json:"contentType"` // e.g., "image/png", "application/octet-stream"
	Content     string `json:"content"`     // Base64 encoded string
}

func (brc BlobResourceContents) GetContentType() string { return brc.ContentType }

// ListResourcesRequestParams defines parameters for 'resources/list'.
type ListResourcesRequestParams struct {
	// TODO: Add filtering options (kind, query, etc.)
	Cursor string `json:"cursor,omitempty"`
}

// ListResourcesResult defines the result for 'resources/list'.
type ListResourcesResult struct {
	Resources  []Resource `json:"resources"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

// ReadResourceRequestParams defines parameters for 'resources/read'.
type ReadResourceRequestParams struct {
	URI     string `json:"uri"`               // URI of the resource to read
	Version string `json:"version,omitempty"` // Optional specific version to read
}

// ReadResourceResult defines the result for 'resources/read'.
// Uses ResourceContents interface for polymorphism.
type ReadResourceResult struct {
	Resource Resource         `json:"resource"` // Metadata of the read resource
	Contents ResourceContents `json:"contents"` // Actual content (Text or Blob)
}

// --- Prompt Structures ---

// PromptArgument defines an input parameter for a prompt template.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"` // e.g., "string", "number", "boolean"
	Required    bool   `json:"required,omitempty"`
}

// PromptMessage represents a single message within a prompt sequence.
type PromptMessage struct {
	Role    string    `json:"role"`    // e.g., "system", "user", "assistant"
	Content []Content `json:"content"` // Array of content parts
}

// Prompt represents a prompt template available from the server.
type Prompt struct {
	URI         string                 `json:"uri"`                   // Unique identifier (e.g., "mcp://prompts/summarize")
	Title       string                 `json:"title,omitempty"`       // Human-readable title
	Description string                 `json:"description,omitempty"` // Longer description
	Arguments   []PromptArgument       `json:"arguments,omitempty"`   // Input arguments for the template
	Messages    []PromptMessage        `json:"messages"`              // The sequence of messages forming the prompt
	Metadata    map[string]interface{} `json:"metadata,omitempty"`    // Additional arbitrary metadata
}

// PromptReference allows referencing a prompt, potentially with arguments.
type PromptReference struct {
	URI       string                 `json:"uri"`                 // URI of the prompt template
	Arguments map[string]interface{} `json:"arguments,omitempty"` // Arguments to fill into the template
}

// ListPromptsRequestParams defines parameters for 'prompts/list'.
type ListPromptsRequestParams struct {
	// TODO: Add filtering options
	Cursor string `json:"cursor,omitempty"`
}

// ListPromptsResult defines the result for 'prompts/list'.
type ListPromptsResult struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

// GetPromptRequestParams defines parameters for 'prompts/get'.
type GetPromptRequestParams struct {
	URI string `json:"uri"` // URI of the prompt to get
}

// GetPromptResult defines the result for 'prompts/get'.
type GetPromptResult struct {
	Prompt Prompt `json:"prompt"`
}

// --- Logging Structures ---

// LoggingLevel defines the possible logging levels.
type LoggingLevel string

const (
	LogLevelError LoggingLevel = "error"
	LogLevelWarn  LoggingLevel = "warn"
	LogLevelInfo  LoggingLevel = "info"
	LogLevelDebug LoggingLevel = "debug"
	LogLevelTrace LoggingLevel = "trace" // Added based on common practice
)

// SetLevelRequestParams defines parameters for 'logging/set_level'.
type SetLevelRequestParams struct {
	Level LoggingLevel `json:"level"`
}

// LoggingMessageParams defines parameters for 'notifications/message'.
type LoggingMessageParams struct {
	Level   LoggingLevel `json:"level"`
	Message string       `json:"message"`
}

// --- Sampling Structures ---

// SamplingMessage represents a message in the context provided for sampling.
type SamplingMessage struct {
	Role    string    `json:"role"`    // e.g., "system", "user", "assistant"
	Content []Content `json:"content"` // Array of content parts
	// TODO: Add optional 'name' field if needed for tool results
}

// ModelPreferences specifies desired model characteristics.
type ModelPreferences struct {
	ModelURI string `json:"modelUri,omitempty"` // Preferred model URI
	// TODO: Add other preference fields (temperature, top_k, etc.)
}

// ModelHint provides information about the model used for a response.
type ModelHint struct {
	ModelURI string `json:"modelUri"` // URI of the model used
	// TODO: Add other hint fields (token counts, finish reason, etc.)
}

// CreateMessageRequestParams defines parameters for 'sampling/create_message'.
type CreateMessageRequestParams struct {
	Context     []SamplingMessage `json:"context"`               // The message history
	Preferences *ModelPreferences `json:"preferences,omitempty"` // Optional model preferences
}

// CreateMessageResult defines the result for 'sampling/create_message'.
type CreateMessageResult struct {
	Message   SamplingMessage `json:"message"`             // The generated message
	ModelHint *ModelHint      `json:"modelHint,omitempty"` // Optional info about the model used
}

// --- Roots Structures ---

// Root represents a root context or workspace available on the client.
type Root struct {
	URI         string                 `json:"uri"`             // Unique identifier (e.g., "file:///path/to/workspace")
	Kind        string                 `json:"kind,omitempty"`  // e.g., "workspace", "project"
	Title       string                 `json:"title,omitempty"` // Human-readable title
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ListRootsRequestParams defines parameters for 'roots/list'. (Currently empty)
type ListRootsRequestParams struct{}

// ListRootsResult defines the result for 'roots/list'.
type ListRootsResult struct {
	Roots []Root `json:"roots"`
}

// --- Constants ---

const (
	// CurrentProtocolVersion defines the MCP version this library implementation supports.
	CurrentProtocolVersion = "2025-03-26" // Updated version

	// --- Message Type (Method Name) Constants ---
	// These now align with the JSON-RPC 'method' field names from the spec.

	// Initialization
	MethodInitialize  = "initialize"
	MethodInitialized = "initialized" // Notification

	// Tools
	MethodListTools = "tools/list"
	MethodCallTool  = "tools/call"
	// TODO: Add MethodToolListChanged = "notifications/tools/list_changed"

	// Resources
	MethodListResources = "resources/list"
	MethodReadResource  = "resources/read"
	// TODO: Add resource notification methods

	// Prompts
	MethodListPrompts = "prompts/list"
	MethodGetPrompt   = "prompts/get"
	// TODO: Add prompt notification methods

	// Logging
	MethodLoggingSetLevel     = "logging/set_level"
	MethodNotificationMessage = "notifications/message" // Note: This is a notification

	// Sampling
	MethodSamplingCreateMessage = "sampling/create_message"

	// Roots
	MethodRootsList = "roots/list"
	// TODO: Add roots notification methods

	// Old Handshake types (REMOVED)
	// MessageTypeHandshakeRequest  = "HandshakeRequest"
	// MessageTypeHandshakeResponse = "HandshakeResponse"
	// Old Tool types (REMOVED)
	// MessageTypeToolDefinitionRequest  = "ToolDefinitionRequest"
	// MessageTypeToolDefinitionResponse = "ToolDefinitionResponse"
	// MessageTypeUseToolRequest         = "UseToolRequest"
	// MessageTypeUseToolResponse        = "UseToolResponse"

	// MessageTypeError identifies an Error message (conceptually).
	MessageTypeError = "Error" // This might become irrelevant

	// --- Standard JSON-RPC Error Codes ---
	ErrorCodeParseError     = -32700
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternalError  = -32603
	// -32000 to -32099 are reserved for implementation-defined server-errors.

	// --- MCP / Implementation-Defined Error Codes (Example Range) ---
	// Using -32000 range for MCP/implementation specific errors
	ErrorCodeMCPHandshakeFailed            = -32000 // Custom code for handshake phase errors (will become Initialize errors)
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
