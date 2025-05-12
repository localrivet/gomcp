// Package protocol defines the structures and constants for the Model Context Protocol (MCP).
package protocol

// ErrorCode defines the type for JSON-RPC error codes.
type ErrorCode int

// ResourceKind defines the type for resource kinds.
type ResourceKind string

const (
	// --- Protocol Versions ---
	CurrentProtocolVersion = "2025-03-26"
	OldProtocolVersion     = "2024-11-05"

	// --- Standard JSON-RPC Error Codes ---
	CodeParseError     ErrorCode = -32700
	CodeInvalidRequest ErrorCode = -32600
	CodeMethodNotFound ErrorCode = -32601
	CodeInvalidParams  ErrorCode = -32602
	CodeInternalError  ErrorCode = -32603

	// --- MCP Specific Error Codes (Example Range: -32000 to -32099) ---
	CodeMCPAuthenticationFailed    ErrorCode = -32000
	CodeMCPToolNotFound            ErrorCode = -32001 // Added for tool not found
	CodeMCPToolExecutionError      ErrorCode = -32002 // Added for tool execution errors
	CodeMCPResourceNotFound        ErrorCode = -32003
	CodeMCPUnsupportedResourceKind ErrorCode = -32004
	CodeMCPOperationFailed         ErrorCode = -32005
	CodeMCPRateLimitExceeded       ErrorCode = -32006
	CodeMCPServerOverloaded        ErrorCode = -32007
	CodeMCPBillingError            ErrorCode = -32008

	// Resource Kinds
	ResourceKindFile  ResourceKind = "file"
	ResourceKindDir   ResourceKind = "dir"
	ResourceKindBlob  ResourceKind = "blob"
	ResourceKindText  ResourceKind = "text" // Added based on ReadResourceResult content types
	ResourceKindAudio ResourceKind = "audio"

	// --- Request Methods ---
	MethodInitialize             = "initialize"
	MethodShutdown               = "shutdown"
	MethodPing                   = "ping"
	MethodListTools              = "tools/list" // List available tools
	MethodCallTool               = "tools/call"
	MethodListResources          = "resources/list"
	MethodReadResource           = "resources/read"
	MethodSubscribeResource      = "resources/subscribe"
	MethodUnsubscribeResource    = "resources/unsubscribe"
	MethodResourcesListTemplates = "resources/templates/list"
	MethodListPrompts            = "prompts/list"
	MethodGetPrompt              = "prompts/get"
	MethodLoggingSetLevel        = "logging/set_level"
	// V2025 Method (Server -> Client Request)
	MethodSamplingCreateMessage = "sampling/createMessage"

	// --- Notification Methods ---
	MethodInitialized                = "initialized"
	MethodExit                       = "exit"
	MethodCancelled                  = "$/cancelled"
	MethodProgress                   = "$/progress"
	MethodNotificationMessage        = "notifications/message" // Client -> Server log message
	MethodNotifyResourcesListChanged = "notifications/resources/list_changed"
	MethodNotifyResourceUpdated      = "notifications/resources/updated"
	MethodNotifyPromptsListChanged   = "notifications/prompts/list_changed"
	MethodNotifyToolsListChanged     = "notifications/tools/list_changed"

	// --- Server Info ---
)
