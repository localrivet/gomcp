package main

import (
	"log"
	"os"

	mcp "github.com/localrivert/gomcp" // Import the root package
)

func main() {
	// Configure logging for the main application as well
	// Server/Client will also configure their logging, but this catches startup issues.
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	log.Println("Starting MCP Server...")

	// Create and run the server
	// The server name can be configured (e.g., via flags or env vars)
	server := mcp.NewServer("GoExampleServer")
	err := server.Run()
	if err != nil {
		// Log fatal will print message and exit(1)
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server finished.") // Should ideally not be reached if Run loops forever
}
