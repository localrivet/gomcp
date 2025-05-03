---
title: 'Transports'
weight: 20
---

## Transports

GoMCP separates the core protocol logic from how clients and servers communicate. Supported transports are found in the `transport/` directory. The `gomcp/client` package provides constructors for the supported transports, abstracting away the low-level communication details.

### Choosing a Transport

Select the transport that corresponds to the server you are connecting to:

- **Stdio:** For communicating with a server running as a local child process, piping standard input/output.
- **SSE + HTTP:** For connecting to servers implementing the `2024-11-05` protocol's HTTP+SSE transport model (HTTP POST for requests, SSE for server-to-client messages).
- **WebSocket:** For connecting to servers implementing the `2025-03-26` protocol's Streamable HTTP transport model (single WebSocket connection for all messages).
- **TCP:** A lower-level option for raw TCP socket communication.

### Stdio Transport

The Stdio transport facilitates communication over standard input and output streams. This is particularly useful for:

- **Local Inter-Process Communication:** When your client application launches and manages the server as a child process.
- **Simple Examples and Testing:** Provides an easy way to demonstrate basic MCP communication without network setup.

To create a client using the Stdio transport, use the `client.NewStdioClient` constructor:

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Stdio Client...")

	// Create a client instance for the stdio transport
	// Provide a client name and optional ClientOptions
	clt, err := client.NewStdioClient("MyStdioClient", client.ClientOptions{
		// Optional configurations can go here
		// Logger: myCustomLogger,
	})
	if err != nil {
		log.Fatalf("Failed to create stdio client: %v", err)
	}
	// Ensure the client connection is closed when main exits
	defer clt.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect to the server and perform the initialization handshake
	log.Println("Connecting to server via stdio...")
	err = clt.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect and initialize: %v", err)
	}
	log.Printf("Connected to server: %s", clt.ServerInfo().Name)

	// Client is now ready to make requests using the 'clt' instance
	// ... example requests like ListTools, CallTool, etc. ...

	log.Println("Client operations finished.")
}
```

When using Stdio, the client typically writes JSON-RPC messages to standard output, which are read by the server from its standard input, and vice-versa.

### SSE + HTTP Hybrid Transport

This transport is designed for network communication and is compatible with servers implementing the transport model introduced in the `2024-11-05` protocol specification. It utilizes:

- **HTTP POST:** For the client to send requests (like `initialize`, `tools/call`).
- **Server-Sent Events (SSE):** For the server to send asynchronous messages (like `initialize` responses, notifications, server-initiated requests) to the client over a persistent connection.

To create a client for this transport, use the `client.NewSSEClient` constructor:

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting SSE Client...")

	// Server's base URL (e.g., http://localhost:8080)
	baseURL := "http://localhost:8080"
	// The base path for MCP endpoints on the server (e.g., /mcp)
	basePath := "/mcp"

	// Create a client instance for the SSE transport
	clt, err := client.NewSSEClient("MySSEClient", baseURL, basePath, client.ClientOptions{
		// Optional configurations
	})
	if err != nil {
		log.Fatalf("Failed to create SSE client: %v", err)
	}
	defer clt.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect to the server and perform the initialization handshake
	log.Println("Connecting to server via SSE+HTTP...")
	err = clt.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect and initialize: %v", err)
	}
	log.Printf("Connected to server: %s", clt.ServerInfo().Name)

	// Client is now ready to make requests using the 'clt' instance
	// ... example requests ...

	log.Println("Client operations finished.")
}
```

This transport is suitable for web-based clients or environments where SSE is a preferred mechanism for server-push.

### WebSocket Transport

The WebSocket transport is the recommended network transport for servers implementing the `2025-03-26` protocol specification's "Streamable HTTP" model. It uses a single, full-duplex WebSocket connection for all communication, allowing both client and server to send messages asynchronously over the same connection.

To create a client for this transport, use the `client.NewWebSocketClient` constructor:

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting WebSocket Client...")

	// Server's WebSocket URL (e.g., ws://localhost:8080/mcp)
	wsURL := "ws://localhost:8080/mcp"

	// Create a client instance for the WebSocket transport
	clt, err := client.NewWebSocketClient("MyWebSocketClient", wsURL, client.ClientOptions{
		// Optional configurations
	})
	if err != nil {
		log.Fatalf("Failed to create WebSocket client: %v", err)
	}
	defer clt.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect to the server and perform the initialization handshake
	log.Println("Connecting to server via WebSocket...")
	err = clt.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect and initialize: %v", err)
	}
	log.Printf("Connected to server: %s", clt.ServerInfo().Name)

	// Client is now ready to make requests using the 'clt' instance
	// ... example requests ...

	log.Println("Client operations finished.")
}
```

WebSocket is generally preferred for new implementations supporting the latest protocol version due to its simplicity and efficiency for bidirectional communication.

### TCP Transport

The TCP transport provides a lower-level option for raw TCP socket connections. This might be used in specific scenarios where a custom layer is built on top of TCP or for direct socket-based communication.

To create a client for this transport, use the `client.NewTCPClient` constructor:

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting TCP Client...")

	// Server's TCP address (e.g., localhost:6000)
	tcpAddr := "localhost:6000"

	// Create a client instance for the TCP transport
	clt, err := client.NewTCPClient("MyTCPClient", tcpAddr, client.ClientOptions{
		// Optional configurations
	})
	if err != nil {
		log.Fatalf("Failed to create TCP client: %v", err)
	}
	defer clt.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect to the server and perform the initialization handshake
	log.Println("Connecting to server via TCP...")
	err = clt.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect and initialize: %v", err)
	}
	log.Printf("Connected to server: %s", clt.ServerInfo().Name)

	// Client is now ready to make requests using the 'clt' instance
	// ... example requests ...

	log.Println("Client operations finished.")
}
```

When using the TCP transport, you are responsible for ensuring that the data sent and received over the socket adheres to the JSON-RPC 2.0 and MCP specifications.

### Client Options (`client.ClientOptions`)

All client transport constructors accept a `client.ClientOptions` struct for configuration:

```go
type ClientOptions struct {
	// ClientCapabilities allows specifying the capabilities the client supports.
	// This is sent to the server during the initialization handshake.
	ClientCapabilities protocol.ClientCapabilities

	Logger types.Logger

	// PreferredProtocolVersion allows the client to request a specific protocol version.
	// If nil, the client will attempt to negotiate the latest supported version.
	PreferredProtocolVersion *string

	// Custom options can be provided as key-value pairs for transport-specific settings.
	Custom map[string]interface{}
}
```

- `ClientCapabilities`: Define the features your client supports.
- `Logger`: Provide a custom logger if you don't want to use the default.
- `PreferredProtocolVersion`: Specify a desired protocol version.
- `Custom`: Pass transport-specific options.

By choosing the appropriate transport and configuring the client options, you can build GoMCP clients that effectively communicate with various MCP servers.
