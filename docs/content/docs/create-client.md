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
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
)

func main() {
	// Server's base URL (adjust as needed)
	serverURL := "http://localhost:8080" // Example URL

	// Define client information
	clientInfo := types.Implementation{
		Name:    "my-simple-client",
		Version: "0.1.0",
	}

	// Define client capabilities (optional, customize as needed)
	clientCapabilities := protocol.ClientCapabilities{
		// Add specific capabilities your client supports
	}

	// Create client options
	opts := client.NewClientOptions(clientInfo, clientCapabilities)
	// opts.Logger = /* provide a custom logger if desired */

	// Create a new client instance
	c := client.NewClient(serverURL, opts)

	// --- Optional: Register handlers for server-sent messages ---
	// Example: Handle log messages from the server
	c.RegisterNotificationHandler(protocol.MethodLogMessage, func(ctx context.Context, params []byte) error {
		var logParams protocol.LoggingMessageParams
		if err := json.Unmarshal(params, &logParams); err != nil {
			log.Printf("Error unmarshalling log message params: %v", err)
			return nil // Don't kill connection for bad log message
		}
		log.Printf("SERVER LOG [%s]: %s", logParams.Level, logParams.Message)
		return nil
	})
	// Register other handlers for notifications or server requests as needed...

	// --- Connect and Initialize ---
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Add a timeout
	defer cancel()

	serverInfo, err := c.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect and initialize with server: %v", err)
	}
	log.Printf("Connected to server: %s v%s", serverInfo.Name, serverInfo.Version)
	log.Printf("Server capabilities: %+v", c.ServerCapabilities()) // Access cached capabilities

	// --- Client is now ready to make requests ---

	// Example: List available tools
	listToolsCtx, listToolsCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer listToolsCancel()

	toolsResult, err := c.ListTools(listToolsCtx, protocol.ListToolsRequestParams{})
	if err != nil {
		log.Printf("Error listing tools: %v", err)
	} else {
		log.Printf("Available tools (%d):", len(toolsResult.Tools))
		for _, tool := range toolsResult.Tools {
			log.Printf("- %s: %s", tool.Name, tool.Description)
		}
	}

	// Add more client logic here (CallTool, GetResource, etc.)

	// Keep the client running (e.g., wait for user input or another signal)
	log.Println("Client running. Press Ctrl+C to exit.")
	<-ctx.Done() // Wait for context cancellation (e.g., timeout or manual cancel)

	// Disconnect (optional, closes SSE connection)
	c.Disconnect()
	log.Println("Client disconnected.")
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
