package mcp

import (
	// Need this for the test
	"fmt" // Need this for the test error formatting
	"io"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

// Helper to create a pair of connected Connections using io.Pipe
// Simulates the connection between a client and server for testing.
func createTestConnections() (*Connection, *Connection) {
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	serverConn := NewConnection(serverReader, serverWriter)
	clientConn := NewConnection(clientReader, clientWriter)
	return serverConn, clientConn
}

// TestHandshakeSuccess tests a successful handshake between a client and server.
func TestHandshakeSuccess(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalOutput)

	serverConn, clientConn := createTestConnections()
	defer serverConn.Close()
	defer clientConn.Close()

	serverName := "TestServer-HS-Success"
	clientName := "TestClient-HS-Success"

	server := &Server{conn: serverConn, serverName: serverName}
	client := &Client{conn: clientConn, clientName: clientName}

	var wg sync.WaitGroup
	var serverErr error
	var clientErr error
	var receivedClientName string

	wg.Add(2)

	go func() {
		defer wg.Done()
		receivedClientName, serverErr = server.handleHandshake()
	}()

	go func() {
		defer wg.Done()
		clientErr = client.Connect()
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Handshake test timed out")
	}

	if serverErr != nil {
		t.Errorf("Server handshake failed unexpectedly: %v", serverErr)
	}
	if clientErr != nil {
		t.Errorf("Client handshake failed unexpectedly: %v", clientErr)
	}
	if receivedClientName != clientName {
		t.Errorf("Server expected client name %q, got %q", clientName, receivedClientName)
	}
	if client.ServerName() != serverName {
		t.Errorf("Client expected server name %q, got %q", serverName, client.ServerName())
	}
}

// TestHandshakeUnsupportedVersionByServer tests the case where the client sends
// a list of supported versions that the server does not support.
func TestHandshakeUnsupportedVersionByServer(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalOutput)

	serverConn, clientConn := createTestConnections()
	defer serverConn.Close()
	defer clientConn.Close()

	serverName := "TestServer-HS-FailVer"
	clientName := "TestClient-HS-FailVer"

	server := &Server{conn: serverConn, serverName: serverName}

	var wg sync.WaitGroup
	var serverErr error
	var clientErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		_, serverErr = server.handleHandshake()
	}()

	go func() {
		defer wg.Done()
		badPayload := HandshakeRequestPayload{
			SupportedProtocolVersions: []string{"0.1", "0.9"},
			ClientName:                clientName,
		}
		err := clientConn.SendMessage(MessageTypeHandshakeRequest, badPayload)
		if err != nil {
			clientErr = fmt.Errorf("simulated client failed to send request: %w", err)
			return
		}
		msg, err := clientConn.ReceiveMessage()
		if err != nil {
			clientErr = fmt.Errorf("simulated client failed to receive response: %w", err)
			return
		}
		if msg.MessageType != MessageTypeError {
			clientErr = fmt.Errorf("simulated client expected Error message, got %s", msg.MessageType)
			return
		}
		var errPayload ErrorPayload
		err = UnmarshalPayload(msg.Payload, &errPayload)
		if err != nil {
			clientErr = fmt.Errorf("simulated client failed to unmarshal error payload: %w", err)
			return
		}
		if errPayload.Code != "UnsupportedProtocolVersion" {
			clientErr = fmt.Errorf("simulated client expected error code 'UnsupportedProtocolVersion', got '%s'", errPayload.Code)
			return
		}
		clientErr = fmt.Errorf("received expected MCP Error: [%s] %s", errPayload.Code, errPayload.Message)
	}()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Handshake version test timed out")
	}

	if serverErr == nil {
		t.Error("Server handshake should have failed (version mismatch), but succeeded")
	} else if !strings.Contains(serverErr.Error(), "client does not support protocol version") {
		t.Errorf("Server error message unexpected: %v", serverErr)
	}
	if clientErr == nil {
		t.Error("Client simulation should have captured an error (version mismatch), but didn't")
	} else if !strings.Contains(clientErr.Error(), "UnsupportedProtocolVersion") {
		t.Errorf("Client simulation error message unexpected, should contain 'UnsupportedProtocolVersion': %v", clientErr)
	}
}

// TestHandshakeInvalidSequence tests the server response when the client sends
// a message other than HandshakeRequest first.
func TestHandshakeInvalidSequence(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalOutput)

	serverConn, clientConn := createTestConnections()
	defer serverConn.Close()
	defer clientConn.Close()

	serverName := "TestServer-HS-BadSeq"
	server := &Server{conn: serverConn, serverName: serverName}

	var wg sync.WaitGroup
	var serverErr error
	var clientErr error

	wg.Add(2)

	// Server Goroutine: Runs the handshake logic
	go func() {
		defer wg.Done()
		_, serverErr = server.handleHandshake()
	}()

	// Client Simulation Goroutine: Sends an invalid first message
	go func() {
		defer wg.Done()
		// Send an Error message instead of HandshakeRequest
		invalidFirstMessagePayload := ErrorPayload{Code: "ClientError", Message: "Sending wrong message"}
		err := clientConn.SendMessage(MessageTypeError, invalidFirstMessagePayload)
		if err != nil {
			// Record the error if sending fails, though the main check is serverErr
			clientErr = fmt.Errorf("simulated client failed to send invalid message: %w", err)
			return
		}
		// Do not attempt to receive. The server should error out on its side.
		// The pipe will likely be closed by the server goroutine exiting.
	}()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Handshake invalid sequence test timed out")
	}

	// Check results: Expect server error about wrong message type
	if serverErr == nil {
		t.Error("Server handshake should have failed (invalid sequence), but succeeded")
	} else if !strings.Contains(serverErr.Error(), "expected HandshakeRequest, got") {
		t.Errorf("Server error message unexpected for invalid sequence: %v", serverErr)
	}

	// We don't strictly need to check clientErr here, as the primary goal
	// is to ensure the server correctly identifies the invalid sequence and errors out.
	// The client's state after the server errors is less critical for this specific test.
	if clientErr != nil {
		t.Logf("Client simulation encountered an error as expected: %v", clientErr) // Log for info, but don't fail test if nil
	}
}

// TestHandshakeMalformedPayload tests the server response when the client sends
// a HandshakeRequest with an invalid payload structure.
func TestHandshakeMalformedPayload(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalOutput)

	serverConn, clientConn := createTestConnections()
	defer serverConn.Close()
	defer clientConn.Close()

	serverName := "TestServer-HS-BadPayload"
	server := &Server{conn: serverConn, serverName: serverName}

	var wg sync.WaitGroup
	var serverErr error
	var clientErr error

	wg.Add(2)

	// Server Goroutine: Runs the handshake logic
	go func() {
		defer wg.Done()
		_, serverErr = server.handleHandshake()
	}()

	// Client Simulation Goroutine: Sends HandshakeRequest with bad payload
	go func() {
		defer wg.Done()
		// Send HandshakeRequest message type, but with a completely wrong payload struct
		malformedPayload := map[string]int{"wrong": 123} // Not HandshakeRequestPayload
		err := clientConn.SendMessage(MessageTypeHandshakeRequest, malformedPayload)
		if err != nil {
			// Record the error if sending fails, though the main check is serverErr
			clientErr = fmt.Errorf("simulated client failed to send malformed payload: %w", err)
			return
		}
		// Do not attempt to receive. The server should error out on its side.
	}()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Handshake malformed payload test timed out")
	}
	// Check results: Expect server error about missing required field
	if serverErr == nil {
		t.Error("Server handshake should have failed (malformed payload), but succeeded")
	} else if !strings.Contains(serverErr.Error(), "malformed HandshakeRequest payload: missing or empty supported_protocol_versions") {
		t.Errorf("Server error message unexpected for malformed payload: %v", serverErr)
	}

	// We don't strictly need to check clientErr here, as the primary goal
	// is to ensure the server correctly identifies the malformed payload and errors out.
	if clientErr != nil {
		t.Logf("Client simulation encountered an error as expected: %v", clientErr) // Log for info, but don't fail test if nil
	}
}

// TODO: Add more tests:
// - Connection closing during handshake
