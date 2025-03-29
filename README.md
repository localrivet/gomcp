# GoMCP - Go Model Context Protocol Library

[![Go Reference](https://pkg.go.dev/badge/github.com/localrivet/gomcp.svg)](https://pkg.go.dev/github.com/localrivet/gomcp)

<!-- TODO: Add build status badge once CI is set up -->
<!-- TODO: Add code coverage badge once tests are added -->

`gomcp` provides a Go implementation of the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction), enabling communication between language models/agents and external tools or resources via a standardized protocol.

This library facilitates building MCP clients (applications that consume tools/resources) and MCP servers (applications that provide tools/resources). Communication primarily occurs over standard input/output using newline-delimited JSON messages conforming to the JSON-RPC 2.0 specification.

**Current Status:** Alpha - Core Features Implemented (2025-03-26 Spec Alignment)

The library implements the core features of the MCP specification (targeting version 2025-03-26):

- **JSON-RPC 2.0 Transport:** Handles requests, responses, and notifications over stdio (`transport.go`).
- **Protocol Structures:** Defines Go structs for all MCP methods and notifications (`protocol.go`).
- **Initialization:** Full client/server initialization sequence (`client.go`, `server.go`).
- **Tooling:** `tools/list`, `tools/call` methods and handlers.
- **Resources:** `resources/list`, `resources/read` methods and handlers. Infrastructure for `resources/subscribe` and `notifications/resources/changed`.
- **Prompts:** `prompts/list`, `prompts/get` methods and handlers.
- **Logging:** `logging/set_level` method and `notifications/message` infrastructure.
- **Sampling:** `sampling/create_message` method.
- **Roots:** `roots/list` method.
- **Ping:** `ping` method.
- **Cancellation:** Infrastructure for `$/cancelled` notifications and context integration for tool handlers.
- **Progress:** Infrastructure for `$/progress` notifications and `_meta.progressToken`.
- **Notifications:** Infrastructure for `list_changed` notifications (tools, resources, prompts, roots), with dynamic triggering implemented for tool registration.

Full dynamic logic for some notification types (`list_changed` for resources/prompts/roots, `resources/changed`, `progress`) requires further application-level implementation or library helpers.

## Installation

```bash
go get github.com/localrivet/gomcp
```

## Basic Usage

The core logic resides in the root package (`github.com/localrivet/gomcp`).

_(Note: The usage examples below are simplified and may require updates based on the latest library refactoring. The examples in `cmd/` and `examples/` are the most up-to-date reference but also need updating.)_

### Implementing an MCP Server

```go
package main

import (
	"context" // Needed for tool handlers
	"log"
	"os"

	mcp "github.com/localrivet/gomcp"
)

// Example tool handler
func myToolHandler(ctx context.Context, progressToken *mcp.ProgressToken, arguments map[string]interface{}) (content []mcp.Content, isError bool) {
	log.Printf("Executing myTool with args: %v", arguments)
	// Check for cancellation:
	// if ctx.Err() != nil { return nil, true /* or specific error content */ }
	// Report progress (if token provided):
	// if progressToken != nil { server.SendProgress(...) }
	return []mcp.Content{mcp.TextContent{Type: "text", Text: "Tool executed!"}}, false
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting My MCP Server...")

	server := mcp.NewServer("MyGoMCPServer")

	// Register tools
	myTool := mcp.Tool{
		Name:        "my_tool",
		Description: "A simple example tool",
		InputSchema: mcp.ToolInputSchema{Type: "object"}, // Define schema as needed
	}
	err := server.RegisterTool(myTool, myToolHandler)
	if err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}

	// Run the server's main loop (handles initialization and message dispatch)
	err = server.Run()
	if err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server finished.")
}

```

### Implementing an MCP Client

```go
package main

import (
	"log"
	"os"
	"time" // For Ping timeout

	mcp "github.com/localrivet/gomcp"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting My MCP Client...")

	client := mcp.NewClient("MyGoMCPClient")

	// Connect and perform initialization
	err := client.Connect()
	if err != nil {
		log.Fatalf("Client failed to connect: %v", err)
	}
	log.Printf("Client connected successfully to server: %s", client.ServerName())

	// Example: List tools
	listParams := mcp.ListToolsRequestParams{} // Add cursor if needed
	toolsResult, err := client.ListTools(listParams)
	if err != nil {
		log.Printf("Error listing tools: %v", err)
	} else {
		log.Printf("Available tools: %d", len(toolsResult.Tools))
		for _, tool := range toolsResult.Tools {
			log.Printf("  - %s: %s", tool.Name, tool.Description)
		}
	}

	// Example: Call a tool (assuming 'my_tool' exists)
	callParams := mcp.CallToolParams{
		Name:      "my_tool",
		Arguments: map[string]interface{}{"input": "hello"},
		// Meta: &mcp.RequestMeta{ ProgressToken: &token }, // Optional progress
	}
	callResult, err := client.CallTool(callParams, nil) // Pass nil for no progress token
	if err != nil {
		log.Printf("Error calling tool 'my_tool': %v", err)
	} else {
		log.Printf("Tool 'my_tool' result: %+v", callResult)
	}

	// Example: Ping the server
	err = client.Ping(5 * time.Second)
	if err != nil {
		log.Printf("Ping failed: %v", err)
	} else {
		log.Println("Ping successful!")
	}

	// Close the client connection when done (optional for stdio)
	err = client.Close()
	if err != nil {
		log.Printf("Error closing client: %v", err)
	}

	log.Println("Client finished.")
}
```

## Example Executables

The `cmd/` directory contains simple example programs demonstrating the use of the library:

- `cmd/mcp-server`: A basic server that performs initialization. _(Needs update)_
- `cmd/mcp-client`: A basic client that performs initialization. _(Needs update)_

You can test the initialization sequence by running them connected via a pipe:

```bash
go run ./cmd/mcp-server/main.go | go run ./cmd/mcp-client/main.go
```

_(Check the log output on stderr for details)_

The `examples/` directory contains more elaborate client/server pairs demonstrating specific features like tool usage, but these also **require updates** to align with the latest library changes.

## Documentation

More detailed documentation can be found in the [GitHub Pages site](https://gomcp.dev) (powered by the `/docs` directory). _(Needs update)_

Go package documentation is available via:

- [pkg.go.dev](https://pkg.go.dev/github.com/localrivet/gomcp)
- Running `godoc -http=:6060` locally and navigating to `github.com/localrivet/gomcp`.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the [MIT License](LICENSE).
