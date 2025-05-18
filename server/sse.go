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
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsSSE(address string) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create SSE transport with the provided address
	sseTransport := sse.NewTransport(address)

	// Configure the transport with an empty path prefix by default
	// Users can set a custom prefix using AsSSEWithPaths if needed

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

// AsSSEWithPaths configures the server to use the Server-Sent Events transport with custom path configurations.
//
// This method allows you to customize the paths used for SSE endpoints:
//
// Parameters:
//   - address: The listening address for the server (e.g., ":8080" for all interfaces on port 8080)
//   - pathPrefix: Optional prefix for all endpoints (e.g., "/api/v1")
//   - eventsPath: Custom path for the SSE events endpoint (default is "/sse")
//   - messagePath: Custom path for the message posting endpoint (default is "/message")
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsSSEWithPaths(address, pathPrefix, eventsPath, messagePath string) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create SSE transport with the provided address
	sseTransport := sse.NewTransport(address)

	// Configure the transport with custom paths
	if pathPrefix != "" {
		sseTransport.SetPathPrefix(pathPrefix)
	}

	if eventsPath != "" {
		sseTransport.SetEventPath(eventsPath)
	}

	if messagePath != "" {
		sseTransport.SetMessagePath(messagePath)
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
