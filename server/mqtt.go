package server

import (
	"github.com/localrivet/gomcp/transport/mqtt"
)

// AsMQTT configures the server to use MQTT for communication
// with optional configuration options.
//
// MQTT provides a publish/subscribe-based communication model,
// suitable for IoT applications and distributed systems with
// potentially intermittent connectivity.
//
// Parameters:
//   - brokerURL: The MQTT broker URL (e.g., "tcp://broker.example.com:1883")
//   - options: Optional configuration settings (QoS, credentials, topics, etc.)
//
// Example:
//
//	server.AsMQTT("tcp://broker.example.com:1883")
//	// With options:
//	server.AsMQTT("tcp://broker.example.com:1883",
//	    mqtt.WithQoS(1),
//	    mqtt.WithCredentials("username", "password"),
//	    mqtt.WithTopicPrefix("custom/topic/prefix"))
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsMQTT(brokerURL string, options ...mqtt.MQTTOption) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create MQTT transport in server mode
	mqttTransport := mqtt.NewTransport(brokerURL, true, options...)

	// Configure the message handler
	mqttTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = mqttTransport

	s.logger.Info("server configured with MQTT transport",
		"broker", brokerURL)
	return s
}

// AsMQTTWithClientID configures the server to use MQTT with a specific client ID
// along with optional configuration options.
//
// This is useful when you need to control the client ID to implement
// features like persistent sessions or shared subscriptions.
//
// Parameters:
//   - brokerURL: The MQTT broker URL (e.g., "tcp://broker.example.com:1883")
//   - clientID: The client ID to use for the MQTT connection
//   - options: Optional configuration settings (QoS, credentials, topics, etc.)
//
// Example:
//
//	server.AsMQTTWithClientID("tcp://broker.example.com:1883", "mcp-server-1")
//	// With options:
//	server.AsMQTTWithClientID("tcp://broker.example.com:1883", "mcp-server-1",
//	    mqtt.WithQoS(2),
//	    mqtt.WithTopicPrefix("my-org/mcp"))
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsMQTTWithClientID(brokerURL string, clientID string, options ...mqtt.MQTTOption) Server {
	// Prepend the client ID option to the options list
	allOptions := append([]mqtt.MQTTOption{mqtt.WithClientID(clientID)}, options...)
	return s.AsMQTT(brokerURL, allOptions...)
}

// AsMQTTWithTLS configures the server to use MQTT with TLS security
// along with optional configuration options.
//
// This is recommended for production environments to encrypt
// communications between the server and the MQTT broker.
//
// Parameters:
//   - brokerURL: The MQTT broker URL (e.g., "ssl://broker.example.com:8883")
//   - tlsConfig: TLS configuration with certificates and verification settings
//   - options: Optional configuration settings (QoS, credentials, topics, etc.)
//
// Example:
//
//	server.AsMQTTWithTLS("ssl://broker.example.com:8883",
//	    mqtt.TLSConfig{
//	        CertFile: "/path/to/cert.pem",
//	        KeyFile: "/path/to/key.pem",
//	        CAFile: "/path/to/ca.pem",
//	    })
//	// With options:
//	server.AsMQTTWithTLS("ssl://broker.example.com:8883",
//	    mqtt.TLSConfig{
//	        CertFile: "/path/to/cert.pem",
//	        KeyFile: "/path/to/key.pem",
//	        CAFile: "/path/to/ca.pem",
//	    },
//	    mqtt.WithQoS(2),
//	    mqtt.WithCredentials("username", "password"))
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsMQTTWithTLS(brokerURL string, tlsConfig mqtt.TLSConfig, options ...mqtt.MQTTOption) Server {
	// Prepend the TLS option to the options list
	allOptions := append([]mqtt.MQTTOption{mqtt.WithTLS(tlsConfig)}, options...)
	return s.AsMQTT(brokerURL, allOptions...)
}
