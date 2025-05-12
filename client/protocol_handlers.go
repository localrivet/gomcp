package client

import (
	"encoding/json"
	"fmt"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
)

// Protocol handler factory
func newProtocolHandler(version string, logger logx.Logger) ProtocolHandler {
	switch version {
	case ProtocolVersion2024:
		return &protocol2024Handler{logger: logger}
	case ProtocolVersion2025:
		return &protocol2025Handler{logger: logger}
	default:
		// Default to latest
		return &protocol2025Handler{logger: logger}
	}
}

// protocol2024Handler implements the ProtocolHandler interface for the 2024-11-05 version
type protocol2024Handler struct {
	logger logx.Logger
}

func (h *protocol2024Handler) FormatRequest(method string, params interface{}) (*protocol.JSONRPCRequest, error) {
	id := generateID()
	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
	}

	// Set parameters directly if provided
	if params != nil {
		req.Params = params
	}

	return req, nil
}

func (h *protocol2024Handler) ParseResponse(resp *protocol.JSONRPCResponse) (interface{}, error) {
	// Handle error responses
	if resp.Error != nil {
		return nil, NewServerError("server", "", int(resp.Error.Code), resp.Error.Message, nil)
	}

	// Get the result as a raw JSON message
	rawResult, ok := resp.Result.(json.RawMessage)
	if !ok {
		// If it's not already a json.RawMessage, try to convert it
		var err error
		rawResult, err = json.Marshal(resp.Result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", err)
		}
	}

	// Try to unmarshal the result based on the structure
	// For initialize - check this FIRST
	if initResult, err := tryUnmarshalInitializeResult(rawResult); err == nil {
		return initResult, nil
	}

	// For tools
	if toolsResult, err := tryUnmarshalToolsResult(rawResult); err == nil {
		return toolsResult, nil
	}

	// For call tool
	if callToolResult, err := tryUnmarshalCallToolV2024Result(rawResult); err == nil {
		return callToolResult, nil
	}

	// For resources list
	if resourcesResult, err := tryUnmarshalResourcesResult(rawResult); err == nil {
		return resourcesResult, nil
	}

	// For resource read
	if readResourceResult, err := tryUnmarshalReadResourceResult(rawResult); err == nil {
		return readResourceResult, nil
	}

	// For prompts list
	if promptsResult, err := tryUnmarshalPromptsResult(rawResult); err == nil {
		return promptsResult, nil
	}

	// For prompt messages
	if promptMessages, err := tryUnmarshalPromptMessages(rawResult); err == nil {
		return promptMessages, nil
	}

	// Return the raw result if we can't determine the type
	return resp.Result, nil
}

// Helper functions to try unmarshaling different result types
func tryUnmarshalToolsResult(result json.RawMessage) (*protocol.ListToolsResult, error) {
	// First try to unmarshal as a ListToolsResult
	var toolsResult protocol.ListToolsResult
	err := json.Unmarshal(result, &toolsResult)
	if err == nil && len(toolsResult.Tools) > 0 {
		return &toolsResult, nil
	}

	// If that fails, try to unmarshal as a direct array of tools
	var tools []protocol.Tool
	err = json.Unmarshal(result, &tools)
	if err != nil || len(tools) == 0 {
		return nil, fmt.Errorf("not a tools result: %v", err)
	}

	// Create a ListToolsResult with the tools
	return &protocol.ListToolsResult{
		Tools: tools,
	}, nil
}

func tryUnmarshalCallToolV2024Result(result json.RawMessage) (*protocol.CallToolResultV2024, error) {
	var callResult protocol.CallToolResultV2024
	err := protocol.UnmarshalPayload(result, &callResult)
	if err != nil || len(callResult.Content) == 0 {
		return nil, fmt.Errorf("not a call tool v2024 result")
	}
	return &callResult, nil
}

func tryUnmarshalResourcesResult(result json.RawMessage) (*protocol.ListResourcesResult, error) {
	var resourcesResult protocol.ListResourcesResult
	err := protocol.UnmarshalPayload(result, &resourcesResult)
	if err != nil {
		return nil, fmt.Errorf("not a resources result")
	}
	return &resourcesResult, nil
}

func tryUnmarshalReadResourceResult(result json.RawMessage) (*protocol.ReadResourceResult, error) {
	var readResult protocol.ReadResourceResult
	err := protocol.UnmarshalPayload(result, &readResult)
	if err != nil || readResult.Resource.URI == "" {
		return nil, fmt.Errorf("not a read resource result")
	}
	return &readResult, nil
}

func tryUnmarshalPromptsResult(result json.RawMessage) (*protocol.ListPromptsResult, error) {
	var promptsResult protocol.ListPromptsResult
	err := protocol.UnmarshalPayload(result, &promptsResult)
	if err != nil {
		return nil, fmt.Errorf("not a prompts result")
	}
	return &promptsResult, nil
}

func tryUnmarshalPromptMessages(result json.RawMessage) ([]protocol.PromptMessage, error) {
	var messages []protocol.PromptMessage
	err := protocol.UnmarshalPayload(result, &messages)
	if err != nil || len(messages) == 0 {
		return nil, fmt.Errorf("not prompt messages")
	}
	return messages, nil
}

func tryUnmarshalInitializeResult(result json.RawMessage) (*protocol.InitializeResult, error) {
	// Debug message with more context
	// log.Printf("[Client] Attempting to unmarshal response as initialize result (part of type detection)")
	// Note: We can't use a logger here because we're not in a handler. This function is used for type detection.
	// and doesn't have access to a logger instance. These debug messages should be removed or handled differently.

	var initResult protocol.InitializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		// This is expected for non-initialize responses - not an error
		return nil, fmt.Errorf("not an initialize result: %w", err)
	}

	// Check if it has the required fields for an initialize result
	if initResult.ServerInfo.Name == "" ||
		initResult.ProtocolVersion == "" {
		// Not an error, just part of type detection
		return nil, fmt.Errorf("not an initialize result: missing required fields")
	}

	// Using a comment instead of log.Printf to avoid stdout pollution
	// log.Printf("[Client] Successfully identified and unmarshaled initialize result from %s (v%s)",
	//	initResult.ServerInfo.Name, initResult.ProtocolVersion)
	return &initResult, nil
}

func (h *protocol2024Handler) FormatCallToolRequest(name string, args map[string]interface{}) (interface{}, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}
	return params, nil
}

func (h *protocol2024Handler) ParseCallToolResult(result interface{}) ([]protocol.Content, error) {
	callResult, ok := result.(*protocol.CallToolResultV2024)
	if !ok {
		return nil, fmt.Errorf("invalid call tool result type")
	}
	return callResult.Content, nil
}

// protocol2025Handler implements the ProtocolHandler interface for the 2025-03-26 version
type protocol2025Handler struct {
	logger logx.Logger
}

func (h *protocol2025Handler) FormatRequest(method string, params interface{}) (*protocol.JSONRPCRequest, error) {
	id := generateID()
	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
	}

	// Set parameters directly if provided
	if params != nil {
		req.Params = params
	}

	return req, nil
}

func (h *protocol2025Handler) ParseResponse(resp *protocol.JSONRPCResponse) (interface{}, error) {
	// Handle error responses
	if resp.Error != nil {
		return nil, NewServerError("server", "", int(resp.Error.Code), resp.Error.Message, nil)
	}

	// Get the result as a raw JSON message
	rawResult, ok := resp.Result.(json.RawMessage)
	if !ok {
		// If it's not already a json.RawMessage, try to convert it
		var err error
		rawResult, err = json.Marshal(resp.Result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", err)
		}
	}

	// Try to unmarshal the result based on the structure
	// For initialize - check this FIRST
	if initResult, err := tryUnmarshalInitializeResult(rawResult); err == nil {
		return initResult, nil
	}

	// For tools
	if toolsResult, err := tryUnmarshalToolsResult(rawResult); err == nil {
		return toolsResult, nil
	}

	// For call tool
	if callToolResult, err := tryUnmarshalCallToolResult(rawResult); err == nil {
		return callToolResult, nil
	}

	// For resources list
	if resourcesResult, err := tryUnmarshalResourcesResult(rawResult); err == nil {
		return resourcesResult, nil
	}

	// For resource read
	if readResourceResult, err := tryUnmarshalReadResourceResult(rawResult); err == nil {
		return readResourceResult, nil
	}

	// For prompts list
	if promptsResult, err := tryUnmarshalPromptsResult(rawResult); err == nil {
		return promptsResult, nil
	}

	// For prompt messages
	if promptMessages, err := tryUnmarshalPromptMessages(rawResult); err == nil {
		return promptMessages, nil
	}

	// Return the raw result if we can't determine the type
	return resp.Result, nil
}

func tryUnmarshalCallToolResult(result json.RawMessage) (*protocol.CallToolResult, error) {
	var callResult protocol.CallToolResult
	err := protocol.UnmarshalPayload(result, &callResult)
	if err != nil || callResult.ToolCallID == "" {
		return nil, fmt.Errorf("not a call tool result")
	}
	return &callResult, nil
}

func (h *protocol2025Handler) FormatCallToolRequest(name string, args map[string]interface{}) (interface{}, error) {
	// Create the ToolCall structure
	toolCall := &protocol.ToolCall{
		ID:       generateID(),
		ToolName: name,
	}

	// Marshal arguments into Input field if provided
	if args != nil {
		input, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool arguments: %w", err)
		}
		toolCall.Input = input
	}

	// Create the request params
	params := protocol.CallToolRequestParams{
		ToolCall: toolCall,
	}

	return params, nil
}

func (h *protocol2025Handler) ParseCallToolResult(result interface{}) ([]protocol.Content, error) {
	callResult, ok := result.(*protocol.CallToolResult)
	if !ok {
		return nil, fmt.Errorf("invalid call tool result type")
	}

	// Check for errors
	if callResult.Error != nil {
		return nil, NewServerError("tool", callResult.ToolCallID, int(callResult.Error.Code), callResult.Error.Message, nil)
	}

	// Parse output
	var content []protocol.Content
	err := json.Unmarshal(callResult.Output, &content)
	if err != nil {
		// If unmarshaling fails as array, try as a simple string
		var simpleString string
		stringErr := json.Unmarshal(callResult.Output, &simpleString)
		if stringErr == nil {
			// If it's a JSON string, use the unwrapped value
			content = []protocol.Content{
				protocol.TextContent{
					Type: "text",
					Text: simpleString,
				},
			}
		} else {
			// As fallback, use as raw text
			content = []protocol.Content{
				protocol.TextContent{
					Type: "text",
					Text: string(callResult.Output),
				},
			}
		}
	}

	return content, nil
}
