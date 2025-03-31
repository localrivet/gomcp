---
title: Authentication
weight: 50 # Fifth example
---

This page details the example found in the `/examples/auth` directory, demonstrating how to add a simple authentication layer to an MCP server, typically implemented as middleware in the transport layer.

MCP itself doesn't prescribe a specific authentication mechanism, allowing flexibility. This example uses a simple token check within HTTP middleware.

## Authenticated Server (`examples/auth/server`)

This example builds upon the HTTP+SSE transport, adding middleware to check for a valid `Authorization` header on incoming HTTP requests (both for the `/message` POSTs and the initial `/events` SSE connection).

**Key parts:**

```go
package main

import (
	"fmt" // Added for root handler
	"log"
	"net/http"
	"strings"
	// ... other imports: server, protocol, types, sse ...
)

// Simple authentication middleware
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authToken := r.Header.Get("Authorization")
		// In a real app, validate the token properly!
		// This is a placeholder check.
		expectedToken := "Bearer my-secret-token"

		if authToken != expectedToken {
			log.Printf("Auth failed for %s: Invalid token '%s'", r.URL.Path, authToken)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return // Stop processing if auth fails
		}

		log.Printf("Auth successful for %s", r.URL.Path)
		next.ServeHTTP(w, r) // Call the next handler if auth succeeds
	})
}


func main() {
	// 1. Setup MCP Server (as usual)
	serverInfo := types.Implementation{Name: "auth-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	srv := server.NewServer(opts)
	// Register tools, etc.

	// 2. Create SSE Transport Server
	sseServer := sse.NewServer(srv, opts.Logger)

	// 3. Setup HTTP Handlers
	mux := http.NewServeMux()
	// Apply middleware *only* to the MCP handlers
	authedEventsHandler := authMiddleware(http.HandlerFunc(sseServer.HTTPHandler))
	authedMessageHandler := authMiddleware(http.HandlerFunc(srv.HTTPHandler))

	mux.Handle("/events", authedEventsHandler)
	mux.Handle("/message", authedMessageHandler)

	// Add an unauthenticated root handler for testing
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "MCP Auth Server running. Use /events and /message with Authorization header.")
	})


	// 4. Start HTTP Server
	log.Println("Starting Auth MCP server on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
```

**To Run:**

1. Navigate to `examples/auth/server` and run `go run main.go`.
2. Use an MCP client (like the one in `examples/auth/client`) or tools like `curl` to interact:
   - `curl -H "Authorization: Bearer my-secret-token" http://localhost:8080/events` (for SSE)
   - `curl -X POST -H "Authorization: Bearer my-secret-token" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0", "method":"initialize", ...}' http://localhost:8080/message`

Requests without the correct `Authorization: Bearer my-secret-token` header will receive a `401 Unauthorized` response.
