package server

import "encoding/json"

// createErrorResponse creates a JSON-RPC 2.0 error response.
// This function formats error information according to the JSON-RPC 2.0 specification,
// with the appropriate error code, message, and optional additional data.
//
// Parameters:
//   - id: The request ID to include in the response (can be string, number, or null)
//   - code: The JSON-RPC error code (e.g., -32600 for invalid request)
//   - message: A human-readable error message
//   - data: Optional additional data to include in the error object
//
// Returns:
//   - Serialized JSON bytes containing the error response
func createErrorResponse(id interface{}, code int, message string, data interface{}) []byte {
	response := struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      interface{} `json:"id"`
		Error   struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		} `json:"error"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Error: struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		}{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	responseBytes, _ := json.Marshal(response)
	return responseBytes
}

// Error returns the error message, implementing the error interface.
// This method allows RPCError to be used as a standard Go error.
//
// Returns:
//   - The error message as a string
func (e *RPCError) Error() string {
	return e.Message
}
