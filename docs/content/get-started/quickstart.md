---
title: 'Quickstart'
weight: 30
---

Let's create a simple MCP server that exposes a calculator tool and a client that uses it, communicating over stdio.

**Server (`calculator_server/main.go`)**

```go
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// Arguments for the 'add' tool
type AddArgs struct {
	A float64 `json:"a" description:"First number" required:"true"`
	B float64 `json:"b" description:"Second number" required:"true"`
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[CalcServer] ")
	log.Println("Starting Calculator MCP Server (Stdio)...")

	// Create the MCP server instance
	srv := server.NewServer("calculator-stdio")

	// Add the 'add' tool using the server.AddTool helper
	// The helper infers the schema from the handler's argument type (AddArgs)
	err := server.AddTool(
		srv,
		"add",
		"Add two numbers.",
		// Handler function using the args struct
		func(args AddArgs) (protocol.Content, error) {
			result := args.A + args.B
			log.Printf("[add tool] %f + %f -> %f", args.A, args.B, result)
			// Use server.Text helper for simple text responses
			return server.Text(strconv.FormatFloat(result, 'f', -1, 64)), nil
		},
	)
	if err != nil {
		log.Fatalf("Failed to add 'add' tool: %v", err)
	}

	// Start the server using the built-in stdio handler.
	// This blocks until the server exits (e.g., stdin is closed).
	log.Println("Server setup complete. Listening on stdio...")
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
	log.Println("Server shutdown complete.")
}

```

**Client (`calculator_client/main.go`)**

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[CalcClient] ")
	log.Println("Starting Calculator MCP Client (Stdio)...")

	// Create a client configured for stdio communication
	// Point it to the server executable (adjust path as needed)
	// For simple Go examples, you might run the server first and then the client.
	// If running separately, use the actual command to start the server:
	// cmd := []string{"go", "run", "../calculator_server/main.go"}
	// clt, err := client.NewStdioClientWithCommand("MyCalcClient", client.ClientOptions{}, cmd)
	// For simplicity here, assume server is already running and connected to stdin/stdout
	clt, err := client.NewStdioClient("MyCalcClient", client.ClientOptions{})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Set a timeout for the connection and operations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect and perform initialization handshake
	log.Println("Connecting to server via stdio...")
	if err = clt.Connect(ctx); err != nil {
		log.Fatalf("Client failed to connect: %v", err)
	}
	defer clt.Close() // Ensure connection resources are cleaned up

	serverInfo := clt.ServerInfo()
	log.Printf("Connected to server: %s (Version: %s)", serverInfo.Name, serverInfo.Version)

	// Call the 'add' tool
	log.Println("--- Calling 'add' Tool ---")
	addArgs := map[string]interface{}{"a": 15.5, "b": 4.5}
	callParams := protocol.CallToolParams{Name: "add", Arguments: addArgs}

	callCtx, callCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer callCancel()

	callResult, err := clt.CallTool(callCtx, callParams, nil) // No progress token needed
	if err != nil {
		log.Printf("Error calling tool 'add': %v", err)
	} else if callResult.IsError {
		log.Printf("Tool 'add' call returned an error:")
		for _, content := range callResult.Content {
			if textContent, ok := content.(protocol.TextContent); ok {
				log.Printf("  Error Content: %s", textContent.Text)
			}
		}
	} else {
		log.Printf("Tool 'add' call successful:")
		for _, content := range callResult.Content {
			if textContent, ok := content.(protocol.TextContent); ok {
				log.Printf("  Result: %s", textContent.Text) // Expecting "20"
			}
		}
	}

	log.Println("Client finished.")
}
```

**Running the Quickstart:**

1.  Save the server code as `calculator_server/main.go`.
2.  Save the client code as `calculator_client/main.go`.
3.  Compile and run the server: `go run calculator_server/main.go`
4.  In **another terminal**, compile and run the client: `go run calculator_client/main.go`

The client will connect to the server via stdio, call the `add` tool, print the result, and exit.
