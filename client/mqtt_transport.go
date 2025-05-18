package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/localrivet/gomcp/transport/mqtt"
)

// MQTTTransport implements the Transport interface for MQTT.
type MQTTTransport struct {
	brokerURL           string
	clientID            string
	client              paho.Client
	topicPrefix         string
	serverTopic         string
	clientTopic         string
	qos                 byte
	connected           bool
	connMu              sync.RWMutex
	username            string
	password            string
	cleanSession        bool
	tlsConfig           *mqtt.TLSConfig
	requestTimeout      time.Duration
	connectionTimeout   time.Duration
	notificationChan    chan mqttNotification
	notificationHandler func(method string, params []byte)
	done                chan struct{}
}

type mqttNotification struct {
	Method string
	Params []byte
}

// MQTTTransportOption represents a configuration option for the MQTT transport
type MQTTTransportOption func(*MQTTTransport)

// NewMQTTTransport creates a new MQTT transport.
func NewMQTTTransport(brokerURL string, options ...MQTTTransportOption) *MQTTTransport {
	t := &MQTTTransport{
		brokerURL:         brokerURL,
		topicPrefix:       mqtt.DefaultTopicPrefix,
		serverTopic:       mqtt.DefaultServerTopic,
		clientTopic:       mqtt.DefaultClientTopic,
		qos:               mqtt.DefaultQoS,
		cleanSession:      true,
		requestTimeout:    30 * time.Second,
		connectionTimeout: 10 * time.Second,
		notificationChan:  make(chan mqttNotification, 100),
		done:              make(chan struct{}),
	}

	// Generate a random client ID if none is provided
	if t.clientID == "" {
		t.clientID = fmt.Sprintf("mcp-client-%d", time.Now().UnixNano())
	}

	// Apply options
	for _, option := range options {
		option(t)
	}

	return t
}

// Connect implements the Transport.Connect method.
func (t *MQTTTransport) Connect() error {
	// Create MQTT client options
	opts := paho.NewClientOptions()
	opts.AddBroker(t.brokerURL)
	opts.SetClientID(t.clientID)
	opts.SetCleanSession(t.cleanSession)
	opts.SetAutoReconnect(true)
	opts.SetConnectTimeout(t.connectionTimeout)

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
	})

	// Create MQTT client
	t.client = paho.NewClient(opts)

	// Connect to broker
	if token := t.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	t.connMu.Lock()
	t.connected = true
	t.connMu.Unlock()

	// Subscribe to client's response topic
	responseTopic := fmt.Sprintf("%s/%s/%s", t.topicPrefix, t.clientTopic, t.clientID)
	if token := t.client.Subscribe(responseTopic, t.qos, t.messageHandler); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	// Start notification handler goroutine
	go t.handleNotifications()

	return nil
}

// ConnectWithContext implements the Transport.ConnectWithContext method.
func (t *MQTTTransport) ConnectWithContext(ctx context.Context) error {
	// Create a channel to signal when the connection is complete
	done := make(chan error, 1)

	// Start the connection in a goroutine
	go func() {
		done <- t.Connect()
	}()

	// Wait for the connection to complete or the context to be canceled
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Disconnect implements the Transport.Disconnect method.
func (t *MQTTTransport) Disconnect() error {
	close(t.done)

	t.connMu.Lock()
	defer t.connMu.Unlock()

	// Disconnect MQTT client
	if t.client != nil && t.client.IsConnected() {
		t.client.Disconnect(250) // Disconnect with 250ms timeout
	}

	t.connected = false
	return nil
}

// Send implements the Transport.Send method.
func (t *MQTTTransport) Send(message []byte) ([]byte, error) {
	return t.SendWithContext(context.Background(), message)
}

// SendWithContext implements the Transport.SendWithContext method.
func (t *MQTTTransport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	t.connMu.RLock()
	connected := t.connected
	t.connMu.RUnlock()

	if !connected {
		return nil, errors.New("not connected to MQTT broker")
	}

	// Parse the message to get the request ID
	var requestMap map[string]interface{}
	if err := json.Unmarshal(message, &requestMap); err != nil {
		return nil, fmt.Errorf("invalid JSON message: %w", err)
	}

	requestID, _ := requestMap["id"]

	// Define the listener for responses
	responseCh := make(chan []byte, 1)
	errorCh := make(chan error, 1)

	// Create a message handler for this request
	responseHandler := func(client paho.Client, msg paho.Message) {
		var responseMap map[string]interface{}
		if err := json.Unmarshal(msg.Payload(), &responseMap); err != nil {
			errorCh <- fmt.Errorf("invalid JSON response: %w", err)
			return
		}

		// Check if the response ID matches the request ID
		if responseMap["id"] == requestID {
			responseCh <- msg.Payload()
		}
	}

	// Subscribe to response topic for this specific request
	responseTopic := fmt.Sprintf("%s/%s/%s", t.topicPrefix, t.clientTopic, t.clientID)
	token := t.client.Subscribe(responseTopic, t.qos, responseHandler)
	if token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	// Ensure we unsubscribe when done
	defer func() {
		t.client.Unsubscribe(responseTopic)
	}()

	// Send the request
	requestTopic := fmt.Sprintf("%s/%s", t.topicPrefix, t.serverTopic)
	token = t.client.Publish(requestTopic, t.qos, false, message)
	if token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	// Wait for the response with context timeout
	select {
	case response := <-responseCh:
		return response, nil
	case err := <-errorCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SetRequestTimeout implements the Transport.SetRequestTimeout method.
func (t *MQTTTransport) SetRequestTimeout(timeout time.Duration) {
	t.requestTimeout = timeout
}

// SetConnectionTimeout implements the Transport.SetConnectionTimeout method.
func (t *MQTTTransport) SetConnectionTimeout(timeout time.Duration) {
	t.connectionTimeout = timeout
}

// RegisterNotificationHandler implements the Transport.RegisterNotificationHandler method.
func (t *MQTTTransport) RegisterNotificationHandler(handler func(method string, params []byte)) {
	t.notificationHandler = handler
}

// messageHandler processes incoming MQTT messages
func (t *MQTTTransport) messageHandler(client paho.Client, msg paho.Message) {
	var jsonMsg map[string]interface{}
	if err := json.Unmarshal(msg.Payload(), &jsonMsg); err != nil {
		return // Invalid JSON
	}

	// Check if this is a notification (no ID field)
	if _, hasID := jsonMsg["id"]; !hasID {
		if method, ok := jsonMsg["method"].(string); ok {
			if params, ok := jsonMsg["params"]; ok {
				// Handle notification
				if t.notificationHandler != nil {
					var paramsBytes []byte
					if params != nil {
						paramsBytes, _ = json.Marshal(params)
					}
					t.notificationChan <- mqttNotification{
						Method: method,
						Params: paramsBytes,
					}
				}
			}
		}
	}
}

// handleNotifications processes notifications in a separate goroutine
func (t *MQTTTransport) handleNotifications() {
	for {
		select {
		case <-t.done:
			return
		case notification := <-t.notificationChan:
			if t.notificationHandler != nil {
				t.notificationHandler(notification.Method, notification.Params)
			}
		}
	}
}

// MQTT Transport Options

// WithMQTTClientID sets the client ID
func WithMQTTClientID(clientID string) MQTTTransportOption {
	return func(t *MQTTTransport) {
		t.clientID = clientID
	}
}

// WithMQTTQoS sets the MQTT Quality of Service level
func WithMQTTQoS(qos byte) MQTTTransportOption {
	return func(t *MQTTTransport) {
		if qos <= 2 {
			t.qos = qos
		}
	}
}

// WithMQTTCredentials sets the username and password for MQTT broker authentication
func WithMQTTCredentials(username, password string) MQTTTransportOption {
	return func(t *MQTTTransport) {
		t.username = username
		t.password = password
	}
}

// WithMQTTTopicPrefix sets the topic prefix for MQTT messages
func WithMQTTTopicPrefix(prefix string) MQTTTransportOption {
	return func(t *MQTTTransport) {
		t.topicPrefix = prefix
	}
}

// WithMQTTTLS sets TLS configuration for secure MQTT connections
func WithMQTTTLS(config *mqtt.TLSConfig) MQTTTransportOption {
	return func(t *MQTTTransport) {
		t.tlsConfig = config
	}
}
