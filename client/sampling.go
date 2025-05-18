// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"encoding/json"
	"fmt"
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
	ProtocolVersion  string                   `json:"-"` // Internal field for version tracking
}

// SamplingResponse represents the response to a sampling/createMessage request.
type SamplingResponse struct {
	Role       string                 `json:"role"`
	Content    SamplingMessageContent `json:"content"`
	Model      string                 `json:"model,omitempty"`
	StopReason string                 `json:"stopReason,omitempty"`
}

// Note: SamplingRequest is defined in sampling_request.go

// Note: StreamingSamplingRequest is defined in sampling_streaming.go

// SamplingChunk represents a chunk of a streaming response.
type SamplingChunk struct {
	Content SamplingMessageContent `json:"content"`
}

// SamplingCompletion represents the completion of a streaming response.
type SamplingCompletion struct {
	Model      string `json:"model,omitempty"`
	StopReason string `json:"stopReason,omitempty"`
}

// SamplingHandler is a function that handles sampling/createMessage requests.
type SamplingHandler func(params SamplingCreateMessageParams) (SamplingResponse, error)

// WithSamplingHandler sets the client's sampling handler
func (c *clientImpl) WithSamplingHandler(handler SamplingHandler) Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.samplingHandler = handler

	// Add sampling capability if not already present
	c.capabilities.Sampling = map[string]interface{}{}

	return c
}

// GetSamplingHandler returns the client's sampling handler
func (c *clientImpl) GetSamplingHandler() SamplingHandler {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.samplingHandler
}

// handleSamplingCreateMessage handles a sampling/createMessage request from the server
func (c *clientImpl) handleSamplingCreateMessage(id int64, paramsJSON []byte) error {
	c.logger.Debug("received sampling/createMessage request", "id", id)

	// Parse the parameters
	var params SamplingCreateMessageParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		c.logger.Error("failed to parse sampling/createMessage request", "error", err)
		return c.sendJsonRpcErrorResponse(id, -32700, "Parse error", err.Error())
	}

	// Validate content types against protocol version
	for _, msg := range params.Messages {
		if !msg.Content.IsValidForVersion(c.negotiatedVersion) {
			errMsg := fmt.Sprintf("Content type '%s' not supported in protocol version '%s'",
				msg.Content.Type, c.negotiatedVersion)
			c.logger.Error("invalid content type", "type", msg.Content.Type, "version", c.negotiatedVersion)
			return c.sendJsonRpcErrorResponse(id, -32600, "Invalid Request", errMsg)
		}
	}

	// Check if we have a handler
	handler := c.GetSamplingHandler()
	if handler == nil {
		c.logger.Info("no sampling handler registered, rejecting request")
		return c.sendJsonRpcErrorResponse(id, -1, "User rejected sampling request", "No sampling handler registered")
	}

	// Call the handler
	response, err := handler(params)
	if err != nil {
		c.logger.Error("sampling handler failed", "error", err)
		return c.sendJsonRpcErrorResponse(id, -1, "Sampling error", err.Error())
	}

	// Validate response content type against protocol version
	if !response.Content.IsValidForVersion(c.negotiatedVersion) {
		errMsg := fmt.Sprintf("Response content type '%s' not supported in protocol version '%s'",
			response.Content.Type, c.negotiatedVersion)
		c.logger.Error("invalid response content type", "type", response.Content.Type, "version", c.negotiatedVersion)
		return c.sendJsonRpcErrorResponse(id, -32600, "Invalid Response", errMsg)
	}

	// Send the response
	return c.sendJsonRpcSuccessResponse(id, response)
}

// sendJsonRpcErrorResponse sends a JSON-RPC error response.
func (c *clientImpl) sendJsonRpcErrorResponse(id int64, code int, message, data string) error {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}

	if data != "" {
		response["error"].(map[string]interface{})["data"] = data
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal error response: %w", err)
	}

	_, err = c.transport.Send(responseJSON)
	if err != nil {
		return fmt.Errorf("failed to send error response: %w", err)
	}

	return nil
}

// sendJsonRpcSuccessResponse sends a JSON-RPC success response.
func (c *clientImpl) sendJsonRpcSuccessResponse(id int64, result interface{}) error {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal success response: %w", err)
	}

	_, err = c.transport.Send(responseJSON)
	if err != nil {
		return fmt.Errorf("failed to send success response: %w", err)
	}

	return nil
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

// RegisterSamplingHandler registers a sampling handler on the client
func RegisterSamplingHandler(c Client, handler SamplingHandler) {
	c.WithSamplingHandler(handler)
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

// SamplingContentHandler is the interface for all sampling content handlers
type SamplingContentHandler interface {
	ToMessageContent() SamplingMessageContent
	Validate() error
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
