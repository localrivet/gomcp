---
title: Defining Server Tools
weight: 40
---

One of the core features of MCP is the ability for servers to expose "tools" that clients can execute. This guide explains how to define and register tools in your `gomcp` server application.

## 1. Define the Tool Structure (`protocol.Tool`)

First, you need to define the metadata for your tool using the `protocol.Tool` struct. This includes:

- **`Name`**: A unique string identifier for the tool (e.g., "calculate_sum", "read_file").
- **`Description`**: (Optional) A human-readable description of what the tool does.
- **`InputSchema`**: A `protocol.ToolInputSchema` defining the expected arguments. This uses a subset of JSON Schema:
  - `Type`: Should typically be "object".
  - `Properties`: A map where keys are argument names and values are `protocol.PropertyDetail` structs describing the argument's `Type` (e.g., "string", "number", "boolean", "array", "object"), `Description`, allowed `Enum` values, or `Format`.
  - `Required`: A slice of strings listing the names of mandatory arguments.
- **`Annotations`**: (Optional) A `protocol.ToolAnnotations` struct providing hints about the tool's behavior (e.g., `ReadOnlyHint`, `DestructiveHint`).

**Example: An "echo" tool definition**

```go
import "github.com/localrivet/gomcp/protocol"

var echoTool = protocol.Tool{
	Name:        "echo",
	Description: "Simple tool that echoes back the input text.",
	InputSchema: protocol.ToolInputSchema{
		Type: "object",
		Properties: map[string]protocol.PropertyDetail{
			"input": {
				Type:        "string",
				Description: "The text to be echoed back by the server.",
			},
			"prefix": {
				Type:        "string",
				Description: "An optional prefix to add to the echoed text.",
				// Note: Not listed in 'Required', so it's optional.
			},
		},
		Required: []string{"input"}, // Only 'input' is mandatory
	},
	Annotations: protocol.ToolAnnotations{
		// Example annotation: This tool doesn't modify state
		ReadOnlyHint: func(b bool) *bool { return &b }(true),
	},
}
```

## 2. Implement the Tool Handler Function (`server.ToolHandlerFunc`)

Next, create a function that implements the actual logic of your tool. This function must match the `server.ToolHandlerFunc` signature:

```go
type ToolHandlerFunc func(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool)
```

- It receives a `context.Context` for cancellation/deadlines.
- It receives an optional `*protocol.ProgressToken` if the client requested progress updates for this call.
- It receives the `arguments` provided by the client in the `tools/call` request as an `any` type. You need to parse and validate these arguments, ideally using the `schema.HandleArgs` helper function (see below).
- It returns a slice of `protocol.Content` objects (e.g., `protocol.TextContent`, `protocol.ImageContent`) representing the result of the tool execution.
- It returns a boolean `isError`. If `true`, the returned `content` is treated as error information (e.g., a `TextContent` explaining the error). If `false`, the `content` is the successful result. The `schema.HandleArgs` helper can generate appropriate error content automatically for invalid arguments.

**Example: Handler for the "echo" tool**

```go
package main

import (
	"context"
	"log" // Added for logging

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/util/schema"
)

// Define arguments struct for the echo tool
type EchoArgs struct {
	Input  string `json:"input" description:"The text to be echoed back by the server."`
	Prefix string `json:"prefix,omitempty" description:"An optional prefix to add to the echoed text."` // Use omitempty for optional
}

// Example: Handler for the "echo" tool using schema.HandleArgs
func handleEchoTool(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	// Use schema.HandleArgs to parse and validate arguments against the EchoArgs struct.
	// It automatically handles type checking, required fields, and generates error content.
	args, errContent, isErr := schema.HandleArgs[EchoArgs](arguments)
	if isErr {
		log.Printf("Error handling echo args: %v", errContent)
		return errContent, true // Return the error content from HandleArgs
	}

	// If parsing succeeded, 'args' is a populated EchoArgs struct.
	log.Printf("Executing echo tool with input: '%s', prefix: '%s'", args.Input, args.Prefix)

	// Construct the result message
	resultText := args.Prefix + args.Input

	// Return the result as TextContent
	result := []protocol.Content{
		protocol.TextContent{
			Type: "text", // Always specify the content type
			Text: resultText,
		},
	}
	return result, false // Return false for isError on success
}

```

## 3. Register the Tool with the Server

Finally, use the `RegisterTool` method on your `server.Server` instance _before_ running the server. Pass the `protocol.Tool` definition and the corresponding handler function.

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

// Assume echoTool definition (protocol.Tool) and
// handleEchoTool (server.ToolHandlerFunc) are defined as above.

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

	// Start the server using stdio
	log.Println("Starting server on stdio...")
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
	log.Println("Server stopped.")
}

```

Now, when a client connects and sends a `tools/call` request for the "echo" tool with valid arguments, the `handleEchoTool` function will be executed, and its result will be sent back to the client. If the client sends a `tools/list` request, the `echoTool` definition will be included in the response.
