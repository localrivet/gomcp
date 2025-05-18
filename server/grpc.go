package server

import (
	"time"

	"github.com/localrivet/gomcp/transport/grpc"
)

// AsGRPC configures the server to use the gRPC transport.
// The gRPC transport allows clients to connect to the server using the gRPC protocol,
// providing bi-directional streaming and strongly-typed RPC communication.
//
// Parameters:
//   - address: The listening address for the server (e.g., ":50051" for all interfaces on port 50051)
//   - options: Optional configuration options for the gRPC transport
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsGRPC(address string, options ...grpc.Option) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create gRPC transport with the provided address
	grpcTransport := grpc.NewTransport(address, true, options...)

	// Configure the transport
	grpcTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = grpcTransport

	s.logger.Info("server configured with gRPC transport",
		"address", address)
	return s
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

// DefaultGRPCServerOptions returns a set of default options for gRPC server.
func DefaultGRPCServerOptions() []grpc.Option {
	return []grpc.Option{
		grpc.WithBufferSize(1000),
		grpc.WithKeepAliveParams(10*time.Second, 3*time.Second),
		grpc.WithMaxMessageSize(4 * 1024 * 1024), // 4MB
	}
}
