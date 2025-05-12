package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
	"github.com/sashabaranov/go-openai"
)

// Message represents a chat message
type Message struct {
	Role    string
	Content string
}

// ChatRequest represents a request from the client
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse represents a response to the client
type ChatResponse struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

// ProgressHandler handles progress updates from the server
type ProgressHandler struct {
	conn               net.Conn
	accumulatedContent string
}

// NewProgressHandler creates a new progress handler
func NewProgressHandler(conn net.Conn) *ProgressHandler {
	return &ProgressHandler{
		conn:               conn,
		accumulatedContent: "",
	}
}

// Handle processes a progress update
func (h *ProgressHandler) Handle(params *protocol.ProgressParams) error {
	// Extract content from the progress message
	if params.Value != nil {
		// Check if the value is a map with a message string
		if progressMap, ok := params.Value.(map[string]interface{}); ok {
			// Look for a message field that might contain the content
			if msg, ok := progressMap["message"].(string); ok {
				// The message format is "chunk X: content"
				parts := strings.SplitN(msg, ": ", 2)
				if len(parts) == 2 {
					content := parts[1]
					if content != "" {
						h.accumulatedContent += content

						// Send the chunk to the WebSocket client
						response := ChatResponse{
							Role:    "assistant",
							Content: content,
							Done:    false,
						}

						if err := wsutil.WriteServerText(h.conn, mustMarshal(response)); err != nil {
							log.Printf("Error writing to WebSocket: %v", err)
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

// GetAccumulatedContent returns the accumulated content
func (h *ProgressHandler) GetAccumulatedContent() string {
	return h.accumulatedContent
}

// RunWebServer starts a web server that interacts with OpenAI API via MCP
func RunWebServer() {
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

	// Connect to the MCP servers
	if err := mcp.Connect(); err != nil {
		log.Fatalf("Failed to connect to MCP servers: %v", err)
	}
	defer mcp.Close()

	// Set up the web server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("examples/openai/client/index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	// WebSocket upgrade endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// Upgrade connection to WebSocket
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Handle the WebSocket connection in a goroutine
		go handleWebSocket(conn, mcp, ctx)
	})

	// Start the server
	fmt.Println("Starting web server on port 3366...")
	fmt.Println("Open your browser to http://localhost:3366")
	log.Fatal(http.ListenAndServe(":3366", nil))
}

// handleWebSocket handles the WebSocket connection for chat
func handleWebSocket(conn net.Conn, mcp *client.MCP, ctx context.Context) {
	defer conn.Close()

	for {
		// Read message from WebSocket
		msg, _, err := wsutil.ReadClientData(conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading WebSocket message: %v", err)
			}
			break
		}

		// Parse the request
		var chatReq ChatRequest
		if err := json.Unmarshal(msg, &chatReq); err != nil {
			log.Printf("Error parsing chat request: %v", err)
			continue
		}

		// Get server name
		var serverName string
		for name := range mcp.Servers {
			serverName = name
			break
		}

		if serverName == "" {
			sendErrorResponse(conn, "No MCP servers available")
			continue
		}

		// Set up the arguments for the chat completion
		args := map[string]interface{}{
			"model": "gpt-3.5-turbo",
			"messages": []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: chatReq.Message,
				},
			},
			"stream": true,
		}

		// Create a progress handler to process streaming responses
		progressHandler := NewProgressHandler(conn)

		// Register the progress handler with the server's client
		server := mcp.Servers[serverName]
		server.OnProgress(func(params *protocol.ProgressParams) error {
			return progressHandler.Handle(params)
		})

		// Create a progress channel for the CallTool method
		progressCh := make(chan protocol.ProgressParams)

		// Call the OpenAI tool and process the response
		contents, err := server.CallTool(ctx, "chat_completion", args, progressCh)
		if err != nil {
			sendErrorResponse(conn, fmt.Sprintf("Error calling chat_completion: %v", err))
			continue
		}

		// Send final message to indicate completion
		finalContent := progressHandler.GetAccumulatedContent()

		// If the accumulated content is empty but we have a response,
		// use that instead (non-streaming case)
		if finalContent == "" && len(contents) > 0 {
			for _, content := range contents {
				if content.GetType() == "text" {
					if textContent, ok := content.(protocol.TextContent); ok {
						finalContent = textContent.Text
						break
					}
				}
			}
		}

		finalResponse := ChatResponse{
			Role:    "assistant",
			Content: finalContent,
			Done:    true,
		}

		if err := wsutil.WriteServerText(conn, mustMarshal(finalResponse)); err != nil {
			log.Printf("Error writing final response to WebSocket: %v", err)
		}
	}
}

// sendErrorResponse sends an error message over the WebSocket
func sendErrorResponse(conn net.Conn, errMsg string) {
	response := ChatResponse{
		Role:    "system",
		Content: "Error: " + errMsg,
		Done:    true,
	}

	wsutil.WriteServerText(conn, mustMarshal(response))
}

// mustMarshal marshals an object to JSON or panics
func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
