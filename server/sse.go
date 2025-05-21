package server

import (
	"github.com/localrivet/gomcp/transport/sse"
)

// AsSSE configures the server to use the Server-Sent Events transport.
// The SSE transport enables one-way, real-time communication from the server to clients
// using the HTTP-based Server-Sent Events standard. This transport is particularly
// useful for streaming updates and notifications to web-based clients.
//
// As per the MCP specification, this transport provides two endpoints:
// 1. An SSE endpoint (/sse by default) for clients to establish a connection and receive messages
// 2. A message endpoint (/message by default) for clients to send messages via HTTP POST
//
// Parameters:
//   - address: The listening address for the server (e.g., ":8080" for all interfaces on port 8080)
//   - options: Optional configuration options for the SSE transport
//
// Returns:
//   - The server instance for method chaining
//
// Example usage:
//
//	// Basic usage with default paths
//	server.AsSSE(":8080")
//
//	// With custom path options
//	server.AsSSE(":8080", sse.SSE.WithPathPrefix("/api/v1"), sse.SSE.WithEventsPath("/events"))
func (s *serverImpl) AsSSE(address string, options ...sse.Option) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create SSE transport with the provided address
	sseTransport := sse.NewTransport(address)

	// Apply any provided options
	for _, option := range options {
		option(sseTransport)
	}

	// Configure the message handler
	sseTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = sseTransport

	s.logger.Info("server configured with Server-Sent Events transport",
		"address", address,
		"events_endpoint", sseTransport.GetFullEventsPath(),
		"message_endpoint", sseTransport.GetFullMessagePath())
	return s
}
