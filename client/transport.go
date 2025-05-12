package client

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/localrivet/gomcp/protocol"
)

// ClientTransport handles the actual communication with the server
type ClientTransport interface {
	// Connection management
	Connect(ctx context.Context) error
	Close() error
	IsConnected() bool

	// Synchronous request/response
	SendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error)

	// Asynchronous request/response and notification handling
	SendRequestAsync(ctx context.Context, req *protocol.JSONRPCRequest, responseCh chan<- *protocol.JSONRPCResponse) error
	SetNotificationHandler(handler NotificationHandler)

	// Transport-specific properties
	GetTransportType() TransportType
	GetTransportInfo() map[string]interface{}
}

// TransportType represents the type of transport
type TransportType string

// Transport types
const (
	TransportTypeSSE       TransportType = "sse"
	TransportTypeWebSocket TransportType = "websocket"
	TransportTypeStdio     TransportType = "stdio"
	TransportTypeInMemory  TransportType = "inmemory"
	TransportTypeTCP       TransportType = "tcp"
)

// TransportOption configures a transport
type TransportOption func(options *TransportOptions)

// TransportOptions holds configuration for transports
type TransportOptions struct {
	Headers        http.Header
	AuthProvider   AuthProvider
	HTTPClient     *http.Client
	RequestTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	KeepAlive      bool
	ConnectTimeout time.Duration
	RetryStrategy  BackoffStrategy

	// SSE-specific options
	SSELastEventID string

	// WebSocket-specific options
	WebSocketProtocols   []string
	WebSocketCompression bool

	// Stdio-specific options
	StdioReader    io.Reader
	StdioWriter    io.Writer
	StdioCloseFunc func() error

	// InMemory-specific options
	InMemoryServer interface{} // Will be *server.Server in real usage
}

// WithHeaders sets the HTTP headers for the transport
func WithHeaders(headers http.Header) TransportOption {
	return func(options *TransportOptions) {
		options.Headers = headers
	}
}

// WithHTTPClient sets the HTTP client for the transport
func WithHTTPClient(client *http.Client) TransportOption {
	return func(options *TransportOptions) {
		options.HTTPClient = client
	}
}

// WithRequestTimeout sets the request timeout for the transport
func WithRequestTimeout(timeout time.Duration) TransportOption {
	return func(options *TransportOptions) {
		options.RequestTimeout = timeout
	}
}

// WithReadTimeout sets the read timeout for the transport
func WithReadTimeout(timeout time.Duration) TransportOption {
	return func(options *TransportOptions) {
		options.ReadTimeout = timeout
	}
}

// WithWriteTimeout sets the write timeout for the transport
func WithWriteTimeout(timeout time.Duration) TransportOption {
	return func(options *TransportOptions) {
		options.WriteTimeout = timeout
	}
}

// WithKeepAlive sets whether to use keep-alive for the transport
func WithKeepAlive(keepAlive bool) TransportOption {
	return func(options *TransportOptions) {
		options.KeepAlive = keepAlive
	}
}

// WithConnectTimeout sets the connection timeout for the transport
func WithConnectTimeout(timeout time.Duration) TransportOption {
	return func(options *TransportOptions) {
		options.ConnectTimeout = timeout
	}
}

// WithTransportAuth sets the authentication provider for the transport
func WithTransportAuth(authProvider AuthProvider) TransportOption {
	return func(options *TransportOptions) {
		options.AuthProvider = authProvider
	}
}

// WithTransportRetryStrategy sets the retry strategy for the transport
func WithTransportRetryStrategy(strategy BackoffStrategy) TransportOption {
	return func(options *TransportOptions) {
		options.RetryStrategy = strategy
	}
}

// SSE-specific options

// WithSSELastEventID sets the last event ID for the SSE transport
func WithSSELastEventID(id string) TransportOption {
	return func(options *TransportOptions) {
		options.SSELastEventID = id
	}
}

// WebSocket-specific options

// WithWebSocketProtocols sets the WebSocket protocols
func WithWebSocketProtocols(protocols []string) TransportOption {
	return func(options *TransportOptions) {
		options.WebSocketProtocols = protocols
	}
}

// WithWebSocketCompression enables or disables WebSocket compression
func WithWebSocketCompression(compression bool) TransportOption {
	return func(options *TransportOptions) {
		options.WebSocketCompression = compression
	}
}

// Stdio-specific options

// WithStdioReaderWriter sets the reader and writer for the stdio transport
func WithStdioReaderWriter(reader io.Reader, writer io.Writer, closeFunc func() error) TransportOption {
	return func(options *TransportOptions) {
		options.StdioReader = reader
		options.StdioWriter = writer
		options.StdioCloseFunc = closeFunc
	}
}

// InMemory-specific options

// WithInMemoryServer sets the server for the in-memory transport
func WithInMemoryServer(server interface{}) TransportOption {
	return func(options *TransportOptions) {
		options.InMemoryServer = server
	}
}

// DefaultTransportOptions returns the default transport options
func DefaultTransportOptions() *TransportOptions {
	return &TransportOptions{
		Headers:        make(http.Header),
		HTTPClient:     http.DefaultClient,
		RequestTimeout: 30 * time.Second,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		KeepAlive:      true,
		ConnectTimeout: 10 * time.Second,
		RetryStrategy:  NewExponentialBackoff(100*time.Millisecond, 5*time.Second, 3),
	}
}
