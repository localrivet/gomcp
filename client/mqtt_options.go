package client

// WithMQTT configures the client to use MQTT for communication
// with optional configuration options.
//
// Parameters:
// - brokerURL: The MQTT broker URL (e.g., "tcp://broker.example.com:1883")
// - options: Optional configuration settings (QoS, credentials, topics, etc.)
//
// Example:
//
//	client, err := gomcp.NewClient("test-service",
//	    client.WithMQTT("tcp://broker.example.com:1883"),
//	    // or with options:
//	    client.WithMQTT("tcp://broker.example.com:1883",
//	        client.WithMQTTClientID("my-client"),
//	        client.WithMQTTQoS(1),
//	        client.WithMQTTCredentials("username", "password")),
//	)
func WithMQTT(brokerURL string, options ...MQTTTransportOption) Option {
	return func(c *clientImpl) {
		// Create a new MQTT transport
		transport := NewMQTTTransport(brokerURL, options...)

		// Set the transport on the client
		c.transport = transport
	}
}
