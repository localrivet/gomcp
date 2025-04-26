package main

import (
	"context"
	"log"
	"os"
	"time" // For sleep/timeout

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol" // Added for CallTool test
	// "github.com/localrivet/gomcp/transport/tcp" // No longer needed
	// "github.com/localrivet/gomcp/types"         // No longer needed directly
)

const defaultServerBaseURL = "http://127.0.0.1:8081" // Use explicit IP and correct port

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	serverBaseURL := defaultServerBaseURL
	// TODO: Allow configuring address via flag/env var
	log.Printf("Starting MCP Client, connecting to %s...", serverBaseURL)

	// Create client options for SSE+HTTP
	clientOpts := client.ClientOptions{
		ServerBaseURL: serverBaseURL,
		// Use default "/sse" and "/message" endpoints
	}

	// Create a client
	clt, err := client.NewClient("GoExampleSSEClient", clientOpts) // Use new constructor
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Connect (establishes SSE, performs handshake via POST/SSE)
	// Use a context with timeout for connection attempt
	connectCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = clt.Connect(connectCtx)
	if err != nil {
		log.Fatalf("Client failed to connect: %v", err)
	}

	log.Println("Client connected successfully!")

	// --- Example Interaction (Optional) ---
	log.Println("Attempting to call 'echo' tool...")
	callCtx, callCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer callCancel()
	echoParams := protocol.CallToolParams{
		Name:      "echo", // Assuming server registers an 'echo' tool
		Arguments: map[string]interface{}{"message": "Hello via SSE+HTTP!"},
	}
	result, err := clt.CallTool(callCtx, echoParams, nil)
	if err != nil {
		log.Printf("Error calling tool 'echo': %v", err)
	} else {
		log.Printf("'echo' tool result: %+v", result)
	}
	// --- End Example Interaction ---

	// Give time for potential async operations or just exit
	log.Println("Waiting briefly before closing...")
	time.Sleep(2 * time.Second)

	// Close the client connection when done
	err = clt.Close()
	if err != nil {
		log.Printf("Error closing client: %v", err)
	}

	log.Println("Client finished.")
}
