// initialize_test.go (Refactored)
package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"testing"
	// "time" // No longer needed for timeouts in these specific tests
	// Use blank import for mcp package if only types/constants are needed directly in test
	// _ "github.com/localrivet/gomcp"
)

// Helper to create a pair of connected Connections using io.Pipe
func createTestConnections() (*Connection, *Connection) {
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	serverConn := NewConnection(serverReader, serverWriter)
	clientConn := NewConnection(clientReader, clientWriter)
	return serverConn, clientConn
}

// TestInitializeSuccess tests a successful initialization sequence.
func TestInitializeSuccess(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalOutput)

	serverConn, clientConn := createTestConnections()
	defer serverConn.Close()
	defer clientConn.Close()

	serverName := "TestServer-Init-Success"
	clientName := "TestClient-Init-Success"

	// Use NewServerWithConnection to inject the test connection
	server := NewServerWithConnection(serverName, serverConn)
	// Client uses the library's Connect method which now handles initialize
	// Need to create client with test connection too
	client := &Client{conn: clientConn, clientName: clientName}

	var wg sync.WaitGroup
	var serverErr error
	var clientErr error
	var serverReceivedClientInfo Implementation // Capture info received by server

	wg.Add(2)

	// Run server initialization logic in a goroutine
	go func() {
		defer wg.Done()
		// handleInitialize now returns client info/caps, ignore caps for now
		serverReceivedClientInfo, _, serverErr = server.handleInitialize()
		// Server now also waits for Initialized notification within handleInitialize
	}()

	// Run client initialization logic in a goroutine
	go func() {
		defer wg.Done()
		clientErr = client.Connect() // This now performs initialize and sends initialized
	}()

	// Wait for both to complete
	wg.Wait()

	// Check results
	if serverErr != nil {
		t.Errorf("Server initialization failed unexpectedly: %v", serverErr)
	}
	if clientErr != nil {
		t.Errorf("Client initialization failed unexpectedly: %v", clientErr)
	}
	// Check if server received the correct client name via ClientInfo
	if serverReceivedClientInfo.Name != clientName {
		t.Errorf("Server expected client name %q in ClientInfo, got %q", clientName, serverReceivedClientInfo.Name)
	}
	// Check if client stored the correct server name from ServerInfo
	if client.ServerName() != serverName {
		t.Errorf("Client expected server name %q, got %q", serverName, client.ServerName())
	}
}

// TestInitializeUnsupportedVersion tests server rejecting unsupported client version.
func TestInitializeUnsupportedVersion(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalOutput)

	serverConn, clientConn := createTestConnections()
	defer serverConn.Close()
	defer clientConn.Close()

	serverName := "TestServer-Init-FailVer"
	clientName := "TestClient-Init-FailVer"

	server := NewServerWithConnection(serverName, serverConn)
	// Client will send request via simulation

	var wg sync.WaitGroup
	var serverErr error
	var clientErr error

	wg.Add(2)

	// Run server initialization logic
	go func() {
		defer wg.Done()
		// Explicitly close connection after handleInitialize returns, especially in error cases
		defer serverConn.Close()
		_, _, serverErr = server.handleInitialize()
	}()

	// Simulate client sending request with bad version
	go func() {
		defer wg.Done()
		// Send InitializeRequest with wrong protocol version
		badReqParams := InitializeRequestParams{ // Use new struct
			ProtocolVersion: "1999-12-31", // Unsupported version
			ClientInfo:      Implementation{Name: clientName, Version: "0.1"},
			Capabilities:    ClientCapabilities{},
		}
		// Send using SendRequest
		reqID, err := clientConn.SendRequest(MethodInitialize, badReqParams)
		if err != nil {
			clientErr = fmt.Errorf("simulated client failed to send request: %w", err)
			return
		}
		// Expect Error response from server
		rawJSON, err := clientConn.ReceiveRawMessage() // Use ReceiveRawMessage
		if err != nil {
			clientErr = fmt.Errorf("simulated client failed to receive response: %w", err)
			return
		}
		// Parse as JSONRPCResponse
		var jsonrpcResp JSONRPCResponse
		err = json.Unmarshal(rawJSON, &jsonrpcResp)
		if err != nil {
			clientErr = fmt.Errorf("simulated client failed to parse response JSON: %w", err)
			return
		}
		// Check ID and error presence
		if jsonrpcResp.ID != reqID {
			clientErr = fmt.Errorf("simulated client received error with mismatched ID. Expected %v, Got %v", reqID, jsonrpcResp.ID)
			return
		}
		if jsonrpcResp.Error == nil {
			clientErr = fmt.Errorf("simulated client expected error response, but error field was nil")
			return
		}
		errPayload := *jsonrpcResp.Error
		// Check for the numeric code (already done below)
		// err = UnmarshalPayload(rawPayload, &errPayload) // No longer needed
		if err != nil {
			clientErr = fmt.Errorf("simulated client failed to unmarshal error payload: %w", err)
			return
		}
		// Check for the numeric code
		if errPayload.Code != ErrorCodeMCPUnsupportedProtocolVersion {
			clientErr = fmt.Errorf("simulated client expected error code %d, got %d", ErrorCodeMCPUnsupportedProtocolVersion, errPayload.Code)
			return
		}
		clientErr = fmt.Errorf("received expected MCP Error: [%d] %s", errPayload.Code, errPayload.Message)
	}()

	wg.Wait()

	// Check results
	if serverErr == nil {
		t.Error("Server initialization should have failed (version mismatch), but succeeded")
	} else if !strings.Contains(serverErr.Error(), "Unsupported protocol version") { // Check server's internal error
		t.Errorf("Server error message unexpected: %v", serverErr)
	}
	if clientErr == nil {
		t.Error("Client simulation should have captured an error (version mismatch), but didn't")
	} else if !strings.Contains(clientErr.Error(), fmt.Sprintf("[%d]", ErrorCodeMCPUnsupportedProtocolVersion)) { // Check client received correct code
		t.Errorf("Client simulation error message unexpected, should contain code %d: %v", ErrorCodeMCPUnsupportedProtocolVersion, clientErr)
	}
}

// TestInitializeInvalidSequence tests server rejecting non-initialize first message.
func TestInitializeInvalidSequence(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalOutput)

	serverConn, clientConn := createTestConnections()
	defer serverConn.Close()
	defer clientConn.Close()

	serverName := "TestServer-Init-BadSeq"
	server := NewServerWithConnection(serverName, serverConn)

	var wg sync.WaitGroup
	var serverErr error
	var clientErr error

	wg.Add(2)

	// Run server initialization logic
	go func() {
		defer wg.Done()
		// Explicitly close connection after handleInitialize returns
		defer serverConn.Close()
		_, _, serverErr = server.handleInitialize()
	}()

	// Simulate client sending wrong first message
	go func() {
		defer wg.Done()
		// Send an invalid first message (e.g., a notification instead of initialize request)
		invalidFirstMessagePayload := InitializedNotificationParams{} // Example payload
		// Use SendNotification
		err := clientConn.SendNotification(MethodInitialized, invalidFirstMessagePayload) // Send 'initialized' instead of 'initialize'
		if err != nil {
			clientErr = fmt.Errorf("simulated client failed to send invalid message: %w", err)
			return
		}
		// Server should error out. Attempt to read the error message from the server
		// to prevent blocking the server's SendMessage call on the pipe.
		_, err = clientConn.ReceiveRawMessage() // Use ReceiveRawMessage
		if err != nil && !errors.Is(err, io.ErrClosedPipe) && !strings.Contains(err.Error(), "pipe") {
			// Log if receiving the error message itself failed unexpectedly
			log.Printf("Client simulator (invalid sequence): Error receiving server response: %v", err)
			// We don't set clientErr here as the main check is serverErr
		}
	}()

	wg.Wait()

	// Check results: Expect server error about wrong message type/method
	if serverErr == nil {
		t.Error("Server initialization should have failed (invalid sequence), but succeeded")
	} else if !strings.Contains(serverErr.Error(), "Expected method 'initialize', got") { // Check for correct error message
		t.Errorf("Server error message unexpected for invalid sequence: %v", serverErr)
	}
	if clientErr != nil {
		t.Logf("Client simulation encountered an error as expected: %v", clientErr)
	}
}

// TestInitializeMalformedPayload tests server rejecting malformed initialize params.
func TestInitializeMalformedPayload(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalOutput)

	serverConn, clientConn := createTestConnections()
	defer serverConn.Close()
	defer clientConn.Close()

	serverName := "TestServer-Init-BadPayload"
	server := NewServerWithConnection(serverName, serverConn)

	var wg sync.WaitGroup
	var serverErr error
	var clientErr error

	wg.Add(2)

	// Run server initialization logic
	go func() {
		defer wg.Done()
		// Explicitly close connection after handleInitialize returns
		defer serverConn.Close()
		_, _, serverErr = server.handleInitialize()
	}()

	// Simulate client sending initialize request with bad params structure
	go func() {
		defer wg.Done()
		// Send MethodInitialize, but payload isn't InitializeRequestParams
		malformedPayload := map[string]int{"wrong_field": 123}
		_, err := clientConn.SendRequest(MethodInitialize, malformedPayload) // Use SendRequest
		if err != nil {
			clientErr = fmt.Errorf("simulated client failed to send malformed payload: %w", err)
			return
		}
		// Server should error out. Attempt to read the error message from the server
		// to prevent blocking the server's SendMessage call on the pipe.
		_, err = clientConn.ReceiveRawMessage() // Use ReceiveRawMessage
		if err != nil && !errors.Is(err, io.ErrClosedPipe) && !strings.Contains(err.Error(), "pipe") {
			// Log if receiving the error message itself failed unexpectedly
			log.Printf("Client simulator (malformed payload): Error receiving server response: %v", err)
			// We don't set clientErr here as the main check is serverErr
		}
	}()

	wg.Wait()

	// Check results: Expect server error about failing to unmarshal params
	if serverErr == nil {
		t.Error("Server initialization should have failed (malformed payload), but succeeded")
	} else if !strings.Contains(serverErr.Error(), "malformed InitializeRequest params: missing protocolVersion") { // Check for the validation error
		t.Errorf("Server error message unexpected for malformed payload: %v", serverErr)
	}
	if clientErr != nil {
		t.Logf("Client simulation encountered an error as expected: %v", clientErr)
	}
}

// TODO: Add test for missing Initialized notification from client
// TODO: Add test for connection closing during initialization
