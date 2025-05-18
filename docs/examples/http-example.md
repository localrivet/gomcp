# HTTP Transport Example

The HTTP transport provides a standard request-response communication pattern using JSON-RPC over HTTP. This transport is ideal for simple integrations and REST-like services.

## Overview

In this example, we demonstrate:

- Setting up an MCP server using HTTP transport
- Creating an MCP client that connects to the HTTP server
- Registering and calling a simple echo tool

## Server Configuration

```go
// Create a new server
srv := server.NewServer("http-example-server")

// Configure the server with HTTP transport
srv.AsHTTP(address)

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
serverURL := fmt.Sprintf("http://%s", address)

// Create a new client
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
    "message": "Hello from HTTP client!",
})
if err != nil {
    log.Fatalf("Echo call failed: %v", err)
}
fmt.Printf("Echo result: %v\n", echoResult)
```

## Advantages of HTTP Transport

- **Simple**: Familiar request-response pattern
- **Widely supported**: Works across firewalls and proxies
- **Stateless**: No persistent connection to manage
- **Scalable**: Easy to load balance across servers

## Limitations of HTTP Transport

- **No bidirectional communication**: Server cannot push messages to clients
- **No built-in streaming**: Not suited for streaming responses
- **Higher latency**: Each request incurs connection overhead

## Running the Example

```bash
# Run the example directly
go run examples/http/http_example.go
```

## Complete Example

See the full example in [examples/http/http_example.go](https://github.com/localrivet/gomcp/tree/main/examples/http/http_example.go) in the GOMCP repository.
