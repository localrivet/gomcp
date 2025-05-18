package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
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
	case "draft", "2025-03-26":
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

// SamplingCreateMessageParams represents the parameters for a sampling/createMessage request.
type SamplingCreateMessageParams struct {
	Messages         []SamplingMessage        `json:"messages"`
	ModelPreferences SamplingModelPreferences `json:"modelPreferences"`
	SystemPrompt     string                   `json:"systemPrompt,omitempty"`
	MaxTokens        int                      `json:"maxTokens,omitempty"`
}

// SamplingResponse represents the response to a sampling/createMessage request.
type SamplingResponse struct {
	Role       string                 `json:"role"`
	Content    SamplingMessageContent `json:"content"`
	Model      string                 `json:"model,omitempty"`
	StopReason string                 `json:"stopReason,omitempty"`
}

// SamplingContentHandler is the interface for all sampling content handlers
type SamplingContentHandler interface {
	ToMessageContent() SamplingMessageContent
	Validate() error
}

// TextSamplingContent creates a text content struct for sampling messages
type TextSamplingContent struct {
	Text string
}

// ToMessageContent converts TextSamplingContent to a SamplingMessageContent
func (t *TextSamplingContent) ToMessageContent() SamplingMessageContent {
	return SamplingMessageContent{
		Type: "text",
		Text: t.Text,
	}
}

// Validate ensures the text content is valid
func (t *TextSamplingContent) Validate() error {
	if t.Text == "" {
		return fmt.Errorf("text content cannot be empty")
	}
	return nil
}

// ImageSamplingContent creates an image content struct for sampling messages
type ImageSamplingContent struct {
	Data     string
	MimeType string
}

// ToMessageContent converts ImageSamplingContent to a SamplingMessageContent
func (i *ImageSamplingContent) ToMessageContent() SamplingMessageContent {
	return SamplingMessageContent{
		Type:     "image",
		Data:     i.Data,
		MimeType: i.MimeType,
	}
}

// Validate ensures the image content is valid
func (i *ImageSamplingContent) Validate() error {
	if i.Data == "" {
		return fmt.Errorf("image data cannot be empty")
	}
	if i.MimeType == "" {
		return fmt.Errorf("image mime type cannot be empty")
	}
	return nil
}

// AudioSamplingContent creates an audio content struct for sampling messages
type AudioSamplingContent struct {
	Data     string
	MimeType string
}

// ToMessageContent converts AudioSamplingContent to a SamplingMessageContent
func (a *AudioSamplingContent) ToMessageContent() SamplingMessageContent {
	return SamplingMessageContent{
		Type:     "audio",
		Data:     a.Data,
		MimeType: a.MimeType,
	}
}

// Validate ensures the audio content is valid
func (a *AudioSamplingContent) Validate() error {
	if a.Data == "" {
		return fmt.Errorf("audio data cannot be empty")
	}
	if a.MimeType == "" {
		return fmt.Errorf("audio mime type cannot be empty")
	}
	return nil
}

// CreateSamplingMessage creates a sampling message with the provided content handler
func CreateSamplingMessage(role string, content SamplingContentHandler) (SamplingMessage, error) {
	if err := content.Validate(); err != nil {
		return SamplingMessage{}, fmt.Errorf("invalid content: %w", err)
	}

	return SamplingMessage{
		Role:    role,
		Content: content.ToMessageContent(),
	}, nil
}

// ValidateContentForVersion checks if a content handler is valid for the given protocol version
func ValidateContentForVersion(content SamplingContentHandler, version string) error {
	if content == nil {
		return fmt.Errorf("content cannot be nil")
	}

	// First validate the content itself
	if err := content.Validate(); err != nil {
		return err
	}

	// Then check if the content type is supported in this version
	msgContent := content.ToMessageContent()
	if !msgContent.IsValidForVersion(version) {
		return fmt.Errorf("content type '%s' not supported in protocol version '%s'",
			msgContent.Type, version)
	}

	return nil
}

// ProcessSamplingCreateMessage processes a sampling create message request from the client.
// This is typically a client-side method, so we just return an error.
func (s *serverImpl) ProcessSamplingCreateMessage(ctx *Context) (interface{}, error) {
	// This is a server->client request, so should not be called directly by clients
	return nil, fmt.Errorf("method not supported: %s", ctx.Request.Method)
}

// RequestSamplingOptions defines options for sampling requests
type RequestSamplingOptions struct {
	Timeout          time.Duration // Maximum time to wait for a response
	MaxRetries       int           // Maximum number of retry attempts on timeout
	RetryInterval    time.Duration // Time to wait between retries
	IgnoreCapability bool          // Whether to ignore client capability validation
	ForceSession     bool          // Whether to force using the specified session
}

// DefaultSamplingOptions returns the default options for sampling requests
func DefaultSamplingOptions() RequestSamplingOptions {
	return RequestSamplingOptions{
		Timeout:       30 * time.Second,
		MaxRetries:    0,
		RetryInterval: 1 * time.Second,
	}
}

// RequestSampling sends a sampling request to the client with the default options
func (s *serverImpl) RequestSampling(messages []SamplingMessage, preferences SamplingModelPreferences, systemPrompt string, maxTokens int) (*SamplingResponse, error) {
	return s.RequestSamplingWithOptions(messages, preferences, systemPrompt, maxTokens, DefaultSamplingOptions())
}

// requestTracker manages pending requests and correlates them with responses
type requestTracker struct {
	mu           sync.RWMutex
	requests     map[int]chan json.RawMessage
	timeouts     map[int]*time.Timer // Track timeout timers
	pendingCount int                 // Count of active pending requests
}

// newRequestTracker creates a new request tracker
func newRequestTracker() *requestTracker {
	return &requestTracker{
		requests: make(map[int]chan json.RawMessage),
		timeouts: make(map[int]*time.Timer),
	}
}

// addRequest adds a new request to track and returns a channel to receive the response
func (rt *requestTracker) addRequest(id int) chan json.RawMessage {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Create a buffered channel to prevent deadlock if response arrives after timeout
	responseChan := make(chan json.RawMessage, 1)
	rt.requests[id] = responseChan
	rt.pendingCount++

	return responseChan
}

// resolveRequest resolves a request with its response
// Returns true if the request was found and resolved
func (rt *requestTracker) resolveRequest(id int, response json.RawMessage) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	responseChan, exists := rt.requests[id]
	if !exists {
		return false
	}

	// Cancel any pending timeout for this request
	if timer, hasTimer := rt.timeouts[id]; hasTimer && timer != nil {
		timer.Stop()
		delete(rt.timeouts, id)
	}

	// Send the response on the channel (non-blocking to handle case where no one is listening)
	select {
	case responseChan <- response:
		// Response sent successfully
	default:
		// No one is listening, likely due to a timeout
	}

	// Clean up the request
	delete(rt.requests, id)
	rt.pendingCount--

	return true
}

// removeRequest removes a request from tracking without sending a response
// Used for cleanup after timeouts
func (rt *requestTracker) removeRequest(id int) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Cancel any pending timeout
	if timer, hasTimer := rt.timeouts[id]; hasTimer && timer != nil {
		timer.Stop()
		delete(rt.timeouts, id)
	}

	// Remove the request
	delete(rt.requests, id)
	rt.pendingCount--
}

// setupTimeout creates a timeout for a request
// When the timeout expires, the request will be automatically cleaned up
func (rt *requestTracker) setupTimeout(id int, timeout time.Duration) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Create a timer that will clean up the request when expired
	timer := time.AfterFunc(timeout, func() {
		rt.removeRequest(id)
	})

	rt.timeouts[id] = timer
}

// getPendingCount returns the number of currently pending requests
func (rt *requestTracker) getPendingCount() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.pendingCount
}

// GetSessionFromContext retrieves the client session associated with a context
// Returns the session and a boolean indicating if it was found
func (s *serverImpl) GetSessionFromContext(ctx *Context) (*ClientSession, bool) {
	// Check if context has session ID in metadata
	if ctx == nil || ctx.Metadata == nil {
		return nil, false
	}

	// Get the session ID from metadata
	sessionIDVal, ok := ctx.Metadata["sessionID"]
	if !ok {
		return nil, false
	}

	// Convert to string and then to SessionID type
	sessionIDStr, ok := sessionIDVal.(string)
	if !ok {
		return nil, false
	}

	// Retrieve session from session manager
	return s.sessionManager.GetSession(SessionID(sessionIDStr))
}

// GetClientCapabilitiesFromContext retrieves the sampling capabilities
// associated with the client session in the given context
func (s *serverImpl) GetClientCapabilitiesFromContext(ctx *Context) (SamplingCapabilities, bool) {
	// Get session from context
	session, ok := s.GetSessionFromContext(ctx)
	if !ok {
		// If no session found, use fallback based on protocol version
		caps := DetectClientCapabilities(ctx.Version)
		return caps, true
	}

	// Return session-specific capabilities
	return session.ClientInfo.SamplingCaps, true
}

// RequestSamplingFromContext initiates a sampling request based on the context's session information
func (s *serverImpl) RequestSamplingFromContext(ctx *Context, messages []SamplingMessage, preferences SamplingModelPreferences, systemPrompt string, maxTokens int) (*SamplingResponse, error) {
	if ctx == nil || ctx.Metadata == nil {
		// Use default session if no context is provided
		return s.RequestSampling(messages, preferences, systemPrompt, maxTokens)
	}

	// Try to get the session ID from the context
	sessionIDVal, ok := ctx.Metadata["sessionID"]
	if !ok {
		// No session ID in context, use default
		return s.RequestSampling(messages, preferences, systemPrompt, maxTokens)
	}

	// Convert the session ID to string
	sessionIDStr, ok := sessionIDVal.(string)
	if !ok {
		return nil, fmt.Errorf("invalid session ID in context")
	}

	// Use the context's protocol version
	protocolVersion := ctx.Version

	return s.RequestSamplingWithSessionAndOptions(SessionID(sessionIDStr), protocolVersion, messages, preferences, systemPrompt, maxTokens, DefaultSamplingOptions())
}

// RequestSamplingWithOptions sends a sampling request with custom options
func (s *serverImpl) RequestSamplingWithOptions(messages []SamplingMessage, preferences SamplingModelPreferences, systemPrompt string, maxTokens int, options RequestSamplingOptions) (*SamplingResponse, error) {
	// Get the current protocol version
	s.mu.RLock()
	protocolVersion := s.protocolVersion
	sessionID := SessionID("")
	if s.defaultSession != nil {
		sessionID = s.defaultSession.ID
	}
	s.mu.RUnlock()

	return s.RequestSamplingWithSessionAndOptions(sessionID, protocolVersion, messages, preferences, systemPrompt, maxTokens, options)
}

// RequestSamplingWithSessionAndOptions sends a sampling request to the client with a specific session
// and custom options for timeout and retry behavior
func (s *serverImpl) RequestSamplingWithSessionAndOptions(sessionID SessionID, protocolVersion string, messages []SamplingMessage, preferences SamplingModelPreferences, systemPrompt string, maxTokens int, options RequestSamplingOptions) (*SamplingResponse, error) {
	// Apply default options if not specified
	if options.Timeout == 0 {
		options.Timeout = 30 * time.Second // Default 30-second timeout
	}
	if options.MaxRetries < 0 {
		options.MaxRetries = 0 // Default no retries
	}
	if options.RetryInterval == 0 {
		options.RetryInterval = 1 * time.Second // Default 1-second retry interval
	}

	// Validate protocol version for this request
	if protocolVersion == "" {
		s.mu.RLock()
		protocolVersion = s.protocolVersion
		s.mu.RUnlock()
	}

	// Validate against protocol and server constraints using the sampling controller
	if s.samplingController != nil && !options.IgnoreCapability {
		if err := s.samplingController.ValidateForProtocol(protocolVersion, messages, maxTokens); err != nil {
			return nil, err
		}
	}

	// Get client capabilities for this session
	clientInfo, found := s.getClientInfoForSession(sessionID)
	if !found {
		return nil, fmt.Errorf("client session not found")
	}

	// Validate messages against client capabilities if not ignoring capability validation
	if !options.IgnoreCapability {
		for _, msg := range messages {
			switch msg.Content.Type {
			case "audio":
				if !clientInfo.SamplingCaps.AudioSupport {
					return nil, fmt.Errorf("client does not support audio content")
				}
			case "image":
				if !clientInfo.SamplingCaps.ImageSupport {
					return nil, fmt.Errorf("client does not support image content")
				}
			}

			// Also validate against protocol version
			if !msg.Content.IsValidForVersion(protocolVersion) {
				return nil, fmt.Errorf("content type '%s' not supported in protocol version '%s'",
					msg.Content.Type, protocolVersion)
			}
		}
	}

	// Create sampling parameters
	params := SamplingCreateMessageParams{
		Messages:         messages,
		ModelPreferences: preferences,
		SystemPrompt:     systemPrompt,
		MaxTokens:        maxTokens,
	}

	// Marshal the params
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sampling parameters: %w", err)
	}

	// Create request ID
	requestID := s.generateRequestID()

	// Create the JSON-RPC request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"method":  "sampling/createMessage",
		"params":  json.RawMessage(paramsJSON),
	}

	// Convert to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sampling request: %w", err)
	}

	// Create request tracker if not exists
	if s.requestTracker == nil {
		s.requestTracker = newRequestTracker()
	}

	// Register this request
	responseChan := s.requestTracker.addRequest(int(requestID))

	// Set up timeout handling using the enhanced request tracker
	s.requestTracker.setupTimeout(int(requestID), options.Timeout)

	// Log the request
	s.logger.Debug("sending sampling request",
		"id", requestID,
		"sessionID", string(sessionID),
		"timeout", options.Timeout.String(),
		"messageCount", len(messages),
		"maxTokens", maxTokens)

	// Send the request
	err = s.transport.Send(requestJSON)
	if err != nil {
		s.requestTracker.removeRequest(int(requestID))
		return nil, fmt.Errorf("failed to send sampling request: %w", err)
	}

	// Wait for response with timeout
	var responseJSON json.RawMessage
	var timedOut bool

	select {
	case responseJSON = <-responseChan:
		// Got a response
	case <-time.After(options.Timeout):
		timedOut = true
	}

	// Handle timeout with improved error handling and retry management
	if timedOut {
		// The request will be automatically cleaned up by the request tracker
		s.logger.Warn("timeout waiting for sampling response",
			"id", requestID,
			"sessionID", string(sessionID),
			"timeout", options.Timeout.String(),
			"retriesRemaining", options.MaxRetries)

		// Check if we should retry
		if options.MaxRetries > 0 {
			s.logger.Info("retrying sampling request",
				"id", requestID,
				"retriesLeft", options.MaxRetries)

			// Decrement max retries and retry with slightly increased timeout
			newOptions := options
			newOptions.MaxRetries--
			newOptions.Timeout += options.RetryInterval

			// Add a small delay before retrying
			time.Sleep(options.RetryInterval)

			return s.RequestSamplingWithSessionAndOptions(sessionID, protocolVersion, messages, preferences, systemPrompt, maxTokens, newOptions)
		}

		// Handle graceful degradation if enabled
		if s.samplingConfig != nil && s.samplingConfig.GracefulDegradation {
			s.logger.Info("attempting graceful degradation after timeout",
				"id", requestID,
				"sessionID", string(sessionID))

			// Generate a fallback response that indicates failure but provides a usable response
			fallbackResponse := &SamplingResponse{
				Role: "assistant",
				Content: SamplingMessageContent{
					Type: "text",
					Text: "I apologize, but I was unable to process your request in time. Please try again or rephrase your request.",
				},
				StopReason: "timeout",
			}

			return fallbackResponse, nil
		}

		return nil, fmt.Errorf("timeout waiting for sampling response")
	}

	// Parse the response
	var response struct {
		JSONRPC string            `json:"jsonrpc"`
		ID      json.RawMessage   `json:"id"`
		Result  *SamplingResponse `json:"result,omitempty"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    string `json:"data,omitempty"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return nil, fmt.Errorf("failed to parse sampling response: %w", err)
	}

	// Log response received
	s.logger.Debug("received sampling response",
		"id", requestID,
		"sessionID", string(sessionID),
		"hasError", response.Error != nil)

	// Check for error
	if response.Error != nil {
		// Check if this is a retryable error
		isRetryable := false

		// Network or temporary errors often have codes in 500+ range
		if response.Error.Code >= 500 && response.Error.Code < 600 {
			isRetryable = true
		}

		// Also look for specific error messages that indicate temporary issues
		errMsg := response.Error.Message
		if strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "rate limit") ||
			strings.Contains(errMsg, "temporarily unavailable") ||
			strings.Contains(errMsg, "try again") {
			isRetryable = true
		}

		// Retry if appropriate
		if isRetryable && options.MaxRetries > 0 {
			s.logger.Info("retrying after retryable error",
				"id", requestID,
				"errorCode", response.Error.Code,
				"errorMessage", response.Error.Message,
				"retriesLeft", options.MaxRetries)

			// Decrement max retries and retry with slightly increased timeout
			newOptions := options
			newOptions.MaxRetries--
			newOptions.Timeout += options.RetryInterval

			// Add a small delay before retrying
			time.Sleep(options.RetryInterval)

			return s.RequestSamplingWithSessionAndOptions(sessionID, protocolVersion, messages, preferences, systemPrompt, maxTokens, newOptions)
		}

		// Otherwise, return the error
		return nil, fmt.Errorf("sampling error: %s (code %d)", response.Error.Message, response.Error.Code)
	}

	// Ensure we have a valid result
	if response.Result == nil {
		return nil, fmt.Errorf("sampling response contains no result")
	}

	// Validate the response content type
	if !response.Result.Content.IsValidForVersion(protocolVersion) {
		return nil, fmt.Errorf("response content type '%s' not supported in protocol version '%s'",
			response.Result.Content.Type, protocolVersion)
	}

	return response.Result, nil
}

// CreateTextSamplingMessage creates a sampling message with text content.
func CreateTextSamplingMessage(role, text string) SamplingMessage {
	content := &TextSamplingContent{
		Text: text,
	}

	// We're ignoring validation errors here since this is a simplified helper
	msg, _ := CreateSamplingMessage(role, content)
	return msg
}

// CreateImageSamplingMessage creates a sampling message with image content.
func CreateImageSamplingMessage(role, imageData, mimeType string) SamplingMessage {
	content := &ImageSamplingContent{
		Data:     imageData,
		MimeType: mimeType,
	}

	// We're ignoring validation errors here since this is a simplified helper
	msg, _ := CreateSamplingMessage(role, content)
	return msg
}

// CreateAudioSamplingMessage creates a sampling message with audio content.
func CreateAudioSamplingMessage(role, audioData, mimeType string) SamplingMessage {
	content := &AudioSamplingContent{
		Data:     audioData,
		MimeType: mimeType,
	}

	// We're ignoring validation errors here since this is a simplified helper
	msg, _ := CreateSamplingMessage(role, content)
	return msg
}

// SamplingCapabilities defines what sampling features a client supports
type SamplingCapabilities struct {
	Supported    bool
	TextSupport  bool
	ImageSupport bool
	AudioSupport bool
}

// ClientInfo represents information about a connected client
type ClientInfo struct {
	SamplingSupported bool
	SamplingCaps      SamplingCapabilities
	ProtocolVersion   string
	// Add other client capabilities here
}

// getClientInfo returns information about the connected client
func (s *serverImpl) getClientInfo() (ClientInfo, bool) {
	// Use the default session if available
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.defaultSession != nil {
		return s.defaultSession.ClientInfo, true
	}

	// If no default session is available, return default client info
	return ClientInfo{
		SamplingSupported: true,
		SamplingCaps: SamplingCapabilities{
			Supported:    true,
			TextSupport:  true,
			ImageSupport: true,
			AudioSupport: s.protocolVersion != "2024-11-05", // Audio not supported in v20241105
		},
		ProtocolVersion: s.protocolVersion,
	}, true
}

// getClientInfoForSession returns client info for a specific session
func (s *serverImpl) getClientInfoForSession(sessionID SessionID) (ClientInfo, bool) {
	if sessionID == "" {
		return s.getClientInfo()
	}

	session, exists := s.sessionManager.GetSession(sessionID)
	if !exists {
		return s.getClientInfo()
	}

	return session.ClientInfo, true
}

// generateRequestID generates a unique request ID
func (s *serverImpl) generateRequestID() int64 {
	// Generate a unique request ID using atomic operations for thread safety
	s.mu.Lock()
	defer s.mu.Unlock()

	// Simple incrementing counter for request IDs
	// We store this as part of the server state
	if s.lastRequestID == 0 {
		s.lastRequestID = time.Now().UnixNano() // Initialize with current time for better uniqueness
	}

	s.lastRequestID++
	return s.lastRequestID
}

// HandleJSONRPCResponse processes a JSON-RPC response from the client
func (s *serverImpl) HandleJSONRPCResponse(responseJSON []byte) error {
	// Parse the response ID
	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
	}

	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return fmt.Errorf("failed to parse JSON-RPC response: %w", err)
	}

	// Parse the ID as an integer
	var id int
	if err := json.Unmarshal(response.ID, &id); err != nil {
		return fmt.Errorf("failed to parse response ID: %w", err)
	}

	// If we have a request tracker, resolve the request
	if s.requestTracker != nil {
		if !s.requestTracker.resolveRequest(id, responseJSON) {
			s.logger.Warn("received response for unknown request", "id", id)
		}
	}

	return nil
}
