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
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/localrivet/gomcp/events"
	"github.com/localrivet/gomcp/mcp"
)

// RequestOption represents an option that can be passed to client methods.
type RequestOption interface {
	apply()
}

// TimeoutOption creates a request option that sets a timeout for the request.
type TimeoutOption struct {
	Duration time.Duration
}

func (t TimeoutOption) apply() {}

// WithRequestTimeoutOption creates a TimeoutOption from a duration.
func WithRequestTimeoutOption(d time.Duration) TimeoutOption {
	return TimeoutOption{Duration: d}
}

// ResourceParamsOption creates a request option that adds parameters to a resource request.
type ResourceParamsOption struct {
	Params map[string]interface{}
}

func (r ResourceParamsOption) apply() {}

// WithResourceParams creates a ResourceParamsOption for adding parameters to GetResource requests.
//
// This allows passing additional parameters alongside the URI in resources/read requests
// as specified in the MCP protocol.
//
// Example:
//
//	resource, err := client.GetResource("/api/users",
//	    client.WithResourceParams(map[string]interface{}{
//	        "include_posts": true,
//	        "limit": 50,
//	    }),
//	    client.WithRequestTimeoutOption(10*time.Second),
//	)
func WithResourceParams(params map[string]interface{}) ResourceParamsOption {
	return ResourceParamsOption{Params: params}
}

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
	//
	// For timeout support:
	//  result, err := client.CallTool("translate", map[string]interface{}{
	//      "text": "Hello world",
	//      "target_language": "Spanish",
	//  }, client.WithRequestTimeoutOption(10*time.Second))
	CallTool(name string, args map[string]interface{}, opts ...RequestOption) (interface{}, error)

	// GetResource retrieves a resource from the server.
	//
	// The path parameter specifies the resource URI to retrieve.
	//
	// Example:
	//  resource, err := client.GetResource("/files/readme.txt")
	//  // Access content based on protocol version:
	//  if len(resource.Content) > 0 {
	//      // 2024-11-05 format
	//      fmt.Println("Text:", resource.Content[0].Text)
	//  } else if len(resource.Contents) > 0 {
	//      // 2025-03-26 format
	//      fmt.Println("Text:", resource.Contents[0].Content[0].Text)
	//  }
	//
	// For timeout support:
	//  resource, err := client.GetResource("/files/readme.txt",
	//      client.WithRequestTimeoutOption(5*time.Second))
	//
	// For passing additional parameters:
	//  resource, err := client.GetResource("/api/users",
	//      client.WithResourceParams(map[string]interface{}{
	//          "include_posts": true,
	//          "limit": 50,
	//      }),
	//      client.WithRequestTimeoutOption(5*time.Second))
	GetResource(path string, opts ...RequestOption) (*ResourceResponse, error)

	// GetPrompt retrieves a prompt from the server.
	//
	// The name parameter specifies the prompt to retrieve. The variables parameter
	// contains any variables to substitute in the prompt template.
	//
	// Example:
	//  prompt, err := client.GetPrompt("greeting", map[string]interface{}{
	//      "name": "Alice",
	//  })
	//
	// For timeout support:
	//  prompt, err := client.GetPrompt("greeting", map[string]interface{}{
	//      "name": "Alice",
	//  }, client.WithRequestTimeoutOption(3*time.Second))
	GetPrompt(name string, variables map[string]interface{}, opts ...RequestOption) (*PromptResponse, error)

	// GetRoot retrieves the root resource from the server.
	//
	// This is a convenience method equivalent to calling GetResource("/").
	//
	// Example:
	//  root, err := client.GetRoot()
	//  // Access content based on protocol version:
	//  if len(root.Content) > 0 {
	//      // 2024-11-05 format
	//      fmt.Println("Root text:", root.Content[0].Text)
	//  } else if len(root.Contents) > 0 {
	//      // 2025-03-26 format
	//      fmt.Println("Root text:", root.Contents[0].Content[0].Text)
	//  }
	GetRoot() (*ResourceResponse, error)

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

	// ListTools retrieves the list of available tools from the server.
	//
	// This method calls the tools/list endpoint as specified in the MCP protocol.
	// It automatically handles pagination internally and returns all available tools.
	// The returned slice contains all available tools with their names, descriptions,
	// and input schemas, which can be used for tool discovery and proxy patterns.
	//
	// Example:
	//  tools, err := client.ListTools()
	//  for _, tool := range tools {
	//      fmt.Printf("Tool: %s - %s\n", tool.Name, tool.Description)
	//      fmt.Printf("Schema: %+v\n", tool.InputSchema)
	//  }
	ListTools() ([]Tool, error)

	// ListResources retrieves the list of available resources from the server.
	//
	// This method calls the resources/list endpoint as specified in the MCP protocol.
	// It automatically handles pagination internally and returns all available resources.
	// The returned slice contains all available resources with their URIs, names,
	// descriptions, and MIME types, which can be used for resource discovery.
	//
	// Example:
	//  resources, err := client.ListResources()
	//  for _, resource := range resources {
	//      fmt.Printf("Resource: %s - %s\n", resource.Name, resource.Description)
	//      fmt.Printf("URI: %s, MIME Type: %s\n", resource.URI, resource.MimeType)
	//  }
	ListResources(opts ...RequestOption) ([]Resource, error)

	// ListPrompts retrieves the list of available prompts from the server.
	//
	// This method calls the prompts/list endpoint as specified in the MCP protocol.
	// It automatically handles pagination internally and returns all available prompts.
	// The returned slice contains all available prompts with their names, descriptions,
	// and argument specifications, which can be used for prompt discovery.
	//
	// Example:
	//  prompts, err := client.ListPrompts()
	//  for _, prompt := range prompts {
	//      fmt.Printf("Prompt: %s - %s\n", prompt.Name, prompt.Description)
	//      fmt.Printf("Arguments: %d\n", len(prompt.Arguments))
	//  }
	ListPrompts(opts ...RequestOption) ([]Prompt, error)

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

	// Events returns the events subject for subscribing to client events.
	//
	// This provides access to the event system for monitoring client lifecycle,
	// errors, and other significant events. Subscribers can listen for specific
	// event types such as initialization, connection changes, and errors.
	//
	// Example:
	//  events := client.Events()
	//  events.Subscribe(events.TopicClientError, func(event events.ClientErrorEvent) {
	//      log.Printf("Client error: %s", event.Error)
	//  })
	Events() *events.Subject

	// RequestSampling initiates a sampling request to the server.
	//
	// This is the unified method for all sampling operations, supporting both
	// regular and streaming modes through options configuration.
	//
	// Example:
	//  opts := client.NewSamplingOptions(messages, prefs).
	//      WithSystemPrompt("You are a helpful assistant").
	//      WithMaxTokens(1000)
	//  response, err := client.RequestSampling(opts)
	//
	// For streaming (protocol version 2025-03-26 only):
	//  opts := client.NewSamplingOptions(messages, prefs).
	//      WithStreaming(func(chunk *SamplingResponse) error {
	//          fmt.Printf("Received chunk: %s\n", chunk.Content.Text)
	//          return nil
	//      })
	//  response, err := client.RequestSampling(opts)
	RequestSampling(opts *SamplingOptions) (*SamplingResponse, error)

	// SendBatch sends multiple requests to the server in a single batch operation.
	//
	// This method implements JSON-RPC 2.0 batch requests, allowing multiple operations
	// to be sent together for improved efficiency. The requests parameter contains
	// the individual requests to include in the batch.
	//
	// The method returns a slice of BatchResponse objects, where each response
	// corresponds to a request in the batch (excluding notifications). The order
	// of responses matches the order of requests.
	//
	// Example:
	//  requests := []BatchRequest{
	//      {Method: "tools/call", Params: map[string]interface{}{"name": "calculator", "arguments": map[string]interface{}{"op": "add", "a": 1, "b": 2}}},
	//      {Method: "resources/read", Params: map[string]interface{}{"uri": "/config.json"}},
	//      {Method: "prompts/get", Params: map[string]interface{}{"name": "greeting", "arguments": map[string]interface{}{"name": "Alice"}}},
	//  }
	//  responses, err := client.SendBatch(requests)
	//  for i, response := range responses {
	//      if response.Error != nil {
	//          fmt.Printf("Request %d failed: %v\n", i, response.Error)
	//      } else {
	//          fmt.Printf("Request %d result: %v\n", i, response.Result)
	//      }
	//  }
	//
	// For timeout support:
	//  responses, err := client.SendBatch(requests, client.WithRequestTimeoutOption(30*time.Second))
	SendBatch(requests []BatchRequest, opts ...RequestOption) ([]BatchResponse, error)

	// BatchBuilder creates a new batch builder for constructing batch requests.
	//
	// The batch builder provides a fluent interface for adding multiple requests
	// to a batch operation. Use AddRequest to add individual requests.
	//
	// Example:
	//  responses, err := client.BatchBuilder().
	//      AddRequest("tools/call", map[string]interface{}{
	//          "name": "calculator",
	//          "arguments": map[string]interface{}{"op": "add", "a": 1, "b": 2}
	//      }, 1).
	//      AddRequest("resources/read", map[string]interface{}{"uri": "/config.json"}, 2).
	//      AddRequest("prompts/get", map[string]interface{}{
	//          "name": "greeting",
	//          "arguments": map[string]interface{}{"name": "Alice"}
	//      }, 3).
	//      Execute()
	BatchBuilder() *BatchRequestBuilder

	// GetServerCapabilities returns the server's declared capabilities from initialization.
	//
	// This method returns the capabilities that the server declared during the MCP
	// initialization handshake. These capabilities indicate which optional protocol
	// features are supported by the server, such as resources, prompts, tools, and
	// logging. Returns nil if the client has not been initialized yet.
	//
	// Example:
	//  caps := client.GetServerCapabilities()
	//  if caps != nil && caps.Resources != nil {
	//      fmt.Println("Server supports resources")
	//      if caps.Resources.Subscribe {
	//          fmt.Println("Server supports resource subscriptions")
	//      }
	//  }
	GetServerCapabilities() *ServerCapabilities

	// GetServerInfo returns the server's identification information from initialization.
	//
	// This method returns the server information (name, version) that was provided
	// during the MCP initialization handshake. Returns nil if the client has not
	// been initialized yet.
	//
	// Example:
	//  info := client.GetServerInfo()
	//  if info != nil {
	//      fmt.Printf("Connected to %s version %s\n", info.Name, info.Version)
	//  }
	GetServerInfo() *ServerInfo

	// GetServerInstructions returns optional instructions provided by the server.
	//
	// This method returns any optional instructions that the server provided during
	// initialization (available in MCP protocol version 2025-03-26). Returns an
	// empty string if no instructions were provided or if using an older protocol version.
	//
	// Example:
	//  instructions := client.GetServerInstructions()
	//  if instructions != "" {
	//      fmt.Printf("Server instructions: %s\n", instructions)
	//  }
	GetServerInstructions() string

	// HasCapability checks if the server supports a specific top-level capability.
	//
	// This is a convenience method for checking server capability support. The
	// capability parameter should be one of: "logging", "prompts", "resources",
	// "tools", or "experimental". Returns false if the client has not been
	// initialized or if the capability is not supported.
	//
	// Example:
	//  if client.HasCapability("resources") {
	//      resources, err := client.ListResources()
	//      // ... handle resources
	//  }
	HasCapability(capability string) bool

	// SupportsResourceSubscriptions checks if the server supports resource change subscriptions.
	//
	// This is a convenience method for checking if the server's resource capability
	// includes the "subscribe" sub-capability, which allows clients to receive
	// notifications when specific resources change. Returns false if the server
	// doesn't support resources or subscriptions.
	//
	// Example:
	//  if client.SupportsResourceSubscriptions() {
	//      // Can subscribe to resource changes
	//  }
	SupportsResourceSubscriptions() bool

	// SupportsListChangedNotifications checks if the server supports list change notifications.
	//
	// This method checks if the server supports notifications when the list of items
	// of a specific type changes. The resourceType parameter should be one of:
	// "prompts", "resources", or "tools". Returns false if the server doesn't
	// support the specified resource type or list change notifications.
	//
	// Example:
	//  if client.SupportsListChangedNotifications("resources") {
	//      // Server will notify when the resource list changes
	//  }
	SupportsListChangedNotifications(resourceType string) bool

	// Ping sends a ping request to the server to verify connection health.
	Ping() error

	// WaitForReady waits for the client to be fully connected and ready to handle requests.
	//
	// This method blocks until the client is connected, initialized, and can successfully
	// ping the server, or until the timeout is reached. It's useful for ensuring the
	// server is fully ready before making API calls.
	//
	// Example:
	//  if err := client.WaitForReady(5 * time.Second); err != nil {
	//      log.Fatal("Server not ready:", err)
	//  }
	//  // Now safe to make API calls
	//  tools, err := client.ListTools()
	WaitForReady(timeout time.Duration) error

	// GetServerStatus returns the status of a managed MCP server.
	//
	// This method is only available for clients created with WithServersFromConfig
	// or WithServerConfig options. It returns the running status, any errors,
	// and process information for the specified server.
	//
	// Example:
	//  status := client.GetServerStatus("file-server")
	//  if !status.Running {
	//      log.Printf("File server is not running: %v", status.Error)
	//  }
	GetServerStatus(serverName string) *ServerStatus

	// RestartServer attempts to restart a managed MCP server.
	//
	// This method is only available for clients created with WithServersFromConfig
	// or WithServerConfig options. It stops the specified server if running and
	// starts it again with the same configuration.
	//
	// Example:
	//  err := client.RestartServer("file-server")
	//  if err != nil {
	//      log.Fatalf("Failed to restart server: %v", err)
	//  }
	RestartServer(serverName string) error
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
	rootsManager      *rootsManager
	capabilities      ClientCapabilities
	samplingHandler   SamplingHandler

	// Server capabilities and info (received during initialization)
	// Set once during initialization, protected by c.mu, never change after
	serverCapabilities *ServerCapabilities
	serverInfo         *ServerInfo
	serverInstructions string

	// Server management
	serverRegistry *ServerRegistry
	serverName     string

	// Events
	events *events.Subject
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
		capabilities: ClientCapabilities{
			Roots: RootsCapability{
				ListChanged: true,
			},
		},
		events: events.NewSubject(),
	}

	// Initialize the roots manager with the actor pattern
	c.rootsManager = newRootsManager(c)

	// Apply options
	for _, option := range options {
		option(c)
	}

	// Emit client initializing event
	go func() {
		if err := events.Publish[events.ClientInitializingEvent](c.events, events.TopicClientInitializing, events.ClientInitializingEvent{
			URL: url,
		}); err != nil {
			// Log the error but don't fail initialization
			c.logger.Warn("failed to publish client initializing event", "error", err)
		}
	}()

	// If no transport is provided, one will be selected based on the URL
	// when Connect() is called

	// Immediately connect to the server
	if err := c.Connect(); err != nil {
		cancel() // Clean up resources
		go func() {
			if pubErr := events.Publish[events.ClientErrorEvent](c.events, events.TopicClientError, events.ClientErrorEvent{
				Error: err.Error(),
			}); pubErr != nil {
				c.logger.Warn("failed to publish client error event", "error", pubErr)
			}
		}()
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	// Emit client initialized event
	go func() {
		if err := events.Publish[events.ClientInitializedEvent](c.events, events.TopicClientInitialized, events.ClientInitializedEvent{
			URL: url,
		}); err != nil {
			c.logger.Warn("failed to publish client initialized event", "error", err)
		}
	}()

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

// CallTool calls a tool on the server.
func (c *clientImpl) CallTool(name string, args map[string]interface{}, opts ...RequestOption) (interface{}, error) {
	timeout := c.extractTimeout(opts...)
	return c.sendRequestWithTimeout("tools/call", map[string]interface{}{
		"name":      name,
		"arguments": args,
	}, timeout)
}

// GetResource retrieves a resource from the server.
func (c *clientImpl) GetResource(uri string, opts ...RequestOption) (*ResourceResponse, error) {
	timeout := c.extractTimeout(opts...)
	resourceParams := c.extractResourceParams(opts...)

	// Build request parameters starting with the URI
	params := map[string]interface{}{
		"uri": uri,
	}

	// Add any additional resource parameters if provided
	for key, value := range resourceParams {
		params[key] = value
	}

	result, err := c.sendRequestWithTimeout("resources/read", params, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	// Parse the result into a ResourceResponse
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format from resources/read")
	}

	response := &ResourceResponse{}

	// Handle both 2024-11-05 and 2025-03-26 formats
	if contentArray, hasContent := resultMap["content"]; hasContent {
		// 2024-11-05 format: flat content array
		if contentItems, ok := contentArray.([]interface{}); ok {
			response.Content = make([]ContentItem, 0, len(contentItems))
			for _, item := range contentItems {
				if itemMap, ok := item.(map[string]interface{}); ok {
					contentItem := ContentItem{
						Type:     getString(itemMap, "type"),
						Text:     getString(itemMap, "text"),
						ImageURL: getString(itemMap, "imageUrl"),
						AltText:  getString(itemMap, "altText"),
						URL:      getString(itemMap, "url"),
						Title:    getString(itemMap, "title"),
						Blob:     getString(itemMap, "blob"),
						MimeType: getString(itemMap, "mimeType"),
						Filename: getString(itemMap, "filename"),
						Data:     itemMap["data"],
					}
					response.Content = append(response.Content, contentItem)
				}
			}
		}
	}

	if contentsArray, hasContents := resultMap["contents"]; hasContents {
		// 2025-03-26 format: contents array with embedded content
		if contentsItems, ok := contentsArray.([]interface{}); ok {
			response.Contents = make([]ResourceContent, 0, len(contentsItems))
			for _, item := range contentsItems {
				if itemMap, ok := item.(map[string]interface{}); ok {
					resourceContent := ResourceContent{
						URI:      getString(itemMap, "uri"),
						Text:     getString(itemMap, "text"),
						Metadata: getMap(itemMap, "metadata"),
					}

					// Parse embedded content array
					if contentArray, hasEmbeddedContent := itemMap["content"]; hasEmbeddedContent {
						if contentItems, ok := contentArray.([]interface{}); ok {
							resourceContent.Content = make([]ContentItem, 0, len(contentItems))
							for _, embeddedItem := range contentItems {
								if embeddedMap, ok := embeddedItem.(map[string]interface{}); ok {
									contentItem := ContentItem{
										Type:     getString(embeddedMap, "type"),
										Text:     getString(embeddedMap, "text"),
										ImageURL: getString(embeddedMap, "imageUrl"),
										AltText:  getString(embeddedMap, "altText"),
										URL:      getString(embeddedMap, "url"),
										Title:    getString(embeddedMap, "title"),
										Blob:     getString(embeddedMap, "blob"),
										MimeType: getString(embeddedMap, "mimeType"),
										Filename: getString(embeddedMap, "filename"),
										Data:     embeddedMap["data"],
									}
									resourceContent.Content = append(resourceContent.Content, contentItem)
								}
							}
						}
					}
					response.Contents = append(response.Contents, resourceContent)
				}
			}
		}
	}

	// Handle metadata
	if metadata, hasMetadata := resultMap["metadata"]; hasMetadata {
		if metadataMap, ok := metadata.(map[string]interface{}); ok {
			response.Metadata = metadataMap
		}
	}

	return response, nil
}

// GetPrompt retrieves a prompt from the server.
func (c *clientImpl) GetPrompt(name string, variables map[string]interface{}, opts ...RequestOption) (*PromptResponse, error) {
	timeout := c.extractTimeout(opts...)

	params := map[string]interface{}{
		"name": name,
	}

	if variables != nil {
		params["arguments"] = variables
	}

	result, err := c.sendRequestWithTimeout("prompts/get", params, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt: %w", err)
	}

	// Parse the response into concrete types
	responseMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format from prompts/get")
	}

	promptResponse := &PromptResponse{
		Description: getString(responseMap, "description"),
	}

	// Parse messages
	if messagesData, ok := responseMap["messages"].([]interface{}); ok {
		promptResponse.Messages = make([]PromptMessage, 0, len(messagesData))
		for _, messageData := range messagesData {
			if messageMap, ok := messageData.(map[string]interface{}); ok {
				message := PromptMessage{
					Role: getString(messageMap, "role"),
				}

				// Parse content
				if contentData, ok := messageMap["content"].(map[string]interface{}); ok {
					message.Content = PromptContent{
						Type: getString(contentData, "type"),
						Text: getString(contentData, "text"),
					}
				}

				promptResponse.Messages = append(promptResponse.Messages, message)
			}
		}
	}

	return promptResponse, nil
}

// extractTimeout extracts a timeout duration from request options.
func (c *clientImpl) extractTimeout(opts ...RequestOption) time.Duration {
	if len(opts) > 0 {
		for _, opt := range opts {
			if timeout, ok := opt.(TimeoutOption); ok {
				return timeout.Duration
			}
		}
	}
	return c.requestTimeout
}

// extractResourceParams extracts resource parameters from request options.
func (c *clientImpl) extractResourceParams(opts ...RequestOption) map[string]interface{} {
	if len(opts) > 0 {
		for _, opt := range opts {
			if params, ok := opt.(ResourceParamsOption); ok {
				return params.Params
			}
		}
	}
	return nil
}

// GetRoot retrieves the root resource from the server.
func (c *clientImpl) GetRoot() (*ResourceResponse, error) {
	return c.GetResource("/")
}

// ListTools retrieves the list of available tools from the server.
func (c *clientImpl) ListTools() ([]Tool, error) {
	var allTools []Tool
	cursor := ""

	for {
		// Prepare parameters for the request
		params := map[string]interface{}{}
		if cursor != "" {
			params["cursor"] = cursor
		}

		// Send the tools/list request
		result, err := c.sendRequest("tools/list", params)
		if err != nil {
			return nil, fmt.Errorf("failed to list tools: %w", err)
		}

		c.logger.Debug(fmt.Sprintf("Tools list result: %#v", result))

		// Parse the response using struct-based unmarshaling
		var responseBytes []byte
		switch v := result.(type) {
		case nil: // Handle nil response (empty result)
			// For nil responses, create an empty JSON object to parse
			responseBytes = []byte("{}")
		case []byte: // If sendRequest returns raw JSON bytes
			responseBytes = v
		case string: // If sendRequest returns JSON as a string
			responseBytes = []byte(v)
		case map[string]interface{}: // If sendRequest returns a parsed generic JSON object
			// This is a common scenario. To use json.Unmarshal with our specific structs
			// (like the anonymous struct for apiData), we marshal this map back to
			// JSON bytes first. This allows encoding/json to use the struct tags
			// for proper mapping.
			var marshalErr error
			responseBytes, marshalErr = json.Marshal(v)
			if marshalErr != nil {
				return nil, fmt.Errorf("failed to re-marshal response for parsing: %w", marshalErr)
			}
		default:
			return nil, fmt.Errorf("unexpected response type from sendRequest: %T", result)
		}

		// Use the anonymous struct for unmarshaling with proper JSON tags
		var apiData struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor,omitempty"`
		}

		if err := json.Unmarshal(responseBytes, &apiData); err != nil {
			return nil, fmt.Errorf("failed to parse tools/list response: %w", err)
		}

		// Add the tools to our collection
		allTools = append(allTools, apiData.Tools...)

		// Check if there are more pages
		if apiData.NextCursor == "" {
			break
		}
		cursor = apiData.NextCursor
	}

	return allTools, nil
}

// ListResources retrieves the list of available resources from the server.
func (c *clientImpl) ListResources(opts ...RequestOption) ([]Resource, error) {
	var allResources []Resource
	cursor := ""

	for {
		// Prepare parameters for the request
		params := map[string]interface{}{}
		if cursor != "" {
			params["cursor"] = cursor
		}

		// Send the resources/list request
		result, err := c.sendRequest("resources/list", params)
		if err != nil {
			return nil, fmt.Errorf("failed to list resources: %w", err)
		}

		// Parse the response
		response, ok := result.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid response format from resources/list")
		}

		// Extract resources from the response
		resourcesData, ok := response["resources"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid resources format in response")
		}

		// Convert each resource to our Resource struct
		for _, resourceData := range resourcesData {
			resourceMap, ok := resourceData.(map[string]interface{})
			if !ok {
				continue
			}

			resource := Resource{
				URI:         getString(resourceMap, "uri"),
				Name:        getString(resourceMap, "name"),
				Description: getString(resourceMap, "description"),
				MimeType:    getString(resourceMap, "mimeType"),
				Annotations: getMap(resourceMap, "annotations"),
			}

			allResources = append(allResources, resource)
		}

		// Check if there are more pages
		nextCursor, hasMore := response["nextCursor"].(string)
		if !hasMore || nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return allResources, nil
}

// ListPrompts retrieves the list of available prompts from the server.
//
// This method calls the prompts/list endpoint as specified in the MCP protocol.
// It automatically handles pagination internally and returns all available prompts.
// The returned slice contains all available prompts with their names, descriptions,
// and argument specifications, which can be used for prompt discovery.
//
// Example:
//
//	prompts, err := client.ListPrompts()
//	for _, prompt := range prompts {
//	    fmt.Printf("Prompt: %s - %s\n", prompt.Name, prompt.Description)
//	    fmt.Printf("Arguments: %d\n", len(prompt.Arguments))
//	}
func (c *clientImpl) ListPrompts(opts ...RequestOption) ([]Prompt, error) {
	var allPrompts []Prompt
	cursor := ""

	for {
		// Prepare parameters for the request
		params := map[string]interface{}{}
		if cursor != "" {
			params["cursor"] = cursor
		}

		// Send the prompts/list request
		result, err := c.sendRequest("prompts/list", params)
		if err != nil {
			return nil, fmt.Errorf("failed to list prompts: %w", err)
		}

		// Parse the response using struct-based unmarshaling
		var responseBytes []byte
		switch v := result.(type) {
		case nil: // Handle nil response (empty result)
			// For nil responses, create an empty JSON object to parse
			responseBytes = []byte("{}")
		case []byte: // If sendRequest returns raw JSON bytes
			responseBytes = v
		case string: // If sendRequest returns JSON as a string
			responseBytes = []byte(v)
		case map[string]interface{}: // If sendRequest returns a parsed generic JSON object
			// This is a common scenario. To use json.Unmarshal with our specific structs
			// (like the anonymous struct for apiData), we marshal this map back to
			// JSON bytes first. This allows encoding/json to use the struct tags
			// for proper mapping.
			var marshalErr error
			responseBytes, marshalErr = json.Marshal(v)
			if marshalErr != nil {
				return nil, fmt.Errorf("failed to re-marshal response for parsing: %w", marshalErr)
			}
		default:
			return nil, fmt.Errorf("unexpected response type from sendRequest: %T", result)
		}

		// Use the anonymous struct for unmarshaling with proper JSON tags
		var apiData struct {
			Prompts    []Prompt `json:"prompts"`
			NextCursor string   `json:"nextCursor,omitempty"`
		}

		if err := json.Unmarshal(responseBytes, &apiData); err != nil {
			return nil, fmt.Errorf("failed to parse prompts/list response: %w", err)
		}

		// Add the prompts to our collection
		allPrompts = append(allPrompts, apiData.Prompts...)

		// Check if there are more pages
		if apiData.NextCursor == "" {
			break
		}
		cursor = apiData.NextCursor
	}

	return allPrompts, nil
}

// Helper functions for safe type conversion
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key].(map[string]interface{}); ok {
		return val
	}
	return nil
}

// SendBatch sends multiple requests to the server in a single batch operation.
func (c *clientImpl) SendBatch(requests []BatchRequest, opts ...RequestOption) ([]BatchResponse, error) {
	if len(requests) == 0 {
		return []BatchResponse{}, nil
	}

	timeout := c.extractTimeout(opts...)

	// Convert BatchRequest to JSON-RPC format using structured types
	var jsonRPCRequests []*mcp.JSONRPCRequest
	for _, req := range requests {
		var id interface{}
		if req.ID != nil {
			id = req.ID
		}

		jsonReq := mcp.NewRequest(id, req.Method, req.Params)
		jsonRPCRequests = append(jsonRPCRequests, jsonReq)
	}

	// Send the batch request
	result, err := c.sendBatchRequestWithTimeout(jsonRPCRequests, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to send batch request: %w", err)
	}

	// Handle the case where all requests were notifications (no response)
	if result == nil {
		return []BatchResponse{}, nil
	}

	// Parse the batch response
	responseArray, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid batch response format")
	}

	var responses []BatchResponse
	for _, respData := range responseArray {
		respMap, ok := respData.(map[string]interface{})
		if !ok {
			continue
		}

		response := BatchResponse{
			ID: respMap["id"],
		}

		if result, hasResult := respMap["result"]; hasResult {
			response.Result = result
		}

		if errorData, hasError := respMap["error"].(map[string]interface{}); hasError {
			response.Error = &BatchError{
				Code:    int(errorData["code"].(float64)),
				Message: errorData["message"].(string),
			}
			if data, hasData := errorData["data"]; hasData {
				response.Error.Data = data
			}
		}

		responses = append(responses, response)
	}

	return responses, nil
}

// BatchBuilder creates a new batch builder for constructing batch requests.
func (c *clientImpl) BatchBuilder() *BatchRequestBuilder {
	return &BatchRequestBuilder{
		client:   c,
		requests: make([]BatchRequest, 0),
		nextID:   0,
	}
}

// sendBatchRequestWithTimeout sends a batch request with timeout support.
func (c *clientImpl) sendBatchRequestWithTimeout(requests []*mcp.JSONRPCRequest, timeout time.Duration) (interface{}, error) {
	// Use the existing transport mechanism but send as a batch
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()

	// Marshal the batch request
	requestBytes, err := json.Marshal(requests)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	// Send via transport
	responseBytes, err := c.transport.SendWithContext(ctx, requestBytes)
	if err != nil {
		return nil, fmt.Errorf("transport error: %w", err)
	}

	// Handle empty response (all notifications)
	if len(responseBytes) == 0 {
		return nil, nil
	}

	// Parse response
	var result interface{}
	if err := json.Unmarshal(responseBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse batch response: %w", err)
	}

	return result, nil
}

// Events returns the events subject for subscribing to client events.
func (c *clientImpl) Events() *events.Subject {
	return c.events
}

// Ping sends a ping request to the server to verify connection health.
func (c *clientImpl) Ping() error {
	_, err := c.sendRequestWithTimeout("ping", nil, c.requestTimeout)
	return err
}

// GetServerCapabilities returns the server's declared capabilities from initialization.
func (c *clientImpl) GetServerCapabilities() *ServerCapabilities {
	return c.serverCapabilities
}

// GetServerInfo returns the server's identification information from initialization.
func (c *clientImpl) GetServerInfo() *ServerInfo {
	return c.serverInfo
}

// GetServerInstructions returns optional instructions provided by the server.
func (c *clientImpl) GetServerInstructions() string {
	return c.serverInstructions
}

// HasCapability checks if the server supports a specific top-level capability.
func (c *clientImpl) HasCapability(capability string) bool {
	if c.serverCapabilities == nil {
		return false
	}

	switch capability {
	case "logging":
		return c.serverCapabilities.Logging != nil
	case "prompts":
		return c.serverCapabilities.Prompts != nil
	case "resources":
		return c.serverCapabilities.Resources != nil
	case "tools":
		return c.serverCapabilities.Tools != nil
	case "experimental":
		return c.serverCapabilities.Experimental != nil
	default:
		return false
	}
}

// SupportsResourceSubscriptions checks if the server supports resource change subscriptions.
func (c *clientImpl) SupportsResourceSubscriptions() bool {
	return c.serverCapabilities != nil &&
		c.serverCapabilities.Resources != nil &&
		c.serverCapabilities.Resources.Subscribe
}

// SupportsListChangedNotifications checks if the server supports list change notifications.
func (c *clientImpl) SupportsListChangedNotifications(resourceType string) bool {
	if c.serverCapabilities == nil {
		return false
	}

	switch resourceType {
	case "prompts":
		return c.serverCapabilities.Prompts != nil && c.serverCapabilities.Prompts.ListChanged
	case "resources":
		return c.serverCapabilities.Resources != nil && c.serverCapabilities.Resources.ListChanged
	case "tools":
		return c.serverCapabilities.Tools != nil && c.serverCapabilities.Tools.ListChanged
	default:
		return false
	}
}

// WaitForReady waits for the client to be fully connected and ready to handle requests.
//
// This method blocks until the client is connected, initialized, and can successfully
// ping the server, or until the timeout is reached. It's useful for ensuring the
// server is fully ready before making API calls.
//
// Example:
//
//	if err := client.WaitForReady(5 * time.Second); err != nil {
//	    log.Fatal("Server not ready:", err)
//	}
//	// Now safe to make API calls
//	tools, err := client.ListTools()
func (c *clientImpl) WaitForReady(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for client to be ready")
		case <-ticker.C:
			if c.IsConnected() && c.IsInitialized() {
				// Try a ping to verify the connection is working
				if err := c.Ping(); err == nil {
					return nil
				}
			}
		}
	}
}

// GetServerStatus returns the status of a managed MCP server.
func (c *clientImpl) GetServerStatus(serverName string) *ServerStatus {
	c.mu.RLock()
	registry := c.serverRegistry
	c.mu.RUnlock()

	if registry == nil {
		return &ServerStatus{
			Running: false,
			Error:   fmt.Errorf("client not created with server registry"),
		}
	}

	// Check if the server exists in the registry
	client, err := registry.GetClient(serverName)
	if err != nil {
		return &ServerStatus{
			Running: false,
			Error:   err,
		}
	}

	// Check if the client is connected
	if !client.IsConnected() {
		return &ServerStatus{
			Running: false,
			Error:   fmt.Errorf("server is not connected"),
		}
	}

	// Try to ping the server to verify it's responsive
	if err := client.Ping(); err != nil {
		return &ServerStatus{
			Running: false,
			Error:   fmt.Errorf("server ping failed: %w", err),
		}
	}

	return &ServerStatus{
		Running: true,
		Error:   nil,
	}
}

// RestartServer attempts to restart a managed MCP server.
func (c *clientImpl) RestartServer(serverName string) error {
	c.mu.RLock()
	registry := c.serverRegistry
	c.mu.RUnlock()

	if registry == nil {
		return fmt.Errorf("client not created with server registry")
	}

	// Stop the server
	if err := registry.StopServer(serverName); err != nil {
		return fmt.Errorf("failed to stop server %s: %w", serverName, err)
	}

	// Note: To restart the server, we would need the original ServerDefinition
	// This would require storing the configuration in the registry or client
	// For now, return an error indicating this limitation
	return fmt.Errorf("server restart not yet implemented - please recreate the client")
}
