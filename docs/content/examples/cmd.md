---
title: Command-Line (CLI)
weight: 70 # Seventh example
---

This page details the example found in the `/examples/cmd` directory, demonstrating how to build command-line interface (CLI) applications that function as MCP servers or clients.

This is useful for creating standalone tools that can be invoked from the terminal and communicate using MCP, often over the `stdio` transport.

## CLI Server (`examples/cmd/server`)

This example likely uses the standard Go `flag` package or a library like [Cobra](https://github.com/spf13/cobra) to parse command-line arguments and then starts an MCP server (probably using `stdio` transport) based on those arguments.

**Conceptual Snippets (Illustrative - check `main.go` for actual implementation):**

```go
package main

import (
	"flag" // Or "github.com/spf13/cobra"
	"log"
	"os"
	// ... other imports: server, protocol, types, stdio ...
	"github.com/localrivet/gomcp/types" // Added for types.Transport
)

func main() {
	// --- Define Command-Line Flags ---
	// Example using standard 'flag' package
	port := flag.Int("port", 0, "Port to listen on (if using TCP/WebSocket transport, 0 for stdio)")
	serverName := flag.String("name", "cli-server", "Name of the server")
	// Add flags for tool definitions, config files, etc.
	flag.Parse() // Parse the flags

	// --- Setup MCP Server ---
	serverInfo := types.Implementation{Name: *serverName, Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	// Configure capabilities based on flags...
	srv := server.NewServer(opts)
	// Register tools based on flags or config files...

	// --- Choose Transport Based on Flags ---
	var transport types.Transport
	if *port > 0 {
		// Setup TCP or WebSocket transport on the specified port (example)
		// log.Printf("Starting TCP server on port %d", *port)
		// transport = tcp.NewFactory(srv, opts.Logger).Listen(fmt.Sprintf(":%d", *port)) // Hypothetical
		log.Fatalf("TCP/WebSocket transport not fully implemented in this snippet")
	} else {
		log.Println("Starting stdio MCP server...")
		transport = stdio.NewStdioTransport(os.Stdin, os.Stdout, opts.Logger)
	}

	// --- Run the Server ---
	if err := srv.Run(transport); err != nil {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server stopped.")
}
```

**To Run:**

1. Navigate to `examples/cmd/server`.
2. Build the binary: `go build -o my-mcp-server`
3. Run it:
   - `./my-mcp-server` (Uses stdio transport by default)
   - `./my-mcp-server -name "My Custom Server"` (Overrides server name)
   - _(Add other flags as defined in the actual `main.go`)_

This pattern allows creating flexible MCP servers or clients whose behavior can be controlled via command-line arguments.
