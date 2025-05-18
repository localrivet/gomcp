# Standard I/O Transport Example

The Standard I/O (stdio) transport uses the standard input and output streams for communication. This transport is ideal for command-line tools, language server protocols, and child processes that communicate with parent processes.

## Overview

In this example, we demonstrate:

- Setting up an MCP server using stdio transport
- Creating an example client that interacts with the server
- Registering and calling a simple echo tool
- Properly handling logging in stdio transport

## Server Configuration

```go
// Create a new server with a name
srv := server.NewServer("stdio-example-server")

// Configure the server with stdio transport
// Provide a log file path to avoid logging to stdout/stderr
srv.AsStdio("logs/mcp-server.log")

// Register a simple echo tool
srv.Tool("echo", "Echo the message back", func(ctx *server.Context, args struct {
    Message string `json:"message"`
}) (map[string]interface{}, error) {
    // Log to the server's logger (goes to the log file)
    ctx.Logger().Info("Echo tool called", "message", args.Message)

    return map[string]interface{}{
        "message": args.Message,
    }, nil
})

// Start the server - this will block until the server exits
if err := srv.Run(); err != nil {
    log.Fatalf("Server error: %v", err)
}
```

## Client Implementation

For stdio transport, clients typically communicate by:

1. Starting the server as a child process
2. Writing JSON-RPC messages to the server's stdin
3. Reading JSON-RPC responses from the server's stdout

Here's how the client can be implemented:

```go
// Create a client with stdio transport by starting the server process
c, err := client.NewClient("stdio-client",
    client.WithStdioCommand("./stdio-server"),
)
if err != nil {
    log.Fatalf("Failed to create client: %v", err)
}
defer c.Close()

// Call the echo tool
echoResult, err := c.CallTool("echo", map[string]interface{}{
    "message": "Hello from stdio client!",
})
if err != nil {
    log.Fatalf("Echo call failed: %v", err)
}
fmt.Printf("Echo result: %v\n", echoResult)
```

## Important Stdio Considerations

### Logging

When using stdio transport, it's crucial to handle logging properly:

1. **Never log to stdout/stderr** in the server - this will corrupt the JSON-RPC communication
2. **Always provide a log file path** to the `AsStdio()` method
3. Use the server's logger via the context: `ctx.Logger()`

### Server Management

For production applications, use the server management functionality:

```go
config := client.ServerConfig{
    MCPServers: map[string]client.ServerDefinition{
        "stdio-server": {
            Command: "./stdio-server",
            Args: []string{"-config", "server.json"},
            Env: map[string]string{"DEBUG": "true"},
        },
    },
}

client, err := client.NewClient("stdio-client",
    client.WithServers(config, "stdio-server"),
)
```

This approach:

- Automatically starts the server process
- Handles environment variables
- Provides proper cleanup on client close

## Advantages of Stdio Transport

- **Simple Communication**: Direct process-to-process communication
- **Zero Network Overhead**: No network stack involved
- **Process Isolation**: Clear separation between client and server processes
- **Security**: No exposed network ports or access control needed
- **Standard Pattern**: Common approach for language servers, command-line tools, and other utilities

## Limitations of Stdio Transport

- **Local Only**: Limited to local processes
- **Limited Concurrency**: Single input/output stream
- **Careful Logging Required**: Must avoid stdout/stderr for server logs
- **Process Management**: Requires proper child process handling

## Running the Example

```bash
# Build the server
go build -o stdio-server examples/stdio/server/main.go

# Run the client (which starts the server as a child process)
go run examples/stdio/client/main.go
```

## Complete Example

See the full example in [examples/stdio](https://github.com/localrivet/gomcp/tree/main/examples/stdio) in the GOMCP repository.
