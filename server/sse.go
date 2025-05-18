package server

import (
	"github.com/localrivet/gomcp/transport/sse"
)

// AsSSE configures the server to use the Server-Sent Events transport.
// The address parameter specifies the listening address for the server,
// e.g., ":8080" for all interfaces on port 8080.
func (s *serverImpl) AsSSE(address string) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create SSE transport with the provided address
	sseTransport := sse.NewTransport(address)

	// Configure the transport
	sseTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = sseTransport

	s.logger.Info("server configured with Server-Sent Events transport", "address", address)
	return s
}
