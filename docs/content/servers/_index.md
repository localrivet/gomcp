---
title: Servers
weight: 20
cascade:
  type: docs
---

The `gomcp` library provides the core logic for building mcp servers. The main server logic resides in the `server` package, which is designed to be transport-agnostic. This means the core server doesn't care how messages are sent or received; that's handled by a separate `Transport` implementation.

### The `server.Server` and `client.Client`

- `server.Server`: The central object for your MCP server application. You register tools, resources, and prompts with it.
- `client.Client`: Represents a connection to an MCP server, allowing you to interact with its capabilities.

```go
// Server Initialization
srv := server.NewServer("my-awesome-server")

// Client Initialization (Stdio example)
clt, err := client.NewStdioClient("my-cool-client", client.ClientOptions{})
if err != nil { /* handle error */ }
```

## Server Role

An MCP server built with `gomcp` typically performs the following functions:

1.  **Listens for Connections:** Accepts incoming client connections via a specific transport mechanism (e.g., Stdio, SSE+HTTP, WebSockets).
2.  **Handles Initialization:** Manages the MCP initialization handshake (`initialize` request and `initialized` notification) with connecting clients to exchange capabilities and establish a session.
3.  **Exposes Capabilities:** Advertises its supported features and the capabilities it provides (tools, resources, prompts) to connected clients.
4.  **Registers Offerings:** Allows you to register the specific tools, resources, and prompts that your server makes available.
5.  **Handles Client Requests:** Processes incoming JSON-RPC requests from clients (e.g., `tools/call`, `resources/get`, `prompts/list`) and sends back appropriate responses or errors.
6.  **Handles Client Notifications:** Processes incoming JSON-RPC notifications from clients (e.g., `$/cancelled`, `initialized`).
7.  **Sends Server Messages:** Can send server-initiated notifications to clients (e.g., `$/progress`, `notifications/message`, `notifications/resources/updated`) to provide updates or information.

## Initializing the Server (`server` package)

The `server.Server` struct holds the state and core logic for your MCP server instance. You create a new server using the `server.NewServer` constructor, providing a server name and optional configuration options.

```go
package main

import (
	"log"
	"os"

	"github.com/localrivet/gomcp/server"
	// Import necessary transport package(s)
	"github.com/localrivet/gomcp/transport/stdio"
)

func main() {
	// Configure logger (optional, defaults to stderr)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting My MCP Server...")

	// 1. Create the Server Instance
	// Provide a unique name for your server and optional server options.
	srv := server.NewServer("my-gomcp-server", server.ServerOptions{
		// Optional: Configure server capabilities if needed (defaults are reasonable)
		// ServerCapabilities: protocol.ServerCapabilities{
		// 	Tools: &protocol.ToolsCaps{ListChanged: true}, // Example: Indicate support for tools list changes
		// },
		// Optional: Provide a custom logger
		// Logger: myCustomLogger,
		// Optional: Set server instructions (sent to client during initialization)
		// Instructions: "This server provides tools for data analysis.",
	})

	// 2. Define and Register Capabilities (Tools, Resources, Prompts)
	// This is where you define and register the specific functionalities
	// your server offers. See the dedicated guides for each type:
	// - [Tools]({{< ref "servers/defining-tools" >}})
	// - [Resources]({{< ref "servers/resources" >}})
	// - [Prompts]({{< ref "servers/prompts" >}})

	// Example: Register a simple tool (details in the Tools guide)
	// srv.RegisterTool(...)

	// 3. Run the Server using a built-in transport handler
	// Choose the appropriate ServeX function based on your desired transport.
	log.Println("Server setup complete. Starting server...")
	if err := server.ServeStdio(srv); err != nil { // Example using Stdio transport
		// ServeStdio blocks until the transport is closed or an error occurs
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server stopped.")
}
```

The `server.ServerOptions` struct allows you to customize the server's behavior:

- `ServerCapabilities` (`protocol.ServerCapabilities`): Define the features your server supports.
- `Logger` (`types.Logger`): Provide a custom logger implementation.
- `Instructions` (`string`): A string containing instructions or a brief description of the server, sent to the client during initialization.

## Registering Capabilities

Before running the server, you need to register the specific tools, resources, and prompts that your server will expose to clients. The `server.Server` provides methods for this:

- `RegisterTool(tool protocol.Tool, handler hooks.FinalToolHandler)`: Registers a tool definition and the handler function that will be executed when a client calls the tool. See the [Tools]({{< ref "servers/defining-tools" >}}) guide for details.
- `RegisterResource(resource protocol.Resource)`: Registers a resource definition. See the [Resources]({{< ref "servers/resources" >}}) guide for details on defining and providing resource content.
- `RegisterPrompt(prompt protocol.Prompt)`: Registers a prompt template definition. See the [Prompts]({{< ref "servers/prompts" >}}) guide for details.

## Running the Server

After creating the server instance and registering its capabilities, you need to start it using a transport handler. The `server` package provides convenience functions for common transports:

- `server.ServeStdio(srv *Server)`: Runs the server using Standard Input/Output.
- `server.ServeSSE(srv *Server, addr string, basePath string)`: Runs the server using the HTTP+SSE transport.
- `server.ServeWebSocket(srv *Server, addr string, path string)`: Runs the server using the WebSocket transport.
- `server.ServeTCP(srv *Server, addr string)`: Runs the server using a raw TCP socket connection.

Choose the appropriate `ServeX` function based on how you want your server to communicate. These functions typically block, running the server's main loop until the transport is closed or an error occurs.

Alternatively, for more control, you can implement the `types.Transport` interface yourself and use the `srv.Run(transport types.Transport)` method.

## Handling Client Messages

The `server.Server` automatically handles incoming JSON-RPC requests and notifications based on the registered capabilities and standard protocol methods (like `initialize`, `tools/call`, `resources/get`, `$/cancelled`). You generally don't need to handle raw messages directly unless implementing custom notifications or requests.

## Sending Server Messages

Your server application can send notifications or requests to connected clients. This is usually done via the `types.ClientSession` interface, which is managed internally by the server and implemented by the transport layer. Methods like `session.SendNotification` and `session.SendResponse` (accessible via the `ClientSession` object passed to handlers or retrieved from the server's session management) facilitate this. The `server.Server` also provides convenience methods like `srv.SendProgress` and `srv.NotifyResourceUpdated` for common notifications. See the [Context]({{< ref "servers/context" >}}) guide for more on accessing session context and sending messages.
