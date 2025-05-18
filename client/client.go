// Package client provides the client-side implementation of the MCP protocol.
//
// This package contains the Client interface and implementation for communicating with MCP services.
// It enables Go applications to interact with MCP servers through a clean, type-safe API that handles
// all aspects of the protocol, including version negotiation, connection management, and request/response
// handling.
//
// # Basic Usage
//
//	// Create a new client and connect to an MCP server
//	client, err := client.NewClient("my-client",
//		client.WithProtocolVersion("2025-03-26"),
//		client.WithLogger(logger),
//	)
//	if err != nil {
//		log.Fatalf("Failed to connect: %v", err)
//	}
//	defer client.Close()
//
//	// Call a tool
//	result, err := client.CallTool("calculate", map[string]interface{}{
//		"operation": "add",
//		"values": []float64{1.5, 2.5, 3.0},
//	})
//
// # Client Options
//
// The NewClient function accepts various options to customize client behavior:
//
//   - WithProtocolVersion: Set a specific protocol version
//   - WithProtocolNegotiation: Enable/disable automatic protocol negotiation
//   - WithLogger: Configure a custom logger
//   - WithTransport: Specify a custom transport implementation
//   - WithRequestTimeout: Set request timeout duration
//   - WithConnectionTimeout: Set connection timeout duration
//   - WithSamplingOptimizations: Configure sampling performance optimizations
//
// # Thread Safety
//
// All Client methods are thread-safe and can be called concurrently from multiple goroutines.
package client

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/localrivet/gomcp/mcp"
)

// Client represents an MCP client for communicating with MCP servers.
// It provides methods for all MCP operations including tool calls, resource access,
// prompt rendering, root management, and sampling functionality.
type Client interface {
	// CallTool invokes a tool on the connected MCP server.
	//
	// The name parameter specifies the tool to call. The args parameter contains
	// the arguments to pass to the tool as key-value pairs. The returned interface{}
	// contains the tool's output, which can be any JSON-serializable value.
	//
	// Example:
	//  result, err := client.CallTool("translate", map[string]interface{}{
	//      "text": "Hello world",
	//      "target_language": "Spanish",
	//  })
	CallTool(name string, args map[string]interface{}) (interface{}, error)

	// GetResource retrieves a resource from the server by its path.
	//
	// The path parameter specifies the resource to retrieve. The returned interface{}
	// contains the resource content, which can be any JSON-serializable value.
	//
	// Example:
	//  resource, err := client.GetResource("/users/123")
	GetResource(path string) (interface{}, error)

	// GetPrompt retrieves and renders a prompt from the server.
	//
	// The name parameter specifies the prompt to render. The variables parameter
	// contains the variables to substitute in the prompt template.
	//
	// Example:
	//  prompt, err := client.GetPrompt("greeting", map[string]interface{}{
	//      "name": "Alice",
	//      "time_of_day": "morning",
	//  })
	GetPrompt(name string, variables map[string]interface{}) (interface{}, error)

	// GetRoot retrieves the root resource from the server.
	//
	// This is a convenience method equivalent to calling GetResource("/").
	//
	// Example:
	//  root, err := client.GetRoot()
	GetRoot() (interface{}, error)

	// Close closes the client connection to the server and releases all resources.
	//
	// After calling Close, the client cannot be used for further operations.
	// It is good practice to defer this call after creating a client.
	//
	// Example:
	//  client, err := client.NewClient("my-client")
	//  if err != nil {
	//      log.Fatal(err)
	//  }
	//  defer client.Close()
	Close() error

	// AddRoot registers a new root endpoint with the server.
	//
	// The uri parameter specifies the path of the root. The name parameter
	// provides a human-readable name for the root.
	//
	// Example:
	//  err := client.AddRoot("/api/v2", "API Version 2")
	AddRoot(uri string, name string) error

	// RemoveRoot unregisters a root endpoint from the server.
	//
	// The uri parameter specifies the path of the root to remove.
	//
	// Example:
	//  err := client.RemoveRoot("/api/v1")
	RemoveRoot(uri string) error

	// GetRoots retrieves the list of root endpoints from the server.
	//
	// The returned slice contains all registered roots with their URIs and names.
	//
	// Example:
	//  roots, err := client.GetRoots()
	//  for _, root := range roots {
	//      fmt.Printf("Root: %s (%s)\n", root.URI, root.Name)
	//  }
	GetRoots() ([]Root, error)

	// Version returns the negotiated protocol version with the server.
	//
	// This returns one of the standardized version strings: "draft", "2024-11-05",
	// or "2025-03-26".
	//
	// Example:
	//  version := client.Version()
	//  fmt.Printf("Connected using MCP protocol version %s\n", version)
	Version() string

	// IsInitialized returns whether the client has been initialized.
	//
	// Initialization occurs during the first operation that requires
	// server communication.
	IsInitialized() bool

	// IsConnected returns whether the client is currently connected to the server.
	//
	// Example:
	//  if client.IsConnected() {
	//      fmt.Println("Client is connected to the server")
	//  } else {
	//      fmt.Println("Client is not connected")
	//  }
	IsConnected() bool

	// WithSamplingHandler registers a handler for sampling requests.
	//
	// The handler will be called when the server requests sampling (e.g., for LLM interactions).
	// Returns the client instance for method chaining.
	//
	// Example:
	//  client = client.WithSamplingHandler(func(params SamplingCreateMessageParams) (SamplingResponse, error) {
	//      // Process sampling request
	//      return SamplingResponse{...}, nil
	//  })
	WithSamplingHandler(handler SamplingHandler) Client

	// GetSamplingHandler returns the currently registered sampling handler.
	GetSamplingHandler() SamplingHandler

	// RequestSampling initiates a sampling request to the server.
	//
	// This is typically used by advanced clients that need to request
	// sampling capabilities from the server.
	RequestSampling(req *SamplingRequest) (*SamplingResponse, error)

	// RequestStreamingSampling initiates a streaming sampling request to the server.
	//
	// The streaming API is available only in protocol version 2025-03-26 and later.
	// The handler is called for each chunk of the streaming response.
	RequestStreamingSampling(req *StreamingSamplingRequest, handler StreamingResponseHandler) (*StreamingSamplingSession, error)
}

// clientImpl is the concrete implementation of the Client interface.
type clientImpl struct {
	url               string
	transport         Transport
	logger            *slog.Logger
	versionDetector   *mcp.VersionDetector
	negotiatedVersion string
	requestTimeout    time.Duration
	connectionTimeout time.Duration
	requestIDCounter  atomic.Int64
	initialized       bool
	connected         bool
	mu                sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	roots             []Root
	rootsMu           sync.RWMutex
	capabilities      ClientCapabilities
	samplingHandler   SamplingHandler

	// Server management
	serverRegistry *ServerRegistry
	serverName     string

	// Performance optimization fields
	samplingCache   *SamplingCache
	sizeAnalyzer    *ContentSizeAnalyzer
	samplingMetrics *SamplingPerformanceMetrics
}

// NewClient creates a new MCP client with the given URL and options.
// The client will automatically detect and adapt to the server's MCP specification version.
// It immediately establishes a connection to the server and returns an error if the connection fails.
//
// The url parameter is interpreted based on its format:
//   - "stdio:///": Uses Standard I/O for communication (useful for child processes)
//   - "ws://host:port/path": Uses WebSocket protocol
//   - "http://host:port/path": Uses HTTP protocol
//   - "sse://host:port/path": Uses Server-Sent Events protocol
//   - Custom schemes can be handled with a custom Transport implementation
//
// Errors returned by NewClient may include:
//   - Connection failures (e.g., server unreachable)
//   - Protocol negotiation failures
//   - Transport initialization errors
//
// Example:
//
//	// Basic client with default options
//	client, err := client.NewClient("ws://localhost:8080/mcp")
//	if err != nil {
//		log.Fatalf("Failed to create client: %v", err)
//	}
//
//	// Client with custom options
//	client, err := client.NewClient("http://api.example.com/mcp",
//		client.WithProtocolVersion("2025-03-26"),
//		client.WithLogger(myCustomLogger),
//		client.WithRequestTimeout(time.Second * 20),
//	)
func NewClient(url string, options ...Option) (Client, error) {
	ctx, cancel := context.WithCancel(context.Background())

	c := &clientImpl{
		url:               url,
		logger:            slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
		versionDetector:   mcp.NewVersionDetector(),
		requestTimeout:    30 * time.Second,
		connectionTimeout: 10 * time.Second,
		ctx:               ctx,
		cancel:            cancel,
		roots:             []Root{},
		capabilities: ClientCapabilities{
			Roots: RootsCapability{
				ListChanged: true,
			},
		},
	}

	// Apply options
	for _, option := range options {
		option(c)
	}

	// If no transport is provided, one will be selected based on the URL
	// when Connect() is called

	// Immediately connect to the server
	if err := c.Connect(); err != nil {
		cancel() // Clean up resources
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	return c, nil
}

// generateRequestID generates a unique request ID.
func (c *clientImpl) generateRequestID() int64 {
	return c.requestIDCounter.Add(1)
}

// Version returns the negotiated protocol version.
func (c *clientImpl) Version() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.negotiatedVersion
}

// IsInitialized returns whether the client has been initialized.
func (c *clientImpl) IsInitialized() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.initialized
}

// IsConnected returns whether the client is connected.
func (c *clientImpl) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}
