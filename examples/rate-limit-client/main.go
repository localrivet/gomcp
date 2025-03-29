package main

import (
	// Needed for context.Background() potentially, though not used directly here yet
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	mcp "github.com/localrivet/gomcp" // Import root package
)

// useTool sends a CallToolRequest using the client and processes the response.
// It returns the result Content slice or an error.
func useTool(client *mcp.Client, toolName string, args map[string]interface{}) ([]mcp.Content, error) {
	// log.Printf("Sending CallToolRequest for tool '%s'...", toolName) // Reduce log noise for rapid requests
	reqParams := mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	}

	// Call the tool using the client method
	// The client's internal sendRequestAndWait handles timeouts.
	// Pass nil for progressToken as we are not using progress reporting here.
	result, err := client.CallTool(reqParams, nil)
	if err != nil {
		// This error could be a transport error, timeout, or an MCP error response
		// Check for specific rate limit error text (adjust if server uses a specific code)
		if strings.Contains(err.Error(), "Too many requests") || strings.Contains(err.Error(), "RateLimitExceeded") {
			return nil, fmt.Errorf("RateLimitExceeded") // Return a simplified error for counting
		}
		return nil, fmt.Errorf("CallTool '%s' failed: %w", toolName, err)
	}

	// Check if the tool execution itself resulted in an error (IsError flag)
	if result.IsError != nil && *result.IsError {
		errMsg := fmt.Sprintf("Tool '%s' execution reported an error", toolName)
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(mcp.TextContent); ok {
				errMsg = fmt.Sprintf("Tool '%s' failed: %s", toolName, textContent.Text)
			} else {
				errMsg = fmt.Sprintf("Tool '%s' failed with non-text error content: %T", toolName, result.Content[0])
			}
		}
		// Return the content along with the error, as the content might contain error details
		return result.Content, fmt.Errorf(errMsg)
	}

	// log.Printf("Tool '%s' executed successfully.", toolName) // Reduce log noise
	return result.Content, nil
}

// runClientLogic creates a client, connects, and tests the rate limited tool.
func runClientLogic(clientName string) error {
	// Create a new client instance
	client := mcp.NewClient(clientName)

	// Connect and perform initialization
	log.Println("Connecting to server...")
	err := client.Connect()
	if err != nil {
		return fmt.Errorf("client failed to connect: %w", err)
	}
	defer client.Close() // Ensure connection is closed eventually
	log.Printf("Client connected successfully to server: %s (Version: %s)", client.ServerInfo().Name, client.ServerInfo().Version)
	// log.Printf("Server Capabilities: %+v", client.ServerCapabilities()) // Optional: Log capabilities

	// --- Test Rate Limiting ---
	log.Println("\n--- Testing Rate Limited Echo Tool ---")
	toolName := "limited-echo"
	successCount := 0
	rateLimitErrors := 0
	otherErrors := 0
	totalRequests := 10 // Number of requests to send rapidly

	log.Printf("Sending %d requests rapidly to test rate limit...", totalRequests)
	startTime := time.Now()

	var wg sync.WaitGroup
	var mu sync.Mutex // To protect counters

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(reqNum int) {
			defer wg.Done()
			message := fmt.Sprintf("Request %d", reqNum)
			args := map[string]interface{}{"message": message}
			_, err := useTool(client, toolName, args) // Pass client instance

			mu.Lock()
			if err != nil {
				if err.Error() == "RateLimitExceeded" { // Check for our simplified error
					rateLimitErrors++
				} else {
					log.Printf("Attempt %d: UNEXPECTED ERROR - %v", reqNum, err) // Log unexpected errors
					otherErrors++
				}
			} else {
				successCount++
			}
			mu.Unlock()
			// No artificial delay - attempt to hit the burst/rate limit
		}(i + 1)
	}

	wg.Wait() // Wait for all requests to complete
	duration := time.Since(startTime)

	log.Printf("\nRate Limit Test Summary (%v elapsed):", duration)
	log.Printf("  Total Requests: %d", totalRequests)
	log.Printf("  Successful:     %d", successCount)
	log.Printf("  Rate Limited:   %d", rateLimitErrors)
	log.Printf("  Other Errors:   %d", otherErrors)

	// Basic validation: Expect some successes and some rate limit errors
	// The exact numbers depend on timing, but both should ideally be non-zero for a good test
	if successCount == 0 && totalRequests > 0 {
		log.Println("WARNING: Expected some successful requests, but got none.")
	}
	if rateLimitErrors == 0 && totalRequests > 5 { // Expect errors if sending more than burst+rate
		log.Println("WARNING: Expected some rate limit errors, but got none (increase totalRequests or check server limits?).")
	}
	if otherErrors > 0 {
		log.Println("ERROR: Encountered unexpected errors during rate limit test.")
		// Return an error if unexpected errors occurred
		return fmt.Errorf("encountered %d unexpected errors during rate limit test", otherErrors)
	}

	log.Println("Client operations finished.")
	return nil // Indicate overall success if no unexpected errors
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Rate Limit Example MCP Client...")

	clientName := "GoRateLimitClient-Refactored"

	// Run the core client logic
	err := runClientLogic(clientName)
	if err != nil {
		log.Fatalf("Client exited with error: %v", err)
	}

	log.Println("Client finished successfully.")
}

// Helper function to get a pointer to a time.Duration value.
func Ptr[T any](v T) *T {
	return &v
}
