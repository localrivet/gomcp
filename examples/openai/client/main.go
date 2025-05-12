package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/sashabaranov/go-openai"
)

func main() {
	// Define command-line flags
	webMode := flag.Bool("web", false, "Run in web server mode")
	flag.Parse()

	// Check which mode to run in
	if *webMode {
		fmt.Println("Starting in web server mode...")
		RunWebServer()
		return
	}

	// Otherwise, run in command-line mode
	fmt.Println("Starting in command-line mode...")
	RunCommandLine()
}

// RunCommandLine runs the command-line version of the OpenAI MCP client
func RunCommandLine() {
	// Load the MCP server configuration
	configPath := "examples/openai/openai-config.json"

	// Set up the MCP client
	mcpConfig, err := client.LoadFromFile(configPath, nil)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create the client with custom options
	mcp, err := client.New(mcpConfig,
		client.WithTimeout(60*time.Second), // Longer timeout for API calls
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Set up context and connect
	ctx := context.Background()
	mcp.Roots([]string{"./examples/openai"}).
		Logger(logx.NewDefaultLogger()).
		WithContext(ctx)

	if err := mcp.Connect(); err != nil {
		log.Fatalf("Failed to connect to MCP servers: %v", err)
	}
	defer mcp.Close()

	// Get server name
	var serverName string
	for name := range mcp.Servers {
		serverName = name
		break
	}

	if serverName == "" {
		log.Fatalf("No MCP servers available")
	}

	// Example 1: Chat completion
	fmt.Println("=== Chat Completion Example ===")
	chatResponse, err := mcp.CallTool(serverName, "chat_completion", map[string]interface{}{
		"model": openai.GPT3Dot5Turbo,
		"messages": []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Tell me a short joke about programming.",
			},
		},
		"temperature": 0.7,
		"max_tokens":  150,
	})
	if err != nil {
		log.Printf("Error calling chat_completion: %v", err)
	} else {
		printResponse(chatResponse)
	}

	// Example 2: Image generation
	fmt.Println("\n=== Image Generation Example ===")
	imageResponse, err := mcp.CallTool(serverName, "generate_image", map[string]interface{}{
		"prompt": "A cute robot programmer working on a Go project",
		"size":   openai.CreateImageSize512x512,
		"n":      1,
	})
	if err != nil {
		log.Printf("Error calling generate_image: %v", err)
	} else {
		printResponse(imageResponse)
	}

	// Example 3: Embeddings
	fmt.Println("\n=== Embeddings Example ===")
	embeddingsResponse, err := mcp.CallTool(serverName, "create_embeddings", map[string]interface{}{
		"input": []string{
			"The food was delicious and the waiter was very friendly.",
			"I enjoyed the meal.",
		},
		"model": openai.SmallEmbedding3,
	})
	if err != nil {
		log.Printf("Error calling create_embeddings: %v", err)
	} else {
		printResponse(embeddingsResponse)
	}
}

// Helper function to print tool response
func printResponse(response []protocol.Content) {
	for _, content := range response {
		if content.GetType() == "text" {
			if textContent, ok := content.(protocol.TextContent); ok {
				fmt.Println(textContent.Text)
			}
		}
	}
}
