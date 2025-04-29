package stdio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/localrivet/gomcp/logx" // Use centralized logger
	"github.com/localrivet/gomcp/types"
)

// TestStdioTransportSendReceive tests basic message exchange using pipes.
func TestStdioTransportSendReceive(t *testing.T) {
	// Use pipes to simulate stdin/stdout for testing
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	var wg sync.WaitGroup
	wg.Add(2) // One for server, one for client

	var serverErr error
	var clientErr error

	// Create transports with pipes
	logger := logx.NewDefaultLogger() // Use logx logger
	serverOpts := types.TransportOptions{Logger: logger}
	clientOpts := types.TransportOptions{Logger: logger}
	serverTransport := NewStdioTransportWithReadWriter(serverReader, serverWriter, serverOpts)
	clientTransport := NewStdioTransportWithReadWriter(clientReader, clientWriter, clientOpts)

	// Server Goroutine
	go func() {
		defer wg.Done()
		defer serverWriter.Close() // Close writer when done
		defer serverReader.Close() // Close reader when done

		// Server Receive
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		receivedBytes, err := serverTransport.Receive(ctx) // Use Receive
		if err != nil {
			serverErr = err
			if !errors.Is(err, io.EOF) && !errors.Is(err, context.DeadlineExceeded) { // EOF is expected if client closes pipe
				t.Errorf("Server failed to receive message: %v", err)
			}
			return
		}
		// Verify received message (sent by client below)
		testMsg := map[string]interface{}{"jsonrpc": "2.0", "id": "client-to-server", "method": "test"}
		testMsgBytes, _ := json.Marshal(testMsg)
		testMsgBytesWithNL := append(testMsgBytes, '\n')
		if !bytes.Equal(receivedBytes, testMsgBytesWithNL) {
			serverErr = err
			t.Errorf("Server received wrong message.\nExpected: %s\nGot:      %s", string(testMsgBytesWithNL), string(receivedBytes))
			return
		}
		t.Log("Server received message correctly.")

		// Server Send
		testMsgServer := map[string]interface{}{"jsonrpc": "2.0", "id": "server-to-client", "method": "test"}
		testMsgServerBytes, _ := json.Marshal(testMsgServer)
		// testMsgServerBytesWithNL := append(testMsgServerBytes, '\n') // Send already adds newline
		t.Log("Server sending message...")
		if err := serverTransport.Send(ctx, testMsgServerBytes); err != nil { // Use Send with context
			serverErr = err
			t.Errorf("Server failed to send message: %v", err)
			return
		}
		t.Log("Server message sent.")
	}()

	// Client Goroutine
	go func() {
		defer wg.Done()
		defer clientWriter.Close() // Close writer when done
		defer clientReader.Close() // Close reader when done

		// Client Send
		testMsgClient := map[string]interface{}{"jsonrpc": "2.0", "id": "client-to-server", "method": "test"}
		testMsgClientBytes, _ := json.Marshal(testMsgClient)
		// testMsgClientBytesWithNL := append(testMsgClientBytes, '\n') // Send already adds newline
		t.Log("Client sending message...")
		ctxSend, cancelSend := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelSend()
		if err := clientTransport.Send(ctxSend, testMsgClientBytes); err != nil { // Use Send with context
			clientErr = err
			t.Errorf("Client failed to send message: %v", err)
			return
		}
		t.Log("Client message sent.")

		// Client Receive
		ctxRecv, cancelRecv := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelRecv()
		receivedBytes, err := clientTransport.Receive(ctxRecv) // Use Receive
		if err != nil {
			clientErr = err
			if !errors.Is(err, io.EOF) && !errors.Is(err, context.DeadlineExceeded) { // EOF is expected if server closes pipe
				t.Errorf("Client failed to receive message: %v", err)
			}
			return
		}
		// Verify received message (sent by server above)
		testMsg := map[string]interface{}{"jsonrpc": "2.0", "id": "server-to-client", "method": "test"}
		testMsgBytes, _ := json.Marshal(testMsg)
		testMsgBytesWithNL := append(testMsgBytes, '\n')
		if !bytes.Equal(receivedBytes, testMsgBytesWithNL) {
			clientErr = err
			t.Errorf("Client received wrong message.\nExpected: %s\nGot:      %s", string(testMsgBytesWithNL), string(receivedBytes))
			return
		}
		t.Log("Client received message correctly.")
	}()

	// Wait for goroutines
	wg.Wait()

	// Final check for unexpected errors
	if serverErr != nil && !errors.Is(serverErr, io.EOF) && !errors.Is(serverErr, context.DeadlineExceeded) && !strings.Contains(serverErr.Error(), "pipe closed") {
		t.Fatalf("Server goroutine encountered unexpected error: %v", serverErr)
	}
	if clientErr != nil && !errors.Is(clientErr, io.EOF) && !errors.Is(clientErr, context.DeadlineExceeded) && !strings.Contains(clientErr.Error(), "pipe closed") {
		t.Fatalf("Client goroutine encountered unexpected error: %v", clientErr)
	}
}
