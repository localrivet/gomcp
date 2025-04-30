# Hello Demo

> **Minimal MCP Server Showcase.** A tiny Go program demonstrating all three core MCP primitives: Tools, Prompts, and Resources. Ideal for understanding the basics or as a starting point for your own server.

---

## Why This Demo?

| Goal                  | How Hello Demo Helps                             |
| --------------------- | ------------------------------------------------ |
| Understand MCP Basics | Clearly shows registration and use of primitives |
| Quick Start Template  | Easy to copy and adapt for new MCP servers       |
| Simplicity & Clarity  | Minimal code (~70 LOC) focused on core concepts  |
| Primitive Reference   | Provides a concrete example for each MCP type    |

The demo focuses on clarity over complexity, providing a solid foundation.

---

## Primitive Line-Up

| Primitive | Name       | Responsibility                                     |
| --------- | ---------- | -------------------------------------------------- |
| Tool      | `hello`    | Returns a plain-text greeting based on input name  |
| Prompt    | `greet`    | Provides a standard assistant welcome message      |
| Resource  | `icon.png` | Serves a static binary blob (placeholder PNG data) |

---

## Architecture

```mermaid
flowchart TD
    subgraph "Client (e.g., Pipe via Stdio)"
        direction LR
        ReqHello["{\"method\":\"hello\"..."] --> Srv
        ReqGreet["{\"method\":\"greet\"..."] --> Srv
        ReqIcon["{\"method\":\"icon.png\"..."] --> Srv
    end
    subgraph "Hello Demo Server (Stdio Transport)"
        direction LR
        Srv(MCP Server Instance)
        Srv -- Registers --> T(Tool: hello)
        Srv -- Registers --> P(Prompt: greet)
        Srv -- Registers --> R(Resource: icon.png)
    end
    T -->|Response| ClientRes
    P -->|Response| ClientRes
    R -->|Response| ClientRes
    ClientRes[Response]
```

---

## Quick Start

> **Prerequisite:** Go 1.24+

Run the server (listens on stdio):

```bash
go run .
```

Send a request via stdin:

```bash
# Call the hello tool
printf '{"method":"hello","params":{"name":"World"}}\n' | go run .
```

Expected response:

```jsonc
// Example response for 'hello' tool
{
  "role": "tool",
  "name": "hello",
  "content": {
    "type": "text",
    "text": "Hello, World!"
  },
  "isError": false
}
```

---

## Features

- **Tool Implementation:** Demonstrates a simple tool (`hello`) that takes arguments and returns text content.
- **Prompt Implementation:** Shows how to register a prompt (`greet`) for generating canned messages.
- **Resource Implementation:** Provides an example of serving static binary data (`icon.png`) via a resource.
- **Server Setup:** Minimal server initialization using `gomcp/server`.
- **Stdio Transport:** Utilizes the standard input/output transport, suitable for simple integrations or testing.

---

## Extending the Demo

- **Add Authentication:** Secure the `hello` tool using techniques from the `auth` example.
- **Swap Transport:** Replace `ServeStdio` with `ServeHTTP` (using SSE or WebSockets) for network accessibility.
- **Real Resources:** Modify `icon.png` to load an actual image file from disk.
- **More Primitives:** Add more complex tools, prompts with arguments, or different resource types.
- **Configuration:** Introduce configuration loading (e.g., for server name or ports).

---

## Project Structure

```
hello-demo/
├── main.go     # Server implementation with tool, prompt, resource
└── README.md   # You are here
```
