package main

import (
	"log"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// Example handler for a resource template
func handleExampleResource(ctx *server.Context, id string) (string, error) {
	ctx.Info("Handling example resource with ID: " + id)
	return "Content for resource with ID: " + id, nil
}

func main() {
	// Create and configure a new server instance using chaining
	svr := server.NewServer("Demo Server 🚀").
		// Configure transports (choose one or more)
		AsStdio(). // Configure for standard I/O transport

		// Example of configuring WebSocket transport (uncomment to use)
		// AsWebsocket(":8080", "/mcp"). // Configure for WebSocket transport on address :8080 and path /mcp

		// Example of configuring SSE transport (uncomment to use)
		// AsSSE(":8081", "/mcp"). // Configure for SSE transport on address :8081 and base path /mcp

		// Register a prompt
		Prompt("Add two numbers", "Add two numbers using a tool",
			server.System("You are a helpful assistant that adds two numbers."),
			server.User("What is 2 + 2?"),
		).

		// Register tools
		Tool("add", "Add two numbers", func(ctx *server.Context, args struct{ A, B int }) (int, error) {
			ctx.Info("Adding numbers")
			return args.A + args.B, nil
		}).
		Tool("subtract", "Subtract two numbers", func(ctx *server.Context, args struct{ A, B int }) (int, error) {
			ctx.Info("Subtracting numbers")
			return args.A - args.B, nil
		}).
		Tool("multiply", "Multiply two numbers", func(ctx *server.Context, args struct{ A, B int }) (int, error) {
			ctx.Info("Multiplying numbers")
			return args.A * args.B, nil
		}).
		Tool("divide", "Divide two numbers", func(ctx *server.Context, args struct{ A, B int }) (int, error) {
			ctx.Info("Dividing numbers")
			return args.A / args.B, nil
		}).

		// Register static resources
		Resource(protocol.Resource{
			URI:         "file:///path/to/example.txt", // Replace with a valid path if needed
			Kind:        "file",
			Title:       "Example File",
			Description: "A sample text file.",
		}).
		Resource(protocol.Resource{
			URI:         "database://get/user/{id}", // Example URI, handler needed for reading
			Kind:        "database",
			Title:       "Example Database",
			Description: "A sample database.",
		}).

		// Register a resource template
		ResourceTemplate("example://resource/{id}", handleExampleResource). // Example resource template

		// Register roots
		Root(protocol.Root{
			URI:         "file:///path/to/workspace", // Replace with a valid path if needed
			Kind:        "workspace",
			Title:       "Example Workspace",
			Description: "The root of the example project.",
		})

	// Defer server shutdown
	defer svr.Close()

	// Run the server
	log.Println("Starting server...")
	// If using HTTP transports (WebSocket or SSE), you would typically run an HTTP server here
	// For example:
	/*
		go func() {
			log.Printf("HTTP server listening on :8080 or :8081...")
			// You would need to set up your HTTP router (e.g., using net/http, gorilla/mux, etc.)
			// and register the transport handlers with the router.
			// Example using net/http default mux:
			// http.HandleFunc("/mcp", svr.TransportManager.GetWebsocketHandler()) // For WebSocket
			// http.HandleFunc("/mcp", svr.TransportManager.GetSSEHandler()) // For SSE
			// log.Fatal(http.ListenAndServe(":8080", nil)) // Or :8081
		}()
	*/

	if err := svr.Run(); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
