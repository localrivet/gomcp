# GoMCP Server

The GoMCP server is a Go implementation of the Multimodal Chat Protocol (MCP), which enables communication between clients and AI systems.

## Features

- Supports multiple transport mechanisms:
  - Standard I/O (default)
  - WebSockets
  - Server-Sent Events (SSE)
  - HTTP
- Implements the MCP protocol specification:
  - Draft version
  - v2024-11-05
  - v2025-03-26
- Extensible design with the ability to register tools and resources
- Supports JSON Schema for tool and resource arguments
- Provides helper methods for executing tools and accessing resources

## Quick Start

```go
package main

import (
	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create a new server
	srv := server.NewServer("my-server")

	// Register a tool
	srv.Tool("hello", "Says hello to someone", func(ctx *server.Context, args map[string]interface{}) (interface{}, error) {
		name, _ := args["name"].(string)
		if name == "" {
			name = "world"
		}
		return "Hello, " + name + "!", nil
	})

	// Register a resource
	srv.Resource("/greeting/{name}", "A greeting resource", func(ctx *server.Context, args map[string]interface{}) (interface{}, error) {
		name, _ := args["name"].(string)
		if name == "" {
			name = "world"
		}
		return "Hello, " + name + "!", nil
	})

	// Start the server using standard I/O (default transport)
	if err := srv.AsStdio().Serve(); err != nil {
		panic(err)
	}
}
```

## Server Initialization

To create a new MCP server:

```go
srv := server.NewServer("my-server")
```

## Transport Options

The server supports multiple transport mechanisms. You can choose a transport using the following methods:

```go
// Use Standard I/O (default)
srv.AsStdio().Serve()

// Use WebSockets
srv.AsWebsocket("localhost", 8080).Serve()

// Use Server-Sent Events (SSE)
srv.AsSSE("localhost", 8080).Serve()

// Use HTTP
srv.AsHTTP("localhost", 8080).Serve()
```

## Tool Registration

Tools are functions that can be executed by clients. To register a tool:

```go
srv.Tool("tool-name", "Tool description", handler)
```

There are several ways to define handlers:

### 1. Map Handler

```go
srv.Tool("echo", "Echoes input", func(ctx *server.Context, args map[string]interface{}) (interface{}, error) {
    message := args["message"].(string)
    return message, nil
})
```

### 2. Struct Handler

```go
type EchoArgs struct {
    Message string `json:"message"`
}

srv.Tool("echo", "Echoes input", func(ctx *server.Context, args *EchoArgs) (interface{}, error) {
    return args.Message, nil
})
```

### 3. Composite Tools

You can create tools that call other tools internally:

```go
srv.Tool("composite", "Calls multiple tools", func(ctx *server.Context, args map[string]interface{}) (interface{}, error) {
    // Call the "echo" tool
    result, err := ctx.ExecuteTool("echo", map[string]interface{}{
        "message": "Hello from composite tool",
    })
    if err != nil {
        return nil, err
    }
    return result, nil
})
```

### Tool Helper Methods

The `Context` type provides several helper methods for working with tools:

- `ExecuteTool`: Executes a tool from within another tool handler
- `GetRegisteredTools`: Returns a list of all registered tools
- `GetToolDetails`: Returns detailed information about a specific tool

## Resource Registration

Resources are data or services that can be accessed by clients using URIs. To register a resource:

```go
srv.Resource("/path/to/resource", "Resource description", handler)
```

### Resource Paths

Resources use URI templates to define paths, which can include path parameters:

```go
// Simple resource with no parameters
srv.Resource("/hello", "Greeting resource", func(ctx *server.Context, args map[string]interface{}) (interface{}, error) {
    return "Hello, world!", nil
})

// Resource with path parameters
srv.Resource("/users/{id}", "Get user by ID", func(ctx *server.Context, args map[string]interface{}) (interface{}, error) {
    id := args["id"].(string)
    return "User: " + id, nil
})

// Resource with multiple path parameters
srv.Resource("/users/{userId}/posts/{postId}", "Get post by user and post ID", func(ctx *server.Context, args map[string]interface{}) (interface{}, error) {
    userId := args["userId"].(string)
    postId := args["postId"].(string)
    return fmt.Sprintf("User: %s, Post: %s", userId, postId), nil
})

// Resource with wildcard path parameter (matches multiple path segments)
srv.Resource("/docs/{path*}", "Access documentation", func(ctx *server.Context, args map[string]interface{}) (interface{}, error) {
    path := args["path"].(string)
    return "Documentation for: " + path, nil
})
```

### Resource Handler Types

Similar to tools, there are several ways to define resource handlers:

#### 1. Map Handler

```go
srv.Resource("/hello", "Greeting resource", func(ctx *server.Context, args map[string]interface{}) (interface{}, error) {
    name, _ := args["name"].(string)
    if name == "" {
        name = "world"
    }
    return "Hello, " + name + "!", nil
})
```

#### 2. Struct Handler

```go
type UserParams struct {
    ID string `json:"id"`
}

srv.Resource("/users/{id}", "Get user by ID", func(ctx *server.Context, args *UserParams) (interface{}, error) {
    return "User: " + args.ID, nil
})
```

### Composite Resources

Resources can call other resources internally:

```go
srv.Resource("/combined", "Combined resource", func(ctx *server.Context, args map[string]interface{}) (interface{}, error) {
    // Call another resource
    result, err := ctx.ExecuteResource("/hello")
    if err != nil {
        return nil, err
    }
    return "Combined: " + result.(string), nil
})
```

### Resource Helper Methods

The `Context` type provides several helper methods for working with resources:

- `ExecuteResource`: Executes a resource from within another resource handler
- `GetRegisteredResources`: Returns a list of all registered resources
- `GetResourceDetails`: Returns detailed information about a specific resource

## Content Response Formatting

Both tool and resource responses are automatically formatted based on the MCP specification version being used:

- For string responses, they are wrapped in a content object with type "text"
- For map/struct responses with a "content" field, they are used as-is
- For other types, they are converted to JSON and then wrapped as text content

For example:

```go
// String response will be formatted as text content
return "Hello, world!", nil

// Structured response will be formatted as is
return map[string]interface{}{
    "content": []map[string]interface{}{
        {
            "type": "text",
            "text": "Hello, world!",
        },
    },
}, nil

// Image response will be formatted appropriately
return map[string]interface{}{
    "imageUrl": "https://example.com/image.jpg",
    "altText": "Example image",
}, nil
```

## Error Handling

Both tool and resource handlers can return errors, which are automatically formatted as JSON-RPC error responses:

```go
return nil, fmt.Errorf("something went wrong")
```

This will generate a response like:

```json
{
  "jsonrpc": "2.0",
  "id": "request-id",
  "error": {
    "code": -32000,
    "message": "something went wrong"
  }
}
```

## JSON Schema Support

The server automatically extracts JSON Schema from struct types used in tool and resource handlers:

```go
type UserArgs struct {
    Name     string `json:"name" required:"true"`
    Age      int    `json:"age" minimum:"0"`
    Email    string `json:"email" format:"email"`
    IsActive bool   `json:"isActive"`
}

srv.Tool("create-user", "Creates a new user", func(ctx *server.Context, args *UserArgs) (interface{}, error) {
    // Implementation
})
```

The schema is extracted using struct tags and reflection, and is used to validate incoming requests and generate documentation.

## Customizing Server Behavior

You can customize the server's behavior using various options:

```go
srv := server.NewServer("my-server").
    WithLogger(customLogger).  // Set a custom logger
    WithMiddleware(middleware) // Add middleware for request/response processing
```
