---
title: Creating a Server
weight: 30
---

The `gomcp` library provides the core logic for building applications that act as MCP servers, exposing capabilities like tools, resources, and prompts to connected clients.

The main server logic resides in the `server` package. This package is transport-agnostic, meaning the core server doesn't care how messages are sent or received; that's handled by a separate `Transport` implementation.

## Server Role

An MCP server typically:

1.  Listens for incoming client connections via a specific transport (e.g., Stdio, SSE+HTTP, WebSockets).
2.  Handles the initialization handshake (`initialize` / `initialized`) with connecting clients.
3.  Exposes its capabilities (server info, supported protocol features).
4.  Registers and makes available its specific offerings:
    - Tools that clients can execute.
    - Resources that clients can access or subscribe to.
    - Prompts that clients can list or retrieve.
5.  Handles client requests (`tools/call`, `resources/get`, etc.) and sends back responses or errors.
6.  Handles client notifications (`$/cancelled`, `initialized`, etc.).
7.  Sends server-initiated notifications to clients (`$/progress`, `notifications/message`, `notifications/resources/content_changed`, etc.).

## Initializing the Server (`server` package)

The `server.Server` struct holds the state and logic for the MCP server.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/util/schema" // Use schema helper
)

// Define arguments struct for the echo tool
type EchoArgs struct {
	Input string `json:"input" description:"Text to echo"`
}

// Example Tool Handler Function using the correct signature
// and schema.HandleArgs for parsing.
func handleEchoTool(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	args, errContent, isErr := schema.HandleArgs[EchoArgs](arguments)
	if isErr {
		log.Printf("Error handling echo args: %v", errContent)
		return errContent, true
	}

	log.Printf("Executing echo tool with input: %s", args.Input)
	return []protocol.Content{
		protocol.TextContent{Type: "text", Text: "Echo: " + args.Input},
	}, false
}

func main() {
	// Configure logger
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	// 1. Create the Server Instance
	// Provide a name and optional server options
	srv := server.NewServer("my-gomcp-server", server.ServerOptions{
		// Define server capabilities if needed (defaults are reasonable)
		ServerCapabilities: protocol.ServerCapabilities{
			Tools: &protocol.ToolsCaps{ListChanged: true}, // Example
		},
		// Logger: provide a custom logger if desired
	})

	// 2. Define and Register Capabilities (e.g., Tools)
	echoTool := protocol.Tool{
		Name:        "echo",
		Description: "Simple tool that echoes back the input text.",
		InputSchema: schema.FromStruct(EchoArgs{}), // Generate schema from struct
	}
	// Register the tool with its handler
	err := srv.RegisterTool(echoTool, handleEchoTool)
	if err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}
	log.Printf("Registered tool: %s", echoTool.Name)

	// Register resources and prompts similarly if needed

	// 3. Run the Server using a built-in transport handler
	// Using ServeStdio for simplicity in this example
	log.Println("Starting MCP server on stdio...")
	if err := server.ServeStdio(srv); err != nil {
		// ServeStdio blocks until the transport is closed or an error occurs
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server stopped.")
}
```

## Registering Capabilities

- **`RegisterTool(tool protocol.Tool, handler server.ToolHandlerFunc)`:** Registers a tool definition and the function that implements its logic. The handler receives the arguments provided by the client.
- **`RegisterResource(resource protocol.Resource, provider server.ResourceProvider)`:** Registers a resource and a provider responsible for fetching its content.
- **`RegisterPrompt(prompt protocol.Prompt)`:** Registers a predefined prompt template.

See [Defining Tools]({{< relref "defining-tools" >}}) for more details on tool registration. (Resource and Prompt registration follow similar patterns).

## Running the Server

- **`Run(transport types.Transport)`:** Starts the server's main loop, using the provided transport to receive messages from clients and send messages back. This method typically blocks until the transport is closed or an unrecoverable error occurs.

While `Run` is the core method requiring a pre-configured transport, the `server` package also provides convenience functions to quickly start a server with common transport mechanisms:

- **`ServeStdio(srv *Server)`**: Runs the server using Standard Input/Output, suitable for local process communication.
- **`ServeSSE(srv *Server, addr string, basePath string)`**: Runs the server using the HTTP+SSE transport (compatible with `2024-11-05` clients), listening on the specified network address (e.g., `:8080`) and base HTTP path (e.g., `/mcp`).
- **`ServeWebSocket(srv *Server, addr string, path string)`**: Runs the server using the WebSocket transport (compatible with `2025-03-26` clients), listening on the specified network address (e.g., `:8080`) and WebSocket path (e.g., `/mcp`).
- **`ServeTCP(srv *Server, addr string)`**: Runs the server using a raw TCP socket connection, listening on the specified network address (e.g., `:6000`).

Choose the appropriate `ServeX` function based on the desired transport mechanism for your server.

## Handling Client Messages

The `server.Server` automatically handles incoming JSON-RPC requests and notifications based on the registered capabilities and standard protocol methods (`initialize`, `$/cancelled`, etc.). You generally don't need to handle raw messages directly unless implementing custom notifications or requests.

## Sending Server Messages

The server can send notifications or requests to connected clients. This is usually done via the `ClientSession` interface, which is managed internally by the server and implemented by the transport layer. Methods like `srv.SendProgress(sessionID, token, value)` or `srv.NotifyResourceChanged(uri)` facilitate this. Accessing the correct `sessionID` often requires careful context management within handlers.
