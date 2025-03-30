# GoMCP - Go Model Context Protocol Library

[![Go Reference](https://pkg.go.dev/badge/github.com/localrivet/gomcp.svg)](https://pkg.go.dev/github.com/localrivet/gomcp)

<!-- TODO: Add build status badge once CI is set up -->
<!-- TODO: Add code coverage badge once tests are added -->

`gomcp` provides a Go implementation of the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction), enabling communication between language models/agents and external tools or resources via a standardized protocol.

This library facilitates building MCP clients (applications that consume tools/resources) and MCP servers (applications that provide tools/resources). Communication primarily occurs over standard input/output using newline-delimited JSON messages conforming to the JSON-RPC 2.0 specification, although other transports (like SSE) are supported.

**Current Status:** Alpha - Compliant with MCP Specification v2025-03-26

The core library implements the features defined in the [MCP Specification version 2025-03-26](https://modelcontextprotocol.io/specification/2025-03-26/):

- **Transport Agnostic Core:** Server (`server/`) and Client (`client/`) logic is separated from the transport layer.
- **Transports:** Implementations for Stdio (`transport/stdio/`) and SSE (`transport/sse/`) are provided.
- **Protocol Structures:** Defines Go structs for all specified MCP methods, notifications, and content types (`protocol/`).
- **Initialization:** Full client/server initialization sequence, including capability exchange.
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

The core logic resides in the `server`, `client`, and `protocol` packages.

_(Note: The usage examples below are simplified. See the `examples/` directory for more complete implementations.)_

### Implementing an MCP Server (using Stdio)

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal" // For graceful shutdown

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types" // For logger
)

// Example tool handler
func myToolHandler(ctx context.Context, progressToken *protocol.ProgressToken, arguments map[string]interface{}) (content []protocol.Content, isError bool) {
	log.Printf("Executing myTool with args: %v", arguments)
	// ... tool logic ...
	return []protocol.Content{protocol.TextContent{Type: "text", Text: "Tool executed!"}}, false
}

// Simple server loop for stdio
func runServerLoop(ctx context.Context, srv *server.Server, transport types.Transport) error {
	// Stdio typically represents a single "session"
	session := server.NewStdioSession("stdio-session") // Use helper if available, or mock
	if err := srv.RegisterSession(session); err != nil {
		return fmt.Errorf("failed to register session: %w", err)
	}
	defer srv.UnregisterSession(session.SessionID())

	log.Println("Server listening on stdio...")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			rawMsg, err := transport.ReceiveWithContext(ctx) // Use context-aware receive
			if err != nil {
				// Handle EOF, pipe closed, context canceled as clean exit
				if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "pipe closed") || errors.Is(err, context.Canceled) {
					log.Printf("Input closed or context cancelled.")
					return nil
				}
				return fmt.Errorf("transport receive error: %w", err)
			}

			response := srv.HandleMessage(ctx, session.SessionID(), rawMsg)

			// For stdio, HandleMessage might return response directly OR send via session.
			// This example assumes direct return or ignores session send for simplicity.
			// A real implementation might need the mockSession pattern from tests if session.SendResponse is used.
			if response != nil {
				respBytes, err := json.Marshal(response)
				if err != nil {
					log.Printf("ERROR: server failed to marshal response: %v", err)
					continue
				}
				if err := transport.Send(respBytes); err != nil {
					// Handle EOF/pipe closed during send
					if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "pipe closed") {
						log.Printf("Output closed during send.")
						return nil
					}
					return fmt.Errorf("transport send error: %w", err)
				}
			}
		}
	}
}


func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting My MCP Server...")

	// Create server core
	srv := server.NewServer("MyGoMCPServer", server.ServerOptions{
		// Logger: provide custom logger if needed
	})

	// Register tools
	myTool := protocol.Tool{
		Name:        "my_tool",
		Description: "A simple example tool",
		InputSchema: protocol.ToolInputSchema{Type: "object"}, // Define schema as needed
	}
	err := srv.RegisterTool(myTool, myToolHandler)
	if err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}

	// Create stdio transport
	transport := stdio.NewStdioTransport()

	// Run the server's message handling loop
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err = runServerLoop(ctx, srv, transport) // Pass context, server, transport
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("Server loop exited with error: %v", err)
	}

	log.Println("Server finished.")
}

// Note: Need a simple StdioSession implementation for runServerLoop
type stdioSession struct { id string }
func NewStdioSession(id string) *stdioSession { return &stdioSession{id: id} }
func (s *stdioSession) SessionID() string { return s.id }
func (s *stdioSession) SendNotification(notification protocol.JSONRPCNotification) error { return fmt.Errorf("stdio transport does not support server-to-client notifications") }
func (s *stdioSession) SendResponse(response protocol.JSONRPCResponse) error { return fmt.Errorf("stdio transport does not support async server-to-client responses via session") }
func (s *stdioSession) Close() error { return nil }
func (s *stdioSession) Initialize() {}
func (s *stdioSession) Initialized() bool { return true } // Assume initialized for stdio simplicity
var _ server.ClientSession = (*stdioSession)(nil)

// Need these imports for the example:
import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

```

### Implementing an MCP Client (using SSE)

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"   // Use client package
	"github.com/localrivet/gomcp/protocol" // Use protocol package
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting My MCP Client...")

	// Create client instance, providing server URL
	clt, err := client.NewClient("MyGoMCPClient", client.ClientOptions{
		ServerBaseURL: "http://127.0.0.1:8080", // Adjust if server runs elsewhere
		// Logger: provide custom logger if needed
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Connect and perform initialization (use context for timeout/cancellation)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = clt.Connect(ctx)
	if err != nil {
		log.Fatalf("Client failed to connect: %v", err)
	}
	defer clt.Close() // Ensure connection is closed eventually

	serverInfo := clt.ServerInfo() // Get server info after connection
	log.Printf("Client connected successfully to server: %s (Version: %s)", serverInfo.Name, serverInfo.Version)

	// Example: List tools
	listParams := protocol.ListToolsRequestParams{}
	toolsResult, err := clt.ListTools(ctx, listParams) // Pass context
	if err != nil {
		log.Printf("Error listing tools: %v", err)
	} else {
		log.Printf("Available tools: %d", len(toolsResult.Tools))
		for _, tool := range toolsResult.Tools {
			log.Printf("  - %s: %s", tool.Name, tool.Description)
		}
	}

	// Example: Call a tool (assuming 'my_tool' exists)
	callParams := protocol.CallToolParams{
		Name:      "my_tool",
		Arguments: map[string]interface{}{"input": "hello"},
	}
	callResult, err := clt.CallTool(ctx, callParams, nil) // Pass context, nil progress token
	if err != nil {
		log.Printf("Error calling tool 'my_tool': %v", err)
	} else {
		log.Printf("Tool 'my_tool' result: %+v", callResult)
	}

	// Ping is handled via standard request/response, no special client method needed

	log.Println("Client finished.")
}
```

## Example Executables

The `examples/` directory contains more elaborate client/server pairs demonstrating specific features like authentication, rate limiting, different transports (SSE), and various tool implementations. These provide a more comprehensive demonstration of the library's capabilities.

To run the basic stdio server/client example:

1.  Build the server: `go build -o mcp_server ./examples/server/`
2.  Build the client: `go build -o mcp_client ./examples/client/`
3.  Run them connected via a pipe: `./mcp_server | ./mcp_client`

_(Check the log output on stderr for details)_

To run the SSE example:

1.  Start the server: `go run ./examples/sse-server/main.go` (or build and run)
2.  In another terminal, start the client: `go run ./examples/sse-client/main.go` (or build and run)

## Documentation

More detailed documentation can be found in the [GitHub Pages site](https://gomcp.dev) (powered by the `/docs` directory). _(Needs update)_

Go package documentation is available via:

- [pkg.go.dev](https://pkg.go.dev/github.com/localrivet/gomcp)
- Running `godoc -http=:6060` locally and navigating to `github.com/localrivet/gomcp`.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the [MIT License](LICENSE).
