package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	natst "github.com/localrivet/gomcp/transport/nats"
	"github.com/nats-io/nats.go"
)

// NATSTransport implements the Transport interface for NATS.
type NATSTransport struct {
	serverURL           string
	clientID            string
	conn                *nats.Conn
	subjectPrefix       string
	serverSubject       string
	clientSubject       string
	username            string
	password            string
	token               string
	tlsConfig           *natst.TLSConfig
	requestTimeout      time.Duration
	connectionTimeout   time.Duration
	notificationChan    chan natsNotification
	notificationHandler func(method string, params []byte)
	done                chan struct{}
	connected           bool
	connMu              sync.RWMutex
	responseSubs        map[string]*nats.Subscription
	responseSubsMu      sync.RWMutex
}

type natsNotification struct {
	Method string
	Params []byte
}

// NATSTransportOption represents a configuration option for the NATS transport
type NATSTransportOption func(*NATSTransport)

// NewNATSTransport creates a new NATS transport.
func NewNATSTransport(serverURL string, options ...NATSTransportOption) *NATSTransport {
	t := &NATSTransport{
		serverURL:         serverURL,
		subjectPrefix:     natst.DefaultSubjectPrefix,
		serverSubject:     natst.DefaultServerSubject,
		clientSubject:     natst.DefaultClientSubject,
		requestTimeout:    30 * time.Second,
		connectionTimeout: 10 * time.Second,
		notificationChan:  make(chan natsNotification, 100),
		done:              make(chan struct{}),
		responseSubs:      make(map[string]*nats.Subscription),
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
func (t *NATSTransport) Connect() error {
	// Create NATS connection options
	opts := []nats.Option{
		nats.Name(t.clientID),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1), // Infinite reconnects
		nats.Timeout(t.connectionTimeout),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			t.connMu.Lock()
			t.connected = false
			t.connMu.Unlock()
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			t.connMu.Lock()
			t.connected = true
			t.connMu.Unlock()
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
		return fmt.Errorf("failed to connect to NATS server: %w", err)
	}

	t.connMu.Lock()
	t.connected = true
	t.connMu.Unlock()

	// Subscribe to client's response subject for notifications
	responseSubject := fmt.Sprintf("%s.%s.%s", t.subjectPrefix, t.clientSubject, t.clientID)
	sub, err := t.conn.Subscribe(responseSubject, t.notificationMessageHandler)
	if err != nil {
		t.conn.Close()
		return fmt.Errorf("failed to subscribe to response subject: %w", err)
	}

	// Store the subscription
	t.responseSubsMu.Lock()
	t.responseSubs["notification"] = sub
	t.responseSubsMu.Unlock()

	// Start notification handler goroutine
	go t.handleNotifications()

	return nil
}

// ConnectWithContext implements the Transport.ConnectWithContext method.
func (t *NATSTransport) ConnectWithContext(ctx context.Context) error {
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
func (t *NATSTransport) Disconnect() error {
	close(t.done)

	t.responseSubsMu.Lock()
	for _, sub := range t.responseSubs {
		if sub != nil {
			sub.Unsubscribe()
		}
	}
	t.responseSubs = make(map[string]*nats.Subscription)
	t.responseSubsMu.Unlock()

	t.connMu.Lock()
	defer t.connMu.Unlock()

	// Disconnect NATS client
	if t.conn != nil && t.conn.IsConnected() {
		t.conn.Close()
	}

	t.connected = false
	return nil
}

// Send implements the Transport.Send method.
func (t *NATSTransport) Send(message []byte) ([]byte, error) {
	return t.SendWithContext(context.Background(), message)
}

// SendWithContext implements the Transport.SendWithContext method.
func (t *NATSTransport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	t.connMu.RLock()
	connected := t.connected
	t.connMu.RUnlock()

	if !connected {
		return nil, errors.New("not connected to NATS server")
	}

	// Parse the message to get the request ID
	var requestMap map[string]interface{}
	if err := json.Unmarshal(message, &requestMap); err != nil {
		return nil, fmt.Errorf("invalid JSON message: %w", err)
	}

	// Use NATS request-reply pattern
	subject := fmt.Sprintf("%s.%s", t.subjectPrefix, t.serverSubject)

	// Create a context with timeout for the request
	reqCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, t.requestTimeout)
		defer cancel()
	}

	// Send the request and wait for the response
	msg, err := t.conn.RequestWithContext(reqCtx, subject, message)
	if err != nil {
		return nil, err
	}

	return msg.Data, nil
}

// SetRequestTimeout implements the Transport.SetRequestTimeout method.
func (t *NATSTransport) SetRequestTimeout(timeout time.Duration) {
	t.requestTimeout = timeout
}

// SetConnectionTimeout implements the Transport.SetConnectionTimeout method.
func (t *NATSTransport) SetConnectionTimeout(timeout time.Duration) {
	t.connectionTimeout = timeout
}

// RegisterNotificationHandler implements the Transport.RegisterNotificationHandler method.
func (t *NATSTransport) RegisterNotificationHandler(handler func(method string, params []byte)) {
	t.notificationHandler = handler
}

// notificationMessageHandler processes incoming NATS messages for notifications (non-request/response)
func (t *NATSTransport) notificationMessageHandler(msg *nats.Msg) {
	var jsonMsg map[string]interface{}
	if err := json.Unmarshal(msg.Data, &jsonMsg); err != nil {
		return // Invalid JSON
	}

	// Check if this is a notification (has method but no id)
	method, hasMethod := jsonMsg["method"].(string)
	_, hasID := jsonMsg["id"]

	if hasMethod && !hasID {
		// This is a notification
		params, _ := json.Marshal(jsonMsg["params"])
		t.notificationChan <- natsNotification{
			Method: method,
			Params: params,
		}
	}
}

// handleNotifications processes notifications in a dedicated goroutine
func (t *NATSTransport) handleNotifications() {
	for {
		select {
		case notification := <-t.notificationChan:
			if t.notificationHandler != nil {
				t.notificationHandler(notification.Method, notification.Params)
			}
		case <-t.done:
			return
		}
	}
}

// WithNATSClientID sets the client ID for the NATS transport
func WithNATSClientID(clientID string) NATSTransportOption {
	return func(t *NATSTransport) {
		t.clientID = clientID
	}
}

// WithNATSCredentials sets the username and password for the NATS transport
func WithNATSCredentials(username, password string) NATSTransportOption {
	return func(t *NATSTransport) {
		t.username = username
		t.password = password
	}
}

// WithNATSToken sets the token for the NATS transport
func WithNATSToken(token string) NATSTransportOption {
	return func(t *NATSTransport) {
		t.token = token
	}
}

// WithNATSSubjectPrefix sets the subject prefix for the NATS transport
func WithNATSSubjectPrefix(prefix string) NATSTransportOption {
	return func(t *NATSTransport) {
		t.subjectPrefix = prefix
	}
}

// WithNATSTLS sets the TLS configuration for the NATS transport
func WithNATSTLS(config *natst.TLSConfig) NATSTransportOption {
	return func(t *NATSTransport) {
		t.tlsConfig = config
	}
}
