// Package nats provides a NATS implementation of the MCP transport.
//
// This package implements the Transport interface using NATS protocol,
// suitable for cloud-native applications requiring high-performance, scalable messaging.
package nats

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/localrivet/gomcp/transport"
	"github.com/nats-io/nats.go"
)

// DefaultConnectTimeout is the default timeout for connecting to the NATS server
const DefaultConnectTimeout = 10 * time.Second

// DefaultSubjectPrefix is the default subject prefix for MCP messages
const DefaultSubjectPrefix = "mcp"

// DefaultServerSubject is the default subject for server-bound messages
const DefaultServerSubject = "requests"

// DefaultClientSubject is the default subject for client-bound messages
const DefaultClientSubject = "responses"

// Transport implements the transport.Transport interface for NATS
type Transport struct {
	transport.BaseTransport
	serverURL     string
	clientID      string
	conn          *nats.Conn
	isServer      bool
	subjectPrefix string
	serverSubject string
	clientSubject string
	username      string
	password      string
	token         string
	tlsConfig     *TLSConfig
	subs          map[string]*nats.Subscription
	subsMu        sync.RWMutex
	connected     bool
	connMu        sync.RWMutex
	done          chan struct{}
	handler       transport.MessageHandler
}

// TLSConfig holds TLS configuration for NATS connections
type TLSConfig struct {
	CertFile   string
	KeyFile    string
	CAFile     string
	ServerName string
	SkipVerify bool
}

// NATSOption represents a configuration option for the NATS transport
type NATSOption func(*Transport)

// NewTransport creates a new NATS transport
func NewTransport(serverURL string, isServer bool, options ...NATSOption) *Transport {
	t := &Transport{
		serverURL:     serverURL,
		isServer:      isServer,
		subjectPrefix: DefaultSubjectPrefix,
		serverSubject: DefaultServerSubject,
		clientSubject: DefaultClientSubject,
		subs:          make(map[string]*nats.Subscription),
		done:          make(chan struct{}),
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
	// Create NATS connection options
	opts := []nats.Option{
		nats.Name(t.clientID),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1), // Infinite reconnects
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			t.connMu.Lock()
			t.connected = false
			t.connMu.Unlock()
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			t.connMu.Lock()
			t.connected = true
			t.connMu.Unlock()

			// Resubscribe to subjects if needed
			t.resubscribe()
		}),
	}

	// Set credentials if provided
	if t.username != "" && t.password != "" {
		opts = append(opts, nats.UserInfo(t.username, t.password))
	} else if t.token != "" {
		opts = append(opts, nats.Token(t.token))
	}

	// Configure TLS if provided
	if t.tlsConfig != nil {
		// TLS configuration would be implemented here
		// opts = append(opts, nats.ClientCert(t.tlsConfig.CertFile, t.tlsConfig.KeyFile))
		// opts = append(opts, nats.RootCAs(t.tlsConfig.CAFile))
	}

	// Connect to NATS server
	var err error
	t.conn, err = nats.Connect(t.serverURL, opts...)
	if err != nil {
		return err
	}

	t.connMu.Lock()
	t.connected = true
	t.connMu.Unlock()

	return nil
}

// resubscribe resubscribes to all tracked subjects
func (t *Transport) resubscribe() {
	t.subsMu.RLock()
	defer t.subsMu.RUnlock()

	for subject := range t.subs {
		// Remove old subscription
		delete(t.subs, subject)
		// Create new subscription
		t.subscribe(subject)
	}
}

// Start starts the transport
func (t *Transport) Start() error {
	// Server subscribes to request subjects
	if t.isServer {
		requestSubject := t.getServerSubject("*")
		if err := t.subscribe(requestSubject); err != nil {
			return err
		}
	}

	return nil
}

// Stop stops the transport
func (t *Transport) Stop() error {
	close(t.done)

	t.subsMu.Lock()
	defer t.subsMu.Unlock()

	// Unsubscribe all subscriptions
	for subject, sub := range t.subs {
		if sub != nil {
			sub.Unsubscribe()
		}
		delete(t.subs, subject)
	}

	// Close NATS connection
	if t.conn != nil {
		t.conn.Close()
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
		return errors.New("not connected to NATS server")
	}

	var subject string
	if t.isServer {
		subject = t.getClientSubject("all") // Broadcast to all clients
	} else {
		subject = t.getServerSubject("") // Send to server
	}

	return t.conn.Publish(subject, message)
}

// Receive is not implemented for NATS as it uses callbacks
func (t *Transport) Receive() ([]byte, error) {
	return nil, errors.New("not implemented: NATS transport uses subscription callbacks")
}

// getServerSubject returns the full subject for sending to server
func (t *Transport) getServerSubject(clientID string) string {
	if clientID == "" {
		return fmt.Sprintf("%s.%s", t.subjectPrefix, t.serverSubject)
	}
	return fmt.Sprintf("%s.%s.%s", t.subjectPrefix, t.serverSubject, clientID)
}

// getClientSubject returns the full subject for sending to clients
func (t *Transport) getClientSubject(clientID string) string {
	if clientID == "all" {
		return fmt.Sprintf("%s.%s.>", t.subjectPrefix, t.clientSubject)
	}
	return fmt.Sprintf("%s.%s.%s", t.subjectPrefix, t.clientSubject, clientID)
}

// messageHandler processes incoming NATS messages
func (t *Transport) messageHandler(msg *nats.Msg) {
	// Get the client ID from the subject if this is a server
	var clientID string
	if t.isServer {
		// Extract clientID from the subject if it exists
		parts := strings.Split(msg.Subject, ".")
		if len(parts) >= 3 {
			clientID = parts[2]
		}
	}

	// Process the message
	if t.handler != nil {
		response, err := t.HandleMessage(msg.Data)
		if err != nil {
			// Could log the error here
			return
		}

		// If there's a reply subject and we have a response, send it
		if msg.Reply != "" && response != nil {
			t.conn.Publish(msg.Reply, response)
			return
		}

		// For server sending to a specific client
		if t.isServer && clientID != "" && response != nil {
			responseSubject := t.getClientSubject(clientID)
			t.conn.Publish(responseSubject, response)
		}
	}
}

// subscribe subscribes to a subject
func (t *Transport) subscribe(subject string) error {
	t.subsMu.Lock()
	defer t.subsMu.Unlock()

	// Check if we're already subscribed
	if _, exists := t.subs[subject]; exists {
		return nil
	}

	// Subscribe to the subject
	sub, err := t.conn.Subscribe(subject, t.messageHandler)
	if err != nil {
		return err
	}

	// Store the subscription
	t.subs[subject] = sub

	return nil
}

// unsubscribe unsubscribes from a subject
func (t *Transport) unsubscribe(subject string) error {
	t.subsMu.Lock()
	defer t.subsMu.Unlock()

	// Check if we're subscribed
	sub, exists := t.subs[subject]
	if !exists {
		return nil
	}

	// Unsubscribe
	err := sub.Unsubscribe()
	if err != nil {
		return err
	}

	// Remove from our tracking
	delete(t.subs, subject)

	return nil
}

// SetMessageHandler sets the message handler
func (t *Transport) SetMessageHandler(handler transport.MessageHandler) {
	t.handler = handler
}

// WithClientID sets the client ID for the NATS transport
func WithClientID(clientID string) NATSOption {
	return func(t *Transport) {
		t.clientID = clientID
	}
}

// WithCredentials sets the username and password for the NATS transport
func WithCredentials(username, password string) NATSOption {
	return func(t *Transport) {
		t.username = username
		t.password = password
	}
}

// WithToken sets the token for the NATS transport
func WithToken(token string) NATSOption {
	return func(t *Transport) {
		t.token = token
	}
}

// WithSubjectPrefix sets the subject prefix for the NATS transport
func WithSubjectPrefix(prefix string) NATSOption {
	return func(t *Transport) {
		t.subjectPrefix = prefix
	}
}

// WithServerSubject sets the server subject for the NATS transport
func WithServerSubject(subject string) NATSOption {
	return func(t *Transport) {
		t.serverSubject = subject
	}
}

// WithClientSubject sets the client subject for the NATS transport
func WithClientSubject(subject string) NATSOption {
	return func(t *Transport) {
		t.clientSubject = subject
	}
}

// WithTLS sets the TLS configuration for the NATS transport
func WithTLS(config TLSConfig) NATSOption {
	return func(t *Transport) {
		t.tlsConfig = &config
	}
}
