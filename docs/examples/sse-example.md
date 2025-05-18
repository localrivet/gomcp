# SSE (Server-Sent Events) Transport Example

The Server-Sent Events (SSE) transport provides a lightweight, server-to-client streaming mechanism over HTTP. This transport is ideal for applications requiring real-time updates or monitoring from the server to clients, particularly in web applications.

## Overview

In this example, we demonstrate:

- Setting up an MCP server using SSE transport
- Creating an MCP client that connects to the SSE server
- Registering and calling a simple echo tool
- Handling the unidirectional streaming nature of SSE

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
// Create a new client with SSE transport
// For SSE, we format the address as an HTTP URL
serverURL := fmt.Sprintf("http://%s", address)

c, err := client.NewClient(serverURL,
    client.WithConnectionTimeout(5*time.Second),
    client.WithRequestTimeout(30*time.Second),
)
if err != nil {
    log.Fatalf("Failed to create client: %v", err)
}
defer c.Close()

// Call the echo tool
echoResult, err := c.CallTool("echo", map[string]interface{}{
    "message": "Hello from SSE client!",
})
if err != nil {
    log.Fatalf("Echo call failed: %v", err)
}
fmt.Printf("Echo result: %v\n", echoResult)
```

## How SSE Works

Server-Sent Events is a standard for one-way communication from the server to the client:

1. The client makes an initial HTTP request to the server
2. The server keeps the connection open and sends events as formatted text messages
3. The client processes these events as they arrive
4. If the connection drops, the client automatically attempts to reconnect

Unlike WebSockets, SSE only allows the server to send data to the client, not vice versa. For client-to-server communication with SSE, regular HTTP requests are used.

## Advantages of SSE Transport

- **Simple Protocol**: Text-based, easy to debug and implement
- **Built-in Reconnection**: Clients automatically reconnect if the connection is lost
- **Browser Native**: Supported in all modern browsers via the EventSource API
- **Works with HTTP Infrastructure**: Compatible with proxies, load balancers, and firewalls
- **Efficient**: Lower overhead than WebSockets for one-way communication

## Limitations of SSE Transport

- **Unidirectional**: Server-to-client only; requires separate HTTP requests for client-to-server messages
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
