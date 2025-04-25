package stdio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

// TestStdioTransportSendReceive tests basic message sending and receiving.
func TestStdioTransportSendReceive(t *testing.T) {
	// Create pipes to simulate stdin/stdout connection
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()

	// Create transports
	opts := types.TransportOptions{Logger: NewNilLogger()}
	serverTransport := NewStdioTransportWithReadWriter(serverReader, serverWriter, opts)
	clientTransport := NewStdioTransportWithReadWriter(clientReader, clientWriter, opts)

	// Use WaitGroup to wait for goroutines
	var wg sync.WaitGroup
	wg.Add(2) // One for server read, one for client read

	// Test message
	testMsg := map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": "test"}
	testMsgBytes, _ := json.Marshal(testMsg)

	var receivedFromServer []byte
	var receivedFromClient []byte
	var errFromServer error
	var errFromClient error

	// Server Read Goroutine
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		receivedFromServer, errFromServer = serverTransport.ReceiveWithContext(ctx)
	}()

	// Client Read Goroutine
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		receivedFromClient, errFromClient = clientTransport.ReceiveWithContext(ctx)
	}()

	// Client Send -> Server Receive
	t.Log("Client sending message...")
	if err := clientTransport.Send(testMsgBytes); err != nil {
		t.Fatalf("Client failed to send message: %v", err)
	}
	t.Log("Client message sent.")

	// Server Send -> Client Receive
	t.Log("Server sending message...")
	if err := serverTransport.Send(testMsgBytes); err != nil {
		t.Fatalf("Server failed to send message: %v", err)
	}
	t.Log("Server message sent.")

	// Wait for reads to complete
	wg.Wait()

	// Check server receive result
	if errFromServer != nil {
		t.Fatalf("Server failed to receive message: %v", errFromServer)
	}
	// Trim trailing newline added by Send before comparing
	if !bytes.Equal(bytes.TrimSpace(receivedFromServer), testMsgBytes) {
		t.Errorf("Server received wrong message.\nExpected: %s\nGot:      %s", string(testMsgBytes), string(receivedFromServer))
	} else {
		t.Log("Server received message correctly.")
	}

	// Check client receive result
	if errFromClient != nil {
		t.Fatalf("Client failed to receive message: %v", errFromClient)
	}
	// Trim trailing newline added by Send before comparing
	if !bytes.Equal(bytes.TrimSpace(receivedFromClient), testMsgBytes) {
		t.Errorf("Client received wrong message.\nExpected: %s\nGot:      %s", string(testMsgBytes), string(receivedFromClient))
	} else {
		t.Log("Client received message correctly.")
	}

	// Close transports (closing the writers of the pipes is sufficient)
	clientWriter.Close()
	serverWriter.Close()
	// Allow goroutines to potentially exit cleanly if blocked on read after close
	time.Sleep(50 * time.Millisecond)
}
