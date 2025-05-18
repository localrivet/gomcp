package main

import (
	"fmt"
	"os"

	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create server with minimal setup
	srv := server.NewServer("mcp-server").
		AsStdio("logs/mcp.log").
		Tool("say_hello", "Say hello", func(ctx *server.Context, args struct {
			Name string `json:"name"`
		}) (string, error) {
			// Extract message from arguments
			return fmt.Sprintf("Hello, %s!", args.Name), nil
		}).
		Tool("calculator", "Calculator", func(ctx *server.Context, args struct {
			Operation string `json:"operation"`
			X         int    `json:"x"`
			Y         int    `json:"y"`
		}) (int, error) {
			switch args.Operation {
			case "add":
				return args.X + args.Y, nil
			case "subtract":
				return args.X - args.Y, nil
			case "multiply":
				return args.X * args.Y, nil
			case "divide":
				if args.Y == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				return args.X / args.Y, nil
			default:
				return 0, fmt.Errorf("invalid operation: %s", args.Operation)
			}
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

	// Run server with logging to file
	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run server: %v\n", err)
	}
}
