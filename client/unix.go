package client

import (
	"context"
	"fmt"
	"time"

	"github.com/localrivet/gomcp/transport/unix"
)

// UnixSocketOption is a function that configures a Unix Domain Socket transport.
// These options allow customizing the behavior of the Unix Domain Socket client connection.
type UnixSocketOption func(*unixConfig)

// unixConfig holds configuration for Unix Domain Socket transport.
// These settings control the behavior of the Unix Domain Socket client connection.
type unixConfig struct {
	socketPath     string
	permissions    uint32
	bufferSize     int
	timeout        time.Duration
	reconnect      bool
	reconnectDelay time.Duration
	maxRetries     int
}

// WithTimeout sets the timeout for Unix Domain Socket operations
func WithTimeout(timeout time.Duration) UnixSocketOption {
	return func(cfg *unixConfig) {
		cfg.timeout = timeout
	}
}

// WithReconnect enables automatic reconnection for Unix Domain Socket transport
func WithReconnect(enabled bool) UnixSocketOption {
	return func(cfg *unixConfig) {
		cfg.reconnect = enabled
	}
}

// WithReconnectDelay sets the delay between reconnection attempts
func WithReconnectDelay(delay time.Duration) UnixSocketOption {
	return func(cfg *unixConfig) {
		cfg.reconnectDelay = delay
	}
}

// WithMaxRetries sets the maximum number of reconnection attempts
func WithMaxRetries(maxRetries int) UnixSocketOption {
	return func(cfg *unixConfig) {
		cfg.maxRetries = maxRetries
	}
}

// WithBufferSize sets the buffer size for socket I/O operations
func WithBufferSize(size int) UnixSocketOption {
	return func(cfg *unixConfig) {
		cfg.bufferSize = size
	}
}

// WithPermissions sets the permissions for the socket file (server-side only)
func WithPermissions(permissions uint32) UnixSocketOption {
	return func(cfg *unixConfig) {
		cfg.permissions = permissions
	}
}

// WithUnixSocket configures the client to use Unix Domain Sockets for communication
// with optional configuration options.
//
// Parameters:
// - socketPath: The path to the Unix socket file
// - options: Optional configuration settings (timeouts, reconnection, etc.)
//
// Example:
//
//	client.New(
//	    client.WithUnixSocket("/tmp/mcp.sock"),
//	    // or with options:
//	    client.WithUnixSocket("/tmp/mcp.sock",
//	        unix.WithTimeout(time.Second*5),
//	        unix.WithReconnect(true))
//	)
func WithUnixSocket(socketPath string, options ...UnixSocketOption) Option {
	return func(c *clientImpl) {
		// Create default config
		cfg := &unixConfig{
			socketPath:     socketPath,
			permissions:    0600,
			bufferSize:     4096,
			timeout:        30 * time.Second,
			reconnect:      false,
			reconnectDelay: 1 * time.Second,
			maxRetries:     5,
		}

		// Apply options
		for _, option := range options {
			option(cfg)
		}

		// Create transport options
		transportOptions := []unix.UnixSocketOption{}

		// Apply buffer size if specified
		if cfg.bufferSize > 0 {
			transportOptions = append(transportOptions, unix.WithBufferSize(cfg.bufferSize))
		}

		// Create and configure the transport
		transport := unix.NewTransport(socketPath, transportOptions...)

		// Set the transport in the client
		c.transport = wrapUnixTransport(transport, cfg)

		// Configure timeouts if specified
		if cfg.timeout > 0 {
			c.requestTimeout = cfg.timeout
			c.connectionTimeout = cfg.timeout
			c.transport.SetRequestTimeout(cfg.timeout)
			c.transport.SetConnectionTimeout(cfg.timeout)
		}
	}
}

// unixTransportWrapper wraps the unix transport to add client-specific functionality
type unixTransportWrapper struct {
	transport           *unix.Transport
	config              *unixConfig
	reconnectCount      int
	reconnecting        bool
	notificationHandler func(method string, params []byte)
}

// wrapUnixTransport wraps a unix transport with client-specific functionality
func wrapUnixTransport(transport *unix.Transport, config *unixConfig) Transport {
	return &unixTransportWrapper{
		transport: transport,
		config:    config,
	}
}

// Connect establishes a connection to the server
func (w *unixTransportWrapper) Connect() error {
	return w.transport.Initialize()
}

// ConnectWithContext establishes a connection to the server with context
func (w *unixTransportWrapper) ConnectWithContext(ctx context.Context) error {
	// Unix transport doesn't support context for now, so just call normal Connect
	return w.Connect()
}

// Disconnect closes the connection to the server
func (w *unixTransportWrapper) Disconnect() error {
	return w.transport.Stop()
}

// Send sends a message to the server and waits for a response
func (w *unixTransportWrapper) Send(message []byte) ([]byte, error) {
	// Send the message
	err := w.transport.Send(message)
	if err != nil {
		if w.config.reconnect && !w.reconnecting {
			if w.tryReconnect() {
				// Try again after successful reconnection
				return w.Send(message)
			}
		}
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Reset reconnect count on successful send
	w.reconnectCount = 0

	// Receive the response
	return w.transport.Receive()
}

// SendWithContext sends a message with context for timeout/cancellation
func (w *unixTransportWrapper) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	// Unix transport doesn't support context directly, so just call normal Send
	// A future implementation could add proper context support
	return w.Send(message)
}

// SetRequestTimeout sets the default timeout for request operations
func (w *unixTransportWrapper) SetRequestTimeout(timeout time.Duration) {
	w.config.timeout = timeout
}

// SetConnectionTimeout sets the default timeout for connection operations
func (w *unixTransportWrapper) SetConnectionTimeout(timeout time.Duration) {
	w.config.timeout = timeout
}

// RegisterNotificationHandler registers a handler for server-initiated messages
func (w *unixTransportWrapper) RegisterNotificationHandler(handler func(method string, params []byte)) {
	w.notificationHandler = handler
}

// tryReconnect attempts to reconnect to the server
func (w *unixTransportWrapper) tryReconnect() bool {
	if w.reconnectCount >= w.config.maxRetries {
		return false
	}

	w.reconnecting = true
	defer func() { w.reconnecting = false }()

	// Disconnect first
	_ = w.transport.Stop()

	// Wait before reconnecting
	time.Sleep(w.config.reconnectDelay)

	// Try to reconnect
	err := w.transport.Initialize()
	if err != nil {
		w.reconnectCount++
		return false
	}

	return true
}
