---
title: Examples
weight: 80
cascade:
  type: docs
---

This section contains runnable examples demonstrating various features and use cases of the `gomcp` library.

You can find the full source code for these and more complex examples in the [`/examples`](https://github.com/localrivet/gomcp/tree/main/examples) directory of the repository.

## Basic Stdio Server

This example sets up a minimal server communicating over standard input/output.

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
	serverInfo := types.Implementation{Name: "stdio-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	srv := server.NewServer(opts)
	transport := stdio.NewStdioTransport(os.Stdin, os.Stdout, nil)

	log.Println("Starting stdio MCP server...")
	if err := srv.Run(transport); err != nil {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server stopped.")
}
```

_(See `examples/basic/stdio/main.go` for the full runnable version)_

## Registering a Simple Tool

Here's how you define and register a basic "echo" tool within your server setup.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	// ... other necessary imports from basic server example ...
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types"
)

// 1. Define the Tool Handler
func handleEcho(ctx context.Context, args map[string]interface{}) ([]protocol.Content, error) {
	input, ok := args["text"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'text' argument")
	}
	return []protocol.Content{
		protocol.TextContent{Type: "text", Text: "Server Echo: " + input},
	}, nil
}

func main() {
	serverInfo := types.Implementation{Name: "tool-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	// Indicate tool support
	opts.Capabilities.Tools = &protocol.ToolsCaps{}
	srv := server.NewServer(opts)

	// 2. Define the Tool Structure
	echoToolDef := protocol.Tool{
		Name:        "echo",
		Description: "Echoes back the provided text.",
		InputSchema: protocol.ToolInputSchema{
			Type: "object",
			Properties: map[string]protocol.PropertyDetail{
				"text": {Type: "string", Description: "Text to echo"},
			},
			Required: []string{"text"},
		},
	}

	// 3. Register the Tool
	if err := srv.RegisterTool(echoToolDef, handleEcho); err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}
	log.Println("Registered 'echo' tool.")

	transport := stdio.NewStdioTransport(os.Stdin, os.Stdout, nil)
	log.Println("Starting tool MCP server on stdio...")
	if err := srv.Run(transport); err != nil {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server stopped.")
}

```

_(See `examples/basic/tools/main.go` for the full runnable version)_

Explore the other examples in the repository for more advanced scenarios involving different transports (HTTP/SSE, WebSockets), authentication, resource providers, and more.
