# Server Refactoring Plan Checklist

This document outlines a plan to refactor the `server/server.go` and `server/helpers.go` files to improve code organization, maintainability, and simplicity, drawing inspiration from the modular structure in the `site` directory.

**Goal:** Break down the large `server/server.go` file into smaller, logically grouped files and integrate relevant helper functions from `server/helpers.go`.

**Checklist:**

- [x] **Phase 1: Hooks Migration**

  - [x] Ensure `server/hooks.go` exists with the `Hooks` struct and its associated registration and execution methods. (Completed)
  - [ ] Modify the `Server` struct in `server/server.go` to embed the `Hooks` struct.
    - [ ] Remove the individual hook slice fields (e.g., `beforeHandleMessageHooks`, `beforeUnmarshalHooks`, etc.).
    - [ ] Remove the `hooksMu` mutex field.
    - [ ] Add `*Hooks` to embed the struct.
  - [ ] Update the `NewServer` function in `server/server.go`.
    - [ ] Remove the initialization of individual hook slices.
    - [ ] Initialize the embedded `Hooks` struct by calling `NewHooks()`.
  - [ ] Update calls to hook execution methods within `server/server.go`.
    - [ ] Change calls like `s.beforeHandleMessageHooks(...)` to `s.Hooks.runBeforeHandleMessageHooks(...)`.
    - [ ] Update calls for all hook types (`beforeUnmarshalHooks`, `serverBeforeHandleRequestHooks`, etc.).
  - [ ] Remove the unused `time` import in `server/server.go`.
  - [ ] Address any compiler errors in `server/server.go` related to the hook migration (e.g., incorrect method calls, argument mismatches based on the `hooks.go` definitions).
  - [ ] **BLOCKING ISSUE:** Address compiler errors in `server/server.go` and `server/hooks.go` related to the hook migration (e.g., incorrect method calls, argument mismatches based on the `hooks.go` definitions, undefined symbols like `ServeWebsocket`, `ServeSSE`, protocol methods, and auth permissions).

- [ ] **Phase 2: Transport Management Refactoring**

  - [ ] Create a new file: `server/transport.go`.
  - [ ] Define a struct (e.g., `TransportManager`) in `server/transport.go` to hold transport-related configuration and methods.
  - [ ] Move the `TransportConfig` struct definition to `server/transport.go`.
  - [ ] Move the `EnableStdio`, `EnableWebsocket`, and `EnableSSE` methods from `server/server.go` to `server/transport.go`.
    - [ ] Update their receiver to use the new `TransportManager` struct (e.g., `func (tm *TransportManager) EnableStdio() *TransportManager`).
    - [ ] Update the return type to be chainable with the `TransportManager`.
  - [ ] Move the `Run` method from `server/server.go` to `server/transport.go`.
    - [ ] Update its receiver to use the `TransportManager` struct.
    - [ ] Ensure calls to `ServeStdio`, `ServeWebsocket`, and `ServeSSE` within `Run` are correct and have the right number of arguments (addressing the "not enough arguments" error for `ServeSSE`). This might require examining `server/sse_server.go` and `server/websocket_server.go` to confirm the correct function signatures.
  - [ ] Modify the `Server` struct in `server/server.go` to embed the `TransportManager` struct.
  - [ ] Update the `NewServer` function in `server/server.go` to initialize the embedded `TransportManager` struct.
  - [ ] Update calls to transport methods in `server/server.go` (e.g., in `main` function or wherever the server is configured and run) to use the embedded `TransportManager` (e.g., `srv.EnableStdio()`).

- [ ] **Phase 3: Registry Management Refactoring**

  - [ ] Create a new file: `server/registry.go`.
  - [ ] Define a struct (e.g., `Registry`) in `server/registry.go` to hold the registry maps.
    - [ ] Include `toolRegistry`, `toolHandlers`, `resourceRegistry`, `promptRegistry`, and `registryMu`.
  - [ ] Move the `RegisterTool`, `RegisterResource`, `UnregisterResource`, `ResourceRegistry`, `RegisterPrompt`, and `UnregisterPrompt` methods from `server/server.go` to `server/registry.go`.
    - [ ] Update their receiver to use the `Registry` struct.
  - [ ] Modify the `Server` struct in `server/server.go` to embed the `Registry` struct.
  - [ ] Update the `NewServer` function in `server/server.go` to initialize the embedded `Registry` struct.
  - [ ] Update calls to registry methods within `server/server.go` and other files to use the embedded `Registry`.

- [ ] **Phase 4: Message Handling and Dispatch Refactoring**

  - [ ] Create a new file: `server/messaging.go`.
  - [ ] Define a struct (e.g., `MessageHandler`) in `server/messaging.go` to hold message handling logic.
    - [ ] Include `activeRequests`, `requestMu`, `notificationHandlers`, and `notificationMu`.
    - [ ] The `MessageHandler` struct will likely need a reference back to the main `Server` struct (or relevant parts like the embedded `Hooks` and `Registry`) to access necessary data and call other methods. Consider passing these dependencies during `MessageHandler` initialization.
  - [ ] Move the `HandleMessage`, `handleSingleMessage`, `handleRequest`, and `handleNotification` methods from `server/server.go` to `server/messaging.go`.
    - [ ] Update their receiver to use the `MessageHandler` struct.
    - [ ] Update internal calls within these methods to use the embedded `Hooks` and `Registry` (e.g., `mh.Hooks.runBeforeHandleMessageHooks(...)`).
  - [ ] Move the `RegisterNotificationHandler` method to `server/messaging.go`.
    - [ ] Update its receiver to use the `MessageHandler` struct.
  - [ ] Modify the `Server` struct in `server/server.go` to embed the `MessageHandler` struct.
  - [ ] Update the `NewServer` function in `server/server.go` to initialize the embedded `MessageHandler` struct, passing necessary dependencies.
  - [ ] Update calls to message handling methods in `server/server.go` (e.g., in transport handling logic) to use the embedded `MessageHandler`.

- [ ] **Phase 5: Protocol Message Handlers Refactoring**

  - [ ] Consider creating a new directory: `server/handlers`.
  - [ ] Create separate files within `server/handlers` for different categories of protocol message handlers (e.g., `server/handlers/tool.go`, `server/handlers/resource.go`, `server/handlers/lifecycle.go`, `server/handlers/utility.go`).
  - [ ] Move the individual `handle...` functions for specific protocol methods (e.g., `handleListToolsRequest`, `handleReadResource`, `handleInitializationMessage`, `handlePing`) from `server/server.go` to the appropriate files in `server/handlers`.
    - [ ] These handler functions will need access to the `Server`'s registries, session management, and sending capabilities. Consider defining an interface or passing a struct containing the necessary dependencies to these handler functions.
  - [ ] Update the dispatch logic in `server/messaging.go` (specifically in `handleRequest` and `handleNotification`) to call these new handler functions in `server/handlers`.
  - [ ] Address compiler errors related to accessing `Server` fields/methods from the moved handler functions by using the dependency injection approach chosen.

- [ ] **Phase 6: Response/Notification Sending Refactoring**

  - [ ] Create a new file: `server/sender.go`.
  - [ ] Define a struct (e.g., `MessageSender`) in `server/sender.go` to hold message sending logic.
    - [ ] The `MessageSender` struct will likely need a reference back to the main `Server` struct (or relevant parts like sessions and hooks) to access necessary data and call other methods. Consider passing these dependencies during `MessageSender` initialization.
  - [ ] Move the `SendProgress`, `NotifyResourceUpdated`, `broadcastNotification`, and `sendNotificationToSession` methods from `server/server.go` to `server/sender.go`.
    - [ ] Update their receiver to use the `MessageSender` struct.
    - [ ] Update internal calls within these methods to use the embedded `Hooks` (e.g., `ms.Hooks.runServerBeforeSendNotificationHooks(...)`).
  - [ ] Move the `createSuccessResponse` and `createErrorResponse` helper functions to `server/sender.go`.
  - [ ] Modify the `Server` struct in `server/server.go` to embed the `MessageSender` struct.
  - [ ] Update the `NewServer` function in `server/server.go` to initialize the embedded `MessageSender` struct, passing necessary dependencies.
  - [ ] Update calls to sending methods within `server/server.go` and other new files to use the embedded `MessageSender`.

- [ ] **Phase 7: Refactor `server/helpers.go`**

  - [ ] Read the content of `server/helpers.go`.
  - [ ] Identify functions that logically belong with the new modules (transport, registry, messaging, sender, handlers).
  - [ ] Move those functions to the appropriate new files.
  - [ ] Update imports and calls in all affected files.
  - [ ] If any general utility functions remain, keep them in `server/helpers.go` or consider moving them to a more general `server/util.go` if appropriate.

- [ ] **Phase 8: Update Imports and References**

  - [ ] Go through all modified and new `.go` files in the `server` directory.
  - [ ] Ensure all necessary imports are present and correct for the types and functions used from other packages (like `context`, `encoding/json`, `fmt`, `reflect`, `sync`, `time`, `auth`, `hooks`, `logx`, `protocol`, `types`, `schema`) and from the newly created local packages within `server`.
  - [ ] Update any references to moved types, fields, or methods to use the correct embedded struct or package name (e.g., `s.hooksMu` becomes `s.Hooks.hooksMu`, `handleInitializeRequest` might become `s.MessageHandler.handleInitializeRequest` or `handlers.HandleInitializeRequest`).

- [ ] **Phase 9: Run Tests**

  - [ ] Run the existing tests in `server/server_test.go` and any other test files in the `server` directory.
  - [ ] Address any test failures resulting from the refactoring.
  - [ ] Consider writing new tests for the functionality moved to the new files to ensure adequate test coverage.

- [ ] **Phase 10: Review and Clean Up**
  - [ ] Review the entire `server` directory for clarity, consistency, and adherence to Go best practices.
  - [ ] Ensure naming conventions are consistent.
  - [ ] Remove any unused code, comments, or imports.
  - [ ] Update documentation (if any) to reflect the new structure.

This checklist provides a detailed roadmap for the refactoring process. It breaks down the task into manageable steps, starting with the hooks migration as requested. Each step involves moving related code and updating the references, followed by testing to ensure correctness.

That’s definitely already a lot more concise than the “raw” server wiring you had before. It reads almost like the Python version—but there are a few Go-idiomatic tweaks you could layer on to make it feel even more ergonomic:

1. **Chainable API**
   Instead of calling `server.AddTool(...)` as a free function, return the server from each call so you can fluently chain:

   ```go
   func main() {
     mcp := server.NewServer("Demo 🚀").
       AddPrompt(server.Prompt{
         Title:       "Add two numbers",
         Description: "Add two numbers",
         Messages: []protocol.PromptMessage{
           {Role: "system", Content: []protocol.Content{server.Text("You are a helpful assistant that adds two numbers.")}},
           {Role: "user",   Content: []protocol.Content{server.Text("What is 2 + 2?")}},
         },
       }).
       AddTool("add", "Add two numbers", func(args struct{ A, B int }) (protocol.Content, error) {
         return server.Text(fmt.Sprintf("%d", args.A+args.B)), nil
       })

     if err := mcp.WithStdio(); err != nil {
       log.Fatal(err)
     }
   }
   ```

   Now you only ever refer to `mcp` and can collapse your imports a bit.

2. **Tool registration via method**
   Embed the tool-registration into the server type so you don’t import both `server` and `protocol` at the call site:

   ```go
   // in your sdk/server.go
   func (s *Server) Tool[Args any, Ret any](
     name, desc string,
     fn func(Args) (Ret, error),
   ) *Server {
     wrapper := func(raw json.RawMessage) (protocol.Content, error) {
       var args Args
       if err := json.Unmarshal(raw, &args); err != nil {
         return nil, err
       }
       result, err := fn(args)
       if err != nil {
         return nil, err
       }
       // assume Ret implements fmt.Stringer or use fmt.Sprint
       return Text(fmt.Sprint(result)), nil
     }
     AddTool(s, name, desc, wrapper)
     return s
   }
   ```

   Then your `main()` becomes:

   ```go
   func main() {
     server.New("Demo 🚀").
       Prompt("Add two numbers", "Add two numbers",
         system("You are a helpful assistant that adds two numbers."),
         user("What is 2 + 2?"),
       ).
       Tool[struct{ A, B int }, int]("add", "Add two numbers", func(a struct{ A, B int }) int {
         return a.A + a.B
       }).
       RunStdIO()
   }
   ```

   That’s almost as terse as the Python decorator style, but in Go.

3. **Automatic prompt builders**
   Define helpers for common prompt roles:

   ```go
   func system(msg string) protocol.PromptMessage {
     return protocol.PromptMessage{Role: "system", Content: []protocol.Content{Text(msg)}}
   }
   func user(msg string) protocol.PromptMessage {
     return protocol.PromptMessage{Role: "user", Content: []protocol.Content{Text(msg)}}
   }
   ```

   Now you can inline your messages without repeating the struct literal.

---

#### Verdict

What you have is “simple enough” as a first draft. If you want the Go experience to feel even more like Python’s decorator style, I’d recommend:

- Turning `AddTool` into a method on your server type
- Adding generics over the args/result so handlers stay type-safe
- Providing small helpers (e.g. `system()`, `user()`) for prompt construction
- Exposing a final `Run()` or `RunStdIO()` chain that bundles `WithStdio()` under the hood

With those tweaks, you’ll hit the sweet spot: a Go API that’s ergonomic, type-safe, and as easy to read as the Python version.
