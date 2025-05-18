package client

import (
	"context"
	"fmt"
	"time"

	"github.com/localrivet/gomcp/transport/grpc"
)

// GRPCTransport wraps a grpc.Transport to implement the client.Transport interface
type GRPCTransport struct {
	transport     *grpc.Transport
	notifyHandler func(method string, params []byte)
	reqTimeout    time.Duration
	connTimeout   time.Duration
}

// Connect establishes a connection to the server
func (t *GRPCTransport) Connect() error {
	if err := t.transport.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize gRPC transport: %w", err)
	}
	return t.transport.Start()
}

// ConnectWithContext establishes a connection to the server with context
func (t *GRPCTransport) ConnectWithContext(ctx context.Context) error {
	// Using the context for cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return t.Connect()
	}
}

// Disconnect closes the connection to the server
func (t *GRPCTransport) Disconnect() error {
	return t.transport.Stop()
}

// Send sends a message to the server and waits for a response
func (t *GRPCTransport) Send(message []byte) ([]byte, error) {
	if err := t.transport.Send(message); err != nil {
		return nil, err
	}

	// Set up a timeout context for receiving the response
	ctx := context.Background()
	if t.reqTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.reqTimeout)
		defer cancel()
	}

	// Create a separate goroutine to handle the response
	responseCh := make(chan []byte, 1)
	errorCh := make(chan error, 1)

	go func() {
		resp, err := t.transport.Receive()
		if err != nil {
			errorCh <- err
			return
		}
		responseCh <- resp
	}()

	// Wait for response or timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errorCh:
		return nil, err
	case resp := <-responseCh:
		return resp, nil
	}
}

// SendWithContext sends a message with context for timeout/cancellation
func (t *GRPCTransport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	if err := t.transport.Send(message); err != nil {
		return nil, err
	}

	// Create a separate goroutine to handle the response
	responseCh := make(chan []byte, 1)
	errorCh := make(chan error, 1)

	go func() {
		resp, err := t.transport.Receive()
		if err != nil {
			errorCh <- err
			return
		}
		responseCh <- resp
	}()

	// Wait for response or context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errorCh:
		return nil, err
	case resp := <-responseCh:
		return resp, nil
	}
}

// SetRequestTimeout sets the default timeout for request operations
func (t *GRPCTransport) SetRequestTimeout(timeout time.Duration) {
	t.reqTimeout = timeout
}

// SetConnectionTimeout sets the default timeout for connection operations
func (t *GRPCTransport) SetConnectionTimeout(timeout time.Duration) {
	t.connTimeout = timeout
	// Could also update the transport's connection timeout if needed
}

// RegisterNotificationHandler registers a handler for server-initiated messages
func (t *GRPCTransport) RegisterNotificationHandler(handler func(method string, params []byte)) {
	t.notifyHandler = handler
	// Set up a goroutine to listen for notifications from the gRPC transport
	// and forward them to the handler
}

// WithGRPC returns a client configuration option that uses gRPC transport.
// The gRPC transport provides bi-directional streaming and high-performance communication.
//
// Parameters:
//   - address: The server address to connect to (e.g., "localhost:50051")
//   - options: Optional configuration options for the gRPC transport
//
// Returns:
//   - A client configuration option
func WithGRPC(address string, options ...grpc.Option) Option {
	return func(c *clientImpl) {
		// Create the gRPC transport
		grpcTransport := grpc.NewTransport(address, false, options...)

		// Wrap it with our adapter
		transport := &GRPCTransport{
			transport:   grpcTransport,
			reqTimeout:  c.requestTimeout,
			connTimeout: c.connectionTimeout,
		}

		c.transport = transport
	}
}

// WithGRPCTLS configures TLS for the gRPC transport.
func WithGRPCTLS(certFile, keyFile, caFile string) grpc.Option {
	return grpc.WithTLS(certFile, keyFile, caFile)
}

// WithGRPCKeepAlive configures keepalive parameters for the gRPC transport.
func WithGRPCKeepAlive(time, timeout time.Duration) grpc.Option {
	return grpc.WithKeepAliveParams(time, timeout)
}

// WithGRPCTimeout sets the connection timeout for the gRPC transport.
func WithGRPCTimeout(timeout time.Duration) grpc.Option {
	return grpc.WithConnectionTimeout(timeout)
}

// WithGRPCMaxMessageSize sets the maximum message size for the gRPC transport.
func WithGRPCMaxMessageSize(size int) grpc.Option {
	return grpc.WithMaxMessageSize(size)
}

// DefaultGRPCClientOptions returns a set of default options for gRPC client.
func DefaultGRPCClientOptions() []grpc.Option {
	return []grpc.Option{
		grpc.WithBufferSize(100),
		grpc.WithConnectionTimeout(5 * time.Second),
		grpc.WithKeepAliveParams(10*time.Second, 3*time.Second),
		grpc.WithMaxMessageSize(4 * 1024 * 1024), // 4MB
	}
}
