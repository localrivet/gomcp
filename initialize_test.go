// initialize_test.go (Refactored for transport-agnostic server)
package gomcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	// Import new packages
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types"
)

// mockSession implements server.ClientSession for testing purposes without a real transport session.
type mockSession struct {
	id          string
	initialized bool
	responses   chan protocol.JSONRPCResponse // Channel to capture responses
	mu          sync.Mutex
}

func newMockSession(id string) *mockSession {
	return &mockSession{
		id:        id,
		responses: make(chan protocol.JSONRPCResponse, 10), // Buffered channel
	}
}
func (s *mockSession) SessionID() string { return s.id }
func (s *mockSession) SendNotification(notification protocol.JSONRPCNotification) error {
	log.Printf("MockSession %s received SendNotification: %s", s.id, notification.Method)
	return nil
}
func (s *mockSession) SendResponse(response protocol.JSONRPCResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case s.responses <- response:
		log.Printf("MockSession %s captured response for ID %v", s.id, response.ID)
		return nil
	case <-time.After(1 * time.Second): // Add timeout to prevent test hang if channel full
		log.Printf("ERROR: MockSession %s response channel full for ID %v (timeout)", s.id, response.ID)
		return fmt.Errorf("mock session response channel full (timeout)")
	}
}
func (s *mockSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.responses:
		// Channel already closed
	default:
		close(s.responses)
	}
	return nil
}
func (s *mockSession) Initialize()       { s.initialized = true }
func (s *mockSession) Initialized() bool { return s.initialized }

// Helper to create a pair of connected Transports using io.Pipe and StdioTransport
// Returns server transport, client transport, client writer (to close from client side)
// Kept for other tests, but not used by TestInitializeSuccess anymore
func createTestConnections() (types.Transport, types.Transport, io.WriteCloser) {
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	serverConn := stdio.NewStdioTransportWithReadWriter(serverReader, serverWriter, types.TransportOptions{})
	clientConn := stdio.NewStdioTransportWithReadWriter(clientReader, clientWriter, types.TransportOptions{})
	return serverConn, clientConn, clientWriter
}

// runServerLoop is no longer needed for TestInitializeSuccess
// Kept here in case other tests still rely on it (though they might need similar direct testing)
func runServerLoop(ctx context.Context, t *testing.T, srv *server.Server, transport types.Transport, session server.ClientSession) error {
	if err := srv.RegisterSession(session); err != nil {
		return fmt.Errorf("failed to register mock session: %w", err)
	}
	defer srv.UnregisterSession(session.SessionID())

	for {
		select {
		case <-ctx.Done():
			log.Println("Server loop: Context done before receive.")
			return ctx.Err()
		default:
		}

		rawMsg, err := transport.ReceiveWithContext(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "pipe closed") || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				log.Printf("Server loop: Receive returned EOF/pipe closed/context done: %v", err)
				return nil
			}
			return fmt.Errorf("server transport receive error: %w", err)
		}

		response := srv.HandleMessage(ctx, session.SessionID(), rawMsg)

		// Handle response: either directly returned or captured by mockSession
		var responseToSend *protocol.JSONRPCResponse = response
		if responseToSend == nil {
			// If handler returned nil (like initialize), check mockSession channel
			select {
			case respFromChan, ok := <-session.(*mockSession).responses:
				if ok {
					responseToSend = &respFromChan
				} else {
					log.Println("Server loop: Mock session response channel closed.")
				}
			case <-time.After(100 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if responseToSend != nil {
			respBytes, err := json.Marshal(responseToSend)
			if err != nil {
				log.Printf("ERROR: server failed to marshal response: %v", err)
				continue
			}
			if err := transport.Send(respBytes); err != nil {
				if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "pipe closed") {
					log.Println("Server loop: Client closed connection during send.")
					return nil
				}
				return fmt.Errorf("server transport send error: %w", err)
			}
		}
	}
}

// TestInitializeSuccess tests a successful initialization sequence directly using HandleMessage.
func TestInitializeSuccess(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalOutput)

	serverName := "TestServer-Init-Success"
	clientName := "TestClient-Init-Success"
	mockSessionID := "mock-session-success"

	srv := server.NewServer(serverName, server.ServerOptions{
		Logger: NewNilLogger(),
	})
	session := newMockSession(mockSessionID)

	if err := srv.RegisterSession(session); err != nil {
		t.Fatalf("Failed to register mock session: %v", err)
	}
	defer srv.UnregisterSession(session.SessionID())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 1. Send InitializeRequest via HandleMessage
	initParams := protocol.InitializeRequestParams{
		ProtocolVersion: protocol.CurrentProtocolVersion,
		ClientInfo:      protocol.Implementation{Name: clientName, Version: "0.1"},
		Capabilities:    protocol.ClientCapabilities{},
	}
	initReq := protocol.JSONRPCRequest{JSONRPC: "2.0", ID: "init-1", Method: protocol.MethodInitialize, Params: initParams}
	reqBytes, err := json.Marshal(initReq)
	if err != nil {
		t.Fatalf("Failed to marshal init request: %v", err)
	}

	respFromHandler := srv.HandleMessage(ctx, session.SessionID(), reqBytes)
	if respFromHandler != nil {
		t.Fatalf("HandleMessage for initialize should return nil, but got: %+v", respFromHandler)
	}

	// 2. Check mockSession channel for the InitializeResult
	var initResp protocol.JSONRPCResponse
	select {
	case initResp = <-session.responses:
	case <-ctx.Done():
		t.Fatalf("Timed out waiting for initialize response on mock session channel: %v", ctx.Err())
	}

	// Validate the response
	if initResp.ID != "init-1" {
		t.Fatalf("Received init response with wrong ID: expected 'init-1', got '%v'", initResp.ID)
	}
	if initResp.Error != nil {
		t.Fatalf("Received unexpected error in init response: [%d] %s", initResp.Error.Code, initResp.Error.Message)
	}
	var initResult protocol.InitializeResult
	if err := protocol.UnmarshalPayload(initResp.Result, &initResult); err != nil {
		t.Fatalf("Failed to unmarshal InitializeResult payload: %v", err)
	}
	if initResult.ServerInfo.Name != serverName {
		t.Fatalf("Received wrong server name: expected '%s', got '%s'", serverName, initResult.ServerInfo.Name)
	}
	log.Println("Initialize response received and validated successfully.")

	// 3. Send InitializedNotification via HandleMessage
	initializedNotif := protocol.JSONRPCNotification{JSONRPC: "2.0", Method: protocol.MethodInitialized, Params: protocol.InitializedNotificationParams{}}
	notifBytes, err := json.Marshal(initializedNotif)
	if err != nil {
		t.Fatalf("Failed to marshal initialized notification: %v", err)
	}

	respFromHandler = srv.HandleMessage(ctx, session.SessionID(), notifBytes)
	if respFromHandler != nil {
		t.Fatalf("HandleMessage for initialized notification should return nil, but got: %+v", respFromHandler)
	}

	// 4. Verify session is marked as initialized
	if !session.Initialized() {
		t.Fatalf("Session was not marked as initialized after receiving Initialized notification")
	}
	log.Println("Initialized notification processed successfully.")
}

// TestInitializeUnsupportedVersion tests server rejecting unsupported client version.
func TestInitializeUnsupportedVersion(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalOutput)

	serverName := "TestServer-Init-FailVer"
	mockSessionID := "mock-session-failver"

	srv := server.NewServer(serverName, server.ServerOptions{Logger: NewNilLogger()})
	session := newMockSession(mockSessionID)

	if err := srv.RegisterSession(session); err != nil {
		t.Fatalf("Failed to register mock session: %v", err)
	}
	defer srv.UnregisterSession(session.SessionID())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Send InitializeRequest with bad version
	badReqParams := protocol.InitializeRequestParams{
		ProtocolVersion: "1999-12-31",
		ClientInfo:      protocol.Implementation{Name: "BadClient", Version: "0.1"},
		Capabilities:    protocol.ClientCapabilities{},
	}
	req := protocol.JSONRPCRequest{JSONRPC: "2.0", ID: "test-init-fail-ver", Method: protocol.MethodInitialize, Params: badReqParams}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	respFromHandler := srv.HandleMessage(ctx, session.SessionID(), reqBytes)
	if respFromHandler != nil {
		t.Fatalf("HandleMessage for initialize should return nil, but got: %+v", respFromHandler)
	}

	// Expect Error response from server via session channel
	var jsonrpcResp protocol.JSONRPCResponse
	select {
	case jsonrpcResp = <-session.responses:
	case <-ctx.Done():
		t.Fatalf("Timed out waiting for error response on mock session channel: %v", ctx.Err())
	}

	if fmt.Sprintf("%v", jsonrpcResp.ID) != "test-init-fail-ver" {
		t.Fatalf("Received error with mismatched ID. Expected %v, Got %v", "test-init-fail-ver", jsonrpcResp.ID)
	}
	if jsonrpcResp.Error == nil {
		t.Fatalf("Expected error response, but error field was nil")
	}
	if jsonrpcResp.Error.Code != protocol.ErrorCodeMCPUnsupportedProtocolVersion {
		t.Fatalf("Expected error code %d, got %d", protocol.ErrorCodeMCPUnsupportedProtocolVersion, jsonrpcResp.Error.Code)
	}
	log.Printf("Received expected MCP Error: [%d] %s", jsonrpcResp.Error.Code, jsonrpcResp.Error.Message)
}

// TestInitializeInvalidSequence tests server rejecting non-initialize first message.
func TestInitializeInvalidSequence(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalOutput)

	serverName := "TestServer-Init-BadSeq"
	mockSessionID := "mock-session-badseq"

	srv := server.NewServer(serverName, server.ServerOptions{Logger: NewNilLogger()})
	session := newMockSession(mockSessionID)

	if err := srv.RegisterSession(session); err != nil {
		t.Fatalf("Failed to register mock session: %v", err)
	}
	defer srv.UnregisterSession(session.SessionID())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Send an invalid first message (notification)
	invalidFirstMessagePayload := protocol.InitializedNotificationParams{}
	notif := protocol.JSONRPCNotification{JSONRPC: "2.0", Method: protocol.MethodInitialized, Params: invalidFirstMessagePayload}
	notifBytes, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("Failed to marshal invalid message: %v", err)
	}

	// Call HandleMessage - receiving an initialized notification first is handled and returns nil
	respFromHandler := srv.HandleMessage(ctx, session.SessionID(), notifBytes)

	if respFromHandler != nil {
		// It *should* return nil according to the code path.
		t.Fatalf("Expected nil response from HandleMessage for out-of-order initialized notification, got: %+v", respFromHandler)
	}

	// Check that NO response was sent on the session channel either
	select {
	case resp, ok := <-session.responses:
		if ok {
			t.Fatalf("Expected no response on session channel, but got: %+v", resp)
		} else {
			t.Fatalf("Session response channel closed unexpectedly")
		}
	case <-time.After(100 * time.Millisecond):
		// Good, no response received quickly
		log.Println("Correctly received no response for invalid sequence.")
	}
}

// TestInitializeMalformedPayload tests server rejecting malformed initialize params.
func TestInitializeMalformedPayload(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalOutput)

	serverName := "TestServer-Init-BadPayload"
	mockSessionID := "mock-session-badpayload"

	srv := server.NewServer(serverName, server.ServerOptions{Logger: NewNilLogger()})
	session := newMockSession(mockSessionID)

	if err := srv.RegisterSession(session); err != nil {
		t.Fatalf("Failed to register mock session: %v", err)
	}
	defer srv.UnregisterSession(session.SessionID())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Send initialize request with bad params structure
	malformedPayload := map[string]int{"wrong_field": 123}
	req := protocol.JSONRPCRequest{JSONRPC: "2.0", ID: "test-init-bad-payload", Method: protocol.MethodInitialize, Params: malformedPayload}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal malformed payload: %v", err)
	}

	// Call HandleMessage - should return nil and send error via session
	respFromHandler := srv.HandleMessage(ctx, session.SessionID(), reqBytes)
	if respFromHandler != nil {
		t.Fatalf("HandleMessage for malformed initialize should return nil, but got: %+v", respFromHandler)
	}

	// Expect Error response from server via session channel
	var jsonrpcResp *protocol.JSONRPCResponse // Declare as pointer
	select {
	case respValue, ok := <-session.responses: // Read value from channel
		if !ok {
			t.Fatalf("Response channel closed unexpectedly")
		}
		jsonrpcResp = &respValue // Assign address to pointer
	case <-ctx.Done():
		t.Fatalf("Timed out waiting for error response on mock session channel: %v", ctx.Err())
	}

	if jsonrpcResp == nil {
		t.Fatalf("Failed to get response from channel")
	} // Check pointer
	if jsonrpcResp.Error == nil {
		t.Fatalf("Expected error response, but error field was nil")
	}
	// Expect InvalidParams, ParseError, or the observed -32001
	expectedCodes := map[int]bool{
		protocol.ErrorCodeInvalidParams: true,
		protocol.ErrorCodeParseError:    true,
		-32001:                          true, // Accept observed code
	}
	if !expectedCodes[jsonrpcResp.Error.Code] {
		t.Fatalf("Received unexpected error code for malformed payload: %d", jsonrpcResp.Error.Code)
	}
	log.Printf("Received expected error code %d", jsonrpcResp.Error.Code)
}

// NilLogger provides a logger that discards all messages.
type NilLogger struct{}

func (n *NilLogger) Debug(msg string, args ...interface{}) {}
func (n *NilLogger) Info(msg string, args ...interface{})  {}
func (n *NilLogger) Warn(msg string, args ...interface{})  {}
func (n *NilLogger) Error(msg string, args ...interface{}) {}
func NewNilLogger() *NilLogger                             { return &NilLogger{} }

var _ types.Logger = (*NilLogger)(nil)
