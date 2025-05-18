package sse

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func getRandomPort() string {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf(":%d", port)
}

func TestNewTransport(t *testing.T) {
	// Test server mode
	randomPort := getRandomPort()
	serverTransport := NewTransport(randomPort)
	if serverTransport.isClient {
		t.Errorf("Expected server mode for address '%s', got client mode", randomPort)
	}

	// Test client mode
	randomClientPort := 9876 // Use a specific port number for predictable testing
	clientUrl := fmt.Sprintf("http://localhost:%d", randomClientPort)
	clientTransport := NewTransport(clientUrl)
	if !clientTransport.isClient {
		t.Errorf("Expected client mode for address '%s', got server mode", clientUrl)
	}
}

func TestServerMode(t *testing.T) {
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

	// Test sending a message
	err = transport.Send([]byte("test message"))
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Clean up
	err = transport.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestClientMode(t *testing.T) {
	// Create a test SSE server
	messages := []string{"message 1", "message 2", "message 3"}
	messageIndex := 0
	serverDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected ResponseWriter to be a Flusher")
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send initial comment
		fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		// Send all messages
		for messageIndex < len(messages) {
			fmt.Fprintf(w, "data: %s\n\n", messages[messageIndex])
			flusher.Flush()
			messageIndex++
			time.Sleep(100 * time.Millisecond)
		}

		// Signal that we've sent all messages
		close(serverDone)

		// Keep the connection open with a timeout
		select {
		case <-time.After(500 * time.Millisecond):
			// Connection kept alive for enough time, now we can exit
			return
		}
	}))
	defer server.Close()

	// Create client transport
	transport := NewTransport(server.URL)

	// Set up message handler to collect received messages
	receivedMessages := make([]string, 0, len(messages))

	transport.SetMessageHandler(func(msg []byte) ([]byte, error) {
		receivedMessages = append(receivedMessages, string(msg))
		return nil, nil
	})

	// Initialize and start
	err := transport.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	err = transport.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for all messages to be sent by the server
	select {
	case <-serverDone:
		// Server is done sending messages
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for server to send messages")
	}

	// Wait a bit for client to process messages
	time.Sleep(500 * time.Millisecond)

	// Try to receive any remaining messages
	for i := 0; i < 3; i++ {
		msg, err := transport.Receive()
		if err == nil && msg != nil {
			found := false
			for _, expected := range messages {
				if string(msg) == expected {
					found = true
					break
				}
			}
			if found {
				receivedMessages = append(receivedMessages, string(msg))
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Clean up
	err = transport.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify that we got at least some messages
	if len(receivedMessages) == 0 {
		t.Errorf("Expected to receive some messages, got none")
	}
}

func TestClientSendError(t *testing.T) {
	// Create client transport
	transport := NewTransport("http://localhost:9999") // Use unlikely port

	// Send should fail in client mode
	err := transport.Send([]byte("test message"))
	if err == nil {
		t.Error("Expected Send to fail in client mode, but it succeeded")
	}
}

func TestServerReceiveError(t *testing.T) {
	// Create server transport
	transport := NewTransport(getRandomPort())

	// Receive should fail in server mode
	_, err := transport.Receive()
	if err == nil {
		t.Error("Expected Receive to fail in server mode, but it succeeded")
	}
}
