# GoMCP - Go Model Context Protocol Library

[![Go Reference](https://pkg.go.dev/badge/github.com/localrivet/gomcp.svg)](https://pkg.go.dev/github.com/localrivet/gomcp)

<!-- TODO: Add build status badge once CI is set up -->
<!-- TODO: Add code coverage badge once tests are added -->

`gomcp` provides a Go implementation of the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction), enabling communication between language models/agents and external tools or resources via a standardized protocol.

This library facilitates building MCP clients (applications that consume tools/resources) and MCP servers (applications that provide tools/resources). Communication primarily occurs over standard input/output using newline-delimited JSON messages conforming to the JSON-RPC 2.0 specification, although other transports (like SSE, WebSocket, TCP) are supported. The library supports negotiation between different MCP specification versions.

**Current Status:** Compliant with MCP Specification v2025-03-26 and v2024-11-05

The core library implements the features defined in the [MCP Specification versions 2025-03-26](https://modelcontextprotocol.io/specification/2025-03-26/) and [2024-11-05](https://modelcontextprotocol.io/specification/2024-11-05/):

- **Transport Agnostic Core:** Server (`server/`) and Client (`client/`) logic is separated from the transport layer.
- **Protocol Version Negotiation:** Client and Server negotiate the protocol version during initialization, supporting both `2025-03-26` and `2024-11-05`.
- **Transports:** Implementations for Stdio (`transport/stdio/`), SSE (`transport/sse/`), and WebSocket (`transport/websocket/`) are provided.
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
	"encoding/json" // Added
	"errors"        // Added
	"fmt"           // Added
	"io"            // Added
	"log"
	"os"
	"os/signal" // For graceful shutdown
	"strings"   // Added

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types" // For logger
	"github.com/localrivet/gomcp/util/schema" // Added for schema helpers
)

// --- Tool 1: Simple Tool ---
// Example tool handler
func myToolHandler(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	log.Printf("Executing myTool with args: %v", arguments)
	// ... tool logic ...
	return []protocol.Content{protocol.TextContent{Type: "text", Text: "Tool executed!"}}, false
}

// --- Tool 2: Add Tool using Schema Helpers ---
// Define arguments struct for the add tool
type AddArgs struct {
	Num1 int `json:"num1" description:"The first number to add"`
	Num2 int `json:"num2" description:"The second number to add"`
	// Optional fields use pointers
	Comment *string `json:"comment,omitempty" description:"An optional comment"`
}

// Handler for the add tool using schema.HandleArgs
func addToolHandler(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	args, errContent, isErr := schema.HandleArgs[AddArgs](arguments)
	if isErr {
		log.Printf("Error handling add args: %v", errContent)
		return errContent, true
	}

	log.Printf("Executing add tool with args: %+v", args)
	sum := args.Num1 + args.Num2
	resultText := fmt.Sprintf("The sum of %d and %d is %d.", args.Num1, args.Num2)
	if args.Comment != nil {
		resultText += fmt.Sprintf(" Comment: %s", *args.Comment)
	}

	return []protocol.Content{protocol.TextContent{Type: "text", Text: resultText}}, false
}


// --- Server Setup and Loop ---
// Simple server loop for stdio
func runServerLoop(ctx context.Context, srv *server.Server, transport types.Transport) error {
	// Stdio typically represents a single "session"
	// For stdio, we need a simple ClientSession implementation.
	session := NewStdioSession("stdio-session") // Use local mock below
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

			// HandleMessage now returns a slice of responses
			responses := srv.HandleMessage(ctx, session.SessionID(), rawMsg)

			// Send back any responses generated
			if responses != nil && len(responses) > 0 {
				for _, responseToSend := range responses {
					if responseToSend == nil {
						continue
					}
					respBytes, err := json.Marshal(responseToSend)
					if err != nil {
						log.Printf("ERROR: server failed to marshal response: %v", err)
						continue // Skip this response
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
		InputSchema: protocol.ToolInputSchema{Type: "object"}, // Manually defined schema
	}
	err := srv.RegisterTool(myTool, myToolHandler)
	if err != nil {
		log.Fatalf("Failed to register 'my_tool': %v", err)
	}

	// Register add tool using schema helpers
	addTool := protocol.Tool{
		Name:        "add",
		Description: "Adds two numbers together.",
		InputSchema: schema.FromStruct(AddArgs{}), // Generate schema from struct
	}
	err = srv.RegisterTool(addTool, addToolHandler)
	if err != nil {
		log.Fatalf("Failed to register 'add' tool: %v", err)
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

// Note: Need a simple StdioSession implementation for runServerLoop that satisfies the interface
type stdioSession struct {
	id                string
	initialized       bool
	negotiatedVersion string
	clientCaps        protocol.ClientCapabilities // Added
}

func NewStdioSession(id string) *stdioSession { return &stdioSession{id: id} }
func (s *stdioSession) SessionID() string      { return s.id }
func (s *stdioSession) SendNotification(notification protocol.JSONRPCNotification) error {
	// Stdio typically cannot send async notifications back to the client process easily.
	return fmt.Errorf("stdio transport does not support server-to-client notifications via session")
}
func (s *stdioSession) SendResponse(response protocol.JSONRPCResponse) error {
	// Responses for stdio are handled directly by HandleMessage return value in this example.
	return fmt.Errorf("stdio transport does not support async server-to-client responses via session")
}
func (s *stdioSession) Close() error                        { return nil } // Stdio streams managed by OS pipes
func (s *stdioSession) Initialize()                         { s.initialized = true }
func (s *stdioSession) Initialized() bool                   { return s.initialized }
func (s *stdioSession) SetNegotiatedVersion(version string) { s.negotiatedVersion = version }
func (s *stdioSession) GetNegotiatedVersion() string        { return s.negotiatedVersion }
func (s *stdioSession) StoreClientCapabilities(caps protocol.ClientCapabilities) { s.clientCaps = caps } // Added
func (s *stdioSession) GetClientCapabilities() protocol.ClientCapabilities { return s.clientCaps } // Added

var _ server.ClientSession = (*stdioSession)(nil)

// Imports moved to the top

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

## Examples

The `examples/` directory contains various client/server pairs demonstrating specific features and transports. Each example is a self-contained Go module.

**Running Examples:**

Most examples follow a similar pattern. To run an example:

1.  Navigate to the example's directory (e.g., `cd examples/basic/server`).
2.  Run the server using `go run .`.
3.  In another terminal, navigate to the corresponding client directory (e.g., `cd examples/basic/client`).
4.  Run the client using `go run .`.

**Example Categories:**

- **`examples/basic/`**: Demonstrates simple stdio communication.
- **`examples/http/`**: Shows integration with various Go HTTP frameworks/routers (Chi, Echo, Fiber, Gin, Go-Zero, Gorilla/Mux, HttpRouter, Beego, Iris, Net/HTTP) using the SSE transport. Run the server from `examples/http/<framework>/server/` and use a generic SSE client (like the one in `examples/cmd/gomcp-client` configured for SSE) or a browser-based client.
- **`examples/websocket/`**: Demonstrates the WebSocket transport. Run the server from `examples/websocket/server/` and use a generic WebSocket client (like `examples/cmd/gomcp-client` configured for WebSocket).
- **`examples/configuration/`**: Shows how to load server configuration from JSON, YAML, or TOML files. Run the specific server (e.g., `cd examples/configuration/json/server && go run .`) which loads the corresponding config file (e.g., `examples/configuration/json/config.json`).
- **`examples/auth/`**: Example demonstrating authentication concepts (details TBD).
- **`examples/billing/`**: Example demonstrating billing/quota concepts (details TBD).
- **`examples/rate-limit/`**: Example demonstrating rate limiting (details TBD).
- **`examples/kitchen-sink/`**: A more complex example combining multiple features (details TBD).
- **`examples/cmd/`**: Contains generic command-line client and server implementations that can be configured for different transports.

_(Check the specific README within each example directory for more detailed instructions if available.)_

## Documentation

More detailed documentation can be found in the [GitHub Pages site](https://gomcp.dev) (powered by the `/docs` directory). _(Needs update)_

Go package documentation is available via:

- [pkg.go.dev](https://pkg.go.dev/github.com/localrivet/gomcp)
- Running `godoc -http=:6060` locally and navigating to `github.com/localrivet/gomcp`.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the [MIT License](LICENSE).
