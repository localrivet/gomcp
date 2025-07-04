// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/localrivet/gomcp/mcp"
)

// SamplingMessageContent represents the content of a sampling message.
type SamplingMessageContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// IsValidForVersion checks if the content type is valid for the given protocol version
func (c *SamplingMessageContent) IsValidForVersion(version string) bool {
	switch version {
	case "draft", "2025-03-26", "2025-06-18":
		// These versions support text, image, and audio content types
		return c.Type == "text" || c.Type == "image" || c.Type == "audio"
	case "2024-11-05":
		// This version only supports text and image content types
		return c.Type == "text" || c.Type == "image"
	default:
		// Unknown version, default to most restrictive
		return c.Type == "text"
	}
}

// SamplingMessage represents a message in a sampling conversation.
type SamplingMessage struct {
	Role    string                 `json:"role"`
	Content SamplingMessageContent `json:"content"`
}

// SamplingModelHint represents a hint for model selection in sampling requests.
type SamplingModelHint struct {
	Name string `json:"name"`
}

// SamplingModelPreferences represents the model preferences for a sampling request.
type SamplingModelPreferences struct {
	Hints                []SamplingModelHint `json:"hints,omitempty"`
	CostPriority         *float64            `json:"costPriority,omitempty"`
	SpeedPriority        *float64            `json:"speedPriority,omitempty"`
	IntelligencePriority *float64            `json:"intelligencePriority,omitempty"`
}

// SamplingOptions configures how sampling requests are made.
type SamplingOptions struct {
	// Core request parameters
	Messages         []SamplingMessage
	ModelPreferences SamplingModelPreferences
	SystemPrompt     string
	MaxTokens        int

	// Request configuration
	Context         context.Context
	Timeout         time.Duration
	ProtocolVersion string

	// Streaming options (only for protocol version 2025-03-26)
	Streaming      bool
	StreamHandler  func(*SamplingResponse) error // Called for each chunk when streaming
	ChunkSize      int                           // Maximum size of text chunks
	MaxChunks      int                           // Maximum number of chunks (0 for unlimited)
	StopOnComplete bool                          // Stop streaming when isComplete=true

	// Retry configuration
	MaxRetries      int
	RetryInterval   time.Duration
	RetryMultiplier float64
	MaxInterval     time.Duration
}

// SamplingResponse represents the response to a sampling request.
type SamplingResponse struct {
	Role       string                 `json:"role"`
	Content    SamplingMessageContent `json:"content"`
	Model      string                 `json:"model,omitempty"`
	StopReason string                 `json:"stopReason,omitempty"`

	// Streaming fields
	IsComplete bool `json:"isComplete,omitempty"` // Only for streaming responses
	ChunkIndex int  `json:"chunkIndex,omitempty"` // Only for streaming responses
}

// SamplingHandler is a function that handles sampling/createMessage requests from the server.
type SamplingHandler func(params SamplingCreateMessageParams) (SamplingResponse, error)

// SamplingCreateMessageParams represents the parameters for a sampling/createMessage request.
type SamplingCreateMessageParams struct {
	Messages         []SamplingMessage        `json:"messages"`
	ModelPreferences SamplingModelPreferences `json:"modelPreferences"`
	SystemPrompt     string                   `json:"systemPrompt,omitempty"`
	MaxTokens        int                      `json:"maxTokens,omitempty"`
}

// DefaultSamplingOptions returns default sampling options.
func DefaultSamplingOptions() *SamplingOptions {
	return &SamplingOptions{
		Timeout:         30 * time.Second,
		MaxRetries:      3,
		RetryInterval:   1 * time.Second,
		RetryMultiplier: 2.0,
		MaxInterval:     10 * time.Second,
		StopOnComplete:  true,
	}
}

// NewSamplingOptions creates new sampling options with the given messages and preferences.
func NewSamplingOptions(messages []SamplingMessage, prefs SamplingModelPreferences) *SamplingOptions {
	opts := DefaultSamplingOptions()
	opts.Messages = messages
	opts.ModelPreferences = prefs
	return opts
}

// WithContext sets the context for the sampling request.
func (opts *SamplingOptions) WithContext(ctx context.Context) *SamplingOptions {
	opts.Context = ctx
	return opts
}

// WithTimeout sets the timeout for the sampling request.
func (opts *SamplingOptions) WithTimeout(timeout time.Duration) *SamplingOptions {
	opts.Timeout = timeout
	return opts
}

// WithSystemPrompt sets the system prompt for the sampling request.
func (opts *SamplingOptions) WithSystemPrompt(prompt string) *SamplingOptions {
	opts.SystemPrompt = prompt
	return opts
}

// WithMaxTokens sets the maximum tokens for the sampling request.
func (opts *SamplingOptions) WithMaxTokens(maxTokens int) *SamplingOptions {
	opts.MaxTokens = maxTokens
	return opts
}

// WithStreaming enables streaming mode (only available in protocol version 2025-03-26).
func (opts *SamplingOptions) WithStreaming(handler func(*SamplingResponse) error) *SamplingOptions {
	opts.Streaming = true
	opts.StreamHandler = handler
	return opts
}

// WithChunkSize sets the chunk size for streaming (only for streaming mode).
func (opts *SamplingOptions) WithChunkSize(size int) *SamplingOptions {
	opts.ChunkSize = size
	return opts
}

// Validate validates the sampling options.
func (opts *SamplingOptions) Validate() error {
	if len(opts.Messages) == 0 {
		return fmt.Errorf("at least one message is required")
	}

	// Validate streaming options
	if opts.Streaming {
		if opts.ProtocolVersion != "2025-03-26" && opts.ProtocolVersion != "2025-06-18" {
			return fmt.Errorf("streaming is only supported in protocol versions 2025-03-26 and 2025-06-18")
		}
		if opts.StreamHandler == nil {
			return fmt.Errorf("stream handler is required for streaming mode")
		}
		if opts.ChunkSize > 0 && opts.ChunkSize < 10 {
			return fmt.Errorf("chunk size must be at least 10 characters")
		}
		if opts.ChunkSize > 1000 {
			return fmt.Errorf("chunk size cannot exceed 1000 characters")
		}
	}

	return nil
}

// RequestSampling sends a sampling request to the server.
// This is the single, unified method for all sampling operations.
func (c *clientImpl) RequestSampling(opts *SamplingOptions) (*SamplingResponse, error) {
	// Set defaults
	if opts.ProtocolVersion == "" {
		opts.ProtocolVersion = c.negotiatedVersion
	}
	if opts.Context == nil {
		var cancel context.CancelFunc
		opts.Context, cancel = context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()
	}

	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid sampling options: %w", err)
	}

	// Validate message content for protocol version
	for i, msg := range opts.Messages {
		if !msg.Content.IsValidForVersion(opts.ProtocolVersion) {
			return nil, fmt.Errorf("message %d content type '%s' not supported in protocol version '%s'",
				i, msg.Content.Type, opts.ProtocolVersion)
		}
	}

	// Build request
	requestID := c.generateRequestID()
	params := map[string]interface{}{
		"messages":         opts.Messages,
		"modelPreferences": opts.ModelPreferences,
	}
	request := mcp.NewRequest(requestID, "sampling/createMessage", params)

	// Add optional parameters
	if opts.SystemPrompt != "" {
		params["systemPrompt"] = opts.SystemPrompt
	}
	if opts.MaxTokens > 0 {
		params["maxTokens"] = opts.MaxTokens
	}

	// Add streaming parameters if enabled
	if opts.Streaming {
		streamParams := map[string]interface{}{}
		if opts.ChunkSize > 0 {
			streamParams["chunkSize"] = opts.ChunkSize
		}
		if opts.MaxChunks > 0 {
			streamParams["maxChunks"] = opts.MaxChunks
		}
		streamParams["stopOnComplete"] = opts.StopOnComplete
		params["streaming"] = streamParams
	}

	// Update request with final params
	request.Params = params

	// Marshal request
	requestJSON, err := request.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request with retry logic
	if opts.Streaming {
		return c.sendStreamingSamplingRequest(opts, requestJSON)
	}
	return c.sendRegularSamplingRequest(opts, requestJSON)
}

// sendRegularSamplingRequest sends a regular (non-streaming) sampling request.
func (c *clientImpl) sendRegularSamplingRequest(opts *SamplingOptions, requestJSON []byte) (*SamplingResponse, error) {
	var responseJSON []byte
	var err error

	// Retry logic
	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		if attempt > 0 {
			retryInterval := time.Duration(float64(opts.RetryInterval) * float64(attempt) * opts.RetryMultiplier)
			if retryInterval > opts.MaxInterval {
				retryInterval = opts.MaxInterval
			}

			select {
			case <-opts.Context.Done():
				return nil, opts.Context.Err()
			case <-time.After(retryInterval):
			}
		}

		responseJSON, err = c.transport.SendWithContext(opts.Context, requestJSON)
		if err == nil || !c.isRetryableError(err) {
			break
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to send sampling request: %w", err)
	}

	// Parse response
	var jsonResponse struct {
		JSONRPC string            `json:"jsonrpc"`
		ID      int64             `json:"id"`
		Result  *SamplingResponse `json:"result,omitempty"`
		Error   *struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(responseJSON, &jsonResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if jsonResponse.Error != nil {
		return nil, fmt.Errorf("server error: %s (code %d)", jsonResponse.Error.Message, jsonResponse.Error.Code)
	}

	if jsonResponse.Result == nil {
		return nil, fmt.Errorf("no result in response")
	}

	// Validate response content
	if !jsonResponse.Result.Content.IsValidForVersion(opts.ProtocolVersion) {
		return nil, fmt.Errorf("response content type '%s' not supported in protocol version '%s'",
			jsonResponse.Result.Content.Type, opts.ProtocolVersion)
	}

	return jsonResponse.Result, nil
}

// sendStreamingSamplingRequest sends a streaming sampling request.
func (c *clientImpl) sendStreamingSamplingRequest(opts *SamplingOptions, requestJSON []byte) (*SamplingResponse, error) {
	// For now, return an error since streaming requires transport-level support
	// In a real implementation, this would use WebSockets or SSE
	return nil, fmt.Errorf("streaming sampling not yet implemented - requires transport-level streaming support")
}

// isRetryableError determines if an error should trigger a retry.
func (c *clientImpl) isRetryableError(err error) bool {
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	errStr := err.Error()
	return containsSubstring(errStr, "timeout") ||
		containsSubstring(errStr, "temporary") ||
		containsSubstring(errStr, "connection") ||
		containsSubstring(errStr, "reset") ||
		containsSubstring(errStr, "broken pipe")
}

// containsSubstring checks if a string contains a substring.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr
}

// WithSamplingHandler sets the client's sampling handler for incoming requests.
func (c *clientImpl) WithSamplingHandler(handler SamplingHandler) Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.samplingHandler = handler
	c.capabilities.Sampling = map[string]interface{}{}
	return c
}

// GetSamplingHandler returns the client's sampling handler.
func (c *clientImpl) GetSamplingHandler() SamplingHandler {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.samplingHandler
}

// handleSamplingCreateMessage handles incoming sampling requests from the server.
func (c *clientImpl) handleSamplingCreateMessage(id int64, paramsJSON []byte) error {
	var params SamplingCreateMessageParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return c.sendJsonRpcErrorResponse(id, -32700, "Parse error", err.Error())
	}

	// Validate content types
	for _, msg := range params.Messages {
		if !msg.Content.IsValidForVersion(c.negotiatedVersion) {
			errMsg := fmt.Sprintf("Content type '%s' not supported in protocol version '%s'",
				msg.Content.Type, c.negotiatedVersion)
			return c.sendJsonRpcErrorResponse(id, -32600, "Invalid Request", errMsg)
		}
	}

	// Get handler
	handler := c.GetSamplingHandler()
	if handler == nil {
		return c.sendJsonRpcErrorResponse(id, -1, "User rejected sampling request", "No sampling handler registered")
	}

	// Call handler
	response, err := handler(params)
	if err != nil {
		return c.sendJsonRpcErrorResponse(id, -1, "Sampling error", err.Error())
	}

	// Validate response
	if !response.Content.IsValidForVersion(c.negotiatedVersion) {
		errMsg := fmt.Errorf("Response content type '%s' not supported in protocol version '%s'",
			response.Content.Type, c.negotiatedVersion)
		return c.sendJsonRpcErrorResponse(id, -32600, "Invalid Response", errMsg.Error())
	}

	return c.sendJsonRpcSuccessResponse(id, response)
}

// Helper functions for JSON-RPC responses
func (c *clientImpl) sendJsonRpcErrorResponse(id int64, code int, message, data string) error {
	var dataInterface interface{}
	if data != "" {
		dataInterface = data
	}

	response := mcp.NewErrorResponse(id, code, message, dataInterface)
	responseJSON, err := response.Marshal()
	if err != nil {
		return err
	}
	_, err = c.transport.Send(responseJSON)
	return err
}

func (c *clientImpl) sendJsonRpcSuccessResponse(id int64, result interface{}) error {
	response := mcp.NewSuccessResponse(id, result)
	responseJSON, err := response.Marshal()
	if err != nil {
		return err
	}
	_, err = c.transport.Send(responseJSON)
	return err
}

// Convenience functions for creating messages
func CreateTextMessage(role, text string) SamplingMessage {
	// Validate the role
	if role != "user" && role != "assistant" {
		// Log a warning but create the message anyway to avoid breaking existing code
		// In a future version, this could be changed to return an error
		// Use slog.Default() instead of fmt.Printf to avoid polluting stdout in stdio transport
		slog.Default().Warn("sampling messages should only use 'user' and 'assistant' roles",
			"role", role,
			"suggestion", "use systemPrompt parameter for system instructions")
	}

	return SamplingMessage{
		Role: role,
		Content: SamplingMessageContent{
			Type: "text",
			Text: text,
		},
	}
}

func CreateImageMessage(role, imageData, mimeType string) SamplingMessage {
	// Validate the role
	if role != "user" && role != "assistant" {
		// Log a warning but create the message anyway to avoid breaking existing code
		// Use slog.Default() instead of fmt.Printf to avoid polluting stdout in stdio transport
		slog.Default().Warn("sampling messages should only use 'user' and 'assistant' roles",
			"role", role,
			"suggestion", "use systemPrompt parameter for system instructions")
	}

	return SamplingMessage{
		Role: role,
		Content: SamplingMessageContent{
			Type:     "image",
			Data:     imageData,
			MimeType: mimeType,
		},
	}
}

func CreateAudioMessage(role, audioData, mimeType string) SamplingMessage {
	// Validate the role
	if role != "user" && role != "assistant" {
		// Log a warning but create the message anyway to avoid breaking existing code
		// Use slog.Default() instead of fmt.Printf to avoid polluting stdout in stdio transport
		slog.Default().Warn("sampling messages should only use 'user' and 'assistant' roles",
			"role", role,
			"suggestion", "use systemPrompt parameter for system instructions")
	}

	return SamplingMessage{
		Role: role,
		Content: SamplingMessageContent{
			Type:     "audio",
			Data:     audioData,
			MimeType: mimeType,
		},
	}
}
