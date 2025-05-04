# gomcp Client API Parity Plan

This document outlines the plan for enhancing the gomcp client API to achieve feature parity with FastMCP's client capabilities.

## Overview

FastMCP provides a powerful, ergonomic client API that simplifies interaction with MCP servers. The gomcp library already has a basic client implementation but needs enhancements to match FastMCP's user-friendly interface and features.

## Feature Comparison with FastMCP

| Feature                  | FastMCP Client                        | Current gomcp Client    | Goal                                           |
| ------------------------ | ------------------------------------- | ----------------------- | ---------------------------------------------- |
| Transport Abstraction    | Separate Client and Transport         | Basic transport support | Enhanced transport layer with clear separation |
| Ergonomic Interface      | Simple async methods                  | JSON-RPC focused        | Higher-level API with Go idioms                |
| Connection Management    | Async context manager                 | Manual connection       | Structured connection with contexts            |
| Multiple Transports      | HTTP/SSE, WebSocket, Stdio, In-memory | Limited set             | Full transport support                         |
| Transport Auto-detection | Auto-infer from URL/path              | Manual setup            | Smart transport selection                      |
| Error Handling           | Rich error types                      | Basic errors            | Comprehensive error system                     |
| Resource Types           | Strong typing for resources           | Basic content handling  | Enhanced type safety                           |
| Progress Reporting       | Callbacks for progress                | Basic support           | First-class progress handling                  |
| Auto Reconnect           | Optional reconnection                 | Not available           | Configurable reconnection                      |
| Middleware/Hooks         | Request/response hooks                | Limited                 | Comprehensive middleware system                |
| Authentication           | Built-in auth support                 | Manual                  | Authentication helpers                         |
| Raw Protocol Access      | `*_mcp` methods                       | Direct only             | Both high and low level APIs                   |
| Fluent Interface         | N/A (Python-style)                    | Not supported           | Method chaining for configuration              |

## Protocol Version Support

A critical requirement for the gomcp client is to support both the 2024-11-05 and 2025-03-26 versions of the Model Context Protocol. These versions have important differences that our client implementation must accommodate:

### Key Differences Between Protocol Versions

| Feature               | 2024-11-05                    | 2025-03-26                         | Impact                                               |
| --------------------- | ----------------------------- | ---------------------------------- | ---------------------------------------------------- |
| Message Format        | Basic JSON-RPC                | Added JSON-RPC Batch support       | Need to handle batch requests/responses              |
| Content Types         | Text, Image, EmbeddedResource | Added AudioContent                 | Support new content type in client API               |
| Progress Notification | Basic fields                  | Added "message" field              | Enhanced progress reporting                          |
| Annotations           | Embedded in types             | Separated into dedicated interface | More consistent annotation handling                  |
| Tool Annotations      | Not present                   | Added ToolAnnotations interface    | Support for tool hints (readOnly, destructive, etc.) |
| Content Structure     | "Annotated" base type         | "annotations" field                | Different structure for annotations                  |

### Protocol Negotiation Strategy

1. **Version Detection**: During the initialization handshake, the client will:

   - Offer the latest protocol version (2025-03-26) by default
   - Accept the server's preferred version in the response
   - Configure internal handlers based on the negotiated version

2. **Dual-Mode Implementation**:

   - Use protocol-specific interfaces for each version
   - Implement adapters to convert between versions when needed
   - Provide type safety for both protocol versions

3. **Client API Abstraction**:
   - High-level API remains consistent regardless of protocol version
   - Internal implementation handles version-specific differences
   - Return rich types that include all fields from both versions

### Implementation Approach

```go
// Protocol versioning types and constants
const (
    ProtocolVersion2024 = "2024-11-05"
    ProtocolVersion2025 = "2025-03-26"
    LatestProtocolVersion = ProtocolVersion2025
)

// ProtocolHandler interface abstracts version-specific behavior
type ProtocolHandler interface {
    // Common operations with version-specific implementations
    FormatRequest(method string, params interface{}) (*protocol.JSONRPCRequest, error)
    ParseResponse(resp *protocol.JSONRPCResponse) (interface{}, error)
    FormatCallToolRequest(name string, args map[string]interface{}) (interface{}, error)
    ParseCallToolResult(result json.RawMessage) ([]protocol.Content, error)
    // Other methods for version-specific handling
}

// Client option for protocol version preference
func WithPreferredProtocolVersion(version string) ClientOption {
    return func(config *ClientConfig) {
        config.PreferredProtocolVersion = version
    }
}

// Protocol handler factory
func newProtocolHandler(version string) ProtocolHandler {
    switch version {
    case ProtocolVersion2024:
        return &protocol2024Handler{}
    case ProtocolVersion2025:
        return &protocol2025Handler{}
    default:
        // Default to latest
        return &protocol2025Handler{}
    }
}
```

The client will automatically handle content type differences and structural changes between versions, providing a consistent API to client code while maintaining compatibility with servers implementing either protocol version.

## Technical Design

### 1. Core Types and Interfaces

```go
// Client is the main interface for interacting with MCP servers
type Client interface {
    // Connection Management
    Connect(ctx context.Context) error
    Close() error
    IsConnected() bool

    // Run starts the client's connection lifecycle and blocks until the connection
    // is closed or the context is canceled. This manages asynchronous messages,
    // notifications, connection status, and reconnection if configured.
    Run(ctx context.Context) error

    // MCP Methods - High-level API
    ListTools(ctx context.Context) ([]protocol.Tool, error)
    CallTool(ctx context.Context, name string, args map[string]interface{}, progressCh chan<- *protocol.Progress) ([]protocol.Content, error)
    ListResources(ctx context.Context) ([]protocol.Resource, error)
    ReadResource(ctx context.Context, uri string) ([]protocol.ResourceContents, error)
    ListPrompts(ctx context.Context) ([]protocol.Prompt, error)
    GetPrompt(ctx context.Context, name string, args map[string]interface{}) ([]protocol.PromptMessage, error)

    // Server Information
    ServerInfo() protocol.ServerInfo
    ServerCapabilities() protocol.ServerCapabilities

    // Raw Protocol Access
    SendRequest(ctx context.Context, method string, params interface{}) (*protocol.JSONRPCResponse, error)

    // Configuration methods (fluent interface)
    WithTimeout(timeout time.Duration) Client
    WithRetry(maxAttempts int, backoff BackoffStrategy) Client
    WithMiddleware(middleware ClientMiddleware) Client
    WithAuth(auth AuthProvider) Client
    WithLogger(logger logx.Logger) Client
}

// ClientTransport handles the actual communication with the server
type ClientTransport interface {
    Connect(ctx context.Context) error
    Close() error
    SendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error)
    SendRequestAsync(ctx context.Context, req *protocol.JSONRPCRequest, responseCh chan<- *protocol.JSONRPCResponse) error
}

// ClientOption configures a client instance
type ClientOption func(*ClientConfig)

// ClientConfig holds the configuration for a client
type ClientConfig struct {
    Name string
    Logger logx.Logger
    Capabilities protocol.ClientCapabilities
    ReconnectOptions ReconnectOptions
    Middleware []ClientMiddleware
    // ... other configuration options
}
```

### 2. Transport Layer Enhancements

For each transport type, implement a specific concrete ClientTransport:

```go
// SSETransport communicates with a server over HTTP/SSE
type SSETransport struct {
    baseURL string
    httpClient *http.Client
    // ... other fields
}

// WebSocketTransport communicates with a server over WebSockets
type WebSocketTransport struct {
    url string
    conn *websocket.Conn
    // ... other fields
}

// StdioTransport communicates with a server over standard I/O
type StdioTransport struct {
    cmd *exec.Cmd
    stdin io.Writer
    stdout io.Reader
    // ... other fields
}

// InMemoryTransport communicates with a server in the same process
type InMemoryTransport struct {
    server *server.Server
    // ... other fields
}
```

### 3. Client Factory Methods

For ease of use, provide factory methods similar to FastMCP:

```go
// NewClient creates a new client with automatic transport detection
func NewClient(nameOrURL string, options ...ClientOption) (*Client, error)

// NewSSEClient creates a client that connects via HTTP/SSE
func NewSSEClient(name, baseURL, basePath string, options ...ClientOption) (*Client, error)

// NewWebSocketClient creates a client that connects via WebSocket
func NewWebSocketClient(name, url string, options ...ClientOption) (*Client, error)

// NewStdioClient creates a client that connects via stdio
func NewStdioClient(name string, options ...ClientOption) (*Client, error)

// NewStdioClientWithCommand creates a client that launches and connects to a command
func NewStdioClientWithCommand(name string, options ClientOptions, cmd []string) (*Client, error)

// NewInMemoryClient creates a client that connects to a server in the same process
func NewInMemoryClient(name string, server *server.Server, options ...ClientOption) (*Client, error)
```

### Example of Fluent Interface Usage

```go
// Example of client creation and configuration with fluent interface
client, err := client.NewSSEClient("MyClient", "http://localhost", "/mcp").
    WithTimeout(30 * time.Second).
    WithRetry(3, ExponentialBackoff(100*time.Millisecond)).
    WithAuth(BearerAuth("my-token")).
    WithLogger(customLogger)
if err != nil {
    log.Fatalf("Failed to create client: %v", err)
}

// Method 1: Manual connection management
if err := client.Connect(ctx); err != nil {
    log.Fatalf("Failed to connect: %v", err)
}
defer client.Close()

// Use the client
tools, err := client.ListTools(ctx)
if err != nil {
    log.Printf("Error listing tools: %v", err)
}

// Method 2: Using Run() for connection lifecycle management
// This approach handles connection, reconnection, and async messages
go func() {
    if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
        log.Printf("Client run error: %v", err)
    }
}()

// Wait for connection to be established
for !client.IsConnected() {
    time.Sleep(50 * time.Millisecond)
    if ctx.Err() != nil {
        log.Fatalf("Context canceled while waiting for connection: %v", ctx.Err())
    }
}

// Now use the client with the active connection
result, err := client.CallTool(ctx, "calculate", map[string]interface{}{
    "operation": "add",
    "values": []int{1, 2, 3, 4, 5},
}, nil)

// To shutdown gracefully
cancel() // Cancel the context to trigger shutdown
```

### 4. Middleware and Hooks System

Design a middleware system for intercepting and modifying requests/responses:

```go
// ClientMiddleware intercepts client operations
type ClientMiddleware interface {
    BeforeSendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCRequest, error)
    AfterReceiveResponse(ctx context.Context, resp *protocol.JSONRPCResponse) (*protocol.JSONRPCResponse, error)
}

// Middleware factories
func WithAuthentication(authProvider AuthProvider) ClientOption
func WithLogging(logLevel string) ClientOption
func WithRetry(maxAttempts int, backoffStrategy BackoffStrategy) ClientOption
```

### 5. Improved Error Handling

Create a rich error model with specific error types:

```go
// ClientError is the base error type for client errors
type ClientError struct {
    // Common fields
    Message string
    Code    int
    Cause   error
}

// TransportError indicates a problem with the transport layer
type TransportError struct {
    ClientError
    Transport string
}

// ConnectionError indicates a connection issue
type ConnectionError struct {
    ClientError
    Endpoint string
}

// TimeoutError indicates a timeout
type TimeoutError struct {
    ClientError
    Operation string
    Timeout   time.Duration
}

// ServerError represents an error returned by the server
type ServerError struct {
    ClientError
    RawError json.RawMessage
}
```

### 6. Notification and Event Handling

The client should provide ways to register handlers for various types of notifications and events:

```go
// NotificationHandler processes incoming notifications
type NotificationHandler func(notification *protocol.JSONRPCNotification) error

// ProgressHandler processes progress updates
type ProgressHandler func(progress *protocol.Progress) error

// ResourceUpdateHandler processes resource update notifications
type ResourceUpdateHandler func(uri string) error

// LogHandler processes log messages
type LogHandler func(level protocol.LoggingLevel, message string) error

// ConnectionStatusHandler processes connection status changes
type ConnectionStatusHandler func(connected bool) error

// Client interface extensions for notification handling
type Client interface {
    // ... existing methods ...

    // Notification registration methods
    OnNotification(method string, handler NotificationHandler) Client
    OnProgress(handler ProgressHandler) Client
    OnResourceUpdate(uri string, handler ResourceUpdateHandler) Client
    OnLog(handler LogHandler) Client
    OnConnectionStatus(handler ConnectionStatusHandler) Client
}
```

Example usage:

```go
client.
    OnProgress(func(progress *protocol.Progress) error {
        log.Printf("Progress: %d/%d - %s", progress.Value, progress.MaxValue, progress.Message)
        return nil
    }).
    OnResourceUpdate("file://config.json", func(uri string) error {
        log.Printf("Resource updated: %s", uri)
        // Reload the resource
        content, err := client.ReadResource(ctx, uri)
        if err != nil {
            return err
        }
        // Process updated content
        log.Printf("Updated content: %v", content)
        return nil
    }).
    OnLog(func(level protocol.LoggingLevel, message string) error {
        log.Printf("[%s] %s", level, message)
        return nil
    }).
    OnConnectionStatus(func(connected bool) error {
        if connected {
            log.Println("Connected to server")
        } else {
            log.Println("Disconnected from server")
        }
        return nil
    })
```

## Implementation Phases

### Phase 1: Core Client Interface and Basic Transports

1. Define the core Client interface and base types
2. Implement the ClientTransport interface
3. Create concrete implementations for SSE, WebSocket, and Stdio transports
4. Enhance error handling with specific error types
5. Provide basic factory methods for client creation

### Phase 2: High-Level API and Connection Management

1. Implement high-level API methods (ListTools, CallTool, etc.)
2. Add structured connection management with proper context support
3. Improve resource type handling
4. Add progress reporting
5. Implement the middleware system

### Phase 3: Advanced Features

1. Add automatic transport detection
2. Implement advanced reconnection strategies
3. Add authentication helpers
4. Create the InMemoryTransport for testing
5. Add comprehensive logging and observability
6. Ensure raw protocol access is available alongside high-level API

### Phase 4: Testing and Examples

1. Create comprehensive unit tests for all client features
2. Develop integration tests with various server types
3. Write examples for each transport type and common use cases
4. Create a client usage guide

## Testing Strategy

1. Unit Tests:

   - Test each transport implementation
   - Test middleware and hooks
   - Test error handling
   - Test connection management

2. Integration Tests:

   - Test with real HTTP/SSE servers
   - Test with WebSocket servers
   - Test with in-process servers
   - Test reconnection strategies

3. Example Applications:
   - Create examples for each transport type
   - Demonstrate error handling and recovery
   - Show advanced use cases (middleware, auth, etc.)

## Documentation

1. Add godoc comments for all types and methods
2. Create a client usage guide
3. Document each transport type with examples
4. Provide best practices for client usage
5. Document error handling strategies

## Acceptance Criteria

1. Full feature parity with FastMCP's client API
2. Clean, idiomatic Go interface
3. Comprehensive documentation
4. Example applications demonstrating all features
5. High test coverage
6. Seamless interoperability with any MCP-compliant server
