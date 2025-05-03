package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
)

// Context provides access to server capabilities within tool and resource handlers.
// It also embeds context.Context for cancellation.
type Context struct {
	context.Context
	requestID      string
	session        types.ClientSession
	progressToken  interface{}
	server         *Server
	messageHandler *MessageHandler
}

// NewContext creates a new Context instance for a request.
// It requires a parent context (e.g., from context.WithCancel).
func NewContext(parentCtx context.Context, requestID string, session types.ClientSession, progressToken interface{}, srv *Server) *Context {
	return &Context{
		Context:        parentCtx,
		requestID:      requestID,
		session:        session,
		progressToken:  progressToken,
		server:         srv,
		messageHandler: srv.MessageHandler,
	}
}

// TODO: Add methods for accessing capabilities (e.g., Log, ReportProgress, ReadResource, CallTool)

// Log sends a log message to the client.
func (c *Context) Log(level string, message string) {
	// Format message including request ID
	logMsg := fmt.Sprintf("[%s] %s", c.requestID, message)

	// Map level string to protocol.LoggingLevel (might need validation)
	protoLevel := protocol.LoggingLevel(level)
	if protoLevel != protocol.LogLevelError && protoLevel != protocol.LogLevelWarn &&
		protoLevel != protocol.LogLevelInfo && protoLevel != protocol.LogLevelDebug {
		// Default to info or log an internal warning if level is unknown
		c.server.logger.Warn("Context Log: Unknown level '%s' used, defaulting to info", level)
		protoLevel = protocol.LogLevelInfo
	}

	params := protocol.LoggingMessageParams{
		Level:   protoLevel,
		Message: logMsg,
		// Logger: // Could potentially add a logger name like "tool-execution"
		// Data: // Could add request ID here as structured data
	}
	notification := protocol.NewNotification(protocol.MethodNotificationMessage, params)

	// Send notification via the session associated with this context
	if err := c.session.SendNotification(*notification); err != nil {
		// Log error using the *server's* logger
		c.server.logger.Error("Context Log: Failed to send log notification for session %s, request %s: %v", c.session.SessionID(), c.requestID, err)
	}
}

// Info sends an info level log message.
func (c *Context) Info(message string) {
	c.Log("info", message)
}

// Debug sends a debug level log message.
func (c *Context) Debug(message string) {
	c.Log("debug", message)
}

// Warning sends a warning level log message.
func (c *Context) Warning(message string) {
	c.Log(string(protocol.LogLevelWarn), message)
}

// Error sends an error level log message.
func (c *Context) Error(message string) {
	c.Log("error", message)
}

// ReadResource reads the content of a resource.
func (c *Context) ReadResource(uri string) (protocol.ResourceContents, error) {
	// Check for cancellation before proceeding
	select {
	case <-c.Done():
		c.server.logger.Info("Context ReadResource [%s]: Cancelled before reading %s", c.requestID, uri)
		return nil, c.Err() // Return context error (cancelled or deadline exceeded)
	default:
		// Continue if not cancelled
	}

	c.server.logger.Info("Context ReadResource [%s]: Reading resource %s", c.requestID, uri)

	// 1. Get resource metadata from registry
	resource, ok := c.server.Registry.GetResource(uri)
	if !ok {
		return nil, &protocol.MCPError{
			ErrorPayload: protocol.ErrorPayload{
				Code:    protocol.CodeMCPResourceNotFound,
				Message: fmt.Sprintf("Resource not found in registry: %s", uri),
			},
		}
	}

	// 2. Read content based on kind
	var contents protocol.ResourceContents
	var readErr error

	switch resource.Kind {
	case string(protocol.ResourceKindFile):
		u, err := url.Parse(resource.URI)
		if err != nil || u.Scheme != "file" {
			readErr = fmt.Errorf("invalid or unsupported file URI (%s): %w", resource.URI, err)
			break
		}
		filePath := u.Path
		// Check for cancellation before reading file
		select {
		case <-c.Done():
			return nil, c.Err()
		default:
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			readErr = fmt.Errorf("failed to read file %s: %w", filePath, err)
			break
		}
		contentType := "text/plain" // TODO: Determine content type
		contents = protocol.TextResourceContents{
			ContentType: contentType,
			Content:     string(data),
		}

	case string(protocol.ResourceKindBlob):
		u, err := url.Parse(resource.URI)
		if err != nil || u.Scheme != "file" {
			readErr = fmt.Errorf("invalid or unsupported blob file URI (%s): %w", resource.URI, err)
			break
		}
		filePath := u.Path
		// Check for cancellation before reading file
		select {
		case <-c.Done():
			return nil, c.Err()
		default:
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			readErr = fmt.Errorf("failed to read blob file %s: %w", filePath, err)
			break
		}
		contentType := "application/octet-stream" // TODO: Determine content type
		contents = protocol.BlobResourceContents{
			ContentType: contentType,
			Blob:        base64.StdEncoding.EncodeToString(data),
		}

	case string(protocol.ResourceKindAudio):
		u, err := url.Parse(resource.URI)
		if err != nil || u.Scheme != "file" {
			readErr = fmt.Errorf("invalid or unsupported audio file URI (%s): %w", resource.URI, err)
			break
		}
		filePath := u.Path
		// Check for cancellation before reading file
		select {
		case <-c.Done():
			return nil, c.Err()
		default:
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			readErr = fmt.Errorf("failed to read audio file %s: %w", filePath, err)
			break
		}
		contentType := "audio/unknown" // TODO: Determine content type
		contents = protocol.AudioResourceContents{
			ContentType: contentType,
			Audio:       base64.StdEncoding.EncodeToString(data),
		}

	default:
		readErr = fmt.Errorf("unsupported resource kind for reading via context: %s", resource.Kind)
	}

	if readErr != nil {
		c.server.logger.Error("Context ReadResource [%s]: Error reading %s: %v", c.requestID, uri, readErr)
		// Return a generic MCPError
		return nil, &protocol.MCPError{
			ErrorPayload: protocol.ErrorPayload{
				Code:    protocol.CodeMCPOperationFailed,
				Message: fmt.Sprintf("Failed to read resource: %v", readErr),
			},
		}
	}

	return contents, nil
}

// ReportProgress reports the progress of a long-running operation.
func (c *Context) ReportProgress(message string, current, total int) {
	// Check for cancellation before sending progress
	select {
	case <-c.Done():
		log.Printf("Context ReportProgress [%s]: Cancelled before reporting progress", c.requestID)
		return // Don't report progress if cancelled
	default:
		// Continue if not cancelled
	}

	if c.progressToken == nil {
		log.Printf("Context ReportProgress [%s]: No progress token, skipping notification", c.requestID)
		return
	}

	// Convert progressToken to string for logging/debugging if needed
	tokenStr := fmt.Sprintf("%v", c.progressToken)

	log.Printf("Context ReportProgress [%s] (Token: %s): %s (%d/%d)", c.requestID, tokenStr, message, current, total)

	// Construct the progress value payload
	progressValue := map[string]interface{}{
		"message": message,
		"current": current,
		"total":   total,
	}

	// Construct the progress notification parameters
	// Convert token to string as protocol.ProgressParams expects string
	tokenStr = fmt.Sprintf("%v", c.progressToken)
	progressParams := protocol.ProgressParams{
		Token: tokenStr,
		Value: progressValue,
	}

	// Construct the JSON-RPC notification
	notification := protocol.NewNotification("$/progress", progressParams)

	// Send the notification to the specific connection
	// Use the session stored in the context
	c.session.SendNotification(*notification) // Use session.SendNotification directly
}

// CreateMessage sends a sampling/createMessage request to the connected client and waits for a response.
// This allows server-side logic (e.g., a tool) to request an LLM completion via the client.
func (c *Context) CreateMessage(messages []protocol.SamplingMessage, opts *protocol.SamplingRequestParams) (*protocol.SamplingResult, error) {
	// Check if the client supports the sampling method (using negotiated version for now)
	if c.session.GetNegotiatedVersion() != protocol.CurrentProtocolVersion {
		return nil, fmt.Errorf("sampling/createMessage is not supported by the negotiated protocol version (%s)", c.session.GetNegotiatedVersion())
	}

	// Check if the transport supports sending requests (SSE does not)
	if _, isSSE := c.session.(interface {
		SendRequest(protocol.JSONRPCRequest) error
	}); !isSSE {
		// We can try a type assertion to the concrete type if we know it,
		// but a cleaner way might be to add a capability flag or method to the interface.
		// For now, let's assume any session that doesn't cause the interface check above to fail
		// implicitly means it's an SSE session or similar that doesn't support SendRequest fully.
		// Let's refine this check. Check if SendRequest returns the specific SSE error.
		if err := c.session.SendRequest(protocol.JSONRPCRequest{}); err != nil && err.Error() == "sending requests from server to client is not supported over SSE transport" {
			return nil, fmt.Errorf("sampling/createMessage cannot be sent: underlying transport (SSE) does not support server-to-client requests")
		}
		// If SendRequest exists but doesn't return the specific SSE error, we proceed, assuming it's implemented.
	}

	// Construct Params
	params := protocol.SamplingRequestParams{
		Messages: messages,
	}
	if opts != nil {
		params.ModelPreferences = opts.ModelPreferences
		params.SystemPrompt = opts.SystemPrompt
		params.MaxTokens = opts.MaxTokens
		// Copy other relevant fields from opts if necessary
	}

	// Generate unique request ID using UUID
	reqID := uuid.NewString()

	// Create response channel & register
	respChan := make(chan *protocol.JSONRPCResponse, 1)
	c.messageHandler.requestMu.Lock()
	c.messageHandler.activeRequests[reqID] = respChan
	c.messageHandler.requestMu.Unlock()

	// Ensure channel is removed if function exits unexpectedly (e.g., panic)
	// Or if request times out
	defer func() {
		c.messageHandler.requestMu.Lock()
		delete(c.messageHandler.activeRequests, reqID)
		c.messageHandler.requestMu.Unlock()
	}()

	// Construct request
	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  protocol.MethodSamplingCreateMessage,
		Params:  params,
	}

	// Send request via session
	if err := c.session.SendRequest(req); err != nil {
		return nil, fmt.Errorf("failed to send sampling/createMessage request to client: %w", err)
	}

	// Wait for response (with timeout based on context? Add config?)
	// Use a default timeout for now, maybe make configurable later
	select {
	case resp := <-respChan:
		if resp == nil {
			// Channel closed unexpectedly
			return nil, fmt.Errorf("response channel closed unexpectedly for request ID %s", reqID)
		}

		// Process response
		if resp.Error != nil {
			return nil, fmt.Errorf("client returned error for sampling request (ID: %s): Code=%d, Message='%s'", reqID, resp.Error.Code, resp.Error.Message)
		}

		// Unmarshal result
		var result protocol.SamplingResult
		if err := protocol.UnmarshalPayload(resp.Result, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal sampling result from client (ID: %s): %w", reqID, err)
		}
		return &result, nil

	case <-c.Context.Done(): // Check if the parent context (e.g., original request) was cancelled
		return nil, fmt.Errorf("context cancelled while waiting for sampling response (ID: %s): %w", reqID, c.Context.Err())
	case <-time.After(30 * time.Second): // Default 30-second timeout
		return nil, fmt.Errorf("timed out waiting for sampling response from client (ID: %s)", reqID)
	}
}

// CallTool sends a 'tools/call' request to the client and waits for the result.
// This allows a server-side tool to invoke a client-side tool.
// Input is marshalled to JSON.
// Returns the raw JSON output from the client tool and any tool execution error reported by the client.
func (c *Context) CallTool(toolName string, input interface{}) (json.RawMessage, *protocol.ToolError, error) {
	// 1. Generate unique request ID
	// Note: Using google/uuid requires adding it to go.mod
	// requestID := uuid.New().String() // Requires import "github.com/google/uuid"
	// Simple alternative for now:
	requestID := fmt.Sprintf("server-tool-call-%d", time.Now().UnixNano())

	// 2. Marshal input
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal tool input for %s: %w", toolName, err)
	}

	// 3. Construct ToolCall and Params
	toolCall := &protocol.ToolCall{
		// ID:       uuid.New().String(), // Unique ID for the tool call instance itself
		ID:       fmt.Sprintf("tc-%d", time.Now().UnixNano()), // Simple alternative
		ToolName: toolName,
		Input:    json.RawMessage(inputBytes),
	}
	params := protocol.CallToolRequestParams{
		ToolCall: toolCall,
		// Meta: Can be added if needed, e.g., to propagate progress token back
	}

	// 4. Create JSON-RPC Request
	// req := protocol.NewRequest(requestID, protocol.MethodCallTool, params) // Helper doesn't exist
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal CallToolRequestParams: %w", err)
	}
	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      requestID,
		Method:  protocol.MethodCallTool,
		Params:  json.RawMessage(paramsBytes),
	}

	// 5. Register response channel
	respChan := make(chan *protocol.JSONRPCResponse, 1)
	c.server.MessageHandler.requestMu.Lock()
	if _, exists := c.server.MessageHandler.activeRequests[requestID]; exists {
		// Very unlikely due to timestamp nano, but handle collision just in case
		c.server.MessageHandler.requestMu.Unlock()
		return nil, nil, fmt.Errorf("internal error: request ID collision for %s", requestID)
	}
	c.server.MessageHandler.activeRequests[requestID] = respChan
	c.server.MessageHandler.requestMu.Unlock()

	// Cleanup function to remove the channel
	cleanup := func() {
		c.server.MessageHandler.requestMu.Lock()
		delete(c.server.MessageHandler.activeRequests, requestID)
		c.server.MessageHandler.requestMu.Unlock()
		// Don't close respChan here, the select block handles it or sender closes
	}
	defer cleanup() // Ensure cleanup happens

	// 6. Send Request via Session
	if err := c.session.SendRequest(*req); err != nil {
		// cleanup() // defer handles cleanup
		return nil, nil, fmt.Errorf("failed to send tools/call request to client: %w", err)
	}

	// 7. Wait for Response (with timeout and context cancellation)
	// TODO: Make timeout configurable via server options or context
	timeoutDuration := 30 * time.Second
	select {
	case resp := <-respChan:
		if resp == nil {
			return nil, nil, fmt.Errorf("response channel closed unexpectedly for tools/call request %s", requestID)
		}

		// 8. Process Response
		if resp.Error != nil {
			// Top-level JSON-RPC error from client
			return nil, nil, fmt.Errorf("client returned JSON-RPC error for tools/call request %s: code=%d, msg=%s",
				requestID, resp.Error.Code, resp.Error.Message)
		}

		if resp.Result == nil {
			return nil, nil, fmt.Errorf("client returned nil result for tools/call request %s", requestID)
		}

		// Marshal the result interface{} before unmarshalling into CallToolResult
		resultBytes, err := json.Marshal(resp.Result)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal client response result for %s: %w", requestID, err)
		}

		var toolResult protocol.CallToolResult
		if err := json.Unmarshal(resultBytes, &toolResult); err != nil {
			// TODO: Handle potential V2024 result format from client?
			// For now, assume client responds with V2025 format.
			return nil, nil, fmt.Errorf("failed to unmarshal CallToolResult from client for %s: %w", requestID, err)
		}

		// Check if the ToolCallID matches (optional but good practice)
		if toolResult.ToolCallID != toolCall.ID {
			c.server.logger.Warn("Mismatched ToolCallID in client response for request %s: expected %s, got %s",
				requestID, toolCall.ID, toolResult.ToolCallID)
			// Depending on strictness, could return an error here
		}

		return toolResult.Output, toolResult.Error, nil // Return output and potential tool error

	case <-time.After(timeoutDuration):
		// cleanup() // defer handles cleanup
		return nil, nil, fmt.Errorf("timeout waiting for client response to tools/call request %s after %v", requestID, timeoutDuration)
	case <-c.Done(): // Access the embedded context directly
		// cleanup() // defer handles cleanup
		return nil, nil, fmt.Errorf("context cancelled while waiting for client response to tools/call request %s: %w", requestID, c.Err())
	}
}

// TODO: Add methods for CallTool, Sample, etc.
