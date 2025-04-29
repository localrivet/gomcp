package tcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors" // Import errors package
	"net"
	"sync"
	"testing"
	"time"

	"github.com/localrivet/gomcp/types"
)

// TestTCPTransportSendReceive tests basic message exchange over TCP.
func TestTCPTransportSendReceive(t *testing.T) {
	// Start TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0") // Listen on random available port
	if err != nil {
		t.Fatalf("Failed to start TCP listener: %v", err)
	}
	defer listener.Close()
	listenerAddr := listener.Addr().String()
	t.Logf("TCP Listener started on %s", listenerAddr)

	var wg sync.WaitGroup
	wg.Add(2) // One for server, one for client

	var serverErr error
	var clientErr error
	var serverTransport types.Transport
	var clientTransport types.Transport

	// Server Accept and Send/Receive Goroutine
	go func() {
		defer wg.Done()
		conn, err := listener.Accept()
		if err != nil {
			serverErr = err
			t.Errorf("Listener failed to accept connection: %v", err)
			return
		}
		defer conn.Close()
		t.Logf("Listener accepted connection from %s", conn.RemoteAddr())
		opts := types.TransportOptions{Logger: NewNilLogger()} // Use local NilLogger for test
		serverTransport = NewTCPTransport(conn, opts)

		// Server Receive
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// Use Receive with context
		receivedBytes, err := serverTransport.Receive(ctx)
		if err != nil {
			serverErr = err
			// Check for context errors specifically
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				t.Logf("Server Receive context error (expected on close): %v", err)
			} else {
				t.Errorf("Server failed to receive message: %v", err)
			}
			return
		}
		// Verify received message (sent by client below)
		testMsg := map[string]interface{}{"jsonrpc": "2.0", "id": "client-to-server", "method": "test"}
		testMsgBytes, _ := json.Marshal(testMsg)
		if !bytes.Equal(bytes.TrimSpace(receivedBytes), testMsgBytes) {
			serverErr = err
			t.Errorf("Server received wrong message.\nExpected: %s\nGot:      %s", string(testMsgBytes), string(receivedBytes))
			return
		}
		t.Log("Server received message correctly.")

		// Server Send
		testMsgServer := map[string]interface{}{"jsonrpc": "2.0", "id": "server-to-client", "method": "test"}
		testMsgServerBytes, _ := json.Marshal(testMsgServer)
		t.Log("Server sending message...")
		// Send message from server to client, passing context
		if err := serverTransport.Send(ctx, testMsgServerBytes); err != nil {
			serverErr = err
			t.Errorf("Server failed to send message: %v", err)
			return // Exit goroutine on send error
		}
		t.Log("Server message sent.")
	}()

	// Client Dial and Send/Receive Goroutine
	go func() {
		defer wg.Done()
		conn, err := net.DialTimeout("tcp", listenerAddr, 1*time.Second)
		if err != nil {
			clientErr = err
			t.Errorf("Client failed to dial listener: %v", err)
			return
		}
		defer conn.Close()
		t.Logf("Client connected to %s", listenerAddr)
		opts := types.TransportOptions{Logger: NewNilLogger()} // Use local NilLogger for test
		clientTransport = NewTCPTransport(conn, opts)

		// Client Send
		testMsgClient := map[string]interface{}{"jsonrpc": "2.0", "id": "client-to-server", "method": "test"}
		testMsgClientBytes, _ := json.Marshal(testMsgClient)
		t.Log("Client sending message...")
		// Send message from client to server, passing context
		// Use a fresh context for send
		sendCtx, sendCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer sendCancel()
		if err := clientTransport.Send(sendCtx, testMsgClientBytes); err != nil {
			clientErr = err
			t.Errorf("Client failed to send message: %v", err)
			return // Exit goroutine on send error
		}
		t.Log("Client message sent.")

		// Client Receive
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// Use Receive with context
		receivedBytes, err := clientTransport.Receive(ctx)
		if err != nil {
			clientErr = err
			// Check for context errors specifically
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				t.Logf("Client Receive context error (expected on close): %v", err)
			} else {
				t.Errorf("Client failed to receive message: %v", err)
			}
			return
		}
		// Verify received message (sent by server above)
		testMsg := map[string]interface{}{"jsonrpc": "2.0", "id": "server-to-client", "method": "test"}
		testMsgBytes, _ := json.Marshal(testMsg)
		if !bytes.Equal(bytes.TrimSpace(receivedBytes), testMsgBytes) {
			clientErr = err
			t.Errorf("Client received wrong message.\nExpected: %s\nGot:      %s", string(testMsgBytes), string(receivedBytes))
			return
		}
		t.Log("Client received message correctly.")
	}()

	// Wait for goroutines
	wg.Wait()

	// Final check for errors reported via channel/variable
	if serverErr != nil {
		// Ignore context errors as they might be expected during shutdown
		if !errors.Is(serverErr, context.Canceled) && !errors.Is(serverErr, context.DeadlineExceeded) {
			t.Fatalf("Server goroutine encountered unexpected error: %v", serverErr)
		}
	}
	if clientErr != nil {
		if !errors.Is(clientErr, context.Canceled) && !errors.Is(clientErr, context.DeadlineExceeded) {
			t.Fatalf("Client goroutine encountered unexpected error: %v", clientErr)
		}
	}
}

// --- Mock Logger ---
type NilLogger struct{}

func (n *NilLogger) Debug(msg string, args ...interface{}) {}
func (n *NilLogger) Info(msg string, args ...interface{})  {}
func (n *NilLogger) Warn(msg string, args ...interface{})  {}
func (n *NilLogger) Error(msg string, args ...interface{}) {}
func NewNilLogger() *NilLogger                             { return &NilLogger{} }

var _ types.Logger = (*NilLogger)(nil)
