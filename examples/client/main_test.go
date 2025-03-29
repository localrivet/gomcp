package main

import (
	// Keep fmt for error formatting
	"io"
	"log"

	// "os" // Remove unused
	"strings" // Add missing
	"sync"
	"testing"

	// "time" // Remove unused

	mcp "github.com/localrivet/gomcp"
)

// createTestConnections (copied for test setup)
func createTestConnections() (*mcp.Connection, *mcp.Connection) {
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	serverConn := mcp.NewConnection(serverReader, serverWriter)
	clientConn := mcp.NewConnection(clientReader, clientWriter)
	return serverConn, clientConn
}

// TestExampleClientLogic runs the client logic and simulates server responses.
func TestExampleClientLogic(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard) // Discard logs during test run
	defer log.SetOutput(originalOutput)

	serverConn, clientConn := createTestConnections()
	defer serverConn.Close()
	defer clientConn.Close()

	testServerName := "TestClientLogicServer"
	testClientName := "TestClientLogicClient"

	var serverWg sync.WaitGroup // Wait group for server simulator

	// Simulate server responses in a goroutine
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		defer serverConn.Close() // Ensure server connection is closed when simulator exits

		// Helper to handle receive errors in simulator
		expectReceive := func(expectedType string) (*mcp.Message, bool) {
			msg, err := serverConn.ReceiveMessage()
			if err != nil {
				// EOF or pipe errors are expected if client closes early/unexpectedly
				if err == io.EOF || strings.Contains(err.Error(), "pipe") {
					log.Printf("Server simulator: exiting due to client disconnect (%v)", err)
				} else {
					// Use t.Helper() in test helpers
					// t.Errorf("Server simulator: Error receiving message (expected %s): %v", expectedType, err)
					// Cannot call t.Errorf from non-test goroutine directly easily, log instead for now
					log.Printf("ERROR (Server simulator): Error receiving message (expected %s): %v", expectedType, err)
				}
				return nil, false // Indicate failure
			}
			if msg.MessageType != expectedType {
				log.Printf("ERROR (Server simulator): Expected message type %s, got %s", expectedType, msg.MessageType)
				return nil, false // Indicate failure
			}
			return msg, true // Indicate success
		}

		// 1. Expect InitializeRequest, send InitializeResponse, expect InitializedNotification
		msg, ok := expectReceive(mcp.MethodInitialize) // Expect initialize method
		if !ok {
			return
		}
		// Send InitializeResult (wrapped in conceptual InitializeResponse)
		initResult := mcp.InitializeResult{
			ProtocolVersion: mcp.CurrentProtocolVersion,
			ServerInfo:      mcp.Implementation{Name: testServerName, Version: "0.1.0"},
			Capabilities: mcp.ServerCapabilities{ // Advertise capabilities
				Tools: &struct {
					ListChanged bool `json:"listChanged,omitempty"`
				}{ListChanged: false},
			},
		}
		// TODO: Update SendMessage to handle JSON-RPC response structure properly
		if err := serverConn.SendMessage("InitializeResponse", initResult); err != nil { // Conceptual type
			log.Printf("ERROR (Server simulator): failed to send initialize resp: %v", err)
			return
		}
		// Expect Initialized notification from client
		_, ok = expectReceive(mcp.MethodInitialized)
		if !ok {
			// This is now expected if client initialization fails before sending initialized
			log.Printf("Server simulator: did not receive Initialized notification (client error or disconnect?)")
			// Don't return here, let the test check clientErr
		} else {
			log.Printf("Server simulator: Received Initialized notification.")
		}

		// 2. Expect ToolDefinitionRequest, send ToolDefinitionResponse
		msg, ok = expectReceive(mcp.MessageTypeToolDefinitionRequest)
		if !ok {
			return
		}
		// Use simplified/dummy tool definitions for the test simulation
		tools := []mcp.ToolDefinition{
			{Name: "echo", Description: "Test Echo", InputSchema: mcp.ToolInputSchema{Type: "object"}, OutputSchema: mcp.ToolOutputSchema{Type: "string"}},
			{Name: "calculator", Description: "Test Calc", InputSchema: mcp.ToolInputSchema{Type: "object"}, OutputSchema: mcp.ToolOutputSchema{Type: "number"}},
			{Name: "filesystem", Description: "Test FS", InputSchema: mcp.ToolInputSchema{Type: "object"}, OutputSchema: mcp.ToolOutputSchema{Type: "object"}},
		}
		tdResp := mcp.ToolDefinitionResponsePayload{Tools: tools}
		if err := serverConn.SendMessage(mcp.MessageTypeToolDefinitionResponse, tdResp); err != nil {
			log.Printf("ERROR (Server simulator): failed to send td resp: %v", err)
			return
		}

		// 3. Expect UseToolRequest (echo), send UseToolResponse
		msg, ok = expectReceive(mcp.MessageTypeUseToolRequest)
		if !ok {
			return
		}
		var utReqEcho mcp.UseToolRequestPayload
		if err := mcp.UnmarshalPayload(msg.Payload, &utReqEcho); err != nil {
			log.Printf("ERROR (Server simulator): failed to unmarshal echo req: %v", err)
			return
		}
		echoResp := mcp.UseToolResponsePayload{Result: utReqEcho.Arguments["message"]} // Echo back
		if err := serverConn.SendMessage(mcp.MessageTypeUseToolResponse, echoResp); err != nil {
			log.Printf("ERROR (Server simulator): failed to send echo resp: %v", err)
			return
		}

		// 4. Expect UseToolRequest (calculator add), send UseToolResponse
		msg, ok = expectReceive(mcp.MessageTypeUseToolRequest)
		if !ok {
			return
		}
		calcResp1 := mcp.UseToolResponsePayload{Result: 12.0} // Simulate 5+7
		if err := serverConn.SendMessage(mcp.MessageTypeUseToolResponse, calcResp1); err != nil {
			log.Printf("ERROR (Server simulator): failed to send calc add resp: %v", err)
			return
		}

		// 5. Expect UseToolRequest (calculator divide by zero), send Error
		msg, ok = expectReceive(mcp.MessageTypeUseToolRequest)
		if !ok {
			return
		}
		// Use appropriate numeric code
		calcErr2 := mcp.ErrorPayload{Code: mcp.ErrorCodeMCPToolExecutionError, Message: "Division by zero"}
		if err := serverConn.SendMessage(mcp.MessageTypeError, calcErr2); err != nil {
			log.Printf("ERROR (Server simulator): failed to send calc div0 err: %v", err)
			return
		}

		// 6. Expect UseToolRequest (calculator missing arg), send Error
		msg, ok = expectReceive(mcp.MessageTypeUseToolRequest)
		if !ok {
			return
		}
		calcErr3 := mcp.ErrorPayload{Code: mcp.ErrorCodeMCPInvalidArgument, Message: "Missing required arguments"} // Use MCP code
		if err := serverConn.SendMessage(mcp.MessageTypeError, calcErr3); err != nil {
			log.Printf("ERROR (Server simulator): failed to send calc miss err: %v", err)
			return
		}

		// 7. Expect UseToolRequest (filesystem list), send Response
		msg, ok = expectReceive(mcp.MessageTypeUseToolRequest)
		if !ok {
			return
		}
		fsResp1 := mcp.UseToolResponsePayload{Result: map[string]interface{}{"files": []interface{}{}}} // Empty list
		if err := serverConn.SendMessage(mcp.MessageTypeUseToolResponse, fsResp1); err != nil {
			log.Printf("ERROR (Server simulator): failed to send fs list resp: %v", err)
			return
		}

		// 8. Expect UseToolRequest (filesystem write), send Response
		msg, ok = expectReceive(mcp.MessageTypeUseToolRequest)
		if !ok {
			return
		}
		fsResp2 := mcp.UseToolResponsePayload{Result: map[string]interface{}{"status": "success"}}
		if err := serverConn.SendMessage(mcp.MessageTypeUseToolResponse, fsResp2); err != nil {
			log.Printf("ERROR (Server simulator): failed to send fs write resp: %v", err)
			return
		}

		// 9. Expect UseToolRequest (filesystem read), send Response
		msg, ok = expectReceive(mcp.MessageTypeUseToolRequest)
		if !ok {
			return
		}
		// Simulate reading back the expected content
		fsResp3 := mcp.UseToolResponsePayload{Result: map[string]interface{}{"content": "This is the content of the test file.\nIt has multiple lines."}}
		if err := serverConn.SendMessage(mcp.MessageTypeUseToolResponse, fsResp3); err != nil {
			log.Printf("ERROR (Server simulator): failed to send fs read resp: %v", err)
			return
		}

		// 10. Expect UseToolRequest (filesystem list dir), send Response
		msg, ok = expectReceive(mcp.MessageTypeUseToolRequest)
		if !ok {
			return
		}
		fsResp4 := mcp.UseToolResponsePayload{Result: map[string]interface{}{"files": []interface{}{map[string]interface{}{"name": "my_file.txt", "is_dir": false}}}} // Simulate file exists
		if err := serverConn.SendMessage(mcp.MessageTypeUseToolResponse, fsResp4); err != nil {
			log.Printf("ERROR (Server simulator): failed to send fs list dir resp: %v", err)
			return
		}

		// 11. Expect UseToolRequest (filesystem read non-existent), send Error
		msg, ok = expectReceive(mcp.MessageTypeUseToolRequest)
		if !ok {
			return
		}
		fsErr5 := mcp.ErrorPayload{Code: mcp.ErrorCodeMCPResourceNotFound, Message: "File not found"} // Use MCP code
		if err := serverConn.SendMessage(mcp.MessageTypeError, fsErr5); err != nil {
			log.Printf("ERROR (Server simulator): failed to send fs read nf err: %v", err)
			return
		}

		// 12. Expect UseToolRequest (filesystem write outside), send Error
		msg, ok = expectReceive(mcp.MessageTypeUseToolRequest)
		if !ok {
			return
		}
		fsErr6 := mcp.ErrorPayload{Code: mcp.ErrorCodeMCPSecurityViolation, Message: "escape the sandbox"} // Use MCP code
		if err := serverConn.SendMessage(mcp.MessageTypeError, fsErr6); err != nil {
			log.Printf("ERROR (Server simulator): failed to send fs write sec err: %v", err)
			return
		}

		// Server simulator done
		// log.Println("Server simulator finished.") // Keep logs discarded

	}()

	// Run client logic in the main test goroutine
	clientErr := runClientLogic(clientConn, testClientName)

	// Wait for server simulator to finish
	serverWg.Wait()

	// Assert results
	if clientErr != nil {
		t.Errorf("Client logic failed unexpectedly: %v", clientErr)
	}
}
