---
title: Gin
weight: 20 # After net/http
---

This page details the example found in `/examples/http/gin`, demonstrating how to integrate the HTTP+SSE transport with the [Gin](https://github.com/gin-gonic/gin) web framework.

## Gin Server (`examples/http/gin`)

This example shows how to mount the `sse.Server` and `server.Server` HTTP handlers onto a Gin router.

**Key parts:**

```go
package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	// ... other imports: server, protocol, types, sse ...
)

func main() {
	// 1. Setup MCP Server (as usual)
	serverInfo := types.Implementation{Name: "gin-http-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	srv := server.NewServer(opts)
	// Register tools, etc.

	// 2. Create SSE Transport Server
	sseServer := sse.NewServer(srv, opts.Logger)

	// 3. Setup Gin Router
	router := gin.Default()

	// Wrap the MCP handlers for Gin
	// Gin expects handlers of type gin.HandlerFunc
	eventsHandler := gin.WrapH(http.HandlerFunc(sseServer.HTTPHandler))
	messageHandler := gin.WrapH(http.HandlerFunc(srv.HTTPHandler))

	// Mount handlers
	router.GET("/events", eventsHandler)
	router.POST("/message", messageHandler)

	// Add a root handler for testing
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Gin MCP Server running. Use /events and /message.")
	})

	// 4. Start Gin Server
	log.Println("Starting Gin HTTP+SSE MCP server on :8080...")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Gin server error: %v", err)
	}
}
```

**To Run:** Navigate to `examples/http/gin` and run `go run main.go`. Clients connect as described in the `net/http` example (SSE to `/events`, POST to `/message`).

The key difference is using `gin.WrapH` to adapt the standard `http.HandlerFunc` provided by the `gomcp` library to the `gin.HandlerFunc` expected by the Gin router.
