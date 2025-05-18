# WebSocket Transport Example

The WebSocket transport provides bidirectional, persistent communication over a single TCP connection. This transport is ideal for web applications requiring real-time updates and interactive sessions.

## Overview

In this example, we demonstrate:

- Setting up an MCP server using WebSocket transport
- Creating an MCP client that connects to the WebSocket server
- Registering and calling a simple echo tool
- Establishing a persistent connection for efficient communication

## Server Configuration

```go
// Create a new server
srv := server.NewServer("websocket-example-server")

// Configure the server with WebSocket transport
srv.AsWebsocket(address)

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
// Format the address as a URL
serverURL := fmt.Sprintf("ws://%s", address)

// Create a new client with the WebSocket server URL
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
    "message": "Hello from WebSocket client!",
})
if err != nil {
    log.Fatalf("Echo call failed: %v", err)
}
fmt.Printf("Echo result: %v\n", echoResult)
```

## Advantages of WebSocket Transport

- **Bidirectional communication**: Server can push messages to clients
- **Persistent connection**: Reduced overhead for multiple requests
- **Real-time updates**: Ideal for applications requiring low-latency updates
- **Native browser support**: Works well with web applications

## Limitations of WebSocket Transport

- **Connection management**: Requires handling connection drops and reconnects
- **Stateful**: Needs connection state management
- **Proxy challenges**: Some proxies may not handle WebSockets well

## Running the Example

```bash
# Run the example directly
go run examples/websocket/websocket_example.go
```

## Complete Example

See the full example in [examples/websocket/websocket_example.go](https://github.com/localrivet/gomcp/tree/main/examples/websocket/websocket_example.go) in the GOMCP repository.

## Path Customization

For more advanced use cases, you can customize the WebSocket endpoint path:

```go
// Configure the server with custom paths
srv.AsWebsocketWithPaths(address, "/api/v1", "/socket")
```

This configures:

- Base path prefix as "/api/v1"
- WebSocket endpoint at "/api/v1/socket"
