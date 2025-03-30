---
layout: default
title: Basic Usage
nav_order: 4 # After Installation
---

# Basic Usage

The core logic resides in the `server`, `client`, and `protocol` packages.

    ## Implementing an MCP Server (using Stdio)

    This example shows the basic structure for creating an MCP server, registering a simple tool, and running the server using the stdio transport.

    ```go
    package main

    import (
    	"context"
    	"encoding/json" // Needed for runServerLoop
    	"errors"        // Needed for runServerLoop
    	"fmt"           // Needed for runServerLoop
    	"io"            // Needed for runServerLoop
    	"log"
    	"os"
    	"os/signal" // For graceful shutdown
    	"strings"   // Needed for runServerLoop

    	"github.com/localrivet/gomcp/protocol"
    	"github.com/localrivet/gomcp/server"
    	"github.com/localrivet/gomcp/transport/stdio"
    	"github.com/localrivet/gomcp/types"
    )

    // Example tool handler
    func myToolHandler(ctx context.Context, progressToken *protocol.ProgressToken, arguments map[string]interface{}) (content []protocol.Content, isError bool) {
    	log.Printf("Executing myTool with args: %v", arguments)
    	// ... tool logic ...
    	return []protocol.Content{protocol.TextContent{Type: "text", Text: "Tool executed!"}}, false
    }

    // Simple server loop for stdio (replace with actual implementation or copy from examples)
    func runServerLoop(ctx context.Context, srv *server.Server, transport types.Transport) error {
        session := server.NewStdioSession("stdio-session") // Use helper if available, or mock
        if err := srv.RegisterSession(session); err != nil { return fmt.Errorf("failed to register session: %w", err) }
        defer srv.UnregisterSession(session.SessionID())

        log.Println("Server listening on stdio...")
        for {
            select {
            case <-ctx.Done(): return ctx.Err()
            default:
                rawMsg, err := transport.ReceiveWithContext(ctx)
                if err != nil {
                    if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "pipe closed") || errors.Is(err, context.Canceled) { return nil }
                    return fmt.Errorf("transport receive error: %w", err)
                }
                response := srv.HandleMessage(ctx, session.SessionID(), rawMsg)
                if response != nil {
                    respBytes, err := json.Marshal(response)
                    if err != nil { log.Printf("ERROR: server failed to marshal response: %v", err); continue }
                    if err := transport.Send(respBytes); err != nil {
                        if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "pipe closed") { return nil }
                        return fmt.Errorf("transport send error: %w", err)
                    }
                }
            }
        }
    }

    // Note: Need a simple StdioSession implementation for runServerLoop
    type stdioSession struct { id string }
    func NewStdioSession(id string) *stdioSession { return &stdioSession{id: id} }
    func (s *stdioSession) SessionID() string { return s.id }
    func (s *stdioSession) SendNotification(notification protocol.JSONRPCNotification) error { return fmt.Errorf("stdio transport does not support server-to-client notifications") }
    func (s *stdioSession) SendResponse(response protocol.JSONRPCResponse) error { return fmt.Errorf("stdio transport does not support async server-to-client responses via session") }
    func (s *stdioSession) Close() error { return nil }
    func (s *stdioSession) Initialize() {}
    func (s *stdioSession) Initialized() bool { return true } // Assume initialized for stdio simplicity
    var _ server.ClientSession = (*stdioSession)(nil)


    func main() {
    	log.SetOutput(os.Stderr)
    	log.SetFlags(log.Ltime | log.Lshortfile)
    	log.Println("Starting My MCP Server...")

    	// Create server core
    	srv := server.NewServer("MyGoMCPServer", server.ServerOptions{
    		// Logger: provide custom logger if needed
    	})

    	// Register tools
    	myTool := protocol.Tool{
    		Name:        "my_tool",
    		Description: "A simple example tool",
    		InputSchema: protocol.ToolInputSchema{Type: "object"}, // Define schema as needed
    	}
    	err := srv.RegisterTool(myTool, myToolHandler)
    	if err != nil {
    		log.Fatalf("Failed to register tool: %v", err)
    	}

    	// Create stdio transport
    	transport := stdio.NewStdioTransport()

    	// Run the server's message handling loop
    	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    	defer stop()
    	err = runServerLoop(ctx, srv, transport) // Pass context, server, transport
    	if err != nil && !errors.Is(err, context.Canceled) {
    		log.Fatalf("Server loop exited with error: %v", err)
    	}

    	log.Println("Server finished.")
    }
    ```
    *(See the `examples/` directory for more complete server implementations.)*

    ## Implementing an MCP Client (using SSE)

    This example shows the basic structure for creating an MCP client using the SSE transport, connecting to a server, and making basic requests.

    ```go
    package main

    import (
    	"context"
    	"log"
    	"os"
    	"time"

    	"github.com/localrivet/gomcp/client"   // Use client package
    	"github.com/localrivet/gomcp/protocol" // Use protocol package
    )

    func main() {
    	log.SetOutput(os.Stderr)
    	log.SetFlags(log.Ltime | log.Lshortfile)
    	log.Println("Starting My MCP Client...")

    	// Create client instance, providing server URL
    	clt, err := client.NewClient("MyGoMCPClient", client.ClientOptions{
    		ServerBaseURL: "http://127.0.0.1:8080", // Adjust if server runs elsewhere
    		// Logger: provide custom logger if needed
    	})
    	if err != nil {
    		log.Fatalf("Failed to create client: %v", err)
    	}

    	// Connect and perform initialization (use context for timeout/cancellation)
    	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    	defer cancel()
    	err = clt.Connect(ctx)
    	if err != nil {
    		log.Fatalf("Client failed to connect: %v", err)
    	}
    	defer clt.Close() // Ensure connection is closed eventually

    	serverInfo := clt.ServerInfo() // Get server info after connection
    	log.Printf("Client connected successfully to server: %s (Version: %s)", serverInfo.Name, serverInfo.Version)

    	// Example: List tools
    	listParams := protocol.ListToolsRequestParams{}
    	toolsResult, err := clt.ListTools(ctx, listParams) // Pass context
    	if err != nil {
    		log.Printf("Error listing tools: %v", err)
    	} else {
    		log.Printf("Available tools: %d", len(toolsResult.Tools))
    		for _, tool := range toolsResult.Tools {
    			log.Printf("  - %s: %s", tool.Name, tool.Description)
    		}
    	}

    	// Example: Call a tool (assuming 'my_tool' exists)
    	callParams := protocol.CallToolParams{
    		Name:      "my_tool",
    		Arguments: map[string]interface{}{"input": "hello"},
    	}
    	callResult, err := clt.CallTool(ctx, callParams, nil) // Pass context, nil progress token
    	if err != nil {
    		log.Printf("Error calling tool 'my_tool': %v", err)
    	} else {
    		log.Printf("Tool 'my_tool' result: %+v", callResult)
    	}

    	// Ping is handled via standard request/response, no special client method needed

    	log.Println("Client finished.")
    }
    ```
    *(See the `examples/` directory for more detailed client implementations.)*
