package server

import (
	"github.com/localrivet/gomcp/transport/http"
)

// AsHTTP configures the server to use the HTTP transport.
// The HTTP transport allows clients to connect to the server using the standard HTTP protocol,
// sending JSON-RPC requests as HTTP POST requests and receiving responses in the HTTP response body.
//
// Parameters:
//   - address: The listening address for the server (e.g., ":8080" for all interfaces on port 8080)
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsHTTP(address string) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create HTTP transport with the provided address
	httpTransport := http.NewTransport(address)

	// Configure the transport
	httpTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = httpTransport

	s.logger.Info("server configured with HTTP transport",
		"address", address,
		"api_endpoint", httpTransport.GetFullAPIPath())
	return s
}

// AsHTTPWithPaths configures the server to use the HTTP transport with custom path configurations.
//
// This method allows you to customize the path used for the HTTP API endpoint:
//
// Parameters:
//   - address: The listening address for the server (e.g., ":8080" for all interfaces on port 8080)
//   - pathPrefix: Optional prefix for the endpoint (e.g., "/api/v1")
//   - apiPath: Custom path for the HTTP API endpoint (default is "/api")
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsHTTPWithPaths(address, pathPrefix, apiPath string) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create HTTP transport with the provided address
	httpTransport := http.NewTransport(address)

	// Configure the transport with custom paths
	if pathPrefix != "" {
		httpTransport.SetPathPrefix(pathPrefix)
	}

	if apiPath != "" {
		httpTransport.SetAPIPath(apiPath)
	}

	// Configure the message handler
	httpTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = httpTransport

	s.logger.Info("server configured with HTTP transport",
		"address", address,
		"api_endpoint", httpTransport.GetFullAPIPath())
	return s
}
