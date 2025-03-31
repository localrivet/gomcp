---
title: net/http
weight: 10 # First HTTP example
---

This page details the examples found in the `/examples/http` directory, demonstrating how to set up an MCP server using the HTTP + Server-Sent Events (SSE) hybrid transport.

This transport uses:

- **HTTP POST:** For client-to-server messages (requests like `tools/call`, notifications like `initialized`).
- **Server-Sent Events (SSE):** For server-to-client messages (responses like `initialize` result, notifications like `$/progress`, server-sent requests).

The `transport/sse` package provides an `sse.Server` that handles the transport logic. You typically integrate its handlers into your chosen Go web framework or the standard `net/http` library.

## Standard Library (`net/http`) Example (`examples/http/nethttp`)

This example shows how to integrate the `sse.Server` with Go's built-in `net/http` package.

**Key parts:**

```go
package main

import (
	"log"
	"net/http"
	// ... other imports: server, protocol, types ...
	"github.com/localrivet/gomcp/transport/sse"
)

func main() {
	// 1. Setup MCP Server (like in basic examples)
	serverInfo := types.Implementation{Name: "http-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	// opts.Capabilities... // Set capabilities
	srv := server.NewServer(opts)
	// srv.RegisterTool(...) // Register tools, resources, etc.

	// 2. Create SSE Transport Server
	sseServer := sse.NewServer(srv, opts.Logger) // Pass the MCP server instance

	// 3. Setup HTTP Handlers
	mux := http.NewServeMux()
	// The SSE handler manages the persistent event stream
	mux.HandleFunc("/events", sseServer.HTTPHandler)
	// The Message handler receives client POST requests
	mux.HandleFunc("/message", srv.HTTPHandler) // Use the MCP server's handler

	// 4. Start HTTP Server
	log.Println("Starting HTTP+SSE MCP server on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
```

**To Run:** Navigate to `examples/http/nethttp` and run `go run main.go`. Clients can then connect by establishing an SSE connection to `/events` and sending POST requests to `/message`.

## Other Frameworks

The `/examples/http` directory contains similar integrations for popular frameworks like Gin, Echo, Chi, Fiber, etc. The core principle remains the same:

1. Initialize the `server.Server`.
2. Initialize the `sse.Server`, passing the `server.Server` instance.
3. Mount the `sseServer.HTTPHandler` to an endpoint for the event stream (e.g., `/events`).
4. Mount the `srv.HTTPHandler` to an endpoint for receiving client messages (e.g., `/message`).

Refer to the specific subdirectories for framework-specific integration details.
