---
title: Rate Limiting
weight: 90 # Last example
---

This page details the example found in the `/examples/rate-limit` directory, demonstrating how to implement rate limiting for an MCP server, typically using middleware in the transport layer.

Rate limiting is crucial for preventing abuse and ensuring fair usage of server resources. This example likely uses a token bucket algorithm, possibly via the `golang.org/x/time/rate` package.

## Rate-Limited Server (`examples/rate-limit/server`)

This example adds middleware to the HTTP+SSE transport to limit the rate at which clients can send messages (e.g., `tools/call` requests).

**Key parts:**

```go
package main

import (
	"log"
	"net" // Added for net.SplitHostPort
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate" // For rate limiting
	// ... other imports: server, protocol, types, sse ...
	"github.com/localrivet/gomcp/server" // Added for server types
	"github.com/localrivet/gomcp/types"  // Added for types.Implementation
)

// Simple IP-based rate limiter store
var (
	visitors = make(map[string]*rate.Limiter)
	mu       sync.Mutex
)

// Function to get or create a limiter for an IP address
func getVisitorLimiter(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	limiter, exists := visitors[ip]
	if !exists {
		// Example: Allow 1 request every 2 seconds (burst of 3)
		limiter = rate.NewLimiter(rate.Every(2*time.Second), 3)
		visitors[ip] = limiter
	}
	return limiter
}

// Rate limiting middleware
func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP (simplified, consider X-Forwarded-For in real apps)
		ip := r.RemoteAddr
		// For simplicity, just taking the host part if port is present
		if host, _, err := net.SplitHostPort(ip); err == nil {
			ip = host
		}

		limiter := getVisitorLimiter(ip)
		if !limiter.Allow() {
			log.Printf("Rate limit exceeded for %s on %s", ip, r.URL.Path)
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return // Stop processing if rate limit exceeded
		}

		// Allow request if within limit
		next.ServeHTTP(w, r)
	})
}


func main() {
	// 1. Setup MCP Server (as usual)
	serverInfo := types.Implementation{Name: "rate-limit-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	srv := server.NewServer(opts)
	// Register tools, etc.

	// 2. Create SSE Transport Server
	sseServer := sse.NewServer(srv, opts.Logger)

	// 3. Setup HTTP Handlers with Middleware
	mux := http.NewServeMux()

	// Apply rate limiting middleware *only* to the message handler
	// (SSE connection itself is usually not rate-limited this way)
	rateLimitedMessageHandler := rateLimitMiddleware(http.HandlerFunc(srv.HTTPHandler))

	mux.Handle("/message", rateLimitedMessageHandler)
	mux.HandleFunc("/events", sseServer.HTTPHandler) // SSE handler without rate limit

	// 4. Start HTTP Server
	log.Println("Starting Rate-Limited MCP server on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
```

**To Run:**

1. Navigate to `examples/rate-limit/server` and run `go run main.go`.
2. Use an MCP client (like the one in `examples/rate-limit/client`) to send multiple requests quickly to `/message`. Observe that after a few successful requests, subsequent requests will receive a `429 Too Many Requests` error until the rate limit interval passes.

**Note:** This is a basic IP-based limiter. Real-world applications might require more sophisticated rate limiting based on authenticated users, API keys, or specific resources being accessed.
