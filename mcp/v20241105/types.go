package v20241105

import (
	"encoding/json"
	"fmt"
)

// Constants for JSON-RPC
const (
	JSONRPCVersion = "2.0"
)

// Message is the base interface for all MCP messages
type Message interface {
	IsRequest() bool
	IsResponse() bool
	IsNotification() bool
}

// Request represents a JSON-RPC request message
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsRequest identifies this as a request message
func (r *Request) IsRequest() bool { return true }

// IsResponse identifies this as not a response message
func (r *Request) IsResponse() bool { return false }

// IsNotification identifies this as not a notification message
func (r *Request) IsNotification() bool { return false }

// NewRequest creates a new request with the specified ID, method, and parameters
func NewRequest(id interface{}, method string, params interface{}) (*Request, error) {
	var paramsJSON json.RawMessage
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	return &Request{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  paramsJSON,
	}, nil
}

// ParseParams parses the params field into the specified struct
func (r *Request) ParseParams(v interface{}) error {
	if r.Params == nil {
		return nil
	}
	return json.Unmarshal(r.Params, v)
}

// Response represents a JSON-RPC response message
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// IsRequest identifies this as not a request message
func (r *Response) IsRequest() bool { return false }

// IsResponse identifies this as a response message
func (r *Response) IsResponse() bool { return true }

// IsNotification identifies this as not a notification message
func (r *Response) IsNotification() bool { return false }

// NewResponse creates a new successful response with the specified ID and result
func NewResponse(id interface{}, result interface{}) (*Response, error) {
	var resultJSON json.RawMessage
	if result != nil {
		var err error
		resultJSON, err = json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", err)
		}
	}

	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  resultJSON,
	}, nil
}

// NewErrorResponse creates a new error response with the specified ID and error
func NewErrorResponse(id interface{}, err *Error) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error:   err,
	}
}

// ParseResult parses the result field into the specified struct
func (r *Response) ParseResult(v interface{}) error {
	if r.Result == nil {
		return fmt.Errorf("no result to parse")
	}
	return json.Unmarshal(r.Result, v)
}

// Notification represents a JSON-RPC notification message
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsRequest identifies this as not a request message
func (n *Notification) IsRequest() bool { return false }

// IsResponse identifies this as not a response message
func (n *Notification) IsResponse() bool { return false }

// IsNotification identifies this as a notification message
func (n *Notification) IsNotification() bool { return true }

// NewNotification creates a new notification with the specified method and parameters
func NewNotification(method string, params interface{}) (*Notification, error) {
	var paramsJSON json.RawMessage
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	return &Notification{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  paramsJSON,
	}, nil
}

// ParseParams parses the params field into the specified struct
func (n *Notification) ParseParams(v interface{}) error {
	if n.Params == nil {
		return nil
	}
	return json.Unmarshal(n.Params, v)
}

// Error represents a JSON-RPC error object
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Standard error codes as defined in the JSON-RPC 2.0 specification
const (
	ParseErrorCode     = -32700
	InvalidRequestCode = -32600
	MethodNotFoundCode = -32601
	InvalidParamsCode  = -32602
	InternalErrorCode  = -32603
	// -32000 to -32099 are reserved for implementation-defined server errors
)

// NewError creates a new Error with the specified code and message
func NewError(code int, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// NewErrorWithData creates a new Error with the specified code, message, and data
func NewErrorWithData(code int, message string, data interface{}) (*Error, error) {
	var dataJSON json.RawMessage
	if data != nil {
		var err error
		dataJSON, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal error data: %w", err)
		}
	}

	return &Error{
		Code:    code,
		Message: message,
		Data:    dataJSON,
	}, nil
}

// ParseData parses the data field into the specified struct
func (e *Error) ParseData(v interface{}) error {
	if e.Data == nil {
		return nil
	}
	return json.Unmarshal(e.Data, v)
}

// Error implements the error interface
func (e *Error) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}
