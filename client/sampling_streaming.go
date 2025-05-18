// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// StreamingSamplingRequest represents a request for streaming sampling operations
type StreamingSamplingRequest struct {
	// Base sampling request
	SamplingRequest

	// Streaming-specific options
	ChunkSize      int  // Maximum size of text chunks (applicable for text streaming)
	MaxChunks      int  // Maximum number of chunks to receive (0 for unlimited)
	StopOnComplete bool // Whether to stop streaming when isComplete=true is received
}

// NewStreamingSamplingRequest creates a new streaming sampling request
func NewStreamingSamplingRequest(messages []SamplingMessage, prefs SamplingModelPreferences) *StreamingSamplingRequest {
	return &StreamingSamplingRequest{
		SamplingRequest: *NewSamplingRequest(messages, prefs),
		ChunkSize:       100,  // Default chunk size
		MaxChunks:       0,    // Default to unlimited
		StopOnComplete:  true, // Default to stop on complete
	}
}

// WithChunkSize sets the maximum chunk size for text streaming
func (req *StreamingSamplingRequest) WithChunkSize(size int) *StreamingSamplingRequest {
	req.ChunkSize = size
	return req
}

// WithMaxChunks sets the maximum number of chunks to receive
func (req *StreamingSamplingRequest) WithMaxChunks(maxChunks int) *StreamingSamplingRequest {
	req.MaxChunks = maxChunks
	return req
}

// WithStopOnComplete sets whether to stop streaming when isComplete=true is received
func (req *StreamingSamplingRequest) WithStopOnComplete(stop bool) *StreamingSamplingRequest {
	req.StopOnComplete = stop
	return req
}

// BuildStreamingCreateMessageRequest builds a JSON-RPC request for streaming sampling
func (req *StreamingSamplingRequest) BuildStreamingCreateMessageRequest(id int) ([]byte, error) {
	// Create the parameters object
	params := SamplingCreateMessageParams{
		Messages:         req.Messages,
		ModelPreferences: req.ModelPreferences,
		SystemPrompt:     req.SystemPrompt,
		MaxTokens:        req.MaxTokens,
	}

	// Create streaming-specific parameters
	// For protocol versions that support these options
	paramsMap := map[string]interface{}{
		"messages":         params.Messages,
		"modelPreferences": params.ModelPreferences,
		"systemPrompt":     params.SystemPrompt,
		"maxTokens":        params.MaxTokens,
		"streaming":        true,
	}

	// Add streaming-specific options
	if req.ChunkSize > 0 {
		paramsMap["chunkSize"] = req.ChunkSize
	}

	// Add protocol version if specified
	if req.ProtocolVersion != "" {
		paramsMap["protocolVersion"] = req.ProtocolVersion
	}

	// Create the request object
	var request map[string]interface{}

	if req.FormatAsNotification {
		// Format as a notification (no id)
		request = map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "sampling/createMessage",
			"params":  paramsMap,
		}
	} else {
		// Format as a request with id
		request = map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  "sampling/createMessage",
			"params":  paramsMap,
		}
	}

	// Marshal to JSON
	return json.Marshal(request)
}

// StreamingSamplingSession represents an active streaming sampling session
type StreamingSamplingSession struct {
	ctx          context.Context
	cancel       context.CancelFunc
	client       *clientImpl
	handler      StreamingResponseHandler
	responses    []*StreamingSamplingResponse
	responsesMu  sync.RWMutex
	request      *StreamingSamplingRequest
	complete     bool
	chunkCount   int
	combinedResp *SamplingResponse
}

// Stop stops the streaming session
func (s *StreamingSamplingSession) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// IsComplete returns whether the streaming session is complete
func (s *StreamingSamplingSession) IsComplete() bool {
	s.responsesMu.RLock()
	defer s.responsesMu.RUnlock()
	return s.complete
}

// GetResponses returns all received responses
func (s *StreamingSamplingSession) GetResponses() []*StreamingSamplingResponse {
	s.responsesMu.RLock()
	defer s.responsesMu.RUnlock()

	// Return a copy to avoid race conditions
	responses := make([]*StreamingSamplingResponse, len(s.responses))
	copy(responses, s.responses)
	return responses
}

// GetCombinedResponse returns a combined response from all chunks
func (s *StreamingSamplingSession) GetCombinedResponse() *SamplingResponse {
	s.responsesMu.RLock()
	defer s.responsesMu.RUnlock()

	// If already combined, return it
	if s.combinedResp != nil {
		return s.combinedResp
	}

	// If no responses, return nil
	if len(s.responses) == 0 {
		return nil
	}

	// Create a combined response
	var combinedText string
	var lastResp *StreamingSamplingResponse

	for _, resp := range s.responses {
		if resp.Content.Type == "text" {
			combinedText += resp.Content.Text
		}
		lastResp = resp
	}

	// Use the last response as a template
	combined := &SamplingResponse{
		Role: lastResp.Role,
		Content: SamplingMessageContent{
			Type: "text",
			Text: combinedText,
		},
		Model:      lastResp.Model,
		StopReason: lastResp.StopReason,
	}

	return combined
}

// handleChunk processes a streaming response chunk
func (s *StreamingSamplingSession) handleChunk(chunk *StreamingSamplingResponse) error {
	s.responsesMu.Lock()
	defer s.responsesMu.Unlock()

	// Add the chunk to our responses
	s.responses = append(s.responses, chunk)
	s.chunkCount++

	// Check if we should mark the session as complete
	if (chunk.IsComplete && s.request.StopOnComplete) ||
		(s.request.MaxChunks > 0 && s.chunkCount >= s.request.MaxChunks) {
		s.complete = true
	}

	// Call the handler if provided
	if s.handler != nil {
		if err := s.handler(chunk); err != nil {
			return err
		}
	}

	// If complete, finalize the combined response
	if s.complete {
		s.combinedResp = &SamplingResponse{
			Role: chunk.Role,
			Content: SamplingMessageContent{
				Type: chunk.Content.Type,
				Text: "",
			},
			Model:      chunk.Model,
			StopReason: chunk.StopReason,
		}

		// Combine all text content
		if chunk.Content.Type == "text" {
			for _, resp := range s.responses {
				if resp.Content.Type == "text" {
					s.combinedResp.Content.Text += resp.Content.Text
				}
			}
		} else {
			// For non-text content, use the last chunk
			s.combinedResp.Content = chunk.Content
		}
	}

	return nil
}

// RequestStreamingSampling sends a streaming sampling request to the server
func (c *clientImpl) RequestStreamingSampling(
	req *StreamingSamplingRequest,
	handler StreamingResponseHandler,
) (*StreamingSamplingSession, error) {
	// Set protocol version if not already set
	if req.ProtocolVersion == "" {
		req.ProtocolVersion = c.negotiatedVersion
	}

	// Check if streaming is supported in this version
	if !isStreamingSupportedForVersion(req.ProtocolVersion) {
		return nil, NewSamplingError(
			"validation",
			fmt.Sprintf("streaming not supported in protocol version %s", req.ProtocolVersion),
			false,
			nil,
		)
	}

	// Validate the request
	if err := req.Validate(); err != nil {
		return nil, NewSamplingError("validation", "invalid streaming request", false, err)
	}

	// Validate protocol-specific constraints
	if req.ProtocolVersion == "2025-03-26" {
		// Validate chunk size for 2025-03-26
		if req.ChunkSize > 0 && req.ChunkSize < 10 {
			return nil, NewSamplingError(
				"validation",
				"chunkSize must be at least 10 characters for protocol version 2025-03-26",
				false,
				nil,
			)
		}
		if req.ChunkSize > 1000 {
			return nil, NewSamplingError(
				"validation",
				"chunkSize exceeds maximum value of 1000 for protocol version 2025-03-26",
				false,
				nil,
			)
		}
	}

	// Create a context for the streaming session
	var ctx context.Context
	var cancel context.CancelFunc

	if req.Context != nil {
		// Derive from the request context if provided
		ctx, cancel = context.WithCancel(req.Context)
	} else {
		// Create a new context with timeout
		ctx, cancel = context.WithTimeout(context.Background(), req.Timeout)
	}

	// Create the streaming session
	session := &StreamingSamplingSession{
		ctx:        ctx,
		cancel:     cancel,
		client:     c,
		handler:    handler,
		responses:  make([]*StreamingSamplingResponse, 0),
		request:    req,
		complete:   false,
		chunkCount: 0,
	}

	// Build the request
	requestID := c.generateRequestID()
	requestJSON, err := req.BuildStreamingCreateMessageRequest(int(requestID))
	if err != nil {
		cancel() // Clean up
		return nil, NewSamplingError("request", "failed to build streaming request", false, err)
	}

	// Start a goroutine to handle the streaming responses
	go func() {
		defer cancel() // Ensure context is canceled when done

		// Track retries
		var retryCount int
		var retryInterval time.Duration = req.RetryConfig.RetryInterval

		for retryCount <= req.RetryConfig.MaxRetries {
			// Check if the context is still valid
			if ctx.Err() != nil {
				c.logger.Debug("streaming context canceled or timed out", "error", ctx.Err())
				return
			}

			// If this is a retry, wait before retrying
			if retryCount > 0 {
				c.logger.Debug("retrying streaming request", "attempt", retryCount+1, "retryInterval", retryInterval)
				select {
				case <-ctx.Done():
					return
				case <-time.After(retryInterval):
					// Update retry interval for next attempt
					retryInterval = time.Duration(float64(retryInterval) * req.RetryConfig.RetryMultiplier)
					if retryInterval > req.RetryConfig.MaxInterval {
						retryInterval = req.RetryConfig.MaxInterval
					}
				}
			}

			// Send the request
			responseJSON, sendErr := c.transport.SendWithContext(ctx, requestJSON)

			// Handle send errors
			if sendErr != nil {
				if !isRetryableError(sendErr) || retryCount >= req.RetryConfig.MaxRetries {
					c.logger.Error("streaming request failed", "error", sendErr, "retries", retryCount)
					return
				}

				// Prepare for retry
				retryCount++
				continue
			}

			// Parse the response
			chunk, parseErr := parseStreamingSamplingResponse(responseJSON)
			if parseErr != nil {
				// Handle parse errors
				c.logger.Error("failed to parse streaming response", "error", parseErr)

				// Check if we should retry
				if samplingErr, ok := parseErr.(*SamplingError); ok && samplingErr.IsRetryable() && retryCount < req.RetryConfig.MaxRetries {
					retryCount++
					continue
				}

				return
			}

			// Validate chunk against protocol version
			validateErr := validateStreamingSamplingResponseForVersion(chunk, req.ProtocolVersion)
			if validateErr != nil {
				c.logger.Error("invalid streaming response for protocol version",
					"version", req.ProtocolVersion,
					"error", validateErr)
				return
			}

			// Process the chunk
			if err := session.handleChunk(chunk); err != nil {
				c.logger.Error("failed to handle chunk", "error", err)
				return
			}

			// Check if the session is complete
			if session.IsComplete() {
				return
			}

			// For subsequent chunks - the exact mechanism depends on the transport
			// In a real implementation, we'd set up a stream using SSE or WebSockets
			// This is just a placeholder that would be replaced with actual transport-specific code

			// Reset retry count after successful processing
			retryCount = 0

			// In a real implementation, we'd either:
			// 1. Wait for the next message from a stream (WebSockets/SSE case)
			// 2. Make a new request with a "continuationToken" (poll case)

			// For simplicity in this example, we'll just exit after processing one chunk
			// In a real implementation, this would be replaced with proper streaming logic
			c.logger.Debug("streaming placeholder - would continue receiving chunks in real implementation")
			return
		}
	}()

	return session, nil
}

// isStreamingSupportedForVersion checks if streaming is supported in a given protocol version
func isStreamingSupportedForVersion(version string) bool {
	switch version {
	case "2025-03-26":
		return true // Only the latest version supports streaming
	case "2024-11-05", "draft", "":
		return false // Older versions don't support streaming
	default:
		return false // Unknown versions don't support streaming
	}
}
