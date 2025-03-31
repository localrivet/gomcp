---
title: go-zero
weight: 29 # Last HTTP framework example
---

This page details the example found in `/examples/http/go-zero`, demonstrating how to integrate the HTTP+SSE transport with the [go-zero](https://github.com/zeromicro/go-zero) web framework.

## go-zero Server (`examples/http/go-zero`)

This example shows how to mount the `sse.Server` and `server.Server` HTTP handlers onto a go-zero `rest.Server`.

**Key parts:**

```go
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	// ... other imports: server, protocol, types, sse ...
)

// Define go-zero config structure (if needed, often minimal for this)
type Config struct {
	rest.RestConf
}

func main() {
	// --- go-zero Config Loading ---
	var configFile = flag.String("f", "etc/config.yaml", "the config file") // go-zero convention
	flag.Parse()

	var c Config
	conf.MustLoad(*configFile, &c) // Load config (might just contain RestConf)

	// --- Setup MCP Server ---
	serverInfo := types.Implementation{Name: "go-zero-http-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	srv := server.NewServer(opts)
	// Register tools, etc.

	// --- Create SSE Transport Server ---
	sseServer := sse.NewServer(srv, opts.Logger)

	// --- Setup go-zero Server and Routes ---
	engine := rest.MustNewServer(c.RestConf)
	defer engine.Stop()

	// Mount standard http.Handlers directly
	engine.AddRoute(rest.Route{
		Method:  http.MethodGet,
		Path:    "/events",
		Handler: http.HandlerFunc(sseServer.HTTPHandler),
	})
	engine.AddRoute(rest.Route{
		Method:  http.MethodPost,
		Path:    "/message",
		Handler: http.HandlerFunc(srv.HTTPHandler),
	})

	// Add a root handler for testing
	engine.AddRoute(rest.Route{
		Method: http.MethodGet,
		Path:   "/",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("go-zero MCP Server running. Use /events and /message."))
		},
	})

	// --- Start go-zero Server ---
	log.Printf("Starting go-zero HTTP+SSE MCP server on %s:%d...", c.Host, c.Port)
	engine.Start() // Blocks until interrupted
}

```

_(Note: This assumes a minimal `etc/config.yaml` exists for go-zero's `rest.RestConf`, e.g., specifying Host and Port)_

**To Run:** Navigate to `examples/http/go-zero` and run `go run main.go -f etc/config.yaml`. Clients connect as described in the `net/http` example (SSE to `/events`, POST to `/message`).

go-zero's `rest.Server` allows adding routes with standard `http.HandlerFunc` handlers, making integration straightforward.
