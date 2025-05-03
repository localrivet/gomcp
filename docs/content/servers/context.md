---
title: Context
weight: 50
---

When implementing handlers for tools, resources, or notifications in your GoMCP server, you often need access to contextual information about the client session making the request, the server itself, or the request's lifecycle (like cancellation). The GoMCP library provides this context primarily through the standard `context.Context` object and the `types.ClientSession` interface passed to your handlers.

## Accessing Client Session Information

Handlers for requests and notifications receive a `context.Context` and, for most request types, a `types.ClientSession` object. The `types.ClientSession` interface represents the active connection from a single client and provides methods to interact with that specific client and retrieve session-specific details.

```go
package main

import (
	"context"
	"log"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/types" // Import the types package
)

// Example Tool Handler demonstrating access to ClientSession
func handleMyTool(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	// Access the ClientSession from the context
	// The server automatically adds the session to the context for request handlers.
	session, ok := types.SessionFromContext(ctx)
	if !ok {
		// This check is a safeguard; in standard request handlers, the session should be present.
		log.Println("Error: ClientSession not found in context.")
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Internal server error: Session context missing."}}, true
	}

	// Get session ID
	sessionID := session.SessionID()
	log.Printf("Tool 'myTool' called by session: %s", sessionID)

	// Get client capabilities advertised during initialization
	clientCaps := session.GetClientCapabilities()
	log.Printf("Client capabilities for session %s: %+v", sessionID, clientCaps)

	// Get the negotiated protocol version for this session
	negotiatedVersion := session.GetNegotiatedVersion()
	log.Printf("Negotiated protocol version for session %s: %s", sessionID, negotiatedVersion)

	// You can use the session object to send messages back to this specific client
	// For example, sending a custom notification:
	// customNotificationParams := map[string]string{"status": "processing"}
	// notification := protocol.JSONRPCNotification{Method: "myServer/statusUpdate", Params: customNotificationParams}
	// if err := session.SendNotification(notification); err != nil {
	//     log.Printf("Failed to send status update notification to session %s: %v", sessionID, err)
	// }


	// ... rest of your tool handler logic ...

	return []protocol.Content{protocol.TextContent{Type: "text", Text: "Tool executed successfully."}}, false
}
```

The `types.ClientSession` interface provides essential methods for session management and communication:

- `SessionID() string`: Returns a unique identifier string for the client session.
- `GetClientCapabilities() protocol.ClientCapabilities`: Returns the capabilities object sent by the client in the `initialize` request.
- `GetNegotiatedVersion() string`: Returns the protocol version that was successfully negotiated during the initialization handshake.
- `SendNotification(notification protocol.JSONRPCNotification) error`: Allows the server to send an asynchronous JSON-RPC notification to this specific client session.
- `SendResponse(response protocol.JSONRPCResponse) error`: Allows the server to send a JSON-RPC response to a request. While the core server handles most responses automatically, this method is available for advanced use cases.
- `Close() error`: Terminates the connection to this client session.

## Logging

The `server.Server` instance is typically configured with a logger (`types.Logger`). You should use this logger within your handlers for consistent and structured logging of server activity, tool execution, and errors. The logger is usually accessible via the `types.SessionFromContext` or by passing the logger instance down to your handlers.

```go
package main

import (
	"context"
	"log" // Using standard log for simplicity in this example, but use types.Logger in practice

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/types"
)

// Assume your server is initialized with a logger:
// srv := server.NewServer("MyServer", server.ServerOptions{Logger: myCustomLogger})

// Example Tool Handler demonstrating logging
func handleAnotherTool(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	session, ok := types.SessionFromContext(ctx)
	if !ok {
		log.Println("Error: ClientSession not found in context.")
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Internal server error: Session context missing."}}, true
	}

	// Access the logger associated with the session (assuming it's available via context or session)
	// In a real implementation, you might pass the logger to the handler closure or access it differently.
	// For illustrative purposes, let's assume a logger is available:
	// logger := getLoggerFromSomewhere() // Replace with actual logger access

	log.Printf("Executing 'anotherTool' for session %s", session.SessionID()) // Using standard log for now
	log.Printf("Received arguments: %+v", arguments)

	// ... rest of your tool handler logic ...

	log.Printf("'anotherTool' finished for session %s", session.SessionID())

	return []protocol.Content{protocol.TextContent{Type: "text", Text: "Tool executed."}}, false
}
```

Using the server's configured logger ensures that all logs from your handlers are processed consistently, potentially including session-specific information or routing to different outputs.

## Progress Reporting (`$/progress`)

For long-running operations within your handlers (like complex computations, file processing, or external API calls), you can send progress notifications to the client using the `$/progress` notification. This allows clients to provide feedback to the user about the ongoing operation.

To send progress, the client must include a `protocol.ProgressToken` in the `_meta` field of their request (`tools/call` or `resources/read`). Your handler receives this token as the `progressToken` parameter.

You can then use the `server.SendProgress` method (which requires access to the `server.Server` instance) to send progress updates to the specific session.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/types"
)

// Assume srv is your initialized *server.Server instance, accessible in the handler via closure or other means.

// Example Tool Handler demonstrating progress reporting
func handleLongRunningTool(srv *server.Server) hooks.FinalToolHandler { // Handler factory using closure
	return func(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
		session, ok := types.SessionFromContext(ctx)
		if !ok {
			log.Println("Error: ClientSession not found in context.")
			return []protocol.Content{protocol.TextContent{Type: "text", Text: "Internal server error: Session context missing."}}, true
		}

		sessionID := session.SessionID()
		log.Printf("Long running tool started for session: %s", sessionID)

		if progressToken != nil {
			// Send initial progress report if the client provided a token
			progressParams := protocol.ProgressParams{
				Token: *progressToken, // Use the token provided by the client
				Value: map[string]interface{}{
					"message": "Starting operation...",
					"percent": 0,
				},
			}
			if err := srv.SendProgress(sessionID, progressParams); err != nil {
				log.Printf("Failed to send initial progress to session %s: %v", sessionID, err)
			} else {
				log.Printf("Sent initial progress for session %s, token %v", sessionID, *progressToken)
			}
		}

		// Simulate work with progress updates and cancellation checks
		totalSteps := 5
		for i := 1; i <= totalSteps; i++ {
			select {
			case <-ctx.Done():
				// Check for cancellation signal from the context
				log.Printf("Long running tool cancelled for session %s", sessionID)
				return []protocol.Content{protocol.TextContent{Type: "text", Text: "Operation cancelled."}}, true
			case <-time.After(1 * time.Second):
				// Simulate a step of work
				log.Printf("Working on step %d for session %s", i, sessionID)

				if progressToken != nil {
					// Send progress update
					progressParams := protocol.ProgressParams{
						Token: *progressToken,
						Value: map[string]interface{}{
							"message": fmt.Sprintf("Processing step %d of %d...", i, totalSteps),
							"percent": (i * 100) / totalSteps,
						},
					}
					if err := srv.SendProgress(sessionID, progressParams); err != nil {
						log.Printf("Failed to send progress update to session %s: %v", sessionID, err)
					} else {
						log.Printf("Sent progress update for session %s, token %v, percent %d", sessionID, *progressToken, (i*100)/totalSteps)
					}
				}
			}
		}

		if progressToken != nil {
			// Send final progress report
			progressParams := protocol.ProgressParams{
				Token: *progressToken,
				Value: map[string]interface{}{
					"message": "Operation complete.",
					"percent": 100,
				},
			}
			if err := srv.SendProgress(sessionID, progressParams); err != nil {
				log.Printf("Failed to send final progress to session %s: %v", sessionID, err)
			} else {
				log.Printf("Sent final progress for session %s, token %v", sessionID, *progressToken)
			}
		}

		log.Printf("Long running tool finished for session: %s", sessionID)
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Operation completed successfully."}}, false
	}
}

// When registering the handler, use the factory:
// srv := server.NewServer(...)
// srv.RegisterTool(longRunningToolDefinition, handleLongRunningTool(srv)) // Pass srv to the factory
```

The `protocol.ProgressParams` struct contains the `Token` (matching the client's token) and a `Value`, which is an arbitrary JSON object containing the progress details (e.g., message, percentage).

## Cancellation (`$/cancelled`)

Clients can request the cancellation of a long-running request by sending a `$/cancelled` notification with the ID of the request they wish to cancel. The GoMCP server automatically handles this notification and cancels the `context.Context` that was passed to your handler for that specific request.

Your handler should monitor the provided `context.Context` (`ctx`) for cancellation signals. The `ctx.Done()` method returns a channel that is closed when the context is cancelled. You should check this channel periodically, especially before or during potentially blocking operations, and return early if a cancellation signal is received.

```go
// Inside your handler function:
func handleCancellableTool(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
	// ... setup ...

	select {
	case <-ctx.Done():
		// Cancellation requested
		log.Println("Operation cancelled via context.")
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Operation cancelled."}}, true
	default:
		// Not cancelled, proceed with work
	}

	// Example of checking cancellation within a loop:
	for i := 0; i < 100; i++ {
		select {
		case <-ctx.Done():
			log.Println("Operation cancelled during loop.")
			return []protocol.Content{protocol.TextContent{Type: "text", Text: "Operation cancelled."}}, true
		default:
			// Perform a small piece of work
			time.Sleep(100 * time.Millisecond)
		}
	}

	// ... rest of handler logic ...
	return []protocol.Content{protocol.TextContent{Type: "text", Text: "Operation completed."}}, false
}
```

By checking `<-ctx.Done()`, your handlers can gracefully respond to client cancellation requests, preventing unnecessary resource usage for operations that are no longer needed.
