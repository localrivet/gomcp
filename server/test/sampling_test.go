package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// mockTransport is a simple mock transport for testing
type mockTransport struct {
	messages       [][]byte
	sendCallback   func(message []byte) ([]byte, error)
	requestHistory [][]byte
	messageHandler func([]byte)
}

func (m *mockTransport) Send(message []byte) ([]byte, error) {
	// Record the request in history
	if m.requestHistory == nil {
		m.requestHistory = make([][]byte, 0)
	}
	m.requestHistory = append(m.requestHistory, message)

	if m.sendCallback != nil {
		return m.sendCallback(message)
	}
	return nil, nil
}

func (m *mockTransport) SetMessageHandler(handler func([]byte)) {
	m.messageHandler = handler
}

// GetRequestHistory returns the history of requests sent through this transport
func (m *mockTransport) GetRequestHistory() [][]byte {
	return m.requestHistory
}

// SimulateMessage simulates receiving a message from the client
func (m *mockTransport) SimulateMessage(message []byte) {
	if m.messageHandler != nil {
		m.messageHandler(message)
	}
}

func TestTextSamplingContent(t *testing.T) {
	textContent := &server.TextSamplingContent{
		Text: "Hello, world!",
	}

	// Test conversion to message content
	msgContent := textContent.ToMessageContent()
	if msgContent.Type != "text" {
		t.Errorf("Expected content type 'text', got: %s", msgContent.Type)
	}
	if msgContent.Text != "Hello, world!" {
		t.Errorf("Expected text 'Hello, world!', got: %s", msgContent.Text)
	}

	// Test validation
	err := textContent.Validate()
	if err != nil {
		t.Errorf("Expected no validation error, got: %v", err)
	}

	// Test validation with empty text
	emptyContent := &server.TextSamplingContent{
		Text: "",
	}
	err = emptyContent.Validate()
	if err == nil {
		t.Errorf("Expected validation error for empty text, got none")
	}
}

func TestImageSamplingContent(t *testing.T) {
	imageContent := &server.ImageSamplingContent{
		Data:     "base64data",
		MimeType: "image/png",
	}

	// Test conversion to message content
	msgContent := imageContent.ToMessageContent()
	if msgContent.Type != "image" {
		t.Errorf("Expected content type 'image', got: %s", msgContent.Type)
	}
	if msgContent.Data != "base64data" {
		t.Errorf("Expected data 'base64data', got: %s", msgContent.Data)
	}
	if msgContent.MimeType != "image/png" {
		t.Errorf("Expected MIME type 'image/png', got: %s", msgContent.MimeType)
	}

	// Test validation
	err := imageContent.Validate()
	if err != nil {
		t.Errorf("Expected no validation error, got: %v", err)
	}

	// Test validation with empty data
	invalidContent := &server.ImageSamplingContent{
		Data:     "",
		MimeType: "image/png",
	}
	err = invalidContent.Validate()
	if err == nil {
		t.Errorf("Expected validation error for empty data, got none")
	}

	// Test validation with empty MIME type
	invalidContent = &server.ImageSamplingContent{
		Data:     "base64data",
		MimeType: "",
	}
	err = invalidContent.Validate()
	if err == nil {
		t.Errorf("Expected validation error for empty MIME type, got none")
	}
}

func TestAudioSamplingContent(t *testing.T) {
	audioContent := &server.AudioSamplingContent{
		Data:     "base64audio",
		MimeType: "audio/wav",
	}

	// Test conversion to message content
	msgContent := audioContent.ToMessageContent()
	if msgContent.Type != "audio" {
		t.Errorf("Expected content type 'audio', got: %s", msgContent.Type)
	}
	if msgContent.Data != "base64audio" {
		t.Errorf("Expected data 'base64audio', got: %s", msgContent.Data)
	}
	if msgContent.MimeType != "audio/wav" {
		t.Errorf("Expected MIME type 'audio/wav', got: %s", msgContent.MimeType)
	}

	// Test validation
	err := audioContent.Validate()
	if err != nil {
		t.Errorf("Expected no validation error, got: %v", err)
	}

	// Test validation with empty data
	invalidContent := &server.AudioSamplingContent{
		Data:     "",
		MimeType: "audio/wav",
	}
	err = invalidContent.Validate()
	if err == nil {
		t.Errorf("Expected validation error for empty data, got none")
	}

	// Test validation with empty MIME type
	invalidContent = &server.AudioSamplingContent{
		Data:     "base64audio",
		MimeType: "",
	}
	err = invalidContent.Validate()
	if err == nil {
		t.Errorf("Expected validation error for empty MIME type, got none")
	}
}

func TestValidateContentForVersion(t *testing.T) {
	// Test valid content types for different versions
	testCases := []struct {
		content       server.SamplingContentHandler
		version       string
		expectError   bool
		errorContains string
	}{
		// text is valid in all versions
		{&server.TextSamplingContent{Text: "hello"}, "draft", false, ""},
		{&server.TextSamplingContent{Text: "hello"}, "2024-11-05", false, ""},
		{&server.TextSamplingContent{Text: "hello"}, "2025-03-26", false, ""},

		// image is valid in 2024-11-05, 2025-03-26 and draft
		{&server.ImageSamplingContent{Data: "data", MimeType: "image/png"}, "draft", false, ""},
		{&server.ImageSamplingContent{Data: "data", MimeType: "image/png"}, "2024-11-05", false, ""},
		{&server.ImageSamplingContent{Data: "data", MimeType: "image/png"}, "2025-03-26", false, ""},

		// audio is only valid in 2025-03-26 and draft
		{&server.AudioSamplingContent{Data: "data", MimeType: "audio/wav"}, "draft", false, ""},
		{&server.AudioSamplingContent{Data: "data", MimeType: "audio/wav"}, "2024-11-05", true, "content type 'audio' not supported"},
		{&server.AudioSamplingContent{Data: "data", MimeType: "audio/wav"}, "2025-03-26", false, ""},

		// invalid content should fail validation
		{&server.TextSamplingContent{Text: ""}, "draft", true, "cannot be empty"},
		{&server.ImageSamplingContent{Data: "", MimeType: "image/png"}, "draft", true, "cannot be empty"},
		{&server.AudioSamplingContent{Data: "data", MimeType: ""}, "draft", true, "cannot be empty"},

		// unknown version should default to most restrictive (text only)
		{&server.TextSamplingContent{Text: "hello"}, "unknown", false, ""},
		{&server.ImageSamplingContent{Data: "data", MimeType: "image/png"}, "unknown", true, "content type 'image' not supported"},
		{&server.AudioSamplingContent{Data: "data", MimeType: "audio/wav"}, "unknown", true, "content type 'audio' not supported"},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Case %d", i), func(t *testing.T) {
			err := server.ValidateContentForVersion(tc.content, tc.version)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				} else if tc.errorContains != "" && !contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tc.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestCreateSamplingMessage(t *testing.T) {
	// Test with valid content
	textContent := &server.TextSamplingContent{
		Text: "Hello, world!",
	}

	msg, err := server.CreateSamplingMessage("user", textContent)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if msg.Role != "user" {
		t.Errorf("Expected role 'user', got: %s", msg.Role)
	}

	if msg.Content.Type != "text" {
		t.Errorf("Expected content type 'text', got: %s", msg.Content.Type)
	}

	if msg.Content.Text != "Hello, world!" {
		t.Errorf("Expected text 'Hello, world!', got: %s", msg.Content.Text)
	}

	// Test with invalid content
	invalidContent := &server.TextSamplingContent{
		Text: "",
	}

	_, err = server.CreateSamplingMessage("user", invalidContent)
	if err == nil {
		t.Errorf("Expected error for invalid content, got none")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
