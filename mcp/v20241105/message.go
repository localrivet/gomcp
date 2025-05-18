package v20241105

import (
	"encoding/json"
	"fmt"
)

// MessageType defines the type of MCP message
type MessageType string

// MCP message types
const (
	MessageTypeRequest  MessageType = "request"
	MessageTypeResponse MessageType = "response"
	MessageTypeError    MessageType = "error"
)

// BaseMessage represents a base MCP protocol message
type BaseMessage struct {
	Type     MessageType     `json:"type"`
	ID       string          `json:"id,omitempty"`
	Content  json.RawMessage `json:"content"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// RequestType defines the type of request
type RequestType string

// Request types
const (
	RequestTypeToolCall      RequestType = "tool_call"
	RequestTypeResourceFetch RequestType = "resource_fetch"
	RequestTypePromptRender  RequestType = "prompt_render"
	RequestTypeList          RequestType = "list"
)

// RequestMessage represents an MCP request message
type RequestMessage struct {
	Type     RequestType     `json:"type"`
	ID       string          `json:"id,omitempty"`
	Content  json.RawMessage `json:"content"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// ToolCallRequest represents a tool call request
type ToolCallRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ResourceFetchRequest represents a resource fetch request
type ResourceFetchRequest struct {
	Path string `json:"path"`
}

// PromptRenderRequest represents a prompt render request
type PromptRenderRequest struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// ListRequest represents a list request
type ListRequest struct {
	Type string `json:"type"`
}

// ResponseType defines the type of response
type ResponseType string

// Response types
const (
	ResponseTypeToolResult      ResponseType = "tool_result"
	ResponseTypeResourceContent ResponseType = "resource_content"
	ResponseTypePromptContent   ResponseType = "prompt_content"
	ResponseTypeList            ResponseType = "list"
)

// ResponseMessage represents an MCP response message
type ResponseMessage struct {
	Type     ResponseType    `json:"type"`
	ID       string          `json:"id,omitempty"`
	Content  json.RawMessage `json:"content"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// ToolResultResponse represents a tool result response
type ToolResultResponse struct {
	Result interface{} `json:"result"`
}

// ResourceContentResponse represents a resource content response
type ResourceContentResponse struct {
	Content interface{} `json:"content"`
}

// PromptContentResponse represents a prompt content response
type PromptContentResponse struct {
	Content []Content `json:"content"`
}

// ListResponse represents a list response
type ListResponse struct {
	Items []ItemDefinition `json:"items"`
}

// ItemDefinition represents an item in a list response
type ItemDefinition struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ErrorCode defines the type of error
type ErrorCode string

// Error codes
const (
	ErrorCodeInvalidRequest      ErrorCode = "invalid_request"
	ErrorCodeInvalidArguments    ErrorCode = "invalid_arguments"
	ErrorCodeToolNotFound        ErrorCode = "tool_not_found"
	ErrorCodeResourceNotFound    ErrorCode = "resource_not_found"
	ErrorCodePromptNotFound      ErrorCode = "prompt_not_found"
	ErrorCodeInternalServerError ErrorCode = "internal_server_error"
)

// ErrorResponse represents an MCP error message
type ErrorResponse struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// Content represents a prompt content block
type Content interface {
	GetType() string
}

// TextContent represents a text content block
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// GetType returns the content type
func (tc TextContent) GetType() string {
	return tc.Type
}

// UnmarshalRequest unmarshals a BaseMessage into a specific request type
func UnmarshalRequest(msg BaseMessage) (interface{}, error) {
	var req RequestMessage
	if err := json.Unmarshal(msg.Content, &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	switch req.Type {
	case RequestTypeToolCall:
		var toolReq ToolCallRequest
		if err := json.Unmarshal(req.Content, &toolReq); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool call request: %w", err)
		}
		return toolReq, nil
	case RequestTypeResourceFetch:
		var resourceReq ResourceFetchRequest
		if err := json.Unmarshal(req.Content, &resourceReq); err != nil {
			return nil, fmt.Errorf("failed to unmarshal resource fetch request: %w", err)
		}
		return resourceReq, nil
	case RequestTypePromptRender:
		var promptReq PromptRenderRequest
		if err := json.Unmarshal(req.Content, &promptReq); err != nil {
			return nil, fmt.Errorf("failed to unmarshal prompt render request: %w", err)
		}
		return promptReq, nil
	case RequestTypeList:
		var listReq ListRequest
		if err := json.Unmarshal(req.Content, &listReq); err != nil {
			return nil, fmt.Errorf("failed to unmarshal list request: %w", err)
		}
		return listReq, nil
	default:
		return nil, fmt.Errorf("unknown request type: %s", req.Type)
	}
}

// UnmarshalResponse unmarshals a BaseMessage into a specific response type
func UnmarshalResponse(msg BaseMessage) (interface{}, error) {
	var resp ResponseMessage
	if err := json.Unmarshal(msg.Content, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	switch resp.Type {
	case ResponseTypeToolResult:
		var toolResp ToolResultResponse
		if err := json.Unmarshal(resp.Content, &toolResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool result response: %w", err)
		}
		return toolResp, nil
	case ResponseTypeResourceContent:
		var resourceResp ResourceContentResponse
		if err := json.Unmarshal(resp.Content, &resourceResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal resource content response: %w", err)
		}
		return resourceResp, nil
	case ResponseTypePromptContent:
		var promptResp PromptContentResponse
		if err := json.Unmarshal(resp.Content, &promptResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal prompt content response: %w", err)
		}
		return promptResp, nil
	case ResponseTypeList:
		var listResp ListResponse
		if err := json.Unmarshal(resp.Content, &listResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal list response: %w", err)
		}
		return listResp, nil
	default:
		return nil, fmt.Errorf("unknown response type: %s", resp.Type)
	}
}
