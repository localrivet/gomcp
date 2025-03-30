// Package protocol defines the structures and constants for the Model Context Protocol (MCP).
package protocol

import "fmt"

// MCPError wraps ErrorPayload to implement the error interface.
// Handlers can return this type to provide specific JSON-RPC error details.
type MCPError struct {
	ErrorPayload
}

// Error implements the error interface for MCPError.
func (e *MCPError) Error() string {
	return fmt.Sprintf("MCP Error: Code=%d, Message=%s", e.Code, e.Message)
}

// ErrorMessage represents an MCP Error message conceptually.
// The actual message sent is a JSONRPCResponse with the 'error' field populated.
type ErrorMessage struct {
	Payload ErrorPayload `json:"error"` // Field name MUST be "error" for JSON-RPC compliance
}

const (
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
