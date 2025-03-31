---
title: Protocol Tools
weight: 22
---

# Protocol Tools

MCP servers can expose tools that clients can execute. This document describes the protocol messages related to tool discovery and execution.

## Tool Definition (`Tool`)

Represents a tool offered by the server.

- `name` (string, required): The unique identifier for the tool.
- `description` (string, optional): A human-readable description of what the tool does.
- `inputSchema` (object, required): A JSON Schema (`ToolInputSchema`) defining the expected arguments for the tool.
  - `type` (string, required): Typically "object".
  - `properties` (object, optional): A map where keys are argument names and values are `PropertyDetail` objects describing the argument (type, description, enum, format).
  - `required` (array, optional): A list of required argument names (strings).
- `annotations` (object, optional): Optional hints about the tool's behavior (`ToolAnnotations`).
  - `title` (string, optional): Human-readable title.
  - `readOnlyHint` (boolean, optional): Indicates if the tool only reads data.
  - `destructiveHint` (boolean, optional): Indicates if the tool might modify or delete data.
  - `idempotentHint` (boolean, optional): Indicates if calling the tool multiple times with the same arguments has the same effect as calling it once.
  - `openWorldHint` (boolean, optional): Indicates if the tool interacts with external systems or the real world.

## Request Messages

### `tools/list`

Sent from the client to the server to retrieve the list of available tools. Supports pagination.

**Parameters (`ListToolsRequestParams`):**

- `cursor` (string, optional): A cursor returned from a previous `tools/list` response to fetch the next page of results.

### `tools/call`

Sent from the client to the server to execute a specific tool with provided arguments.

**Parameters (`CallToolParams`):**

- `name` (string, required): The name of the tool to execute.
- `arguments` (object, optional): A map containing the arguments for the tool, conforming to the tool's `inputSchema`.
- `_meta` (object, optional): Metadata associated with the request (`RequestMeta`), potentially including a `progressToken`.

## Response Messages

### `tools/list` (Result)

The successful response to a `tools/list` request.

**Payload (`ListToolsResult`):**

- `tools` (array, required): A list of available tools (`Tool[]`).
- `nextCursor` (string, optional): A cursor to use in a subsequent `tools/list` request to fetch the next page of results. If omitted, there are no more tools.

### `tools/call` (Result)

The successful response to a `tools/call` request.

**Payload (`CallToolResult`):**

- `content` (array, required): The result of the tool execution, represented as an array of `Content` objects (e.g., `TextContent`, `ImageContent`).
- `isError` (boolean, optional): If true, indicates the `content` represents an error message rather than a successful result.
- `_meta` (object, optional): Metadata associated with the response (`RequestMeta`).

## Notification Messages

### `notifications/tools/list_changed`

Sent from the server to the client when the set of available tools has changed (e.g., tools added or removed). The client should typically re-fetch the tool list using `tools/list`.

**Parameters (`ToolsListChangedParams`):**

- _(None)_ - The params object is empty.
