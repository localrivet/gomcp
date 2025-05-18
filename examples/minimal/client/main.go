package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/localrivet/gomcp/client"
)

func main() {
	// Create a simple logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	fmt.Println("Connecting to the server...")

	// Create a new client using the client package directly
	// The client will connect immediately during initialization
	c, err := client.NewClient("minimal-client",
		client.WithLogger(logger),
		client.WithProtocolVersion("draft"),
	)
	if err != nil {
		log.Fatalf("Failed to create and connect client: %v", err)
	}

	fmt.Println("Connected successfully!")

	// Call the say_hello tool
	fmt.Println("\nCalling the say_hello tool...")
	response, err := c.CallTool("say_hello", map[string]interface{}{
		"name": "MCP User",
	})
	if err != nil {
		log.Fatalf("Tool call failed: %v", err)
	}

	fmt.Printf("Response: %v\n", response)

	// Call the calculator tool
	fmt.Println("\nCalling the calculator tool...")
	result, err := c.CallTool("calculator", map[string]interface{}{
		"operation": "add",
		"x":         42,
		"y":         58,
	})
	if err != nil {
		log.Fatalf("Calculator tool call failed: %v", err)
	}

	fmt.Printf("42 + 58 = %v\n", result)

	// Get a resource
	fmt.Println("\nGetting a resource...")
	resource, err := c.GetResource("/users/123")
	if err != nil {
		log.Fatalf("Resource get failed: %v", err)
	}

	prettyJSON, _ := json.MarshalIndent(resource, "", "  ")
	fmt.Printf("User resource: %s\n", prettyJSON)

	fmt.Println("\nClient example completed successfully!")

	// Close the client connection
	if err := c.Close(); err != nil {
		log.Fatalf("Failed to close client: %v", err)
	}
}
