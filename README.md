# GoMCP - Go Model Context Protocol Library

[![Go Reference](https://pkg.go.dev/badge/github.com/localrivet/gomcp.svg)](https://pkg.go.dev/github.com/localrivet/gomcp)

<!-- TODO: Add build status badge once CI is set up -->
<!-- TODO: Add code coverage badge once tests are added -->

`gomcp` provides a Go implementation of the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction), enabling communication between language models/agents and external tools or resources via a standardized protocol.

This library facilitates building MCP clients (applications that consume tools/resources) and MCP servers (applications that provide tools/resources). It supports multiple transport mechanisms:

- **Standard Input/Output (Stdio):** For local process communication.
- **HTTP+SSE:** The primary network transport for `2024-11-05` specification compatibility.
- **WebSocket:** The primary network transport for `2025-03-26` specification compatibility.
- **TCP:** For raw socket communication.
  The library supports negotiation between different MCP specification versions (`2024-11-05` and `2025-03-26`). Communication uses newline-delimited JSON messages conforming to the JSON-RPC 2.0 specification.

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

_(Note: The usage examples below use the **Stdio** transport for simplicity. For examples demonstrating network communication using **HTTP+SSE** or **WebSocket**, please see the `examples/` directory.)_

### Implementing an MCP Server (using Stdio)

```go
package main

import (
	"fmt"
	"log"
	"os" // Added for log output configuration

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// Define arguments struct for the hello tool
type HelloArgs struct {
	Name string `json:"name" description:"Name to greet" required:"true"`
}

func main() {
	// Configure logger (optional, defaults to stderr)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Hello Demo MCP Server...")

	// Create the MCP server instance
	srv := server.NewServer("hello-demo")

	// Add the 'hello' tool
	err := server.AddTool(
		srv,
		"hello",
		"Return a friendly greeting for the supplied name.",
		// Simple handler function directly using the args struct
		func(args HelloArgs) (protocol.Content, error) {
			greeting := fmt.Sprintf("Hello, %s!", args.Name)
			log.Printf("[hello] -> %q", greeting) // Optional logging
			// Use server.Text helper for simple text responses
			return server.Text(greeting), nil
		},
	)
	if err != nil {
		log.Fatalf("Failed to add hello tool: %v", err)
	}

	// Start the server using the built-in stdio handler.
	// This blocks until the server exits.
	log.Println("Server setup complete. Listening on stdio...")
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server shutdown complete.")
}

```

### Implementing an MCP Client (using Stdio)

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
)

func main() {
	// Configure logger (optional, defaults to stderr)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Simple MCP Client...")

	// Create a client configured for stdio communication
	// NewStdioClient handles the transport setup internally.
	clt, err := client.NewStdioClient("MySimpleClient", client.ClientOptions{
		// Logger: provide custom logger if needed
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Set a timeout for the connection and operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect and perform initialization handshake
	log.Println("Connecting to server via stdio...")
	err = clt.Connect(ctx)
	if err != nil {
		log.Fatalf("Client failed to connect: %v", err)
	}
	defer clt.Close() // Ensure connection resources are cleaned up

	serverInfo := clt.ServerInfo()
	log.Printf("Connected to server: %s (Version: %s)", serverInfo.Name, serverInfo.Version)

	// Example: List available tools
	log.Println("\n--- Listing Tools ---")
	listParams := protocol.ListToolsRequestParams{}
	toolsResult, err := clt.ListTools(ctx, listParams)
	if err != nil {
		log.Printf("Error listing tools: %v", err)
	} else {
		log.Printf("Available tools (%d):", len(toolsResult.Tools))
		for _, tool := range toolsResult.Tools {
			log.Printf("  - %s: %s", tool.Name, tool.Description)
		}
	}

	// Example: Call the 'add' tool (assuming server has it registered)
	log.Println("\n--- Calling 'add' Tool ---")
	addArgs := map[string]interface{}{
		"num1":    15,
		"num2":    27,
		"comment": "Example call",
	}
	callParams := protocol.CallToolParams{
		Name:      "add",
		Arguments: addArgs,
	}
	callResult, err := clt.CallTool(ctx, callParams, nil) // No progress token needed
	if err != nil {
		log.Printf("Error calling tool 'add': %v", err)
	} else {
		log.Printf("Tool 'add' call successful (IsError=%v):", callResult.IsError)
		for i, content := range callResult.Content {
			// Log content (could be text, json, image, etc.)
			log.Printf("  Content[%d]: Type=%s", i, content.ContentType())
			if textContent, ok := content.(protocol.TextContent); ok {
				log.Printf("    Text: %s", textContent.Text)
			}
			// Add checks for other content types if needed
		}
	}

	log.Println("\nClient finished.")
}

```

### Example: Dual Transport Server (SSE & WebSocket) + WebSocket Client

This example demonstrates how to run a single MCP server instance that listens for connections using both the HTTP+SSE transport (for `2024-11-05` compatibility) and the WebSocket transport (for `2025-03-26` compatibility). It also shows a client connecting via WebSocket to call a tool.

_(Note: This requires the transport packages to expose `http.Handler` implementations or similar integration points. The exact handler creation (`sse.NewSSEHandler`, `websocket.NewWebSocketHandler`) is hypothetical and might differ in the actual library implementation.)_

**Server (`dual_server/main.go` - Conceptual)**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/util/schema"
	// Import transport packages to get handlers
	"github.com/localrivet/gomcp/transport/sse"
	"github.com/localrivet/gomcp/transport/websocket"
)

// Define arguments struct for the add tool
type AddArgs struct {
	Num1 int `json:"num1" description:"The first number to add"`
	Num2 int `json:"num2" description:"The second number to add"`
}

// Handler for the add tool
func addToolHandler(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	args, errContent, isErr := schema.HandleArgs[AddArgs](arguments)
	if isErr {
		log.Printf("Error handling add args: %v", errContent)
		return errContent, true
	}
	log.Printf("Executing add tool with args: %+v", args)
	sum := args.Num1 + args.Num2
	resultText := fmt.Sprintf("The sum is %d.", sum)
	return []protocol.Content{protocol.TextContent{Type: "text", Text: resultText}}, false
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Dual Transport MCP Server...")

	// Create the core server instance
	srv := server.NewServer("MyDualServer")

	// Define and register the 'add' tool
	addTool := protocol.Tool{
		Name:        "add",
		Description: "Adds two numbers.",
		InputSchema: schema.FromStruct(AddArgs{}),
	}
	if err := srv.RegisterTool(addTool, addToolHandler); err != nil {
		log.Fatalf("Failed to register 'add' tool: %v", err)
	}

	// --- Setup HTTP Server for Both Transports ---
	mux := http.NewServeMux()

	// Create and register the SSE handler (assuming a function like this exists)
	sseBasePath := "/mcp-sse" // Use a distinct path for SSE
	sseHandler, err := sse.NewSSEHandler(srv, sseBasePath) // Hypothetical constructor
	if err != nil {
		log.Fatalf("Failed to create SSE handler: %v", err)
	}
	mux.Handle(sseBasePath, sseHandler)
	log.Printf("SSE transport configured at path: %s", sseBasePath)

	// Create and register the WebSocket handler (assuming a function like this exists)
	wsPath := "/mcp-ws" // Use a distinct path for WebSocket
	wsHandler, err := websocket.NewWebSocketHandler(srv, wsPath) // Hypothetical constructor
	if err != nil {
		log.Fatalf("Failed to create WebSocket handler: %v", err)
	}
	mux.Handle(wsPath, wsHandler)
	log.Printf("WebSocket transport configured at path: %s", wsPath)

	// Configure the HTTP server
	httpServer := &http.Server{
		Addr:    ":8080", // Listen on port 8080
		Handler: mux,
	}

	// Start the HTTP server in a goroutine
	go func() {
		log.Printf("HTTP server listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe error: %v", err)
		}
	}()

	// --- Graceful Shutdown Handling ---
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop // Wait for interrupt signal

	log.Println("Shutting down server...")

	// Create a deadline context for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	} else {
		log.Println("HTTP server gracefully stopped.")
	}

	log.Println("Server shutdown complete.")
}
```

**Client (`ws_client/main.go` - Conceptual)**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting WebSocket MCP Client...")

	// WebSocket URL for the dual server example
	wsURL := "ws://localhost:8080/mcp-ws"

	// Create a WebSocket client instance
	// Specify the preferred protocol version if needed (defaults to older)
	preferredVersion := protocol.CurrentProtocolVersion // Prefer 2025-03-26
	clt, err := client.NewWebSocketClient(wsURL, client.ClientOptions{
		PreferredProtocolVersion: &preferredVersion,
	})
	if err != nil {
		log.Fatalf("Failed to create WebSocket client: %v", err)
	}

	// Connect and initialize
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Printf("Connecting to server via WebSocket: %s", wsURL)
	err = clt.Connect(ctx)
	if err != nil {
		log.Fatalf("Client failed to connect: %v", err)
	}
	defer clt.Close()

	// Assuming NegotiatedVersion() method exists on client
	// serverInfo := clt.ServerInfo()
	// log.Printf("Connected to server: %s (Version: %s), Negotiated Protocol: %s",
	// 	serverInfo.Name, serverInfo.Version, clt.NegotiatedVersion())

	// Call the 'add' tool
	log.Println("\n--- Calling 'add' Tool via WebSocket ---")
	addArgs := map[string]interface{}{"num1": 50, "num2": 33}
	callParams := protocol.CallToolParams{Name: "add", Arguments: addArgs}

	callCtx, callCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer callCancel()

	callResult, err := clt.CallTool(callCtx, callParams, nil)
	if err != nil {
		log.Printf("Error calling tool 'add': %v", err)
	} else {
		log.Printf("Tool 'add' call successful:")
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

The `examples/` directory contains various client/server pairs demonstrating specific features and transports. Each example is a self-contained Go module.

**Running Examples:**

Most examples follow a similar pattern. To run an example:

1.  Navigate to the example's directory (e.g., `cd examples/basic/server`).
2.  Run the server using `go run .`.
3.  In another terminal, navigate to the corresponding client directory (e.g., `cd examples/basic/client`).
4.  Run the client using `go run .`.

**Example Categories:**

- **`examples/basic/`**: Demonstrates simple stdio communication.
- **`examples/http/`**: Shows integration with various Go HTTP frameworks/routers using the **HTTP+SSE transport** (compatible with the `2024-11-05` specification). Run the server from `examples/http/<framework>/server/` and use a generic SSE client (like the one in `examples/cmd/gomcp-client` configured for SSE).
- **`examples/websocket/`**: Demonstrates the **WebSocket transport** (compatible with the `2025-03-26` specification). Run the server from `examples/websocket/server/` and use a generic WebSocket client (like `examples/cmd/gomcp-client` configured for WebSocket).
- **`examples/configuration/`**: Shows how to load server configuration from JSON, YAML, or TOML files. Run the specific server (e.g., `cd examples/configuration/json/server && go run .`) which loads the corresponding config file (e.g., `examples/configuration/json/config.json`).
- **`examples/auth/`**: Demonstrates simple API key authentication (stdio).
- **`examples/billing/`**: Builds on the auth example, simulating billing/tracking (stdio).
- **`examples/rate-limit/`**: Builds on the auth example, adding simple global rate limiting (stdio).
- **`examples/kitchen-sink/`**: A comprehensive server example combining multiple features (stdio).
- **`examples/cmd/`**: Contains generic command-line client and server implementations that can be configured for different transports.

_(Check the specific README within each example directory for more detailed instructions if available.)_

## Documentation

More detailed documentation can be found in the [GitHub Pages site](https://gomcp.dev) (powered by the `/docs` directory).

Go package documentation is available via:

- [pkg.go.dev](https://pkg.go.dev/github.com/localrivet/gomcp)
- Running `godoc -http=:6060` locally and navigating to `github.com/localrivet/gomcp`.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the [MIT License](LICENSE).
