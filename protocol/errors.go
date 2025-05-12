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

// Note: Standard JSON-RPC codes (-32700 to -32600) and MCP-specific codes (-32000 to -32099)
// are defined in constants.go to avoid duplication.

// Helper function to create a new MCPError for Invalid Params
func NewInvalidParamsError(message string) *MCPError {
	return &MCPError{
		ErrorPayload: ErrorPayload{
			Code:    CodeInvalidParams,
			Message: message,
		},
	}
}

// Helper function to create a new MCPError for Method Not Found
func NewMethodNotFoundError(methodName string) *MCPError {
	return &MCPError{
		ErrorPayload: ErrorPayload{
			Code:    CodeMethodNotFound,
			Message: fmt.Sprintf("Method not found: %s", methodName),
		},
	}
}
