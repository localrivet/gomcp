---
title: Chi
weight: 22 # After Gin
---

This page details the example found in `/examples/http/chi`, demonstrating how to integrate the HTTP+SSE transport with the [Chi](https://github.com/go-chi/chi) router.

## Chi Server (`examples/http/chi`)

This example shows how to mount the `sse.Server` and `server.Server` HTTP handlers onto a Chi router.

**Key parts:**

```go
package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	// ... other imports: server, protocol, types, sse ...
)

func main() {
	// 1. Setup MCP Server (as usual)
	serverInfo := types.Implementation{Name: "chi-http-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	srv := server.NewServer(opts)
	// Register tools, etc.

	// 2. Create SSE Transport Server
	sseServer := sse.NewServer(srv, opts.Logger)

	// 3. Setup Chi Router
	r := chi.NewRouter()
	r.Use(middleware.Logger) // Example middleware

	// Mount handlers directly as they satisfy http.Handler
	r.Get("/events", sseServer.HTTPHandler)
	r.Post("/message", srv.HTTPHandler)

	// Add a root handler for testing
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Chi MCP Server running. Use /events and /message."))
	})

	// 4. Start Chi Server
	log.Println("Starting Chi HTTP+SSE MCP server on :8080...")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Chi server error: %v", err)
	}
}
```

**To Run:** Navigate to `examples/http/chi` and run `go run main.go`. Clients connect as described in the `net/http` example (SSE to `/events`, POST to `/message`).

Since the `gomcp` handlers (`sseServer.HTTPHandler` and `srv.HTTPHandler`) implement the standard `http.Handler` interface, they can be mounted directly onto the Chi router using methods like `r.Get` and `r.Post`.
