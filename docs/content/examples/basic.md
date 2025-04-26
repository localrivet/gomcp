---
title: Basic (Stdio Multi-Tool)
weight: 10 # First example
---

This page details the example found in the `/examples/basic` directory. It demonstrates a fundamental MCP server setup using the `stdio` transport, featuring multiple registered tools (`echo`, `calculator`, `filesystem`), and a corresponding client that interacts with these tools.

## Basic Multi-Tool Server (`examples/basic/server`)

This example showcases a server communicating over standard input/output and managing several distinct tools.

**Key parts:**

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/util/schema"
)

// Define arguments for the echo tool
type EchoArgs struct {
	Message string `json:"message" description:"The message to echo."`
}

// Handler for the echo tool
func echoHandler(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	args, errContent, isErr := schema.HandleArgs[EchoArgs](arguments)
	if isErr {
		log.Printf("Error handling echo args: %v", errContent)
		return errContent, true
	}
	log.Printf("Executing echo tool with message: %s", args.Message)
	// Note: The actual example prepends "Echo: " in its response
	return []protocol.Content{protocol.TextContent{Type: "text", Text: args.Message}}, false
}

// (Handlers for calculator and filesystem tools are also defined in the full example)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Basic Multi-Tool MCP Server...")

	// Create the core server instance
	srv := server.NewServer("GoMultiToolServer", server.ServerOptions{})

	// Define the echo tool
	echoTool := protocol.Tool{
		Name:        "echo",
		Description: "Echoes back the provided message.",
		InputSchema: schema.FromStruct(EchoArgs{}), // Generate schema from struct
	}
	// Register the echo tool
	if err := srv.RegisterTool(echoTool, echoHandler); err != nil {
		log.Fatalf("Failed to register echo tool: %v", err)
	}

	// (Calculator and Filesystem tools are also registered in the full example)

	log.Println("Server setup complete. Listening on stdio...")
	// Start the server using the built-in stdio handler.
	// This blocks until the server exits (e.g., EOF on stdin or error).
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
	log.Println("Server shutdown complete.")
}
```

## Basic Client (`examples/basic/client`)

The corresponding client connects to the server via stdio, lists the available tools, and then calls each tool (`echo`, `calculator`, `filesystem`) with various arguments to demonstrate interaction and error handling. See the `examples/basic/client/main.go` file for the full implementation.

## Running the Example

The server and client are designed to be connected via standard input/output.

1.  Navigate to the main `examples` directory in your terminal.
2.  Run the following command to pipe the client's output to the server's input and vice-versa:

    ```bash
    (cd basic/server && go run .) | (cd basic/client && go run .)
    ```

You will see log output from both the server and the client in your terminal, showing the handshake, tool listing, and results of each tool call.
