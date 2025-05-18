# Unix Socket Transport Example

The Unix Socket transport provides high-performance interprocess communication (IPC) for processes running on the same machine. This transport is ideal for microservices architecture or any scenario requiring fast, efficient communication between local processes.

## Overview

In this example, we demonstrate:

- Setting up an MCP server using Unix Socket transport
- Creating an MCP client that connects to the Unix Socket server
- Registering and calling a simple echo tool
- Configuring Unix Socket-specific options like permissions

## Server Configuration

```go
// Create a new server
srv := server.NewServer("unix-example-server")

// Configure the server with Unix Socket transport
// Options allow customizing socket permissions and buffer sizes
srv.AsUnixSocket("/tmp/mcp-example.sock",
    unix.WithPermissions(0600),
    unix.WithBufferSize(4096),
)

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
// Create a new client with the Unix Socket path
c, err := client.NewClient("unix-client",
    client.WithUnixSocket("/tmp/mcp-example.sock"),
    client.WithConnectionTimeout(5*time.Second),
    client.WithRequestTimeout(30*time.Second),
)
if err != nil {
    log.Fatalf("Failed to create client: %v", err)
}
defer c.Close()

// Call the echo tool
echoResult, err := c.CallTool("echo", map[string]interface{}{
    "message": "Hello from Unix Socket client!",
})
if err != nil {
    log.Fatalf("Echo call failed: %v", err)
}
fmt.Printf("Echo result: %v\n", echoResult)
```

## Advantages of Unix Socket Transport

- **High Performance**: Lower overhead compared to TCP sockets for local communication
- **Secure**: Communication isolated to the local machine
- **Reliable**: Full-duplex, reliable transmission without network-related issues
- **Permissions**: File system permissions can restrict access to the socket
- **Simple**: No need for port management or firewall configuration

## Limitations of Unix Socket Transport

- **Local Only**: Limited to processes on the same machine
- **File Descriptor**: Requires careful management of the socket file
- **Permission Complexity**: May require additional setup for cross-user communication

## Running the Example

```bash
# First terminal: Run the server
go run examples/unix/unix_example.go server

# Second terminal: Run the client
go run examples/unix/unix_example.go client
```

## Complete Example

See the full example in [examples/unix/unix_example.go](https://github.com/localrivet/gomcp/tree/main/examples/unix/unix_example.go) in the GOMCP repository.
