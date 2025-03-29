package main

import (
	"log"
	"os"

	"github.com/localrivert/gomcp/pkg/mcp" // Use the updated module path
)

func main() {
	// Configure logging for the main application
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	log.Println("Starting MCP Client...")

	// Create a client
	client := mcp.NewClient("GoExampleClient")

	// Connect (performs handshake)
	err := client.Connect()
	if err != nil {
		log.Fatalf("Client failed to connect: %v", err)
	}

	log.Println("Client connected successfully!")

	// TODO: Implement logic to send other messages (e.g., ToolDefinitionRequest, UseTool)

	// Close the client connection when done (optional for stdio)
	err = client.Close()
	if err != nil {
		log.Printf("Error closing client: %v", err)
	}

	log.Println("Client finished.")
}
