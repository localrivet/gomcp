---
title: Billing / Usage Tracking
weight: 60 # Sixth example
---

This page details the example found in the `/examples/billing` directory, demonstrating a conceptual approach to tracking usage or costs associated with MCP tool calls.

MCP doesn't define a billing protocol, so this example shows one possible implementation strategy, likely involving middleware or logic within tool handlers to record usage events.

## Billing Server (`examples/billing/server`)

This example likely intercepts tool calls (perhaps using middleware similar to the auth example, or within the tool handlers themselves) to record usage information before or after the tool executes.

**Conceptual Snippets (Illustrative - check `main.go` for actual implementation):**

_Middleware Approach:_

```go
// Hypothetical middleware to track tool calls
func billingMiddleware(srv *server.Server) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Attempt to decode the request to see if it's a tools/call
			// (This is complex as the body needs to be read and potentially replaced)
			// ... decode logic ...

			isToolCall := false // Assume decoding logic sets this
			toolName := ""      // Assume decoding logic sets this
			clientID := ""      // Assume auth middleware sets this

			if isToolCall {
				// Record usage before calling the actual handler
				log.Printf("BILLING: Recording usage for client '%s', tool '%s'", clientID, toolName)
				// recordUsage(clientID, toolName, time.Now()) // Your billing logic
			}

			next.ServeHTTP(w, r) // Call the main MCP handler

			// Could also record after the call, potentially based on response status
		})
	}
}

func main() {
	// ... setup srv, sseServer ...
	mux := http.NewServeMux()

	// Wrap the message handler with billing and auth middleware
	messageHandler := http.HandlerFunc(srv.HTTPHandler)
	// Order matters: auth first, then billing
	wrappedHandler := authMiddleware(billingMiddleware(srv)(messageHandler))

	mux.Handle("/message", wrappedHandler)
	mux.Handle("/events", authMiddleware(http.HandlerFunc(sseServer.HTTPHandler))) // Auth SSE too

	// ... start server ...
}
```

_Handler Approach:_

```go
// Tool handler includes billing logic
func handleBillableTool(ctx context.Context, args map[string]interface{}) ([]protocol.Content, error) {
	clientID := "" // Get client ID from context (if passed via middleware)
	toolName := "billableTool"

	// Record usage at the start
	log.Printf("BILLING: Recording usage for client '%s', tool '%s'", clientID, toolName)
	// recordUsage(clientID, toolName, time.Now())

	// --- Actual tool logic ---
	resultText := "Executed billable tool."
	// --- End tool logic ---

	return []protocol.Content{protocol.TextContent{Type: "text", Text: resultText}}, nil
}

func main() {
	// ... setup srv ...

	// Register the tool with the billing-aware handler
	srv.RegisterTool(billableToolDef, handleBillableTool)

	// ... setup transport and run ...
}
```

**To Run:** Navigate to `examples/billing/server` and run `go run main.go`. Observe the server logs for billing messages when tools are called (requires a client like the one in `examples/billing/client`).

**Note:** The actual implementation in `examples/billing/server/main.go` should be consulted for the precise mechanism used. These snippets illustrate common patterns. Implementing robust billing often involves integrating with external systems or databases.
