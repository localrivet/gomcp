// Package unix provides a Unix Domain Socket implementation of the MCP transport.
//
// This package implements the Transport interface using Unix Domain Sockets,
// suitable for high-performance local inter-process communication between
// processes running on the same machine.
package unix

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/localrivet/gomcp/transport"
)

// DefaultShutdownTimeout is the default timeout for graceful shutdown.
// This is used when closing connections to avoid abruptly terminating active communications.
const DefaultShutdownTimeout = 10 * time.Second

// DefaultSocketPermissions is the default file permissions for the socket file.
// 0600 means the socket is readable and writable only by the owner.
const DefaultSocketPermissions = 0600

// Transport implements the transport.Transport interface for Unix Domain Sockets.
// It supports both server and client modes for local inter-process communication.
type Transport struct {
	transport.BaseTransport
	socketPath       string
	listener         net.Listener
	conns            map[net.Conn]bool
	connsMu          sync.Mutex
	isClient         bool
	permissions      os.FileMode
	socketBufferSize int

	// For client mode
	clientConn net.Conn
	clientMu   sync.Mutex
	readCh     chan []byte
	errCh      chan error
	doneCh     chan struct{}
}

// UnixSocketOption is a function that configures a Transport
// These options allow customizing the behavior of the Unix Domain Socket transport.
type UnixSocketOption func(*Transport)

// WithPermissions sets the file permissions for the socket file.
// This option is used to control who can connect to the socket.
func WithPermissions(perm os.FileMode) UnixSocketOption {
	return func(t *Transport) {
		t.permissions = perm
	}
}

// WithBufferSize sets the buffer size for socket IO operations.
// Larger buffer sizes can improve performance for larger messages.
func WithBufferSize(size int) UnixSocketOption {
	return func(t *Transport) {
		t.socketBufferSize = size
	}
}

// NewTransport creates a new Unix Domain Socket transport.
//
// Parameters:
//   - socketPath: The path to the Unix domain socket file. Using an absolute path
//     or a path with "./" or "../" prefix creates a server-mode transport.
//     Otherwise, it creates a client-mode transport.
//   - options: Optional configuration settings (permissions, buffer size, etc.)
//
// Example:
//
//	// Server mode
//	serverTransport := unix.NewTransport("/tmp/mcp.sock")
//
//	// Client mode
//	clientTransport := unix.NewTransport("mcp.sock")
//
//	// With options
//	transport := unix.NewTransport("/tmp/mcp.sock",
//	    unix.WithPermissions(0644),
//	    unix.WithBufferSize(8192))
func NewTransport(socketPath string, options ...UnixSocketOption) *Transport {
	// Determine if we're in client or server mode
	isClient := !strings.HasPrefix(socketPath, "/") && !strings.HasPrefix(socketPath, "./") && !strings.HasPrefix(socketPath, "../")

	t := &Transport{
		socketPath:       socketPath,
		conns:            make(map[net.Conn]bool),
		isClient:         isClient,
		permissions:      DefaultSocketPermissions,
		socketBufferSize: 4096,
	}

	if isClient {
		t.readCh = make(chan []byte, 100)
		t.errCh = make(chan error, 1)
		t.doneCh = make(chan struct{})
	}

	// Apply options
	for _, option := range options {
		option(t)
	}

	return t
}

// Initialize initializes the transport.
// For client mode, it establishes the connection.
// For server mode, it ensures the directory for the socket exists.
func (t *Transport) Initialize() error {
	if t.isClient {
		// In client mode, connect to the server
		return t.connectToServer()
	}

	// In server mode, ensure the directory for the socket exists
	socketDir := filepath.Dir(t.socketPath)
	if socketDir != "." {
		if err := os.MkdirAll(socketDir, 0755); err != nil {
			return fmt.Errorf("failed to create socket directory: %w", err)
		}
	}

	return nil
}

// connectToServer establishes a connection to the Unix socket server.
// This is an internal function used in client mode.
func (t *Transport) connectToServer() error {
	t.clientMu.Lock()
	defer t.clientMu.Unlock()

	// Close existing connection if any
	if t.clientConn != nil {
		t.clientConn.Close()
	}

	// Connect to the server
	conn, err := net.Dial("unix", t.socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to unix socket %s: %w", t.socketPath, err)
	}

	t.clientConn = conn

	// Start reading messages
	go t.readClientMessages()

	return nil
}

// Start starts the transport.
// For client mode, this is a no-op as the connection is established in Initialize.
// For server mode, it creates and binds to the Unix domain socket.
func (t *Transport) Start() error {
	if t.isClient {
		// Client mode already started in Initialize
		return nil
	}

	// Server mode - remove the socket file if it already exists
	if _, err := os.Stat(t.socketPath); err == nil {
		if err := os.Remove(t.socketPath); err != nil {
			return fmt.Errorf("failed to remove existing socket file: %w", err)
		}
	}

	// Create the listener
	listener, err := net.Listen("unix", t.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket: %w", err)
	}
	t.listener = listener

	// Set file permissions
	if err := os.Chmod(t.socketPath, t.permissions); err != nil {
		// Close listener and clean up on error
		t.listener.Close()
		os.Remove(t.socketPath)
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	// Accept connections in a goroutine
	go t.acceptConnections()

	return nil
}

// acceptConnections accepts incoming connections and handles them.
// This is an internal function used in server mode to manage client connections.
func (t *Transport) acceptConnections() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			// Check if the listener was closed
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			// Log error and continue
			// Log error and continue
			fmt.Printf("Unix Socket Transport: Error accepting connection: %v\n", err)
			continue
		}

		// Register the connection
		t.connsMu.Lock()
		t.conns[conn] = true
		t.connsMu.Unlock()

		// Handle the connection in a goroutine
		go t.handleServerConnection(conn)
	}
}

// handleServerConnection processes messages from a client connection.
// This is an internal function used in server mode to handle communication
// with each connected client in its own goroutine.
func (t *Transport) handleServerConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		t.connsMu.Lock()
		delete(t.conns, conn)
		t.connsMu.Unlock()
	}()

	reader := bufio.NewReaderSize(conn, t.socketBufferSize)

	for {
		// Read message length (JSON-RPC messages are newline-delimited)
		message, err := reader.ReadBytes('\n')
		if err != nil {
			// Connection closed or error
			if err != io.EOF {
				fmt.Printf("Unix Socket Transport: Error reading from connection: %v\n", err)
			}
			return
		}

		// Remove trailing newline
		message = message[:len(message)-1]

		// Process the message
		response, err := t.HandleMessage(message)
		if err != nil {
			// Log error
			fmt.Printf("Unix Socket Transport: Error handling message: %v\n", err)
			// Try to send error response if possible
			errorResp := createErrorResponse(message, err)
			if errorResp != nil {
				conn.Write(append(errorResp, '\n'))
			}
			continue
		}

		if response != nil {
			// Send response back to the client
			_, err = conn.Write(append(response, '\n'))
			if err != nil {
				fmt.Printf("Unix Socket Transport: Error writing response: %v\n", err)
				return
			}
		}
	}
}

// createErrorResponse creates a JSON-RPC error response for error situations.
// This helper function constructs a properly formatted JSON-RPC error response
// based on the original request and error that occurred.
func createErrorResponse(request []byte, err error) []byte {
	// Try to parse the request to get the ID
	var req struct {
		ID      interface{} `json:"id"`
		JSONRPC string      `json:"jsonrpc"`
	}

	if err := json.Unmarshal(request, &req); err != nil {
		// Couldn't parse the request, create a response without an ID
		req.ID = nil
		req.JSONRPC = "2.0"
	}

	// Create error response
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"error": map[string]interface{}{
			"code":    -32000,
			"message": "Server error",
			"data":    err.Error(),
		},
	}

	// Marshal to JSON
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return nil
	}

	return respBytes
}

// Stop stops the transport.
// For client mode, it closes the connection to the server.
// For server mode, it closes the listener and all client connections,
// then removes the socket file.
func (t *Transport) Stop() error {
	if t.isClient {
		// Client mode
		close(t.doneCh)

		t.clientMu.Lock()
		defer t.clientMu.Unlock()

		if t.clientConn != nil {
			return t.clientConn.Close()
		}
		return nil
	}

	// Server mode
	if t.listener != nil {
		// Close the listener
		if err := t.listener.Close(); err != nil {
			return err
		}

		// Close all connections
		t.connsMu.Lock()
		for conn := range t.conns {
			conn.Close()
		}
		t.conns = make(map[net.Conn]bool)
		t.connsMu.Unlock()

		// Remove the socket file
		os.Remove(t.socketPath)
	}

	return nil
}

// Send sends a message.
// For client mode, it sends the message to the server.
// For server mode, it broadcasts the message to all connected clients.
func (t *Transport) Send(message []byte) error {
	if t.isClient {
		// Client mode - send to server
		t.clientMu.Lock()
		defer t.clientMu.Unlock()

		if t.clientConn == nil {
			return errors.New("not connected to server")
		}

		// Add newline as message delimiter
		message = append(message, '\n')
		_, err := t.clientConn.Write(message)
		return err
	}

	// Server mode - send to all clients
	t.connsMu.Lock()
	defer t.connsMu.Unlock()

	var lastErr error
	message = append(message, '\n')

	for conn := range t.conns {
		_, err := conn.Write(message)
		if err != nil {
			// Note the error but continue trying to send to other clients
			lastErr = err
			// Remove failed connection
			conn.Close()
			delete(t.conns, conn)
		}
	}

	return lastErr
}

// Receive receives a message (client mode only).
// This method is used in client mode to receive responses from the server.
// In server mode, this method returns an error as server-side message handling
// is done via the message handler callback.
func (t *Transport) Receive() ([]byte, error) {
	if !t.isClient {
		return nil, errors.New("receive is only supported in client mode")
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

// readClientMessages continuously reads messages from the server in client mode.
// This is an internal function running in a goroutine that reads incoming messages
// from the server and places them in a channel for the Receive method to consume.
func (t *Transport) readClientMessages() {
	defer func() {
		t.clientMu.Lock()
		if t.clientConn != nil {
			t.clientConn.Close()
			t.clientConn = nil
		}
		t.clientMu.Unlock()
	}()

	reader := bufio.NewReaderSize(t.clientConn, t.socketBufferSize)

	for {
		select {
		case <-t.doneCh:
			return
		default:
			// Read message (JSON-RPC messages are newline-delimited)
			message, err := reader.ReadBytes('\n')
			if err != nil {
				// Connection closed or error
				if err != io.EOF {
					t.errCh <- fmt.Errorf("error reading from server: %w", err)
				} else {
					t.errCh <- errors.New("connection closed by server")
				}
				return
			}

			// Remove trailing newline
			message = message[:len(message)-1]

			select {
			case t.readCh <- message:
				// Message sent to channel
			default:
				// Channel full, discard oldest message
				<-t.readCh
				t.readCh <- message
			}
		}
	}
}
