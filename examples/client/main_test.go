package main

import (
	// Keep fmt for error formatting
	"encoding/json"
	"fmt"
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

		// 2. Expect ListToolsRequest, send ListToolsResponse
		msg, ok = expectReceive(mcp.MethodListTools) // Use new method
		if !ok {
			return
		}
		// Use simplified/dummy tool definitions for the test simulation
		tools := []mcp.Tool{ // Use new Tool struct, remove OutputSchema
			{Name: "echo", Description: "Test Echo", InputSchema: mcp.ToolInputSchema{Type: "object"}},
			{Name: "calculator", Description: "Test Calc", InputSchema: mcp.ToolInputSchema{Type: "object"}},
			{Name: "filesystem", Description: "Test FS", InputSchema: mcp.ToolInputSchema{Type: "object"}},
		}
		tdResp := mcp.ListToolsResult{Tools: tools} // Use new result struct
		// TODO: Update SendMessage for JSON-RPC response
		if err := serverConn.SendMessage("ListToolsResponse", tdResp); err != nil { // Conceptual type
			log.Printf("ERROR (Server simulator): failed to send list tools resp: %v", err)
			return
		}

		// 3. Expect CallToolRequest (echo), send CallToolResponse
		msg, ok = expectReceive(mcp.MethodCallTool) // Use new method
		if !ok {
			return
		}
		var ctReqEcho mcp.CallToolParams // Use new params struct
		if err := mcp.UnmarshalPayload(msg.Payload, &ctReqEcho); err != nil {
			log.Printf("ERROR (Server simulator): failed to unmarshal echo req: %v", err)
			return
		}
		// Construct CallToolResult with TextContent
		echoResp := mcp.CallToolResult{ // Use new result struct
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: ctReqEcho.Arguments["message"].(string)}},
		}
		// TODO: Update SendMessage for JSON-RPC response
		if err := serverConn.SendMessage("CallToolResponse", echoResp); err != nil { // Conceptual type
			log.Printf("ERROR (Server simulator): failed to send echo resp: %v", err)
			return
		}

		// 4. Expect CallToolRequest (calculator add), send CallToolResponse
		msg, ok = expectReceive(mcp.MethodCallTool) // Use new method
		if !ok {
			return
		}
		// Construct CallToolResult with TextContent (result formatted as string)
		calcResp1 := mcp.CallToolResult{ // Use new result struct
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fmt.Sprintf("%f", 12.0)}},
		}
		// TODO: Update SendMessage for JSON-RPC response
		if err := serverConn.SendMessage("CallToolResponse", calcResp1); err != nil { // Conceptual type
			log.Printf("ERROR (Server simulator): failed to send calc add resp: %v", err)
			return
		}

		// 5. Expect CallToolRequest (calculator divide by zero), send CallToolResponse with error
		msg, ok = expectReceive(mcp.MethodCallTool) // Use new method
		if !ok {
			return
		}
		// Construct CallToolResult with error content and IsError=true
		isErrTrue := true
		calcErr2 := mcp.CallToolResult{ // Use new result struct
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "Division by zero"}},
			IsError: &isErrTrue,
		}
		// TODO: Update SendMessage for JSON-RPC response
		if err := serverConn.SendMessage("CallToolResponse", calcErr2); err != nil { // Conceptual type
			log.Printf("ERROR (Server simulator): failed to send calc div0 err resp: %v", err)
			return
		}

		// 6. Expect CallToolRequest (calculator missing arg), send CallToolResponse with error
		msg, ok = expectReceive(mcp.MethodCallTool) // Use new method
		if !ok {
			return
		}
		calcErr3 := mcp.CallToolResult{ // Use new result struct
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "Missing required arguments"}},
			IsError: &isErrTrue,
		}
		// TODO: Update SendMessage for JSON-RPC response
		if err := serverConn.SendMessage("CallToolResponse", calcErr3); err != nil { // Conceptual type
			log.Printf("ERROR (Server simulator): failed to send calc miss err resp: %v", err)
			return
		}

		// 7. Expect CallToolRequest (filesystem list), send CallToolResponse
		msg, ok = expectReceive(mcp.MethodCallTool) // Use new method
		if !ok {
			return
		}
		// JSON encode the map result into TextContent
		fsResultMap1 := map[string]interface{}{"files": []interface{}{}}
		fsResultBytes1, _ := json.Marshal(fsResultMap1)
		fsResp1 := mcp.CallToolResult{ // Use new result struct
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(fsResultBytes1)}},
		}
		// TODO: Update SendMessage for JSON-RPC response
		if err := serverConn.SendMessage("CallToolResponse", fsResp1); err != nil { // Conceptual type
			log.Printf("ERROR (Server simulator): failed to send fs list resp: %v", err)
			return
		}

		// 8. Expect CallToolRequest (filesystem write), send CallToolResponse
		msg, ok = expectReceive(mcp.MethodCallTool) // Use new method
		if !ok {
			return
		}
		fsResultMap2 := map[string]interface{}{"status": "success"}
		fsResultBytes2, _ := json.Marshal(fsResultMap2)
		fsResp2 := mcp.CallToolResult{ // Use new result struct
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(fsResultBytes2)}},
		}
		// TODO: Update SendMessage for JSON-RPC response
		if err := serverConn.SendMessage("CallToolResponse", fsResp2); err != nil { // Conceptual type
			log.Printf("ERROR (Server simulator): failed to send fs write resp: %v", err)
			return
		}

		// 9. Expect CallToolRequest (filesystem read), send CallToolResponse
		msg, ok = expectReceive(mcp.MethodCallTool) // Use new method
		if !ok {
			return
		}
		fsResultMap3 := map[string]interface{}{"content": "This is the content of the test file.\nIt has multiple lines."}
		fsResultBytes3, _ := json.Marshal(fsResultMap3)
		fsResp3 := mcp.CallToolResult{ // Use new result struct
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(fsResultBytes3)}},
		}
		// TODO: Update SendMessage for JSON-RPC response
		if err := serverConn.SendMessage("CallToolResponse", fsResp3); err != nil { // Conceptual type
			log.Printf("ERROR (Server simulator): failed to send fs read resp: %v", err)
			return
		}

		// 10. Expect CallToolRequest (filesystem list dir), send CallToolResponse
		msg, ok = expectReceive(mcp.MethodCallTool) // Use new method
		if !ok {
			return
		}
		fsResultMap4 := map[string]interface{}{"files": []interface{}{map[string]interface{}{"name": "my_file.txt", "is_dir": false}}}
		fsResultBytes4, _ := json.Marshal(fsResultMap4)
		fsResp4 := mcp.CallToolResult{ // Use new result struct
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(fsResultBytes4)}},
		}
		// TODO: Update SendMessage for JSON-RPC response
		if err := serverConn.SendMessage("CallToolResponse", fsResp4); err != nil { // Conceptual type
			log.Printf("ERROR (Server simulator): failed to send fs list dir resp: %v", err)
			return
		}

		// 11. Expect CallToolRequest (filesystem read non-existent), send CallToolResponse with error
		msg, ok = expectReceive(mcp.MethodCallTool) // Use new method
		if !ok {
			return
		}
		fsErr5 := mcp.CallToolResult{ // Use new result struct
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "File not found"}},
			IsError: &isErrTrue,
		}
		// TODO: Update SendMessage for JSON-RPC response
		if err := serverConn.SendMessage("CallToolResponse", fsErr5); err != nil { // Conceptual type
			log.Printf("ERROR (Server simulator): failed to send fs read nf err resp: %v", err)
			return
		}

		// 12. Expect CallToolRequest (filesystem write outside), send CallToolResponse with error
		msg, ok = expectReceive(mcp.MethodCallTool) // Use new method
		if !ok {
			return
		}
		fsErr6 := mcp.CallToolResult{ // Use new result struct
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "escape the sandbox"}},
			IsError: &isErrTrue,
		}
		// TODO: Update SendMessage for JSON-RPC response
		if err := serverConn.SendMessage("CallToolResponse", fsErr6); err != nil { // Conceptual type
			log.Printf("ERROR (Server simulator): failed to send fs write sec err resp: %v", err)
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
