// Package tcp provides a types.Transport implementation using TCP sockets.
package tcp

import (
	"fmt"
	"net"
	"time"

	"github.com/localrivet/gomcp/types"
)

// DefaultDialTimeout is the default timeout for establishing a TCP connection.
const DefaultDialTimeout = 10 * time.Second

// Dial establishes a TCP connection to the given address and returns a TCPTransport.
func Dial(address string, opts types.TransportOptions) (types.Transport, error) {
	logger := opts.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	logger.Info("TCPTransport: Dialing %s...", address)
	// Use net.DialTimeout for connection establishment
	conn, err := net.DialTimeout("tcp", address, DefaultDialTimeout)
	if err != nil {
		logger.Error("TCPTransport: Failed to dial %s: %v", address, err)
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}
	logger.Info("TCPTransport: Successfully connected to %s", address)

	// Wrap the connection in our transport
	return NewTCPTransport(conn, opts), nil
}

// Listener wraps a net.Listener to accept connections and create TCPTransports.
type Listener struct {
	listener net.Listener
	opts     types.TransportOptions
	logger   types.Logger
}

// Listen starts a TCP listener on the given address.
func Listen(address string, opts types.TransportOptions) (*Listener, error) {
	logger := opts.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	logger.Info("TCPTransport: Listening on %s...", address)
	l, err := net.Listen("tcp", address)
	if err != nil {
		logger.Error("TCPTransport: Failed to listen on %s: %v", address, err)
		return nil, fmt.Errorf("failed to listen on %s: %w", address, err)
	}
	logger.Info("TCPTransport: Successfully listening on %s", l.Addr().String())

	return &Listener{
		listener: l,
		opts:     opts,
		logger:   logger,
	}, nil
}

// Accept waits for and returns the next connection to the listener as a TCPTransport.
func (l *Listener) Accept() (types.Transport, error) {
	l.logger.Debug("TCPTransport Listener: Waiting for connection...")
	conn, err := l.listener.Accept()
	if err != nil {
		l.logger.Error("TCPTransport Listener: Failed to accept connection: %v", err)
		return nil, fmt.Errorf("failed to accept connection: %w", err)
	}
	l.logger.Info("TCPTransport Listener: Accepted connection from %s", conn.RemoteAddr().String())
	return NewTCPTransport(conn, l.opts), nil
}

// Close stops listening for new connections.
func (l *Listener) Close() error {
	l.logger.Info("TCPTransport Listener: Closing listener on %s", l.listener.Addr().String())
	return l.listener.Close()
}

// Addr returns the listener's network address.
func (l *Listener) Addr() net.Addr {
	return l.listener.Addr()
}

// NewTransportFromConn creates a TCPTransport from an existing net.Conn.
// This is useful if the connection is established externally.
// Note: This is essentially the same as the NewTCPTransport constructor.
func NewTransportFromConn(conn net.Conn, opts types.TransportOptions) types.Transport {
	return NewTCPTransport(conn, opts)
}
