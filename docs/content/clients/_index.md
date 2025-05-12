---
title: Clients
weight: 30
---

The `gomcp` library provides tools for building applications that act as MCP clients. An MCP client connects to an MCP server to discover and utilize the server's capabilities, such as executing tools, accessing resources, and retrieving prompt templates.

The core client logic is implemented in the `gomcp/client` package. This package provides a high-level `Client` struct that manages the connection, handles the MCP handshake, and provides methods for interacting with the server. The `Client` struct works with various underlying transport mechanisms, which are covered in detail in the [Client Transports](#client-transports) section below.

### Client Role

An MCP client application typically performs the following tasks:

1.  **Connect to a Server:** Establish a communication link with a known MCP server endpoint using a specific transport (Stdio, SSE+HTTP, WebSocket, TCP).
2.  **Initialize the Session:** Perform the MCP initialization handshake (`initialize` request and `initialized` notification) to exchange capabilities and establish the session.
3.  **Discover Capabilities:** Query the server to discover the available tools (`tools/list`), resources (`resources/list`), and prompts (`prompts/list`).
4.  **Execute Tools:** Call specific tools on the server (`tools/call`) to perform actions or computations, providing necessary arguments and handling the results.
5.  **Access Resources:** Retrieve the content of resources from the server (`resources/read`) to obtain contextual data.
6.  **Subscribe to Updates:** Subscribe to resource updates (`resources/subscribe`) to receive notifications when resource content or metadata changes (`notifications/resources/updated`).
7.  **Retrieve Prompts:** Get the definition of prompt templates from the server (`prompts/get`), potentially with arguments substituted.
8.  **Handle Server Messages:** Process asynchronous messages from the server, such as progress notifications (`$/progress`), logging messages (`notifications/message`), or resource updates.

### Initializing the Client (`client` package)

To create an MCP client, you use one of the transport-specific constructors provided by the `client` package. These constructors return a `*client.Client` instance configured to use the specified transport.

Here's a general example using a placeholder for the transport constructor (refer to [Client Transports](#client-transports) for specific transport examples):

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
	// Configure logger (optional)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting MCP Client...")

	// 1. Create Client Instance using a transport constructor
	// Example using NewStdioClient (replace with appropriate transport constructor)
	clt, err := client.NewStdioClient("MyClient", client.ClientOptions{
		// Optional: Customize client capabilities or provide a logger
		// ClientCapabilities: protocol.ClientCapabilities{ /* ... */ },
		// Logger: myCustomLogger,
		// PreferredProtocolVersion: &protocol.CurrentProtocolVersion,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	// Ensure the client connection is closed when main exits
	defer clt.Close()

	// Use a context for the connection and requests
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 2. Connect to the server and perform the initialization handshake
	log.Println("Connecting to server...")
	err = clt.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect and initialize with server: %v", err)
	}
	log.Printf("Connected to server: %s (Version: %s)", clt.ServerInfo().Name, clt.ServerInfo().Version)
	log.Printf("Server capabilities: %+v", clt.ServerCapabilities())

	// --- Client is now ready to make requests ---

	// Example: List available tools from the server
	log.Println("Listing available tools...")
	toolsResult, err := clt.ListTools(ctx, protocol.ListToolsRequestParams{})
	if err != nil {
		log.Printf("Error listing tools: %v", err)
	} else {
		log.Printf("Available tools (%d):", len(toolsResult.Tools))
		for _, tool := range toolsResult.Tools {
			log.Printf("- %s: %s", tool.Name, tool.Description)
		}
	}

	// Example: Call the "echo" tool
	log.Println("Calling 'echo' tool...")
	callToolParams := protocol.CallToolParams{
		Name: "echo",
		Arguments: map[string]interface{}{
			"input": "Hello from the client!",
		},
	}
	callToolResult, err := clt.CallTool(ctx, callToolParams)
	if err != nil {
		log.Printf("Error calling tool 'echo': %v", err)
	} else {
		log.Println("Tool 'echo' returned content:")
		for _, content := range callToolResult.Content {
			// Assuming TextContent for this example
			if textContent, ok := content.(protocol.TextContent); ok {
				log.Printf("- %s: %s", textContent.Type, textContent.Text)
			} else {
				log.Printf("- Received content of type: %s", content.GetContentType())
			}
		}
		if callToolResult.IsError != nil && *callToolResult.IsError {
			log.Println("Tool execution reported an error.")
		}
	}

	// Example: List available resources
	log.Println("Listing available resources...")
	resourcesResult, err := clt.ListResources(ctx, protocol.ListResourcesRequestParams{})
	if err != nil {
		log.Printf("Error listing resources: %v", err)
	} else {
		log.Printf("Available resources (%d):", len(resourcesResult.Resources))
		for _, resource := range resourcesResult.Resources {
			log.Printf("- %s (Kind: %s, Version: %s)", resource.URI, resource.Kind, resource.Version)
		}
	}

	// Example: List available prompts
	log.Println("Listing available prompts...")
	promptsResult, err := clt.ListPrompts(ctx, protocol.ListPromptsRequestParams{})
	if err != nil {
		log.Printf("Error listing prompts: %v", err)
	} else {
		log.Printf("Available prompts (%d):", len(promptsResult.Prompts))
		for _, prompt := range promptsResult.Prompts {
			log.Printf("- %s (Title: %s)", prompt.URI, prompt.Title)
		}
	}


	log.Println("Client operations finished.")
}
```

The `client.ClientOptions` struct allows you to configure the client's name, specify its capabilities, provide a custom logger, or set a preferred protocol version for the handshake.

The `Connect` method establishes the connection and performs the MCP initialization handshake. It blocks until the handshake is complete or an error occurs. You should always call `Connect` before attempting to send any other requests.

### Making Requests

Once the client is connected and initialized, you can use the methods provided by the `*client.Client` instance to interact with the server. Each request method takes a `context.Context` for cancellation and deadlines, and a parameters struct defined in the `gomcp/protocol` package.

Here are some common request methods:

- **`ListTools(ctx context.Context, params protocol.ListToolsRequestParams) (*protocol.ListToolsResult, error)`**: Retrieves a list of available tools from the server.
- **`CallTool(ctx context.Context, params protocol.CallToolParams) (*protocol.CallToolResult, error)`**: Requests the server to execute a specific tool with the given arguments.
- **`ListResources(ctx context.Context, params protocol.ListResourcesRequestParams) (*protocol.ListResourcesResult, error)`**: Retrieves a list of available resources from the server.
- **`GetResource(ctx context.Context, params protocol.ReadResourceRequestParams) (*protocol.ReadResourceResult, error)`**: Retrieves the content of a specific resource.
- **`SubscribeResources(ctx context.Context, params protocol.SubscribeResourceParams) (*protocol.SubscribeResourceResult, error)`**: Subscribes to updates for a list of resources.
- **`UnsubscribeResources(ctx context.Context, params protocol.UnsubscribeResourceParams) (*protocol.UnsubscribeResourceResult, error)`**: Unsubscribes from updates for a list of resources.
- **`ListPrompts(ctx context.Context, params protocol.ListPromptsRequestParams) (*protocol.ListPromptsResult, error)`**: Retrieves a list of available prompt templates.
- **`GetPrompt(ctx context.Context, params protocol.GetPromptRequestParams) (*protocol.GetPromptResult, error)`**: Retrieves the definition of a specific prompt, potentially with arguments substituted.
- **`SendCancellation(ctx context.Context, id interface{}) error`**: Sends a cancellation notification for a previously sent request with the given ID.
- **`SendProgress(ctx context.Context, params protocol.ProgressParams) error`**: Sends a progress notification (if the client is performing a long-running operation that the server is monitoring).

Refer to the relevant sections below for detailed information on the parameters and results for each method.

## Message Structures

Communication between an MCP client and server involves sending and receiving JSON-RPC messages. This section details the fundamental message structures used in MCP.

### JSON-RPC 2.0 Base Messages

All messages exchanged in MCP adhere to the JSON-RPC 2.0 standard. The core message types are Request, Response, and Notification.

#### Request (`protocol.JSONRPCRequest`)

A Request is sent by a client to a server (or vice-versa, though less common in the base protocol) to invoke a specific method. It expects a Response.

```go
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`          // MUST be "2.0"
	ID      interface{} `json:"id"`               // Request ID (string, number, or null)
	Method  string      `json:"method"`           // Method name (e.g., "initialize", "tools/call")
	Params  interface{} `json:"params,omitempty"` // Parameters (struct or array)
}
```

- `jsonrpc`: Specifies the JSON-RPC version, must be `"2.0"`.
- `id`: A unique identifier for the request. This can be a string, number, or `null`. The corresponding Response must use the same `id`. Notifications do not have an `id`.
- `method`: A string containing the name of the method to be invoked (e.g., `"initialize"`, `"tools/call"`, `"resources/get"`).
- `params`: An optional field containing the parameters for the method. This can be a structured object or an array of values, depending on the method definition.

#### Response (`protocol.JSONRPCResponse`)

A Response is sent by a server in reply to a Request. It contains either a `result` on success or an `error` on failure.

```go
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`          // MUST be "2.0"
	ID      interface{}   `json:"id"`               // MUST be the same as the request ID (or null if error before ID parsing)
	Result  interface{}   `json:"result,omitempty"` // Result object (on success)
	Error   *ErrorPayload `json:"error,omitempty"`  // Error object (on failure)
}
```

- `jsonrpc`: Specifies the JSON-RPC version, must be `"2.0"`.
- `id`: The ID of the Request that this Response is replying to. If an error occurred before the server could parse the Request's ID, this field may be `null`.
- `result`: This field is present on success and contains the data returned by the invoked method. Its structure depends on the method definition.
- `error`: This field is present on failure and contains an `ErrorPayload` object detailing the error.

#### Notification (`protocol.JSONRPCNotification`)

A Notification is sent by a client or server to signal an event or convey information. Unlike Requests, Notifications do not expect a Response.

```go
type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`          // MUST be "2.0"
	Method  string      `json:"method"`           // Method name (e.g., "initialized", "notifications/...")
	Params  interface{} `json:"params,omitempty"` // Parameters (struct or array)
	// Note: Notifications MUST NOT have an 'id' field.
}
```

- `jsonrpc`: Specifies the JSON-RPC version, must be `"2.0"`.
- `method`: A string containing the name of the notification method (e.g., `"initialized"`, `"$/cancelled"`, `"notifications/resources/updated"`).
- `params`: An optional field containing parameters for the notification.

#### Error Payload (`protocol.ErrorPayload`)

When a JSON-RPC Request fails, the Response includes an `error` field containing an `ErrorPayload` object.

```go
type ErrorPayload struct {
	Code    int         `json:"code"`           // Numeric error code (JSON-RPC standard or implementation-defined)
	Message string      `json:"message"`        // Short error description
	Data    interface{} `json:"data,omitempty"` // Optional additional error details
}
```

- `code`: A numeric error code. JSON-RPC 2.0 defines a standard range of codes (e.g., -32700 to -32603 for parse errors, invalid requests, etc.). MCP may define additional implementation-specific codes.
- `message`: A concise, human-readable description of the error.
- `data`: An optional field that can contain additional structured data about the error.

### Content Structures (`protocol.Content` and Implementations)

Many MCP messages, particularly tool results and prompt messages, include content. Content is represented as a slice of objects that implement the `protocol.Content` interface. This allows for rich, multi-part content.

```go
type Content interface {
	GetType() string
}
```

The `GetType()` method returns a string indicating the specific type of content. GoMCP defines several standard content types:

#### Text Content (`protocol.TextContent`)

Represents plain or formatted text.

```go
type TextContent struct {
	Type        string              `json:"type"` // "text"
	Text        string              `json:"text"`
	Annotations *ContentAnnotations `json:"annotations,omitempty"`
}
```

- `type`: Always `"text"`.
- `text`: The string containing the text content.
- `annotations`: Optional metadata about the content (e.g., title, audience).

#### Image Content (`protocol.ImageContent`)

Represents image data, typically base64 encoded.

```go
type ImageContent struct {
	Type        string              `json:"type"` // "image"
	Data        string              `json:"data"` // Base64 encoded image data
	MediaType   string              `json:"mediaType"` // e.g., "image/png", "image/jpeg"
	Annotations *ContentAnnotations `json:"annotations,omitempty"`
}
```

- `type`: Always `"image"`.
- `data`: The base64 encoded string of the image data.
- `mediaType`: The MIME type of the image (e.g., `"image/png"`).
- `annotations`: Optional metadata.

#### Audio Content (`protocol.AudioContent`)

Represents audio data, typically base64 encoded.

```go
type AudioContent struct {
	Type        string              `json:"type"` // "audio"
	Data        string              `json:"data"` // Base64 encoded audio data
	MediaType   string              `json:"mediaType"` // e.g., "audio/mpeg", "audio/wav"
	Annotations *ContentAnnotations `json:"annotations,omitempty"`
}
```

- `type`: Always `"audio"`.
- `data`: The base64 encoded string of the audio data.
- `mediaType`: The MIME type of the audio (e.g., `"audio/mpeg"`).
- `annotations`: Optional metadata.

#### Embedded Resource Content (`protocol.EmbeddedResourceContent`)

Represents a reference to a resource that is embedded within the content.

```go
type EmbeddedResourceContent struct {
	Type        string              `json:"type"` // "resource"
	Resource    Resource            `json:"resource"` // The protocol.Resource struct
	Annotations *ContentAnnotations `json:"annotations,omitempty"`
}
```

- `type`: Always `"resource"`.
- `resource`: The `protocol.Resource` struct defining the embedded resource.
- `annotations`: Optional metadata.

### Content Annotations (`protocol.ContentAnnotations`)

Optional metadata that can be applied to individual content parts.

```go
type ContentAnnotations struct {
	Title    *string  `json:"title,omitempty"`
	Audience []string `json:"audience,omitempty"`
	Priority *float64 `json:"priority,omitempty"` // 0-1, higher is more important
}
```

- `title`: An optional title for the content part.
- `audience`: An optional list of strings indicating the intended audience for this content part (e.g., `["user"]`, `["model"]`).
- `priority`: An optional floating-point number between 0 and 1 indicating the importance of this content part, where a higher value means higher priority.

## Client Protocol Methods

This section details the specific JSON-RPC messages used by clients to interact with Resources and Prompts on the server.

### Resource Methods

Clients interact with Resources using the following methods:

#### `resources/list` Request

Clients send the `resources/list` request to discover the resources available on the server.

- **Method:** `"resources/list"`
- **Parameters:** `protocol.ListResourcesRequestParams`
- **Result:** `protocol.ListResourcesResult`

```go
type ListResourcesRequestParams struct {
	Filter map[string]interface{} `json:"filter,omitempty"` // Optional filtering criteria
	Cursor string                 `json:"cursor,omitempty"` // For pagination
}

type ListResourcesResult struct {
	Resources  []Resource `json:"resources"`
	NextCursor string     `json:"nextCursor,omitempty"` // For pagination
}
```

Clients can optionally provide `Filter` criteria to narrow down the list of returned resources and a `Cursor` for pagination. The server responds with a list of `protocol.Resource` definitions.

#### `resources/read` Request

Clients send the `resources/read` request to retrieve the actual content of a specific resource.

- **Method:** `"resources/read"`
- **Parameters:** `protocol.ReadResourceRequestParams`
- **Result:** `protocol.ReadResourceResult`

```go
type ReadResourceRequestParams struct {
	URI     string `json:"uri"`          // The URI of the resource to read
	Version string `json:"version,omitempty"` // Optional version hint
	Meta    *RequestMeta `json:"_meta,omitempty"` // Optional metadata like progress token
}

type ReadResourceResult struct {
	Resource Resource         `json:"resource"` // The resource metadata (may be updated)
	Contents ResourceContents `json:"contents"` // The actual content (Text or Blob)
	Meta    *RequestMeta `json:"_meta,omitempty"` // Optional metadata from the server
}
```

- `URI`: The URI of the resource whose content is requested.
- `Version`: An optional version string that the client has cached. The server can use this as a hint but should return the latest version if available.
- `Meta`: Optional metadata, including a `ProgressToken` if the client wishes to receive progress updates for this read operation.

The server responds with a `ReadResourceResult` containing the resource's metadata (which might be updated) and the actual `Contents`. The `Contents` field will contain an object implementing the `protocol.ResourceContents` interface, such as `protocol.TextResourceContents` or `protocol.BlobResourceContents`.

#### `resources/subscribe` Request

Clients can subscribe to resource updates to be notified when a resource's content or metadata changes.

- **Method:** `"resources/subscribe"`
- **Parameters:** `protocol.SubscribeResourceParams`
- **Result:** `protocol.SubscribeResourceResult` (currently empty)

```go
type SubscribeResourceParams struct {
	URIs []string `json:"uris"` // List of resource URIs to subscribe to
}

type SubscribeResourceResult struct{} // Currently empty
```

Clients provide a list of resource URIs they want to subscribe to. The server tracks these subscriptions per session.

#### `resources/unsubscribe` Request

Clients can unsubscribe from resource updates.

- **Method:** `"resources/unsubscribe"`
- **Parameters:** `protocol.UnsubscribeResourceParams`
- **Result:** `protocol.UnsubscribeResourceResult` (currently empty)

```go
type UnsubscribeResourceParams struct {
	URIs []string `json:"uris"` // List of resource URIs to unsubscribe from
}

type UnsubscribeResourceResult struct{} // Currently empty
```

Clients provide a list of resource URIs they no longer wish to receive updates for.

#### `notifications/resources/updated` Notification

Servers send the `notifications/resources/updated` notification to inform subscribed clients that a specific resource has been updated.

- **Method:** `"notifications/resources/updated"`
- **Parameters:** `protocol.ResourceUpdatedParams`

```go
type ResourceUpdatedParams struct {
	Resource Resource `json:"resource"` // The updated resource metadata
}
```

The server includes the updated `protocol.Resource` metadata in the notification. Clients can then decide whether to re-read the resource content using `resources/read`.

#### `notifications/resources/list_changed` Notification

Servers can send the `notifications/resources/list_changed` notification to inform clients that the list of available resources has changed (resources added or removed).

- **Method:** `"notifications/resources/list_changed"`
- **Parameters:** `protocol.ResourcesListChangedParams` (currently empty)

```go
type ResourcesListChangedParams struct{} // Currently empty
```

This notification does not include the updated list itself, only signals that a change has occurred. Clients must send a `resources/list` request to get the new list.

### Prompt Methods

Clients interact with Prompts using the following methods:

#### `prompts/list` Request

Clients send the `prompts/list` request to discover the prompt templates available on the server.

- **Method:** `"prompts/list"`
- **Parameters:** `protocol.ListPromptsRequestParams`
- **Result:** `protocol.ListPromptsResult`

```go
type ListPromptsRequestParams struct {
	Filter map[string]interface{} `json:"filter,omitempty"` // Optional filtering criteria
	Cursor string                 `json:"cursor,omitempty"` // For pagination
}

type ListPromptsResult struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor string   `json:"nextCursor,omitempty"` // For pagination
}
```

Clients can optionally provide `Filter` criteria and a `Cursor` for pagination. The server responds with a list of `protocol.Prompt` definitions (metadata and message structure with placeholders).

#### `prompts/get` Request

Clients send the `prompts/get` request to retrieve the full definition of a specific prompt, with provided arguments substituted into the message content.

- **Method:** `"prompts/get"`
- **Parameters:** `protocol.GetPromptRequestParams`
- **Result:** `protocol.GetPromptResult`

```go
type GetPromptRequestParams struct {
	URI       string                 `json:"uri"`          // The URI of the prompt to retrieve
	Arguments map[string]interface{} `json:"arguments,omitempty"` // Arguments to substitute into the template
}

type GetPromptResult struct {
	Prompt Prompt `json:"prompt"` // The prompt definition with arguments substituted
}
```

- `URI`: The URI of the prompt template to retrieve.
- `Arguments`: A map of argument names and their values to be substituted into the prompt template's message content.

The server performs the argument substitution in the message content and responds with a `GetPromptResult` containing the updated `protocol.Prompt` struct.

#### `notifications/prompts/list_changed` Notification

Servers can send the `notifications/prompts/list_changed` notification to inform clients that the list of available prompts has changed and they should re-list if they need the updated list.

- **Method:** `"notifications/prompts/list_changed"`
- **Parameters:** `protocol.PromptsListChangedParams` (currently empty)

```go
type PromptsListChangedParams struct{} // Currently empty
```

This notification does not include the updated list itself, only signals that a change has occurred. Clients must send a `prompts/list` request to get the new list.

### Handling Server Messages

MCP servers can send asynchronous messages to clients, such as notifications or server-initiated requests. You can register handlers for these messages using the `RegisterNotificationHandler` and `RegisterRequestHandler` methods on the `client.Client` instance _before_ calling `Connect`.

```go
// Example of registering a notification handler
clt.RegisterNotificationHandler(protocol.MethodProgress, func(ctx context.Context, params interface{}) error {
	// Handle incoming progress notifications
	progressParams, ok := params.(protocol.ProgressParams)
	if !ok {
		log.Printf("Received progress notification with unexpected params type: %T", params)
		return nil // Or return an error if strict type checking is needed
	}
	log.Printf("Received progress update for token %v: %+v", progressParams.Token, progressParams.Value)
	return nil
})

// Example of registering a server-initiated request handler (less common)
// clt.RegisterRequestHandler("server/doSomething", func(ctx context.Context, id interface{}, params interface{}) (interface{}, error) {
// 	// Handle server's request and return a result or error
// 	log.Printf("Received server request 'server/doSomething' with ID %v and params %+v", id, params)
// 	// ... process request ...
// 	return map[string]string{"status": "done"}, nil // Example success result
// })
```

By implementing these handlers, your client application can react to events and requests initiated by the MCP server.

## Client Transports

GoMCP clients can connect to MCP servers using various transport mechanisms. The choice of transport depends on how the server is configured to listen for connections and the specific requirements of your application (e.g., local communication vs. network communication, protocol version compatibility). The `gomcp/client` package provides constructors for the supported transports, abstracting away the low-level communication details.

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

	// Logger is an optional custom logger for client-side logging.
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
