---
title: Protocol Resources
weight: 30 # Third in Protocols section
---

MCP servers can expose resources, which represent data sources that clients can access or subscribe to. This document describes the protocol messages related to resource discovery and access.

## Resource Definition (`Resource`)

Represents a data source provided by the server.

- `uri` (string, required): A unique identifier for the resource (e.g., `file:///path/to/file`, `api://service/endpoint`).
- `kind` (string, optional): A category or type hint for the resource (e.g., "file", "api_spec", "database_table").
- `title` (string, optional): A short, human-readable title for the resource.
- `description` (string, optional): A longer description of the resource.
- `version` (string, optional): An opaque string representing the current version of the resource content. This should change whenever the content changes.
- `metadata` (object, optional): Additional arbitrary key-value pairs associated with the resource.

## Request Messages

### `resources/list`

Sent from the client to the server to retrieve a list of available resources, potentially filtered. Supports pagination.

**Parameters (`ListResourcesRequestParams`):**

- `filter` (object, optional): Criteria to filter the resources (specific filter structure not defined by the core protocol).
- `cursor` (string, optional): A cursor from a previous response to fetch the next page.

### `resources/get` (Corresponds to `resources/read` in Go code)

Sent from the client to the server to retrieve the content of a specific resource.

**Parameters (`ReadResourceRequestParams`):**

- `uri` (string, required): The URI of the resource to retrieve.
- `version` (string, optional): If provided, the server may return a "Not Modified" error if the resource version matches.

### `resources/subscribe`

Sent from the client to the server to request notifications when the content of specific resources changes.

**Parameters (`SubscribeResourceParams`):**

- `uris` (array of strings, required): A list of resource URIs to subscribe to.

### `resources/unsubscribe`

Sent from the client to the server to stop receiving notifications for specific resources.

**Parameters (`UnsubscribeResourceParams`):**

- `uris` (array of strings, required): A list of resource URIs to unsubscribe from.

## Response Messages

### `resources/list` (Result)

The successful response to a `resources/list` request.

**Payload (`ListResourcesResult`):**

- `resources` (array, required): A list of resources (`Resource[]`) matching the filter criteria for the current page.
- `nextCursor` (string, optional): A cursor for fetching the next page. Omitted if this is the last page.

### `resources/get` (Result - Corresponds to `resources/read` in Go code)

The successful response to a `resources/get` request.

**Payload (`ReadResourceResult`):**

- `resource` (object, required): The `Resource` object containing metadata (including the current `version`).
- `contents` (object, required): The actual content of the resource (`ResourceContents`). This will be either:
  - `TextResourceContents`: Contains `contentType` (string) and `content` (string).
  - `BlobResourceContents`: Contains `contentType` (string) and `blob` (string, base64 encoded).

### `resources/subscribe` (Result)

The successful response to a `resources/subscribe` request.

**Payload (`SubscribeResourceResult`):**

- _(None)_ - The result object is currently empty upon success.

### `resources/unsubscribe` (Result)

The successful response to an `resources/unsubscribe` request.

**Payload (`UnsubscribeResourceResult`):**

- _(None)_ - The result object is currently empty upon success.

## Notification Messages

### `notifications/resources/list_changed`

Sent from the server to the client when the set of available resources changes (e.g., resources added or removed). The client should typically re-fetch the resource list using `resources/list`.

**Parameters (`ResourcesListChangedParams`):**

- _(None)_ - The params object is empty.

### `notifications/resources/content_changed`

Sent from the server to the client when the content (and thus version) of a subscribed resource has changed.

**Parameters (`ResourceUpdatedParams`):**

- `resource` (object, required): The updated `Resource` object, containing the new `version` and potentially other changed metadata. The client should typically use `resources/get` to fetch the new content if needed.
