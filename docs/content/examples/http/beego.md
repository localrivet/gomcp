---
title: Beego
weight: 28 # After Iris
---

This page details the example found in `/examples/http/beego`, demonstrating how to integrate the HTTP+SSE transport with the [Beego](https://github.com/beego/beego) web framework.

## Beego Server (`examples/http/beego`)

This example shows how to mount the `sse.Server` and `server.Server` HTTP handlers onto a Beego application.

**Key parts:**

```go
package main

import (
	"log"
	"net/http"

	"github.com/beego/beego/v2/server/web"
	// ... other imports: server, protocol, types, sse ...
)

func main() {
	// 1. Setup MCP Server (as usual)
	serverInfo := types.Implementation{Name: "beego-http-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	srv := server.NewServer(opts)
	// Register tools, etc.

	// 2. Create SSE Transport Server
	sseServer := sse.NewServer(srv, opts.Logger)

	// 3. Setup Beego Handlers
	// Beego typically uses controllers, but we can adapt http.Handlers
	web.Handler("/events", http.HandlerFunc(sseServer.HTTPHandler))
	web.Handler("/message", http.HandlerFunc(srv.HTTPHandler))

	// Add a root handler for testing
	web.Get("/", func(ctx *context.Context) { // Note: Beego v2 context might differ
		ctx.WriteString("Beego MCP Server running. Use /events and /message.")
	})

	// 4. Start Beego Server
	log.Println("Starting Beego HTTP+SSE MCP server on :8080...")
	// Beego uses web.Run() to start the server
	web.Run(":8080")
	// Note: Unlike http.ListenAndServe, web.Run() might not return errors in the same way.
	// Check Beego docs for production error handling.
}
```

**To Run:** Navigate to `examples/http/beego` and run `go run main.go`. Clients connect as described in the `net/http` example (SSE to `/events`, POST to `/message`).

Beego's `web.Handler` function allows registering standard `http.Handler` interfaces directly for specific routes and methods.
