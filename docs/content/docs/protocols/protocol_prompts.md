---
title: Protocol Prompts
weight: 40 # Fourth in Protocols section
---

MCP servers can expose named prompts, which represent pre-defined instructions or templates that clients can utilize, often in conjunction with sampling requests. This document describes the protocol messages related to prompt discovery.

## Prompt Definition (`Prompt`)

Represents a named prompt template provided by the server.

- `uri` (string, required): A unique identifier for the prompt.
- `title` (string, optional): A short, human-readable title.
- `description` (string, optional): A longer description of the prompt's purpose.
- `arguments` (array, optional): A list of arguments (`PromptArgument[]`) that can be used to customize the prompt template. Each argument has:
  - `name` (string, required)
  - `description` (string, optional)
  - `type` (string, required): e.g., "string", "number", "boolean".
  - `required` (boolean, optional)
- `messages` (array, required): The sequence of messages (`PromptMessage[]`) that make up the prompt template. Each message has:
  - `role` (string, required): e.g., "system", "user", "assistant".
  - `content` (array, required): The content parts (`Content[]`) for the message. Content within prompts often includes template variables (syntax not defined by MCP core).
- `metadata` (object, optional): Additional arbitrary key-value pairs.

## Request Messages

### `prompts/list`

Sent from the client to the server to retrieve a list of available prompts, potentially filtered. Supports pagination.

**Parameters (`ListPromptsRequestParams`):**

- `filter` (object, optional): Criteria to filter the prompts (specific filter structure not defined by the core protocol).
- `cursor` (string, optional): A cursor from a previous response to fetch the next page.

### `prompts/get`

Sent from the client to the server to retrieve a specific prompt definition, potentially resolving template arguments.

**Parameters (`GetPromptRequestParams`):**

- `uri` (string, required): The URI of the prompt to retrieve.
- `arguments` (object, optional): Values for the prompt's arguments, used for server-side template resolution if supported.

## Response Messages

### `prompts/list` (Result)

The successful response to a `prompts/list` request.

**Payload (`ListPromptsResult`):**

- `prompts` (array, required): A list of prompts (`Prompt[]`) matching the filter criteria for the current page.
- `nextCursor` (string, optional): A cursor for fetching the next page. Omitted if this is the last page.

### `prompts/get` (Result)

The successful response to a `prompts/get` request.

**Payload (`GetPromptResult`):**

- `prompt` (object, required): The requested `Prompt` definition. If arguments were provided in the request and the server supports template resolution, the `messages` content may be resolved.

## Notification Messages

### `notifications/prompts/list_changed`

Sent from the server to the client when the set of available prompts has changed (e.g., prompts added or removed). The client should typically re-fetch the prompt list using `prompts/list`.

**Parameters (`PromptsListChangedParams`):**

- _(None)_ - The params object is empty.
