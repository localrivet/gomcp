package tcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

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

// TestTCPTransportSendReceive tests basic message sending and receiving over TCP.
func TestTCPTransportSendReceive(t *testing.T) {
	// Start TCP Listener
	listener, err := net.Listen("tcp", "127.0.0.1:0") // Use port 0 for random available port
	if err != nil {
		t.Fatalf("Failed to start TCP listener: %v", err)
	}
	defer listener.Close()
	listenerAddr := listener.Addr().String()
	t.Logf("TCP Listener started on %s", listenerAddr)

	var wg sync.WaitGroup
	wg.Add(2) // For server accept and client dial + send/receive logic

	var serverTransport types.Transport
	var clientTransport types.Transport
	var serverErr error
	var clientErr error

	// Server Accept Goroutine
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
		opts := types.TransportOptions{Logger: NewNilLogger()}
		serverTransport = NewTCPTransport(conn, opts)

		// Server Receive
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		receivedBytes, err := serverTransport.ReceiveWithContext(ctx)
		if err != nil {
			serverErr = err
			t.Errorf("Server failed to receive message: %v", err)
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
		if err := serverTransport.Send(testMsgServerBytes); err != nil {
			serverErr = err
			t.Errorf("Server failed to send message: %v", err)
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
		opts := types.TransportOptions{Logger: NewNilLogger()}
		clientTransport = NewTCPTransport(conn, opts)

		// Client Send
		testMsgClient := map[string]interface{}{"jsonrpc": "2.0", "id": "client-to-server", "method": "test"}
		testMsgClientBytes, _ := json.Marshal(testMsgClient)
		t.Log("Client sending message...")
		if err := clientTransport.Send(testMsgClientBytes); err != nil {
			clientErr = err
			t.Errorf("Client failed to send message: %v", err)
			return
		}
		t.Log("Client message sent.")

		// Client Receive
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		receivedBytes, err := clientTransport.ReceiveWithContext(ctx)
		if err != nil {
			clientErr = err
			t.Errorf("Client failed to receive message: %v", err)
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
		t.Fatalf("Server goroutine encountered error: %v", serverErr)
	}
	if clientErr != nil {
		t.Fatalf("Client goroutine encountered error: %v", clientErr)
	}
}
