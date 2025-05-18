// Package ws provides a WebSocket implementation of the MCP transport.
//
// This package implements the Transport interface using WebSockets,
// suitable for web applications requiring bidirectional communication.
package ws

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/localrivet/gomcp/transport"
)

// DefaultShutdownTimeout is the default timeout for graceful shutdown
const DefaultShutdownTimeout = 10 * time.Second

// Transport implements the transport.Transport interface for WebSocket
type Transport struct {
	transport.BaseTransport
	addr     string
	server   *http.Server
	conns    map[net.Conn]bool
	connsMu  sync.Mutex
	isClient bool

	// For client mode
	clientConn net.Conn
	clientMu   sync.Mutex
	readCh     chan []byte
	errCh      chan error
	doneCh     chan struct{}
}

// NewTransport creates a new WebSocket transport
func NewTransport(addr string) *Transport {
	// Determine if we're in client or server mode based on the address
	isClient := strings.HasPrefix(addr, "ws://") || strings.HasPrefix(addr, "wss://")

	t := &Transport{
		addr:     addr,
		conns:    make(map[net.Conn]bool),
		isClient: isClient,
	}

	if isClient {
		t.readCh = make(chan []byte, 100)
		t.errCh = make(chan error, 1)
		t.doneCh = make(chan struct{})
	}

	return t
}

// Initialize initializes the transport
func (t *Transport) Initialize() error {
	if t.isClient {
		// Connect to the server
		ctx := context.Background()
		conn, _, _, err := ws.Dial(ctx, t.addr)
		if err != nil {
			return err
		}

		t.clientMu.Lock()
		t.clientConn = conn
		t.clientMu.Unlock()

		// Start reading messages
		go t.readClientMessages()
	}
	return nil
}

// Start starts the transport
func (t *Transport) Start() error {
	if t.isClient {
		// Client mode already started in Initialize
		return nil
	}

	// Server mode
	mux := http.NewServeMux()
	mux.HandleFunc("/", t.handleWebSocketRequest)

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

		t.clientMu.Lock()
		defer t.clientMu.Unlock()

		if t.clientConn != nil {
			return t.clientConn.Close()
		}
		return nil
	}

	// Server mode
	ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer cancel()

	// Close all connections
	t.connsMu.Lock()
	for conn := range t.conns {
		conn.Close()
	}
	t.conns = make(map[net.Conn]bool)
	t.connsMu.Unlock()

	// Shutdown the server
	return t.server.Shutdown(ctx)
}

// Send sends a message
func (t *Transport) Send(message []byte) error {
	if t.isClient {
		// Client mode - send to server
		t.clientMu.Lock()
		defer t.clientMu.Unlock()

		if t.clientConn == nil {
			return errors.New("not connected to server")
		}

		return wsutil.WriteClientMessage(t.clientConn, ws.OpText, message)
	}

	// Server mode - send to all clients
	t.connsMu.Lock()
	defer t.connsMu.Unlock()

	var lastErr error
	for conn := range t.conns {
		if err := wsutil.WriteServerMessage(conn, ws.OpText, message); err != nil {
			// Note the error but continue trying to send to other clients
			lastErr = err
			// Remove failed connection
			conn.Close()
			delete(t.conns, conn)
		}
	}

	return lastErr
}

// Receive receives a message (client mode only)
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

// handleWebSocketRequest handles incoming WebSocket connection requests
func (t *Transport) handleWebSocketRequest(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to WebSocket
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		return
	}

	// Register the connection
	t.connsMu.Lock()
	t.conns[conn] = true
	t.connsMu.Unlock()

	// Handle incoming messages in a goroutine
	go t.handleServerConnection(conn)
}

// handleServerConnection processes messages from a client connection
func (t *Transport) handleServerConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		t.connsMu.Lock()
		delete(t.conns, conn)
		t.connsMu.Unlock()
	}()

	for {
		msg, op, err := wsutil.ReadClientData(conn)
		if err != nil {
			// Connection closed or error
			return
		}

		if op == ws.OpClose {
			return
		}

		if op == ws.OpText || op == ws.OpBinary {
			// Process the message
			response, err := t.HandleMessage(msg)
			if err != nil {
				// Log error
				continue
			}

			if response != nil {
				// Send response back to this specific client
				if err := wsutil.WriteServerMessage(conn, ws.OpText, response); err != nil {
					// Log error
					return
				}
			}
		}
	}
}

// readClientMessages continuously reads messages from the server in client mode
func (t *Transport) readClientMessages() {
	defer func() {
		t.clientMu.Lock()
		if t.clientConn != nil {
			t.clientConn.Close()
			t.clientConn = nil
		}
		t.clientMu.Unlock()
	}()

	for {
		select {
		case <-t.doneCh:
			return
		default:
			t.clientMu.Lock()
			conn := t.clientConn
			t.clientMu.Unlock()

			if conn == nil {
				t.errCh <- errors.New("not connected to server")
				return
			}

			msg, op, err := wsutil.ReadServerData(conn)
			if err != nil {
				t.errCh <- err
				return
			}

			if op == ws.OpClose {
				t.errCh <- errors.New("connection closed by server")
				return
			}

			if op == ws.OpText || op == ws.OpBinary {
				select {
				case t.readCh <- msg:
					// Message sent to channel
				default:
					// Channel full, discard oldest message
					<-t.readCh
					t.readCh <- msg
				}
			}
		}
	}
}
