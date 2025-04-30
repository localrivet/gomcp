# MCP Specification Adherence Report (2025-03-26) - Detailed

This report summarizes the adherence of the `gomcp` codebase to the key changes and overall structure of the Model Context Protocol specification version `2025-03-26`, compared to the previous version `2024-11-05`.

## Major Changes from Changelog (`specifications/2025-03-26/changelog.md`)

1.  **Authorization Framework (OAuth 2.1 based):**

    - **Status:** ✅ Implemented
    - **Details:** The `auth/` directory contains core interfaces (`TokenValidator`, `PermissionChecker`), a `JWKSTokenValidator`, and integration hooks (`NewAuthenticationHook`). Server options (`WithAuth`, `WithJWTAuth`) allow easy configuration. The transport layer correctly extracts tokens (`Authorization` header) and places them in the context. Protocol capabilities (`Authorization` field in `ClientCapabilities`/`ServerCapabilities`) are defined and handled correctly based on protocol version. The server advertises the capability when configured.

2.  **Streamable HTTP Transport (Replacing HTTP+SSE):**

    - **Status:** ✅ Implemented
    - **Details:** The transport implementation (`transport/sse/`) uses a hybrid approach: SSE for server-to-client events and standard HTTP POST for client-to-server messages. This constitutes a "Streamable HTTP" transport. The implementation correctly handles protocol version differences for endpoint determination between `2024-11-05` and `2025-03-26`. Other transports (`stdio`, `websocket`, `tcp`) are also available.

3.  **JSON-RPC Batching:**

    - **Status:** ✅ Implemented
    - **Details:** The server (`server/server.go`) detects batch requests (JSON arrays). It processes them only if the negotiated protocol version is `2025-03-26`, correctly returning an error for `2024-11-05`. The client (`client/client.go`) can also handle incoming batch responses/notifications. Tests (`server/server_test.go`) confirm this version-dependent behavior.

4.  **Tool Annotations:**
    - **Status:** ✅ Implemented
    - **Details:** The `protocol/tools.go` file defines the `ToolAnnotations` struct (including `ReadOnlyHint`, `DestructiveHint`, etc.) within `ToolDefinition`. Examples (`examples/kitchen-sink/server/main.go`) demonstrate usage.

## Package-Level Adherence Review

- **`protocol/`:**
  - **Status:** ✅ Adherent
  - **Details:** Defines core JSON-RPC structures, MCP error codes, initialization/capability messages, feature definitions (Tools, Resources, Prompts, Sampling), content types (including `AudioContent`), and utilities (Logging, Progress, Cancellation) accurately reflecting the `2025-03-26` spec. Includes version-specific types where necessary (`V20250326`).
- **`transport/`:**
  - **Status:** ✅ Adherent
  - **Details:** Provides multiple transport options. The primary `sse` transport implements the "Streamable HTTP" requirement for `2025-03-26`. Other transports (`stdio`, `websocket`, `tcp`) are standard implementations. Transports correctly handle message framing and context propagation.
- **`server/`:**
  - **Status:** ✅ Adherent
  - **Details:** Implements robust server logic including initialization handshake, capability negotiation (respecting version), request/notification routing, batch handling, feature registries (tools, resources, prompts), session management, hook system, and configuration options. Integrates cleanly with transports and the auth framework.
- **`client/`:**
  - **Status:** ✅ Largely Adherent (Minor Gap Identified)
  - **Details:** Implements core client logic: connection, initialization handshake, capability negotiation, sending requests/notifications, processing responses/notifications (including batches), state management, and transport integration. Supports tool calls (`CallTool`) and handling sampling requests (`RegisterSamplingHandler`).
  - **Gap:** Lacks dedicated public API methods for direct client interaction with server resources (e.g., `ListResources`, `ReadResource`, `SubscribeResource`) and prompts (e.g., `ListPrompts`, `GetPrompt`). While the client might process related _notifications_ if handlers are registered, it doesn't provide functions to _initiate_ these requests.

## Other Schema Changes (Brief Check)

- **`message` field in `ProgressNotification`:** Implemented (`protocol/messages.go`, `server/server.go`, `client/client.go`).
- **Audio data support:** Implemented (`protocol/messages.go`).
- **`completions` capability:** Implemented (`protocol/messages.go`, `server/server.go`). Server advertises based on config/version. Client stores capability but doesn't use it directly.

## Conclusion

The `gomcp` codebase demonstrates strong adherence to the `2025-03-26` MCP specification across most areas, including protocol definitions, transport mechanisms, and server-side logic. The major changes from the specification update are well-implemented.

The primary area for potential improvement is the **client package's public API**, which currently lacks explicit methods for initiating resource and prompt requests, potentially limiting its utility for certain use cases compared to the server's capabilities.
