package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/localrivet/gomcp/examples/chatbot/client/internal/config"
	"github.com/localrivet/gomcp/examples/chatbot/client/internal/handlers"
	"github.com/localrivet/gomcp/examples/chatbot/client/internal/mcp"
	"github.com/sashabaranov/go-openai"
)

func main() {
	// Load configuration
	cfg := config.New()
	cfg.LoadFromEnv()

	// Validate OpenAI key
	if cfg.OpenAIKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// Initialize OpenAI client
	openaiClient := openai.NewClient(cfg.OpenAIKey)

	// Load MCP configuration
	mcpClient, err := cfg.LoadMCPClient()
	if err != nil {
		log.Fatalf("Failed to load MCP config: %v", err)
	}

	// Create MCP client wrapper
	mcpWrapper := mcp.New(mcpClient)

	// Connect to MCP servers in a background goroutine
	go func() {
		if err := mcpWrapper.Connect(); err != nil {
			log.Printf("Error connecting to MCP servers: %v", err)
		}
	}()

	// Create chat handler
	chatHandler := handlers.NewChatHandler(
		mcpWrapper,
		openaiClient,
		filepath.Join("templates", "index.html"),
	)

	// Set up HTTP routes
	http.HandleFunc("/", chatHandler.IndexHandler)
	http.HandleFunc("/chat", chatHandler.ChatHandler)

	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Start the HTTP server
	port := cfg.Port
	log.Printf("Starting web server on http://localhost:%d", port)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
