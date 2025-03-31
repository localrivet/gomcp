---
title: Kitchen Sink
weight: 80 # Eighth example
---

This page details the example found in the `/examples/kitchen-sink` directory. As the name suggests, this example aims to demonstrate a wide variety of `gomcp` features working together in a single server application.

This includes:

- Multiple transport options (e.g., Stdio, HTTP+SSE, WebSocket) selectable via flags.
- Registration of multiple tools with different functionalities.
- Registration of resources and prompts.
- Handling of configuration, logging, and potentially other aspects.

## Kitchen Sink Server (`examples/kitchen-sink/server`)

This example likely combines elements from many of the other examples, using command-line flags to configure which transport to use and potentially which features to enable.

**Conceptual Snippets (Illustrative - check `main.go` for actual implementation):**

```go
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	// ... many other imports: server, protocol, types, stdio, sse, websocket, viper ...
	"github.com/localrivet/gomcp/protocol" // Added for protocol types
	"github.com/localrivet/gomcp/server"   // Added for server types
	"github.com/localrivet/gomcp/types"    // Added for types.Implementation
)

// ... Define multiple tool handlers (handleEcho, handleCalc, handleReadFile, etc.) ...
// ... Define resource providers ...

func main() {
	// --- Command-Line Flags ---
	transportType := flag.String("transport", "stdio", "Transport type: stdio, http, websocket")
	listenAddr := flag.String("listen", ":8080", "Listen address for http/websocket")
	// ... other flags ...
	flag.Parse()

	// --- Load Configuration (Optional, e.g., using Viper) ---
	// ... viper setup ...

	// --- Setup MCP Server ---
	serverInfo := types.Implementation{Name: "kitchen-sink", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	// Set capabilities based on features being enabled
	opts.Capabilities.Tools = &protocol.ToolsCaps{ /* ... */ }
	opts.Capabilities.Resources = &protocol.ResourcesCaps{ /* ... */ }
	// ... etc ...
	srv := server.NewServer(opts)

	// --- Register Multiple Capabilities ---
	log.Println("Registering capabilities...")
	// Register echo tool
	// err := srv.RegisterTool(echoToolDef, handleEcho) ...
	// Register calculator tool
	// err = srv.RegisterTool(calcToolDef, handleCalc) ...
	// Register file reader tool
	// err = srv.RegisterTool(readFileToolDef, handleReadFile) ...
	// Register resources
	// err = srv.RegisterResource(fileResource, fileProvider) ...
	// Register prompts
	// err = srv.RegisterPrompt(examplePrompt) ...

	// --- Setup and Run Selected Transport ---
	log.Printf("Starting kitchen-sink server using %s transport...", *transportType)
	var runErr error

	switch *transportType {
	case "http":
		sseServer := sse.NewServer(srv, opts.Logger)
		mux := http.NewServeMux()
		mux.HandleFunc("/events", sseServer.HTTPHandler)
		mux.HandleFunc("/message", srv.HTTPHandler)
		log.Printf(" Listening on %s", *listenAddr)
		runErr = http.ListenAndServe(*listenAddr, mux)
	case "websocket":
		wsFactory := websocket.NewFactory(srv, opts.Logger)
		mux := http.NewServeMux()
		mux.HandleFunc("/ws", wsFactory.HTTPHandler)
		log.Printf(" Listening on %s/ws", *listenAddr)
		runErr = http.ListenAndServe(*listenAddr, mux)
	case "stdio":
		fallthrough // Default to stdio
	default:
		transport := stdio.NewStdioTransport(os.Stdin, os.Stdout, opts.Logger)
		runErr = srv.Run(transport)
	}

	// --- Handle Run Error ---
	if runErr != nil {
		log.Fatalf("Server error: %v", runErr)
	}
	log.Println("Server stopped.")
}
```

**To Run:**

1. Navigate to `examples/kitchen-sink/server`.
2. Build: `go build -o kitchen-sink-server`
3. Run with desired transport:
   - `./kitchen-sink-server -transport stdio`
   - `./kitchen-sink-server -transport http -listen :8080`
   - `./kitchen-sink-server -transport websocket -listen :9090`

This example serves as a comprehensive reference for integrating various `gomcp` components. Consult the `main.go` file for the full implementation details.
