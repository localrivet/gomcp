package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	httptransport "github.com/localrivet/gomcp/transport/http"
)

// HTTPTransportAdapter adapts the HTTP transport to the client Transport interface.
type HTTPTransportAdapter struct {
	transport           *httptransport.Transport
	requestTimeout      time.Duration
	connectionTimeout   time.Duration
	notificationHandler func(method string, params []byte)
	client              *http.Client
	connected           bool
}

// NewHTTPTransportAdapter creates a new HTTP transport adapter.
func NewHTTPTransportAdapter(url string) *HTTPTransportAdapter {
	transport := httptransport.NewTransport(url)
	return &HTTPTransportAdapter{
		transport:         transport,
		requestTimeout:    30 * time.Second,
		connectionTimeout: 10 * time.Second,
		connected:         false,
		client:            &http.Client{Timeout: 30 * time.Second},
	}
}

// Connect implements the Transport interface Connect method.
func (t *HTTPTransportAdapter) Connect() error {
	return t.ConnectWithContext(context.Background())
}

// ConnectWithContext implements the Transport interface ConnectWithContext method.
func (t *HTTPTransportAdapter) ConnectWithContext(ctx context.Context) error {
	// HTTP transport doesn't require a persistent connection,
	// but we'll make a test request to ensure the server is accessible
	// Create a simple ping request
	req, err := http.NewRequestWithContext(ctx, "GET", t.transport.GetAddr(), nil)
	if err != nil {
		return err
	}

	// Try to send the request
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Mark as connected
	t.connected = true
	return nil
}

// Disconnect implements the Transport interface Disconnect method.
func (t *HTTPTransportAdapter) Disconnect() error {
	t.connected = false
	return nil
}

// Send implements the Transport interface Send method.
func (t *HTTPTransportAdapter) Send(message []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), t.requestTimeout)
	defer cancel()
	return t.SendWithContext(ctx, message)
}

// SendWithContext implements the Transport interface SendWithContext method.
func (t *HTTPTransportAdapter) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	// Create a new request with the provided context
	req, err := http.NewRequestWithContext(ctx, "POST", t.transport.GetAddr(), bytes.NewReader(message))
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read the response body
	return io.ReadAll(resp.Body)
}

// SetRequestTimeout implements the Transport interface.
func (t *HTTPTransportAdapter) SetRequestTimeout(timeout time.Duration) {
	t.requestTimeout = timeout
	// Update the client timeout
	if t.client != nil {
		t.client.Timeout = timeout
	}
}

// SetConnectionTimeout implements the Transport interface.
func (t *HTTPTransportAdapter) SetConnectionTimeout(timeout time.Duration) {
	t.connectionTimeout = timeout
}

// RegisterNotificationHandler implements the Transport interface.
func (t *HTTPTransportAdapter) RegisterNotificationHandler(handler func(method string, params []byte)) {
	t.notificationHandler = handler
}

// SetClient sets a custom HTTP client.
func (t *HTTPTransportAdapter) SetClient(client *http.Client) {
	t.client = client
}

// GetAddr returns the transport's address.
func (t *HTTPTransportAdapter) GetAddr() string {
	return t.transport.GetAddr()
}

// AddHeader adds a custom header to all HTTP requests.
func (t *HTTPTransportAdapter) AddHeader(key, value string) {
	// This is a no-op for now as we don't have a direct method to add headers
	// We would need to add this to the HTTP transport implementation
}

// SetPollInterval sets the interval for HTTP long-polling.
func (t *HTTPTransportAdapter) SetPollInterval(interval time.Duration) {
	// This is a no-op for now as we don't have direct access to poll interval
}
