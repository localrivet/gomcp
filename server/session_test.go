package server_test

import (
	"io"
	"sync"

	"github.com/localrivet/gomcp/protocol"
)

// mockClientSession implements types.ClientSession for testing.
type mockClientSession struct {
	mu                 sync.Mutex
	sessionID          string
	sentResponses      []protocol.JSONRPCResponse
	sentNotifications  []protocol.JSONRPCNotification
	sentRequests       []protocol.JSONRPCRequest
	isInitialized      bool
	negotiatedVersion  string
	clientCapabilities protocol.ClientCapabilities
	closed             bool
}

func newMockClientSession(id string) *mockClientSession {
	return &mockClientSession{
		sessionID:         id,
		sentResponses:     make([]protocol.JSONRPCResponse, 0),
		sentNotifications: make([]protocol.JSONRPCNotification, 0),
		sentRequests:      make([]protocol.JSONRPCRequest, 0),
	}
}

// --- types.ClientSession implementation ---

func (m *mockClientSession) SessionID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessionID
}

func (m *mockClientSession) SendResponse(response protocol.JSONRPCResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return io.ErrClosedPipe
	}
	m.sentResponses = append(m.sentResponses, response)
	return nil
}

func (m *mockClientSession) SendNotification(notification protocol.JSONRPCNotification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return io.ErrClosedPipe
	}
	m.sentNotifications = append(m.sentNotifications, notification)
	return nil
}

func (m *mockClientSession) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockClientSession) Initialize() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isInitialized = true
}

func (m *mockClientSession) Initialized() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isInitialized
}

func (m *mockClientSession) SetNegotiatedVersion(version string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.negotiatedVersion = version
}

func (m *mockClientSession) GetNegotiatedVersion() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.negotiatedVersion
}

func (m *mockClientSession) StoreClientCapabilities(caps protocol.ClientCapabilities) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clientCapabilities = caps
}

func (m *mockClientSession) GetClientCapabilities() protocol.ClientCapabilities {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clientCapabilities
}

func (m *mockClientSession) GetWriter() io.Writer {
	// Not typically used directly by server logic being tested here
	return nil
}

// SendRequest stores the request for test inspection.
func (m *mockClientSession) SendRequest(request protocol.JSONRPCRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return io.ErrClosedPipe
	}
	m.sentRequests = append(m.sentRequests, request)
	return nil
}

// --- Test Helper Methods ---

func (m *mockClientSession) GetSentResponses() []protocol.JSONRPCResponse {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to avoid race conditions if the test modifies the slice
	responsesCopy := make([]protocol.JSONRPCResponse, len(m.sentResponses))
	copy(responsesCopy, m.sentResponses)
	return responsesCopy
}

func (m *mockClientSession) GetSentNotifications() []protocol.JSONRPCNotification {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy
	notifsCopy := make([]protocol.JSONRPCNotification, len(m.sentNotifications))
	copy(notifsCopy, m.sentNotifications)
	return notifsCopy
}

func (m *mockClientSession) GetSentRequests() []protocol.JSONRPCRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy
	reqsCopy := make([]protocol.JSONRPCRequest, len(m.sentRequests))
	copy(reqsCopy, m.sentRequests)
	return reqsCopy
}

func (m *mockClientSession) ClearMessages() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentResponses = make([]protocol.JSONRPCResponse, 0)
	m.sentNotifications = make([]protocol.JSONRPCNotification, 0)
	m.sentRequests = make([]protocol.JSONRPCRequest, 0)
}
