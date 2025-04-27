package sse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect" // Added for DeepEqual
	"strings"
	"sync"
	"testing"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
)

// --- Mock Logger ---
type NilLogger struct{}

func (n *NilLogger) Debug(msg string, args ...interface{}) {}
func (n *NilLogger) Info(msg string, args ...interface{})  {}
func (n *NilLogger) Warn(msg string, args ...interface{})  {}
func (n *NilLogger) Error(msg string, args ...interface{}) {}
func NewNilLogger() *NilLogger                             { return &NilLogger{} }

var _ types.Logger = (*NilLogger)(nil)

// --- Mock MCPServerLogic ---
type mockMCPServer struct {
	handleMessageFunc func(ctx context.Context, sessionID string, rawMessage json.RawMessage) []*protocol.JSONRPCResponse
	sessions          sync.Map // Store mock sessions if needed for Register/Unregister testing
}

func (m *mockMCPServer) HandleMessage(ctx context.Context, sessionID string, rawMessage json.RawMessage) []*protocol.JSONRPCResponse {
	if m.handleMessageFunc != nil {
		return m.handleMessageFunc(ctx, sessionID, rawMessage)
	}
	return nil // Default behavior
}

func (m *mockMCPServer) RegisterSession(session types.ClientSession) error { // Use types.ClientSession
	m.sessions.Store(session.SessionID(), session)
	return nil
}

func (m *mockMCPServer) UnregisterSession(sessionID string) {
	m.sessions.Delete(sessionID)
}

// --- Mock sseSession for Testing HandleMessage ---
// This mock captures responses instead of queuing them.
type mockTestSession struct {
	id            string
	sentResponses []*protocol.JSONRPCResponse
	mu            sync.Mutex
	logger        types.Logger // Add logger to avoid nil panics if SendResponse fails internally
}

func newMockTestSession(id string, logger types.Logger) *mockTestSession {
	return &mockTestSession{
		id:            id,
		sentResponses: make([]*protocol.JSONRPCResponse, 0),
		logger:        logger,
	}
}

func (m *mockTestSession) SessionID() string { return m.id }
func (m *mockTestSession) SendNotification(notification protocol.JSONRPCNotification) error {
	// Not needed for this test
	return nil
}
func (m *mockTestSession) SendResponse(response protocol.JSONRPCResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Make a copy to avoid data races if the original response is modified later
	respCopy := response
	m.sentResponses = append(m.sentResponses, &respCopy)
	if m.logger != nil { // Log if logger is available
		m.logger.Debug("MockSession %s: Captured response for ID %v", m.id, response.ID)
	}
	return nil
}
func (m *mockTestSession) Close() error                                             { return nil }
func (m *mockTestSession) Initialize()                                              {}
func (m *mockTestSession) Initialized() bool                                        { return true } // Assume initialized for this test
func (m *mockTestSession) SetNegotiatedVersion(version string)                      {}
func (m *mockTestSession) GetNegotiatedVersion() string                             { return "" }
func (m *mockTestSession) StoreClientCapabilities(caps protocol.ClientCapabilities) {}
func (m *mockTestSession) GetClientCapabilities() protocol.ClientCapabilities {
	return protocol.ClientCapabilities{}
}

// Ensure mockTestSession implements the types.ClientSession interface
var _ types.ClientSession = (*mockTestSession)(nil)

// --- Test SSEServer HandleMessage Response Handling ---
func TestSSEServerHandleMessageResponse(t *testing.T) {
	testCases := []struct {
		name                  string
		mockResponsesToReturn []*protocol.JSONRPCResponse // What mockMCPServer returns
		expectedStatusCode    int                         // Expected HTTP status code from HandleMessage
		expectedResponsesSent []*protocol.JSONRPCResponse // What should be captured by mockTestSession.SendResponse
	}{
		{
			name:                  "Nil Response (Notification)",
			mockResponsesToReturn: nil,
			expectedStatusCode:    http.StatusNoContent,
			expectedResponsesSent: nil, // No response sent via SSE
		},
		{
			name:                  "Empty Slice Response",
			mockResponsesToReturn: []*protocol.JSONRPCResponse{},
			expectedStatusCode:    http.StatusNoContent,
			expectedResponsesSent: nil, // No response sent via SSE
		},
		{
			name: "Single Response",
			mockResponsesToReturn: []*protocol.JSONRPCResponse{
				{JSONRPC: "2.0", ID: 1, Result: "test"},
			},
			expectedStatusCode: http.StatusNoContent, // POST is acknowledged
			expectedResponsesSent: []*protocol.JSONRPCResponse{ // Response sent via SSE
				{JSONRPC: "2.0", ID: 1, Result: "test"},
			},
		},
		{
			name: "Multiple Responses (Batch)",
			mockResponsesToReturn: []*protocol.JSONRPCResponse{
				{JSONRPC: "2.0", ID: 1, Result: "test1"},
				{JSONRPC: "2.0", ID: 2, Result: "test2"},
			},
			expectedStatusCode: http.StatusNoContent, // POST is acknowledged
			expectedResponsesSent: []*protocol.JSONRPCResponse{ // Responses sent via SSE
				{JSONRPC: "2.0", ID: 1, Result: "test1"},
				{JSONRPC: "2.0", ID: 2, Result: "test2"},
			},
		},
		{
			name: "Single Error Response",
			mockResponsesToReturn: []*protocol.JSONRPCResponse{
				{JSONRPC: "2.0", ID: 3, Error: &protocol.ErrorPayload{Code: -32600, Message: "Invalid Request"}},
			},
			expectedStatusCode: http.StatusNoContent, // POST is acknowledged
			expectedResponsesSent: []*protocol.JSONRPCResponse{ // Error response sent via SSE
				{JSONRPC: "2.0", ID: 3, Error: &protocol.ErrorPayload{Code: -32600, Message: "Invalid Request"}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup Mock Server Logic
			mockServerLogic := &mockMCPServer{
				handleMessageFunc: func(ctx context.Context, sessionID string, rawMessage json.RawMessage) []*protocol.JSONRPCResponse {
					return tc.mockResponsesToReturn
				},
			}

			// Setup SSEServer with Mock Logic
			sseServer := NewSSEServer(mockServerLogic, SSEServerOptions{Logger: NewNilLogger()})

			// Create and register the mock session that captures SendResponse calls
			mockSess := newMockTestSession("test-session-id", sseServer.logger)
			sseServer.sessions.Store(mockSess.SessionID(), mockSess)
			defer sseServer.sessions.Delete(mockSess.SessionID())

			// Create Request
			reqBody := `{"jsonrpc":"2.0","id":1,"method":"test"}` // Body content doesn't matter much here
			req := httptest.NewRequest(http.MethodPost, "/message?sessionId=test-session-id", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			// Create Response Recorder for the HTTP POST response
			rr := httptest.NewRecorder()

			// Call the handler
			sseServer.HandleMessage(rr, req)

			// Assert HTTP Status Code (should be 204 if responses were sent via SSE)
			if status := rr.Code; status != tc.expectedStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatusCode)
			}

			// Assert HTTP Body (should be empty if responses were sent via SSE)
			if body := rr.Body.String(); body != "" && tc.expectedStatusCode == http.StatusNoContent {
				t.Errorf("handler returned unexpected body for 204 status: got %q want empty", body)
			}

			// Assert Responses Sent via Mock Session's SendResponse
			mockSess.mu.Lock()
			capturedResponses := mockSess.sentResponses
			mockSess.mu.Unlock()

			// Check length first for nil/empty expected cases
			expectedLen := 0
			if tc.expectedResponsesSent != nil {
				expectedLen = len(tc.expectedResponsesSent)
			}
			capturedLen := 0
			if capturedResponses != nil {
				capturedLen = len(capturedResponses)
			}

			if capturedLen != expectedLen {
				t.Errorf("handler sent wrong number of responses via SSE: got %d want %d", capturedLen, expectedLen)
			} else if expectedLen > 0 {
				// Only do DeepEqual if we expect non-empty responses
				if !reflect.DeepEqual(capturedResponses, tc.expectedResponsesSent) {
					// Marshal for better diff output
					capturedJSON, _ := json.MarshalIndent(capturedResponses, "", "  ")
					expectedJSON, _ := json.MarshalIndent(tc.expectedResponsesSent, "", "  ")
					t.Errorf("handler sent unexpected responses via SSE:\ngot:\n%s\n\nwant:\n%s", string(capturedJSON), string(expectedJSON))
				}
			}
			// If expectedLen is 0, we already passed by checking capturedLen == expectedLen
		})
	}
}
