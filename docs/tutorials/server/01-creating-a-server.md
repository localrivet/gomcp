# Creating a Server

This tutorial covers how to create and configure a GOMCP server that can expose functionality to MCP clients.

## Basic Server Creation

Creating an MCP server is straightforward:

```go
package main

import (
    "log"

    "github.com/localrivet/gomcp/server"
)

func main() {
    // Create a server with a name
    s := server.NewServer("example-server")

    // Configure the transport (stdio in this case)
    s = s.AsStdio()

    // Start the server (this blocks until the server exits)
    if err := s.Run(); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

## Server Options

The server can be configured with various options:

```go
// Create a custom logger
logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
})
logger := slog.New(logHandler)

// Create server with options
s := server.NewServer("example-server",
    server.WithLogger(logger),
)
```

## Transport Types

GOMCP supports multiple transport types:

- **Standard I/O**: `s.AsStdio()` - For CLI tools and child processes
- **WebSocket**: `s.AsWebsocket("localhost:8080")` - WebSocket server
- **HTTP**: `s.AsHTTP("localhost:8080")` - HTTP server
- **SSE**: `s.AsSSE("localhost:8080")` - Server-Sent Events

## Server Lifecycle

The typical lifecycle of a server includes:

1. Creation via `NewServer()`
2. Transport configuration via `AsStdio()`, `AsWebsocket()`, etc.
3. Registering tools, resources, and prompts
4. Starting the server with `Run()`

## Adding Basic Functionality

Here's a simple example with a tool:

```go
package main

import (
    "log"

    "github.com/localrivet/gomcp/server"
)

func main() {
    // Create and configure the server
    s := server.NewServer("calculator").AsStdio()

    // Add a simple tool
    s.Tool("add", "Add two numbers", func(ctx *server.Context, args struct {
        X float64 `json:"x" required:"true"`
        Y float64 `json:"y" required:"true"`
    }) (float64, error) {
        return args.X + args.Y, nil
    })

    // Start the server
    if err := s.Run(); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

## Next Steps

- Learn how to [implement tools](02-implementing-tools.md)
- Explore [implementing resources](03-implementing-resources.md)
- See the [API reference](../../api-reference/README.md) for all server options
