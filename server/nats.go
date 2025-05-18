package server

import (
	"github.com/localrivet/gomcp/transport/nats"
)

// AsNATS configures the server to use NATS for communication
// with optional configuration options.
//
// NATS provides a high-performance, cloud native communication system,
// suitable for microservices architectures, IoT messaging, and
// event-driven applications.
//
// Parameters:
//   - serverURL: The NATS server URL (e.g., "nats://localhost:4222")
//   - options: Optional configuration settings (credentials, token, subjects, etc.)
//
// Example:
//
//	server.AsNATS("nats://localhost:4222")
//	// With options:
//	server.AsNATS("nats://localhost:4222",
//	    nats.WithCredentials("username", "password"),
//	    nats.WithSubjectPrefix("custom/subject/prefix"))
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsNATS(serverURL string, options ...nats.NATSOption) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create NATS transport in server mode
	natsTransport := nats.NewTransport(serverURL, true, options...)

	// Configure the message handler
	natsTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = natsTransport

	s.logger.Info("server configured with NATS transport",
		"server", serverURL)
	return s
}

// AsNATSWithClientID configures the server to use NATS with a specific client ID
// along with optional configuration options.
//
// This is useful when you need to control the client ID to implement
// features like connection tracking or client identification.
//
// Parameters:
//   - serverURL: The NATS server URL (e.g., "nats://localhost:4222")
//   - clientID: The client ID to use for the NATS connection
//   - options: Optional configuration settings (credentials, token, subjects, etc.)
//
// Example:
//
//	server.AsNATSWithClientID("nats://localhost:4222", "mcp-server-1")
//	// With options:
//	server.AsNATSWithClientID("nats://localhost:4222", "mcp-server-1",
//	    nats.WithSubjectPrefix("my-org/mcp"))
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsNATSWithClientID(serverURL string, clientID string, options ...nats.NATSOption) Server {
	// Prepend the client ID option to the options list
	allOptions := append([]nats.NATSOption{nats.WithClientID(clientID)}, options...)
	return s.AsNATS(serverURL, allOptions...)
}

// AsNATSWithToken configures the server to use NATS with token authentication
// along with optional configuration options.
//
// This is a simplified authentication method for NATS.
//
// Parameters:
//   - serverURL: The NATS server URL (e.g., "nats://localhost:4222")
//   - token: The authentication token to use
//   - options: Optional configuration settings (client ID, subjects, etc.)
//
// Example:
//
//	server.AsNATSWithToken("nats://localhost:4222", "s3cr3t-t0k3n")
//	// With options:
//	server.AsNATSWithToken("nats://localhost:4222", "s3cr3t-t0k3n",
//	    nats.WithClientID("mcp-server-1"),
//	    nats.WithSubjectPrefix("my-org/mcp"))
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsNATSWithToken(serverURL string, token string, options ...nats.NATSOption) Server {
	// Prepend the token option to the options list
	allOptions := append([]nats.NATSOption{nats.WithToken(token)}, options...)
	return s.AsNATS(serverURL, allOptions...)
}
