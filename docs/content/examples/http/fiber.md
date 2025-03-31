---
title: Fiber
weight: 24 # After Echo
---

This page details the example found in `/examples/http/fiber`, demonstrating how to integrate the HTTP+SSE transport with the [Fiber](https://github.com/gofiber/fiber) web framework.

## Fiber Server (`examples/http/fiber`)

This example shows how to mount the `sse.Server` and `server.Server` HTTP handlers onto a Fiber app, using the `adaptor` package to convert standard `http.Handler`s.

**Key parts:**

```go
package main

import (
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor" // For converting http.Handler
	"github.com/gofiber/fiber/v2/middleware/logger"
	// ... other imports: server, protocol, types, sse ...
)

func main() {
	// 1. Setup MCP Server (as usual)
	serverInfo := types.Implementation{Name: "fiber-http-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	srv := server.NewServer(opts)
	// Register tools, etc.

	// 2. Create SSE Transport Server
	sseServer := sse.NewServer(srv, opts.Logger)

	// 3. Setup Fiber App
	app := fiber.New()
	app.Use(logger.New()) // Example middleware

	// Convert the standard http.HandlerFunc to Fiber handlers
	eventsHandler := adaptor.HTTPHandlerFunc(sseServer.HTTPHandler)
	messageHandler := adaptor.HTTPHandlerFunc(srv.HTTPHandler)

	// Mount handlers
	app.Get("/events", eventsHandler)
	app.Post("/message", messageHandler)

	// Add a root handler for testing
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Fiber MCP Server running. Use /events and /message.")
	})

	// 4. Start Fiber Server
	log.Println("Starting Fiber HTTP+SSE MCP server on :8080...")
	if err := app.Listen(":8080"); err != nil {
		log.Fatalf("Fiber server error: %v", err)
	}
}
```

**To Run:** Navigate to `examples/http/fiber` and run `go run main.go`. Clients connect as described in the `net/http` example (SSE to `/events`, POST to `/message`).

The key here is using `adaptor.HTTPHandlerFunc` from the Fiber middleware package to wrap the standard `http.HandlerFunc` provided by `gomcp` so it can be used with Fiber's routing methods.
