---
title: Getting Started
weight: 10 # Position after main Docs index
---

This guide will walk you through the initial steps to get up and running with the `gomcp` library.

## Prerequisites

- **Go:** Ensure you have a recent version of Go installed (version 1.18 or later is recommended). You can download it from [golang.org](https://golang.org/dl/).

## Installation

First, you need to add the `gomcp` library to your Go project. Please follow the instructions on the [Installation]({{< ref "docs/installation" >}}) page.

## Your First MCP Server

Here's a minimal example of how to create a basic MCP server using the `stdio` transport (communicating over standard input/output):

```go
package main

import (
	"log"
	"os"

	"github.com/localrivet/gomcp/server"
)

func main() {
	// Configure logger (optional, defaults to stderr)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Minimal MCP Server...")

	// Create the core server instance
	// Provide a name and default options
	srv := server.NewServer("MyMinimalServer", server.ServerOptions{})

	// Register any tools here using srv.RegisterTool(...)
	// (See "Defining Tools" guide for details)

	// Start the server using the built-in stdio handler.
	// This blocks until the server exits (e.g., EOF on stdin or error).
	log.Println("Server setup complete. Listening on stdio...")
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server shutdown complete.")
}
```

**To run this:**

1.  Save the code as `main.go`.
2.  Run `go mod init my-simple-server` (if you haven't already initialized a module).
3.  Run `go mod tidy` to fetch dependencies.
4.  Run `go run main.go`.

The server will now listen for MCP JSON-RPC messages on standard input and send responses/notifications to standard output.

## Next Steps

- Explore the guides on implementing a [Server]({{< ref "docs/create-server" >}}) and [Client]({{< ref "docs/create-client" >}}).
- Learn how to define and register [Tools]({{< ref "docs/defining-tools" >}}).
- Check out the protocol details in the [Protocols]({{< ref "docs/protocols" >}}) section:
  - [Messages]({{< ref "docs/protocols/protocol_messages" >}})
  - [Tools]({{< ref "docs/protocols/protocol_tools" >}})
  - [Resources]({{< ref "docs/protocols/protocol_resources" >}})
  - [Prompts]({{< ref "docs/protocols/protocol_prompts" >}})
