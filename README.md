# GoMCP - Go Model Context Protocol Library

[![Go Reference](https://pkg.go.dev/badge/github.com/localrivet/gomcp.svg)](https://pkg.go.dev/github.com/localrivet/gomcp)
[![Go Report Card](https://goreportcard.com/badge/github.com/localrivet/gomcp)](https://goreportcard.com/report/github.com/localrivet/gomcp)

GoMCP is a complete Go implementation of the Model Context Protocol (MCP), designed to facilitate seamless interaction between applications and Large Language Models (LLMs). The library supports all specification versions with automatic negotiation and provides a clean, idiomatic API for both clients and servers.

## Table of Contents

- [Overview](#overview)
- [Key Features](#key-features)
- [Installation](#installation)
- [Quickstart](#quickstart)
  - [Client Example](#client-example)
  - [Server Example](#server-example)
  - [Server Management Example](#server-management-example)
- [Core Concepts](#core-concepts)
  - [Clients and Servers](#clients-and-servers)
  - [Tools](#tools)
  - [Resources](#resources)
  - [Prompts](#prompts)
  - [Transports](#transports)
  - [Server Management](#server-management)
- [Examples](#examples)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Overview

The Model Context Protocol (MCP) standardizes communication between applications and LLMs, enabling:

- **Tool Calling**: Execute actions and functions through LLMs
- **Resource Access**: Provide structured data to LLMs
- **Prompt Rendering**: Create reusable templates for LLM interactions
- **Sampling**: Generate text from LLMs with control over parameters

GoMCP provides an idiomatic Go implementation that handles all the protocol details while offering a clean, developer-friendly API.

## Key Features

- **Complete Protocol Implementation**: Full support for all MCP specification versions
- **Automatic Version Negotiation**: Seamless compatibility between clients and servers
- **Multiple Transport Options**: Support for stdio, HTTP, WebSocket, and Server-Sent Events
- **Type-Safe API**: Leverages Go's type system for safety and expressiveness
- **Server Process Management**: Automatically start, manage, and stop external MCP servers
- **Server Configuration**: Load server definitions from configuration files
- **Flexible Architecture**: Modular design for easy extension and customization

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
	// Create a new client
	c, err := client.NewClient("my-client",
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
	s := server.NewServer("example-server",
		server.WithLogger(logger),
	).AsStdio()

	// Register a tool
	s.Tool("say_hello", "Greet someone", func(ctx *server.Context, args struct {
		Name string `json:"name"`
	}) (string, error) {
		return fmt.Sprintf("Hello, %s!", args.Name), nil
	})

	// Start the server
	s.Run()
}
```

### Server Management Example

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
			"task-master-ai": {
				Command: "npx",
				Args: []string{"-y", "--package=task-master-ai", "task-master-ai"},
				Env: map[string]string{
					"ANTHROPIC_API_KEY": "${ANTHROPIC_API_KEY}",
				},
			},
		},
	}

	// Create a client with automatic server management
	c, err := client.NewClient("my-client",
		client.WithServers(config, "task-master-ai"),
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
}
```

## Core Concepts

### Clients and Servers

- **`client.Client`**: Interface for communicating with MCP servers
- **`server.Server`**: Core component for implementing MCP servers

### Tools

Tools allow you to expose functionality to LLMs:

```go
// Register a tool with a struct for type-safe parameters
s.Tool("calculate", "Perform calculations", func(ctx *server.Context, args struct {
	Operation string `enum:"add,subtract,multiply,divide"` // Tag-based validation
	X         float64
	Y         float64
}) (string, error) {
	// Implementation...
})
```

### Resources

Resources provide data to LLMs:

```go
// Register a static resource
s.Resource("app/version", "Get application version",
	func(ctx *server.Context) (string, error) {
		return "1.0.0", nil
	})

// Register a resource with parameters
s.Resource("users/{id}", "Get user information",
	func(ctx *server.Context, args struct {
		ID string `path:"id"`
	}) (map[string]interface{}, error) {
		return map[string]interface{}{
			"id": args.ID,
			"name": "Example User",
		}, nil
	})
```

### Prompts

Prompts define reusable templates for LLM interactions:

```go
// Register a prompt template
s.Prompt("greeting", "Greet a user",
	func(ctx *server.Context, args struct {
		Name string
		Service string
	}) (string, error) {
		return fmt.Sprintf("Hello %s, welcome to %s!", args.Name, args.Service), nil
	})
```

### Transports

GoMCP supports multiple transport layers:

- **stdio**: For CLI tools and direct LLM integration
- **WebSocket**: For web applications with bidirectional communication
- **Server-Sent Events (SSE)**: For web applications with server-to-client streaming
- **HTTP**: For simple RESTful interfaces

### Server Management

GoMCP provides robust functionality for managing external MCP server processes:

```go
// Load configuration from file
client, err := client.NewClient("my-client",
	client.WithServerConfig("mcpservers.json", "task-master-ai"),
)

// Or define configuration programmatically
config := client.ServerConfig{
	MCPServers: map[string]client.ServerDefinition{
		"memory-server": {
			Command: "npx",
			Args: []string{"-y", "@modelcontextprotocol/server-memory"},
			Env: map[string]string{"DEBUG": "true"},
		},
	},
}

client, err := client.NewClient("my-client",
	client.WithServers(config, "memory-server"),
)
```

The server management system:

- Automatically starts the specified server process
- Connects the client to the server
- Manages environment variables and arguments
- Properly terminates the server when the client is closed

For advanced use cases, you can use the `ServerRegistry` directly:

```go
registry := client.NewServerRegistry()
if err := registry.LoadConfig("mcpservers.json"); err != nil {
	log.Fatalf("Failed to load configuration: %v", err)
}

// Get all available servers
serverNames, _ := registry.GetServerNames()

// Get a client for a specific server
memoryClient, _ := registry.GetClient("memory")

// Stop all servers when done
registry.StopAll()
```

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
