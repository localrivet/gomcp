package server

import (
	"github.com/localrivet/gomcp/transport/http"
)

// AsHTTP configures the server to use the HTTP transport.
// The address parameter specifies the listening address for the server,
// e.g., ":8080" for all interfaces on port 8080.
func (s *serverImpl) AsHTTP(address string) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create HTTP transport with the provided address
	httpTransport := http.NewTransport(address)

	// Configure the transport
	httpTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = httpTransport

	s.logger.Info("server configured with HTTP transport", "address", address)
	return s
}
