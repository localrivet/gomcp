---
title: Basic (Stdio)
weight: 10 # First example
---

This page details the examples found in the `/examples/basic` directory, demonstrating fundamental server setup and tool registration using the `stdio` transport.

## Stdio Server (`examples/basic/stdio`)

This example shows the simplest way to run an MCP server, communicating over standard input and output.

**Key parts:**

```go
package main

import (
	"log"
	"os"

	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types"
)

func main() {
	// 1. Define server info
	serverInfo := types.Implementation{Name: "stdio-server", Version: "0.1.0"}
	// 2. Create server options
	opts := server.NewServerOptions(serverInfo)
	// 3. Create server instance
	srv := server.NewServer(opts)
	// 4. Create stdio transport
	transport := stdio.NewStdioTransport(os.Stdin, os.Stdout, nil)

	log.Println("Starting stdio MCP server...")
	// 5. Run the server
	if err := srv.Run(transport); err != nil {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server stopped.")
}
```

**To Run:** Navigate to `examples/basic/stdio` and run `go run main.go`.

## Basic Tool Server (`examples/basic/tools`)

This builds on the stdio server by defining and registering a simple "echo" tool.

**Key parts:**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types"
)

// Tool Handler
func handleEcho(ctx context.Context, args map[string]interface{}) ([]protocol.Content, error) {
	input, ok := args["text"].(string)
	if !ok { return nil, fmt.Errorf("missing 'text' argument") }
	return []protocol.Content{ protocol.TextContent{Type: "text", Text: "Echo: " + input} }, nil
}

func main() {
	serverInfo := types.Implementation{Name: "tool-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	opts.Capabilities.Tools = &protocol.ToolsCaps{} // Enable tool capability
	srv := server.NewServer(opts)

	// Tool Definition
	echoToolDef := protocol.Tool{
		Name: "echo", Description: "Echoes back text.",
		InputSchema: protocol.ToolInputSchema{ /* ... see full file ... */ },
	}

	// Register Tool
	if err := srv.RegisterTool(echoToolDef, handleEcho); err != nil { /* handle error */ }

	transport := stdio.NewStdioTransport(os.Stdin, os.Stdout, nil)
	log.Println("Starting tool server on stdio...")
	if err := srv.Run(transport); err != nil { /* handle error */ }
	log.Println("Server stopped.")
}
```

**To Run:** Navigate to `examples/basic/tools` and run `go run main.go`. Send a `tools/call` request for the `echo` tool via stdin.
