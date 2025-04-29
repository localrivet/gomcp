package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/localrivet/gomcp/types"
)

// TestWebSocketTransportSendReceive tests basic message exchange.
func TestWebSocketTransportSendReceive(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)

	var serverErr error
	var clientErr error
	var serverTransport types.Transport
	var clientTransport types.Transport

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Test Server: Received upgrade request from %s", r.RemoteAddr)
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			t.Errorf("Server failed to upgrade connection: %v", err)
			serverErr = err
			// ws.UpgradeHTTP handles writing the error response
			return
		}
		t.Logf("Server upgraded connection from %s", conn.RemoteAddr())
		defer conn.Close()

		opts := types.TransportOptions{Logger: NewNilLogger()} // Use local NilLogger
		serverTransport = NewWebSocketTransport(conn, ws.StateServerSide, opts)

		// Server Receive
		ctxRecv, cancelRecv := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelRecv()
		// Use Receive with context
		receivedBytes, err := serverTransport.Receive(ctxRecv)
		if err != nil {
			serverErr = err
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
		testMsgBytesWithNL := append(testMsgBytes, '\n')     // Add newline for comparison if needed by framing
		if !bytes.Equal(receivedBytes, testMsgBytesWithNL) { // Compare with newline if framing includes it
			serverErr = err
			t.Errorf("Server received wrong message.\nExpected: %s\nGot:      %s", string(testMsgBytesWithNL), string(receivedBytes))
			return
		}
		t.Log("Server received message correctly.")

		// Server Send
		testMsgServer := map[string]interface{}{"jsonrpc": "2.0", "id": "server-to-client", "method": "test"}
		testMsgServerBytes, _ := json.Marshal(testMsgServer)
		testMsgBytesWithNL = append(testMsgServerBytes, '\n') // Add newline for framing
		t.Log("Server sending message...")
		// Send message from server to client, passing context
		ctxSend, cancelSend := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelSend()
		if err := serverTransport.Send(ctxSend, testMsgBytesWithNL); err != nil { // Send with newline
			serverErr = err
			t.Errorf("Server failed to send message: %v", err)
			return // Exit goroutine on send error
		}
		t.Log("Server message sent.")
		wg.Done() // Signal server side completion
	}))
	defer server.Close()

	// Client Dial and Send/Receive Goroutine
	go func() {
		defer wg.Done() // Signal client side completion

		// Convert http:// to ws://
		wsURL := "ws" + server.URL[len("http"):]
		t.Logf("Client dialing %s", wsURL)

		// Dial server
		conn, _, _, err := ws.Dial(context.Background(), wsURL)
		if err != nil {
			clientErr = err
			t.Errorf("Client failed to dial server: %v", err)
			return
		}
		defer conn.Close()
		t.Logf("Client connected to %s", wsURL)
		opts := types.TransportOptions{Logger: NewNilLogger()} // Use local NilLogger
		clientTransport = NewWebSocketTransport(conn, ws.StateClientSide, opts)

		// Client Send
		testMsgClient := map[string]interface{}{"jsonrpc": "2.0", "id": "client-to-server", "method": "test"}
		testMsgClientBytes, _ := json.Marshal(testMsgClient)
		testMsgBytesWithNL := append(testMsgClientBytes, '\n') // Add newline for framing
		t.Log("Client sending message...")
		// Send message from client to server, passing context
		ctxSend, cancelSend := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelSend()
		if err := clientTransport.Send(ctxSend, testMsgBytesWithNL); err != nil { // Send with newline
			clientErr = err
			t.Errorf("Client failed to send message: %v", err)
			return // Exit goroutine on send error
		}
		t.Log("Client message sent.")

		// Client Receive
		ctxRecv, cancelRecv := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelRecv()
		// Use Receive with context
		receivedBytes, err := clientTransport.Receive(ctxRecv)
		if err != nil {
			clientErr = err
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
		testMsgBytesWithNL = append(testMsgBytes, '\n')      // Add newline for comparison
		if !bytes.Equal(receivedBytes, testMsgBytesWithNL) { // Compare with newline
			clientErr = err
			t.Errorf("Client received wrong message.\nExpected: %s\nGot:      %s", string(testMsgBytesWithNL), string(receivedBytes))
			return
		}
		t.Log("Client received message correctly.")
	}()

	// Wait for goroutines
	wg.Wait()

	// Final check for errors
	if serverErr != nil {
		if !errors.Is(serverErr, context.Canceled) && !errors.Is(serverErr, context.DeadlineExceeded) && !errors.Is(serverErr, net.ErrClosed) {
			t.Fatalf("Server goroutine encountered unexpected error: %v", serverErr)
		}
	}
	if clientErr != nil {
		if !errors.Is(clientErr, context.Canceled) && !errors.Is(clientErr, context.DeadlineExceeded) && !errors.Is(clientErr, net.ErrClosed) {
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
