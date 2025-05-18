package client

// WithNATS configures the client to use NATS for communication
// with optional configuration options.
//
// Parameters:
// - serverURL: The NATS server URL (e.g., "nats://localhost:4222")
// - options: Optional configuration settings (credentials, token, subjects, etc.)
//
// Example:
//
//	client, err := gomcp.NewClient("test-service",
//	    client.WithNATS("nats://localhost:4222"),
//	    // or with options:
//	    client.WithNATS("nats://localhost:4222",
//	        client.WithNATSClientID("my-client"),
//	        client.WithNATSCredentials("username", "password")),
//	)
func WithNATS(serverURL string, options ...NATSTransportOption) Option {
	return func(c *clientImpl) {
		// Create a new NATS transport
		transport := NewNATSTransport(serverURL, options...)

		// Set the transport on the client
		c.transport = transport
	}
}
