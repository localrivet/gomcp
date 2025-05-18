package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

func TestNewTransport(t *testing.T) {
	// Test server mode
	serverTransport := NewTransport(":8080")
	if serverTransport.isClient {
		t.Errorf("Expected server mode for address ':8080', got client mode")
	}

	// Test client mode
	clientTransport := NewTransport("ws://localhost:8080")
	if !clientTransport.isClient {
		t.Errorf("Expected client mode for address 'ws://localhost:8080', got server mode")
	}
}

func TestServerInitializeAndStart(t *testing.T) {
	transport := NewTransport(":0") // Use random port

	// Initialize should succeed
	err := transport.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Start should succeed
	err = transport.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Clean up
	err = transport.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestEchoHandler(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Upgrade the connection to WebSocket
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			t.Fatalf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		// Echo back any message received
		for {
			msg, op, err := wsutil.ReadClientData(conn)
			if err != nil {
				break
			}

			err = wsutil.WriteServerMessage(conn, op, msg)
			if err != nil {
				break
			}
		}
	}))
	defer server.Close()

	// Replace http:// with ws:// in the test server URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Create client transport
	transport := NewTransport(wsURL)

	// Initialize and start
	err := transport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Instead of using message handler, we'll read directly from the transport
	// We're testing the transport's Send and Receive methods, not the handler
	testMsg := []byte("Hello, WebSocket!")

	// Send test message
	err = transport.Send(testMsg)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Add slight delay to ensure message is sent and response received
	time.Sleep(100 * time.Millisecond)

	// Receive the response
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Create a goroutine to receive the message
	respCh := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go func() {
		resp, err := transport.Receive()
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}()

	// Wait for response or timeout
	select {
	case <-ctx.Done():
		t.Fatal("Timed out waiting for response")
	case err := <-errCh:
		t.Fatalf("Error receiving message: %v", err)
	case resp := <-respCh:
		if string(resp) != string(testMsg) {
			t.Errorf("Expected response %q, got %q", testMsg, resp)
		}
	}

	// Cleanup
	err = transport.Stop()
	if err != nil {
		t.Fatalf("Failed to stop transport: %v", err)
	}
}
