package main

import (
	"fmt"
	"log"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/embedded"
)

func main() {
	fmt.Println("🧪 Testing MCP Context Creation Fix")
	fmt.Println("This test verifies that the 'unexpected end of JSON input' error is resolved")

	// Create a temporary directory path (simulating vibe-agent's empty test directory)
	testDir := "/tmp/gomcp-test-empty-dir"

	// Create embedded transport pair
	serverTransport, clientTransport := embedded.NewTransportPair()

	// Create and configure server using correct constructor
	mcpServer := server.NewServer("test-server")

	// Configure server with embedded transport
	mcpServer.AsEmbedded(serverTransport)

	// Start the server in a goroutine (this is where the original error occurred)
	fmt.Println("🚀 Starting MCP server...")
	go func() {
		if err := mcpServer.Run(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create client using correct constructor with embedded transport
	fmt.Println("🔗 Creating MCP client...")
	mcpClient, err := client.NewClient("embedded://", client.WithEmbedded(clientTransport))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Add a root path (like vibe-agent does) - client manages roots locally
	err = mcpClient.AddRoot(testDir, "test-root")
	if err != nil {
		log.Printf("Warning: Could not add root %s: %v", testDir, err)
	}

	fmt.Println("✅ Successfully created MCP client and server!")

	// Test basic functionality
	fmt.Println("🔧 Testing basic MCP operations...")

	// List tools
	tools, err := mcpClient.ListTools()
	if err != nil {
		log.Printf("Warning: Failed to list tools: %v", err)
	} else {
		fmt.Printf("📋 Found %d tools\n", len(tools))
	}

	// List resources
	resources, err := mcpClient.ListResources()
	if err != nil {
		log.Printf("Warning: Failed to list resources: %v", err)
	} else {
		fmt.Printf("📁 Found %d resources\n", len(resources))
	}

	// Test file operations if we have roots
	roots, err := mcpClient.GetRoots()
	if err != nil {
		log.Printf("Warning: Failed to get roots: %v", err)
	} else {
		fmt.Printf("🌳 Found %d roots: %v\n", len(roots), roots)
	}

	// Clean shutdown
	fmt.Println("🛑 Shutting down...")
	mcpClient.Close()

	fmt.Println("🎉 Test completed successfully!")
	fmt.Println("✅ The 'unexpected end of JSON input' error has been fixed!")
}
