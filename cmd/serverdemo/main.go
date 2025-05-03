package main

import (
	"log"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// Define helper functions for prompt messages
func system(msg string) protocol.PromptMessage {
	// Assuming server.Text exists and works with protocol.Content
	return protocol.PromptMessage{Role: "system", Content: []protocol.Content{server.Text(msg)}}
}

func user(msg string) protocol.PromptMessage {
	// Assuming server.Text exists and works with protocol.Content
	return protocol.PromptMessage{Role: "user", Content: []protocol.Content{server.Text(msg)}}
}

func main() {
	// Create a new server instance
	svr := server.NewServer("Demo Server 🚀")

	// Configure transport (StdIO as default)
	svr.AsStdio()

	// Add a sample prompt
	svr.Prompt("Add two numbers", "Add two numbers using a tool",
		system("You are a helpful assistant that adds two numbers."),
		user("What is 2 + 2?"),
	)

	// Add a sample tool
	// Using any for the function signature due to Go method generic limitations
	svr.Tool("add", "Add two numbers", func(args struct{ A, B int }) (int, error) {
		return args.A + args.B, nil
	})

	// Defer server shutdown
	defer func() {
		if err := svr.Close(); err != nil {
			log.Printf("Error during server shutdown: %v", err)
		}
	}()

	// Run the server
	log.Println("Starting server...")
	if err := svr.Run(); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
