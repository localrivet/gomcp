---
title: Echo
weight: 23 # After Chi
---

This page details the example found in `/examples/http/echo`, demonstrating how to integrate the HTTP+SSE transport with the [Echo](https://github.com/labstack/echo) web framework.

## Echo Server (`examples/http/echo`)

This example shows how to mount the `sse.Server` and `server.Server` HTTP handlers onto an Echo router.

**Key parts:**

```go
package main

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	// ... other imports: server, protocol, types, sse ...
)

func main() {
	// 1. Setup MCP Server (as usual)
	serverInfo := types.Implementation{Name: "echo-http-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	srv := server.NewServer(opts)
	// Register tools, etc.

	// 2. Create SSE Transport Server
	sseServer := sse.NewServer(srv, opts.Logger)

	// 3. Setup Echo Instance
	e := echo.New()
	e.Use(middleware.Logger()) // Example middleware

	// Wrap the MCP handlers for Echo
	// Echo expects handlers of type echo.HandlerFunc
	eventsHandler := echo.WrapHandler(http.HandlerFunc(sseServer.HTTPHandler))
	messageHandler := echo.WrapHandler(http.HandlerFunc(srv.HTTPHandler))

	// Mount handlers
	e.GET("/events", eventsHandler)
	e.POST("/message", messageHandler)

	// Add a root handler for testing
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Echo MCP Server running. Use /events and /message.")
	})

	// 4. Start Echo Server
	log.Println("Starting Echo HTTP+SSE MCP server on :8080...")
	if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Echo server error: %v", err)
	}
}
```

**To Run:** Navigate to `examples/http/echo` and run `go run main.go`. Clients connect as described in the `net/http` example (SSE to `/events`, POST to `/message`).

Similar to the Gin example, the key is using `echo.WrapHandler` to adapt the standard `http.HandlerFunc` provided by `gomcp` to the `echo.HandlerFunc` expected by the Echo framework.
