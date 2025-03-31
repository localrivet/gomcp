---
title: HttpRouter
weight: 26 # After Gorilla Mux
---

This page details the example found in `/examples/http/httprouter`, demonstrating how to integrate the HTTP+SSE transport with the [HttpRouter](https://github.com/julienschmidt/httprouter) router.

## HttpRouter Server (`examples/http/httprouter`)

This example shows how to mount the `sse.Server` and `server.Server` HTTP handlers onto an HttpRouter instance.

**Key parts:**

```go
package main

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
	// ... other imports: server, protocol, types, sse ...
)

// Helper function to adapt http.Handler to httprouter.Handle
func wrapHandler(h http.Handler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		h.ServeHTTP(w, r)
	}
}


func main() {
	// 1. Setup MCP Server (as usual)
	serverInfo := types.Implementation{Name: "httprouter-http-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	srv := server.NewServer(opts)
	// Register tools, etc.

	// 2. Create SSE Transport Server
	sseServer := sse.NewServer(srv, opts.Logger)

	// 3. Setup HttpRouter
	router := httprouter.New()

	// Wrap and mount handlers
	router.GET("/events", wrapHandler(http.HandlerFunc(sseServer.HTTPHandler)))
	router.POST("/message", wrapHandler(http.HandlerFunc(srv.HTTPHandler)))

	// Add a root handler for testing
	router.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Write([]byte("HttpRouter MCP Server running. Use /events and /message."))
	})

	// 4. Start Server with HttpRouter
	log.Println("Starting HttpRouter HTTP+SSE MCP server on :8080...")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
```

**To Run:** Navigate to `examples/http/httprouter` and run `go run main.go`. Clients connect as described in the `net/http` example (SSE to `/events`, POST to `/message`).

HttpRouter requires handlers of type `httprouter.Handle`. A simple wrapper function (`wrapHandler` in this example) is needed to adapt the standard `http.Handler` provided by `gomcp` to the required signature.
