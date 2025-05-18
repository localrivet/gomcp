// Package sse provides a Server-Sent Events implementation of the MCP transport.
//
// This package implements the Transport interface using Server-Sent Events (SSE),
// suitable for applications requiring server-to-client real-time updates.
package sse

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/localrivet/gomcp/transport"
)

// DefaultShutdownTimeout is the default timeout for graceful shutdown
const DefaultShutdownTimeout = 10 * time.Second

// Transport implements the transport.Transport interface for SSE
type Transport struct {
	transport.BaseTransport
	addr     string
	server   *http.Server
	isClient bool

	// For server mode
	clients   map[chan []byte]bool
	clientsMu sync.Mutex

	// For client mode
	url       string
	client    *http.Client
	readCh    chan []byte
	errCh     chan error
	doneCh    chan struct{}
	connected bool
	connMu    sync.Mutex
}

// NewTransport creates a new SSE transport
func NewTransport(addr string) *Transport {
	isClient := strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://")

	t := &Transport{
		addr:     addr,
		isClient: isClient,
	}

	if isClient {
		t.url = addr
		t.client = &http.Client{}
		t.readCh = make(chan []byte, 100)
		t.errCh = make(chan error, 1)
		t.doneCh = make(chan struct{})
	} else {
		t.clients = make(map[chan []byte]bool)
	}

	return t
}

// Initialize initializes the transport
func (t *Transport) Initialize() error {
	if t.isClient {
		// Client mode - nothing to initialize yet
		// We'll connect when Start is called
		return nil
	}

	// Server mode - nothing to initialize yet
	// We'll start the HTTP server when Start is called
	return nil
}

// Start starts the transport
func (t *Transport) Start() error {
	if t.isClient {
		// Start the client connection
		go t.startClientConnection()
		return nil
	}

	// Start the server
	mux := http.NewServeMux()
	mux.HandleFunc("/", t.handleSSERequest)

	t.server = &http.Server{
		Addr:    t.addr,
		Handler: mux,
	}

	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error
		}
	}()

	return nil
}

// Stop stops the transport
func (t *Transport) Stop() error {
	if t.isClient {
		close(t.doneCh)
		t.connMu.Lock()
		t.connected = false
		t.connMu.Unlock()
		return nil
	}

	// Server mode
	ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer cancel()

	// Notify all clients that we're shutting down
	t.clientsMu.Lock()
	for clientCh := range t.clients {
		close(clientCh)
	}
	t.clients = make(map[chan []byte]bool)
	t.clientsMu.Unlock()

	// Shutdown the server
	return t.server.Shutdown(ctx)
}

// Send sends a message
func (t *Transport) Send(message []byte) error {
	if t.isClient {
		return errors.New("send is not supported in client mode for SSE transport")
	}

	// Server mode - send to all clients
	t.clientsMu.Lock()
	defer t.clientsMu.Unlock()

	for clientCh := range t.clients {
		select {
		case clientCh <- message:
			// Message sent
		default:
			// Channel full, remove the client
			close(clientCh)
			delete(t.clients, clientCh)
		}
	}

	return nil
}

// Receive receives a message (client mode only)
func (t *Transport) Receive() ([]byte, error) {
	if !t.isClient {
		return nil, errors.New("receive is only supported in client mode")
	}

	t.connMu.Lock()
	connected := t.connected
	t.connMu.Unlock()

	if !connected {
		return nil, errors.New("not connected to server")
	}

	select {
	case msg := <-t.readCh:
		return msg, nil
	case err := <-t.errCh:
		return nil, err
	case <-t.doneCh:
		return nil, errors.New("transport closed")
	}
}

// handleSSERequest handles incoming SSE connection requests
func (t *Transport) handleSSERequest(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a channel for this client
	clientCh := make(chan []byte, 10)

	// Register the client
	t.clientsMu.Lock()
	t.clients[clientCh] = true
	t.clientsMu.Unlock()

	// Clean up when the client disconnects
	defer func() {
		t.clientsMu.Lock()
		delete(t.clients, clientCh)
		close(clientCh)
		t.clientsMu.Unlock()
	}()

	// Ensure the connection stays open with a flush
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial comment to establish connection
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	// Handle client disconnect
	clientClosed := r.Context().Done()

	// Send events to the client
	for {
		select {
		case <-clientClosed:
			// Client disconnected
			return
		case msg, ok := <-clientCh:
			if !ok {
				// Channel closed
				return
			}

			// Format the message as an SSE event
			fmt.Fprintf(w, "data: %s\n\n", string(msg))
			flusher.Flush()
		}
	}
}

// startClientConnection establishes and maintains the SSE connection
func (t *Transport) startClientConnection() {
	defer func() {
		t.connMu.Lock()
		t.connected = false
		t.connMu.Unlock()
	}()

	for {
		select {
		case <-t.doneCh:
			return
		default:
			// Attempt to connect or reconnect
			if err := t.connectToSSE(); err != nil {
				select {
				case t.errCh <- err:
					// Error sent
				default:
					// Error channel full, discard
				}

				// Wait before reconnecting
				select {
				case <-time.After(5 * time.Second):
					// Try again
				case <-t.doneCh:
					return
				}
			}
		}
	}
}

// connectToSSE establishes a connection to the SSE server
func (t *Transport) connectToSSE() error {
	req, err := http.NewRequest("GET", t.url, nil)
	if err != nil {
		return err
	}

	// Set headers for SSE request
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Context that can be canceled when Stop is called
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register cancel function to be called when doneCh is closed
	go func() {
		select {
		case <-t.doneCh:
			cancel()
		case <-ctx.Done():
			// Context already canceled
			return
		}
	}()

	req = req.WithContext(ctx)

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Set connected status
	t.connMu.Lock()
	t.connected = true
	t.connMu.Unlock()

	// Parse SSE events
	reader := bufio.NewReader(resp.Body)
	var buf bytes.Buffer

	// Send an initial empty message to notify that the connection is established
	select {
	case t.readCh <- []byte("connection established"):
		// Message sent
	default:
		// Channel full, continue without blocking
	}

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = bytes.TrimSpace(line)

		// Skip comment lines
		if bytes.HasPrefix(line, []byte(":")) {
			continue
		}

		// Handle data lines
		if bytes.HasPrefix(line, []byte("data:")) {
			// Extract the data
			data := bytes.TrimPrefix(line, []byte("data:"))
			data = bytes.TrimSpace(data)
			buf.Write(data)
		} else if len(line) == 0 && buf.Len() > 0 {
			// Empty line indicates end of event
			msg := buf.Bytes()

			// Process the message
			response, err := t.HandleMessage(msg)
			if err != nil {
				// Log error but continue processing
				continue
			}

			if response != nil {
				// We can't send responses in SSE client mode,
				// but we can deliver them via the readCh
				select {
				case t.readCh <- response:
					// Message sent
				default:
					// Channel full, discard oldest message
					<-t.readCh
					t.readCh <- response
				}
			} else {
				// If there's no response from the handler, still send the original message
				// to the read channel so clients can receive it
				select {
				case t.readCh <- msg:
					// Message sent
				default:
					// Channel full, discard oldest message
					<-t.readCh
					t.readCh <- msg
				}
			}

			// Reset buffer for next event
			buf.Reset()
		}
	}

	return errors.New("SSE connection closed")
}
