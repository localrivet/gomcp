package client

import (
	"context"
	"fmt"
	"time"

	"github.com/localrivet/gomcp/transport/udp"
)

// UDPOption is a function that configures a UDP transport.
// These options allow customizing the behavior of the UDP client connection.
type UDPOption func(*udpConfig)

// udpConfig holds configuration for UDP transport.
// These settings control the behavior of the UDP client connection.
type udpConfig struct {
	addr               string        // UDP address (host:port)
	maxPacketSize      int           // Maximum packet size
	readBufferSize     int           // Read buffer size
	writeBufferSize    int           // Write buffer size
	readTimeout        time.Duration // Read timeout
	writeTimeout       time.Duration // Write timeout
	reconnect          bool          // Whether to attempt reconnection
	reconnectDelay     time.Duration // Delay before reconnection
	maxRetries         int           // Maximum number of retries
	fragmentTTL        time.Duration // Time-to-live for fragments
	reliabilityEnabled bool          // Whether reliability mechanisms are enabled
}

// WithMaxPacketSize sets the maximum UDP packet size.
func WithMaxPacketSize(size int) UDPOption {
	return func(cfg *udpConfig) {
		if size > 0 {
			cfg.maxPacketSize = size
		}
	}
}

// WithReadBufferSize sets the read buffer size.
func WithReadBufferSize(size int) UDPOption {
	return func(cfg *udpConfig) {
		if size > 0 {
			cfg.readBufferSize = size
		}
	}
}

// WithWriteBufferSize sets the write buffer size.
func WithWriteBufferSize(size int) UDPOption {
	return func(cfg *udpConfig) {
		if size > 0 {
			cfg.writeBufferSize = size
		}
	}
}

// WithReadTimeout sets the read timeout.
func WithReadTimeout(timeout time.Duration) UDPOption {
	return func(cfg *udpConfig) {
		if timeout > 0 {
			cfg.readTimeout = timeout
		}
	}
}

// WithWriteTimeout sets the write timeout.
func WithWriteTimeout(timeout time.Duration) UDPOption {
	return func(cfg *udpConfig) {
		if timeout > 0 {
			cfg.writeTimeout = timeout
		}
	}
}

// WithReconnect enables automatic reconnection for UDP transport.
func WithUDPReconnect(enabled bool) UDPOption {
	return func(cfg *udpConfig) {
		cfg.reconnect = enabled
	}
}

// WithReconnectDelay sets the delay between reconnection attempts.
func WithUDPReconnectDelay(delay time.Duration) UDPOption {
	return func(cfg *udpConfig) {
		if delay > 0 {
			cfg.reconnectDelay = delay
		}
	}
}

// WithMaxRetries sets the maximum number of reconnection attempts.
func WithUDPMaxRetries(retries int) UDPOption {
	return func(cfg *udpConfig) {
		if retries > 0 {
			cfg.maxRetries = retries
		}
	}
}

// WithFragmentTTL sets the time-to-live for message fragments.
func WithFragmentTTL(ttl time.Duration) UDPOption {
	return func(cfg *udpConfig) {
		if ttl > 0 {
			cfg.fragmentTTL = ttl
		}
	}
}

// WithReliability enables or disables reliability mechanisms.
func WithReliability(enabled bool) UDPOption {
	return func(cfg *udpConfig) {
		cfg.reliabilityEnabled = enabled
	}
}

// WithUDP configures the client to use UDP for communication
// with optional configuration options.
//
// UDP provides low-latency communication with minimal overhead,
// suitable for high-throughput scenarios where occasional packet
// loss is acceptable.
//
// Parameters:
// - address: The UDP address in the format "host:port"
// - options: Optional configuration settings (buffering, timeouts, etc.)
//
// Example:
//
//	client.New(
//	    client.WithUDP("example.com:8080"),
//	    // or with options:
//	    client.WithUDP("example.com:8080",
//	        client.WithReadTimeout(5*time.Second),
//	        client.WithReliability(true))
//	)
func WithUDP(address string, options ...UDPOption) Option {
	return func(c *clientImpl) {
		// Create default config
		cfg := &udpConfig{
			addr:               address,
			maxPacketSize:      udp.DefaultMaxPacketSize,
			readBufferSize:     udp.DefaultReadBufferSize,
			writeBufferSize:    udp.DefaultWriteBufferSize,
			readTimeout:        udp.DefaultReadTimeout,
			writeTimeout:       udp.DefaultWriteTimeout,
			reconnect:          false,
			reconnectDelay:     udp.DefaultReconnectDelay,
			maxRetries:         udp.DefaultMaxRetries,
			fragmentTTL:        udp.DefaultFragmentTTL,
			reliabilityEnabled: false,
		}

		// Apply options
		for _, option := range options {
			option(cfg)
		}

		// Create transport options
		transportOptions := []udp.UDPOption{}

		// Apply configuration options
		if cfg.maxPacketSize > 0 {
			transportOptions = append(transportOptions, udp.WithMaxPacketSize(cfg.maxPacketSize))
		}
		if cfg.readBufferSize > 0 {
			transportOptions = append(transportOptions, udp.WithReadBufferSize(cfg.readBufferSize))
		}
		if cfg.writeBufferSize > 0 {
			transportOptions = append(transportOptions, udp.WithWriteBufferSize(cfg.writeBufferSize))
		}
		if cfg.readTimeout > 0 {
			transportOptions = append(transportOptions, udp.WithReadTimeout(cfg.readTimeout))
		}
		if cfg.writeTimeout > 0 {
			transportOptions = append(transportOptions, udp.WithWriteTimeout(cfg.writeTimeout))
		}
		if cfg.reconnectDelay > 0 {
			transportOptions = append(transportOptions, udp.WithReconnectDelay(cfg.reconnectDelay))
		}
		if cfg.maxRetries > 0 {
			transportOptions = append(transportOptions, udp.WithMaxRetries(cfg.maxRetries))
		}
		if cfg.fragmentTTL > 0 {
			transportOptions = append(transportOptions, udp.WithFragmentTTL(cfg.fragmentTTL))
		}

		transportOptions = append(transportOptions, udp.WithReliability(cfg.reliabilityEnabled))

		// Create and configure the transport (in client mode: isServer=false)
		transport := udp.NewTransport(address, false, transportOptions...)

		// Set the transport in the client
		c.transport = wrapUDPTransport(transport, cfg)

		// Configure timeouts
		c.requestTimeout = cfg.readTimeout
		c.connectionTimeout = cfg.readTimeout
		c.transport.SetRequestTimeout(cfg.readTimeout)
		c.transport.SetConnectionTimeout(cfg.readTimeout)
	}
}

// udpTransportWrapper wraps the UDP transport to add client-specific functionality
type udpTransportWrapper struct {
	transport           *udp.Transport
	config              *udpConfig
	reconnectCount      int
	reconnecting        bool
	notificationHandler func(method string, params []byte)
}

// wrapUDPTransport wraps a UDP transport with client-specific functionality
func wrapUDPTransport(transport *udp.Transport, config *udpConfig) Transport {
	return &udpTransportWrapper{
		transport: transport,
		config:    config,
	}
}

// Connect establishes a connection to the server
func (w *udpTransportWrapper) Connect() error {
	// Initialize the UDP transport
	err := w.transport.Initialize()
	if err != nil {
		return err
	}

	// Start the transport
	return w.transport.Start()
}

// ConnectWithContext establishes a connection to the server with context
func (w *udpTransportWrapper) ConnectWithContext(ctx context.Context) error {
	// UDP transport doesn't support context for now, so just call normal Connect
	return w.Connect()
}

// Disconnect closes the connection to the server
func (w *udpTransportWrapper) Disconnect() error {
	return w.transport.Stop()
}

// Send sends a message to the server and waits for a response
func (w *udpTransportWrapper) Send(message []byte) ([]byte, error) {
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
func (w *udpTransportWrapper) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	// UDP transport doesn't support context directly, so just call normal Send
	// A future implementation could add proper context support
	return w.Send(message)
}

// SetRequestTimeout sets the default timeout for request operations
func (w *udpTransportWrapper) SetRequestTimeout(timeout time.Duration) {
	w.config.readTimeout = timeout
}

// SetConnectionTimeout sets the default timeout for connection operations
func (w *udpTransportWrapper) SetConnectionTimeout(timeout time.Duration) {
	w.config.readTimeout = timeout
}

// RegisterNotificationHandler registers a handler for server-initiated messages
func (w *udpTransportWrapper) RegisterNotificationHandler(handler func(method string, params []byte)) {
	w.notificationHandler = handler
}

// tryReconnect attempts to reconnect to the server
func (w *udpTransportWrapper) tryReconnect() bool {
	if w.reconnectCount >= w.config.maxRetries {
		return false
	}

	w.reconnecting = true
	defer func() { w.reconnecting = false }()

	// Disconnect first
	_ = w.transport.Stop()

	// Wait before reconnecting
	time.Sleep(w.config.reconnectDelay)

	// Try to reconnect (reinitialize and start the transport)
	if err := w.transport.Initialize(); err != nil {
		w.reconnectCount++
		return false
	}

	if err := w.transport.Start(); err != nil {
		w.reconnectCount++
		return false
	}

	return true
}
