// Package mqtt provides a MQTT implementation of the MCP transport.
//
// This package implements the Transport interface using MQTT protocol,
// suitable for IoT applications and scenarios where publish/subscribe patterns are useful.
package mqtt

import (
	"errors"
	"fmt"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/localrivet/gomcp/transport"
)

// DefaultQoS is the default Quality of Service level for MQTT
const DefaultQoS = 1 // Default to QoS 1 (at least once)

// DefaultConnectTimeout is the default timeout for connecting to the MQTT broker
const DefaultConnectTimeout = 10 * time.Second

// DefaultTopicPrefix is the default topic prefix for MCP messages
const DefaultTopicPrefix = "mcp"

// DefaultServerTopic is the default topic for server-bound messages
const DefaultServerTopic = "requests"

// DefaultClientTopic is the default topic for client-bound messages
const DefaultClientTopic = "responses"

// Transport implements the transport.Transport interface for MQTT
type Transport struct {
	transport.BaseTransport
	brokerURL    string
	clientID     string
	client       paho.Client
	isServer     bool
	topicPrefix  string
	serverTopic  string
	clientTopic  string
	qos          byte
	username     string
	password     string
	cleanSession bool
	tlsConfig    *TLSConfig
	clientsMu    sync.RWMutex
	clients      map[string]paho.Client
	connected    bool
	connMu       sync.RWMutex
	subsMu       sync.RWMutex
	subs         map[string]byte
	done         chan struct{}
	handler      transport.MessageHandler
	handlerMu    sync.RWMutex
}

// TLSConfig holds TLS configuration for MQTT connections
type TLSConfig struct {
	CertFile   string
	KeyFile    string
	CAFile     string
	ServerName string
	SkipVerify bool
}

// MQTTOption represents a configuration option for the MQTT transport
type MQTTOption func(*Transport)

// NewTransport creates a new MQTT transport
func NewTransport(brokerURL string, isServer bool, options ...MQTTOption) *Transport {
	t := &Transport{
		brokerURL:    brokerURL,
		isServer:     isServer,
		topicPrefix:  DefaultTopicPrefix,
		serverTopic:  DefaultServerTopic,
		clientTopic:  DefaultClientTopic,
		qos:          DefaultQoS,
		cleanSession: true,
		clients:      make(map[string]paho.Client),
		subs:         make(map[string]byte),
		done:         make(chan struct{}),
	}

	// Generate a random client ID if none is provided
	if t.clientID == "" {
		t.clientID = fmt.Sprintf("mcp-%s-%d", t.roleString(), time.Now().UnixNano())
	}

	// Apply options
	for _, option := range options {
		option(t)
	}

	return t
}

// roleString returns a string representing the role (server or client)
func (t *Transport) roleString() string {
	if t.isServer {
		return "server"
	}
	return "client"
}

// Initialize initializes the transport
func (t *Transport) Initialize() error {
	// Create MQTT client options
	opts := paho.NewClientOptions()
	opts.AddBroker(t.brokerURL)
	opts.SetClientID(t.clientID)
	opts.SetCleanSession(t.cleanSession)
	opts.SetAutoReconnect(true)
	opts.SetConnectTimeout(DefaultConnectTimeout)

	// Set credentials if provided
	if t.username != "" {
		opts.SetUsername(t.username)
		opts.SetPassword(t.password)
	}

	// Configure TLS if provided
	if t.tlsConfig != nil {
		// TLS configuration would be implemented here
	}

	// Set connection lost handler
	opts.SetConnectionLostHandler(func(client paho.Client, err error) {
		t.connMu.Lock()
		t.connected = false
		t.connMu.Unlock()
		// Could log the connection loss here
	})

	// Set OnConnect handler to resubscribe to topics on reconnection
	opts.SetOnConnectHandler(func(client paho.Client) {
		t.connMu.Lock()
		t.connected = true
		t.connMu.Unlock()

		// Resubscribe to topics
		t.subsMu.RLock()
		defer t.subsMu.RUnlock()

		for topic, qos := range t.subs {
			t.subscribe(topic, qos)
		}
	})

	// Create MQTT client
	t.client = paho.NewClient(opts)

	return nil
}

// Start starts the transport
func (t *Transport) Start() error {
	if token := t.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	t.connMu.Lock()
	t.connected = true
	t.connMu.Unlock()

	// Server subscribes to request topics
	if t.isServer {
		requestTopic := t.getServerTopic("#")
		if err := t.subscribe(requestTopic, t.qos); err != nil {
			return err
		}
	}

	return nil
}

// Stop stops the transport
func (t *Transport) Stop() error {
	close(t.done)

	t.clientsMu.Lock()
	defer t.clientsMu.Unlock()

	// Disconnect client
	if t.client != nil && t.client.IsConnected() {
		t.client.Disconnect(250) // Disconnect with 250ms timeout
	}

	// Disconnect all tracked clients (for server mode)
	for _, client := range t.clients {
		if client.IsConnected() {
			client.Disconnect(250)
		}
	}

	t.connMu.Lock()
	t.connected = false
	t.connMu.Unlock()

	return nil
}

// Send sends a message over the transport
func (t *Transport) Send(message []byte) error {
	t.connMu.RLock()
	connected := t.connected
	t.connMu.RUnlock()

	if !connected {
		return errors.New("not connected to MQTT broker")
	}

	var topic string
	if t.isServer {
		topic = t.getClientTopic("all") // Broadcast to all clients
	} else {
		topic = t.getServerTopic("") // Send to server
	}

	token := t.client.Publish(topic, t.qos, false, message)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// Receive is not implemented for MQTT as it uses callbacks
func (t *Transport) Receive() ([]byte, error) {
	return nil, errors.New("not implemented: MQTT transport uses subscription callbacks")
}

// getServerTopic returns the full topic for sending to server
func (t *Transport) getServerTopic(clientID string) string {
	if clientID == "" {
		return fmt.Sprintf("%s/%s", t.topicPrefix, t.serverTopic)
	}
	return fmt.Sprintf("%s/%s/%s", t.topicPrefix, t.serverTopic, clientID)
}

// getClientTopic returns the full topic for sending to clients
func (t *Transport) getClientTopic(clientID string) string {
	if clientID == "all" {
		return fmt.Sprintf("%s/%s/#", t.topicPrefix, t.clientTopic)
	}
	return fmt.Sprintf("%s/%s/%s", t.topicPrefix, t.clientTopic, clientID)
}

// messageHandler processes incoming MQTT messages
func (t *Transport) messageHandler(client paho.Client, msg paho.Message) {
	if handler := t.handler; handler != nil {
		response, err := handler(msg.Payload())
		if err == nil && response != nil {
			// Extract the client ID from the topic to respond directly to that client
			// This is a simple implementation that assumes topics like "mcp/requests/client-123"
			// For more complex topic structures, parsing would need to be more sophisticated

			// Get response topic by converting request topic
			// E.g., mcp/requests/client-123 -> mcp/responses/client-123
			topic := msg.Topic()
			responseTopic := topic
			if t.isServer {
				// Server responding to client
				// Convert from server topic to client topic
				responseTopic = topic
				// Implementation would extract client ID and build response topic
			}

			// Publish response
			token := t.client.Publish(responseTopic, t.qos, false, response)
			token.Wait()
		}
	}
}

// subscribe subscribes to an MQTT topic
func (t *Transport) subscribe(topic string, qos byte) error {
	token := t.client.Subscribe(topic, qos, t.messageHandler)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	t.subsMu.Lock()
	t.subs[topic] = qos
	t.subsMu.Unlock()

	return nil
}

// unsubscribe unsubscribes from an MQTT topic
func (t *Transport) unsubscribe(topic string) error {
	token := t.client.Unsubscribe(topic)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	t.subsMu.Lock()
	delete(t.subs, topic)
	t.subsMu.Unlock()

	return nil
}

// MQTT Transport Options

// WithClientID sets the client ID
func WithClientID(clientID string) MQTTOption {
	return func(t *Transport) {
		t.clientID = clientID
	}
}

// WithQoS sets the MQTT Quality of Service level
// QoS 0: At most once delivery
// QoS 1: At least once delivery
// QoS 2: Exactly once delivery
func WithQoS(qos byte) MQTTOption {
	return func(t *Transport) {
		if qos <= 2 {
			t.qos = qos
		}
	}
}

// WithCredentials sets the username and password for MQTT broker authentication
func WithCredentials(username, password string) MQTTOption {
	return func(t *Transport) {
		t.username = username
		t.password = password
	}
}

// WithTopicPrefix sets the topic prefix for MQTT messages
func WithTopicPrefix(prefix string) MQTTOption {
	return func(t *Transport) {
		t.topicPrefix = prefix
	}
}

// WithServerTopic sets the topic name for server-bound messages
func WithServerTopic(topic string) MQTTOption {
	return func(t *Transport) {
		t.serverTopic = topic
	}
}

// WithClientTopic sets the topic name for client-bound messages
func WithClientTopic(topic string) MQTTOption {
	return func(t *Transport) {
		t.clientTopic = topic
	}
}

// WithCleanSession sets whether to start a clean session
func WithCleanSession(clean bool) MQTTOption {
	return func(t *Transport) {
		t.cleanSession = clean
	}
}

// WithTLS sets TLS configuration for secure MQTT connections
func WithTLS(config TLSConfig) MQTTOption {
	return func(t *Transport) {
		t.tlsConfig = &config
	}
}

// SetMessageHandler sets the handler for incoming messages
func (t *Transport) SetMessageHandler(handler transport.MessageHandler) {
	t.handlerMu.Lock()
	defer t.handlerMu.Unlock()
	t.handler = handler
}

// HandleMessage processes an incoming message using the registered handler
func (t *Transport) HandleMessage(message []byte) ([]byte, error) {
	t.handlerMu.RLock()
	handler := t.handler
	t.handlerMu.RUnlock()

	if handler != nil {
		return handler(message)
	}
	return nil, errors.New("no message handler registered")
}
