# Cancellation Notification Support in GoMCP

The MCP specification includes support for cancellation notifications (`notifications/cancelled`), which allow clients to cancel in-progress requests. This document explains how to use the cancellation support in GoMCP.

## Cancellation Mechanism

When a client sends a cancellation notification, the server:

1. Matches the cancellation to the in-progress request by ID
2. Stops processing of the cancelled request
3. Cleans up resources associated with the cancelled request
4. Ensures no response is sent for a properly cancelled request

## Server-Side Implementation

### Tool Handlers with Cancellation Support

Tool handlers can check for cancellation in several ways:

```go
// Option 1: Use the CheckCancellation helper method
func myToolHandler(ctx *server.Context, args struct{}) (interface{}, error) {
    // Check for cancellation before starting expensive work
    if err := ctx.CheckCancellation(); err != nil {
        return nil, fmt.Errorf("cancelled before starting: %w", err)
    }

    // Do some work...

    // Check for cancellation periodically
    if err := ctx.CheckCancellation(); err != nil {
        return nil, fmt.Errorf("cancelled during processing: %w", err)
    }

    return result, nil
}

// Option 2: Use the cancellation channel directly
func myLongRunningTool(ctx *server.Context, args struct{}) (interface{}, error) {
    // Register for cancellation
    cancelCh := ctx.RegisterForCancellation()

    // Do work in a loop with cancellation checks
    for i := 0; i < workItems; i++ {
        select {
        case <-cancelCh:
            return nil, fmt.Errorf("task cancelled after processing %d items", i)
        default:
            // Continue with work
        }

        // Process item...
    }

    return result, nil
}
```

## Client-Side Implementation

To cancel a request from the client side:

```go
// Create a cancellation notification
notification := map[string]interface{}{
    "jsonrpc": "2.0",
    "method":  "notifications/cancelled",
    "params": map[string]interface{}{
        "requestId": requestID, // Must match the ID of the request to cancel
        "reason":    "Optional reason for cancellation",
    },
}

// Send the notification
notificationBytes, _ := json.Marshal(notification)
conn.Write(notificationBytes)
```

## Example

The GoMCP repository includes a complete example demonstrating cancellation:

1. Create a server that registers a long-running tool
2. Start a client that calls the tool
3. Send a cancellation notification from the client
4. Handle the cancellation on the server side

See the full example in `examples/cancellation/`.

## Cancellation and Different Transports

Cancellation works across all transport types supported by GoMCP. The implementation correctly handles race conditions where:

- A request might complete before cancellation arrives
- Multiple cancellation requests might be sent
- A cancelled request might still produce a result

Each transport properly propagates cancellation notifications to ensure reliable cancellation behavior regardless of the communication mechanism used.
