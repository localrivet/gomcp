# MCP Client Helpers Demo

This example directory explores patterns for creating helper functions to simplify using the Go MCP client (`github.com/localrivet/gomcp/client`).

## Goal

The primary goal is to develop a configuration loading mechanism (`LoadServersFromConfig`) that allows a client application to connect to multiple MCP servers based on a configuration structure. This loader should:

1.  Parse a configuration defining different servers.
2.  Support connecting to servers using different methods based on a prefix in the configuration's `command` field:
    - `@sse:<url>`: Connect to a running server via Server-Sent Events.
    - `@ws:<url>`: Connect to a running server via WebSocket.
    - `@stdio:<command_string>`: **(Intended)** Launch the specified command as a local process and connect to its standard input/output streams.
3.  Return a map of connected `*client.Client` instances, keyed by the server name from the configuration.

## Current State & Iteration

We iterated on the implementation within `loader.go`:

1.  **Initial Idea:** Support all prefixes, including launching local processes for `@stdio:` and potentially `@docker:`.
2.  **Blocker Identified:** Realized that the standard `client.NewStdioClient` connects to the _current application's_ stdio, not the stdio pipes of an externally launched process. Manually constructing the `client.Client` struct with pipes failed due to unexported fields.
3.  **Hypothetical Solution:** Proposed needing a new constructor in the core `client` package (e.g., `NewClientFromPipes` or `NewClientWithTransport`) that could accept pre-existing `io.Reader`/`io.WriteCloser` pipes.
    A placeholder function (`newClientFromPipes`) was added to `loader.go` to represent this needed functionality.
4.  **Current Code (Connect-Only Pivot):** To proceed without modifying the core `client` package _yet_, the code was simplified to **only support connection prefixes (`@sse:`, `@ws:`)**. The `@stdio:` prefix was temporarily redefined to use `client.NewStdioClient` (connecting to the _app's_ stdio), and process launching logic (`@docker:`, `default` case, `exec.Cmd` handling, `ManagedClient`, `TerminateManagedClients`) was **removed**.

**Therefore, the current code in `loader.go` does NOT implement the intended functionality for `@stdio:<command_string>` (launching the command).** It only handles connections via `@sse:`, `@ws:`, and the application's own `@stdio:`.

## Next Steps

- Review the current `LoadServersFromConfig` implementation which focuses on connections.
- Decide whether to pursue adding the necessary constructor (like `NewClientFromPipes`) to the core `client` package to enable launching and connecting to stdio-based subprocesses as originally intended for `@stdio:<command_string>`.
- Explore other client helper patterns (e.g., simplified tool calling).
