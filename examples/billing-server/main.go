// examples/billing-server/main.go (Refactored)
package main

import (
	"context"       // Needed for handler signature
	"encoding/json" // For logging structured billing event & server loop
	"errors"        // Added for server loop
	"fmt"           // Added for server loop
	"io"            // Added for server loop
	"log"
	"os"
	"strings" // Added for server loop
	"sync"    // Added for mockSession
	"time"    // For timestamp

	// Import new packages
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types" // Added for logger
)

// For this simple example, the expected API key is hardcoded.
const expectedApiKey = "test-key-123"

// Define the chargeable echo tool
var chargeableEchoTool = protocol.Tool{
	Name:        "chargeable-echo",
	Description: "Echoes back the provided message (Simulates Billing/Tracking).",
	InputSchema: protocol.ToolInputSchema{
		Type: "object",
		Properties: map[string]protocol.PropertyDetail{
			"message": {Type: "string", Description: "The message to echo."},
		},
		Required: []string{"message"},
	},
}

// chargeableEchoHandlerFactory creates a handler closure that captures the apiKey.
func chargeableEchoHandlerFactory(apiKey string) server.ToolHandlerFunc {
	return func(ctx context.Context, progressToken *protocol.ProgressToken, arguments map[string]interface{}) (content []protocol.Content, isError bool) {
		log.Printf("Executing chargeable-echo tool with args: %v", arguments)

		newErrorContent := func(msg string) []protocol.Content {
			return []protocol.Content{protocol.TextContent{Type: "text", Text: msg}}
		}

		// --- Simulate Billing/Tracking Event ---
		billingEvent := map[string]interface{}{
			"event_type": "tool_usage",
			"api_key":    apiKey,
			"tool_name":  chargeableEchoTool.Name,
			"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		}
		eventJson, _ := json.Marshal(billingEvent)
		log.Printf("BILLING_EVENT: %s", string(eventJson))
		// --- End Billing/Tracking ---

		// --- Execute the "chargeable-echo" tool ---
		messageArg, ok := arguments["message"]
		if !ok {
			return newErrorContent("Missing required argument 'message' for tool 'chargeable-echo'"), true
		}
		messageStr, ok := messageArg.(string)
		if !ok {
			return newErrorContent("Argument 'message' for tool 'chargeable-echo' must be a string"), true
		}

		log.Printf("Chargeable Echoing message: %s", messageStr)
		successContent := protocol.TextContent{Type: "text", Text: messageStr}
		return []protocol.Content{successContent}, false
		// --- End chargeable-echo tool execution ---
	}
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
					log.Printf("ERROR: server failed to marshal response: %v", err)
					continue
				}
				if err := transport.Send(respBytes); err != nil {
					if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "pipe closed") {
						log.Println("Server loop: Client closed connection after response.")
						return nil // Client likely closed connection
					}
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
	return nil
}
func (s *mockSession) SendResponse(response protocol.JSONRPCResponse) error {
	log.Printf("MockSession %s received SendResponse (handled by server loop): ID %v", s.id, response.ID)
	return nil
}
func (s *mockSession) Close() error      { return nil }
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

	log.Println("Starting Billing/Tracking Example MCP Server (Refactored)...")

	// Create a new stdio transport
	transport := stdio.NewStdioTransport()

	// Create a logger
	logger := NewDefaultLogger() // Use locally defined helper

	// Create a new server instance using the library
	serverName := "GoBillingServer-Refactored"
	srv := server.NewServer(serverName, server.ServerOptions{ // Updated call
		Logger: logger,
	})

	// Create the handler closure, capturing the validated apiKey
	handler := chargeableEchoHandlerFactory(apiKey)

	// Register the tool and its handler
	err := srv.RegisterTool(chargeableEchoTool, handler)
	if err != nil {
		log.Fatalf("Failed to register chargeable-echo tool: %v", err)
	}

	// Create a mock session for the stdio transport
	session := newMockSession("stdio-session")

	// Run the server's message handling loop using the transport
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
