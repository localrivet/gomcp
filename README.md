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

## Key Features

- **Protocol Version Negotiation:** Automatically negotiates the highest compatible protocol version (`2025-03-26` or `2024-11-05`) during the client-server handshake.
- **Transport Agnostic Core:** Server (`server/`) and Client (`client/`) logic is decoupled from the underlying transport mechanism.
- **Multiple Transports:** Includes implementations for common communication methods:
  - **Streamable HTTP (SSE + POST):** The primary network transport, compliant with the `2025-03-26` specification (and backward compatible with `2024-11-05`'s HTTP+SSE). See `transport/sse/`.
  - **WebSocket:** A common alternative for persistent, bidirectional network communication. See `transport/websocket/`.
  - **Standard Input/Output (Stdio):** Ideal for local inter-process communication. See `transport/stdio/`.
  - **TCP:** For raw TCP socket communication (less common for MCP). See `transport/tcp/`.
- **Full MCP Feature Support:** Implements all core MCP features:
  - Tools (`tools/list`, `tools/call`)
  - Resources (`resources/list`, `resources/read`, `resources/subscribe`, `resources/unsubscribe`)
  - Prompts (`prompts/list`, `prompts/get`)
  - Sampling (`sampling/createMessage`)
  - Logging (`logging/setLevel`, `$/logging/message`)
  - Progress Reporting (`$/progress`)
  - Cancellation (`$/cancelRequest`)
  - Client Roots (`roots/set`)
  - Notifications (`$/listChanged`, `$/resourceChanged`, etc.)
  - Ping (`ping`)
- **`2025-03-26` Enhancements:** Includes support for features added in the latest spec:
  - Authorization Framework (OAuth 2.1 based via Hooks)
  - JSON-RPC Batching
  - Tool Annotations (`readOnlyHint`, `destructiveHint`, etc.)
  - Audio Content Type
  - Completions Capability
- **Helper Utilities:** Provides helpers for argument parsing (`util/schema`), progress reporting (`util/progress`), and more.
- **Extensible Hooks:** Offers a comprehensive hook system (`hooks/`) for intercepting and modifying behavior at various points in the server/client lifecycle.

## Installation

```bash
go get github.com/localrivet/gomcp
```

## Quick Start: Stdio Server & Client

This example demonstrates a simple server exposing a "hello" tool and a client calling it, both using the Stdio transport.

**Server (`stdio_server/main.go`)**

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	// "github.com/localrivet/gomcp/util/schema" // Not typically needed with AddTool helper
)

// Arguments for the 'hello' tool
type HelloArgs struct {
	Name string `json:"name" description:"Name to greet" required:"true"`
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Hello Demo MCP Server (Stdio)...")

	// Create the MCP server instance
	srv := server.NewServer("hello-demo-stdio")

	// Add the 'hello' tool using the server.AddTool helper
	// The helper infers the schema from the handler's argument type (HelloArgs)
	// and handles argument parsing internally.
	err := server.AddTool(
		srv,
		"hello",
		"Return a friendly greeting.",
		// Simple handler function directly using the args struct
		func(args HelloArgs) (protocol.Content, error) {
			greeting := fmt.Sprintf("Hello, %s!", args.Name)
			log.Printf("[hello tool] -> %q", greeting) // Optional logging
			// Use server.Text helper for simple text responses
			return server.Text(greeting), nil
		},
	)
	if err != nil {
		log.Fatalf("Failed to add hello tool: %v", err)
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

**Client (`stdio_client/main.go`)**

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
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Simple MCP Client (Stdio)...")

	// Create a client configured for stdio communication
	clt, err := client.NewStdioClient("MySimpleClient", client.ClientOptions{})
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
	// log.Printf("Negotiated Protocol Version: %s", clt.NegotiatedVersion()) // Assuming NegotiatedVersion() exists

	// Call the 'hello' tool
	log.Println("\n--- Calling 'hello' Tool ---")
	helloArgs := map[string]interface{}{"name": "GoMCP User"}
	callParams := protocol.CallToolParams{Name: "hello", Arguments: helloArgs}

	callCtx, callCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer callCancel()

	callResult, err := clt.CallTool(callCtx, callParams, nil) // No progress token needed
	if err != nil {
		log.Printf("Error calling tool 'hello': %v", err)
	} else if callResult.IsError {
		log.Printf("Tool 'hello' call returned an error:")
		// Process error content
		for _, content := range callResult.Content {
			if textContent, ok := content.(protocol.TextContent); ok {
				log.Printf("  Error Content: %s", textContent.Text)
			}
		}
	} else {
		log.Printf("Tool 'hello' call successful:")
		for _, content := range callResult.Content {
			if textContent, ok := content.(protocol.TextContent); ok {
				log.Printf("  Result: %s", textContent.Text)
			}
		}
	}

	log.Println("\nClient finished.")
}
```

## Examples

The `examples/` directory contains various client/server pairs demonstrating specific features, transports, and integrations:

- **`basic/`**: Simple stdio communication with multiple tools.
- **`http/`**: Integration with various Go HTTP frameworks using the **Streamable HTTP (SSE)** transport.
- **`websocket/`**: Demonstrates the **WebSocket** transport.
- **`configuration/`**: Loading server configuration from files (JSON, YAML, TOML).
- **`auth/`**: Simple API key authentication hook example (stdio).
- **`kitchen-sink/`**: Comprehensive server example combining multiple features (stdio).
- **`cmd/`**: Generic command-line client and server implementations configurable for different transports.
- **`hello-demo/`**: Minimal example showcasing tool, prompt, and resource registration.
- **`code-assistant/`**: Example server providing code review/documentation tools.

**Running Examples:**

1.  Navigate to an example's server directory (e.g., `cd examples/websocket/server`).
2.  Run the server: `go run .`
3.  In another terminal, navigate to the corresponding client directory (e.g., `cd examples/websocket/client`).
4.  Run the client: `go run .`

_(Check the specific README within each example directory for more detailed instructions if available.)_

## Documentation

- **Go Packages:** [pkg.go.dev/github.com/localrivet/gomcp](https://pkg.go.dev/github.com/localrivet/gomcp)
- **MCP Specification:** [modelcontextprotocol.io](https://modelcontextprotocol.io)

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the [MIT License](LICENSE).
