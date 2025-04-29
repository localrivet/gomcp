# MCP Specification Adherence Report (gomcp)

## Summary

The `gomcp` project demonstrates strong adherence to the `2025-03-26` specification at the protocol level but shows a transitional state regarding the transport layer, retaining compatibility with the `2024-11-05` specification.

## Adherence to `2025-03-26`

- **Protocol Messages & Features:** High Adherence.
  - Key changes from the `2025-03-26` changelog are implemented (Authorization capabilities, Audio content, Progress message field, Completions capability, updated Sampling structures, Tool Annotations).
  - Relevant files: `protocol/messages.go`, `protocol/tools.go`.
- **Transport:** Partial/Transitional Adherence.
  - The spec mentions replacing HTTP+SSE with a "Streamable HTTP transport".
  - A WebSocket transport (`transport/websocket/websocket.go`) exists, aligning with the "Streamable HTTP" concept.
  - However, the older HTTP+SSE transport (`transport/sse/client.go`) is still present and adapted, not fully replaced.
- **JSON-RPC Batching:** Enabled.
  - The WebSocket transport allows for sending/receiving batched requests.

## Adherence/Compatibility with `2024-11-05`

- **Protocol Messages & Features:** High Compatibility.
  - Code retains structures specific to `2024-11-05` for backward compatibility (e.g., older sampling structures).
- **Transport:** High Adherence.
  - The `transport/sse/client.go` file implements the HTTP+SSE mechanism characteristic of this version.

## Key Discrepancy & TODO

- **Issue:** The `2025-03-26` specification changelog implies the HTTP+SSE transport was replaced. However, this project aims to maintain compatibility by supporting _both_ the `2024-11-05` (HTTP+SSE) and `2025-03-26` (WebSocket) transport mechanisms.
- **TODO:** Verify that both the HTTP+SSE (`transport/sse/`) and WebSocket (`transport/websocket/`) transports function correctly according to their respective specification versions (`2024-11-05` and `2025-03-26`). Ensure clear mechanisms exist for selecting or negotiating the appropriate transport. Update tests and documentation to reflect dual transport support.
