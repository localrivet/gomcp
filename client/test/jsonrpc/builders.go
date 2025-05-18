package jsonrpc

// This file contains constructor functions for JSON-RPC messages
// to provide type safety and documentation in test code

// NewToolRequest creates a tool request with the given name and arguments
func NewToolRequest(id interface{}, name string, args map[string]interface{}) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Method:  "tool/execute",
		Params: ToolParams{
			Name:      name,
			Arguments: args,
		},
	}
}

// NewResourceRequest creates a resource request for the given path
func NewResourceRequest(id interface{}, path string) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Method:  "resource/get",
		Params: ResourceParams{
			Path: path,
		},
	}
}

// NewResourceRequestWithOptions creates a resource request with additional options
func NewResourceRequestWithOptions(id interface{}, path string, options map[string]interface{}) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Method:  "resource/get",
		Params: ResourceParams{
			Path:    path,
			Options: options,
		},
	}
}

// NewPromptRequest creates a prompt request with the given name and variables
func NewPromptRequest(id interface{}, name string, variables map[string]interface{}) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Method:  "prompt/get",
		Params: PromptParams{
			Name:      name,
			Variables: variables,
		},
	}
}

// NewRootListRequest creates a request to list roots
func NewRootListRequest(id interface{}) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Method:  "roots/list",
	}
}

// NewRootAddRequest creates a request to add a root
func NewRootAddRequest(id interface{}, path, name string) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Method:  "roots/add",
		Params: RootAddParams{
			Path: path,
			Name: name,
		},
	}
}

// NewRootRemoveRequest creates a request to remove a root
func NewRootRemoveRequest(id interface{}, path string) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Method:  "roots/remove",
		Params: RootRemoveParams{
			Path: path,
		},
	}
}

// NewInitializeRequest creates an initialize request
func NewInitializeRequest(id interface{}, clientName, clientVersion string, supportedVersions []string) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Method:  "initialize",
		Params: InitializeParams{
			ClientInfo: ClientInfoObj{
				Name:    clientName,
				Version: clientVersion,
			},
			Versions: supportedVersions,
		},
	}
}

// NewToolResponse creates a response for a tool execution
func NewToolResponse(id interface{}, output interface{}, metadata map[string]interface{}) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Result: ToolResult{
			Output:   output,
			Metadata: metadata,
		},
	}
}

// NewPromptResponse creates a response for a prompt
func NewPromptResponse(id interface{}, prompt, rendered string, metadata map[string]interface{}) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Result: PromptResult{
			Prompt:   prompt,
			Rendered: rendered,
			Metadata: metadata,
		},
	}
}

// NewRootsListResponse creates a response for a roots/list request
func NewRootsListResponse(id interface{}, roots []RootItem) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Result: RootsListResult{
			Roots: roots,
		},
	}
}

// NewErrorResponse creates an error response
func NewErrorResponse(id interface{}, code int, message string, data interface{}) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// NewEmptyResponse creates an empty success response
func NewEmptyResponse(id interface{}) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Result:  map[string]interface{}{},
	}
}

// NewNotificationRequest creates a notification (no ID) with the given method and params
func NewNotificationRequest(method string, params interface{}) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		Method:  method,
		Params:  params,
	}
}

// NewSamplingCreateMessageParams creates parameters for a sampling/createMessage request
func NewSamplingCreateMessageParams(messages []SamplingMessage, modelPreferences SamplingModelPreferences) SamplingCreateMessageParams {
	return SamplingCreateMessageParams{
		Messages:         messages,
		ModelPreferences: modelPreferences,
	}
}

// NewSamplingCreateMessageRequest creates a sampling/createMessage request
func NewSamplingCreateMessageRequest(id interface{}, params SamplingCreateMessageParams) *JSONRPC {
	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Method:  "sampling/createMessage",
		Params:  params,
	}
}

// NewSamplingResponse creates a response for a sampling/createMessage request
func NewSamplingResponse(id interface{}, role string, content SamplingMessageContent, model string, stopReason string) *JSONRPC {
	response := SamplingResponse{
		Role:    role,
		Content: content,
	}

	// Optional fields
	if model != "" {
		response.Model = model
	}
	if stopReason != "" {
		response.StopReason = stopReason
	}

	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Result:  response,
	}
}

// NewSamplingTextResponse creates a text response for a sampling/createMessage request
func NewSamplingTextResponse(id interface{}, text string, model string, stopReason string) *JSONRPC {
	content := SamplingMessageContent{
		Type: "text",
		Text: text,
	}
	return NewSamplingResponse(id, "assistant", content, model, stopReason)
}

// NewSamplingImageResponse creates an image response for a sampling/createMessage request
func NewSamplingImageResponse(id interface{}, data string, mimeType string, model string, stopReason string) *JSONRPC {
	content := SamplingMessageContent{
		Type:     "image",
		Data:     data,
		MimeType: mimeType,
	}
	return NewSamplingResponse(id, "assistant", content, model, stopReason)
}

// NewSamplingAudioResponse creates an audio response for a sampling/createMessage request
func NewSamplingAudioResponse(id interface{}, data string, mimeType string, model string, stopReason string) *JSONRPC {
	content := SamplingMessageContent{
		Type:     "audio",
		Data:     data,
		MimeType: mimeType,
	}
	return NewSamplingResponse(id, "assistant", content, model, stopReason)
}

// NewSamplingStreamingResponse creates a streaming response for a sampling/createStreamingMessage request
func NewSamplingStreamingResponse(id interface{}, chunk *SamplingStreamingChunk, completion *SamplingStreamingCompletion) *JSONRPC {
	streamingResponse := SamplingStreamingResponse{
		Chunk:      chunk,
		Completion: completion,
	}

	return &JSONRPC{
		Version: "2.0",
		ID:      id,
		Result:  streamingResponse,
	}
}

// NewSamplingStreamingChunkResponse creates a streaming chunk response
func NewSamplingStreamingChunkResponse(id interface{}, chunkID string, role string, content SamplingMessageContent) *JSONRPC {
	chunk := &SamplingStreamingChunk{
		ChunkID: chunkID,
		Role:    role,
		Content: content,
	}
	return NewSamplingStreamingResponse(id, chunk, nil)
}

// NewSamplingStreamingCompletionResponse creates a streaming completion response
func NewSamplingStreamingCompletionResponse(id interface{}, model string, stopReason string) *JSONRPC {
	completion := &SamplingStreamingCompletion{
		Model:      model,
		StopReason: stopReason,
	}
	return NewSamplingStreamingResponse(id, nil, completion)
}

// NewSamplingStreamingTextChunk creates a streaming text chunk response
func NewSamplingStreamingTextChunk(id interface{}, chunkID string, text string) *JSONRPC {
	content := SamplingMessageContent{
		Type: "text",
		Text: text,
	}
	return NewSamplingStreamingChunkResponse(id, chunkID, "assistant", content)
}
