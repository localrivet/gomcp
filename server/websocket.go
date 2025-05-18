package server

import (
	"github.com/localrivet/gomcp/transport/ws"
)

// AsWebsocket configures the server to use the WebSocket transport.
// The WebSocket transport provides bidirectional, full-duplex communication
// channels over a single TCP connection, enabling real-time interaction between
// clients and the server with lower overhead than HTTP polling.
//
// As per the MCP custom transport implementation, this transport provides a WebSocket
// endpoint at /ws by default for bidirectional communication.
//
// Parameters:
//   - address: The listening address for the server (e.g., ":8080" for all interfaces on port 8080)
//
// Returns:
//   - The server instance for method chaining
//
// This transport is particularly useful for web applications requiring real-time
// updates and interactive communication.
func (s *serverImpl) AsWebsocket(address string) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create WebSocket transport with the provided address
	wsTransport := ws.NewTransport(address)

	// Configure the transport with an empty path prefix by default
	// Users can set a custom prefix using AsWebsocketWithPaths if needed

	// Configure the message handler
	wsTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = wsTransport

	s.logger.Info("server configured with WebSocket transport",
		"address", address,
		"ws_endpoint", wsTransport.GetFullWSPath())
	return s
}

// AsWebsocketWithPaths configures the server to use the WebSocket transport with custom path configurations.
//
// This method allows you to customize the path used for the WebSocket endpoint:
//
// Parameters:
//   - address: The listening address for the server (e.g., ":8080" for all interfaces on port 8080)
//   - pathPrefix: Optional prefix for the endpoint (e.g., "/api/v1")
//   - wsPath: Custom path for the WebSocket endpoint (default is "/ws")
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsWebsocketWithPaths(address, pathPrefix, wsPath string) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create WebSocket transport with the provided address
	wsTransport := ws.NewTransport(address)

	// Configure the transport with custom paths
	if pathPrefix != "" {
		wsTransport.SetPathPrefix(pathPrefix)
	}

	if wsPath != "" {
		wsTransport.SetWSPath(wsPath)
	}

	// Configure the message handler
	wsTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = wsTransport

	s.logger.Info("server configured with WebSocket transport",
		"address", address,
		"ws_endpoint", wsTransport.GetFullWSPath())
	return s
}
