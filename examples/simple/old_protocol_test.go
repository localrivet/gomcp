package main_test

import (
	"context"
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/transport/sse"
)

// TestOldProtocolClient creates a simple client that requests the 2024-11-05 protocol version
func TestOldProtocolClient(t *testing.T) {
	// Create a simple logger
	logger := &simpleLogger{}

	// Create an SSE client transport with the 2024-11-05 protocol version
	transport, err := sse.NewSSETransport(sse.SSETransportOptions{
		BaseURL:         "http://localhost:4477",
		BasePath:        "/mcp",
		Logger:          logger,
		ProtocolVersion: protocol.OldProtocolVersion, // Use the old 2024-11-05 protocol version
	})
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	// Set a timeout for the entire operation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Establish the receiver
	if err := transport.EstablishReceiver(ctx); err != nil {
		t.Fatalf("Failed to establish receiver: %v", err)
	}
	defer transport.Close()

	t.Log("SSE transport established, sending initialize request...")

	// Create initialize request with 2024-11-05 protocol version
	initReq := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  protocol.MethodInitialize,
		Params: protocol.InitializeRequestParams{
			ProtocolVersion: protocol.OldProtocolVersion,
			ClientInfo: protocol.Implementation{
				Name:    "old-protocol-test",
				Version: "1.0.0",
			},
			Capabilities: protocol.ClientCapabilities{},
		},
	}

	// Marshal the request
	initReqBytes, err := json.Marshal(initReq)
	if err != nil {
		t.Fatalf("Failed to marshal initialize request: %v", err)
	}

	// Send the initialize request
	if err := transport.Send(ctx, initReqBytes); err != nil {
		t.Fatalf("Failed to send initialize request: %v", err)
	}

	t.Log("Initialize request sent, waiting for response...")

	// Wait for the response
	respBytes, err := transport.Receive(ctx)
	if err != nil {
		t.Fatalf("Failed to receive response: %v", err)
	}

	// Parse the response
	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check if response contains an error
	if resp.Error != nil {
		t.Fatalf("Error response: %+v", resp.Error)
	}

	// Try to parse the result
	var initResult protocol.InitializeResult
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("Failed to re-marshal result: %v", err)
	}

	if err := json.Unmarshal(resultBytes, &initResult); err != nil {
		t.Fatalf("Failed to unmarshal initialize result: %v", err)
	}

	t.Logf("Successfully received initialize response with protocol version: %s", initResult.ProtocolVersion)
	t.Logf("Server info: %s v%s", initResult.ServerInfo.Name, initResult.ServerInfo.Version)

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
		t.Fatalf("Failed to marshal tool request: %v", err)
	}

	if err := transport.Send(ctx, toolReqBytes); err != nil {
		t.Fatalf("Failed to send tool request: %v", err)
	}

	t.Log("Tool request sent, waiting for response...")

	// Wait for the tool response
	toolRespBytes, err := transport.Receive(ctx)
	if err != nil {
		t.Fatalf("Failed to receive tool response: %v", err)
	}

	// Parse the tool response
	var toolResp protocol.JSONRPCResponse
	if err := json.Unmarshal(toolRespBytes, &toolResp); err != nil {
		t.Fatalf("Failed to unmarshal tool response: %v", err)
	}

	// Check if response contains an error
	if toolResp.Error != nil {
		t.Fatalf("Error tool response: %+v", toolResp.Error)
	}

	t.Logf("Successfully received tool response: %+v", toolResp.Result)

	t.Log("Test completed successfully!")
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
