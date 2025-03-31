---
title: WebSocket
weight: 30 # Third example
---

This page details the example found in the `/examples/websocket` directory, demonstrating how to set up an MCP server using the WebSocket transport.

The WebSocket transport provides full-duplex communication over a single TCP connection, allowing both client and server to send messages at any time. The `transport/websocket` package provides the necessary components.

## WebSocket Server (`examples/websocket/server`)

This example shows how to integrate the `websocket.Factory` with Go's standard `net/http` server to handle WebSocket connections.

**Key parts:**

```go
package main

import (
	"log"
	"net/http"
	// ... other imports: server, protocol, types ...
	"github.com/localrivet/gomcp/transport/websocket"
)

func main() {
	// 1. Setup MCP Server (like in basic examples)
	serverInfo := types.Implementation{Name: "websocket-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	// opts.Capabilities... // Set capabilities
	srv := server.NewServer(opts)
	// srv.RegisterTool(...) // Register tools, resources, etc.

	// 2. Create a WebSocket Transport Factory
	// The factory creates a new transport instance for each incoming connection
	wsFactory := websocket.NewFactory(srv, opts.Logger) // Pass the MCP server instance

	// 3. Setup HTTP Handler for WebSocket Upgrades
	mux := http.NewServeMux()
	// The factory's HTTPHandler upgrades connections and runs the transport
	mux.HandleFunc("/ws", wsFactory.HTTPHandler)

	// 4. Start HTTP Server
	log.Println("Starting WebSocket MCP server on :8080/ws...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
```

**To Run:** Navigate to `examples/websocket/server` and run `go run main.go`. An MCP client capable of communicating over WebSockets can then connect to `ws://localhost:8080/ws`.

**Note:** Unlike the SSE+HTTP transport which uses separate endpoints for events and messages, the WebSocket transport typically uses a single endpoint (`/ws` in this case) for the entire bidirectional communication after the initial HTTP upgrade request.
