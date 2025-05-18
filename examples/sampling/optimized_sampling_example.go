package server

import (
	"fmt"
	"log"
	"time"

	"github.com/localrivet/gomcp/client"
)

// OptimizedSamplingExample demonstrates the MCP sampling flow with optimizations
func OptimizedSamplingExample() {
	// Example demonstrates the MCP sampling flow with optimizations
	fmt.Println("MCP Optimized Sampling Flow Example")
	fmt.Println("===================================")

	// Create client with all the proper options including optimizations
	opts := client.DefaultSamplingOptimizationOptions()
	opts.CacheCapacity = 50          // Store up to 50 responses
	opts.CacheTTL = 15 * time.Minute // Cache for 15 minutes

	c, err := client.NewClient("my-optimized-client",
		client.WithProtocolVersion("2025-03-26"),
		client.WithProtocolNegotiation(true),
		client.WithSamplingOptimizations(opts),
	)
	if err != nil {
		log.Fatalf("Failed to create and connect client: %v", err)
	}

	// Register base sampling handler (LLM integration)
	c = c.WithSamplingHandler(sampleSamplingHandler)

	// Example of a client app calling the MCP server
	fmt.Println("\n[1] Client App → MCP Server")
	fmt.Println("Client app calls a server method (e.g., summarize_conversation)")

	// Example of server processing and identifying it needs more context
	fmt.Println("\n[2] MCP Server identifies need for sampling")
	fmt.Println("Server identifies it needs an LLM to generate a summary")

	// Construct a sampling request (would be done by the server in a real scenario)
	fmt.Println("\n[3] MCP Server → MCP Client Sampling Request")
	fmt.Println("Server constructs a sampling request with conversation history")

	messages := []client.SamplingMessage{
		client.CreateTextSamplingMessage("user", "How do I implement sampling in MCP?"),
		client.CreateTextSamplingMessage("assistant", "MCP sampling allows dynamic interaction with LLMs during execution. Would you like to see an example?"),
		client.CreateTextSamplingMessage("user", "Yes, please show me!"),
	}

	// Sample preferences include model hints and priorities
	prefs := client.SamplingModelPreferences{
		Hints: []client.SamplingModelHint{
			{Name: "gpt-4-turbo"},
		},
		SpeedPriority: floatPtr(0.5), // Balance of speed and quality
	}

	// Create sampling parameters (this would typically be done inside the server)
	params := client.SamplingCreateMessageParams{
		Messages:         messages,
		ModelPreferences: prefs,
		SystemPrompt:     "You are a helpful assistant explaining MCP sampling.",
		MaxTokens:        1000,
	}

	// In a real implementation, this sampling would happen inside server handlers
	// The server would make this request to the client when needed
	response, err := c.GetSamplingHandler()(params)
	if err != nil {
		log.Fatalf("Sampling error: %v", err)
	}

	// Got response from LLM
	fmt.Println("\n[4] MCP Client → LLM → MCP Client")
	fmt.Printf("LLM processed the request using model: %s\n", response.Model)

	// Server using the LLM response
	fmt.Println("\n[5] MCP Client → MCP Server (Response)")
	fmt.Println("Response is returned to the server for further processing")

	// Server completes its task with the LLM's help
	fmt.Println("\n[6] MCP Server → Client App (Final Result)")
	fmt.Println("Server completes its task and returns the result to the client app")

	// Show sample of response text
	fmt.Println("\nSample Response Text:")
	fmt.Println("---------------------")
	previewText := response.Content.Text
	if len(previewText) > 150 {
		previewText = previewText[:150] + "..."
	}
	fmt.Println(previewText)

	// Show optimizations
	fmt.Println("\nOptimization Benefits:")
	fmt.Println("---------------------")
	fmt.Println("✓ Caching: Repeated similar requests return cached responses")
	fmt.Println("✓ Content Validation: Messages validated before sending to LLM")
	fmt.Println("✓ Performance Metrics: Response times and throughput tracked")
	fmt.Println("✓ Graceful Degradation: Fallback options for errors")

	// Done
	fmt.Println("\nExample completed successfully!")
}

// Sample implementation of a sampling handler that processes LLM requests
func sampleSamplingHandler(params client.SamplingCreateMessageParams) (client.SamplingResponse, error) {
	// In a real implementation, this would call an actual LLM API
	// For this example, we'll simulate a response

	// Simulate processing time
	time.Sleep(200 * time.Millisecond)

	return client.SamplingResponse{
		Role: "assistant",
		Content: client.SamplingMessageContent{
			Type: "text",
			Text: "Here's how to implement sampling in MCP:\n\n1. **Server Side**: When your MCP server handler needs information from an LLM, it uses the client's sampling functionality.\n\n2. **Request Construction**: Create a sampling request with relevant context and a specific question.\n\n3. **LLM Processing**: The request is sent to the LLM which generates the appropriate response.\n\n4. **Response Integration**: The server receives the LLM's response and incorporates it into the ongoing task.\n\nThis allows for dynamic, context-aware LLM assistance at precisely the points in your workflow where it's needed, without requiring all possible context upfront.",
		},
		Model:      "gpt-4-turbo",
		StopReason: "complete",
	}, nil
}

// Helper to create float pointers
func floatPtr(v float64) *float64 {
	return &v
}
