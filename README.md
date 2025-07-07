# GoMCP - Go Model Context Protocol Library

[![Go Reference](https://pkg.go.dev/badge/github.com/localrivet/gomcp.svg)](https://pkg.go.dev/github.com/localrivet/gomcp)
[![Go Report Card](https://goreportcard.com/badge/github.com/localrivet/gomcp)](https://goreportcard.com/report/github.com/localrivet/gomcp)

## MCP Specification Compliance

![Draft Spec: 100%](https://img.shields.io/badge/Draft_Spec-100%25-brightgreen)
![2024-11-05 Spec: 100%](https://img.shields.io/badge/2024--11--05_Spec-100%25-brightgreen)
![2025-03-26 Spec: 100%](https://img.shields.io/badge/2025--03--26_Spec-100%25-brightgreen)

**✅ Full compliance across all MCP specification versions** - See [COMPLIANCE.md](COMPLIANCE.md) for detailed verification.

GoMCP is a complete Go implementation of the Model Context Protocol (MCP), designed to facilitate seamless interaction between applications and Large Language Models (LLMs). The library supports all specification versions with automatic negotiation and provides a clean, idiomatic API for both clients and servers.

## Table of Contents

- [Overview](#overview)
- [Key Features](#key-features)
- [API Stability](#api-stability)
- [Installation](#installation)
- [Quickstart](#quickstart)
  - [Client Example](#client-example)
  - [Client with Automatic Server Management](#client-with-automatic-server-management)
  - [Server Example](#server-example)
  - [Advanced Server Example](#advanced-server-example)
- [Core Concepts](#core-concepts)
  - [Clients and Servers](#clients-and-servers)
  - [Tools](#tools)
  - [Resources](#resources)
  - [Prompts](#prompts)
  - [Batch Operations](#batch-operations)
  - [Event System](#event-system)
  - [Transports](#transports)
  - [Server Management](#server-management)
  - [Session Management](#session-management)
- [Examples](#examples)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Overview

The Model Context Protocol (MCP) standardizes communication between applications and LLMs, enabling:

- **Tool Calling**: Execute actions and functions through LLMs
- **Resource Access**: Provide structured data to LLMs with workspace context
- **Prompt Rendering**: Create reusable templates for LLM interactions
- **Sampling**: Generate text from LLMs with control over parameters
- **Session Management**: Rich context and workspace root access for enhanced tool capabilities

GoMCP provides an idiomatic Go implementation that handles all the protocol details while offering a clean, developer-friendly API.

## Key Features

- **Complete Protocol Implementation**: Full support for all MCP specification versions
- **Automatic Version Negotiation**: Seamless compatibility between clients and servers
- **Multiple Transport Options**: Support for stdio, HTTP, WebSocket, and Server-Sent Events
- **Type-Safe API**: Leverages Go's type system for safety and expressiveness
- **Server Process Management**: Automatically start, manage, and stop external MCP servers
- **Server Configuration**: Load server definitions from configuration files
- **MCP Session Architecture**: Comprehensive session management with transport-aware data extraction
- **Automated Root Fetching**: Automatic workspace root discovery following MCP protocol
- **Flexible Architecture**: Modular design for easy extension and customization

## API Stability

**GoMCP v1.5.0 represents a stable, production-ready release with locked APIs.** The library has reached full maturity with a comprehensive feature set and battle-tested implementations.

### 🔒 **API Lock Guarantee (v1.5.0+)**
- **Client API**: All client methods (`CallTool`, `GetResource`, `GetPrompt`, etc.) are **locked and stable**
- **Server API**: Server registration methods (`Tool`, `Resource`, `Prompt`) and handler patterns are **finalized**
- **Transport Layer**: All transport implementations follow **stable, locked interfaces**
- **Event System**: Event types and subscription patterns are **standardized and locked**
- **Server Management**: Process lifecycle and configuration management APIs are **stable**

### ✅ **Full Protocol Compliance**
- **Complete MCP Specification Support**: Full implementation of all protocol versions (2024-11-05, 2025-03-26, draft)
- **Automatic Version Negotiation**: Seamless compatibility handling between different specification versions
- **Transport Compliance**: All transport layers properly implement their respective MCP specifications
- **Type Safety**: Strong typing throughout ensures API contracts are maintained

### ✅ **Production Ready**
- **Comprehensive Testing**: Extensive test coverage across all major components
- **Error Handling**: Robust error handling with proper MCP error codes and messages  
- **Performance**: Optimized for production workloads with efficient resource management
- **Complete Documentation**: Full API documentation and usage examples

### 🚀 **Future Development**
With v1.5.0's API lock, future releases will focus on:
- **Additive Features**: New functionality that extends but doesn't break existing APIs
- **Performance Optimizations**: Internal improvements that maintain API compatibility
- **Enhanced Documentation**: Expanded examples and integration guides
- **New Transport Options**: Additional transport implementations using the stable transport interface

**Commitment**: The v1.5.0 APIs are locked in and will not change. Any future enhancements will be additive and maintain full backward compatibility. GoMCP is ready for enterprise production deployments.

## Installation

```bash
go get github.com/localrivet/gomcp
```

## Quickstart

### Client Example

```go
package main

import (
	"log"
	"github.com/localrivet/gomcp/client"
)

func main() {
	// Create a new client with stdio transport
	c, err := client.NewClient("stdio:///",
		client.WithStdio(),
		client.WithProtocolVersion("2025-03-26"),
		client.WithProtocolNegotiation(true),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Call a tool on the MCP server
	result, err := c.CallTool("say_hello", map[string]interface{}{
		"name": "World",
	})
	if err != nil {
		log.Fatalf("Tool call failed: %v", err)
	}

	log.Printf("Result: %v", result)
}
```

**Note**: This example uses stdio transport, which means the client expects to communicate with an MCP server via stdin/stdout. For a complete working example that automatically manages the server process, see the "Client with Automatic Server Management" section below.

### Client with Automatic Server Management

**What this does:** This example demonstrates GoMCP's powerful automatic server management feature. Instead of manually starting and stopping MCP server processes, the client can automatically:

- **Launch server processes** on demand using system commands
- **Establish connections** to those servers via stdio/pipes  
- **Environment variable injection** for configuration (API keys, etc.)
- **Automatic cleanup** - server processes are terminated when the client closes
- **Process lifecycle management** - handles server startup, health checks, and shutdown

**Why this matters:** This pattern eliminates the operational complexity of managing MCP servers. You can distribute a single binary that automatically spins up the required MCP servers, making deployment and integration much simpler. It's especially useful for:

- **Development environments** - automatically start dependent services
- **CI/CD pipelines** - spin up servers for testing without manual setup
- **Desktop applications** - embed MCP servers without requiring separate installation
- **Microservice architectures** - manage server dependencies declaratively

```go
package main

import (
	"log"
	"github.com/localrivet/gomcp/client"
)

func main() {
	// Define server configuration
	config := client.ServerConfig{
		MCPServers: map[string]client.ServerDefinition{
			"govibe": {
				Command: "govibe",
				Args: []string{},
				Env: map[string]string{
					"ANTHROPIC_API_KEY": "${ANTHROPIC_API_KEY}",
				},
			},
		},
	}

	// Create a client with automatic server management
	c, err := client.NewClient("my-client",
		client.WithServers(config, "govibe"),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close() // Automatically stops the server process

	// Call a tool on the managed server
	result, err := c.CallTool("add_task", map[string]interface{}{
		"prompt": "Create a login page with authentication",
	})
	if err != nil {
		log.Fatalf("Tool call failed: %v", err)
	}

	log.Printf("Task created: %v", result)

	// Add project roots for the server to access
	err = c.AddRoot("/path/to/project", "project-root")
	if err != nil {
		log.Fatalf("Failed to add root: %v", err)
	}

	// Get a resource that might use the project context
	resource, err := c.GetResource("/project/files/src/main.go")
	if err != nil {
		log.Fatalf("Resource request failed: %v", err)
	}

	log.Printf("Resource content: %v", resource)
}
```

**Key Implementation Details:**
- The `${ANTHROPIC_API_KEY}` syntax automatically injects environment variables from the current process
- Server processes communicate via **stdio pipes** for secure, high-performance IPC
- The client waits for server initialization before accepting requests
- **Graceful shutdown** ensures servers are properly terminated, preventing orphaned processes
- Multiple servers can be managed simultaneously with different configurations

### Server Example

```go
package main

import (
	"fmt"
	"log/slog"
	"os"
	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create a logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create a new server
	srv := server.NewServer("example-server",
		server.WithLogger(logger),
	).AsStdio()

	// Register a tool with inline struct
	srv.Tool("say_hello", "Greet someone", func(ctx *server.Context, args struct {
		Name string `json:"name"`
	}) (interface{}, error) {
		return map[string]interface{}{
			"message": fmt.Sprintf("Hello, %s!", args.Name),
		}, nil
	})

	// Start the server
	if err := srv.Run(); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
```

### Advanced Server Example

```go
package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/sse"
)

func main() {
	// Create a logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create a new server with comprehensive functionality
	srv := server.NewServer("advanced-server",
		server.WithLogger(logger),
	).AsStdio()

	// Register multiple tools with different parameter types
	srv.Tool("calculator", "Perform mathematical calculations", func(ctx *server.Context, args struct {
		Operation string  `json:"operation"`
		A         float64 `json:"a"`
		B         float64 `json:"b"`
	}) (interface{}, error) {
		switch args.Operation {
		case "add":
			return map[string]interface{}{"result": args.A + args.B}, nil
		case "multiply":
			return map[string]interface{}{"result": args.A * args.B}, nil
		case "divide":
			if args.B == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			return map[string]interface{}{"result": args.A / args.B}, nil
		default:
			return nil, fmt.Errorf("unsupported operation: %s", args.Operation)
		}
	})

	srv.Tool("create_file", "Create a file with content", func(ctx *server.Context, args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}) (interface{}, error) {
		// In a real implementation, you'd validate paths and permissions
		return map[string]interface{}{
			"message": fmt.Sprintf("File created at %s with %d bytes", args.Path, len(args.Content)),
			"path":    args.Path,
			"size":    len(args.Content),
		}, nil
	})

	// Register resources with different patterns
	srv.Resource("/config", "Get server configuration", func(ctx *server.Context, args *struct{}) (interface{}, error) {
		return map[string]interface{}{
			"version":     "1.0.0",
			"environment": "development",
			"features":    []string{"tools", "resources", "prompts"},
		}, nil
	})

	// Templated resource for file access
	srv.Resource("/files/{path*}", "Access file system resources", func(ctx *server.Context, args *struct {
		Path string `path:"path"`
	}) (interface{}, error) {
		// Extract file extension for content type detection
		ext := strings.ToLower(filepath.Ext(args.Path))
		
		return map[string]interface{}{
			"path":      args.Path,
			"extension": ext,
			"type":      getFileType(ext),
			"content":   fmt.Sprintf("Mock content for file: %s", args.Path),
		}, nil
	})

	// User profile resource with parameters
	srv.Resource("/users/{id}", "Get user profile information", func(ctx *server.Context, args *struct {
		ID       string `path:"id"`
		IncludePosts bool `json:"include_posts"`
	}) (interface{}, error) {
		user := map[string]interface{}{
			"id":       args.ID,
			"name":     fmt.Sprintf("User %s", args.ID),
			"email":    fmt.Sprintf("user%s@example.com", args.ID),
			"active":   true,
		}
		
		if args.IncludePosts {
			user["posts"] = []map[string]interface{}{
				{"id": 1, "title": "Hello World", "content": "First post"},
				{"id": 2, "title": "Second Post", "content": "Another post"},
			}
		}
		
		return user, nil
	})

	// Register prompts for different use cases
	srv.Prompt("code_review", "Provide code review assistance",
		server.User("Please review this {{language}} code for best practices, potential bugs, and improvements:\n\n```{{language}}\n{{code}}\n```"),
		server.Assistant("I'll analyze your {{language}} code and provide detailed feedback on best practices, potential issues, and suggested improvements."),
	)

	srv.Prompt("email_template", "Generate professional email content",
		server.Assistant("I'll help you create a professional email."),
		server.User("Write a {{tone}} email to {{recipient}} about {{subject}}. Include these key points: {{key_points}}"),
	)

	srv.Prompt("documentation", "Generate technical documentation",
		server.User("Create documentation for this {{type}} with the following details:\n\nName: {{name}}\nPurpose: {{purpose}}\nParameters: {{parameters}}\nExample: {{example}}"),
		server.Assistant("I'll create comprehensive technical documentation following best practices for clarity and completeness."),
	)

	// Start the server
	if err := srv.Run(); err != nil {
		logger.Error("Failed to run server", "error", err)
		os.Exit(1)
	}
}

// Helper function to determine file type from extension
func getFileType(ext string) string {
	switch ext {
	case ".go":
		return "go_source"
	case ".js", ".ts":
		return "javascript"
	case ".py":
		return "python"
	case ".md":
		return "markdown"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	default:
		return "text"
	}
}
```

## Core Concepts

### Clients and Servers

- **`client.Client`**: Interface for communicating with MCP servers. Handles protocol negotiation, request/response management, and server lifecycle.
- **`server.Server`**: Core component for implementing MCP servers. Provides registration methods for tools, resources, and prompts with automatic schema generation.

### Tools

Tools expose functions that LLMs can call to perform actions. GoMCP supports:
- **Type-safe parameters** using inline struct definitions
- **Automatic schema generation** from Go struct tags
- **Error handling** with proper MCP error responses
- **Cancellation support** for long-running operations

**Server Side (Implementation):**
```go
// Simple tool with inline struct
srv.Tool("say_hello", "Greet someone", func(ctx *server.Context, args struct {
    Name string `json:"name"`
}) (interface{}, error) {
    return map[string]interface{}{
        "message": fmt.Sprintf("Hello, %s!", args.Name),
    }, nil
})

// Tool with complex parameters
srv.Tool("calculate", "Perform calculations", func(ctx *server.Context, args struct {
    Operation string  `json:"operation"`
    A         float64 `json:"a"`
    B         float64 `json:"b"`
}) (interface{}, error) {
    switch args.Operation {
    case "add":
        return map[string]interface{}{"result": args.A + args.B}, nil
    case "multiply":
        return map[string]interface{}{"result": args.A * args.B}, nil
    default:
        return nil, fmt.Errorf("unsupported operation: %s", args.Operation)
    }
})
```

**Client Side (Usage):**
```go
// Call a simple tool
result, err := client.CallTool("say_hello", map[string]interface{}{
    "name": "Alice",
})
if err != nil {
    log.Fatalf("Tool call failed: %v", err)
}
fmt.Printf("Result: %v\n", result)

// Call a tool with complex parameters
calcResult, err := client.CallTool("calculate", map[string]interface{}{
    "operation": "add",
    "a": 10.5,
    "b": 20.3,
})
if err != nil {
    log.Fatalf("Calculation failed: %v", err)
}
fmt.Printf("Calculation result: %v\n", calcResult)
```

### Resources

Resources provide structured data to LLMs in various formats:
- **Static resources** with fixed URIs (e.g., `/config`, `/status`)
- **Templated resources** with path parameters (e.g., `/files/{path*}`, `/users/{id}`)
- **Dynamic resources** that can accept additional parameters from request bodies
- **Multiple content types** including text, images, links, and binary data

**Server Side (Implementation):**
```go
// Static resource
srv.Resource("/config", "Get configuration", func(ctx *server.Context, args *struct{}) (interface{}, error) {
    return map[string]interface{}{
        "version": "1.0.0",
        "environment": "production",
    }, nil
})

// Templated resource with path parameters
srv.Resource("/files/{path*}", "Access files", func(ctx *server.Context, args *struct {
    Path string `path:"path"`
}) (interface{}, error) {
    return map[string]interface{}{
        "path": args.Path,
        "content": fmt.Sprintf("Content of %s", args.Path),
    }, nil
})

// Resource with additional parameters
srv.Resource("/users/{id}", "Get user info", func(ctx *server.Context, args *struct {
    ID          string `path:"id"`
    IncludePosts bool   `json:"include_posts"`
}) (interface{}, error) {
    user := map[string]interface{}{
        "id": args.ID,
        "name": fmt.Sprintf("User %s", args.ID),
    }
    if args.IncludePosts {
        user["posts"] = []string{"Post 1", "Post 2"}
    }
    return user, nil
})
```

**Client Side (Usage):**
```go
// Get a static resource
config, err := client.GetResource("/config")
if err != nil {
    log.Fatalf("Failed to get config: %v", err)
}
fmt.Printf("Config: %v\n", config)

// Get a templated resource
fileContent, err := client.GetResource("/files/src/main.go")
if err != nil {
    log.Fatalf("Failed to get file: %v", err)
}
fmt.Printf("File content: %v\n", fileContent)

// Get a resource with additional parameters
user, err := client.GetResource("/users/123", client.WithResourceParams(map[string]interface{}{
    "include_posts": true,
}))
if err != nil {
    log.Fatalf("Failed to get user: %v", err)
}
fmt.Printf("User: %v\n", user)
```

### Prompts

Prompts define reusable message templates for LLM interactions:
- **Template variables** using `{{variable}}` syntax
- **Multiple message types** (User, Assistant, System)
- **Role-based conversations** for complex interaction patterns
- **Argument validation** for template parameters

**Server Side (Implementation):**
```go
// Simple prompt template
srv.Prompt("email_template", "Generate emails",
    server.User("Write a {{tone}} email to {{recipient}} about {{subject}}"),
)

// Multi-message conversation prompt
srv.Prompt("code_review", "Code review assistant",
    server.Assistant("I'll help you review code for best practices and bugs."),
    server.User("Please review this {{language}} code:\n\n```{{language}}\n{{code}}\n```"),
)

// Complex prompt with multiple variables
srv.Prompt("documentation", "Generate docs",
    server.User("Create {{type}} documentation for:\nName: {{name}}\nPurpose: {{purpose}}\nExample: {{example}}"),
    server.Assistant("I'll create comprehensive {{type}} documentation."),
)
```

**Client Side (Usage):**
```go
// Get a simple prompt
emailPrompt, err := client.GetPrompt("email_template", map[string]interface{}{
    "tone": "professional",
    "recipient": "team",
    "subject": "project update",
})
if err != nil {
    log.Fatalf("Failed to get prompt: %v", err)
}
fmt.Printf("Email prompt: %v\n", emailPrompt)

// Get a complex prompt with multiple variables
docPrompt, err := client.GetPrompt("documentation", map[string]interface{}{
    "type": "API",
    "name": "UserService",
    "purpose": "Manage user accounts",
    "example": "userService.CreateUser()",
})
if err != nil {
    log.Fatalf("Failed to get doc prompt: %v", err)
}
fmt.Printf("Documentation prompt: %v\n", docPrompt)
```

### Batch Operations

GoMCP supports JSON-RPC batch operations for improved performance:
- **Reduced network round-trips** by sending multiple requests at once
- **Maintained request ordering** in responses
- **Mixed request types** (tools, resources, prompts) in a single batch
- **Partial failure handling** with individual response errors
- **Fluent builder interface** for easy batch construction

**Client Side (Usage):**
```go
// Create a batch request
batch := client.NewBatch().
    CallTool("say_hello", map[string]interface{}{"name": "Alice"}).
    GetResource("/config").
    GetPrompt("email_template", map[string]interface{}{
        "tone": "professional",
        "recipient": "team",
        "subject": "project update",
    })

// Execute the batch
results, err := c.ExecuteBatch(batch)
if err != nil {
    log.Fatalf("Batch execution failed: %v", err)
}

// Process results
for i, result := range results {
    if result.Error != nil {
        log.Printf("Request %d failed: %v", i, result.Error)
    } else {
        log.Printf("Request %d result: %v", i, result.Result)
    }
}
```

### Event System

GoMCP provides a comprehensive event system that allows you to monitor and react to various activities within your MCP server or client. The event system uses a type-safe, channel-based architecture for maximum performance and reliability.

**Key Benefits:**
- **Real-time monitoring** of server operations and client interactions
- **Type-safe event handling** with strongly-typed event structs
- **Channel-based architecture** for high-performance, non-blocking event processing
- **Comprehensive coverage** of all server lifecycle and operation events
- **Easy integration** with logging, metrics, and monitoring systems

#### Available Event Types

GoMCP emits events for all major operations and lifecycle changes:

**Server Lifecycle Events:**
- `server.initialized` - Server has started and is ready to accept requests
- `server.shutdown` - Server is shutting down

**Client Connection Events:**
- `client.connected` - A client connected to the server
- `client.disconnected` - A client disconnected from the server  
- `client.initializing` - Client is starting to connect (client-side)
- `client.initialized` - Client successfully connected (client-side)
- `client.error` - Client operation failed (client-side)

**Registration Events:**
- `tool.registered` - A tool was registered with the server
- `resource.registered` - A resource was registered with the server

**Operation Events:**
- `tool.executed` - A tool was executed (successful operation)
- `resource.accessed` - A resource was accessed
- `prompt.executed` - A prompt was executed
- `request.failed` - Any MCP request failed

#### Server-Side Event Usage

**Setting Up Event Subscriptions:**
```go
package main

import (
    "context"
    "log/slog"
    "os"
    "github.com/localrivet/gomcp/events"
    "github.com/localrivet/gomcp/server"
)

func main() {
    // Create server
    srv := server.NewServer("my-server")
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

    // Subscribe to server lifecycle events
    events.Subscribe[events.ServerInitializedEvent](srv.Events(), events.TopicServerInitialized,
        func(ctx context.Context, evt events.ServerInitializedEvent) error {
            logger.Info("🚀 Server initialized",
                "name", evt.ServerName,
                "version", evt.ProtocolVersion,
                "tools", evt.ToolCount,
                "resources", evt.ResourceCount)
            return nil
        })

    events.Subscribe[events.ServerShutdownEvent](srv.Events(), events.TopicServerShutdown,
        func(ctx context.Context, evt events.ServerShutdownEvent) error {
            logger.Info("🛑 Server shutting down",
                "name", evt.ServerName,
                "graceful", evt.GracefulExit,
                "reason", evt.Reason)
            return nil
        })

    // Subscribe to client connection events
    events.Subscribe[events.ClientConnectedEvent](srv.Events(), events.TopicClientConnected,
        func(ctx context.Context, evt events.ClientConnectedEvent) error {
            logger.Info("🔌 Client connected",
                "sessionId", evt.SessionID,
                "clientName", evt.ClientInfo.Name,
                "clientVersion", evt.ClientInfo.Version,
                "protocolVersion", evt.ProtocolVersion)
            return nil
        })

    events.Subscribe[events.ClientDisconnectedEvent](srv.Events(), events.TopicClientDisconnected,
        func(ctx context.Context, evt events.ClientDisconnectedEvent) error {
            logger.Info("🔌 Client disconnected",
                "sessionId", evt.SessionID,
                "duration", time.Since(evt.ConnectedAt))
            return nil
        })

    // Subscribe to tool events
    events.Subscribe[events.ToolRegisteredEvent](srv.Events(), events.TopicToolRegistered,
        func(ctx context.Context, evt events.ToolRegisteredEvent) error {
            logger.Info("🔧 Tool registered",
                "name", evt.ToolName,
                "description", evt.Description)
            return nil
        })

    events.Subscribe[events.ToolExecutedEvent](srv.Events(), events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            logger.Info("⚡ Tool executed",
                "method", evt.Method,
                "success", len(evt.ResponseJSON) > 0)
            return nil
        })

    // Subscribe to resource events
    events.Subscribe[events.ResourceRegisteredEvent](srv.Events(), events.TopicResourceRegistered,
        func(ctx context.Context, evt events.ResourceRegisteredEvent) error {
            logger.Info("📄 Resource registered",
                "uri", evt.URI,
                "name", evt.Name,
                "mimeType", evt.MimeType)
            return nil
        })

    events.Subscribe[events.ResourceAccessedEvent](srv.Events(), events.TopicResourceAccessed,
        func(ctx context.Context, evt events.ResourceAccessedEvent) error {
            logger.Info("📖 Resource accessed",
                "uri", evt.URI,
                "success", evt.Success,
                "responseSize", evt.ResponseSize)
            return nil
        })

    // Subscribe to error events  
    events.Subscribe[events.RequestFailedEvent](srv.Events(), events.TopicRequestFailed,
        func(ctx context.Context, evt events.RequestFailedEvent) error {
            logger.Error("❌ Request failed",
                "method", evt.Method,
                "error", evt.Error)
            return nil
        })

    // Register tools and resources
    srv.Tool("greet", "Say hello", func(ctx *server.Context, args struct {
        Name string `json:"name"`
    }) (interface{}, error) {
        return fmt.Sprintf("Hello, %s!", args.Name), nil
    })

    srv.Resource("/status", "Server status", func(ctx *server.Context, args *struct{}) (interface{}, error) {
        return map[string]interface{}{
            "status": "healthy",
            "uptime": time.Since(startTime),
        }, nil
    })

    // Start server
    srv.AsStdio().Run()
}
```

#### Client-Side Event Usage

**Monitoring Client Operations:**
```go
package main

import (
    "context"
    "log/slog" 
    "github.com/localrivet/gomcp/client"
    "github.com/localrivet/gomcp/events"
)

func main() {
    // Create client
    c, err := client.NewClient("my-client")
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer c.Close()

    logger := slog.Default()

    // Subscribe to client lifecycle events
    events.Subscribe[events.ClientInitializingEvent](c.Events(), events.TopicClientInitializing,
        func(ctx context.Context, evt events.ClientInitializingEvent) error {
            logger.Info("🔄 Client connecting", "url", evt.URL)
            return nil
        })

    events.Subscribe[events.ClientInitializedEvent](c.Events(), events.TopicClientInitialized,
        func(ctx context.Context, evt events.ClientInitializedEvent) error {
            logger.Info("✅ Client connected", "url", evt.URL)
            return nil
        })

    events.Subscribe[events.ClientErrorEvent](c.Events(), events.TopicClientError,
        func(ctx context.Context, evt events.ClientErrorEvent) error {
            logger.Error("❌ Client error", "error", evt.Error)
            return nil
        })

    events.Subscribe[events.ClientDisconnectedEvent](c.Events(), events.TopicClientDisconnected,
        func(ctx context.Context, evt events.ClientDisconnectedEvent) error {
            logger.Info("🔌 Client disconnected", "url", evt.URL)
            return nil
        })

    // Subscribe to operation events
    events.Subscribe[events.ToolExecutedEvent](c.Events(), events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            logger.Info("⚡ Tool called", "method", evt.Method)
            return nil
        })

    events.Subscribe[events.RequestFailedEvent](c.Events(), events.TopicRequestFailed,
        func(ctx context.Context, evt events.RequestFailedEvent) error {
            logger.Error("❌ Request failed", "method", evt.Method, "error", evt.Error)
            return nil
        })

    // Connect to server and perform operations
    err = c.ConnectStdio("./my-mcp-server")
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }

    // Tool calls and other operations will now emit events
    result, err := c.CallTool("greet", map[string]interface{}{"name": "World"})
    if err != nil {
        log.Fatalf("Tool call failed: %v", err)
    }
    
    fmt.Printf("Result: %v\n", result)
}
```

#### Event System Integration Patterns

**Metrics Collection:**
```go
type MetricsCollector struct {
    toolCallCount     int64
    errorCount        int64
    connectedClients  int64
}

func (m *MetricsCollector) SetupEventSubscriptions(srv server.Server) {
    // Track tool executions
    events.Subscribe[events.ToolExecutedEvent](srv.Events(), events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            atomic.AddInt64(&m.toolCallCount, 1)
            return nil
        })

    // Track errors
    events.Subscribe[events.RequestFailedEvent](srv.Events(), events.TopicRequestFailed,
        func(ctx context.Context, evt events.RequestFailedEvent) error {
            atomic.AddInt64(&m.errorCount, 1)
            return nil
        })

    // Track connections
    events.Subscribe[events.ClientConnectedEvent](srv.Events(), events.TopicClientConnected,
        func(ctx context.Context, evt events.ClientConnectedEvent) error {
            atomic.AddInt64(&m.connectedClients, 1)
            return nil
        })

    events.Subscribe[events.ClientDisconnectedEvent](srv.Events(), events.TopicClientDisconnected,
        func(ctx context.Context, evt events.ClientDisconnectedEvent) error {
            atomic.AddInt64(&m.connectedClients, -1)
            return nil
        })
}
```

**Audit Logging:**
```go
type AuditLogger struct {
    logger *slog.Logger
}

func (a *AuditLogger) SetupAuditSubscriptions(srv server.Server) {
    // Log all tool executions for security audit
    events.Subscribe[events.ToolExecutedEvent](srv.Events(), events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            a.logger.Info("AUDIT: Tool executed",
                "method", evt.Method,
                "requestJSON", evt.RequestJSON,
                "responseJSON", evt.ResponseJSON,
                "timestamp", time.Now())
            return nil
        })

    // Log all failed requests
    events.Subscribe[events.RequestFailedEvent](srv.Events(), events.TopicRequestFailed,
        func(ctx context.Context, evt events.RequestFailedEvent) error {
            a.logger.Warn("AUDIT: Request failed",
                "method", evt.Method,
                "error", evt.Error,
                "requestJSON", evt.RequestJSON,
                "timestamp", time.Now())
            return nil
        })
}
```

**Health Monitoring:**
```go
type HealthMonitor struct {
    lastToolExecution time.Time
    errorRate         float64
    isHealthy         bool
}

func (h *HealthMonitor) SetupHealthSubscriptions(srv server.Server) {
    events.Subscribe[events.ToolExecutedEvent](srv.Events(), events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            h.lastToolExecution = time.Now()
            h.updateHealthStatus()
            return nil
        })

    events.Subscribe[events.RequestFailedEvent](srv.Events(), events.TopicRequestFailed,
        func(ctx context.Context, evt events.RequestFailedEvent) error {
            h.calculateErrorRate()
            h.updateHealthStatus()
            return nil
        })
}

func (h *HealthMonitor) updateHealthStatus() {
    h.isHealthy = time.Since(h.lastToolExecution) < 5*time.Minute && h.errorRate < 0.1
}
```

#### Event Structure Reference

All events include comprehensive metadata and follow consistent patterns:

**Server Events** include server name, timestamps, and operational metrics
**Client Events** include session IDs, protocol versions, and connection details  
**Operation Events** include method names, actual JSON payloads, and execution results
**Error Events** include detailed error information and context for debugging

For a complete working example with all event types, see [`examples/events_integration/main.go`](examples/events_integration/main.go).

### Transports

GoMCP supports multiple transport layers for different use cases:
- **stdio**: CLI tools and direct LLM integration
- **WebSocket**: Bidirectional web application communication
- **Server-Sent Events (SSE)**: Hybrid pattern with SSE for server-to-client and HTTP POST for client-to-server
- **HTTP**: Simple RESTful interfaces
- **Unix Socket**: High-performance interprocess communication
- **UDP**: Low-overhead, high-throughput communication
- **MQTT**: Publish/subscribe messaging for IoT applications
- **NATS**: Cloud-native, high-performance messaging
- **gRPC**: Service-to-service communication with strong typing

**Server Side (Implementation):**
```go
import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/sse"
)

// Stdio transport (for CLI tools and LLM integration)
srv := server.NewServer("my-server").AsStdio()

// HTTP transport
srv := server.NewServer("my-server").AsHTTP(":8080")

// WebSocket transport
srv := server.NewServer("my-server").AsWebSocket(":8080", "/mcp")

// SSE transport with single MCP endpoint (Streamable HTTP per 2025-03-26 spec)
// WARNING: Current implementation uses deprecated pattern - needs update
srv := server.NewServer("my-server").AsSSE(":8080")

// Custom MCP endpoint path
srv := server.NewServer("my-server").AsSSE(":8080", 
    sse.SSE.WithMCPEndpoint("/mcp"),
)

// Unix socket transport
srv := server.NewServer("my-server").AsUnix("/tmp/mcp.sock")

// UDP transport
srv := server.NewServer("my-server").AsUDP(":8080")

// MQTT transport
srv := server.NewServer("my-server").AsMQTT("mqtt://localhost:1883", "mcp/requests", "mcp/responses")

// NATS transport
srv := server.NewServer("my-server").AsNATS("nats://localhost:4222", "mcp.requests", "mcp.responses")

// gRPC transport
srv := server.NewServer("my-server").AsGRPC(":9090")
```

**Client Side (Usage):**
```go
// Connect via stdio (for connecting to CLI tools)
client, err := client.NewClient("my-client", 
    client.WithStdioTransport("./my-mcp-server"),
)

// Connect via HTTP
client, err := client.NewClient("my-client",
    client.WithHTTPTransport("http://localhost:8080"),
)

// Connect via WebSocket
client, err := client.NewClient("my-client",
    client.WithWebSocketTransport("ws://localhost:8080/mcp"),
)

// Connect via SSE (hybrid: SSE for receiving + HTTP POST for sending)
// The client connects to the base URL; endpoints are discovered automatically
client, err := client.NewClient("my-client",
    client.WithSSE("http://localhost:8080"),
)

// Connect via Unix socket
client, err := client.NewClient("my-client",
    client.WithUnixTransport("/tmp/mcp.sock"),
)

// Connect via UDP
client, err := client.NewClient("my-client",
    client.WithUDPTransport("localhost:8080"),
)

// Connect via MQTT
client, err := client.NewClient("my-client",
    client.WithMQTTTransport("mqtt://localhost:1883", "mcp/responses", "mcp/requests"),
)

// Connect via NATS
client, err := client.NewClient("my-client",
    client.WithNATSTransport("nats://localhost:4222", "mcp.responses", "mcp.requests"),
)

// Connect via gRPC
client, err := client.NewClient("my-client",
    client.WithGRPCTransport("localhost:9090"),
)
```

**Note on SSE Transport:**
The SSE transport correctly implements the MCP "Streamable HTTP" specification (2025-03-26):
1. **Single MCP Endpoint**: Uses one endpoint that handles both GET (for SSE) and POST (for messages)
2. **Client Requests**: Sent via HTTP POST to the MCP endpoint
3. **Server Responses**: Can be immediate JSON responses or SSE streams
4. **Server-Initiated Messages**: Sent via GET-initiated SSE streams
5. **Session Management**: Optional session IDs via `Mcp-Session-Id` header
6. **Backward Compatibility**: Supports legacy 2024-11-05 pattern with automatic fallback

### Server Management

GoMCP provides automatic management of external MCP server processes:
- **Automatic process startup** from configuration files or programmatic definitions
- **Environment variable injection** with `${VAR}` syntax
- **Graceful shutdown** and cleanup when clients disconnect
- **Multi-server support** for complex architectures
- **Health monitoring** and connection management

**Client Side (Usage):**
```go
// Define server configuration
config := client.ServerConfig{
    MCPServers: map[string]client.ServerDefinition{
        "file-server": {
            Command: "python",
            Args: []string{"-m", "mcp_server_files"},
            Env: map[string]string{
                "FILES_ROOT": "/workspace",
                "LOG_LEVEL": "info",
            },
        },
        "database-server": {
            Command: "./db-mcp-server",
            Args: []string{"--config", "config.json"},
            Env: map[string]string{
                "DATABASE_URL": "${DATABASE_URL}",
                "API_KEY": "${DB_API_KEY}",
            },
            WorkingDirectory: "/opt/db-server",
        },
        "ai-tools": {
            Command: "ai-mcp-tools",
            Args: []string{"--model", "gpt-4"},
            Env: map[string]string{
                "OPENAI_API_KEY": "${OPENAI_API_KEY}",
                "ANTHROPIC_API_KEY": "${ANTHROPIC_API_KEY}",
            },
        },
    },
}

// Create client with automatic server management
client, err := client.NewClient("orchestrator",
    client.WithServers(config, "file-server"), // Start file-server
)
if err != nil {
    log.Fatalf("Failed to create client: %v", err)
}
defer client.Close() // Automatically stops managed servers

// Start additional servers on demand
err = client.StartServer("database-server")
if err != nil {
    log.Fatalf("Failed to start database server: %v", err)
}

// Use multiple servers
fileResult, err := client.CallTool("list_files", map[string]interface{}{
    "path": "/workspace/src",
})

// Switch to database server context or use a different client instance
dbClient, err := client.NewClient("db-client",
    client.WithServers(config, "database-server"),
)
defer dbClient.Close()

queryResult, err := dbClient.CallTool("execute_query", map[string]interface{}{
    "sql": "SELECT * FROM users LIMIT 10",
})

// Load configuration from file
configFromFile, err := client.LoadServerConfig("mcp-servers.json")
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}

// Create client with all servers from config
multiClient, err := client.NewClient("multi-server",
    client.WithServersFromConfig(configFromFile, "file-server", "ai-tools"),
)
defer multiClient.Close()

// Health monitoring
status := client.GetServerStatus("file-server")
if !status.Running {
    log.Printf("File server is not running: %v", status.Error)
    err := client.RestartServer("file-server")
    if err != nil {
        log.Fatalf("Failed to restart server: %v", err)
    }
}
```

**Configuration File Example (`mcp-servers.json`):**
```json
{
  "mcpServers": {
    "file-server": {
      "command": "python",
      "args": ["-m", "mcp_server_files"],
      "env": {
        "FILES_ROOT": "/workspace",
        "LOG_LEVEL": "info"
      }
    },
    "database-server": {
      "command": "./db-mcp-server",
      "args": ["--config", "config.json"],
      "env": {
        "DATABASE_URL": "${DATABASE_URL}",
        "API_KEY": "${DB_API_KEY}"
      },
      "workingDirectory": "/opt/db-server"
    }
  }
}
```

### Proper Cleanup Patterns

When using server registries with multiple MCP servers, it's important to follow proper cleanup patterns to avoid race conditions:

#### ✅ Correct Pattern: Use registry.StopAll()

```go
registry := client.NewServerRegistry(
    client.WithRegistryLogger(logger),
)

// IMPORTANT: Use defer registry.StopAll() for proper cleanup
defer func() {
    if err := registry.StopAll(); err != nil {
        log.Printf("Warning: Error during server shutdown: %v", err)
    }
}()

err := registry.LoadConfig(configFile)
// ... use the servers ...
```

#### ❌ Incorrect Pattern: Manual client.Close() before registry.StopAll()

```go
// DON'T DO THIS - creates race condition
defer func() {
    client.Close()         // ❌ Kills connection/process immediately
    registry.StopAll()     // ❌ Tries to wait for already-killed process
}()
```

**Why this creates a race condition:**
1. `client.Close()` immediately terminates the MCP connection and may kill the server process
2. `registry.StopAll()` then tries to gracefully shut down processes that are already dead
3. This results in "Failed to wait for server process error='signal: killed'" errors

#### Best Practices

- **Always use `registry.StopAll()`** - it handles all client cleanup internally
- **Use defer blocks** for guaranteed cleanup on program exit
- **Don't mix manual client.Close() with registry.StopAll()**
- **Handle cleanup errors gracefully** - processes may have already exited

### Session Management

GoMCP v1.5.5 introduces comprehensive session management with the MCP Session Architecture, providing rich context and automated workspace discovery:

#### Server-Side Session Access

```go
// Tool handlers receive session context automatically
srv.Tool("analyze_project", "Analyze project structure", func(ctx *server.Context, args struct {
    AnalysisType string `json:"analysis_type"`
}) (interface{}, error) {
    // Access session environment (from transport headers/process env)
    env := ctx.Session.Env()
    apiKey := env["ANTHROPIC_API_KEY"]
    
    // Access workspace roots (from clientInfo + automated roots/list)
    roots := ctx.Session.Roots()
    primaryRoot := ""
    if len(roots) > 0 {
        primaryRoot = roots[0]
    }
    
    // Access client capabilities
    caps := ctx.Session.Capabilities()
    
    return map[string]interface{}{
        "primary_workspace": primaryRoot,
        "all_roots": roots,
        "has_api_access": apiKey != "",
        "supports_sampling": caps.Sampling.Supported,
        "analysis_type": args.AnalysisType,
    }, nil
})
```

#### Transport-Aware Session Data

GoMCP automatically extracts session data from the transport layer per MCP specification:

- **stdio**: Environment from process environment variables
- **HTTP**: Environment from request headers (`X-Env-*` pattern)
- **WebSocket**: Environment from connection headers
- **SSE**: Environment from initial request headers

#### Automated Workspace Root Discovery

The server automatically detects when clients support the `roots` capability and:

1. **Initial Extraction**: Extracts workspace roots from `clientInfo.roots` during initialization
2. **Capability Detection**: Detects if client advertises `roots` capability 
3. **Automated Fetching**: Sends `roots/list` requests after `notifications/initialized`
4. **Response Handling**: Processes `roots/list` responses with proper request tracking
5. **Context Integration**: Makes roots available via `ctx.Session.Roots()`

#### MCP Protocol Compliance

Full compliance across all three MCP protocol versions:
- **2024-11-05**: Basic root extraction and capability detection
- **2025-03-26**: Enhanced session management with audio support detection
- **draft**: Latest features with full session architecture

#### ClientInfo Structure

Enhanced `ClientInfo` provides comprehensive session data:

```go
type ClientInfo struct {
    Name              string                 `json:"name"`
    Version           string                 `json:"version"`
    SamplingSupported bool                   `json:"sampling_supported,omitempty"`
    SamplingCaps      *SamplingCapabilities  `json:"sampling_caps,omitempty"`
    ProtocolVersion   string                 `json:"protocol_version,omitempty"`
    Env               map[string]string      `json:"env,omitempty"`           // NEW: Environment data from transport
    Roots             []string               `json:"roots,omitempty"`         // NEW: Workspace roots from init + roots/list
}
```

#### Session Convenience Methods

The `ClientSession` interface provides easy access to session data:

```go
// In tool handlers
func MyTool(ctx *server.Context, args MyArgs) (interface{}, error) {
    session := ctx.Session
    
    // Get environment variables (from transport)
    env := session.Env()
    dbUrl := env["DATABASE_URL"]
    
    // Get workspace roots (from init + automated roots/list)
    roots := session.Roots()
    
    // Get client capabilities  
    caps := session.Capabilities()
    supportsSampling := caps.Sampling.Supported
    supportsAudio := caps.Audio.Supported
    
    // Use session data in tool logic...
}
```

#### Benefits

- **Zero Configuration**: Automatic session data extraction with no manual setup
- **MCP Compliant**: Follows official MCP specification for session handling
- **Transport Agnostic**: Works consistently across all transport types
- **Backward Compatible**: No breaking changes to existing tool handlers

## Examples

The `examples/` directory contains complete examples demonstrating various features:

- `examples/minimal/`: Basic client and server examples
- `examples/sampling/`: Examples of text generation via the sampling API
- `examples/server_config/`: Server management and configuration examples
- `examples/server/`: Various server implementation patterns

## Documentation

- [GoDoc](https://pkg.go.dev/github.com/localrivet/gomcp): API reference documentation
- `docs/`: Additional documentation and guides
  - `docs/examples/`: Detailed feature guides
  - `docs/getting-started/`: Getting started guides
  - `docs/api-reference/`: Detailed API documentation

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
