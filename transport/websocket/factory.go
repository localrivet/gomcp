// Package websocket provides a types.Transport implementation using WebSockets.
package websocket

import (
	"context"
	"fmt"
	"io"      // Added for io.ReadWriter
	"net"     // Added for net.Conn
	"net/url" // For client-side dial URL parsing

	"github.com/gobwas/ws"
	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/types"
)

// DefaultDialer is a gobwas/ws Dialer with default options.
var DefaultDialer = ws.Dialer{}

// DefaultHTTPUpgrader is a gobwas/ws HTTPUpgrader with default options.
// Use this in your HTTP handler *before* calling the Upgrade function below.
var DefaultHTTPUpgrader = ws.HTTPUpgrader{}

// Dial establishes a WebSocket connection to the given URL string and returns a WebSocketTransport.
// It uses the gobwas/ws Dialer.
func Dial(ctx context.Context, urlString string, opts types.TransportOptions) (types.Transport, error) {
	logger := opts.Logger
	if logger == nil {
		logger = logx.NewLogger("[WebSocketTransport] ")
	}

	// Parse URL (optional, ws.Dial does this too)
	_, err := url.Parse(urlString)
	if err != nil {
		logger.Error("WebSocketTransport: Invalid URL %s: %v", urlString, err)
		return nil, fmt.Errorf("invalid websocket url: %w", err)
	}

	logger.Info("WebSocketTransport: Dialing %s using gobwas/ws...", urlString)

	// Use ws.Dial - context controls the dial timeout
	conn, _, _, err := DefaultDialer.Dial(ctx, urlString) // Ignoring reader, handshake response for now
	if err != nil {
		// Note: ws.Dial returns net.ErrClosed on context deadline/cancel
		logger.Error("WebSocketTransport: Failed to dial %s: %v", urlString, err)
		return nil, fmt.Errorf("failed to dial websocket %s: %w", urlString, err)
	}

	logger.Info("WebSocketTransport: Successfully connected to %s", urlString)

	// Wrap the net.Conn in our transport, specifying client state for masking
	return NewWebSocketTransport(conn, ws.StateClientSide, opts), nil
}

// Upgrade performs the WebSocket handshake on an existing network connection (io.ReadWriter).
// It's typically used on the server side after hijacking an HTTP connection.
// Returns the handshake information or an error. The original net.Conn should be used
// to create the WebSocketTransport if successful.
func Upgrade(conn io.ReadWriter) (ws.Handshake, error) {
	// Use ws.Upgrade - this performs the handshake on the provided connection
	handshake, err := ws.Upgrade(conn)
	if err != nil {
		// Log is helpful, but error is returned for handler to decide response
		// logger.Error("WebSocketTransport: Failed to upgrade connection: %v", err)
		return handshake, fmt.Errorf("failed to upgrade to websocket: %w", err)
	}
	// logger.Info("WebSocketTransport: Successfully upgraded connection.")
	return handshake, nil
}

// --- Factory Struct (Alternative - Less common with gobwas/ws helpers) ---

// WebSocketTransportFactory can be used if a factory pattern is preferred.
type WebSocketTransportFactory struct {
	Dialer         ws.Dialer   // gobwas Dialer
	Upgrader       ws.Upgrader // gobwas Upgrader (operates on io.ReadWriter)
	DefaultOptions types.TransportOptions
}

// NewWebSocketTransportFactory creates a factory with default dialer/upgrader.
func NewWebSocketTransportFactory(opts types.TransportOptions) *WebSocketTransportFactory {
	return &WebSocketTransportFactory{
		Dialer:         ws.Dialer{},
		Upgrader:       ws.Upgrader{}, // Default Upgrader
		DefaultOptions: opts,
	}
}

// Dial uses the factory's dialer to establish a connection.
func (f *WebSocketTransportFactory) Dial(ctx context.Context, urlString string) (types.Transport, error) {
	logger := f.DefaultOptions.Logger
	if logger == nil {
		logger = logx.NewLogger("[WebSocketTransport] ")
	}
	logger.Info("WebSocketTransportFactory: Dialing %s...", urlString)
	conn, _, _, err := f.Dialer.Dial(ctx, urlString)
	if err != nil {
		logger.Error("WebSocketTransportFactory: Failed to dial %s: %v", urlString, err)
		return nil, fmt.Errorf("factory failed to dial %s: %w", urlString, err)
	}
	// Client side state
	return NewWebSocketTransport(conn, ws.StateClientSide, f.DefaultOptions), nil
}

// Upgrade uses the factory's upgrader to perform the handshake on an existing connection.
// The caller must provide the underlying io.ReadWriter (e.g., from hijacking an HTTP request).
// Returns the *same connection* wrapped in a WebSocketTransport if successful.
func (f *WebSocketTransportFactory) Upgrade(conn net.Conn) (types.Transport, error) {
	logger := f.DefaultOptions.Logger
	if logger == nil {
		logger = logx.NewLogger("[WebSocketTransport] ")
	}
	logger.Info("WebSocketTransportFactory: Upgrading connection from %s...", conn.RemoteAddr())
	_, err := f.Upgrader.Upgrade(conn) // Perform handshake
	if err != nil {
		logger.Error("WebSocketTransportFactory: Failed to upgrade connection: %v", err)
		return nil, fmt.Errorf("factory failed to upgrade connection: %w", err)
	}
	// Server side state - return the *same* conn wrapped in the transport
	return NewWebSocketTransport(conn, ws.StateServerSide, f.DefaultOptions), nil
}
