// examples/auth-server/main.go (Refactored)
package main

import (
	"context"
	"encoding/json" // Added for server loop
	"errors"        // Added for server loop
	"fmt"           // Added for server loop
	"io"            // Added for server loop
	"log"
	"os"
	"strings" // Added for server loop
	"sync"

	// Added for server loop
	// Import new packages
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types" // Added for logger
)

// For this simple example, the expected API key is hardcoded.
const expectedApiKey = "test-key-123"

// Define the secure echo tool
var secureEchoTool = protocol.Tool{
	Name:        "secure-echo",
	Description: "Echoes back the provided message (Requires API Key Auth).",
	InputSchema: protocol.ToolInputSchema{
		Type: "object",
		Properties: map[string]protocol.PropertyDetail{
			"message": {Type: "string", Description: "The message to echo."},
		},
		Required: []string{"message"},
	},
}

// secureEchoHandler implements the logic for the secure-echo tool.
func secureEchoHandler(ctx context.Context, progressToken *protocol.ProgressToken, arguments map[string]interface{}) (content []protocol.Content, isError bool) {
	log.Printf("Executing secure-echo tool with args: %v", arguments)
	// API key check happened at startup.

	newErrorContent := func(msg string) []protocol.Content {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: msg}}
	}

	messageArg, ok := arguments["message"]
	if !ok {
		return newErrorContent("Missing required argument 'message' for tool 'secure-echo'"), true
	}
	messageStr, ok := messageArg.(string)
	if !ok {
		return newErrorContent("Argument 'message' for tool 'secure-echo' must be a string"), true
	}
	log.Printf("Securely Echoing message: %s", messageStr)
	successContent := protocol.TextContent{Type: "text", Text: messageStr}
	return []protocol.Content{successContent}, false
}

// runServerLoop simulates the transport receiving messages and passing them to the server.
// Copied from initialize_test.go refactoring.
func runServerLoop(ctx context.Context, srv *server.Server, transport types.Transport, session server.ClientSession) error {
	if err := srv.RegisterSession(session); err != nil {
		return fmt.Errorf("failed to register mock session: %w", err)
	}
	defer srv.UnregisterSession(session.SessionID())

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			rawMsg, err := transport.Receive()
			if err != nil {
				if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "pipe closed") {
					log.Println("Server loop: Client disconnected (EOF/pipe closed).")
					return nil // Clean exit
				}
				return fmt.Errorf("server transport receive error: %w", err)
			}

			response := srv.HandleMessage(context.Background(), session.SessionID(), rawMsg)

			if response != nil {
				respBytes, err := json.Marshal(response)
				if err != nil {
					// Log error but continue loop if possible
					log.Printf("ERROR: server failed to marshal response: %v", err)
					continue
				}
				if err := transport.Send(respBytes); err != nil {
					if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "pipe closed") {
						log.Println("Server loop: Client closed connection after response.")
						return nil // Client likely closed connection
					}
					// Log error but attempt to continue
					log.Printf("ERROR: server transport send error: %v", err)
					// return fmt.Errorf("server transport send error: %w", err) // Option: exit on send error
				}
			}
		}
	}
}

// mockSession implements server.ClientSession for stdio testing.
// Copied from initialize_test.go refactoring.
type mockSession struct {
	id          string
	initialized bool
	mu          sync.Mutex
}

func newMockSession(id string) *mockSession { return &mockSession{id: id} }
func (s *mockSession) SessionID() string    { return s.id }
func (s *mockSession) SendNotification(notification protocol.JSONRPCNotification) error {
	log.Printf("MockSession %s received SendNotification (ignored in stdio): %s", s.id, notification.Method)
	return nil // Stdio doesn't typically send server->client notifications this way
}
func (s *mockSession) SendResponse(response protocol.JSONRPCResponse) error {
	log.Printf("MockSession %s received SendResponse (handled by server loop): ID %v", s.id, response.ID)
	return nil // Response is sent back via transport in runServerLoop
}
func (s *mockSession) Close() error      { return nil } // Transport handles closing
func (s *mockSession) Initialize()       { s.initialized = true }
func (s *mockSession) Initialized() bool { return s.initialized }

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	// --- API Key Check ---
	apiKey := os.Getenv("MCP_API_KEY")
	if apiKey == "" {
		log.Fatal("FATAL: MCP_API_KEY environment variable not set.")
	}
	if apiKey != expectedApiKey {
		log.Fatalf("FATAL: Invalid MCP_API_KEY provided. Expected '%s'", expectedApiKey)
	}
	log.Println("API Key validated successfully.")
	// --- End API Key Check ---

	log.Println("Starting Auth Example MCP Server (Refactored)...")

	// Create a new stdio transport
	transport := stdio.NewStdioTransport() // Uses os.Stdin, os.Stdout

	// Create a logger (can use default or custom)
	logger := NewDefaultLogger() // Use locally defined helper

	// Create a new server instance using the library
	serverName := "GoAuthServer-Refactored"
	srv := server.NewServer(serverName, server.ServerOptions{ // Updated call
		Logger: logger,
		// Define capabilities if needed, e.g., enabling tool list changes
		// ServerCapabilities: protocol.ServerCapabilities{ ... }
	})

	// Register the secure echo tool and its handler
	err := srv.RegisterTool(secureEchoTool, secureEchoHandler)
	if err != nil {
		log.Fatalf("Failed to register secure-echo tool: %v", err)
	}

	// Create a mock session for the stdio transport (doesn't have real sessions)
	session := newMockSession("stdio-session")

	// Run the server's message handling loop using the transport
	// Use a background context, can be enhanced with signal handling
	log.Println("Server listening on stdio...")
	err = runServerLoop(context.Background(), srv, transport, session) // Replaced srv.Run
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("Server loop exited with error: %v", err)
		os.Exit(1)
	} else if errors.Is(err, context.Canceled) {
		log.Println("Server loop canceled.")
	} else {
		log.Println("Server loop finished.")
	}
}

// Added Default Logger definition if not present in types
type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, args ...interface{}) { log.Printf("DEBUG: "+msg, args...) }
func (l *defaultLogger) Info(msg string, args ...interface{})  { log.Printf("INFO: "+msg, args...) }
func (l *defaultLogger) Warn(msg string, args ...interface{})  { log.Printf("WARN: "+msg, args...) }
func (l *defaultLogger) Error(msg string, args ...interface{}) { log.Printf("ERROR: "+msg, args...) }

// NewDefaultLogger creates a default logger.
func NewDefaultLogger() *defaultLogger {
	return &defaultLogger{}
}

// Ensure defaultLogger implements types.Logger
var _ types.Logger = (*defaultLogger)(nil)
