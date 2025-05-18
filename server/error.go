package server

import "encoding/json"

// createErrorResponse creates a JSON-RPC 2.0 error response
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

// Error returns the error message, implementing the error interface
func (e *RPCError) Error() string {
	return e.Message
}
