---
title: Creating a Client
weight: 50
---

The `gomcp` library provides tools for building applications that act as MCP clients, connecting to MCP servers to utilize their capabilities (tools, resources, prompts).

The primary implementation for a client is found in the `client` package, which uses the SSE+HTTP hybrid transport model.

## Client Role

An MCP client typically:

1.  Connects to a known MCP server endpoint.
2.  Performs the initialization handshake (`initialize` / `initialized`).
3.  Discovers available server capabilities (tools, resources, prompts) using `list` requests.
4.  Executes server tools (`tools/call`).
5.  Accesses server resources (`resources/get`).
6.  Subscribes to resource updates (`resources/subscribe`).
7.  Handles server-sent notifications (e.g., `$/progress`, `notifications/message`).
8.  Handles server-sent requests (if the server needs to request actions from the client, though less common).

## Initializing the Client (`client` package)

The `client` package provides a `Client` struct that manages the connection and communication flow using the SSE+HTTP transport.

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
	// types package might not be needed directly for basic client setup
)

func main() {
	// Configure logger
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Simple Stdio MCP Client...")

	// 1. Create Client Instance for Stdio
	// NewStdioClient handles stdio transport setup internally.
	// Provide a client name and optional ClientOptions.
	clt, err := client.NewStdioClient("my-stdio-client", client.ClientOptions{
		// ClientInfo and Capabilities can be customized here if needed.
		// Example:
		// ClientInfo: protocol.Implementation{Name: "my-stdio-client", Version: "1.0"},
		// Capabilities: protocol.ClientCapabilities{ /* ... */ },
		// Logger: provide a custom logger if desired
	})
	if err != nil {
		log.Fatalf("Failed to create stdio client: %v", err)
	}

	// 2. Connect and Initialize
	// Use a context for timeout/cancellation for the connection attempt.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("Connecting to server via stdio...")
	// Connect performs the MCP initialization handshake over stdio.
	err = clt.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect and initialize with server: %v", err)
	}
	// For stdio, Close is often handled implicitly when stdin/stdout close,
	// but deferring it ensures cleanup if the client exits early.
	defer clt.Close()

	// 3. Access Server Information (Post-Connection)
	// Once connected, you can get info about the server.
	serverInfo := clt.ServerInfo()
	log.Printf("Connected to server: %s (Version: %s)", serverInfo.Name, serverInfo.Version)
	log.Printf("Server capabilities: %+v", clt.ServerCapabilities())

	// --- Client is now ready to make requests ---

	// Example: List available tools from the server
	// Use a derived context for the specific request.
	listToolsCtx, listToolsCancel := context.WithTimeout(ctx, 10*time.Second)
	defer listToolsCancel()

	toolsResult, err := clt.ListTools(listToolsCtx, protocol.ListToolsRequestParams{})
	if err != nil {
		log.Printf("Error listing tools: %v", err)
	} else {
		log.Printf("Available tools (%d):", len(toolsResult.Tools))
		for _, tool := range toolsResult.Tools {
			log.Printf("- %s: %s", tool.Name, tool.Description)
		}
	}

	// Add more client logic here (e.g., CallTool, GetResource) using the 'clt' instance.

	log.Println("Client operations finished.")
	// In a real application, the client might wait for more tasks or exit.
	// For stdio, the client often runs until its input/output streams are closed.
}

```

_Note: This example assumes a server is running at `http://localhost:8080` using the SSE+HTTP transport._

## Making Requests

Once connected, you can use the methods provided by the `client.Client` struct to interact with the server:

- `ListTools(ctx, params)`
- `CallTool(ctx, params)`
- `ListResources(ctx, params)`
- `GetResource(ctx, params)`
- `SubscribeResources(ctx, params)`
- `UnsubscribeResources(ctx, params)`
- `ListPrompts(ctx, params)`
- `GetPrompt(ctx, params)`
- `SendCancellation(ctx, id)`
- `SendProgress(ctx, params)` // If client needs to report progress
- // ... and others

Each method takes a `context.Context` for cancellation and deadlines, and the corresponding parameter struct defined in the `protocol` package.

## Handling Server Messages

Use `RegisterNotificationHandler` and `RegisterRequestHandler` _before_ calling `Connect` to set up functions that will be called when the server sends asynchronous notifications or requests over the SSE connection.
