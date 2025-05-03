# MCP Schema Compliance Plan (2024-11-05 & 2025-03-26)

This document outlines the remaining tasks to ensure the `gomcp` server fully complies with both the `2024-11-05` and `2025-03-26` MCP protocol specifications.

## I. Implement Missing Request Handlers

The following request methods defined in the schemas are currently missing handlers in `server/messaging.go`:

- [x] **`ping`**: Add handler to `handleRequest`, return `protocol.PingResult`. _(Test `TestHandleMessage_Ping` added)_
- [x] **`resources/list_templates`**: Add handler to `handleRequest`, implement retrieval logic, return `protocol.ListResourceTemplatesResult`. _(Test `TestHandleMessage_ResourcesListTemplates` added)_
- [x] **`prompts/get`**: Add handler to `handleRequest`, implement retrieval logic, return `protocol.GetPromptResult`. _(Test `TestHandleMessage_PromptsGet` added)_
- [x] **`completion/complete`**: Add handler for **argument autocompletion**. _(Handler structure and protocol types added; logic is placeholder)_

## II. Implement Missing Notification Handlers

- [x] **`exit`**: Add handler to `handleNotification`, call `mh.lifecycleHandler.ExitHandler()`. _(Test `TestHandleNotification_Exit` added, needs enhancement to verify side effect)_
- [x] **List Change Notifications (`resources/list_changed`, `prompts/list_changed`, `tools/list_changed`)**:
  - [x] Implement server-side change detection logic (via Registry callbacks).
  - [x] Construct appropriate notification params (nil).
  - [x] Send notifications based on negotiated capabilities.
    - _(Test `TestNotificationSending_ListChanged` verifies sending based on caps)_.

## III. Handle Completion/Sampling Requests (Schema Compliance)

- **Note:** The MCP specification defines `completion/complete` (argument autocompletion) and `sampling/createMessage` (server-to-client LLM generation request). The older `completion/complete` name caused confusion.
- [x] Implement handler for `sampling/createMessage` (2025-03-26 spec) via `Context.CreateMessage`.
  - [x] Tested (`TestContext_CreateMessage`).

## IV. Implement Tool Calling

- [x] Implement `tools/call` handler logic in `messaging.go`.
  - [x] Retrieve tool definition and handler from `Registry`.
  - [x] Create `server.Context` for the tool call.
  - [x] Handle parameter marshalling/validation for the tool function.
  - [x] Call the tool function.
  - [x] Format the result/error according to the **negotiated protocol version**:
    - V2025 (`protocol.CallToolResult`): Uses `Output` (raw JSON) and structured `Error`.
    - V2024 (`protocol.CallToolResultV2024`): Uses `Content` (slice) and `IsError` (boolean).
  - [x] Handle progress reporting (`$/progress`) if `_meta.progressToken` is present.
- [x] Implement `Context.CallTool` method in `context.go`.
  - [x] Allow tools to call other tools via the context.
  - [x] Send `tools/call` request **to the client**.
  - [x] Handle response/error from the client.
- [x] Implement tests for tool calling (`TestHandleMessage_ToolsCall_*`, `TestContext_CallTool`). _(`TestHandleMessage_ToolsCall` passes, `TestContext_CallTool` added)_

## V. Implement Server-Initiated Sampling Request

- [x] Add `Context.CreateMessage` method in `context.go`.
  - [x] Allow server-side logic (e.g., within a tool) to request LLM completion via the client.
  - [x] Method should construct and send a `sampling/createMessage` request **to the client session**.
  - [x] Method should wait for and handle the response (`protocol.SamplingResult` or error) from the client.
  - [x] Requires adding `SendRequest` to `types.ClientSession` and handling incoming responses in `MessageHandler`.
- [x] Implement tests for `Context.CreateMessage`.

## VI. Implement Remaining TODOs & Refinements

Review and complete all outstanding `// TODO:` items within `server/messaging.go`, `server/lifecycle_handler.go`, and potentially related files (`server/server.go`, `server/context.go`). Key areas include:

- [x] **Implement `completion/complete` Logic:** Add actual suggestion generation logic (beyond placeholder) if needed.
- [x] **`logging/set_level`:** Implement actual logging level adjustment logic. (Level stored in server, applied to logx). _(Test `TestHandleMessage_LoggingSetLevel` added)_
- [x] **`$/cancelled`:** Implement request cancellation mechanism (using context.Context). _(Test `TestHandleNotification_Cancelled` passed)_
- [x] **`notifications/message` (from client):** Integrate received client logs with the server's logging system. _(Test `TestHandleNotification_ClientMessage` passed - basic check)_
- [x] **`resources/read` (audio/blob):** Implement reading and encoding for audio resources. _(Test `TestHandleMessage_ResourcesRead_Success` passed)_
- [x] **Capability Flags (`server.Implements*`)**: Ensure flags accurately reflect features and are used in negotiation. (Reviewed, `ImplementsResourceListChanged` set to false). _(Tested implicitly via `TestLifecycle_Initialize_`\* tests)\_
- [x] **`Context` methods:** Implement `Log`, `ReportProgress`, `ReadResource`, `CallTool`, `CreateMessage` etc. (All core methods implemented). _(Tests `TestContext_`\* added/passed for implemented methods)\_\n- [x] **Error Handling/Marshalling:** Address TODOs related to error handling during message sending/marshalling. _(General tests cover some errors;`ErrorCode` refactor helped significantly)\_
- [x] **Server Version Info:** Populate `ServerInfo.Version` dynamically in `InitializeHandler` (using constant). _(Tested via `TestLifecycle_Initialize_`\* tests)\_
- [x] **Standardize Logging:** Replace standard `log` usage with `logx`. _(Test `TestNotificationSending_Logging` added)_

By addressing these points, the server should achieve full compliance with both specified MCP schema versions.

## VII. FastMCP Resource Parity (NEW REQUIREMENT)

Achieve functional parity with FastMCP's resource and resource template system.

- [x] **Design Resource Template Storage & Registration:**
  - [x] Modify `server.Registry` to store URI template patterns and associated handler functions.
  - [x] Implement `Server.ResourceTemplate` registration method (using reflection to analyze handler signature and match URI parameters).
  - [x] Implement URI template parsing logic within registration.
- [x] **Implement URI Template Matching:**
  - [x] Integrate or implement RFC 6570-like URI template matching logic.
  - [x] Modify `resources/read` handler in `messaging.go` to attempt template matching after static URI lookup.
- [x] **Implement Template Parameter Extraction & Injection:**
  - [x] Extract parameter values from matched URIs.
  - [x] Convert extracted string parameters to handler function argument types (using reflection).
  - [x] Invoke template handler function with extracted parameters (using reflection).
- [x] **Handle Return Values:**
  - [x] Ensure template handler return values (`string`, `map`, `struct`, `[]byte`) are correctly converted to `protocol.ResourceContents`.
- [ ] **(Stretch Goal) Wildcard Parameter Support (`{param*}`):**
  - [ ] Extend URI matching logic to handle wildcards.
- [ ] **(Stretch Goal) Default Parameter Value Support:**
  - [ ] Modify handler invocation logic to use default values for function parameters not present in the URI.
- [x] **Add Tests for Resource Templates:**
  - [x] Unit tests for URI matching and parameter extraction.
  - [x] Integration tests for `resources/read` with template URIs.
  - [x] Tests for different parameter types, return types, wildcards, and defaults (if implemented).
