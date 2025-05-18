package draft

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
	MessageTypeEvent    MessageType = "event" // New in draft version
)

// BaseMessage represents a base MCP protocol message
type BaseMessage struct {
	Type     MessageType     `json:"type"`
	Version  string          `json:"version"`
	ID       string          `json:"id,omitempty"`
	Content  json.RawMessage `json:"content"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
	TraceID  string          `json:"trace_id,omitempty"` // New in draft version for request tracing
}

// RequestType defines the type of request
type RequestType string

// Request types
const (
	RequestTypeToolCall      RequestType = "tool_call"
	RequestTypeResourceFetch RequestType = "resource_fetch"
	RequestTypePromptRender  RequestType = "prompt_render"
	RequestTypeList          RequestType = "list"
	RequestTypeStream        RequestType = "stream"
	RequestTypeSubscribe     RequestType = "subscribe" // New in draft version
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
	Stream    bool            `json:"stream,omitempty"`
	Cache     *CacheOptions   `json:"cache,omitempty"` // New in draft version
}

// CacheOptions defines caching behavior for tool calls
type CacheOptions struct {
	Enabled   bool   `json:"enabled"`
	TTL       int    `json:"ttl,omitempty"`       // Time-to-live in seconds
	CacheKey  string `json:"cache_key,omitempty"` // Custom cache key
	FreshRead bool   `json:"fresh_read,omitempty"`
}

// ResourceFetchRequest represents a resource fetch request
type ResourceFetchRequest struct {
	Path        string         `json:"path"`
	Version     string         `json:"version,omitempty"`     // New in draft version
	Compression string         `json:"compression,omitempty"` // New in draft version
	Range       *ResourceRange `json:"range,omitempty"`       // New in draft version
}

// ResourceRange specifies a range of data to fetch from a resource
type ResourceRange struct {
	Start int64 `json:"start,omitempty"`
	End   int64 `json:"end,omitempty"`
}

// PromptRenderRequest represents a prompt render request
type PromptRenderRequest struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Format     string                 `json:"format,omitempty"` // New in draft version (e.g., "text", "json", "markdown")
}

// ListRequest represents a list request
type ListRequest struct {
	Type    string `json:"type"`
	Pattern string `json:"pattern,omitempty"` // New in draft version - glob pattern
	Filter  string `json:"filter,omitempty"`  // New in draft version - filter expression
}

// StreamRequest represents a stream request
type StreamRequest struct {
	ID    string `json:"id"`
	State string `json:"state"` // start, stop, pause, resume
}

// SubscribeRequest represents an event subscription request (new in draft)
type SubscribeRequest struct {
	EventType string                 `json:"event_type"`
	Filter    map[string]interface{} `json:"filter,omitempty"`
}

// ResponseType defines the type of response
type ResponseType string

// Response types
const (
	ResponseTypeToolResult      ResponseType = "tool_result"
	ResponseTypeResourceContent ResponseType = "resource_content"
	ResponseTypePromptContent   ResponseType = "prompt_content"
	ResponseTypeList            ResponseType = "list"
	ResponseTypeStreamUpdate    ResponseType = "stream_update"
	ResponseTypeSubscription    ResponseType = "subscription" // New in draft version
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
	Result     interface{}     `json:"result"`
	Streaming  bool            `json:"streaming,omitempty"`
	CacheInfo  *CacheInfo      `json:"cache_info,omitempty"` // New in draft version
	Statistics *ToolStatistics `json:"statistics,omitempty"` // New in draft version
}

// CacheInfo provides information about cache hits/misses
type CacheInfo struct {
	Hit      bool   `json:"hit"`
	Age      int    `json:"age,omitempty"` // Age in seconds
	Source   string `json:"source,omitempty"`
	CacheKey string `json:"cache_key,omitempty"`
}

// ToolStatistics provides execution statistics
type ToolStatistics struct {
	ExecutionTime   int64  `json:"execution_time_ms"`
	MemoryUsed      int64  `json:"memory_used_bytes,omitempty"`
	CPUTime         int64  `json:"cpu_time_ms,omitempty"`
	InvocationCount int    `json:"invocation_count,omitempty"`
	Status          string `json:"status,omitempty"`
}

// ResourceContentResponse represents a resource content response
type ResourceContentResponse struct {
	Content    interface{} `json:"content"`
	Version    string      `json:"version,omitempty"`    // New in draft version
	Timestamp  string      `json:"timestamp,omitempty"`  // New in draft version
	Compressed bool        `json:"compressed,omitempty"` // New in draft version
	Partial    bool        `json:"partial,omitempty"`    // New in draft version for range requests
}

// PromptContentResponse represents a prompt content response
type PromptContentResponse struct {
	Content []Content `json:"content"`
	Format  string    `json:"format,omitempty"` // New in draft version
}

// ListResponse represents a list response
type ListResponse struct {
	Items []ItemDefinition `json:"items"`
	Total int              `json:"total,omitempty"` // New in draft version - total items count
}

// StreamUpdateResponse represents a stream update response
type StreamUpdateResponse struct {
	ID      string      `json:"id"`
	State   string      `json:"state"` // running, paused, stopped, completed, error
	Update  interface{} `json:"update,omitempty"`
	Partial bool        `json:"partial,omitempty"` // New in draft version
}

// SubscriptionResponse represents a subscription response (new in draft)
type SubscriptionResponse struct {
	SubscriptionID string `json:"subscription_id"`
	EventType      string `json:"event_type"`
	Status         string `json:"status"`
}

// ItemDefinition represents an item in a list response
type ItemDefinition struct {
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Streamable  bool     `json:"streamable,omitempty"`
	Cacheable   bool     `json:"cacheable,omitempty"`  // New in draft version
	Versioned   bool     `json:"versioned,omitempty"`  // New in draft version
	Tags        []string `json:"tags,omitempty"`       // New in draft version
	Category    string   `json:"category,omitempty"`   // New in draft version
	Updated     string   `json:"updated_at,omitempty"` // New in draft version
}

// EventType defines the type of event (new in draft)
type EventType string

// Event types
const (
	EventTypeToolExecution    EventType = "tool_execution"
	EventTypeResourceChange   EventType = "resource_change"
	EventTypeServerStatus     EventType = "server_status"
	EventTypeConnectionStatus EventType = "connection_status"
)

// EventMessage represents an event message (new in draft)
type EventMessage struct {
	Type      EventType       `json:"type"`
	Timestamp string          `json:"timestamp"`
	Source    string          `json:"source"`
	Content   json.RawMessage `json:"content"`
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
	ErrorCodeStreamNotFound      ErrorCode = "stream_not_found"
	ErrorCodeStreamUnavailable   ErrorCode = "stream_unavailable"
	ErrorCodeInternalServerError ErrorCode = "internal_server_error"
	ErrorCodeSubscriptionFailed  ErrorCode = "subscription_failed" // New in draft
	ErrorCodePermissionDenied    ErrorCode = "permission_denied"   // New in draft
	ErrorCodeRateLimitExceeded   ErrorCode = "rate_limit_exceeded" // New in draft
)

// ErrorResponse represents an MCP error message
type ErrorResponse struct {
	Code      ErrorCode     `json:"code"`
	Message   string        `json:"message"`
	RequestID string        `json:"request_id,omitempty"`
	Details   *ErrorDetails `json:"details,omitempty"` // New in draft version
}

// ErrorDetails provides additional error context (new in draft)
type ErrorDetails struct {
	RetryAfter    int                    `json:"retry_after,omitempty"`   // Seconds to wait before retry
	Field         string                 `json:"field,omitempty"`         // Field that caused the error
	Suggestion    string                 `json:"suggestion,omitempty"`    // Suggested fix
	Documentation string                 `json:"documentation,omitempty"` // Link to documentation
	Context       map[string]interface{} `json:"context,omitempty"`       // Additional error context
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

// StructuredContent represents a structured content block
type StructuredContent struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
}

// GetType returns the content type
func (sc StructuredContent) GetType() string {
	return sc.Type
}

// RichContent represents a rich content block with multiple formats (new in draft)
type RichContent struct {
	Type    string                 `json:"type"`
	Content interface{}            `json:"content"`
	Formats map[string]interface{} `json:"formats,omitempty"` // Different format representations
}

// GetType returns the content type
func (rc RichContent) GetType() string {
	return rc.Type
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
	case RequestTypeStream:
		var streamReq StreamRequest
		if err := json.Unmarshal(req.Content, &streamReq); err != nil {
			return nil, fmt.Errorf("failed to unmarshal stream request: %w", err)
		}
		return streamReq, nil
	case RequestTypeSubscribe:
		var subscribeReq SubscribeRequest
		if err := json.Unmarshal(req.Content, &subscribeReq); err != nil {
			return nil, fmt.Errorf("failed to unmarshal subscribe request: %w", err)
		}
		return subscribeReq, nil
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
	case ResponseTypeStreamUpdate:
		var streamResp StreamUpdateResponse
		if err := json.Unmarshal(resp.Content, &streamResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal stream update response: %w", err)
		}
		return streamResp, nil
	case ResponseTypeSubscription:
		var subResp SubscriptionResponse
		if err := json.Unmarshal(resp.Content, &subResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal subscription response: %w", err)
		}
		return subResp, nil
	default:
		return nil, fmt.Errorf("unknown response type: %s", resp.Type)
	}
}

// UnmarshalError unmarshals a BaseMessage into an error response
func UnmarshalError(msg BaseMessage) (*ErrorResponse, error) {
	var errResp ErrorResponse
	if err := json.Unmarshal(msg.Content, &errResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal error response: %w", err)
	}
	return &errResp, nil
}

// UnmarshalEvent unmarshals a BaseMessage into an event message (new in draft)
func UnmarshalEvent(msg BaseMessage) (*EventMessage, error) {
	var eventMsg EventMessage
	if err := json.Unmarshal(msg.Content, &eventMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event message: %w", err)
	}
	return &eventMsg, nil
}
