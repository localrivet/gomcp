# GoMCP - Go Model Context Protocol Library

[![Go Reference](https://pkg.go.dev/badge/github.com/localrivet/gomcp.svg)](https://pkg.go.dev/github.com/localrivet/gomcp)
[![Go Report Card](https://goreportcard.com/badge/github.com/localrivet/gomcp)](https://goreportcard.com/report/github.com/localrivet/gomcp)

<!-- TODO: Add build status badge once CI is set up -->
<!-- TODO: Add code coverage badge -->

`gomcp` provides a robust Go implementation of the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction). MCP is an open protocol designed to standardize communication between AI language models/agents and external data sources, tools, and capabilities.

This library enables you to build both:

- **MCP Clients:** Applications that connect to MCP servers to utilize their tools, resources, and prompts.
- **MCP Servers:** Applications that expose tools, resources, and prompts to MCP clients.

**Compliance:** Fully compliant with MCP Specification versions **[2025-03-26](https://modelcontextprotocol.io/specification/2025-03-26/)** and **[2024-11-05](https://modelcontextprotocol.io/specification/2024-11-05/)**.

## Table of Contents

- [What is MCP?](#what-is-mcp)
- [Why GoMCP?](#why-gomcp)
- [Key Features](#key-features)
- [Installation](#installation)
- [Quickstart](#quickstart)
- [Core Concepts](#core-concepts)
  - [The `server.Server` and `client.Client`](#the-serverserver-and-clientclient)
  - [Tools](#tools)
  - [Resources](#resources)
  - [Prompts](#prompts)
  - [Transports](#transports)
- [Examples](#examples)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## What is MCP?

The [Model Context Protocol (MCP)](https://modelcontextprotocol.io) lets you build servers that expose data and functionality to LLM applications in a secure, standardized way. Think of it like a web API, but specifically designed for LLM interactions. MCP servers can:

- Expose data through **Resources** (think GET endpoints; load info into context)
- Provide functionality through **Tools** (think POST/PUT endpoints; execute actions)
- Define interaction patterns through **Prompts** (reusable templates)
- And more!

GoMCP provides a robust, idiomatic Go interface for building and interacting with these servers.

## Why GoMCP?

The MCP protocol is powerful but implementing it involves details like server setup, protocol handlers, content types, and error management. GoMCP handles these protocol details, letting you focus on building your application's capabilities.

GoMCP aims to be:

- **Robust:** Implements the MCP specification thoroughly.
- **Performant:** Leverages Go's concurrency and efficiency.
- **Idiomatic:** Feels natural to Go developers.
- **Complete:** Provides a full implementation of the core MCP specification for both servers and clients.
- **Flexible:** Supports multiple transport layers and offers hooks for customization.

## Key Features

- **Full MCP Compliance:** Supports all core features of the `2025-03-26` and `2024-11-05` specifications.
- **Fluent Interface:** Build servers with a clean, chainable API for improved readability and developer experience.
- **Protocol Version Negotiation:** Automatically negotiates the highest compatible protocol version during the handshake.
- **Transport Agnostic Core:** Server (`server/`) and Client (`client/`) logic is decoupled from the underlying transport.
- **Multiple Transports:** Includes implementations for common communication methods:
  - **Streamable HTTP (SSE + POST):** The primary network transport (`transport/sse/`).
  - **WebSocket:** For persistent, bidirectional network communication (`transport/websocket/`).
  - **Standard Input/Output (Stdio):** Ideal for local inter-process communication (`transport/stdio/`).
  - **TCP:** For raw TCP socket communication (`transport/tcp/`).
- **Helper Utilities:** Provides helpers for argument parsing (`util/schema`), progress reporting (`util/progress`), response creation (`server.Text`, `server.JSON`), etc.
- **Extensible Hooks:** Offers a hook system (`hooks/`) for intercepting and modifying behavior.
- **`2025-03-26` Enhancements:** Includes support for the Authorization Framework, JSON-RPC Batching, Tool Annotations, Audio Content, Completions, and more.

## Installation

```bash
go get github.com/localrivet/gomcp
```

## Quickstart

Let's create a simple MCP server that exposes a calculator tool and a client that uses it, communicating over stdio.

**Server (`calculator_server/main.go`)**

```go
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/localrivet/gomcp/server"
)

// Arguments for the 'add' tool
type AddArgs struct {
	A float64 `json:"a" description:"First number" required:"true"`
	B float64 `json:"b" description:"Second number" required:"true"`
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[CalcServer] ")
	log.Println("Starting Calculator MCP Server (Stdio)...")

	// Create the MCP server instance with fluent interface
	srv := server.NewServer("calculator-stdio").
		// Add the 'add' tool using the fluent Tool method
		// This method infers the schema from the handler's argument type (AddArgs)
		Tool("add", "Add two numbers.",
			// Handler function using the args struct
			func(ctx *server.Context, args AddArgs) (string, error) {
				result := args.A + args.B
				ctx.Info("[add tool] %f + %f -> %f", args.A, args.B, result)
				// Return result as a string
				return strconv.FormatFloat(result, 'f', -1, 64), nil
			})

	// Start the server using the fluent AsStdio and Run methods
	log.Println("Server setup complete. Starting stdio server...")
	if err := srv.AsStdio().Run(); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
	log.Println("Server shutdown complete.")
}

```

**Client (`calculator_client/main.go`)**

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[CalcClient] ")
	log.Println("Starting Calculator MCP Client (Stdio)...")

	// Create a client configured for stdio communication
	// Point it to the server executable (adjust path as needed)
	// For simple Go examples, you might run the server first and then the client.
	// If running separately, use the actual command to start the server:
	// cmd := []string{"go", "run", "../calculator_server/main.go"}
	// clt, err := client.NewStdioClientWithCommand("MyCalcClient", client.ClientOptions{}, cmd)
	// For simplicity here, assume server is already running and connected to stdin/stdout
	clt, err := client.NewStdioClient("MyCalcClient", client.ClientOptions{})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Set a timeout for the connection and operations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect and perform initialization handshake
	log.Println("Connecting to server via stdio...")
	if err = clt.Connect(ctx); err != nil {
		log.Fatalf("Client failed to connect: %v", err)
	}
	defer clt.Close() // Ensure connection resources are cleaned up

	serverInfo := clt.ServerInfo()
	log.Printf("Connected to server: %s (Version: %s)", serverInfo.Name, serverInfo.Version)

	// Call the 'add' tool
	log.Println("--- Calling 'add' Tool ---")
	addArgs := map[string]interface{}{"a": 15.5, "b": 4.5}
	callParams := protocol.CallToolParams{Name: "add", Arguments: addArgs}

	callCtx, callCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer callCancel()

	callResult, err := clt.CallTool(callCtx, callParams, nil) // No progress token needed
	if err != nil {
		log.Printf("Error calling tool 'add': %v", err)
	} else if callResult.IsError {
		log.Printf("Tool 'add' call returned an error:")
		for _, content := range callResult.Content {
			if textContent, ok := content.(protocol.TextContent); ok {
				log.Printf("  Error Content: %s", textContent.Text)
			}
		}
	} else {
		log.Printf("Tool 'add' call successful:")
		for _, content := range callResult.Content {
			if textContent, ok := content.(protocol.TextContent); ok {
				log.Printf("  Result: %s", textContent.Text) // Expecting "20"
			}
		}
	}

	log.Println("Client finished.")
}
```

**Running the Quickstart:**

1.  Save the server code as `calculator_server/main.go`.
2.  Save the client code as `calculator_client/main.go`.
3.  Compile and run the server: `go run calculator_server/main.go`
4.  In **another terminal**, compile and run the client: `go run calculator_client/main.go`

The client will connect to the server via stdio, call the `add` tool, print the result, and exit.

## Core Concepts

These are the building blocks for creating MCP servers and clients with GoMCP.

### The `server.Server` and `client.Client`

- `server.Server`: The central object for your MCP server application. You register tools, resources, and prompts with it.
- `client.Client`: Represents a connection to an MCP server, allowing you to interact with its capabilities.

```go
// Server Initialization with fluent interface
srv := server.NewServer("my-awesome-server").
    Tool("greet", "Greet a user", handleGreeting).
    Resource("app://info/version", server.WithTextContent("1.0.0")).
    AsWebsocket(":9090", "/mcp").
    Run()

// Client Initialization (Stdio example)
clt, err := client.NewStdioClient("my-cool-client", client.ClientOptions{})
if err != nil { /* handle error */ }
```

### Tools

Tools allow clients (like LLMs) to perform actions by executing functions on your server. GoMCP provides a fluent interface for registering tools with automatic schema generation.

```go
package main

import (
	"fmt"
	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create a server with fluent interface
	srv := server.NewServer("tool-demo").
		// Register a simple tool that reverses a string
		Tool("reverse_string", "Reverses the input string",
			func(ctx *server.Context, args struct {
				Input string `json:"input" description:"The string to reverse" required:"true"`
			}) (string, error) {
				// Get the input from args and reverse it
				runes := []rune(args.Input)
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				return string(runes), nil
			})

	// Start the server
	srv.AsStdio().Run()
}
```

You can also use pre-defined struct types for more complex arguments:

```go
// Define the arguments struct
type SearchArgs struct {
	Query      string   `json:"query" description:"Search term" required:"true"`
	MaxResults int      `json:"max_results" description:"Maximum results to return" default:"10"`
	Filters    []string `json:"filters" description:"Result filters"`
}

// Register with the fluent interface
srv.Tool("search", "Search for items",
	func(ctx *server.Context, args SearchArgs) (map[string]interface{}, error) {
		// Implementation...
		return results, nil
	})
```

### Resources

Resources expose data to clients. They are primarily for providing information without significant computation or side effects (like GET requests). In GoMCP, you can register resources using the fluent `Resource` method on the server, along with functional options.

```go
package main

import (
	"fmt"
	"time"

	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create a server with fluent interface
	srv := server.NewServer("resource-demo").
		// Register a simple text resource
		Resource("app://info/version",
			server.WithTextContent("1.2.3"),
			server.WithName("App Version"),
			server.WithDescription("Get the application version."),
			server.WithMimeType("text/plain"),
		).
		// Register a resource with dynamic content from a handler
		Resource("user://data/{userID}/profile",
			server.WithHandler(func(ctx *server.Context, userID string) (map[string]interface{}, error) {
				ctx.Info("Handling user profile request for: %s", userID)
				// Fetch user data from database, etc.
				userData := map[string]interface{}{
					"id":        userID,
					"email":     fmt.Sprintf("user%s@example.com", userID),
					"lastLogin": time.Now().Format(time.RFC3339),
				}
				return userData, nil
			}),
			server.WithName("User Profile"),
			server.WithDescription("Get user profile data."),
		)

	// Start the server
	srv.AsSSE(":9090", "/mcp").Run()
}
```

### Prompts

Prompts define reusable templates or interaction patterns for the client (often an LLM). GoMCP provides a fluent interface for registering prompts with your server.

````go
package main

import (
	"fmt"
	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create a server with fluent interface
	srv := server.NewServer("prompt-demo").
		// Register a simple text prompt
		Prompt("prompt://tasks/summarize",
			server.WithPromptHandler(func(ctx *server.Context, args struct {
				Text string `json:"text" description:"The text to summarize" required:"true"`
			}) (string, error) {
				return fmt.Sprintf("Please summarize the following text concisely:\n\n%s", args.Text), nil
			}),
			server.WithPromptName("Summarize Text"),
			server.WithPromptDescription("Generate a prompt asking the LLM to summarize text.")).
		// Register another prompt
		Prompt("prompt://coding/refactor",
			server.WithPromptHandler(func(ctx *server.Context, args struct {
				Code     string `json:"code" description:"The code to refactor" required:"true"`
				Language string `json:"language" description:"Programming language" default:"go"`
			}) (string, error) {
				return fmt.Sprintf("Refactor this %s code to improve readability and efficiency:\n\n```%s\n%s\n```",
					args.Language, args.Language, args.Code), nil
			}),
			server.WithPromptName("Code Refactoring"),
			server.WithPromptDescription("Generate a prompt for code refactoring."))

	// Start the server
	srv.AsWebsocket(":9090", "/mcp").Run()
}
````

Prompts can also be registered with static content:

```go
srv.Prompt("prompt://greeting/welcome",
	server.WithPromptTextContent("Welcome to our service! I'm an AI assistant ready to help you with any questions about our API."),
	server.WithPromptName("Welcome Greeting"),
	server.WithPromptDescription("A friendly welcome message to new users."))
```

### Transports

GoMCP separates the core protocol logic from how clients and servers communicate. The fluent interface provides simple methods to configure and start the server with different transports:

```go
// WebSocket
srv.AsWebsocket(":9090", "/mcp").Run()

// SSE (Server-Sent Events)
srv.AsSSE(":9090", "/mcp").Run()

// Stdio
srv.AsStdio().Run()

// TCP
srv.AsTCP(":9090").Run()
```

Client-side, use constructors like `client.NewStdioClient(...)` or `client.NewSSEClient(url, ...)`.

## Quick Reference: Complete Server Example

Here's a comprehensive example showing how to build a complete server with the fluent interface pattern:

```go
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/server"
)

func main() {
	// Configure logging
	log.SetOutput(os.Stderr)
	log.SetPrefix("[MCP Server] ")

	// Create a new server with fluent interface
	server.NewServer("demo-server").
		// Register tools
		Tool("greet", "Greet a user",
			func(ctx *server.Context, args struct {
				Name string `json:"name" description:"User name" required:"true"`
			}) (string, error) {
				return fmt.Sprintf("Hello, %s! Welcome to GoMCP.", args.Name), nil
			}).
		Tool("calculate", "Perform a calculation",
			func(ctx *server.Context, args struct {
				Operation string  `json:"operation" description:"Operation to perform (add, subtract, multiply, divide)" required:"true"`
				A         float64 `json:"a" description:"First number" required:"true"`
				B         float64 `json:"b" description:"Second number" required:"true"`
			}) (map[string]interface{}, error) {
				var result float64

				// Report progress
				ctx.ReportProgress("Starting calculation", 0, 3)

				// Simulate some work
				time.Sleep(100 * time.Millisecond)
				ctx.ReportProgress("Processing inputs", 1, 3)

				// Perform the calculation
				switch args.Operation {
				case "add":
					result = args.A + args.B
				case "subtract":
					result = args.A - args.B
				case "multiply":
					result = args.A * args.B
				case "divide":
					if args.B == 0 {
						return nil, fmt.Errorf("division by zero")
					}
					result = args.A / args.B
				default:
					return nil, fmt.Errorf("unknown operation: %s", args.Operation)
				}

				time.Sleep(100 * time.Millisecond)
				ctx.ReportProgress("Calculation complete", 3, 3)

				return map[string]interface{}{
					"operation": args.Operation,
					"a":         args.A,
					"b":         args.B,
					"result":    result,
				}, nil
			}).
		// Register resources
		Resource("app://info/version",
			server.WithTextContent("1.0.0"),
			server.WithName("App Version"),
			server.WithDescription("Get the application version")).
		Resource("app://info/status",
			server.WithJSONContent(map[string]interface{}{
				"status": "online",
				"uptime": "12h",
				"connections": 42,
			}),
			server.WithName("App Status"),
			server.WithDescription("Get server status information")).
		Resource("users://{userID}/profile",
			server.WithHandler(func(ctx *server.Context, userID string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id":       userID,
					"name":     fmt.Sprintf("User %s", userID),
					"lastSeen": time.Now().Format(time.RFC3339),
				}, nil
			}),
			server.WithName("User Profile"),
			server.WithDescription("Get user profile by ID")).
		// Register prompts
		Prompt("prompt://instructions/get-help",
			server.WithPromptTextContent(
				"You are a helpful assistant for the GoMCP API. " +
				"Help the user understand how to use the API effectively. " +
				"Provide code examples when appropriate."),
			server.WithPromptName("Help Instructions"),
			server.WithPromptDescription("Instructions for helping users with the API")).
		// Configure and start
		AsWebsocket(":9090", "/mcp").
		Run()
}
```

This example demonstrates:

- Tool registration with argument validation and progress reporting
- Resource registration with static and dynamic content
- Prompt registration
- Transport configuration
- Server startup

## Examples

The `examples/` directory contains various client/server pairs demonstrating specific features, transports, and integrations:

- **Basic Usage:**
  - `basic/`: Simple stdio communication with multiple tools.
  - `hello-demo/`: Minimal example showcasing tool, prompt, and resource registration (stdio).
  - `client_helpers_demo/`: Demonstrates client-side helpers for configuration loading and tool calls.
- **Network Transports:**
  - `http/`: Integration with various Go HTTP frameworks using the **Streamable HTTP (SSE)** transport (includes Gin, Echo, Chi, Fiber, etc.).
  - `websocket/`: Demonstrates the **WebSocket** transport.
- **Configuration & Deployment:**
  - `configuration/`: Loading server configuration from files (JSON, YAML, TOML).
  - `cmd/`: Generic command-line client and server implementations configurable for different transports.
- **Advanced Features:**
  - `auth/`: Simple API key authentication hook example (stdio).
  - `rate-limit/`: Example of rate limiting client requests (stdio).
  - `kitchen-sink/`: Comprehensive server example combining multiple features (stdio).
  - `code-assistant/`: Example server providing code review/documentation tools.
  - `meta-tool-demo/`: Server demonstrating a tool that calls other tools.

**Running Examples:**

1.  Navigate to an example's server directory (e.g., `cd examples/websocket/server`).
2.  Run the server: `go run .`
3.  In another terminal, navigate to the corresponding client directory (e.g., `cd examples/websocket/client`).
4.  Run the client: `go run .`

_(Check the specific README within each example directory for more detailed instructions if available.)_

## Documentation

- **Go Packages:** [pkg.go.dev/github.com/localrivet/gomcp](https://pkg.go.dev/github.com/localrivet/gomcp) - Browse the API documentation.
- **MCP Specification:** [modelcontextprotocol.io](https://modelcontextprotocol.io) - Understand the underlying protocol.
- **Project Docs Site (WIP):** Check the `docs/` directory and potential future hosted site for guides and tutorials.
- **Project Documentation Site:** [gomcp.dev](https://gomcp.dev) - View guides, tutorials, and examples.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to contribute, report issues, and propose features.

## License

This project is licensed under the [MIT License](LICENSE).
