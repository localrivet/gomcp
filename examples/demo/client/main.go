package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/logx"
)

func main() {
	// Get config path from arguments or use default
	configPath := "mcp-servers.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	mcpConfig, err := client.LoadFromFile(configPath, nil)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create the client from config file with options
	mcp, err := client.New(mcpConfig, client.WithTimeout(30*time.Second))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create a context
	ctx := context.Background()

	// Configure the client with fluent interface
	mcp.Roots([]string{"./path1", "./path2"}).
		Logger(logx.NewDefaultLogger()).
		WithContext(ctx)

	// Connect to all servers
	if err := mcp.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer mcp.Close()

	// List tools from all servers
	fmt.Println("Listing tools from all servers:")
	for name, server := range mcp.Servers {
		// We can use the context directly if needed
		tools, err := server.ListTools(ctx)
		if err != nil {
			fmt.Printf("Failed to list tools for %s: %v\n", name, err)
			continue
		}
		fmt.Printf("Server: %s, Available Tools: %d\n", name, len(tools))
		for _, tool := range tools {
			fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
		}
	}

	// Or use the convenience method - uses the context already set with WithContext
	allTools := mcp.ListTools()
	fmt.Println("\nUsing convenience method:")
	for name, tools := range allTools {
		fmt.Printf("Server: %s, Tools: %d\n", name, len(tools))
	}

	// Example of calling a tool on a specific server
	if len(mcp.Servers) > 0 {
		// Get the first server name
		var firstServer string
		for name := range mcp.Servers {
			firstServer = name
			break
		}

		// Uses the context already set with WithContext
		fmt.Printf("\nCalling 'add' tool on server '%s':\n", firstServer)
		result, err := mcp.CallTool(firstServer, "add", map[string]interface{}{
			"A": 5,
			"B": 7,
		})

		if err != nil {
			fmt.Printf("Error calling tool: %v\n", err)
		} else {
			fmt.Printf("Tool result: %+v\n", result)
		}
	}
}
