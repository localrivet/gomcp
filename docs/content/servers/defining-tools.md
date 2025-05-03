---
title: Tools
weight: 20
---

Tools allow clients (like LLMs) to perform actions by executing functions on your server. Use `server.AddTool` for easy registration with automatic schema generation based on a Go struct for arguments.

```go
package main

import (
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	// ... other imports
)

// Define the arguments struct for your tool
type ReverseArgs struct {
	Input string `json:"input" description:"The string to reverse" required:"true"`
}

// Your tool handler function
func reverseStringHandler(args ReverseArgs) (protocol.Content, error) {
	runes := []rune(args.Input)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	reversed := string(runes)
	return server.Text(reversed), nil // Helper for simple text responses
}

func registerTools(srv *server.Server) error {
	// Register the tool using the helper
	err := server.AddTool(
		srv,
		"reverse_string",
		"Reverses the input string.",
		reverseStringHandler, // Pass the handler function
	)
	return err
}
```

Alternatively, you can provide a `protocol.ToolDefinition` manually for more control.

#### Alternative: Using `RegisterTool` and `schema.FromStruct`

For more direct control over the tool definition and handler signature, you can use `srv.RegisterTool` and explicitly generate the schema using `schema.FromStruct` from the `util/schema` package.

This approach requires you to manually handle argument parsing within your handler function, which receives the raw `protocol.ToolCall`.

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/util/schema" // Import schema helper
	// ... other imports
)

// Define the arguments struct (still useful for schema generation)
type MultiplyArgs struct {
	X float64 `json:"x" description:"Multiplicand" required:"true"`
	Y float64 `json:"y" description:"Multiplier" required:"true"`
}

// Handler function for RegisterTool takes protocol.ToolCall
func multiplyHandler(call protocol.ToolCall) (protocol.ToolResult, error) {
	var args MultiplyArgs
	// Manually parse arguments
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		log.Printf("[multiplyHandler] Error parsing args: %v", err)
		// Return a structured error result
		return protocol.ToolResult{IsError: true, Content: []protocol.Content{server.Text(fmt.Sprintf("Invalid arguments: %v", err))}}, nil
	}

	result := args.X * args.Y
	log.Printf("[multiply tool] %f * %f -> %f", args.X, args.Y, result)

	// Return a successful result
	return protocol.ToolResult{Content: []protocol.Content{server.Text(fmt.Sprintf("%f", result))}}, nil
}


func registerToolsManually(srv *server.Server) {
	// Register the tool using RegisterTool
	srv.RegisterTool(
		protocol.Tool{ // Define the full Tool struct
			Name:        "multiply",
			Description: "Multiplies two numbers.",
			// Generate schema explicitly
			InputSchema: schema.FromStruct(MultiplyArgs{}),
			// OutputSchema can also be defined here if needed
		},
		multiplyHandler, // Pass the handler with the (ToolCall) signature
	)
	// Note: Error handling for RegisterTool might differ or be absent
	// depending on the implementation version or desired behavior.
	// Check server implementation details if needed.
}
```

Servers can send the `notifications/tools/list_changed` notification to inform clients that the list of available tools has changed and they should re-list if they need the updated list.

- **Method:** `"notifications/tools/list_changed"`
- **Parameters:** `protocol.ToolsListChangedParams` (currently empty)

```go
type ToolsListChangedParams struct{} // Currently empty
```

This notification does not include the updated list itself, only signals that a change has occurred. Clients must send a `tools/list` request to get the new list.

## 3. Register the Tool with the Server

Finally, use the `RegisterTool` method on your `server.Server` instance _before_ running the server. Pass the `protocol.Tool` definition and the corresponding handler function.

```go
import (
	"context"
	"log"
	"os"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/util/schema"
)

// Assume echoTool definition (protocol.Tool) and
// handleEchoTool (hooks.FinalToolHandler) are defined as above.

func main() {
	// Configure logger
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	// Create the server instance
	srv := server.NewServer("MyToolServer", server.ServerOptions{})

	// Register the echo tool and its handler
	err := srv.RegisterTool(echoTool, handleEchoTool)
	if err != nil {
		log.Fatalf("Failed to register tool '%s': %v", echoTool.Name, err)
	}
	log.Printf("Registered tool: %s", echoTool.Name)

	// Start the server using stdio (or another transport)
	log.Println("Starting server on stdio...")
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
	log.Println("Server stopped.")
}
```

Now, when a client connects and sends a `tools/call` request for the "echo" tool with valid arguments, the `handleEchoTool` function will be executed, and its result will be sent back to the client. If the client sends a `tools/list` request, the `echoTool` definition will be included in the response.
