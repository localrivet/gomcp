# SSE (Server-Sent Events) Transport Example

The Server-Sent Events (SSE) transport provides a lightweight, server-to-client streaming mechanism over HTTP. This transport is ideal for applications requiring real-time updates or monitoring from the server to clients, particularly in web applications.

## Overview

In this example, we demonstrate:

- Setting up an MCP server using SSE transport
- Creating an MCP client that connects to the SSE server
- Registering and calling a simple echo tool
- Handling the bidirectional communication pattern with SSE

## Server Configuration

```go
// Create a new server
srv := server.NewServer("sse-example-server")

// Configure the server with SSE transport
srv.AsSSE("localhost:8083")

// Register a simple echo tool
srv.Tool("echo", "Echo the message back", func(ctx *server.Context, args struct {
    Message string `json:"message"`
}) (map[string]interface{}, error) {
    fmt.Printf("Server received: %s\n", args.Message)
    return map[string]interface{}{
        "message": args.Message,
    }, nil
})

// Start the server
if err := srv.Run(); err != nil {
    log.Fatalf("Server error: %v", err)
}
```

## Client Configuration

```go
// Use explicit http:// scheme for the SSE server
// Do NOT include the /sse path - the transport will handle that
serverURL := fmt.Sprintf("http://%s", address)

// Create a new client with the SSE server URL
// For SSE connections, the oldest protocol version is automatically used
// for maximum compatibility, unless explicitly overridden
c, err := client.NewClient("",
    client.WithSSE(serverURL),
    client.WithConnectionTimeout(5*time.Second),
    client.WithRequestTimeout(30*time.Second),
)
if err != nil {
    log.Fatalf("Failed to create client: %v", err)
}
defer c.Close()

// Call the echo tool - connection happens automatically
echoResult, err := c.CallTool("echo", map[string]interface{}{
    "message": "Hello from SSE client!",
})
if err != nil {
    log.Fatalf("Echo call failed: %v", err)
}
fmt.Printf("Echo result: %v\n", echoResult)
```

## How SSE Works in GOMCP

The SSE implementation in GOMCP uses a hybrid approach for bidirectional communication:

1. The client establishes an SSE connection to the server's events endpoint (typically `/sse`)
2. The server sends back a message endpoint URL over the SSE connection
3. The client uses this endpoint URL for sending messages via HTTP POST requests
4. The server maintains the SSE connection for sending notifications to the client
5. Client requests receive direct HTTP responses, while server-initiated messages come through the SSE stream

This approach provides several advantages:
- The server can push notifications to the client in real-time
- Regular client-to-server communication uses standard HTTP requests
- The pattern works well with existing web infrastructure and proxies
- Automatic reconnection is built into the client implementation

## Advantages of SSE Transport

- **Simple Protocol**: Text-based, easy to debug and implement
- **Built-in Reconnection**: Clients automatically reconnect if the connection is lost
- **Browser Native**: Supported in all modern browsers via the EventSource API
- **Works with HTTP Infrastructure**: Compatible with proxies, load balancers, and firewalls
- **Efficient**: Lower overhead than WebSockets for one-way communication

## Limitations of SSE Transport

- **Initial Setup Complexity**: Requires coordination between SSE stream and HTTP endpoints
- **Connection Limits**: Browsers typically limit concurrent connections to the same domain
- **Header Limitations**: Cannot set custom headers for reconnection requests in browsers
- **No Binary Support**: Text-based protocol not optimized for binary data transfer

## Running the Example

```bash
# Run the example directly
go run examples/sse/sse_example.go
```

## Complete Example

See the full example in [examples/sse/sse_example.go](https://github.com/localrivet/gomcp/tree/main/examples/sse/sse_example.go) in the GOMCP repository.
