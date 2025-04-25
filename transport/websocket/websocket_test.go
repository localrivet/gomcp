package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gobwas/ws"
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

// TestWebSocketTransportSendReceive tests basic message sending and receiving over WebSocket.
func TestWebSocketTransportSendReceive(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2) // For server handler and client logic

	var serverTransport types.Transport
	var clientTransport types.Transport
	var serverErr error
	var clientErr error

	// Test message
	testMsg := map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": "test"}
	testMsgBytes, _ := json.Marshal(testMsg)
	// Add newline as MCP expects newline-delimited JSON within the WS message frame
	testMsgBytesWithNL := append(testMsgBytes, '\n')

	// WebSocket Upgrade Handler for the server
	serverHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done() // Signal server handler completion
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			serverErr = err
			t.Errorf("Server failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()
		t.Logf("Server upgraded connection from %s", conn.RemoteAddr())

		opts := types.TransportOptions{Logger: NewNilLogger()}
		serverTransport = NewWebSocketTransport(conn, ws.StateServerSide, opts)

		// Server Receive
		ctxRecv, cancelRecv := context.WithTimeout(context.Background(), 3*time.Second) // Increased timeout
		defer cancelRecv()
		receivedBytes, err := serverTransport.ReceiveWithContext(ctxRecv)
		if err != nil {
			serverErr = err
			t.Errorf("Server failed to receive message: %v", err)
			return
		}
		// Compare with newline included, as Send adds it
		if !bytes.Equal(receivedBytes, testMsgBytesWithNL) {
			serverErr = err
			t.Errorf("Server received wrong message.\nExpected: %s\nGot:      %s", string(testMsgBytesWithNL), string(receivedBytes))
			return
		}
		t.Log("Server received message correctly.")

		// Server Send
		t.Log("Server sending message...")
		if err := serverTransport.Send(testMsgBytesWithNL); err != nil { // Send with newline
			serverErr = err
			t.Errorf("Server failed to send message: %v", err)
		}
		t.Log("Server message sent.")
	})

	// Start HTTP Test Server
	httpServer := httptest.NewServer(serverHandler)
	defer httpServer.Close()
	serverURL := "ws" + httpServer.URL[len("http"):] // Convert http:// URL to ws://
	t.Logf("Test WebSocket server listening on %s", serverURL)

	// Client Dial and Send/Receive Goroutine
	go func() {
		defer wg.Done() // Signal client logic completion
		ctxDial, cancelDial := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelDial()

		conn, _, _, err := ws.Dial(ctxDial, serverURL)
		if err != nil {
			clientErr = err
			t.Errorf("Client failed to dial server: %v", err)
			// Need to ensure wg.Done() is called even on error
			return
		}
		defer conn.Close()
		t.Logf("Client connected to %s", serverURL)

		opts := types.TransportOptions{Logger: NewNilLogger()}
		clientTransport = NewWebSocketTransport(conn, ws.StateClientSide, opts)

		// Client Send
		t.Log("Client sending message...")
		if err := clientTransport.Send(testMsgBytesWithNL); err != nil { // Send with newline
			clientErr = err
			t.Errorf("Client failed to send message: %v", err)
			return
		}
		t.Log("Client message sent.")

		// Client Receive
		ctxRecv, cancelRecv := context.WithTimeout(context.Background(), 3*time.Second) // Increased timeout
		defer cancelRecv()
		receivedBytes, err := clientTransport.ReceiveWithContext(ctxRecv)
		if err != nil {
			clientErr = err
			t.Errorf("Client failed to receive message: %v", err)
			return
		}
		// Compare with newline included
		if !bytes.Equal(receivedBytes, testMsgBytesWithNL) {
			clientErr = err
			t.Errorf("Client received wrong message.\nExpected: %s\nGot:      %s", string(testMsgBytesWithNL), string(receivedBytes))
			return
		}
		t.Log("Client received message correctly.")
	}()

	// Wait for goroutines
	wg.Wait()

	// Final check for errors reported via variables
	if serverErr != nil {
		t.Fatalf("Server handler encountered error: %v", serverErr)
	}
	if clientErr != nil {
		t.Fatalf("Client goroutine encountered error: %v", clientErr)
	}
}
