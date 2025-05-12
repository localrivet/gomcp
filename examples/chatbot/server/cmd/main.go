package main

import (
	"flag"
	"log"

	"github.com/localrivet/gomcp/examples/chatbot/server/internal/db"
	"github.com/localrivet/gomcp/examples/chatbot/server/internal/services"
	"github.com/localrivet/gomcp/examples/chatbot/server/internal/tools"
	"github.com/localrivet/gomcp/server"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 4477, "Port to run the MCP server on")
	flag.Parse()

	// Create MCP server
	s := server.NewServer("Chatbot-Helper-Server")

	// Initialize database
	weatherDB := db.NewMockWeatherDB()

	// Initialize services
	weatherService := services.NewWeatherService(weatherDB)
	factsService := services.NewFactsService()

	// Register all tools and resources
	if err := tools.RegisterAllTools(s, weatherService, factsService); err != nil {
		log.Fatalf("Error registering tools: %v", err)
	}

	// Configure SSE transport instead of WebSocket
	s.AsSSE("0.0.0.0:4477", "/mcp")

	// Start the server
	log.Printf("Starting Chatbot Helper MCP server on port %d", *port)
	if err := s.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
