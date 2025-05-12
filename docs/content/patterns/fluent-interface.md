---
title: 'Fluent Interface Pattern'
weight: 10
---

# Fluent Interface Pattern in GoMCP

GoMCP implements a **fluent interface pattern** (also known as method chaining) to provide a clean, readable API for configuring servers, registering tools and resources, and setting up transports.

## What is a Fluent Interface?

A fluent interface is a method of designing object-oriented APIs where multiple method calls can be chained together in a single statement. Each method returns the object itself (or another appropriate object), allowing further method calls on the same statement.

In GoMCP, this pattern is used extensively to make the server configuration more concise and expressive.

## Benefits

- **Improved readability**: Configuration code becomes more declarative and easier to understand
- **Reduced boilerplate**: Less repetition of object references
- **Contextual methods**: Methods are only available in appropriate contexts
- **Natural API discovery**: Code editors can suggest the next appropriate methods in the chain

## Server Configuration Example

Here's an example of the fluent interface pattern in GoMCP for configuring a server:

```go
// Traditional approach (pre-fluent interface)
srv := server.NewServer("my-server")
server.AddTool(srv, "echo", "Echo a message", handleEchoTool)
err := srv.AddResource(protocol.ResourceDefinition{...})
if err != nil {
    // Handle error
}
// Configure and start the server
err = server.ServeWebsocket(srv, ":9090", "/mcp")
if err != nil {
    // Handle error
}

// Fluent interface approach
srv := server.NewServer("my-server").
    Tool("echo", "Echo a message", handleEchoTool).
    Resource("app://info/version", server.WithTextContent("1.0.0")).
    AsWebsocket(":9090", "/mcp").
    Run()
```

## Key Pattern Components

### 1. Server Creation and Configuration

The fluent interface starts with creating a new server:

```go
srv := server.NewServer("my-server")
```

### 2. Tool Registration

Register tools by chaining the `Tool` method:

```go
srv.Tool("tool-name", "Tool description",
    func(ctx *server.Context, args SomeArgsStruct) (interface{}, error) {
        // Tool implementation
        return result, nil
    })
```

### 3. Resource Registration

Register resources with the `Resource` method and functional options:

```go
srv.Resource("resource://uri",
    server.WithTextContent("Some content"),
    server.WithDescription("Resource description"))
```

### 4. Transport Configuration

Configure the transport method:

```go
// WebSocket
srv.AsWebsocket(":9090", "/mcp")

// SSE
srv.AsSSE(":9090", "/mcp")

// Stdio
srv.AsStdio()

// TCP
srv.AsTCP(":9090")
```

### 5. Server Start

Finally, start the server:

```go
srv.Run()
```

## Complete Example

Here's a complete example showing the fluent interface pattern in action:

```go
package main

import (
    "github.com/localrivet/gomcp/server"
)

func main() {
    // Create and configure the server with fluent interface
    server.NewServer("demo-server").
        // Register tools
        Tool("greet", "Greet a user",
            func(ctx *server.Context, args struct {
                Name string `json:"name" description:"User name"`
            }) (string, error) {
                return "Hello, " + args.Name + "!", nil
            }).
        Tool("sum", "Sum two numbers",
            func(ctx *server.Context, args struct {
                A int `json:"a" description:"First number"`
                B int `json:"b" description:"Second number"`
            }) (int, error) {
                return args.A + args.B, nil
            }).
        // Register resources
        Resource("app://info/version",
            server.WithTextContent("1.0.0"),
            server.WithDescription("Application version")).
        Resource("app://status",
            server.WithJSONContent(map[string]interface{}{
                "status": "healthy",
                "uptime": "12h",
            })).
        // Configure and start with WebSocket transport
        AsWebsocket(":9090", "/mcp").
        Run()
}
```

## Error Handling

While the fluent interface pattern provides a cleaner API, it does require careful error handling. In GoMCP:

1. Most configuration errors are logged but don't break the chain
2. Fatal errors during server startup are returned from the `Run()` method
3. For more granular error handling, you can use the traditional non-fluent API alongside the fluent one

## When to Use

The fluent interface pattern is ideal for:

- Initial server setup and configuration
- Registering multiple tools and resources
- Creating simple, concise examples

For more complex scenarios where detailed error handling is required, you might mix the fluent interface with traditional approaches.

## Migration from Pre-Fluent API

If you're upgrading from an older version of GoMCP, here's a guide to migrate from the traditional API to the fluent interface:

| Traditional API                            | Fluent Interface                    |
| ------------------------------------------ | ----------------------------------- |
| `server.AddTool(srv, name, desc, handler)` | `srv.Tool(name, desc, handler)`     |
| `srv.AddResource(resourceDef)`             | `srv.Resource(uri, options...)`     |
| `server.ServeWebsocket(srv, addr, path)`   | `srv.AsWebsocket(addr, path).Run()` |
| `server.ServeSSE(srv, addr, path)`         | `srv.AsSSE(addr, path).Run()`       |
| `server.ServeStdio(srv)`                   | `srv.AsStdio().Run()`               |
| `server.ServeTCP(srv, addr)`               | `srv.AsTCP(addr).Run()`             |
