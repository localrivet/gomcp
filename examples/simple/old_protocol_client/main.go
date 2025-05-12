package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/transport/sse"
)

func main() {
	// Create a simple logger
	logger := &simpleLogger{}

	// Default to localhost if no server URL is provided
	serverURL := "http://localhost:4477"
	if len(os.Args) > 1 {
		serverURL = os.Args[1]
	}

	fmt.Printf("Connecting to MCP server at %s using protocol version %s\n",
		serverURL, protocol.OldProtocolVersion)

	// Create an SSE client transport with the 2024-11-05 protocol version
	transport, err := sse.NewSSETransport(sse.SSETransportOptions{
		BaseURL:         serverURL,
		BasePath:        "/mcp",
		Logger:          logger,
		ProtocolVersion: protocol.OldProtocolVersion, // Use the old 2024-11-05 protocol version
	})
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Set a timeout for the entire operation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Establish the receiver
	fmt.Println("Establishing SSE connection...")
	if err := transport.EstablishReceiver(ctx); err != nil {
		log.Fatalf("Failed to establish receiver: %v", err)
	}
	defer transport.Close()

	fmt.Println("SSE transport established, sending initialize request...")

	// Create initialize request with 2024-11-05 protocol version
	initReq := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  protocol.MethodInitialize,
		Params: protocol.InitializeRequestParams{
			ProtocolVersion: protocol.OldProtocolVersion,
			ClientInfo: protocol.Implementation{
				Name:    "old-protocol-client",
				Version: "1.0.0",
			},
			Capabilities: protocol.ClientCapabilities{},
		},
	}

	// Marshal the request
	initReqBytes, err := json.Marshal(initReq)
	if err != nil {
		log.Fatalf("Failed to marshal initialize request: %v", err)
	}

	// Send the initialize request
	if err := transport.Send(ctx, initReqBytes); err != nil {
		log.Fatalf("Failed to send initialize request: %v", err)
	}

	fmt.Println("Initialize request sent, waiting for response...")

	// Wait for the response
	respBytes, err := transport.Receive(ctx)
	if err != nil {
		log.Fatalf("Failed to receive response: %v", err)
	}

	// Parse the response
	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		log.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check if response contains an error
	if resp.Error != nil {
		log.Fatalf("Error response: %+v", resp.Error)
	}

	// Try to parse the result
	var initResult protocol.InitializeResult
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		log.Fatalf("Failed to re-marshal result: %v", err)
	}

	if err := json.Unmarshal(resultBytes, &initResult); err != nil {
		log.Fatalf("Failed to unmarshal initialize result: %v", err)
	}

	fmt.Printf("Successfully received initialize response with protocol version: %s\n", initResult.ProtocolVersion)
	fmt.Printf("Server info: %s v%s\n", initResult.ServerInfo.Name, initResult.ServerInfo.Version)

	// Now let's try calling a tool
	fmt.Println("\nNow trying to call a tool...")

	// Try calling a simple tool
	toolReq := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  protocol.MethodCallTool,
		Params: protocol.CallToolRequestParams{
			ToolCall: &protocol.ToolCall{
				ID:       "test-tool-call",
				ToolName: "datetime",
				Input:    json.RawMessage("{}"),
			},
		},
	}

	toolReqBytes, err := json.Marshal(toolReq)
	if err != nil {
		log.Fatalf("Failed to marshal tool request: %v", err)
	}

	if err := transport.Send(ctx, toolReqBytes); err != nil {
		log.Fatalf("Failed to send tool request: %v", err)
	}

	fmt.Println("Tool request sent, waiting for response...")

	// Wait for the tool response
	toolRespBytes, err := transport.Receive(ctx)
	if err != nil {
		log.Fatalf("Failed to receive tool response: %v", err)
	}

	// Parse the tool response
	var toolResp protocol.JSONRPCResponse
	if err := json.Unmarshal(toolRespBytes, &toolResp); err != nil {
		log.Fatalf("Failed to unmarshal tool response: %v", err)
	}

	// Check if response contains an error
	if toolResp.Error != nil {
		log.Fatalf("Error tool response: %+v", toolResp.Error)
	}

	// Print the raw result
	toolResultBytes, _ := json.MarshalIndent(toolResp.Result, "", "  ")
	fmt.Printf("Successfully received tool response:\n%s\n", string(toolResultBytes))

	fmt.Println("\nTest completed successfully!")
}

// Simple logger implementation
type simpleLogger struct{}

func (l *simpleLogger) Debug(format string, args ...interface{}) {
	log.Printf("[DEBUG] "+format, args...)
}

func (l *simpleLogger) Info(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}

func (l *simpleLogger) Warn(format string, args ...interface{}) {
	log.Printf("[WARN] "+format, args...)
}

func (l *simpleLogger) Error(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}

func (l *simpleLogger) SetLevel(level string) {
	// Simple logger ignores level setting
}
