---
title: Calling Server Tools
weight: 60
---

Once your MCP client is connected to a server, you can execute the tools the server offers using the `tools/call` request. The `gomcp/client` package provides the `CallTool` method to simplify this process.

## Steps to Call a Tool

1.  **Identify the Tool:** Know the `Name` of the tool you want to call (e.g., by using `ListTools` first).
2.  **Prepare Arguments:** Construct the arguments required by the tool, matching its defined `InputSchema`. This can be a `map[string]interface{}` or a struct that marshals to the expected JSON object.
3.  **Create `CallToolParams`:** Populate a `protocol.CallToolParams` struct with the tool `Name` and the prepared `Arguments`.
4.  **(Optional) Request Progress:** If the tool supports progress reporting and the server advertises the capability, you can generate a unique `protocol.ProgressToken` and include it in the `Meta` field of `CallToolParams`. You also need to register a handler for `$/progress` notifications _before_ connecting.
5.  **Call `client.CallTool`:** Invoke the `CallTool` method on your connected `client.Client` instance, passing a `context.Context` (for cancellation/timeout of the specific call), the `CallToolParams`, and the progress token (if generated).
6.  **Handle Result/Error:**
    - If `CallTool` returns a non-nil `error`, it indicates a protocol-level issue (e.g., connection lost, invalid response format).
    - If `error` is `nil`, check the `IsError` field within the returned `protocol.CallToolResult`. If `true`, the tool execution failed on the server, and the `Content` field likely contains error details (e.g., a `TextContent` message).
    - If `IsError` is `false` (or nil), the call was successful, and the `Content` field contains the tool's output (e.g., `TextContent`, `ImageContent`).

## Example: Calling an "echo" Tool

This example assumes a `client.Client` (`clt`) has already been created and connected to a server offering an "echo" tool.

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
	// "github.com/google/uuid" // Needed if generating progress tokens
)

func main() {
	// Configure logger
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Tool Calling Client...")

	// Create a stdio client (replace with NewClient for network)
	clt, err := client.NewStdioClient("MyToolCaller", client.ClientOptions{})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Connect to the server (assumes a compatible server is running)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = clt.Connect(ctx)
	if err != nil {
		log.Fatalf("Client failed to connect: %v", err)
	}
	defer clt.Close()
	log.Printf("Connected to server: %s", clt.ServerInfo().Name)

	// --- Call a Tool ---

	// 1. Define the arguments for the tool call.
	//    This should match the tool's InputSchema.
	toolArgs := map[string]interface{}{
		"input":  "Hello from client!",
		"prefix": "[MCP] ",
	}

	// 2. Create the CallToolParams struct.
	callParams := protocol.CallToolParams{
		Name:      "echo", // The name of the tool to call
		Arguments: toolArgs,
		// Meta:      nil, // Optional: Include metadata like progress token
	}

	// Example: Requesting progress updates (optional)
	// var progressToken *protocol.ProgressToken
	// if clt.ServerCapabilities().Tools != nil && clt.ServerCapabilities().Tools.Progress {
	// 	token := protocol.ProgressToken(uuid.NewString())
	// 	progressToken = &token
	// 	callParams.Meta = &protocol.RequestMeta{ProgressToken: progressToken}
	// 	// Need to register a handler for $/progress notifications *before* connecting
	// 	// clt.RegisterNotificationHandler(protocol.MethodProgress, handleProgress)
	// 	log.Printf("Requesting progress with token: %s", *progressToken)
	// }

	// 3. Call the tool using the client's CallTool method.
	//    Provide a context for the specific call.
	callCtx, callCancel := context.WithTimeout(ctx, 15*time.Second)
	defer callCancel()

	log.Printf("Calling tool '%s' with args: %+v", callParams.Name, callParams.Arguments)
	result, err := clt.CallTool(callCtx, callParams, progressToken) // Pass progress token if created

	// 4. Handle the result or error.
	if err != nil {
		// This indicates a protocol-level error (e.g., connection issue, invalid response)
		log.Printf("ERROR calling tool '%s': %v", callParams.Name, err)
	} else {
		// Check the IsError flag in the result for application-level errors
		log.Printf("Tool '%s' call finished. IsError: %v", callParams.Name, result.IsError)
		log.Printf("  Result Content (%d items):", len(result.Content))
		for i, contentItem := range result.Content {
			log.Printf("  - [%d] Type: %s", i, contentItem.ContentType())
			// Process specific content types (e.g., TextContent)
			if textContent, ok := contentItem.(protocol.TextContent); ok {
				log.Printf("      Text: %s", textContent.Text)
			}
			// Add checks for other types like ImageContent, JSONContent, etc.
		}
	}

	log.Println("Client finished.")
}

// Example progress handler (if needed)
// func handleProgress(ctx context.Context, params any) error {
// 	progressParams, ok := params.(protocol.ProgressParams)
// 	if !ok {
// 		log.Printf("WARN: Received progress notification with unexpected param type: %T", params)
// 		return nil
// 	}
// 	log.Printf("PROGRESS [%s]: %v", progressParams.Token, progressParams.Value)
// 	return nil
// }
```
