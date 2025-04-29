# Hello‑Demo – Tiny MCP Server Example

A minimal Go program that shows **all three** MCP primitives in action:

| Primitive    | Demo Name  | Purpose                                                |
| ------------ | ---------- | ------------------------------------------------------ |
| **Tool**     | `hello`    | Returns a plain‑text greeting                          |
| **Prompt**   | `greet`    | Sends an assistant message (useful for chat preambles) |
| **Resource** | `icon.png` | Serves a binary blob (here, fake Base64 PNG data)      |

---

## Why This Exists

- **Boilerplate starter** – Copy‑paste to spin up your own tool/prompt/resource server.
- **End‑to‑end reference** – Shows how each primitive is registered and invoked.
- **Tiny surface** – Under 70 LOC, easy to grok.

---

## Run It

> Requires Go 1.24+

```bash
go run .
```

The server listens on **stdio** (great for ChatGPT plug‑in testing). You can pipe JSON requests directly:

```bash
# Call the hello tool
printf '{"method":"hello","params":{"name":"Alma"}}\n' | go run .
```

Expected response:

```jsonc
{ "role": "tool", "name": "hello", "content": "Hello, Alma!" }
```

---

## Anatomy of `main.go`

1. **Server init**
   ```go
   srv := server.NewServer("demo-server")
   ```
2. **Tool** – `hello`
   ```go
   server.AddTool(srv, "hello", "Say hello to someone", func(args HelloArgs) (protocol.Content, error) {
       return server.Text(fmt.Sprintf("Hello, %s!", args.Name)), nil
   })
   ```
3. **Prompt** – `greet`
   ```go
   server.AddPrompt(srv, "greet", "Greeting prompt", func(_ HelloArgs) (protocol.PromptMessage, error) {
       return server.Message("assistant", "How can I help you today?"), nil
   })
   ```
4. **Resource** – `icon.png`
   ```go
   server.AddResource(srv, "icon.png", "image/png", "Server icon", "1.0", fetchIcon)
   ```
5. **Serve**
   ```go
   server.ServeStdio(srv)
   ```

---

## Extending

- **Add auth** – Wrap handlers with an auth guard before calling `AddTool`/`AddPrompt`.
- **Swap transport** – Use `ServeHTTP` or gRPC instead of stdio if you need network access.
- **Real resources** – Load files from disk or S3; set correct `ContentType`.

---

## Directory Layout

```
hello-demo/
├── main.go           # this sample program
└── README.md
```
