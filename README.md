# GoMCP - Go Model Context Protocol Library

[![Go Reference](https://pkg.go.dev/badge/github.com/localrivet/gomcp.svg)](https://pkg.go.dev/github.com/localrivet/gomcp)

<!-- TODO: Add build status badge once CI is set up -->
<!-- TODO: Add code coverage badge once tests are added -->

`gomcp` provides a Go implementation of the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction), enabling communication between language models/agents and external tools or resources via a standardized protocol.

This library facilitates building MCP clients (applications that consume tools/resources) and MCP servers (applications that provide tools/resources). Communication primarily occurs over standard input/output using newline-delimited JSON messages conforming to the JSON-RPC 2.0 specification.

**Current Status:** Alpha - Compliant with MCP Specification v2025-03-26

The core library (`gomcp` package) implements the features defined in the [MCP Specification version 2025-03-26](https://modelcontextprotocol.io/specification/2025-03-26/):

- **JSON-RPC 2.0 Transport:** Handles requests, responses, and notifications over stdio (`transport.go`).
- **Protocol Structures:** Defines Go structs for all specified MCP methods, notifications, and content types (`protocol.go`).
- **Initialization:** Full client/server initialization sequence (`client.go`, `server.go`), including capability exchange.
- **Tooling:** `tools/list`, `tools/call` methods and handlers.
- **Resources:** `resources/list`, `resources/read`, `resources/subscribe`, `resources/unsubscribe` methods and handlers.
- **Prompts:** `prompts/list`, `prompts/get` methods and handlers.
- **Logging:** `logging/set_level` method and `notifications/message` infrastructure.
- **Sampling:** `sampling/create_message` method.
- **Roots:** `roots/list` method and client-side root management.
- **Ping:** `ping` method.
- **Cancellation:** `$/cancelled` notification handling with `context.Context` integration.
- **Progress:** `$/progress` notification infrastructure and `_meta.progressToken` support.
- **Notifications:** Dynamic triggering for `list_changed` (tools, resources, prompts, roots) and `resources/changed` notifications based on library actions and subscriptions.

_(Note: While the library provides the mechanisms, the specific logic within server-side handlers like `handleReadResource`, `handleGetPrompt`, `handleLoggingSetLevel`, and triggering `NotifyResourceUpdated` is application-dependent.)_

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

	"github.com/localrivet/gomcp"
)

// Example tool handler
func myToolHandler(ctx context.Context, progressToken *gomcp.ProgressToken, arguments map[string]interface{}) (content []gomcp.Content, isError bool) {
	log.Printf("Executing myTool with args: %v", arguments)
	// Check for cancellation:
	// if ctx.Err() != nil { return nil, true /* or specific error content */ }
	// Report progress (if token provided):
	// if progressToken != nil { server.SendProgress(...) }
	return []gomcp.Content{gomcp.TextContent{Type: "text", Text: "Tool executed!"}}, false
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting My MCP Server...")

	server := gomcp.NewServer("MyGoMCPServer")

	// Register tools
	myTool := gomcp.Tool{
		Name:        "my_tool",
		Description: "A simple example tool",
		InputSchema: gomcp.ToolInputSchema{Type: "object"}, // Define schema as needed
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

	"github.com/localrivet/gomcp"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting My MCP Client...")

	client := gomcp.NewClient("MyGoMCPClient")

	// Connect and perform initialization
	err := client.Connect()
	if err != nil {
		log.Fatalf("Client failed to connect: %v", err)
	}
	log.Printf("Client connected successfully to server: %s", client.ServerName())

	// Example: List tools
	listParams := gomcp.ListToolsRequestParams{} // Add cursor if needed
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
	callParams := gomcp.CallToolParams{
		Name:      "my_tool",
		Arguments: map[string]interface{}{"input": "hello"},
		// Meta: &gomcp.RequestMeta{ ProgressToken: &token }, // Optional progress
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

The `examples/` directory contains elaborate client/server pairs demonstrating specific features like tool usage. These examples provide a more comprehensive demonstration of the library's capabilities.

**Note:** The simple examples in the `cmd/` directory (`cmd/mcp-server` and `cmd/mcp-client`) are basic demonstrations that only perform initialization. These may be moved to the `examples/` directory or removed in future updates, as the examples in the `examples/` directory provide more comprehensive demonstrations.

If you want to test a basic initialization sequence, you can run:

```bash
go run ./cmd/mcp-server/main.go | go run ./cmd/mcp-client/main.go
```

_(Check the log output on stderr for details)_

## Documentation

More detailed documentation can be found in the [GitHub Pages site](https://gomcp.dev) (powered by the `/docs` directory). _(Needs update)_

Go package documentation is available via:

- [pkg.go.dev](https://pkg.go.dev/github.com/localrivet/gomcp)
- Running `godoc -http=:6060` locally and navigating to `github.com/localrivet/gomcp`.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the [MIT License](LICENSE).
