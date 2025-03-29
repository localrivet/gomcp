---
layout: default
title: Basic Usage
nav_order: 4 # After Installation
---

# Basic Usage

The core logic resides in the root package (`github.com/localrivet/gomcp`).

    ## Implementing an MCP Server

    This example shows the basic structure for creating an MCP server using the library's `Server` type. In a real application, you would add tool definitions and implement the logic within the `Run` loop or associated handlers.

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
    	// This handles the handshake and then listens for messages.
    	// The default Run implementation currently only handles handshake
    	// and returns errors for other message types.
    	err := server.Run()
    	if err != nil {
    		log.Fatalf("Server exited with error: %v", err)
    	}

    	log.Println("Server finished.")
    }
    ```
    *(See the `examples/server/` directory for a more complete server implementation with tool handling.)*

    ## Implementing an MCP Client

    This example shows the basic structure for creating an MCP client using the library's `Client` type and performing the handshake.

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
    	// After connecting, you would typically request tool definitions
    	// and then use the tools provided by the server.
    	// See the `examples/client/` directory for a more complete example.
    	//
    	// Example (Conceptual):
    	// toolDefs, err := requestToolDefinitions(client.Connection()) // Need access to conn
    	// result, err := useTool(client.Connection(), "echo", ...)
    	// ---------------------------------------------

    	// Close the client connection when done (optional for stdio)
    	err = client.Close()
    	if err != nil {
    		log.Printf("Error closing client: %v", err)
    	}

    	log.Println("Client finished.")
    }
    ```
    *(Note: This basic example only performs the handshake. See the `examples/client/` directory for a client that requests definitions and uses tools.)*
