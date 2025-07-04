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
	fmt.Println("🚀 Starting embedded transport example...")

	// Create a pair of connected embedded transports
	serverTransport, clientTransport := embedded.NewTransportPair(
		embedded.WithBufferSize(50),
		embedded.WithTimeout(5*time.Second),
	)

	// Create MCP server and configure it with embedded transport
	mcpServer := server.NewServer("embedded-example").AsEmbedded(serverTransport)

	// Add a simple tool using the correct API
	mcpServer.Tool("greet", "Greet someone with a personalized message", func(ctx *server.Context, args *struct {
		Name string `json:"name" description:"The name of the person to greet"`
	}) (string, error) {
		if args == nil || args.Name == "" {
			return "", fmt.Errorf("name is required")
		}
		return fmt.Sprintf("Hello, %s! Welcome to the embedded MCP server.", args.Name), nil
	})

	// Add a calculation tool
	mcpServer.Tool("calculate", "Perform basic arithmetic", func(ctx *server.Context, args *struct {
		Operation string  `json:"operation" description:"The operation to perform (add, subtract, multiply, divide)"`
		A         float64 `json:"a" description:"First number"`
		B         float64 `json:"b" description:"Second number"`
	}) (map[string]interface{}, error) {
		if args == nil {
			return nil, fmt.Errorf("arguments are required")
		}

		var result float64
		switch args.Operation {
		case "add":
			result = args.A + args.B
		case "subtract":
			result = args.A - args.B
		case "multiply":
			result = args.A * args.B
		case "divide":
			if args.B == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			result = args.A / args.B
		default:
			return nil, fmt.Errorf("unsupported operation: %s", args.Operation)
		}

		return map[string]interface{}{
			"operation": args.Operation,
			"a":         args.A,
			"b":         args.B,
			"result":    result,
			"formula":   fmt.Sprintf("%.2f %s %.2f = %.2f", args.A, args.Operation, args.B, result),
		}, nil
	})

	// Add a sampling tool that requests AI processing from the client
	mcpServer.Tool("analyze_text", "Analyze text using AI sampling from client", func(ctx *server.Context, args *struct {
		Text string `json:"text" description:"Text to analyze"`
	}) (map[string]interface{}, error) {
		if args == nil || args.Text == "" {
			return nil, fmt.Errorf("text is required")
		}

		// Create sampling messages for text analysis
		messages := []server.SamplingMessage{
			server.CreateTextSamplingMessage("user", fmt.Sprintf("Analyze this text for sentiment, tone, and key themes: %s", args.Text)),
		}

		// Create model preferences
		prefs := server.SamplingModelPreferences{
			Hints: []server.SamplingModelHint{
				{Name: "claude-3-sonnet"}, // Prefer analytical models
				{Name: "gpt-4"},
			},
		}

		// Request sampling from the client with system prompt
		response, err := ctx.RequestSampling(messages, prefs, "You are an expert text analyst. Be thorough and analytical.", 300)
		if err != nil {
			return nil, fmt.Errorf("sampling request failed: %w", err)
		}

		return map[string]interface{}{
			"analysis":      response.Content.Text,
			"model":         response.Model,
			"stop_reason":   response.StopReason,
			"original_text": args.Text,
		}, nil
	})

	// Add a resource using the correct API - return simple string
	mcpServer.Resource("/user_data", "User configuration data", func(ctx *server.Context, params interface{}) (string, error) {
		return "User configuration: embedded_user with dark theme and notifications enabled", nil
	})

	// Add a parameterized resource using JSONResource
	mcpServer.Resource("/users/{id}", "User profile data", func(ctx *server.Context, params interface{}) (server.JSONResource, error) {
		// Extract user ID from path parameters
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return server.JSONResource{}, fmt.Errorf("invalid parameters")
		}

		userID, ok := paramsMap["id"].(string)
		if !ok || userID == "" {
			return server.JSONResource{}, fmt.Errorf("invalid or missing user ID")
		}

		// Return JSON data
		return server.JSONResource{
			Data: map[string]interface{}{
				"id":      userID,
				"name":    fmt.Sprintf("User %s", userID),
				"email":   fmt.Sprintf("user%s@example.com", userID),
				"created": time.Now().Format(time.RFC3339),
				"active":  true,
				"role":    "member",
			},
		}, nil
	})

	// Add another resource with TextResource
	mcpServer.Resource("/status", "Server status information", func(ctx *server.Context, params interface{}) (server.TextResource, error) {
		return server.TextResource{
			Text: "Embedded MCP Server is running successfully!",
		}, nil
	})

	// Add a prompt - fix the syntax
	mcpServer.Prompt("greeting", "Generate a personalized greeting",
		server.User("Generate a friendly greeting for {{name}} who is interested in {{topic}}."),
	)

	// Start the server
	fmt.Println("📡 Starting MCP server...")
	go func() {
		if err := mcpServer.Run(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()
	defer mcpServer.Shutdown()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create MCP client with embedded transport
	mcpClient, err := client.NewClient("embedded://local",
		client.WithEmbedded(clientTransport, client.WithEmbeddedTimeout(5*time.Second)),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Add workspace roots to the client for testing roots/list functionality
	fmt.Println("📁 Adding workspace roots to client...")
	err = mcpClient.AddRoot("/Users/user/workspace", "User Workspace")
	if err != nil {
		log.Printf("Warning: Failed to add workspace root: %v", err)
	}

	err = mcpClient.AddRoot("/Users/user/projects/embedded-demo", "Embedded Demo Project")
	if err != nil {
		log.Printf("Warning: Failed to add project root: %v", err)
	}

	err = mcpClient.AddRoot("/tmp/test-workspace", "Temporary Test Workspace")
	if err != nil {
		log.Printf("Warning: Failed to add temp root: %v", err)
	}

	// Set up a sampling handler to respond to sampling requests from the server
	mcpClient = mcpClient.WithSamplingHandler(func(params client.SamplingCreateMessageParams) (client.SamplingResponse, error) {
		fmt.Printf("\n🤖 Server requested sampling with %d messages\n", len(params.Messages))

		// Log the request details
		for i, msg := range params.Messages {
			fmt.Printf("  Message %d (%s): %s\n", i+1, msg.Role, getContentPreview(msg.Content))
		}

		if params.SystemPrompt != "" {
			fmt.Printf("  System prompt: %s\n", params.SystemPrompt)
		}

		// Simulate AI model response based on the content
		responseText := "This is a simulated AI response from the embedded client. "
		if len(params.Messages) > 0 && params.Messages[0].Content.Type == "text" {
			text := params.Messages[0].Content.Text
			if len(text) > 0 {
				responseText += fmt.Sprintf("I analyzed the text: '%s'. This appears to be a neutral statement with informational tone.", text)
			}
		}

		response := client.SamplingResponse{
			Role: "assistant",
			Content: client.SamplingMessageContent{
				Type: "text",
				Text: responseText,
			},
			Model:      "embedded-simulation-v1",
			StopReason: "endTurn",
		}

		fmt.Printf("  ✅ Responding with: %s\n", response.Content.Text)
		return response, nil
	})

	fmt.Println("✅ Server and client started successfully")
	fmt.Println()

	// Test the client connection
	fmt.Println("🔄 Testing MCP client operations...")

	// Test 1: Wait for ready (replaces Initialize)
	fmt.Println("1. Waiting for client to be ready...")
	if err := mcpClient.WaitForReady(5 * time.Second); err != nil {
		log.Fatalf("Failed to wait for client ready: %v", err)
	}
	fmt.Println("   ✅ Client is ready")
	fmt.Println()

	// Test 2: List tools
	fmt.Println("2. Listing available tools...")
	tools, err := mcpClient.ListTools()
	if err != nil {
		log.Printf("Failed to list tools: %v", err)
	} else {
		fmt.Printf("   📋 Found %d tools:\n", len(tools))
		for _, tool := range tools {
			fmt.Printf("      - %s: %s\n", tool.Name, tool.Description)
		}
	}
	fmt.Println()

	// Test 3: Call greet tool
	fmt.Println("3. Calling greet tool...")
	greetArgs := map[string]interface{}{
		"name": "Alice",
	}
	greetResult, err := mcpClient.CallTool("greet", greetArgs)
	if err != nil {
		log.Printf("Failed to call greet tool: %v", err)
	} else {
		fmt.Printf("   🎉 Greet result: %+v\n", greetResult)
	}
	fmt.Println()

	// Test 4: Call calculate tool
	fmt.Println("4. Calling calculate tool...")
	calcArgs := map[string]interface{}{
		"operation": "multiply",
		"a":         7.5,
		"b":         4.2,
	}
	calcResult, err := mcpClient.CallTool("calculate", calcArgs)
	if err != nil {
		log.Printf("Failed to call calculate tool: %v", err)
	} else {
		fmt.Printf("   🧮 Calculate result: %+v\n", calcResult)
	}
	fmt.Println()

	// Test 5: List resources
	fmt.Println("5. Listing available resources...")
	resources, err := mcpClient.ListResources()
	if err != nil {
		log.Printf("Failed to list resources: %v", err)
	} else {
		fmt.Printf("   📂 Found %d resources:\n", len(resources))
		for _, resource := range resources {
			fmt.Printf("      - %s: %s\n", resource.URI, resource.Description)
		}
	}
	fmt.Println()

	// Test 6: Read user_data resource
	fmt.Println("6. Reading user_data resource...")
	userData, err := mcpClient.GetResource("/user_data")
	if err != nil {
		log.Printf("Failed to read user_data resource: %v", err)
	} else {
		fmt.Printf("   📄 User data: %+v\n", userData)
	}
	fmt.Println()

	// Test 7: Read parameterized resource
	fmt.Println("7. Reading user profile resource...")
	userProfile, err := mcpClient.GetResource("/users/123")
	if err != nil {
		log.Printf("Failed to read user profile: %v", err)
	} else {
		fmt.Printf("   👤 User profile: %+v\n", userProfile)
	}
	fmt.Println()

	// Test 8: List prompts
	fmt.Println("8. Listing available prompts...")
	prompts, err := mcpClient.ListPrompts()
	if err != nil {
		log.Printf("Failed to list prompts: %v", err)
	} else {
		fmt.Printf("   💭 Found %d prompts:\n", len(prompts))
		for _, prompt := range prompts {
			fmt.Printf("      - %s: %s\n", prompt.Name, prompt.Description)
		}
	}
	fmt.Println()

	// Test 9: Get prompt
	fmt.Println("9. Getting greeting prompt...")
	promptArgs := map[string]interface{}{
		"name":  "Bob",
		"topic": "machine learning",
	}
	promptResult, err := mcpClient.GetPrompt("greeting", promptArgs)
	if err != nil {
		log.Printf("Failed to get prompt: %v", err)
	} else {
		fmt.Printf("   📝 Prompt result: %+v\n", promptResult)
	}
	fmt.Println()

	// Test 10: Ping
	fmt.Println("10. Testing ping...")
	if err := mcpClient.Ping(); err != nil {
		log.Printf("Ping failed: %v", err)
	} else {
		fmt.Println("   🏓 Ping successful")
	}
	fmt.Println()

	// Test 11: Call sampling tool (demonstrates server requesting AI from client)
	fmt.Println("11. Testing AI sampling tool...")
	samplingArgs := map[string]interface{}{
		"text": "The embedded transport is working perfectly and enables direct in-process communication.",
	}
	samplingResult, err := mcpClient.CallTool("analyze_text", samplingArgs)
	if err != nil {
		log.Printf("Failed to call sampling tool: %v", err)
	} else {
		fmt.Printf("   🤖 Sampling result: %+v\n", samplingResult)
	}
	fmt.Println()

	fmt.Println("✅ All tests completed successfully!")

	// Test the roots functionality by displaying the workspace roots
	fmt.Println("\n🌳 Testing workspace roots functionality...")
	roots, err := mcpClient.GetRoots()
	if err != nil {
		fmt.Printf("   ❌ Failed to get roots: %v\n", err)
	} else {
		fmt.Printf("   📁 Found %d workspace roots:\n", len(roots))
		for i, root := range roots {
			fmt.Printf("      [%d] %s (%s)\n", i+1, root.Name, root.URI)
		}
	}

	fmt.Println("🎉 Embedded transport demonstration finished")
	fmt.Println()

	// Show transport statistics
	fmt.Printf("📊 Transport Statistics: %+v\n", serverTransport.GetChannelStats())
}

// getContentPreview returns a preview of sampling message content
func getContentPreview(content client.SamplingMessageContent) string {
	switch content.Type {
	case "text":
		if len(content.Text) > 50 {
			return content.Text[:50] + "..."
		}
		return content.Text
	case "image":
		return fmt.Sprintf("Image (%s, %d bytes)", content.MimeType, len(content.Data))
	case "audio":
		return fmt.Sprintf("Audio (%s, %d bytes)", content.MimeType, len(content.Data))
	default:
		return fmt.Sprintf("Unknown content type: %s", content.Type)
	}
}
