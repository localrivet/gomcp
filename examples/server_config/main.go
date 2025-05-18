// Package main demonstrates how to use the server configuration functionality
// to manage multiple MCP servers from a configuration file.
package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/localrivet/gomcp/client"
)

func main() {
	// Create a logger for use with clients
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Part 1: Loading servers from configuration file
	fmt.Println("=== Example 1: Loading servers from configuration file ===")

	// Path to the configuration file - adjust for when running from within the server_config directory
	configFile := "mcpservers.json"
	fmt.Printf("Loading server configuration from %s\n", configFile)

	// Load the MCP servers from the config file using the client package directly
	registry := client.NewServerRegistry()
	if err := registry.LoadConfig(configFile); err != nil {
		log.Fatalf("Failed to load server configuration: %v", err)
	}

	// Get all server names
	serverNames, err := registry.GetServerNames()
	if err != nil {
		log.Fatalf("Failed to get server names: %v", err)
	}

	if len(serverNames) == 0 {
		log.Fatalf("No servers were loaded from the configuration")
	}

	fmt.Printf("Successfully started %d MCP servers: %v\n", len(serverNames), serverNames)

	// Create a map to store clients
	clients := make(map[string]client.Client)

	// Get clients for all servers - clients are already connected
	for _, name := range serverNames {
		c, err := registry.GetClient(name)
		if err != nil {
			log.Printf("Warning: Failed to get client for server %s: %v", name, err)
			continue
		}
		clients[name] = c
		fmt.Printf("Successfully connected to server: %s\n", name)
	}

	// Check if we have any clients
	if len(clients) == 0 {
		log.Fatalf("Failed to get any clients for the configured servers")
	}

	// Get the client for the memory server from our configuration
	memoryClient, ok := clients["memory"]
	if !ok {
		log.Fatalf("Memory server not found in configuration")
	}

	fmt.Println("\nUsing connected memory server client")

	// Example: Get the root resource
	fmt.Println("Fetching root resource...")
	root, err := memoryClient.GetRoot()
	if err != nil {
		log.Printf("Warning: Failed to get root resource: %v", err)
	} else {
		fmt.Printf("Root resource: %+v\n", root)
	}

	// Example: Call an echo tool if available
	fmt.Println("\nCalling 'echo' tool...")
	result, err := memoryClient.CallTool("echo", map[string]interface{}{
		"message": "Hello from GOMCP!",
	})
	if err != nil {
		log.Printf("Warning: Failed to call echo tool: %v", err)
	} else {
		fmt.Printf("Tool result: %+v\n", result)
	}

	// Try another tool available in the memory server
	fmt.Println("\nCalling 'get' tool to retrieve a value...")
	_, err = memoryClient.CallTool("set", map[string]interface{}{
		"key":   "greeting",
		"value": "Hello, World!",
	})
	if err != nil {
		log.Printf("Warning: Failed to call set tool: %v", err)
	}

	result, err = memoryClient.CallTool("get", map[string]interface{}{
		"key": "greeting",
	})
	if err != nil {
		log.Printf("Warning: Failed to call get tool: %v", err)
	} else {
		fmt.Printf("Retrieved value: %+v\n", result)
	}

	// Check if we have a filesystem client
	if filesystemClient, ok := clients["filesystem"]; ok {
		fmt.Println("\nUsing connected filesystem server client")

		// Example: Get the root resource
		fmt.Println("Fetching root resource from filesystem server...")
		fsRoot, err := filesystemClient.GetRoot()
		if err != nil {
			log.Printf("Warning: Failed to get filesystem root resource: %v", err)
		} else {
			fmt.Printf("Filesystem root resource: %+v\n", fsRoot)
		}
	}

	// Part 2: Demonstrating direct client creation with WithServers option
	fmt.Println("\n=== Example 2: Direct client creation with server config ===")

	// Create a configuration directly in code
	serverConfig := client.ServerConfig{
		MCPServers: map[string]client.ServerDefinition{
			"direct-memory": {
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-memory"},
				Env:     map[string]string{"NODE_ENV": "development"},
			},
		},
	}

	// Create a client with the WithServers option
	directClient, err := client.NewClient("direct-client",
		client.WithLogger(logger),
		client.WithServers(serverConfig, "direct-memory"),
	)
	if err != nil {
		log.Printf("Warning: Failed to create direct client: %v", err)
	} else {
		fmt.Println("Successfully created direct client with integrated server management")

		// Call a tool on the direct client
		fmt.Println("Calling 'echo' tool on directly created client...")
		result, err := directClient.CallTool("echo", map[string]interface{}{
			"message": "Hello from direct client!",
		})
		if err != nil {
			log.Printf("Warning: Failed to call echo tool on direct client: %v", err)
		} else {
			fmt.Printf("Tool result from direct client: %+v\n", result)
		}

		// Don't forget to close the client when done
		if err := directClient.Close(); err != nil {
			log.Printf("Warning: Error closing direct client: %v", err)
		} else {
			fmt.Println("Successfully closed direct client (server process also terminated)")
		}
	}

	// Setup signal handling for graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\nPress Ctrl+C to stop all servers and exit")

	// Wait for termination signal
	<-signalCh

	fmt.Println("\nShutting down server connections...")

	// Close all client connections
	for name, client := range clients {
		if err := client.Close(); err != nil {
			log.Printf("Warning: Error closing client for %s: %v", name, err)
		} else {
			fmt.Printf("Successfully closed client for %s\n", name)
		}
	}

	fmt.Println("All server connections closed successfully")
}
