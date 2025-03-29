# GoMCP - Go Model Context Protocol Library

[![Go Reference](https://pkg.go.dev/badge/github.com/localrivet/gomcp.svg)](https://pkg.go.dev/github.com/localrivet/gomcp)

<!-- TODO: Add build status badge once CI is set up -->
<!-- TODO: Add code coverage badge once tests are added -->

`gomcp` provides a Go implementation of the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction), enabling communication between language models/agents and external tools or resources via a standardized protocol.

This library facilitates building MCP clients (applications that consume tools/resources) and MCP servers (applications that provide tools/resources). Communication primarily occurs over standard input/output using newline-delimited JSON messages.

**Current Status:** Alpha - Basic Handshake Implemented

The library currently supports the core MCP handshake process:

- Message Structure Definition (`protocol.go`)
- Stdio Transport (`transport.go`)
- Server Handshake Logic (`server.go`)
- Client Handshake Logic (`client.go`)

Support for Tool Definitions, Tool Usage, Resource Access, and Notifications is planned for future development.

## Installation

```bash
go get github.com/localrivet/gomcp
```

## Basic Usage

The core logic resides in the root package (`github.com/localrivet/gomcp`).

### Implementing an MCP Server

```go
package main

import (
	"log"
	"os"

	mcp "github.com/localrivet/gomcp"
)

func main() {
	// Configure logging (optional, library uses stderr by default)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	log.Println("Starting My MCP Server...")

	// Create a new server instance
	// The server name is sent during the handshake
	server := mcp.NewServer("MyGoMCPServer")

	// Run the server's main loop
	// This will handle the handshake and then (in the future)
	// listen for and process other MCP messages.
	// It blocks until an error occurs or the connection closes.
	err := server.Run()
	if err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server finished.")
}

```

### Implementing an MCP Client

```go
package main

import (
	"log"
	"os"

	mcp "github.com/localrivet/gomcp"
)

func main() {
	// Configure logging (optional)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	log.Println("Starting My MCP Client...")

	// Create a new client instance
	// The client name is sent during the handshake
	client := mcp.NewClient("MyGoMCPClient")

	// Perform the handshake with the server
	err := client.Connect()
	if err != nil {
		log.Fatalf("Client failed to connect: %v", err)
	}

	// Use the ServerName() method to get the discovered server name
	log.Printf("Client connected successfully to server: %s", client.ServerName())

	// --- TODO: Add logic to use the connection ---
	// Example (Conceptual - Requires further implementation):
	// toolDefs, err := client.RequestToolDefinitions()
	// if err != nil { ... }
	//
	// result, err := client.UseTool("my_tool", map[string]interface{}{"param": "value"})
	// if err != nil { ... }
	// log.Printf("Tool result: %v", result)
	// ---------------------------------------------

	// Close the client connection when done (optional for stdio)
	err = client.Close()
	if err != nil {
		log.Printf("Error closing client: %v", err)
	}

	log.Println("Client finished.")
}
```

_(Note: The client example includes conceptual calls like `RequestToolDefinitions` and `UseTool` which are not yet implemented but illustrate intended usage.)_

## Example Executables

The `cmd/` directory contains simple example programs demonstrating the use of the library:

- `cmd/mcp-server`: A basic server that performs the handshake.
- `cmd/mcp-client`: A basic client that performs the handshake.

You can test the handshake by running them connected via a pipe:

```bash
go run ./cmd/mcp-server/main.go | go run ./cmd/mcp-client/main.go
```

_(Check the log output on stderr for details)_

## Documentation

More detailed documentation can be found in the `docs/` directory (TODO: Create docs directory and add content).

Go package documentation is available via:

- [pkg.go.dev](https://pkg.go.dev/github.com/localrivet/gomcp)
- Running `godoc -http=:6060` locally and navigating to `github.com/localrivet/gomcp`.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request. (TODO: Add CONTRIBUTING.md)

## License

This project is licensed under the [MIT License](LICENSE). (TODO: Add LICENSE file)
