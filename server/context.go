// Package server provides the server-side implementation of the MCP protocol.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// Context represents the context for a server request.
type Context struct {
	// Standard Go context for cancellation and timeout
	ctx context.Context

	// The raw request bytes
	RequestBytes []byte

	// The parsed request
	Request *Request

	// The response to be sent back
	Response *Response

	// The server instance
	server *serverImpl

	// Logger for this request
	Logger *slog.Logger

	// Version of the MCP protocol being used
	Version string

	// Request ID for tracing
	RequestID string

	// Metadata for storing contextual information during request processing
	Metadata map[string]interface{}
}

// Request represents an incoming JSON-RPC 2.0 request.
type Request struct {
	// JSON-RPC 2.0 fields
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`     // string or number or null
	Method  string          `json:"method"`           // The method to call
	Params  json.RawMessage `json:"params,omitempty"` // Parameters for the method call

	// Parsed params based on method type
	// These fields are populated after parsing
	ToolName     string
	ToolArgs     map[string]interface{}
	ResourcePath string
	PromptName   string
	PromptArgs   map[string]interface{}
}

// Response represents an outgoing JSON-RPC 2.0 response.
type Response struct {
	// JSON-RPC 2.0 fields
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`     // Must match request ID
	Result  interface{} `json:"result,omitempty"` // Result data, null if error
	Error   *RPCError   `json:"error,omitempty"`  // Error data, null if success
}

// RPCError represents a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int         `json:"code"`    // Error code
	Message string      `json:"message"` // Error message
	Data    interface{} `json:"data,omitempty"`
}

// NewContext creates a new request context.
func NewContext(ctx context.Context, requestBytes []byte, server *serverImpl) (*Context, error) {
	// Create a basic context with the server instance
	reqCtx := &Context{
		ctx:          ctx,
		RequestBytes: requestBytes,
		server:       server,
		Logger:       server.logger,
		Metadata:     make(map[string]interface{}),
	}

	// Parse the request
	request := &Request{}
	if err := json.Unmarshal(requestBytes, request); err != nil {
		return reqCtx, err
	}

	reqCtx.Request = request
	reqCtx.RequestID = stringify(request.ID) // Convert ID to string for internal use

	// Default to latest protocol version if not specified
	reqCtx.Version = "2025-03-26"

	// Parse specific request type based on method
	switch request.Method {
	case "tools/call":
		// Parse tool call request params
		var toolParams struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(request.Params, &toolParams); err != nil {
			return reqCtx, err
		}
		request.ToolName = toolParams.Name
		request.ToolArgs = toolParams.Arguments

	case "resources/read":
		// Parse resource request params
		var resourceParams struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(request.Params, &resourceParams); err != nil {
			return reqCtx, err
		}
		request.ResourcePath = resourceParams.URI

	case "prompts/get":
		// Parse prompt request params
		var promptParams struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
		}
		if err := json.Unmarshal(request.Params, &promptParams); err != nil {
			return reqCtx, err
		}
		request.PromptName = promptParams.Name
		request.PromptArgs = promptParams.Arguments
	}

	// Create a response with the same ID and JSON-RPC version
	reqCtx.Response = &Response{
		JSONRPC: "2.0",
		ID:      request.ID,
	}

	return reqCtx, nil
}

// stringify converts an ID (which could be string, number, or null) to a string
func stringify(id interface{}) string {
	if id == nil {
		return ""
	}
	switch v := id.(type) {
	case string:
		return v
	case float64, float32, int, int64, int32:
		return json.Number(fmt.Sprintf("%v", v)).String()
	default:
		return fmt.Sprintf("%v", id)
	}
}

// Done returns a channel that's closed when this context is canceled.
func (c *Context) Done() <-chan struct{} {
	return c.ctx.Done()
}

// Deadline returns the time when this context will be canceled, if any.
func (c *Context) Deadline() (deadline interface{}, ok bool) {
	return c.ctx.Deadline()
}

// Err returns nil if Done is not yet closed, otherwise it returns the reason.
func (c *Context) Err() error {
	return c.ctx.Err()
}

// Value returns the value associated with this context for key, or nil.
func (c *Context) Value(key interface{}) interface{} {
	return c.ctx.Value(key)
}

// ExecuteTool provides a convenient way to execute a tool from within another tool handler.
// This is useful for tool composition and internal tool calls.
func (c *Context) ExecuteTool(toolName string, args map[string]interface{}) (interface{}, error) {
	// Forward to the server's executeTool method
	if c.server == nil {
		return nil, fmt.Errorf("server not available in context")
	}
	return c.server.executeTool(c, toolName, args)
}

// GetRegisteredTools returns a list of all tools registered with the server.
// This is useful for tools that need to inspect or enumerate available tools.
func (c *Context) GetRegisteredTools() ([]*Tool, error) {
	if c.server == nil {
		return nil, fmt.Errorf("server not available in context")
	}

	c.server.mu.RLock()
	defer c.server.mu.RUnlock()

	tools := make([]*Tool, 0, len(c.server.tools))
	for _, tool := range c.server.tools {
		tools = append(tools, tool)
	}

	return tools, nil
}

// GetToolDetails returns detailed information about a specific tool.
// This is useful for tools that need to inspect the capabilities of other tools.
func (c *Context) GetToolDetails(toolName string) (*Tool, error) {
	if c.server == nil {
		return nil, fmt.Errorf("server not available in context")
	}

	c.server.mu.RLock()
	defer c.server.mu.RUnlock()

	tool, exists := c.server.tools[toolName]
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	return tool, nil
}

// ExecuteResource provides a convenient way to execute a resource from within another resource handler.
// This is useful for resource composition and internal resource calls.
func (c *Context) ExecuteResource(resourcePath string) (interface{}, error) {
	// Forward to the server's processResourceRequest method
	if c.server == nil {
		return nil, fmt.Errorf("server not available in context")
	}

	// Find the resource and extract parameters
	resource, params, exists := c.server.findResourceAndExtractParams(resourcePath)
	if !exists {
		return nil, fmt.Errorf("resource not found: %s", resourcePath)
	}

	// Execute the resource handler
	result, err := resource.Handler(c, params)
	if err != nil {
		return nil, fmt.Errorf("resource execution failed: %w", err)
	}

	return result, nil
}

// GetRegisteredResources returns a list of all resources registered with the server.
// This is useful for handlers that need to inspect or enumerate available resources.
func (c *Context) GetRegisteredResources() ([]*Resource, error) {
	if c.server == nil {
		return nil, fmt.Errorf("server not available in context")
	}

	c.server.mu.RLock()
	defer c.server.mu.RUnlock()

	resources := make([]*Resource, 0, len(c.server.resources))
	for _, resource := range c.server.resources {
		resources = append(resources, resource)
	}

	return resources, nil
}

// GetResourceDetails returns detailed information about a specific resource.
// This is useful for handlers that need to inspect the capabilities of other resources.
func (c *Context) GetResourceDetails(resourcePath string) (*Resource, error) {
	if c.server == nil {
		return nil, fmt.Errorf("server not available in context")
	}

	c.server.mu.RLock()
	defer c.server.mu.RUnlock()

	// First try direct match by path
	resource, exists := c.server.resources[resourcePath]
	if exists {
		return resource, nil
	}

	// Otherwise try to find by pattern matching
	for _, resource := range c.server.resources {
		if resource.Template != nil {
			if _, matched := resource.Template.Match(resourcePath); matched {
				return resource, nil
			}
		}
	}

	return nil, fmt.Errorf("resource not found: %s", resourcePath)
}

// GetSamplingController provides access to the server's sampling controller.
func (c *Context) GetSamplingController() (*SamplingController, error) {
	if c.server == nil {
		return nil, fmt.Errorf("server not available in context")
	}

	if c.server.samplingController == nil {
		return nil, fmt.Errorf("sampling controller not initialized")
	}

	return c.server.samplingController, nil
}

// RequestSampling sends a sampling request using the context's session information.
// This is a convenience wrapper around the server's RequestSamplingFromContext method.
func (c *Context) RequestSampling(messages []SamplingMessage, preferences SamplingModelPreferences,
	systemPrompt string, maxTokens int) (*SamplingResponse, error) {

	if c.server == nil {
		return nil, fmt.Errorf("server not available in context")
	}

	return c.server.RequestSamplingFromContext(c, messages, preferences, systemPrompt, maxTokens)
}

// RequestSamplingWithPriority sends a sampling request with a specific priority level.
// The priority affects timeout and retry behavior according to the server's configuration.
func (c *Context) RequestSamplingWithPriority(messages []SamplingMessage, preferences SamplingModelPreferences,
	systemPrompt string, maxTokens int, priority int) (*SamplingResponse, error) {

	if c.server == nil {
		return nil, fmt.Errorf("server not available in context")
	}

	// Get appropriate options based on priority
	controller, err := c.GetSamplingController()
	if err != nil {
		return nil, err
	}

	options := controller.GetRequestOptions(priority)

	// Apply rate limiting
	sessionID := SessionID("")
	if sessionVal, ok := c.Metadata["sessionID"]; ok {
		if sessionIDStr, ok := sessionVal.(string); ok {
			sessionID = SessionID(sessionIDStr)
		}
	}

	// Check if we can process this request based on rate limits
	if !controller.CanProcessRequest(sessionID) {
		return nil, fmt.Errorf("request rate limit exceeded")
	}

	// Record the request for rate limiting purposes
	controller.RecordRequest(sessionID)

	// Set up deferred completion recording
	defer controller.CompleteRequest(sessionID)

	// Execute with appropriate options
	return c.server.RequestSamplingWithSessionAndOptions(
		sessionID,
		c.Version,
		messages,
		preferences,
		systemPrompt,
		maxTokens,
		options,
	)
}

// ValidateSamplingRequest validates that a sampling request is valid for the current protocol version
// and client capabilities.
func (c *Context) ValidateSamplingRequest(messages []SamplingMessage, maxTokens int) error {
	if c.server == nil {
		return fmt.Errorf("server not available in context")
	}

	// Get the controller
	controller, err := c.GetSamplingController()
	if err != nil {
		return err
	}

	// Validate against protocol constraints
	return controller.ValidateForProtocol(c.Version, messages, maxTokens)
}
