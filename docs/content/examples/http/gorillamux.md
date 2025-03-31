---
title: Gorilla Mux
weight: 25 # After Fiber
---

This page details the example found in `/examples/http/gorillamux`, demonstrating how to integrate the HTTP+SSE transport with the [Gorilla Mux](https://github.com/gorilla/mux) router.

## Gorilla Mux Server (`examples/http/gorillamux`)

This example shows how to mount the `sse.Server` and `server.Server` HTTP handlers onto a Gorilla Mux router.

**Key parts:**

```go
package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	// ... other imports: server, protocol, types, sse ...
)

func main() {
	// 1. Setup MCP Server (as usual)
	serverInfo := types.Implementation{Name: "gorillamux-http-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	srv := server.NewServer(opts)
	// Register tools, etc.

	// 2. Create SSE Transport Server
	sseServer := sse.NewServer(srv, opts.Logger)

	// 3. Setup Gorilla Mux Router
	r := mux.NewRouter()

	// Mount handlers directly as they satisfy http.Handler
	r.HandleFunc("/events", sseServer.HTTPHandler).Methods("GET")
	r.HandleFunc("/message", srv.HTTPHandler).Methods("POST")

	// Add a root handler for testing
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Gorilla Mux MCP Server running. Use /events and /message."))
	}).Methods("GET")

	// 4. Start Server with Mux Router
	log.Println("Starting Gorilla Mux HTTP+SSE MCP server on :8080...")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
```

**To Run:** Navigate to `examples/http/gorillamux` and run `go run main.go`. Clients connect as described in the `net/http` example (SSE to `/events`, POST to `/message`).

Similar to Chi, the standard `http.HandlerFunc` provided by `gomcp` can be used directly with Gorilla Mux's `HandleFunc` method, specifying the appropriate HTTP method (GET for SSE, POST for messages).
