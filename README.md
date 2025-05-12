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

	"github.com/localrivet/gomcp/protocol"
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

	// Create the MCP server instance
	srv := server.NewServer("calculator-stdio")

	// Add the 'add' tool using the server.AddTool helper
	// The helper infers the schema from the handler's argument type (AddArgs)
	err := server.AddTool(
		srv,
		"add",
		"Add two numbers.",
		// Handler function using the args struct
		func(args AddArgs) (protocol.Content, error) {
			result := args.A + args.B
			log.Printf("[add tool] %f + %f -> %f", args.A, args.B, result)
			// Use server.Text helper for simple text responses
			return server.Text(strconv.FormatFloat(result, 'f', -1, 64)), nil
		},
	)
	if err != nil {
		log.Fatalf("Failed to add 'add' tool: %v", err)
	}

	// Start the server using the built-in stdio handler.
	// This blocks until the server exits (e.g., stdin is closed).
	log.Println("Server setup complete. Listening on stdio...")
	if err := server.ServeStdio(srv); err != nil {
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
// Server Initialization
srv := server.NewServer("my-awesome-server")

// Client Initialization (Stdio example)
clt, err := client.NewStdioClient("my-cool-client", client.ClientOptions{})
if err != nil { /* handle error */ }
```

### Tools

Tools allow clients (like LLMs) to perform actions by executing functions on your server. Use `server.AddTool` for easy registration with automatic schema generation based on a Go struct for arguments.

```go
package main

import (
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	// ... other imports
)

// Define the arguments struct for your tool
type ReverseArgs struct {
	Input string `json:"input" description:"The string to reverse" required:"true"`
}

// Your tool handler function
func reverseStringHandler(args ReverseArgs) (protocol.Content, error) {
	runes := []rune(args.Input)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	reversed := string(runes)
	return server.Text(reversed), nil // Helper for simple text responses
}

func registerTools(srv *server.Server) error {
	// Register the tool using the helper
	err := server.AddTool(
		srv,
		"reverse_string",
		"Reverses the input string.",
		reverseStringHandler, // Pass the handler function
	)
	return err
}
```

Alternatively, you can provide a `protocol.ToolDefinition` manually for more control.

#### Alternative: Using `RegisterTool` and `schema.FromStruct`

For more direct control over the tool definition and handler signature, you can use `srv.RegisterTool` and explicitly generate the schema using `schema.FromStruct` from the `util/schema` package.

This approach requires you to manually handle argument parsing within your handler function, which receives the raw `protocol.ToolCall`.

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/util/schema" // Import schema helper
	// ... other imports
)

// Define the arguments struct (still useful for schema generation)
type MultiplyArgs struct {
	X float64 `json:"x" description:"Multiplicand" required:"true"`
	Y float64 `json:"y" description:"Multiplier" required:"true"`
}

// Handler function for RegisterTool takes protocol.ToolCall
func multiplyHandler(call protocol.ToolCall) (protocol.ToolResult, error) {
	var args MultiplyArgs
	// Manually parse arguments
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		log.Printf("[multiplyHandler] Error parsing args: %v", err)
		// Return a structured error result
		return protocol.ToolResult{IsError: true, Content: []protocol.Content{server.Text(fmt.Sprintf("Invalid arguments: %v", err))}}, nil
	}

	result := args.X * args.Y
	log.Printf("[multiply tool] %f * %f -> %f", args.X, args.Y, result)

	// Return a successful result
	return protocol.ToolResult{Content: []protocol.Content{server.Text(fmt.Sprintf("%f", result))}}, nil
}


func registerToolsManually(srv *server.Server) {
	// Register the tool using RegisterTool
	srv.RegisterTool(
		protocol.Tool{ // Define the full Tool struct
			Name:        "multiply",
			Description: "Multiplies two numbers.",
			// Generate schema explicitly
			InputSchema: schema.FromStruct(MultiplyArgs{}),
			// OutputSchema can also be defined here if needed
		},
		multiplyHandler, // Pass the handler with the (ToolCall) signature
	)
	// Note: Error handling for RegisterTool might differ or be absent
	// depending on the implementation version or desired behavior.
	// Check server implementation details if needed.
}
```

### Resources

Resources expose data to clients. They are primarily for providing information without significant computation or side effects (like GET requests). Use `server.AddResource`. Dynamic URIs with `{placeholders}` are supported.

```go
package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	// ... other imports
)

// Static resource handler
func handleAppVersion(uri *url.URL) (protocol.Content, error) {
	return server.Text("1.2.3"), nil
}

// Dynamic resource handler (extracting part from URI)
func handleUserData(uri *url.URL) (protocol.Content, error) {
	// Example URI: user://data/123/profile
	// We need to parse the user ID from the path
	userID := "" // Extract from uri.Path, e.g., using strings.Split
	// Fetch user data...
	userData := map[string]interface{}{
		"id":        userID,
		"email":     fmt.Sprintf("user%s@example.com", userID),
		"lastLogin": time.Now().Format(time.RFC3339),
	}
	return server.JSON(userData) // Helper for JSON responses
}


func registerResources(srv *server.Server) error {
	// Register a static resource
	err := srv.AddResource(protocol.ResourceDefinition{
		URI:         "app://info/version",
		Description: "Get the application version.",
		Handler:     handleAppVersion,
	})
	if err != nil { return err }

	// Register a dynamic resource template
	err = srv.AddResource(protocol.ResourceDefinition{
		URI:         "user://data/{userID}/profile", // Template URI
		Description: "Get user profile data.",
		IsTemplate:  true, // Mark as template
		Handler:     handleUserData,
	})
	return err
}
```

### Prompts

Prompts define reusable templates or interaction patterns for the client (often an LLM). Use `server.AddPrompt`.

```go
package main

import (
	"fmt"
	"net/url"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	// ... other imports
)

// Prompt handler
func handleSummarizePrompt(uri *url.URL, args map[string]interface{}) (protocol.Content, error) {
	text, _ := args["text"].(string) // Basic argument handling
	prompt := fmt.Sprintf("Please summarize the following text concisely:

%s", text)
	return server.Text(prompt), nil
}

func registerPrompts(srv *server.Server) error {
	err := srv.AddPrompt(protocol.PromptDefinition{
		URI:         "prompt://tasks/summarize",
		Description: "Generate a prompt asking the LLM to summarize text.",
		Arguments: &protocol.JSONSchema{ // Define expected arguments
			Type: "object",
			Properties: map[string]*protocol.JSONSchema{
				"text": {Type: "string", Description: "The text to summarize."},
			},
			Required: []string{"text"},
		},
		Handler: handleSummarizePrompt,
	})
	return err
}
```

### Transports

GoMCP separates the core protocol logic from how clients and servers communicate. Supported transports are found in the `transport/` directory:

- `transport/stdio`: Communication over standard input/output.
- `transport/sse`: Streamable HTTP using Server-Sent Events (primary network transport).
- `transport/websocket`: Communication over WebSockets.
- `transport/tcp`: Raw TCP socket communication.

Server-side, use functions like `server.ServeStdio(srv)` or `server.ServeSSE(srv, addr)`. Client-side, use constructors like `client.NewStdioClient(...)` or `client.NewSSEClient(url, ...)`.

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
