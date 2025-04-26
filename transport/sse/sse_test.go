package sse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// --- Test SSEServer HandleMessage Response Handling ---
func TestSSEServerHandleMessageResponse(t *testing.T) {
	testCases := []struct {
		name               string
		mockResponses      []*protocol.JSONRPCResponse
		expectedStatusCode int
		expectedBody       string // Expected JSON string, or empty for 204
	}{
		{
			name:               "Nil Response (Notification)",
			mockResponses:      nil,
			expectedStatusCode: http.StatusNoContent,
			expectedBody:       "",
		},
		{
			name:               "Empty Slice Response",
			mockResponses:      []*protocol.JSONRPCResponse{},
			expectedStatusCode: http.StatusNoContent,
			expectedBody:       "",
		},
		{
			name: "Single Response",
			mockResponses: []*protocol.JSONRPCResponse{
				{JSONRPC: "2.0", ID: 1, Result: "test"},
			},
			expectedStatusCode: http.StatusOK,
			expectedBody:       `[{"jsonrpc":"2.0","id":1,"result":"test"}]` + "\n", // Note: json.Encoder adds newline
		},
		{
			name: "Multiple Responses (Batch)",
			mockResponses: []*protocol.JSONRPCResponse{
				{JSONRPC: "2.0", ID: 1, Result: "test1"},
				{JSONRPC: "2.0", ID: 2, Result: "test2"},
			},
			expectedStatusCode: http.StatusOK,
			expectedBody:       `[{"jsonrpc":"2.0","id":1,"result":"test1"},{"jsonrpc":"2.0","id":2,"result":"test2"}]` + "\n",
		},
		{
			name: "Single Error Response",
			mockResponses: []*protocol.JSONRPCResponse{
				{JSONRPC: "2.0", ID: 3, Error: &protocol.ErrorPayload{Code: -32600, Message: "Invalid Request"}},
			},
			expectedStatusCode: http.StatusOK, // HTTP status is OK, error is in JSON-RPC payload
			expectedBody:       `[{"jsonrpc":"2.0","id":3,"error":{"code":-32600,"message":"Invalid Request"}}]` + "\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup Mock Server
			mockServer := &mockMCPServer{
				handleMessageFunc: func(ctx context.Context, sessionID string, rawMessage json.RawMessage) []*protocol.JSONRPCResponse {
					return tc.mockResponses
				},
			}

			// Setup SSEServer with Mock
			sseServer := NewSSEServer(mockServer, SSEServerOptions{Logger: NewNilLogger()})

			// Mock a session registration (needed for HandleMessage check)
			mockSess := &sseSession{sessionID: "test-session-id"} // Minimal mock session
			sseServer.sessions.Store(mockSess.SessionID(), mockSess)
			defer sseServer.sessions.Delete(mockSess.SessionID())

			// Create Request
			reqBody := `{"jsonrpc":"2.0","id":1,"method":"test"}` // Body content doesn't matter for this test
			req := httptest.NewRequest(http.MethodPost, "/message?sessionId=test-session-id", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			// Create Response Recorder
			rr := httptest.NewRecorder()

			// Call the handler
			sseServer.HandleMessage(rr, req)

			// Assert Status Code
			if status := rr.Code; status != tc.expectedStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatusCode)
			}

			// Assert Body
			if body := rr.Body.String(); body != tc.expectedBody {
				// Use JSON comparison for more robust checking if needed, especially for complex objects/arrays
				// For now, direct string comparison works for these examples.
				t.Errorf("handler returned unexpected body:\ngot:  %q\nwant: %q", body, tc.expectedBody)
			}
		})
	}
}
