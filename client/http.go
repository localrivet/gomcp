package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPOption is a function that configures an HTTP transport.
// These options allow customizing the behavior of the HTTP client connection.
type HTTPOption func(*httpConfig)

// httpConfig holds configuration for HTTP transport.
// These settings control the behavior of the HTTP client connection.
type httpConfig struct {
	url           string
	client        *http.Client
	headers       map[string]string
	timeout       time.Duration
	pollInterval  time.Duration
	retryAttempts int
	retryDelay    time.Duration
}

// WithHTTPClient sets a custom HTTP client for the HTTP transport.
func WithHTTPClient(client *http.Client) HTTPOption {
	return func(cfg *httpConfig) {
		cfg.client = client
	}
}

// WithHTTPHeader adds a custom header to HTTP requests.
func WithHTTPHeader(key, value string) HTTPOption {
	return func(cfg *httpConfig) {
		if cfg.headers == nil {
			cfg.headers = make(map[string]string)
		}
		cfg.headers[key] = value
	}
}

// WithHTTPHeaders sets multiple custom headers for HTTP requests.
func WithHTTPHeaders(headers map[string]string) HTTPOption {
	return func(cfg *httpConfig) {
		cfg.headers = headers
	}
}

// WithHTTPPollInterval sets the interval for long-polling in HTTP transport.
func WithHTTPPollInterval(interval time.Duration) HTTPOption {
	return func(cfg *httpConfig) {
		cfg.pollInterval = interval
	}
}

// WithHTTPRetry configures retry behavior for HTTP requests.
func WithHTTPRetry(attempts int, delay time.Duration) HTTPOption {
	return func(cfg *httpConfig) {
		cfg.retryAttempts = attempts
		cfg.retryDelay = delay
	}
}

// WithHTTPTimeout sets a specific timeout for HTTP operations.
func WithHTTPTimeout(timeout time.Duration) HTTPOption {
	return func(cfg *httpConfig) {
		cfg.timeout = timeout
	}
}

// withHTTPTransport creates an adapter that implements the Transport interface
// for HTTP communication.
func withHTTPTransport(cfg *httpConfig) Transport {
	// Basic wrapper for HTTP transport
	return &httpTransport{
		url:               cfg.url,
		client:            cfg.client,
		requestTimeout:    cfg.timeout,
		connectionTimeout: cfg.timeout,
		headers:           cfg.headers,
	}
}

// WithHTTP configures the client to use HTTP transport for communication.
//
// Parameters:
// - url: The endpoint URL (e.g., "http://localhost:8080/mcp")
// - options: Optional configuration settings
//
// Example:
//
//	client.New(
//	    client.WithHTTP("http://localhost:8080/mcp"),
//	    // or with options:
//	    client.WithHTTP("http://localhost:8080/mcp",
//	        client.WithHTTPHeader("Authorization", "Bearer token"),
//	        client.WithHTTPTimeout(10 * time.Second))
//	)
func WithHTTP(url string, options ...HTTPOption) Option {
	return func(c *clientImpl) {
		// Create default config
		cfg := &httpConfig{
			url:           url,
			timeout:       30 * time.Second,
			pollInterval:  2 * time.Second,
			retryAttempts: 3,
			retryDelay:    500 * time.Millisecond,
			client:        &http.Client{Timeout: 30 * time.Second},
		}

		// Apply options
		for _, option := range options {
			option(cfg)
		}

		// Set the transport
		c.transport = withHTTPTransport(cfg)

		// Store timeouts in client for consistency
		c.requestTimeout = cfg.timeout
		c.connectionTimeout = cfg.timeout
	}
}

// httpTransport implements the Transport interface for HTTP communication.
type httpTransport struct {
	url                 string
	client              *http.Client
	requestTimeout      time.Duration
	connectionTimeout   time.Duration
	notificationHandler func(method string, params []byte)
	headers             map[string]string
}

// Connect implements the Transport interface.
func (t *httpTransport) Connect() error {
	// HTTP doesn't need a persistent connection
	return nil
}

// ConnectWithContext implements the Transport interface.
func (t *httpTransport) ConnectWithContext(ctx context.Context) error {
	// HTTP doesn't need a persistent connection
	return nil
}

// Disconnect implements the Transport interface.
func (t *httpTransport) Disconnect() error {
	// Nothing to disconnect for HTTP
	return nil
}

// Send implements the Transport interface.
func (t *httpTransport) Send(message []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), t.requestTimeout)
	defer cancel()
	return t.SendWithContext(ctx, message)
}

// SendWithContext implements the Transport interface.
func (t *httpTransport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	// Prepare the request
	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewReader(message))
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	// Send the request
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// SetRequestTimeout implements the Transport interface.
func (t *httpTransport) SetRequestTimeout(timeout time.Duration) {
	t.requestTimeout = timeout
	if t.client != nil {
		t.client.Timeout = timeout
	}
}

// SetConnectionTimeout implements the Transport interface.
func (t *httpTransport) SetConnectionTimeout(timeout time.Duration) {
	t.connectionTimeout = timeout
}

// RegisterNotificationHandler implements the Transport interface.
func (t *httpTransport) RegisterNotificationHandler(handler func(method string, params []byte)) {
	t.notificationHandler = handler
}
