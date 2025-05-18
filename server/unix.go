package server

import (
	"github.com/localrivet/gomcp/transport/unix"
)

// AsUnixSocket configures the server to use Unix Domain Sockets for communication
// with optional configuration options.
//
// Unix Domain Sockets provide high-performance local inter-process communication
// with lower overhead than network-based transports like HTTP or WebSockets.
//
// Parameters:
//   - socketPath: The path to the Unix socket file
//   - options: Optional configuration settings (permissions, buffering, etc.)
//
// Example:
//
//	server.AsUnixSocket("/tmp/mcp.sock")
//	// With options:
//	server.AsUnixSocket("/tmp/mcp.sock",
//	    unix.WithPermissions(0600),
//	    unix.WithBufferSize(8192))
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsUnixSocket(socketPath string, options ...unix.UnixSocketOption) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create Unix Domain Socket transport with the provided socket path
	unixTransport := unix.NewTransport(socketPath, options...)

	// Configure the message handler
	unixTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = unixTransport

	s.logger.Info("server configured with Unix Domain Socket transport",
		"socket_path", socketPath)
	return s
}
