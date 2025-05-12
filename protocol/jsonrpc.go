// Package protocol defines the structures and constants for the Model Context Protocol (MCP),
// based on the JSON-RPC 2.0 specification.
package protocol

import (
	"encoding/json"
	"fmt"
)

// ErrorPayload defines the structure for the 'error' object within a JSONRPCError response,
// aligning with the JSON-RPC 2.0 specification used by MCP.
type ErrorPayload struct {
	Code    ErrorCode   `json:"code"`           // Use ErrorCode type
	Message string      `json:"message"`        // Short error description
	Data    interface{} `json:"data,omitempty"` // Optional additional error details
}

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

// NewNotification creates a new JSON-RPC notification object.
func NewNotification(method string, params interface{}) *JSONRPCNotification {
	return &JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
}

// UnmarshalPayload is a helper function to unmarshal the payload field from a
// received JSON-RPC params or result field (which is interface{})
// into a specific Go struct pointed to by 'target'.
// It handles the case where the payload might be nil or needs re-marshalling.
func UnmarshalPayload(payload interface{}, target interface{}) error {
	if payload == nil {
		return fmt.Errorf("payload is nil, cannot unmarshal")
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to re-marshal payload (type %T): %w", payload, err)
	}
	if len(payloadBytes) == 0 || string(payloadBytes) == "null" {
		return fmt.Errorf("payload is nil or empty after re-marshalling")
	}
	err = json.Unmarshal(payloadBytes, target)
	if err != nil {
		typeName := fmt.Sprintf("%T", target)
		return fmt.Errorf("failed to unmarshal payload into target type %s: %w", typeName, err)
	}
	return nil
}

// NewSuccessResponse creates a new JSON-RPC success response object.
func NewSuccessResponse(id interface{}, result interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   nil,
	}
}

// NewErrorResponse creates a new JSON-RPC error response object.
func NewErrorResponse(id interface{}, code ErrorCode, message string, data interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id, // Can be null if error occurred before ID parsing
		Result:  nil,
		Error: &ErrorPayload{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}
