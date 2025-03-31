---
title: Protocol Messages
weight: 10 # First in Protocols section
---

This document describes the core JSON-RPC messages used in the Model Context Protocol (MCP).

## Request Messages

### `initialize`

Sent from the client to the server to initiate the connection and exchange capabilities.

**Parameters (`InitializeRequestParams`):**

- `protocolVersion` (string, required): The protocol version the client supports.
- `capabilities` (object, required): The capabilities the client supports (`ClientCapabilities`).
- `clientInfo` (object, required): Information about the client implementation (`Implementation`).
- `trace` (string, optional): Trace setting ('off', 'messages', 'verbose').
- `workspaceFolders` (array, optional): Workspace folders opened by the client (`WorkspaceFolder[]`).

**Result (`InitializeResult`):** See Response Messages section.

### `logging/set_level`

Sent from the client to the server to request a change in the server's logging verbosity.

**Parameters (`SetLevelRequestParams`):**

- `level` (string, required): The desired logging level (`"error"`, `"warn"`, `"info"`, `"debug"`, `"trace"`).

**Result:**

- _(None)_ - A successful response has an empty result.

### `sampling/create_message`

Sent from the client to the server to request a model-generated message based on a provided context.

**Parameters (`CreateMessageRequestParams`):**

- `context` (array, required): A list of messages (`SamplingMessage[]`) providing the conversation history or context. Each message has `role` (string), `content` (array of `Content` objects), and optional `name` (string).
- `preferences` (object, optional): Desired model characteristics (`ModelPreferences`), including `modelUri`, `temperature`, `topP`, `topK`.

**Result (`CreateMessageResult`):** See Response Messages section.

### `roots/list`

Sent from the server to the client to request the list of available root contexts (e.g., workspace folders, open files).

**Parameters (`ListRootsRequestParams`):**

- _(None)_ - The params object is currently empty.

**Result (`ListRootsResult`):** See Response Messages section.

_(Other request message details will go here)_

## Response Messages

### `initialize` (Result)

The successful response to an `initialize` request.

**Payload (`InitializeResult`):**

- `protocolVersion` (string, required): The protocol version the server supports.
- `capabilities` (object, required): The capabilities the server supports (`ServerCapabilities`).
- `serverInfo` (object, required): Information about the server implementation (`Implementation`).
- `instructions` (string, optional): Optional instructions for the client after initialization.

### `sampling/create_message` (Result)

The successful response to a `sampling/create_message` request.

**Payload (`CreateMessageResult`):**

- `message` (object, required): The generated message from the model (`SamplingMessage`).
- `modelHint` (object, optional): Information about the model used (`ModelHint`), including `modelUri`, `inputTokens`, `outputTokens`, `finishReason`.

### `roots/list` (Result)

The successful response to a `roots/list` request.

**Payload (`ListRootsResult`):**

- `roots` (array, required): A list of root contexts (`Root[]`) available on the client. Each root has `uri`, optional `kind`, `title`, `description`, and `metadata`.

_(Other response message details will go here)_

## Notification Messages

### `initialized`

Sent from the client to the server after the client has received and processed the `initialize` result, indicating readiness.

**Parameters (`InitializedNotificationParams`):**

- _(None)_ - The params object is empty.

### `notifications/message`

Sent from the server to the client to provide a log message. This is typically used when the server's logging capabilities are enabled.

**Parameters (`LoggingMessageParams`):**

- `level` (string, required): The severity level of the log message (`"error"`, `"warn"`, `"info"`, `"debug"`, `"trace"`).
- `message` (string, required): The log message content.

### `$/cancelled`

Sent from the client to the server to indicate that a previously sent request should be cancelled.

**Parameters (`CancelledParams`):**

- `id` (integer | string, required): The ID of the request to be cancelled.

### `$/progress`

Sent from the server to the client to report progress on a long-running operation initiated by a request.

**Parameters (`ProgressParams`):**

- `token` (string, required): The progress token associated with the request.
- `value` (any, required): The progress payload, specific to the operation being reported.

### `notifications/roots/list_changed`

Sent from the client to the server when the list of available roots has changed.

**Parameters (`RootsListChangedParams`):**

- _(None)_ - The params object is empty.

### `notifications/tools/list_changed`

Sent from the server to the client when the list of available tools has changed.

**Parameters (`ToolsListChangedParams`):**

- _(None)_ - The params object is empty.

### `notifications/resources/list_changed`

Sent from the server to the client when the list of available resources has changed.

**Parameters (`ResourcesListChangedParams`):**

- _(None)_ - The params object is empty.

### `notifications/prompts/list_changed`

Sent from the server to the client when the list of available prompts has changed.

**Parameters (`PromptsListChangedParams`):**

- _(None)_ - The params object is empty.

_(Other notification message details will go here)_
