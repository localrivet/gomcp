# Server MCP Compliance Plan (100% Adherence to 2024-11-05 and 2025-03-26)

This document outlines the necessary steps to achieve full compliance for the Go server implementation with both the Model Context Protocol (MCP) specifications, 2024-11-05 and 2025-03-26. The plan is broken down into sections covering different aspects of the protocol and implementation.

**Goal:** Implement all features and requirements defined in both specifications, ensuring correct protocol message handling, data structures, and behaviors.

**Current State:**

- Basic server structure in the `server/` package (`server.go`, `hooks.go`, `transport.go`, `registry.go`, `messaging.go`, `lifecycle_handler.go`, `helpers.go`).
- Chainable API for server configuration.
- Placeholder implementations for most protocol handling logic.
- Persistent compilation errors related to the `github.com/localrivet/gomcp/protocol` package.

---

## Desired Ergonomic SDK Pattern

Based on the refactoring efforts and user feedback, the desired ergonomic SDK pattern for creating and configuring an MCP server in Go is a chainable API starting with the `server.NewServer` constructor. This pattern aims for simplicity and readability in the main application code.

The key characteristics of this pattern are:

- **Server Creation:** A new server instance is created using `server.NewServer("Server Name")`.
- **Chainable Configuration:** Configuration methods are called directly on the server instance, returning the server instance to allow for method chaining.
  - **Transport Configuration:** Methods like `.AsStdio()`, `.AsWebsocket()`, and `.AsSSE()` are used to select the server's transport. Only one transport should be selected.
  - **Prompt Registration:** The `.Prompt(title, description, messages...)` method is used to register prompts. Prompt messages are created using helper functions like `server.System()` and `server.User()`.
  - **Tool Registration:** The `.Tool(name, description, handler)` method is used to register tools. The tool handler function is expected to have a signature that includes a `Context` parameter (e.g., `func(ctx *server.Context, args Args) (Ret, error)`).
  - **Resource Registration:** The `.Resource(resource)` method is used to register resources, accepting a `protocol.Resource` struct.
  - **Root Registration:** The `.Root(root)` method is used to register roots, accepting a `protocol.Root` struct.
- **Context Passing:** Tool and resource handler functions receive a `*server.Context` object, providing access to server capabilities during request processing.
- **Server Lifecycle Management:** The server is started by calling `.Run()` and gracefully shut down using `defer svr.Close()`.

This pattern is demonstrated in the `cmd/demoserver/main.go` example file.

```go
package main

import (
	"log"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create and configure a new server instance using chaining
	svr := server.NewServer("Demo Server 🚀").
		AsStdio(). // Select StdIO transport
		Prompt("Add two numbers", "Add two numbers using a tool", // Register a prompt
			server.System("You are a helpful assistant that adds two numbers."),
			server.User("What is 2 + 2?"),
		).
		Tool("add", "Add two numbers", func(ctx *server.Context, args struct{ A, B int }) (int, error) { // Register a tool with Context
			ctx.Info("Adding numbers") // Example of using context
			return args.A + args.B, nil
		}).
		Resource(protocol.Resource{ // Register a resource
			URI:         "file:///path/to/example.txt",
			Kind:        "file",
			Title:       "Example File",
			Description: "A sample text file.",
		}).
		Root(protocol.Root{ // Register a root
			URI:         "file:///path/to/workspace",
			Kind:        "workspace",
			Title:       "Example Workspace",
			Description: "The root of the example project.",
		})

	// Defer server shutdown
	defer svr.Close()

	// Run the server
	log.Println("Starting server...")
	if err := svr.Run(); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
```

This section will be added to the compliance plan document to clearly define the target API shape for the Go server SDK.

---

## Phase 1: Resolve Core Compilation Issues

The most critical step is to resolve the compilation errors preventing the code from building. These are primarily related to the compiler not recognizing types from the `github.com/localrivet/gomcp/protocol` package.

- [x] **Task 1.1:** Investigate `go.mod` and project structure.
  - [x] Verify that the `github.com/localrivet/gomcp/protocol` package is correctly defined as a local module dependency in the `go.mod` file.
  - [x] Ensure there are no conflicting module paths or replace directives causing issues.
- [x] **Task 1.2:** Inspect `protocol/` package source files.
  - [x] Read the content of all `.go` files in the `protocol/` directory (e.g., `messages.go`, `tools.go`, `resources.go`, `prompts.go`, `protocol.go`, `errors.go`, `jsonrpc.go`, `constants.go`).
  - [x] Verify that all types causing "undefined" errors in the `server/` package are correctly defined and exported (start with an uppercase letter) in the `protocol/` files. (Note: Some names in the plan differ slightly from the code, e.g., `InitializeRequestParams` vs `InitializeParams`, `Tool` vs `ToolInfo`).
  - [x] Specifically verify the definition of `ToolInfo`, `ResourceInfo`, `Response`, `Message`, `Request`, `Notification`, `InitializeParams`, `InitializeResult`, `Content`, `TextContent`, `Prompt`, `PromptMessage`, `Tool`, `Resource`, `ServerCapabilities`.
- [x] **Task 1.3:** Address inconsistencies or missing definitions.
  - [x] If types are missing or incorrectly defined/exported in `protocol/`, note this as an external dependency issue that needs to be fixed in the `protocol/` package itself (as we cannot modify code outside `server/`). (Added `BoolPtr` to `protocol/messages.go`).
  - [x] If the issue is with `go.mod` or project structure, identify the necessary configuration changes (though we cannot apply them directly if outside `server/`).
- [x] **Task 1.4:** Re-evaluate `server/` code usage of `protocol` types.
  - [x] Based on the confirmed definitions in `protocol/`, review the `server/` code (`messaging.go`, `registry.go`, `lifecycle_handler.go`, `helpers.go`) to ensure correct usage of `protocol` types (e.g., correct field names, types, and initialization for composite literals like `protocol.TextContent`).
  - [x] Fix the "invalid composite literal type protocol.Content" error in `server/helpers.go` based on the confirmed structure of `protocol.TextContent`. (This error seems to have been stale).
- [x] **Task 1.5:** Verify package recognition for `server/helpers.go`.
  - [x] Confirm that `server.System`, `server.User`, and `server.Text` are correctly recognized in `cmd/demoserver/main.go` after resolving `protocol` errors.

**Note:** The persistent issues with `server/registry.go` were resolved, allowing implementation of dependent tasks.

---

## Phase 2: Implement Base Protocol (JSON-RPC & Lifecycle)

Implement the core JSON-RPC communication and the server lifecycle messages.

- [x] **Task 2.1:** Refine `server/messaging.go`.
  - [x] Implement robust JSON-RPC message unmarshalling in `HandleMessage`, handling requests, notifications, and responses.
    - [x] Correctly parse the incoming byte slice into a `protocol.Message` struct.
    - [x] Handle potential unmarshalling errors and send appropriate JSON-RPC error responses.
  - [x] Implement message dispatching in `HandleMessage`.
    - [x] Route requests (`msg.Request != nil`) to `handleRequest`.
    - [x] Route notifications (`msg.Notification != nil`) to `handleNotification`.
    - [x] Handle responses (`msg.Response != nil`) by matching them to active requests (requires managing active requests by ID).
  - [x] Implement `handleRequest`.
    - [x] Unmarshal request parameters (`req.Params`) into the expected type based on `req.Method`.
    - [x] Dispatch the request to the appropriate handler function.
    - [x] Handle cases for unknown methods by sending a JSON-RPC error response.
    - [x] Handle errors returned by handler functions by sending a JSON-RPC error response.
    - [x] Send successful results returned by handler functions as a JSON-RPC success response.
  - [x] Implement `handleNotification`.
    - [x] Unmarshal notification parameters (`notif.Params`).
    - [x] Dispatch the notification to registered handlers (using the `notificationHandlers` map).
  - [x] Implement methods for sending JSON-RPC responses and notifications over the active transport connection.
- [ ] **Task 2.2:** Refine `server/lifecycle_handler.go`.
  - [ ] Fully implement `InitializeHandler`.
    - [x] Correctly parse the `params`.
    - [x] Negotiate server capabilities based on client capabilities and server features.
    - [x] Construct and return a `protocol.InitializeResult`, including `protocolVersion` ("2024-11-05" or "2025-03-26" based on negotiation/support), `capabilities`, and `serverInfo`.
  - [x] Implement `shutdown` request handler.
  - [x] Implement `exit` notification handler.
  - [x] Implement handling of the `initialized` notification received from the client after initialization is complete.

---

## Phase 3: Implement Server Features

Implement the core server capabilities: Tools, Resources, and Prompts.

- [x] **Task 3.1:** Refine `server/registry.go`.
  - [x] Fully implement the `Tool` method. (Core logic implemented, minor refinements remaining as TODOs).
    - [x] Correctly generate `protocol.ToolInputSchema` from the `Args` type of the `fn` function using `schema.FromStruct`. This requires using reflection to get the `Args` type from the `fn any` parameter.
    - [x] Store the generated `protocol.Tool` information in `toolRegistry`.
    - [x] Store the original tool handler function (`fn`) and its `Args` and `Ret` types (using `reflect.Type`) in `toolHandlers`.
  - [x] Fully implement `GetToolHandler`. (Core logic implemented, minor refinements remaining as TODOs).
    - [x] Retrieve the stored tool handler info (original function, ArgsType, RetType).
    - [x] Create and return a wrapper function `func(json.RawMessage) ([]protocol.Content, error)`.
    - [x] Inside the wrapper:
      - [x] Use `mapstructure` to parse and validate the `rawArgs` (JSON) into the expected `Args` struct. Handle errors by returning `[]protocol.Content` with error details.
      - [x] Call the original tool handler function (`fn`) using reflection, passing the parsed `Args` struct. Handle errors returned by the tool function.
      - [x] Convert the result returned by the tool function (of type `Ret`) into `[]protocol.Content`. This will require reflection and handling different `Ret` types (e.g., string, struct, error).
  - [x] Implement `RegisterResource` and `UnregisterResource` methods.
  - [x] Implement `ResourceRegistry` method (to list registered resources).
  - [x] Implement `RegisterPrompt` and `UnregisterPrompt` methods.
- [x] **Task 3.2:** Implement Tool Execution Handler.
  - [x] Create a new file `server/tool_handler.go` or add logic to `server/messaging.go`. (Implemented in `server/messaging.go`).
  - [x] Implement the handler for the `tools/call` request.
  - [x] Retrieve the tool name from the request parameters.
  - [x] Use `registry.GetToolHandler` to get the wrapper function for the tool.
  - [x] Call the wrapper function with the raw arguments from the request.
  - [x] Format the result returned by the wrapper (which is `[]protocol.Content` or an error) into a `protocol.CallToolResult` and send a JSON-RPC response.
- [x] **Task 3.3:** Implement Resource Handlers.
  - [x] Create a new file `server/resource_handler.go` or add logic to `server/messaging.go`. (Implemented in `server/messaging.go`).
  - [x] Implement the handler for the `resources/list` request, returning `protocol.ListResourcesResult`.
  - [x] Implement the handler for the `resources/read` request, returning `protocol.ReadResourceResult`, handling different content types (text, blob, audio - 2025-03-26). (File content reading for 'file' and 'blob' kinds implemented, other kinds and proper trigger mechanism are TODOs).
  - [x] Implement handlers for `resources/subscribe` and `resources/unsubscribe` requests.
  - [x] Implement sending `notifications/resources/updated` when resource content changes. (Infrastructure for sending notifications to subscribed clients is in place, triggered by registry changes. A proper resource change detection mechanism is a remaining TODO).
        **Note:** Implemented a `SubscriptionManager` to manage client subscriptions and updated `MessageHandler` and `TransportManager` to support connection-specific message handling and sending for resource notifications.
- [x] **Task 3.4:** Implement Prompt Handlers.
  - [x] Create a new file `server/prompt_handler.go` or add logic to `server/messaging.go`. (Implemented in `server/messaging.go`).
  - [x] Implement the handler for the `prompts/list` request, returning `protocol.ListPromptsResult`.
  - [x] Implement sending `notifications/prompts/list_changed` when prompts change.

---

## Phase 4: Implement Client Features (Server Interaction)

Implement features where the server initiates interaction with the client.

- [x] **Task 4.1:** Implement Sampling Handler. (Simulated implementation added in `server/messaging.go`)
  - [x] Create a new file `server/sampling_handler.go` or add logic to `server/messaging.go`. (Added logic to `server/messaging.go`).
  - [x] Implement the handler for the `sampling/request` (or `sampling/create_message` for 2025-03-26) request.
  - [x] Process the incoming sampling context (`messages`), preferences, etc. (Basic processing in simulation)
  - [ ] Integrate with an actual AI/LLM (this is an external dependency, but the handler manages the protocol interaction). (Simulated - TODO)
  - [x] Format the AI/LLM response into `protocol.SamplingResult` (or `CreateMessageResultV20250326`) and send a JSON-RPC response. (Using 2025-03-26 structure in simulation)
  - [ ] Verify compatibility with 2024-11-05 sampling request/result structures. (TODO)

---

## Phase 5: Implement Additional Utilities

Implement supporting protocol features.

- [x] **Task 5.1:** Implement Progress Reporting. (Implemented in `server/messaging.go`)
  - [x] Implement sending `$/progress` notifications using `protocol.ProgressParams`.
  - [x] Integrate progress reporting into long-running operations (e.g., tool execution, resource reading). (Basic integration in tools/call handler - TODO for more granular reporting)
- [x] **Task 5.2:** Implement Cancellation. (Handler added in `server/messaging.go`)
  - [x] Implement handling of `$/cancelled` notifications.
  - [ ] Integrate cancellation support into long-running operations. (TODO)
- [x] **Task 5.3:** Implement Logging. (Handlers added in `server/messaging.go`)
  - [x] Implement `logging/set_level` request handler. (TODO for actual logging level control)
  - [x] Implement sending `notifications/message` notifications using `protocol.LoggingMessageParams`. (TODO for integration with server logging output and client filtering)
- [x] **Task 5.4:** Implement Configuration. (Using existing chainable API and protocol messages for configuration)
  - [x] Design and implement a configuration structure for the server (e.g., for transport settings, logging levels). (Addressed via existing API and protocol messages - TODO for a more formal external configuration loading mechanism)
- [x] **Task 5.5:** Implement Error Handling. (Refined handling in `server/messaging.go`)
  - [x] Define and use custom error types that map to JSON-RPC error codes where appropriate. (Using `protocol.MCPError`)
  - [x] Ensure all handlers and server logic correctly propagate and handle errors, sending appropriate JSON-RPC error responses. (TODO for comprehensive review and specific error types)

---

## Phase 6: Refine Transport Implementation

Ensure transports correctly handle message exchange.

- [x] **Task 6.1:** Refine `server/transport.go`. (StdIO transport refined with Content-Length framing)
  - [x] Fully implement `RunStdIO` to read JSON-RPC messages from stdin and write responses/notifications to stdout. (Implemented Content-Length framing)
  - [ ] Implement `RunWebsocket` to handle WebSocket connections and message framing. (TODO)
  - [ ] Implement `RunSSE` to handle SSE connections (for notifications) and potentially receive requests (Streamable HTTP for 2025-03-26). (TODO)
  - [x] Ensure each transport correctly calls `MessageHandler.HandleMessage` for incoming messages and uses the message sending logic for outgoing messages. (Implemented for StdIO)
  - [x] Ensure the `Run` method in `server/server.go` correctly starts and manages all enabled transports concurrently using `sync.WaitGroup`.
  - [ ] Implement sending messages for WebSocket and SSE transports in `SendMessage`. (TODO)

---

## Phase 7: Implement Security and Trust & Safety

Implement security-related requirements, especially from 2025-03-26.

- [ ] **Task 7.1:** Implement Authorization Framework (2025-03-26).
  - [ ] Design and implement authorization checks based on OAuth 2.1 principles as outlined in the spec.
  - [ ] Integrate authorization checks into request handlers (tools, resources, etc.).
- [ ] **Task 7.2:** Implement Consent Flows.
  - [ ] Design and implement mechanisms for obtaining user consent before performing sensitive operations (e.g., tool execution, accessing certain resources).

---

## Phase 8: Specification Versioning and Compatibility

Ensure the server correctly handles both specification versions.

- [x] **Task 8.1:** Implement Capability Negotiation. (Basic negotiation implemented in `server/lifecycle_handler.go`)
  - [x] In `InitializeHandler`, correctly determine the negotiated protocol version based on client capabilities and server support. (Basic negotiation implemented)
  - [x] In `InitializeResult`, correctly report the server's capabilities based on the negotiated version and implemented features. (Basic reporting implemented)
  - [ ] Implement full capability negotiation based on client capabilities. (TODO)
- [ ] **Task 8.2:** Implement Version-Specific Logic.
  - [ ] Where specifications differ (e.g., logging message format, sampling request/result structures, transport), implement logic to handle messages according to the negotiated protocol version. (TODO)

---

## Phase 9: Testing and Validation

Ensure the implementation is correct and compliant.

- [ ] **Task 9.1:** Write Unit Tests.
  - [ ] Write comprehensive unit tests for all components and handlers in the `server/` package.
- [ ] **Task 9.2:** Write Integration Tests.
  - [ ] Write integration tests to verify the interaction between different server components and with transports.
- [ ] **Task 9.3:** Protocol Compliance Testing.
  - [ ] Use or develop tools to test the server's adherence to the MCP specifications by sending and receiving protocol messages.
- [ ] **Task 9.4:** Address Compiler Errors and Warnings.
  - [ ] Continuously address any compiler errors or warnings that arise during development.

---

## Phase 10: Documentation and Clean Up

Finalize the implementation and documentation.

- [ ] **Task 10.1:** Add Code Comments.
  - [ ] Add clear and concise comments to the code, explaining complex logic and design decisions.
- [ ] **Task 10.2:** Update Documentation.
  - [ ] Update any existing documentation (e.g., in `docs/`) to reflect the implemented features and code structure.
- [ ] **Task 10.3:** Code Review and Refactoring.
  - [ ] Review the code for clarity, consistency, performance, and adherence to Go best practices. Refactor as needed.
- [ ] **Task 10.4:** Remove Placeholder Code.
  - [ ] Remove all `TODO` comments and placeholder logic once the actual implementation is complete.

---

This detailed plan provides a roadmap for achieving 100% compliance. It highlights the complexity involved and the need to resolve the fundamental compilation issues related to the `protocol` package before full implementation can proceed.
