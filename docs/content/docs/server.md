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
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types"
)

// Example Tool Handler Function
func handleEchoTool(ctx context.Context, args map[string]interface{}) ([]protocol.Content, error) {
	inputText, ok := args["input"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'input' argument")
	}
	return []protocol.Content{
		protocol.TextContent{Type: "text", Text: "Echo: " + inputText},
	}, nil
}


func main() {
	// 1. Define Server Information & Capabilities
	serverInfo := types.Implementation{
		Name:    "my-gomcp-server",
		Version: "1.0.0",
	}
	serverCapabilities := protocol.ServerCapabilities{
		// Indicate which optional features are supported
		Tools: &protocol.ToolsCaps{ListChanged: true}, // Example: We support tool list changes
		// Resources: &protocol.ResourcesCaps{Subscribe: true, ListChanged: true},
		// Prompts:   &protocol.PromptsCaps{ListChanged: true},
		// Logging:   &protocol.LoggingCaps{},
	}

	// 2. Create Server Options
	opts := server.NewServerOptions(serverInfo)
	opts.Capabilities = serverCapabilities // Set the defined capabilities
	// opts.Logger = /* provide a custom logger if desired */
	// opts.Instruction = "Welcome! Use tools/list to see available tools." // Optional instruction sent during init

	// 3. Create the Server Instance
	srv := server.NewServer(opts)

	// 4. Register Capabilities (Tools, Resources, Prompts)
	echoTool := protocol.Tool{
		Name:        "echo",
		Description: "Simple tool that echoes back the input text.",
		InputSchema: protocol.ToolInputSchema{
			Type: "object",
			Properties: map[string]protocol.PropertyDetail{
				"input": {Type: "string", Description: "Text to echo"},
			},
			Required: []string{"input"},
		},
	}
	// The handler function executes the tool's logic
	err := srv.RegisterTool(echoTool, handleEchoTool)
	if err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}
	log.Printf("Registered tool: %s", echoTool.Name)

	// Register resources and prompts similarly using srv.RegisterResource(...) and srv.RegisterPrompt(...)

	// 5. Choose and Create a Transport
	// Using Stdio for simplicity in this example
	transport := stdio.NewStdioTransport(os.Stdin, os.Stdout, opts.Logger)

	// 6. Run the Server
	log.Println("Starting MCP server on stdio...")
	if err := srv.Run(transport); err != nil {
		// Run blocks until the transport is closed or an error occurs
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server stopped.")
}

```

## Registering Capabilities

- **`RegisterTool(tool protocol.Tool, handler server.ToolHandlerFunc)`:** Registers a tool definition and the function that implements its logic. The handler receives the arguments provided by the client.
- **`RegisterResource(resource protocol.Resource, provider server.ResourceProvider)`:** Registers a resource and a provider responsible for fetching its content.
- **`RegisterPrompt(prompt protocol.Prompt)`:** Registers a predefined prompt template.

See [Defining Tools]({{< relref "../server/defining-tools" >}}) for more details on tool registration. (Resource and Prompt registration follow similar patterns).

## Running the Server

- **`Run(transport types.Transport)`:** Starts the server's main loop, using the provided transport to receive messages from clients and send messages back. This method typically blocks until the transport is closed or an unrecoverable error occurs.

## Handling Client Messages

The `server.Server` automatically handles incoming JSON-RPC requests and notifications based on the registered capabilities and standard protocol methods (`initialize`, `$/cancelled`, etc.). You generally don't need to handle raw messages directly unless implementing custom notifications or requests.

## Sending Server Messages

The server can send notifications or requests to connected clients. This is usually done via the `ClientSession` interface, which is managed internally by the server and implemented by the transport layer. Methods like `srv.SendProgress(sessionID, token, value)` or `srv.NotifyResourceChanged(uri)` facilitate this. Accessing the correct `sessionID` often requires careful context management within handlers.
