package main

import (
	"fmt"

	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create server with minimal setup
	srv := server.NewServer("mcp-server")
	srv.AsStdio("logs/mcp.log")

	// Register a simple tool
	srv.Tool("echo", "Echo tool", func(ctx *server.Context, args struct {
		Message string `json:"message"`
	}) (string, error) {
		// Extract message from arguments
		message := args.Message
		return fmt.Sprintf("Echo: %s", message), nil
	})

	type calculatorResult struct {
		Result    int    `json:"result"`
		Operation string `json:"operation"`
	}

	// Add calculator tool with better error handling
	srv.Tool("calculator", "Calculator tool", func(ctx *server.Context, args struct {
		Operation string `json:"operation" description:"The operation to perform (add, subtract, multiply, divide)" enum:"add,subtract,multiply,divide"`
		Args      []int  `json:"args" description:"The numbers to operate on"`
	}) (interface{}, error) {
		result := calculatorResult{
			Result:    0,
			Operation: args.Operation,
		}
		// Validate input
		if len(args.Args) == 0 {
			return nil, fmt.Errorf("no arguments provided")
		}

		// Process different operations with proper error handling
		switch args.Operation {
		case "add":
			sum := 0
			for _, arg := range args.Args {
				sum += arg
			}
			result.Result = sum
			return result, nil

		case "subtract":
			diff := args.Args[0]
			for _, arg := range args.Args[1:] {
				diff -= arg
			}
			result.Result = diff
			return result, nil

		case "multiply":
			if len(args.Args) < 1 {
				return nil, fmt.Errorf("multiplication requires at least one argument")
			}
			prod := 1
			for _, arg := range args.Args {
				prod *= arg
			}
			result.Result = prod
			return result, nil

		case "divide":
			if len(args.Args) < 2 {
				return nil, fmt.Errorf("division requires at least two arguments")
			}

			quot := args.Args[0]
			for _, arg := range args.Args[1:] {
				if arg == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				quot /= arg
			}
			result.Result = quot
			return result, nil

		default:
			return nil, fmt.Errorf("invalid operation: %s (supported operations: add, subtract, multiply, divide)", args.Operation)
		}
	})

	// Simple string resource - automatically converted to text content
	srv.Resource("/hello", "A simple greeting resource", func(ctx *server.Context, params map[string]interface{}) (string, error) {
		// Default greeting
		greeting := "Hello, World!"

		// Check if name parameter is provided
		if name, ok := params["name"].(string); ok && name != "" {
			greeting = fmt.Sprintf("Hello, %s!", name)
		}

		// Simply return the string - auto-converted to ResourceResponse
		return greeting, nil
	})

	// Using TextResource explicitly
	srv.Resource("/text", "Text resource example", func(ctx *server.Context, params map[string]interface{}) (server.TextResource, error) {
		return server.TextResource{
			Text: "This is a text resource",
		}, nil
	})

	// Using ImageResource
	srv.Resource("/image", "Image resource example", func(ctx *server.Context, params map[string]interface{}) (server.ImageResource, error) {
		return server.ImageResource{
			URL:      "https://example.com/image.jpg",
			AltText:  "Example image",
			MimeType: "image/jpeg",
		}, nil
	})

	// Using LinkResource
	srv.Resource("/link", "Link resource example", func(ctx *server.Context, params map[string]interface{}) (server.LinkResource, error) {
		return server.LinkResource{
			URL:   "https://example.com",
			Title: "Example website",
		}, nil
	})

	// Using JSONResource
	srv.Resource("/users/{id}", "Get user by ID", func(ctx *server.Context, params map[string]interface{}) (server.JSONResource, error) {
		// Extract user ID from path parameters
		userID, ok := params["id"].(string)
		if !ok || userID == "" {
			return server.JSONResource{}, fmt.Errorf("invalid or missing user ID")
		}

		// Return JSON data
		return server.JSONResource{
			Data: map[string]interface{}{
				"id":   userID,
				"name": fmt.Sprintf("User %s", userID),
				"role": "member",
			},
		}, nil
	})

	// For complex cases, you can still use the explicit ResourceResponse type
	srv.Resource("/mixed-content", "Example with multiple content types", func(ctx *server.Context, params map[string]interface{}) (server.ResourceResponse, error) {
		// Create a response with multiple content items of different types
		return server.NewResourceResponse(
			server.TextContent("Here's some information with mixed content types:"),
			server.LinkContent("https://example.com", "Example Website"),
			server.ImageContent("https://example.com/image.jpg", "Example Image"),
		), nil
	})

	// Simple return with direct error
	srv.Resource("/error-example", "Example of error response", func(ctx *server.Context, params map[string]interface{}) (string, error) {
		// Simply return an error directly
		if _, ok := params["trigger"]; ok {
			return "", fmt.Errorf("example error message")
		}

		return "This is a normal response", nil
	})

	// Add a prompt
	// srv.Prompt("user", "You are a helpful assistant that can answer questions and help with tasks.")
	// srv.Prompt("user", "Use the calculator tool to perform calculations.")

	// Run server with logging to file
	if err := srv.Run(); err != nil {
		srv.Logger().Error("Failed to run server", "error", err)
	}
}
