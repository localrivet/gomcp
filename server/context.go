package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// Context represents the execution context for a server request.
// It encapsulates all request-specific information including request data,
// response data, server reference, and metadata for tracking the request.
// Context objects are created for each incoming client request and provide
// access to server functionality through convenience methods.
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
// It contains both the raw JSON-RPC fields and parsed method-specific fields
// which are populated during request processing based on the method type.
// The struct combines generic JSON-RPC structure with MCP-specific fields to
// avoid multiple parsing steps.
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
// It follows the JSON-RPC 2.0 specification with a result field for successful responses
// and an error field for failed ones. The ID field must match the corresponding request ID
// to allow clients to correlate responses with their requests.
type Response struct {
	// JSON-RPC 2.0 fields
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`     // Must match request ID
	Result  interface{} `json:"result,omitempty"` // Result data, null if error
	Error   *RPCError   `json:"error,omitempty"`  // Error data, null if success
}

// RPCError represents a JSON-RPC 2.0 error object.
// It includes a numeric error code, a human-readable message, and optional additional data.
// Error codes follow the JSON-RPC 2.0 specification: -32700 for parse errors,
// -32600 for invalid requests, -32601 for method not found, -32602 for invalid params,
// and -32603 for internal errors.
type RPCError struct {
	Code    int         `json:"code"`    // Error code
	Message string      `json:"message"` // Error message
	Data    interface{} `json:"data,omitempty"`
}

// NewContext creates a new request context for processing an incoming request.
// It parses the request bytes, initializes response structures, and extracts method-specific
// parameters based on the request method. This function is called for each incoming message
// to create a self-contained context for request processing.
//
// Parameters:
//   - ctx: Standard Go context for cancellation and timeouts
//   - requestBytes: Raw JSON-RPC request bytes
//   - server: Reference to the server implementation
//
// Returns:
//   - A new Context object ready for request processing
//   - An error if request parsing fails
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

// stringify converts an ID (which could be string, number, or null) to a string.
// This utility function handles various JSON-RPC ID formats including strings,
// numbers, and null values, providing a consistent string representation for internal use.
//
// Parameters:
//   - id: The JSON-RPC ID value to convert (interface{} to handle multiple types)
//
// Returns:
//   - A string representation of the ID, or empty string for null
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
// This method implements part of the standard Go context.Context interface,
// allowing the Context to be used with functions expecting a cancellable context.
func (c *Context) Done() <-chan struct{} {
	return c.ctx.Done()
}

// Deadline returns the time when this context will be canceled, if any.
// This method implements part of the standard Go context.Context interface.
func (c *Context) Deadline() (deadline interface{}, ok bool) {
	return c.ctx.Deadline()
}

// Err returns nil if Done is not yet closed, otherwise it returns the reason.
// This method implements part of the standard Go context.Context interface.
func (c *Context) Err() error {
	return c.ctx.Err()
}

// Value returns the value associated with this context for key, or nil.
// This method implements part of the standard Go context.Context interface.
func (c *Context) Value(key interface{}) interface{} {
	return c.ctx.Value(key)
}

// ExecuteTool provides a convenient way to execute a tool from within another tool handler.
// This is useful for tool composition and internal tool calls when one tool needs to
// invoke another as part of its implementation. The method handles parameter validation
// and result formatting automatically.
//
// Parameters:
//   - toolName: The name of the tool to execute
//   - args: A map of argument values to pass to the tool
//
// Returns:
//   - The result of the tool execution
//   - An error if the tool cannot be found or execution fails
func (c *Context) ExecuteTool(toolName string, args map[string]interface{}) (interface{}, error) {
	// Forward to the server's executeTool method
	if c.server == nil {
		return nil, fmt.Errorf("server not available in context")
	}
	return c.server.executeTool(c, toolName, args)
}

// GetRegisteredTools returns a list of all tools registered with the server.
// This is useful for tools that need to inspect or enumerate available tools,
// such as implementing a custom tools/list endpoint or providing tool discovery functionality.
//
// Returns:
//   - A slice of Tool objects containing all registered tools
//   - An error if the server reference is not available
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
// This is useful for tools that need to inspect the capabilities of other tools,
// validate tool availability before executing, or provide detailed tool information
// to clients.
//
// Parameters:
//   - toolName: The name of the tool to retrieve details for
//
// Returns:
//   - A Tool object containing the tool's metadata and handler
//   - An error if the tool doesn't exist or the server reference is not available
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
// This is useful for resource composition and internal resource calls, allowing one resource
// to build upon another or reuse existing resource handlers. The method handles path matching,
// parameter extraction, and result formatting.
//
// Parameters:
//   - resourcePath: The path of the resource to execute
//
// Returns:
//   - The result of the resource execution
//   - An error if the resource cannot be found or execution fails
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
// This is useful for handlers that need to inspect or enumerate available resources,
// such as implementing a custom resources/list endpoint or providing resource discovery
// functionality.
//
// Returns:
//   - A slice of Resource objects containing all registered resources
//   - An error if the server reference is not available
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
// This is useful for handlers that need to inspect the capabilities of other resources,
// validate resource availability, or provide detailed resource information to clients.
// The method supports both exact path matching and template pattern matching.
//
// Parameters:
//   - resourcePath: The path of the resource to retrieve details for
//
// Returns:
//   - A Resource object containing the resource's metadata and handler
//   - An error if the resource doesn't exist or the server reference is not available
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
// The sampling controller manages rate limiting, request tracking, and other aspects of
// the server's sampling behavior. This method is used by sampling-related functions to
// access the controller for validation and configuration.
//
// Returns:
//   - A pointer to the server's SamplingController
//   - An error if the server reference is not available or the controller is not initialized
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
// This is a convenience wrapper around the server's RequestSamplingFromContext method,
// which automatically uses the current context's session, protocol version, and other
// metadata when making the sampling request.
//
// Parameters:
//   - messages: A slice of SamplingMessage objects representing the conversation
//   - preferences: Model preferences for the sampling request
//   - systemPrompt: Optional system prompt to help guide the model's behavior
//   - maxTokens: Maximum number of tokens to generate in the response
//
// Returns:
//   - A SamplingResponse containing the model's generated content
//   - An error if the sampling request fails or the server is not available
func (c *Context) RequestSampling(messages []SamplingMessage, preferences SamplingModelPreferences,
	systemPrompt string, maxTokens int) (*SamplingResponse, error) {

	if c.server == nil {
		return nil, fmt.Errorf("server not available in context")
	}

	return c.server.RequestSamplingFromContext(c, messages, preferences, systemPrompt, maxTokens)
}

// RequestSamplingWithPriority sends a sampling request with a specific priority level.
// The priority affects timeout and retry behavior according to the server's configuration.
// Higher priority levels typically get more generous timeout and retry settings, while
// lower priority requests might have shorter timeouts and fewer retries.
//
// Parameters:
//   - messages: A slice of SamplingMessage objects representing the conversation
//   - preferences: Model preferences for the sampling request
//   - systemPrompt: Optional system prompt to help guide the model's behavior
//   - maxTokens: Maximum number of tokens to generate in the response
//   - priority: Priority level that determines timeout and retry behavior
//
// Returns:
//   - A SamplingResponse containing the model's generated content
//   - An error if the sampling request fails, times out, or the server is not available
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
// and client capabilities. It checks that the message content types are supported in the
// negotiated protocol version and that the requested token count is within acceptable limits.
//
// Parameters:
//   - messages: A slice of SamplingMessage objects to validate
//   - maxTokens: The requested maximum token count for the response
//
// Returns:
//   - An error if validation fails, describing the specific validation error
//   - nil if validation passes
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
