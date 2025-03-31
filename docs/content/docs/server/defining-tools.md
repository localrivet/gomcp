---
title: Defining and Registering Tools
weight: 10 # First item in Server section
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
type ToolHandlerFunc func(ctx context.Context, arguments map[string]interface{}) (result []protocol.Content, err error)
```

- It receives a `context.Context` for cancellation/deadlines.
- It receives the `arguments` provided by the client in the `tools/call` request as a `map[string]interface{}`. You'll need to perform type assertions to access argument values safely.
- It returns a slice of `protocol.Content` objects (e.g., `protocol.TextContent`, `protocol.ImageContent`) representing the successful result of the tool execution.
- It returns an `error` if the tool execution fails. This error will be converted into a JSON-RPC error response sent back to the client.

**Example: Handler for the "echo" tool**

```go
import (
	"context"
	"fmt"
	"github.com/localrivet/gomcp/protocol"
)

func handleEchoTool(ctx context.Context, args map[string]interface{}) ([]protocol.Content, error) {
	// Safely get the required 'input' argument
	inputText, ok := args["input"].(string)
	if !ok || inputText == "" {
		// Return an error if 'input' is missing, empty, or wrong type
		return nil, fmt.Errorf("required argument 'input' (string) is missing or invalid")
	}

	// Safely get the optional 'prefix' argument
	prefixText, _ := args["prefix"].(string) // Ignore error if missing/wrong type, default to ""

	// Construct the result message
	resultText := prefixText + inputText

	// Return the result as TextContent
	result := []protocol.Content{
		protocol.TextContent{
			Type: "text", // Always specify the content type
			Text: resultText,
		},
	}
	return result, nil // Return nil error on success
}
```

## 3. Register the Tool with the Server

Finally, use the `RegisterTool` method on your `server.Server` instance _before_ running the server. Pass the `protocol.Tool` definition and the corresponding handler function.

```go
import (
	"log"
	"github.com/localrivet/gomcp/server"
	// ... other imports
)

func main() {
	// ... (Initialize serverInfo, opts, srv as shown in server.md) ...

	// Register the echo tool and its handler
	err := srv.RegisterTool(echoTool, handleEchoTool)
	if err != nil {
		log.Fatalf("Failed to register tool '%s': %v", echoTool.Name, err)
	}
	log.Printf("Registered tool: %s", echoTool.Name)

	// ... (Create transport and run server) ...
}
```

Now, when a client connects and sends a `tools/call` request for the "echo" tool with valid arguments, the `handleEchoTool` function will be executed, and its result will be sent back to the client. If the client sends a `tools/list` request, the `echoTool` definition will be included in the response.
