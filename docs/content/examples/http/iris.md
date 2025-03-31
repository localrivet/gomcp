---
title: Iris
weight: 27 # After HttpRouter
---

This page details the example found in `/examples/http/iris`, demonstrating how to integrate the HTTP+SSE transport with the [Iris](https://github.com/kataras/iris) web framework.

## Iris Server (`examples/http/iris`)

This example shows how to mount the `sse.Server` and `server.Server` HTTP handlers onto an Iris application.

**Key parts:**

```go
package main

import (
	"log"
	"net/http"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/logger"
	"github.com/kataras/iris/v12/middleware/recover"
	// ... other imports: server, protocol, types, sse ...
)

func main() {
	// 1. Setup MCP Server (as usual)
	serverInfo := types.Implementation{Name: "iris-http-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	srv := server.NewServer(opts)
	// Register tools, etc.

	// 2. Create SSE Transport Server
	sseServer := sse.NewServer(srv, opts.Logger)

	// 3. Setup Iris App
	app := iris.New()
	app.Use(recover.New())
	app.Use(logger.New())

	// Wrap the standard http.HandlerFunc for Iris
	eventsHandler := iris.FromStd(http.HandlerFunc(sseServer.HTTPHandler))
	messageHandler := iris.FromStd(http.HandlerFunc(srv.HTTPHandler))

	// Mount handlers
	app.Get("/events", eventsHandler)
	app.Post("/message", messageHandler)

	// Add a root handler for testing
	app.Get("/", func(ctx iris.Context) {
		ctx.WriteString("Iris MCP Server running. Use /events and /message.")
	})

	// 4. Start Iris Server
	log.Println("Starting Iris HTTP+SSE MCP server on :8080...")
	// Use app.Listen for standard net/http server start
	err := app.Listen(":8080")
	if err != nil {
		log.Fatalf("Iris server error: %v", err)
	}
}
```

**To Run:** Navigate to `examples/http/iris` and run `go run main.go`. Clients connect as described in the `net/http` example (SSE to `/events`, POST to `/message`).

Iris provides the `iris.FromStd` function to easily convert a standard `http.Handler` or `http.HandlerFunc` into a handler compatible with the Iris router.
