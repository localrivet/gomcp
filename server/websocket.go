package server

import (
	"github.com/localrivet/gomcp/transport/ws"
)

// AsWebsocket configures the server to use the WebSocket transport.
// The address parameter specifies the listening address for the server,
// e.g., ":8080" for all interfaces on port 8080.
func (s *serverImpl) AsWebsocket(address string) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create WebSocket transport with the provided address
	wsTransport := ws.NewTransport(address)

	// Configure the transport
	wsTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = wsTransport

	s.logger.Info("server configured with WebSocket transport", "address", address)
	return s
}
