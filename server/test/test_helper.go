// Package test provides utilities for testing the MCP implementation.
package test

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/localrivet/gomcp/server"
)

// MockTransport is a simple mock transport for testing
type MockTransport struct {
	messages       [][]byte
	sendCallback   func(message []byte) ([]byte, error)
	requestHistory [][]byte
	messageHandler func([]byte)
	sendFunc       func(data []byte) error
	responseDelay  time.Duration
	mu             sync.Mutex
}

// NewMockTransport creates a new MockTransport
func NewMockTransport() *MockTransport {
	return &MockTransport{
		messages:       [][]byte{},
		requestHistory: [][]byte{},
		responseDelay:  10 * time.Millisecond,
	}
}

// Send implements the server.Transport interface
func (m *MockTransport) Send(message []byte) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the request in history
	if m.requestHistory != nil {
		m.requestHistory = append(m.requestHistory, message)
	}

	// If sendCallback is set, use it
	if m.sendCallback != nil {
		return m.sendCallback(message)
	}

	// If sendFunc is set, use it
	if m.sendFunc != nil {
		err := m.sendFunc(message)
		return nil, err
	}

	// Default behavior: append to messages
	m.messages = append(m.messages, message)
	return nil, nil
}

// SendAsync implements the server.Transport interface
func (m *MockTransport) SendAsync(ctx context.Context, message []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the request in history
	if m.requestHistory != nil {
		m.requestHistory = append(m.requestHistory, message)
	}

	// If sendFunc is set, use it
	if m.sendFunc != nil {
		return m.sendFunc(message)
	}

	// Default behavior: append to messages
	m.messages = append(m.messages, message)
	return nil
}

// SetHandler implements the server.Transport interface
func (m *MockTransport) SetHandler(handler func([]byte)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messageHandler = handler
}

// GetSendHandler returns a function to call HandleJSONRPCResponse
func (m *MockTransport) GetSendHandler() func([]byte) {
	return m.messageHandler
}

// GetMessages returns the messages sent to the transport
func (m *MockTransport) GetMessages() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.messages
}

// GetRequestHistory returns the request history
func (m *MockTransport) GetRequestHistory() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requestHistory
}

// SetSendCallback sets the callback for Send
func (m *MockTransport) SetSendCallback(callback func(message []byte) ([]byte, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendCallback = callback
}

// SetSendFunc sets the function for Send
func (m *MockTransport) SetSendFunc(sendFunc func(data []byte) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendFunc = sendFunc
}

// SetResponseDelay sets the delay for responses
func (m *MockTransport) SetResponseDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseDelay = delay
}

// ClearMessages clears the messages
func (m *MockTransport) ClearMessages() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = [][]byte{}
}

// ClearRequestHistory clears the request history
func (m *MockTransport) ClearRequestHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestHistory = [][]byte{}
}

// SimulateMessage simulates receiving a message from the client
func (m *MockTransport) SimulateMessage(message []byte) {
	if m.messageHandler != nil {
		m.messageHandler(message)
	}
}

// Connect simulates connecting the transport
func (m *MockTransport) Connect() error {
	return nil
}

// ConnectWithContext simulates connecting the transport with context
func (m *MockTransport) ConnectWithContext(ctx context.Context) error {
	return nil
}

// Disconnect simulates disconnecting the transport
func (m *MockTransport) Disconnect() error {
	return nil
}

// SendWithContext sends a message with context
func (m *MockTransport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	return m.Send(message)
}

// SetRequestTimeout sets the timeout for requests
func (m *MockTransport) SetRequestTimeout(timeout time.Duration) {}

// SetConnectionTimeout sets the timeout for connection
func (m *MockTransport) SetConnectionTimeout(timeout time.Duration) {}

// RegisterNotificationHandler registers a handler for notifications
func (m *MockTransport) RegisterNotificationHandler(handler func(method string, params []byte)) {}

// NewServer creates a new server for testing
func NewServer(name string) server.Server {
	return server.NewServer(name)
}

// WithLogger creates a logger option for the server
func WithLogger(logger *slog.Logger) server.Option {
	return server.WithLogger(logger)
}

// SessionID represents the ID of a client session
type SessionID string

// ClientSession represents a client session
type ClientSession struct {
	ID         SessionID
	ClientInfo ClientInfo
	Metadata   map[string]interface{}
}

// ClientInfo represents information about a client
type ClientInfo struct {
	SamplingSupported bool
	SamplingCaps      SamplingCapabilities
	ProtocolVersion   string
}

// SamplingCapabilities represents client capabilities for sampling
type SamplingCapabilities struct {
	Supported    bool
	TextSupport  bool
	ImageSupport bool
	AudioSupport bool
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[SessionID]*ClientSession),
	}
}

// SessionManager manages client sessions
type SessionManager struct {
	sessions map[SessionID]*ClientSession
	mu       sync.RWMutex
}

// CreateSession creates a new client session
func (sm *SessionManager) CreateSession(clientInfo ClientInfo, protocolVersion string) *ClientSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	id := SessionID("session-" + time.Now().String())
	session := &ClientSession{
		ID:         id,
		ClientInfo: clientInfo,
		Metadata:   make(map[string]interface{}),
	}
	sm.sessions[id] = session
	return session
}

// GetSession gets a client session by ID
func (sm *SessionManager) GetSession(id SessionID) (*ClientSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[id]
	return session, exists
}

// UpdateSession updates a client session
func (sm *SessionManager) UpdateSession(id SessionID, updateFn func(*ClientSession)) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[id]
	if !exists {
		return false
	}

	updateFn(session)
	return true
}

// UpdateClientCapabilities updates client capabilities
func (sm *SessionManager) UpdateClientCapabilities(id SessionID, caps SamplingCapabilities) bool {
	return sm.UpdateSession(id, func(s *ClientSession) {
		s.ClientInfo.SamplingCaps = caps
	})
}

// CloseSession closes a client session
func (sm *SessionManager) CloseSession(id SessionID) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	_, exists := sm.sessions[id]
	if !exists {
		return false
	}

	delete(sm.sessions, id)
	return true
}

// DetectClientCapabilities detects client capabilities based on protocol version
func DetectClientCapabilities(protocolVersion string) SamplingCapabilities {
	switch protocolVersion {
	case "draft", "2025-03-26":
		return SamplingCapabilities{
			Supported:    true,
			TextSupport:  true,
			ImageSupport: true,
			AudioSupport: true,
		}
	case "2024-11-05":
		return SamplingCapabilities{
			Supported:    true,
			TextSupport:  true,
			ImageSupport: true,
			AudioSupport: false,
		}
	default:
		return SamplingCapabilities{
			Supported:    true,
			TextSupport:  true,
			ImageSupport: false,
			AudioSupport: false,
		}
	}
}

// SamplingMessage represents a message in a sampling conversation
type SamplingMessage struct {
	Role    string                 `json:"role"`
	Content SamplingMessageContent `json:"content"`
}

// SamplingMessageContent represents the content of a sampling message
type SamplingMessageContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// IsValidForVersion checks if the content type is valid for the given protocol version
func (c SamplingMessageContent) IsValidForVersion(version string) bool {
	switch c.Type {
	case "text":
		return true // Text is supported in all versions
	case "image":
		return version == "draft" || version == "2024-11-05" || version == "2025-03-26"
	case "audio":
		return version == "draft" || version == "2025-03-26"
	default:
		return false
	}
}

// SamplingModelPreferences represents the model preferences for a sampling request
type SamplingModelPreferences struct {
	Hints                []SamplingModelHint `json:"hints,omitempty"`
	CostPriority         *float64            `json:"costPriority,omitempty"`
	SpeedPriority        *float64            `json:"speedPriority,omitempty"`
	IntelligencePriority *float64            `json:"intelligencePriority,omitempty"`
}

// SamplingModelHint represents a hint for model selection in sampling requests
type SamplingModelHint struct {
	Name string `json:"name"`
}

// SamplingResponse represents the response to a sampling/createMessage request
type SamplingResponse struct {
	Role       string                 `json:"role"`
	Content    SamplingMessageContent `json:"content"`
	Model      string                 `json:"model,omitempty"`
	StopReason string                 `json:"stopReason,omitempty"`
}

// RequestSamplingOptions defines options for sampling requests
type RequestSamplingOptions struct {
	Timeout          time.Duration
	MaxRetries       int
	RetryInterval    time.Duration
	IgnoreCapability bool
	ForceSession     bool
}

// DefaultSamplingOptions returns the default options for sampling requests
func DefaultSamplingOptions() RequestSamplingOptions {
	return RequestSamplingOptions{
		Timeout:       30 * time.Second,
		MaxRetries:    0,
		RetryInterval: time.Second,
	}
}

// CreateTextSamplingMessage creates a text sampling message
func CreateTextSamplingMessage(role, text string) SamplingMessage {
	return SamplingMessage{
		Role: role,
		Content: SamplingMessageContent{
			Type: "text",
			Text: text,
		},
	}
}

// CreateImageSamplingMessage creates an image sampling message
func CreateImageSamplingMessage(role, imageData, mimeType string) SamplingMessage {
	return SamplingMessage{
		Role: role,
		Content: SamplingMessageContent{
			Type:     "image",
			Data:     imageData,
			MimeType: mimeType,
		},
	}
}

// CreateAudioSamplingMessage creates an audio sampling message
func CreateAudioSamplingMessage(role, audioData, mimeType string) SamplingMessage {
	return SamplingMessage{
		Role: role,
		Content: SamplingMessageContent{
			Type:     "audio",
			Data:     audioData,
			MimeType: mimeType,
		},
	}
}

// Context represents the context for a request/response cycle
type Context struct {
	ctx      context.Context
	Request  *Request
	Response *Response
	server   server.Server
	Version  string
	Metadata map[string]interface{}
}

// Request represents a JSON-RPC request
type Request struct {
	JSONRPC  string                 `json:"jsonrpc"`
	ID       interface{}            `json:"id,omitempty"`
	Method   string                 `json:"method"`
	Params   map[string]interface{} `json:"params,omitempty"`
	ToolName string                 `json:"name,omitempty"`
	ToolArgs map[string]interface{} `json:"arguments,omitempty"`
}

// Response represents a JSON-RPC response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// ToolHandler is the type of function that handles tool calls
type ToolHandler func(*Context, interface{}) (interface{}, error)

// Utility function for resource handler conversion
func convertToResourceHandler(handler interface{}) (server.ResourceHandler, bool) {
	return server.ConvertToResourceHandler(handler)
}
