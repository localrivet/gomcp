package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/localrivet/gomcp/events"
	"github.com/localrivet/gomcp/mcp"
)

// handleMessage processes incoming JSON-RPC messages from clients.
// It determines if the message is a request or response and routes it appropriately.
// For requests, it calls HandleMessage to process them; for responses, it calls
// HandleJSONRPCResponse to match them with pending requests.
func (s *serverImpl) handleMessage(message []byte) ([]byte, error) {
	// Check if this is a response (has no "method" field but has "id")
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err == nil {
		if _, hasMethod := msg["method"]; !hasMethod {
			if _, hasID := msg["id"]; hasID {
				// This is a response, process it differently
				if err := s.HandleJSONRPCResponse(message); err != nil {
					s.logger.Error("failed to handle JSON-RPC response", "error", err)
				}
				return nil, nil
			}
		}
	}

	// This is a request, process normally
	return HandleMessage(s, message)
}

// HandleMessage handles an incoming message from the transport.
// It parses the message, routes it to the appropriate handler, and returns the response.
// Supports both single JSON-RPC messages and batch messages (arrays) as required by the MCP specification.
func HandleMessage(s *serverImpl, message []byte) ([]byte, error) {
	// Detect if this is a batch message (JSON array) or single message (JSON object)
	if isBatchMessage(message) {
		return handleBatchMessage(s, message)
	}

	// Handle single message (existing logic)
	return handleSingleMessage(s, message)
}

// isBatchMessage determines if the incoming message is a JSON array (batch) or single object
func isBatchMessage(message []byte) bool {
	// Trim whitespace and check if it starts with '['
	trimmed := bytes.TrimSpace(message)
	return len(trimmed) > 0 && trimmed[0] == '['
}

// handleBatchMessage processes a JSON-RPC batch message according to the JSON-RPC 2.0 specification
func handleBatchMessage(s *serverImpl, message []byte) ([]byte, error) {
	// Parse the batch array
	var batch []json.RawMessage
	if err := json.Unmarshal(message, &batch); err != nil {
		s.logger.Error("failed to parse batch message", "error", err)
		return createErrorResponse(nil, -32700, "Parse error", "Invalid batch format"), nil
	}

	// Validate that the batch is not empty (invalid per JSON-RPC 2.0 spec)
	if len(batch) == 0 {
		s.logger.Error("received empty batch message")
		return createErrorResponse(nil, -32600, "Invalid Request", "Batch cannot be empty"), nil
	}

	// Process each message in the batch
	var responses []interface{}
	for _, rawMessage := range batch {
		response := processBatchItem(s, rawMessage)
		// Only add responses for requests (not notifications)
		if response != nil {
			responses = append(responses, response)
		}
	}

	// If no responses were generated (all notifications), return nothing
	if len(responses) == 0 {
		return nil, nil
	}

	// Return the batch response
	responseBytes, err := json.Marshal(responses)
	if err != nil {
		s.logger.Error("failed to marshal batch response", "error", err)
		return createErrorResponse(nil, -32603, "Internal error", "Failed to marshal batch response"), nil
	}

	return responseBytes, nil
}

// processBatchItem processes a single item within a batch and returns the response (or nil for notifications)
func processBatchItem(s *serverImpl, rawMessage json.RawMessage) interface{} {
	// Process the individual message
	responseBytes, _ := handleSingleMessage(s, rawMessage)

	// If there's no response (notification), return nil
	if responseBytes == nil {
		return nil
	}

	// Parse the response back to an object for inclusion in the batch response
	var response interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		// If we can't parse the response, create an error response using structs
		s.logger.Error("failed to parse individual response in batch", "error", err)
		errorResp := mcp.NewErrorResponse(nil, -32603, "Internal error", "Failed to parse individual response")
		return errorResp
	}

	return response
}

// handleSingleMessage processes a single JSON-RPC message (extracted from original HandleMessage logic)
func handleSingleMessage(s *serverImpl, message []byte) ([]byte, error) {
	// Create a new context with the incoming message
	ctx, err := NewContext(context.Background(), message, s)
	if err != nil {
		s.logger.Error("failed to create context", "error", err)
		return createErrorResponse(nil, -32700, "Parse error", err.Error()), nil
	}

	var result interface{}

	// Process the message based on its method
	switch ctx.Request.Method {
	// Lifecycle methods
	case "initialize":
		result, err = s.ProcessInitialize(ctx)
	case "shutdown":
		result, err = s.ProcessShutdown(ctx)
	case "ping":
		// Simple working ping implementation
		result = map[string]interface{}{} // Return empty object as specified in the protocol

	// Tool methods
	case "tools/list":
		result, err = s.ProcessToolList(ctx)
	case "tools/call":
		result, err = s.ProcessToolCall(ctx)

	// Resource methods
	case "resources/list":
		result, err = s.ProcessResourceList(ctx)
	case "resources/read":
		result, err = s.ProcessResourceRequest(ctx)
	case "resources/subscribe":
		result, err = s.ProcessResourceSubscribe(ctx)
	case "resources/unsubscribe":
		result, err = s.ProcessResourceUnsubscribe(ctx)
	case "resources/templates/list":
		result, err = s.ProcessResourceTemplatesList(ctx)

	// Prompt methods
	case "prompts/list":
		result, err = s.ProcessPromptList(ctx)
	case "prompts/get":
		result, err = s.ProcessPromptRequest(ctx)

	// Utility methods
	case "logging/setLevel":
		result, err = s.ProcessLoggingSetLevel(ctx)
	case "completion/complete":
		result, err = s.ProcessCompletionComplete(ctx)

	// Client methods (server -> client)
	case "sampling/createMessage":
		result, err = s.ProcessSamplingCreateMessage(ctx)
	case "roots/list":
		// This is typically a client method that the server calls
		err = fmt.Errorf("method not implemented: %s", ctx.Request.Method)

	// Notifications
	case "notifications/initialized":
		// The client has finished initialization, process any pending notifications
		// Run asynchronously to avoid potential deadlocks with mutex acquisition
		go s.handleInitializedNotification()
		return nil, nil
	case "notifications/cancelled":
		// Handle cancellation notification
		if err := s.HandleCancelledNotification(message); err != nil {
			s.logger.Error("failed to handle cancellation notification", "error", err)
		}
		return nil, nil
	case "notifications/progress":
		// Handle progress notification
		if err := s.HandleProgressNotification(message); err != nil {
			s.logger.Error("failed to handle progress notification", "error", err)
		}
		return nil, nil
	case "notifications/message":
	case "notifications/resources/list_changed":
	case "notifications/resources/updated":
	case "notifications/tools/list_changed":
	case "notifications/prompts/list_changed":
	case "notifications/roots/list_changed":
		// Handle roots list changed notification by fetching updated roots from client
		s.fetchWorkspaceRoots()
		// Notifications don't need responses
		return nil, nil

	default:
		err = fmt.Errorf("method not found: %s", ctx.Request.Method)
	}

	// Handle errors
	if err != nil {
		// Emit event with actual request JSON and error
		go func() {
			events.Publish[events.RequestFailedEvent](s.events, events.TopicRequestFailed, events.RequestFailedEvent{
				Method:      ctx.Request.Method,
				RequestJSON: string(message),
				Error:       err.Error(),
			})
		}()

		// Determine the appropriate error code based on error type
		var errorCode int
		var errorMessage string

		// Check if this is an InvalidParametersError
		if _, ok := err.(*InvalidParametersError); ok {
			errorCode = -32602 // Invalid params
			errorMessage = "Invalid params"
		} else {
			errorCode = -32603 // Internal error
			errorMessage = "Internal error"
		}

		// Return error response
		return createErrorResponse(ctx.Request.ID, errorCode, errorMessage, err.Error()), nil
	}

	// Check if this is a notification (no ID)
	if ctx.Request.ID == nil {
		// Notifications don't return responses
		return nil, nil
	}

	// Create success response using structs
	response := mcp.NewSuccessResponse(ctx.Request.ID, result)
	responseBytes, err := response.Marshal()
	if err != nil {
		s.logger.Error("failed to marshal response", "error", err)
		errorResp := mcp.NewErrorResponse(ctx.Request.ID, -32603, "Internal error", "Failed to marshal response")
		errorBytes, _ := errorResp.Marshal()
		return errorBytes, nil
	}

	// Emit event with actual request and response JSON
	go func() {
		events.Publish[events.ToolExecutedEvent](s.events, events.TopicToolExecuted, events.ToolExecutedEvent{
			Method:       ctx.Request.Method,
			RequestJSON:  string(message),
			ResponseJSON: string(responseBytes),
		})
	}()

	return responseBytes, nil
}

// HandleMessageWithVersion handles a JSON-RPC message with a forced MCP version.
// This is primarily used for testing and allows processing messages with a
// specific protocol version regardless of what was negotiated during initialization.
// It provides a simplified subset of method handlers compared to the main HandleMessage function.
func HandleMessageWithVersion(srv Server, message []byte, version string) ([]byte, error) {
	serverImpl := srv.GetServer()
	if len(message) == 0 {
		return nil, errors.New("empty message")
	}

	// Parse the message to get the method
	var request Request
	if err := json.Unmarshal(message, &request); err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	// Create a context for the request
	ctx := &Context{
		Request: &request,
		Version: version, // Use the forced version
		server:  serverImpl,
	}

	// Extract resource path if this is a resource request
	if request.Method == "resources/read" && request.Params != nil {
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(request.Params, &params); err == nil && params.URI != "" {
			ctx.Request.ResourcePath = params.URI
		}
	}

	// Process the method
	var result interface{}
	var err error

	// Use the appropriate method handler based on the request method
	switch request.Method {
	case "ping":
		result = map[string]interface{}{} // Return empty object as specified in the protocol
	case "roots/list":
		// This is a client-side method, server should reject it
		return nil, fmt.Errorf("method not implemented: %s", request.Method)
	case "resources/list":
		result, err = serverImpl.ProcessResourceList(ctx)
	case "resources/read":
		result, err = serverImpl.ProcessResourceRequest(ctx)
	case "resources/templates/list":
		result, err = serverImpl.ProcessResourceTemplatesList(ctx)
	case "resources/subscribe":
		result, err = serverImpl.ProcessResourceSubscribe(ctx)
	case "resources/unsubscribe":
		result, err = serverImpl.ProcessResourceUnsubscribe(ctx)
	default:
		return nil, fmt.Errorf("method not found: %s", request.Method)
	}

	if err != nil {
		// Return the error using structs
		errorResp := mcp.NewErrorResponse(request.ID, -32603, err.Error(), nil)
		return errorResp.Marshal()
	}

	// Return the result using structs
	response := mcp.NewSuccessResponse(request.ID, result)
	return response.Marshal()
}
