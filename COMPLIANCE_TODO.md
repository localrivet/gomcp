# MCP Specification Compliance TODO (Dual Transport Support)

This checklist outlines the steps needed to ensure the `gomcp` project correctly supports **both** the `2024-11-05` (HTTP+SSE) and `2025-03-26` (WebSocket) transport mechanisms, aligning with the goal of dual specification adherence as noted in `SPEC_ADHERENCE_REPORT.md`.

## Tasks

- [ ] **1. Verify Transport Implementations:**

  - [ ] Review `transport/sse/` implementation against `2024-11-05` transport specifications.
  - [ ] Review `transport/websocket/` implementation against `2025-03-26` "Streamable HTTP transport" concept and requirements (including batching support).

- [ ] **2. Ensure Client Compatibility:**

  - [ ] Review `client/client.go` to confirm it remains transport-agnostic and correctly uses the `types.Transport` interface.
  - [ ] Verify that specific client constructors (e.g., `client/sse_client.go`, `client/websocket_client.go`) correctly instantiate and configure their respective transports.
  - [ ] Confirm that protocol version negotiation (during `client.Connect`) correctly influences any transport-specific behavior if necessary (e.g., SSE endpoint handling vs. single WebSocket endpoint).

- [ ] **3. Update and Run Tests:**

  - [ ] Ensure test suites (`client/*_test.go`, `transport/*/*_test.go`) adequately cover scenarios for **both** SSE and WebSocket transports.
  - [ ] Add tests specifically verifying protocol negotiation and correct transport behavior based on the negotiated version.
  - [ ] Run all project tests (`make test` or similar) and fix any failures. Pay close attention to tests involving server setup (like the previous failure in `server/sse_server.go` which likely needs its import fixed if it wasn't automatically).

- [ ] **4. Update Documentation:**
  - [ ] Review project documentation (`docs/`) for transport mechanism descriptions.
  - [ ] Update documentation to clearly state that **both** HTTP+SSE (for `2024-11-05` compatibility) and WebSocket (for `2025-03-26`) transports are supported.
  - [ ] Document how to select or configure the desired transport for client/server implementations.
