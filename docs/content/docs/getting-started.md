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
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types"
)

func main() {
	// Define server information
	serverInfo := types.Implementation{
		Name:    "my-simple-server",
		Version: "0.1.0",
	}

	// Create server options (using default logger)
	opts := server.NewServerOptions(serverInfo)

	// Create a new server instance
	srv := server.NewServer(opts)

	// Create a stdio transport
	transport := stdio.NewStdioTransport(os.Stdin, os.Stdout, nil) // Using default logger

	log.Println("Starting simple MCP server on stdio...")

	// Run the server with the transport
	// This will block until the transport closes (e.g., stdin is closed)
	if err := srv.Run(transport); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server stopped.")
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
