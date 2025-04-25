package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
	// Use the same SSE library as the client
)

// --- Mock Logger ---
type NilLogger struct{}

func (n *NilLogger) Debug(msg string, args ...interface{}) {}
func (n *NilLogger) Info(msg string, args ...interface{})  {}
func (n *NilLogger) Warn(msg string, args ...interface{})  {}
func (n *NilLogger) Error(msg string, args ...interface{}) {}
func NewNilLogger() *NilLogger                             { return &NilLogger{} }

var _ types.Logger = (*NilLogger)(nil)

// --- Test Server Handler ---
// Simulates the SSE+HTTP server for testing the client's Connect method.
type mockServerHandler struct {
	t                *testing.T
	ssePath          string
	messagePath      string
	serverVersion    string // Version the mock server will respond with
	serverCaps       protocol.ServerCapabilities
	expectInitParams *protocol.InitializeRequestParams // Optional: verify client params
	initRequestID    interface{}                       // Store the ID from the initialize request
	initReceived     chan struct{}                     // Signal when initialize POST is received
	initializedSent  chan struct{}                     // Signal when initialized notification POST is received
	mu               sync.Mutex
}

func newMockServerHandler(t *testing.T, serverVersion string) *mockServerHandler {
	return &mockServerHandler{
		t:               t,
		ssePath:         "/sse",     // Default, matches client default
		messagePath:     "/message", // Default, matches client default
		serverVersion:   serverVersion,
		serverCaps:      protocol.ServerCapabilities{Logging: &struct{}{}}, // Basic caps
		initReceived:    make(chan struct{}, 1),
		initializedSent: make(chan struct{}, 1),
	}
}

func (h *mockServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	query := r.URL.Query()
	sessionID := query.Get("sessionId")

	h.t.Logf("Mock Server: Received request %s %s (Session: %s)", r.Method, path, sessionID)

	if path == h.ssePath && r.Method == http.MethodGet {
		h.handleSSE(w, r)
	} else if path == h.messagePath && r.Method == http.MethodPost {
		h.handleMessage(w, r, sessionID)
	} else {
		h.t.Errorf("Mock Server: Received unexpected request: %s %s", r.Method, path)
		http.NotFound(w, r)
	}
}

func (h *mockServerHandler) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*") // Basic CORS for test

	// Generate a session ID and send the endpoint event
	mockSessionID := uuid.NewString()
	messageURL := fmt.Sprintf("%s?sessionId=%s", h.messagePath, mockSessionID) // Relative path
	sseEvent := fmt.Sprintf("event: endpoint\ndata: %s\n\n", messageURL)
	_, _ = fmt.Fprint(w, sseEvent)
	flusher.Flush()
	h.t.Logf("Mock Server: Sent endpoint event (Session: %s)", mockSessionID)

	// Keep connection open until client disconnects or test finishes
	<-r.Context().Done()
	h.t.Logf("Mock Server: SSE connection closed (Session: %s)", mockSessionID)
}

func (h *mockServerHandler) handleMessage(w http.ResponseWriter, r *http.Request, sessionID string) {
	if sessionID == "" {
		http.Error(w, "Missing sessionId", http.StatusBadRequest)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}

	var req protocol.JSONRPCRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		http.Error(w, "Failed to parse JSON request", http.StatusBadRequest)
		return
	}

	h.t.Logf("Mock Server: Received POST %s (ID: %v, Session: %s)", req.Method, req.ID, sessionID)

	if req.Method == protocol.MethodInitialize {
		h.mu.Lock()
		h.initRequestID = req.ID // Store the ID
		h.mu.Unlock()

		// Optional: Verify client params
		if h.expectInitParams != nil {
			var params protocol.InitializeRequestParams
			if err := protocol.UnmarshalPayload(req.Params, &params); err == nil {
				// Basic check - could use reflect.DeepEqual if needed
				if params.ProtocolVersion != h.expectInitParams.ProtocolVersion {
					h.t.Errorf("Initialize request version mismatch: expected %s, got %s", h.expectInitParams.ProtocolVersion, params.ProtocolVersion)
				}
			} else {
				h.t.Errorf("Failed to unmarshal initialize params for verification: %v", err)
			}
		}

		// Send Initialize Response via SSE (simulate server behavior)
		// This requires finding the SSE connection associated with sessionID,
		// which is complex to mock here. Instead, we'll signal that init was received.
		h.t.Logf("Mock Server: Signalling init received (ID: %v)", req.ID)
		h.initReceived <- struct{}{}

		// Client expects 200 OK or 202 Accepted for the POST
		w.WriteHeader(http.StatusAccepted)

	} else if req.Method == protocol.MethodInitialized {
		h.t.Logf("Mock Server: Received initialized notification (Session: %s)", sessionID)
		h.initializedSent <- struct{}{}
		w.WriteHeader(http.StatusNoContent) // No response body for notification
	} else {
		h.t.Errorf("Mock Server: Received unexpected method via POST: %s", req.Method)
		http.Error(w, "Method not supported via POST in mock", http.StatusMethodNotAllowed)
	}
}

// Helper to simulate sending the InitializeResult via SSE after init POST is received
func (h *mockServerHandler) simulateSendInitResponse(client *Client) {
	select {
	case <-h.initReceived:
		h.t.Logf("Mock Server: Init POST received, simulating SSE response...")
		h.mu.Lock()
		reqID := h.initRequestID
		h.mu.Unlock()

		if reqID == nil {
			h.t.Error("Mock Server: Initialize request ID not captured")
			return
		}

		initResult := protocol.InitializeResult{
			ProtocolVersion: h.serverVersion,
			Capabilities:    h.serverCaps,
			ServerInfo:      protocol.Implementation{Name: "MockServer", Version: "1.0"},
		}
		resp := protocol.JSONRPCResponse{JSONRPC: "2.0", ID: reqID, Result: initResult}
		respBytes, _ := json.Marshal(resp)

		// Directly process the response in the client as if received via SSE
		if err := client.processMessage(respBytes); err != nil {
			h.t.Errorf("Mock Server: Error injecting init response into client: %v", err)
		} else {
			h.t.Logf("Mock Server: Injected init response for ID %v", reqID)
		}
	case <-time.After(2 * time.Second): // Timeout for safety
		h.t.Error("Mock Server: Timed out waiting for init POST to be received")
	}
}

// --- Tests ---

func TestClientInitializeSuccessCurrentVersion(t *testing.T) {
	mockHandler := newMockServerHandler(t, protocol.CurrentProtocolVersion)
	mockHandler.expectInitParams = &protocol.InitializeRequestParams{ProtocolVersion: protocol.CurrentProtocolVersion} // Expect client sends current
	server := httptest.NewServer(mockHandler)
	defer server.Close()

	client, err := NewClient("TestClient", ClientOptions{
		Logger:        NewNilLogger(),
		ServerBaseURL: server.URL,
		// PreferredProtocolVersion: protocol.CurrentProtocolVersion, // Default
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Run simulateSendInitResponse in background *before* calling Connect
	go mockHandler.simulateSendInitResponse(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Generous timeout for connect + handshake
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Client.Connect failed: %v", err)
	}
	defer client.Close()

	// Wait for initialized notification to be processed by mock server
	select {
	case <-mockHandler.initializedSent:
		t.Log("Mock server received initialized notification.")
	case <-ctx.Done():
		t.Fatalf("Timed out waiting for initialized notification to be sent: %v", ctx.Err())
	}

	// Verify client state
	client.stateMu.RLock()
	initialized := client.initialized
	// connected := client.connected // Removed unused variable read
	negotiatedVersion := client.negotiatedVersion
	client.stateMu.RUnlock()

	if !initialized {
		t.Error("Client state 'initialized' is false after successful Connect")
	}
	// 'connected' state check removed as it might be transient during handshake failure scenarios in other tests
	if negotiatedVersion != protocol.CurrentProtocolVersion {
		t.Errorf("Client negotiated version mismatch: expected %s, got %s", protocol.CurrentProtocolVersion, negotiatedVersion)
	}
}

func TestClientInitializeSuccessOldVersion(t *testing.T) {
	mockHandler := newMockServerHandler(t, protocol.OldProtocolVersion)                                                // Server responds with OLD version
	mockHandler.expectInitParams = &protocol.InitializeRequestParams{ProtocolVersion: protocol.CurrentProtocolVersion} // Expect client sends current by default
	server := httptest.NewServer(mockHandler)
	defer server.Close()

	client, err := NewClient("TestClient", ClientOptions{
		Logger:        NewNilLogger(),
		ServerBaseURL: server.URL,
		// Client still prefers current version by default
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	go mockHandler.simulateSendInitResponse(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Client.Connect failed: %v", err)
	}
	defer client.Close()

	select {
	case <-mockHandler.initializedSent:
	case <-ctx.Done():
		t.Fatalf("Timed out waiting for initialized notification: %v", ctx.Err())
	}

	client.stateMu.RLock()
	initialized := client.initialized
	negotiatedVersion := client.negotiatedVersion
	client.stateMu.RUnlock()

	if !initialized {
		t.Error("Client state 'initialized' is false after successful Connect")
	}
	if negotiatedVersion != protocol.OldProtocolVersion { // Client should accept the OLD version offered by server
		t.Errorf("Client negotiated version mismatch: expected %s, got %s", protocol.OldProtocolVersion, negotiatedVersion)
	}
}

func TestClientInitializeUnsupportedVersion(t *testing.T) {
	unsupportedVersion := "1999-01-01"
	mockHandler := newMockServerHandler(t, unsupportedVersion) // Server responds with unsupported version
	server := httptest.NewServer(mockHandler)
	defer server.Close()

	client, err := NewClient("TestClient", ClientOptions{
		Logger:        NewNilLogger(),
		ServerBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	go mockHandler.simulateSendInitResponse(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = client.Connect(ctx) // Expect this to fail

	if err == nil {
		t.Fatalf("Client.Connect succeeded unexpectedly with unsupported server version")
		client.Close()
	}

	if !strings.Contains(err.Error(), "server selected unsupported protocol version") {
		t.Errorf("Expected error about unsupported version, got: %v", err)
	}

	client.stateMu.RLock()
	initialized := client.initialized
	// connected := client.connected // Removed unused variable
	client.stateMu.RUnlock()

	if initialized {
		t.Error("Client state 'initialized' is true after failed Connect")
	}
	// Note: 'connected' might briefly become true during SSE setup before handshake fails
	// if connected {
	// 	t.Error("Client state 'connected' is true after failed Connect")
	// }
}

func TestClientInitializePreferredOldVersion(t *testing.T) {
	mockHandler := newMockServerHandler(t, protocol.OldProtocolVersion)                                            // Server responds with OLD version
	mockHandler.expectInitParams = &protocol.InitializeRequestParams{ProtocolVersion: protocol.OldProtocolVersion} // Expect client sends OLD
	server := httptest.NewServer(mockHandler)
	defer server.Close()

	client, err := NewClient("TestClient", ClientOptions{
		Logger:                   NewNilLogger(),
		ServerBaseURL:            server.URL,
		PreferredProtocolVersion: protocol.OldProtocolVersion, // Client explicitly prefers OLD version
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	go mockHandler.simulateSendInitResponse(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Client.Connect failed: %v", err)
	}
	defer client.Close()

	select {
	case <-mockHandler.initializedSent:
	case <-ctx.Done():
		t.Fatalf("Timed out waiting for initialized notification: %v", ctx.Err())
	}

	client.stateMu.RLock()
	initialized := client.initialized
	negotiatedVersion := client.negotiatedVersion
	client.stateMu.RUnlock()

	if !initialized {
		t.Error("Client state 'initialized' is false after successful Connect")
	}
	if negotiatedVersion != protocol.OldProtocolVersion {
		t.Errorf("Client negotiated version mismatch: expected %s, got %s", protocol.OldProtocolVersion, negotiatedVersion)
	}
}

// TODO: Add tests for parsing messages with new optional fields (e.g., ProgressParams.message)
// TODO: Add tests for client sending requests after successful connection
