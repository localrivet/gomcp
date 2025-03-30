---
layout: default
title: Basic Usage
nav_order: 4 # After Installation
---

# Basic Usage

The core logic resides in the root package (`github.com/localrivet/gomcp`).

    ## Implementing an MCP Server

    This example shows the basic structure for creating an MCP server, registering a simple tool, and running the server.

    ```go
    package main

    import (
    	"context" // Needed for tool handlers
    	"log"
    	"os"

    	"github.com/localrivet/gomcp"
    )

    // Example tool handler
    func myToolHandler(ctx context.Context, progressToken *gomcp.ProgressToken, arguments map[string]interface{}) (content []gomcp.Content, isError bool) {
    	log.Printf("Executing myTool with args: %v", arguments)
    	// Check for cancellation: if ctx.Err() != nil { ... }
    	// Report progress: if progressToken != nil { server.SendProgress(...) } // Need server instance or wrapper
    	return []gomcp.Content{gomcp.TextContent{Type: "text", Text: "Tool executed!"}}, false
    }

    func main() {
    	log.SetOutput(os.Stderr)
    	log.SetFlags(log.Ltime | log.Lshortfile)
    	log.Println("Starting My MCP Server...")

    	server := gomcp.NewServer("MyGoMCPServer")

    	// Register tools
    	myTool := gomcp.Tool{
    		Name:        "my_tool",
    		Description: "A simple example tool",
    		InputSchema: gomcp.ToolInputSchema{Type: "object"}, // Define schema as needed
    	}
    	err := server.RegisterTool(myTool, myToolHandler)
    	if err != nil {
    		log.Fatalf("Failed to register tool: %v", err)
    	}

    	// Run the server's main loop (handles initialization and message dispatch)
    	// It blocks until an error occurs or the connection closes (e.g., client disconnects).
    	err = server.Run()
    	if err != nil {
    		log.Fatalf("Server exited with error: %v", err)
    	}

    	log.Println("Server finished.")
    }
    ```
    *(See the `examples/server/` directory for a more complete server implementation with multiple tools.)*

    ## Implementing an MCP Client

    This example shows the basic structure for creating an MCP client, connecting to a server, and making basic requests like listing and calling tools.

    ```go
    package main

    import (
    	"log"
    	"os"
    	"time" // For Ping timeout

    	"github.com/localrivet/gomcp"
    )

    func main() {
    	log.SetOutput(os.Stderr)
    	log.SetFlags(log.Ltime | log.Lshortfile)
    	log.Println("Starting My MCP Client...")

    	client := gomcp.NewClient("MyGoMCPClient")

    	// Connect and perform initialization
    	err := client.Connect()
    	if err != nil {
    		log.Fatalf("Client failed to connect: %v", err)
    	}
    	log.Printf("Client connected successfully to server: %s", client.ServerName())

    	// Example: List tools
    	listParams := gomcp.ListToolsRequestParams{} // Add cursor if needed
    	toolsResult, err := client.ListTools(listParams)
    	if err != nil {
    		log.Printf("Error listing tools: %v", err)
    	} else {
    		log.Printf("Available tools: %d", len(toolsResult.Tools))
    		for _, tool := range toolsResult.Tools {
    			log.Printf("  - %s: %s", tool.Name, tool.Description)
    		}
    	}

    	// Example: Call a tool (assuming 'my_tool' exists and was registered by the server)
    	callParams := gomcp.CallToolParams{
    		Name:      "my_tool",
    		Arguments: map[string]interface{}{"input": "hello"},
    		// Meta: &gomcp.RequestMeta{ ProgressToken: &token }, // Optional progress
    	}
    	// Generate a progress token (optional)
    	// token := client.GenerateProgressToken()
    	// Register a handler for progress notifications for this token (if needed)
    	// client.RegisterNotificationHandler(...)
    	callResult, err := client.CallTool(callParams, nil) // Pass token instead of nil to request progress
    	if err != nil {
    		log.Printf("Error calling tool 'my_tool': %v", err)
    	} else {
    		log.Printf("Tool 'my_tool' result: %+v", callResult)
    		// Process result content (e.g., callResult.Content)
    	}

    	// Example: Ping the server
    	err = client.Ping(5 * time.Second)
    	if err != nil {
    		log.Printf("Ping failed: %v", err)
    	} else {
    		log.Println("Ping successful!")
    	}

    	// Close the client connection when done (optional for stdio)
    	err = client.Close()
    	if err != nil {
    		log.Printf("Error closing client: %v", err)
    	}

    	log.Println("Client finished.")
    }
    ```
    *(Note: These examples are simplified. See the `examples/` directory for more detailed client/server pairs demonstrating various features, although they also require updates.)*
