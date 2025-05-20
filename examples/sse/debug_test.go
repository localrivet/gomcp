package main

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/localrivet/gomcp/transport/sse"
)

// Test to debug SSE connection
func TestSSEConnection(t *testing.T) {
	// Start a server that just returns an endpoint event
	go func() {
		http.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
			// Set SSE headers
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			// Send an endpoint event
			fmt.Fprintf(w, "event: endpoint\ndata: http://localhost:8090/message\n\n")
			w.(http.Flusher).Flush()

			// Keep the connection open
			<-r.Context().Done()
		})

		http.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})

		http.ListenAndServe(":8090", nil)
	}()

	// Give the server time to start
	time.Sleep(1 * time.Second)

	// Create SSE transport
	transport := sse.NewTransport("http://localhost:8090")

	// Set handler to print received messages
	transport.SetMessageHandler(func(msg []byte) ([]byte, error) {
		fmt.Printf("Received message: %s\n", string(msg))
		return nil, nil
	})

	// Initialize and start
	if err := transport.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if err := transport.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for connection
	time.Sleep(2 * time.Second)

	// Check connection status
	fmt.Println("Testing connection...")
	msg := []byte(`{"jsonrpc":"2.0","method":"test","params":{},"id":1}`)
	err := transport.Send(msg)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Wait to see if any response comes back 
	time.Sleep(1 * time.Second)

	fmt.Println("Test completed")
} 