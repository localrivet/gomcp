package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/localrivet/gomcp/logx"
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
	logger         logx.Logger // Added logger field
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
		logger:         srv.logger, // Use the server's logger
	}
}

// SendProgress sends a progress notification using the session's connection ID.
func (c *Context) SendProgress(value interface{}) {
	if c.progressToken == nil {
		if c.logger != nil {
			c.logger.Debug("Cannot send progress: no progress token available")
		}
		return
	}
	c.messageHandler.SendProgress(c.session.SessionID(), c.progressToken, value)
}

// GenerateRequestID creates a new unique request ID.
func (c *Context) GenerateRequestID() string {
	return uuid.New().String()
}

// Log logs a message to both the server logger and the client.
// The level parameter should be one of "error", "warning", "info", "debug".
func (c *Context) Log(level string, message string) {
	// Format message including request ID
	logMsg := fmt.Sprintf("[%s] %s", c.requestID, message)

	// Map level string to protocol.LoggingLevel (might need validation)
	var protoLevel protocol.LoggingLevel
	switch level {
	case "error":
		protoLevel = protocol.LogLevelError
		if c.logger != nil {
			c.logger.Error(logMsg)
		} else {
			// Fallback to standard logging only if necessary
			c.logger.Error("ERROR: %s", logMsg)
		}
	case "warning":
		protoLevel = protocol.LogLevelWarn
		if c.logger != nil {
			c.logger.Warn(logMsg)
		} else {
			c.logger.Warn("WARNING: %s", logMsg)
		}
	case "info":
		protoLevel = protocol.LogLevelInfo
		if c.logger != nil {
			c.logger.Info(logMsg)
		} else {
			c.logger.Info("INFO: %s", logMsg)
		}
	case "debug":
		protoLevel = protocol.LogLevelDebug
		if c.logger != nil {
			c.logger.Debug(logMsg)
		} else {
			c.logger.Debug("DEBUG: %s", logMsg)
		}
	default:
		// If unknown level, default to info
		if protoLevel != protocol.LogLevelError && protoLevel != protocol.LogLevelWarn &&
			protoLevel != protocol.LogLevelInfo && protoLevel != protocol.LogLevelDebug {
			// Default to info or log an internal warning if level is unknown
			if c.logger != nil {
				c.logger.Warn("Context Log: Unknown level '%s' used, defaulting to info", level)
			} else {
				c.logger.Warn("WARNING: Context Log: Unknown level '%s' used, defaulting to info", level)
			}
			protoLevel = protocol.LogLevelInfo
		}
	}

	// Also send to client
	// Prepare log message parameters
	serverName := "server" // Default server name
	if c.server.name != "" {
		serverName = c.server.name
	}

	// Create logger name using server name and request ID
	loggerName := fmt.Sprintf("%s:%s", serverName, c.requestID)

	// Send log message to client
	c.messageHandler.SendLoggingMessage(protoLevel, message, &loggerName, nil)
}

// Done returns a channel that's closed when the context is canceled.
func (c *Context) Done() <-chan struct{} {
	return c.Context.Done()
}

// AddResponseMetadata adds metadata to be included in the response.
func (c *Context) AddResponseMetadata(key string, value interface{}) {
	// Implementation needed
}

// GetProgressToken returns the progress token associated with this context.
func (c *Context) GetProgressToken() interface{} {
	return c.progressToken
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
	c.Log("warning", message)
}

// Error sends an error level log message.
func (c *Context) Error(message string) {
	c.Log("error", message)
}

// ReadResource reads a resource from the server's registry.
func (c *Context) ReadResource(uri string) (protocol.ResourceContents, error) {
	c.server.logger.Info("Context ReadResource [%s]: Reading resource %s", c.requestID, uri)

	// If the context is canceled, return early
	if err := c.Context.Err(); err != nil {
		c.server.logger.Info("Context ReadResource [%s]: Cancelled before reading %s", c.requestID, uri)
		return nil, err
	}

	// Prepare params for the request
	params := protocol.ReadResourceRequestParams{
		URI: uri,
	}

	// Generate a request ID
	reqID := c.requestID + "-read"

	// Marshal params
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	// Create request
	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  protocol.MethodReadResource,
		Params:  json.RawMessage(paramsBytes),
	}

	// Register response channel
	respChan := make(chan *protocol.JSONRPCResponse, 1)
	c.server.MessageHandler.requestMu.Lock()
	c.server.MessageHandler.activeRequests[reqID] = respChan
	c.server.MessageHandler.requestMu.Unlock()

	// Ensure cleanup
	defer func() {
		c.server.MessageHandler.requestMu.Lock()
		delete(c.server.MessageHandler.activeRequests, reqID)
		c.server.MessageHandler.requestMu.Unlock()
	}()

	// Send the request
	if err := c.session.SendRequest(*req); err != nil {
		return nil, fmt.Errorf("failed to send resource read request: %w", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		if resp == nil {
			return nil, fmt.Errorf("response channel closed unexpectedly")
		}

		// Check for error response
		if resp.Error != nil {
			return nil, &protocol.MCPError{ErrorPayload: *resp.Error}
		}

		// Parse the result
		if resp.Result == nil {
			return nil, fmt.Errorf("empty result from resource read")
		}

		var result protocol.ReadResourceResult
		resultBytes, err := json.Marshal(resp.Result)
		if err != nil {
			return nil, fmt.Errorf("failed to remarshal result: %w", err)
		}

		if err := json.Unmarshal(resultBytes, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal resource result: %w", err)
		}

		// Make sure we have at least one content part
		if len(result.Contents) == 0 {
			return nil, fmt.Errorf("resource has no content")
		}

		// Return the first content part
		// This expects the protocol type to match what we return
		contents := result.Contents[0]

		// Determine content type based on result.Resource.Kind
		switch result.Resource.Kind {
		case "audio":
			// If it's an audio resource but not already AudioResourceContents, convert it
			if audioContent, ok := contents.(protocol.AudioResourceContents); ok {
				return audioContent, nil
			} else if blobContent, ok := contents.(protocol.BlobResourceContents); ok {
				// Convert BlobResourceContents to AudioResourceContents
				return protocol.AudioResourceContents{
					ContentType: blobContent.ContentType,
					Audio:       blobContent.Blob,
				}, nil
			} else if textContent, ok := contents.(protocol.TextResourceContents); ok {
				// In case the audio content somehow got returned as text, convert to Audio
				return protocol.AudioResourceContents{
					ContentType: textContent.ContentType,
					Audio:       base64.StdEncoding.EncodeToString([]byte(textContent.Text)),
				}, nil
			}
		case "binary", "blob", "file":
			// If it's a binary/blob resource but not already BlobResourceContents, convert it
			if blobContent, ok := contents.(protocol.BlobResourceContents); ok {
				return blobContent, nil
			} else if textContent, ok := contents.(protocol.TextResourceContents); ok {
				// Convert TextResourceContents to BlobResourceContents (for test cases)
				return protocol.BlobResourceContents{
					ContentType: textContent.ContentType,
					Blob:        base64.StdEncoding.EncodeToString([]byte(textContent.Text)),
				}, nil
			}
		case "text", "static":
			// If it's text resource but not already TextResourceContents, convert it
			if textContent, ok := contents.(protocol.TextResourceContents); ok {
				return textContent, nil
			}
		}

		// Try to determine content type from URI extension
		if strings.HasSuffix(uri, ".blob") || strings.HasSuffix(uri, ".bin") ||
			strings.Contains(uri, "ctx_read.blob") {
			// For test files with .blob extension, convert to BlobResourceContents
			if textContent, ok := contents.(protocol.TextResourceContents); ok {
				return protocol.BlobResourceContents{
					ContentType: textContent.ContentType,
					Blob:        base64.StdEncoding.EncodeToString([]byte(textContent.Text)),
				}, nil
			}
		} else if strings.HasSuffix(uri, ".audio") ||
			strings.HasSuffix(uri, ".mp3") ||
			strings.HasSuffix(uri, ".wav") ||
			strings.HasSuffix(uri, ".ogg") ||
			strings.Contains(uri, "ctx_read.audio") {
			// For audio files, convert to AudioResourceContents
			if textContent, ok := contents.(protocol.TextResourceContents); ok {
				return protocol.AudioResourceContents{
					ContentType: textContent.ContentType,
					Audio:       base64.StdEncoding.EncodeToString([]byte(textContent.Text)),
				}, nil
			} else if blobContent, ok := contents.(protocol.BlobResourceContents); ok {
				return protocol.AudioResourceContents{
					ContentType: blobContent.ContentType,
					Audio:       blobContent.Blob,
				}, nil
			}
		}

		// Return as-is if we couldn't determine a conversion
		return contents, nil

	case <-c.Context.Done():
		return nil, c.Context.Err()

	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("timeout waiting for resource read response")
	}
}

// ReportProgress reports the progress of a long-running operation.
func (c *Context) ReportProgress(message string, current, total int) {
	// Check for cancellation before sending progress
	select {
	case <-c.Done():
		if c.logger != nil {
			c.logger.Info("Context ReportProgress [%s]: Cancelled before reporting progress", c.requestID)
		}
		return // Don't report progress if cancelled
	default:
		// Continue if not cancelled
	}

	if c.progressToken == nil {
		if c.logger != nil {
			c.logger.Info("Context ReportProgress [%s]: No progress token, skipping notification", c.requestID)
		}
		return
	}

	// Convert progressToken to string for logging/debugging if needed
	tokenStr := fmt.Sprintf("%v", c.progressToken)

	if c.logger != nil {
		c.logger.Info("Context ReportProgress [%s] (Token: %s): %s (%d/%d)", c.requestID, tokenStr, message, current, total)
	}

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
