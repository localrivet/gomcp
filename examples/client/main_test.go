package main

import (
	// Needed for server handler context
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"testing"

	mcp "github.com/localrivet/gomcp"
)

// createTestConnections creates a pair of connected in-memory pipes
// suitable for testing client-server interactions without real network I/O.
func createTestConnections() (*mcp.Connection, *mcp.Connection) {
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	// Use NewConnectionWithClosers to ensure pipes are closed properly
	serverConn := mcp.NewConnectionWithClosers(serverReader, serverWriter, serverReader, serverWriter)
	clientConn := mcp.NewConnectionWithClosers(clientReader, clientWriter, clientReader, clientWriter)
	return serverConn, clientConn
}

// TestExampleClientLogic runs the client logic and simulates server responses.
func TestExampleClientLogic(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard) // Discard logs during test run
	defer log.SetOutput(originalOutput)

	serverConn, clientConn := createTestConnections()
	// clientConn will be closed by client.Close() called via defer in runClientLogic
	// We still defer close on serverConn in case the test panics before server goroutine finishes
	defer serverConn.Close()

	testServerName := "TestClientLogicServer"
	testClientName := "TestClientLogicClient-Refactored" // Update client name

	var serverWg sync.WaitGroup // Wait group for server simulator
	var clientWg sync.WaitGroup // Wait group for client logic
	var clientErr error         // To capture error from client goroutine

	// Simulate server responses in a goroutine
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		defer serverConn.Close() // Ensure server connection is closed when simulator exits

		// Helper to receive a raw JSON message and decode its basic structure
		expectRawMessage := func() (map[string]interface{}, bool) {
			rawJSON, err := serverConn.ReceiveRawMessage()
			if err != nil {
				if err != io.EOF && !strings.Contains(err.Error(), "pipe") && !strings.Contains(err.Error(), "closed") {
					// Use t.Helper() in test helpers if possible, otherwise log
					log.Printf("ERROR (Server simulator): Error receiving message: %v", err)
				} else {
					log.Printf("Server simulator: exiting due to client disconnect or closed pipe (%v)", err)
				}
				return nil, false // Indicate failure or clean exit
			}
			var baseMsg map[string]interface{}
			if err := json.Unmarshal(rawJSON, &baseMsg); err != nil {
				log.Printf("ERROR (Server simulator): Failed to unmarshal raw message: %v. Raw: %s", err, string(rawJSON))
				return nil, false // Indicate failure
			}
			log.Printf("Server simulator: Received Raw: %s", string(rawJSON))
			return baseMsg, true
		}

		// Helper to send a JSON-RPC response
		sendResponse := func(id interface{}, result interface{}) bool {
			err := serverConn.SendResponse(id, result)
			if err != nil {
				log.Printf("ERROR (Server simulator): failed to send response (ID: %v): %v", id, err)
				return false
			}
			log.Printf("Server simulator: Sent Response (ID: %v)", id)
			return true
		}

		// Helper to send a JSON-RPC error response
		sendError := func(id interface{}, code int, message string) bool {
			errPayload := mcp.ErrorPayload{Code: code, Message: message}
			err := serverConn.SendErrorResponse(id, errPayload)
			if err != nil {
				log.Printf("ERROR (Server simulator): failed to send error response (ID: %v): %v", id, err)
				return false
			}
			log.Printf("Server simulator: Sent Error Response (ID: %v, Code: %d)", id, code)
			return true
		}

		// 1. Expect InitializeRequest, send InitializeResponse
		rawReq, ok := expectRawMessage()
		if !ok {
			t.Errorf("Server simulator: Failed to receive InitializeRequest")
			return
		}
		method, _ := rawReq["method"].(string)
		reqID, idOk := rawReq["id"]
		if method != mcp.MethodInitialize || !idOk {
			log.Printf("ERROR (Server simulator): Expected InitializeRequest, got method '%s', id present: %v", method, idOk)
			t.Errorf("Server simulator: Expected InitializeRequest, got method '%s', id present: %v", method, idOk)
			return
		}
		log.Printf("Server simulator: Received InitializeRequest (ID: %v)", reqID)

		// Send InitializeResult
		initResult := mcp.InitializeResult{
			ProtocolVersion: mcp.CurrentProtocolVersion,
			ServerInfo:      mcp.Implementation{Name: testServerName, Version: "0.1.0"},
			Capabilities: mcp.ServerCapabilities{ // Advertise capabilities
				Tools: &struct {
					ListChanged bool `json:"listChanged,omitempty"`
				}{ListChanged: true}, // Assume server supports list_changed
				Resources: &struct {
					Subscribe   bool `json:"subscribe,omitempty"`
					ListChanged bool `json:"listChanged,omitempty"`
				}{Subscribe: true, ListChanged: true},
				Prompts: &struct {
					ListChanged bool `json:"listChanged,omitempty"`
				}{ListChanged: true},
				Logging: &struct{}{}, // Indicate logging support
			},
		}
		if !sendResponse(reqID, initResult) {
			t.Errorf("Server simulator: failed to send initialize resp")
			return
		}

		// 2. Expect InitializedNotification from client
		initNotifRaw, okInit := expectRawMessage()
		if !okInit || initNotifRaw["method"] != mcp.MethodInitialized || initNotifRaw["id"] != nil {
			log.Printf("ERROR (Server simulator): Expected Initialized notification, got: %+v", initNotifRaw)
			// Don't fail the test here, client might have errored before sending
		} else {
			log.Printf("Server simulator: Received Initialized notification.")
		}

		// 3. Expect ListToolsRequest, send ListToolsResponse
		rawReq, ok = expectRawMessage()
		if !ok {
			t.Errorf("Server simulator: Failed to receive ListToolsRequest")
			return
		}
		method, _ = rawReq["method"].(string)
		reqID, idOk = rawReq["id"]
		if method != mcp.MethodListTools || !idOk {
			t.Errorf("Server simulator: Expected ListToolsRequest")
			return
		}
		log.Printf("Server simulator: Received ListToolsRequest (ID: %v)", reqID)
		tools := []mcp.Tool{
			{Name: "echo", Description: "Test Echo", InputSchema: mcp.ToolInputSchema{Type: "object"}},
			{Name: "calculator", Description: "Test Calc", InputSchema: mcp.ToolInputSchema{Type: "object"}},
			{Name: "filesystem", Description: "Test FS", InputSchema: mcp.ToolInputSchema{Type: "object"}},
		}
		tdResp := mcp.ListToolsResult{Tools: tools}
		if !sendResponse(reqID, tdResp) {
			t.Errorf("Server simulator: failed to send list tools resp")
			return
		}

		// 4. Expect CallToolRequest (echo), send CallToolResponse
		rawReq, ok = expectRawMessage()
		if !ok {
			t.Errorf("Server simulator: Failed to receive CallToolRequest(echo)")
			return
		}
		method, _ = rawReq["method"].(string)
		reqID, idOk = rawReq["id"]
		paramsRaw, paramsOk := rawReq["params"]
		if method != mcp.MethodCallTool || !idOk || !paramsOk {
			t.Errorf("Server simulator: Expected CallToolRequest(echo)")
			return
		}
		log.Printf("Server simulator: Received CallToolRequest(echo) (ID: %v)", reqID)
		var ctReqEcho mcp.CallToolParams
		if err := mcp.UnmarshalPayload(paramsRaw, &ctReqEcho); err != nil || ctReqEcho.Name != "echo" {
			t.Errorf("Server simulator: failed to unmarshal echo req or wrong tool name: %v", err)
			return
		}
		echoResp := mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: ctReqEcho.Arguments["message"].(string)}},
		}
		if !sendResponse(reqID, echoResp) {
			t.Errorf("Server simulator: failed to send echo resp")
			return
		}

		// 5. Expect CallToolRequest (calculator add), send CallToolResponse
		rawReq, ok = expectRawMessage()
		if !ok {
			t.Errorf("Server simulator: Failed to receive CallToolRequest(add)")
			return
		}
		method, _ = rawReq["method"].(string)
		reqID, idOk = rawReq["id"]
		if method != mcp.MethodCallTool || !idOk {
			t.Errorf("Server simulator: Expected CallToolRequest(add)")
			return
		}
		log.Printf("Server simulator: Received CallToolRequest(add) (ID: %v)", reqID)
		calcResp1 := mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fmt.Sprintf("%f", 12.0)}},
		}
		if !sendResponse(reqID, calcResp1) {
			t.Errorf("Server simulator: failed to send calc add resp")
			return
		}

		// 6. Expect CallToolRequest (calculator divide by zero), send CallToolResponse with error
		rawReq, ok = expectRawMessage()
		if !ok {
			t.Errorf("Server simulator: Failed to receive CallToolRequest(div0)")
			return
		}
		method, _ = rawReq["method"].(string)
		reqID, idOk = rawReq["id"]
		if method != mcp.MethodCallTool || !idOk {
			t.Errorf("Server simulator: Expected CallToolRequest(div0)")
			return
		}
		log.Printf("Server simulator: Received CallToolRequest(div0) (ID: %v)", reqID)
		isErrTrue := true
		calcErr2 := mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "Division by zero"}},
			IsError: &isErrTrue,
		}
		if !sendResponse(reqID, calcErr2) {
			t.Errorf("Server simulator: failed to send calc div0 err resp")
			return
		}

		// 7. Expect CallToolRequest (calculator missing arg), send CallToolResponse with error
		rawReq, ok = expectRawMessage()
		if !ok {
			t.Errorf("Server simulator: Failed to receive CallToolRequest(missing arg)")
			return
		}
		method, _ = rawReq["method"].(string)
		reqID, idOk = rawReq["id"]
		if method != mcp.MethodCallTool || !idOk {
			t.Errorf("Server simulator: Expected CallToolRequest(missing arg)")
			return
		}
		log.Printf("Server simulator: Received CallToolRequest(missing arg) (ID: %v)", reqID)
		calcErr3 := mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "Missing required arguments"}},
			IsError: &isErrTrue,
		}
		if !sendResponse(reqID, calcErr3) {
			t.Errorf("Server simulator: failed to send calc miss err resp")
			return
		}

		// 8. Expect CallToolRequest (filesystem list), send CallToolResponse
		rawReq, ok = expectRawMessage()
		if !ok {
			t.Errorf("Server simulator: Failed to receive CallToolRequest(fs list)")
			return
		}
		method, _ = rawReq["method"].(string)
		reqID, idOk = rawReq["id"]
		if method != mcp.MethodCallTool || !idOk {
			t.Errorf("Server simulator: Expected CallToolRequest(fs list)")
			return
		}
		log.Printf("Server simulator: Received CallToolRequest(fs list) (ID: %v)", reqID)
		// Simulate an empty directory for simplicity in test
		fsResultMap1 := map[string]interface{}{"files": []interface{}{}}
		fsResultBytes1, _ := json.Marshal(fsResultMap1)
		fsResp1 := mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(fsResultBytes1)}},
		}
		if !sendResponse(reqID, fsResp1) {
			t.Errorf("Server simulator: failed to send fs list resp")
			return
		}

		// 9. Expect CallToolRequest (filesystem write), send CallToolResponse
		rawReq, ok = expectRawMessage()
		if !ok {
			t.Errorf("Server simulator: Failed to receive CallToolRequest(fs write)")
			return
		}
		method, _ = rawReq["method"].(string)
		reqID, idOk = rawReq["id"]
		if method != mcp.MethodCallTool || !idOk {
			t.Errorf("Server simulator: Expected CallToolRequest(fs write)")
			return
		}
		log.Printf("Server simulator: Received CallToolRequest(fs write) (ID: %v)", reqID)
		fsResultMap2 := map[string]interface{}{"status": "success", "message": "Simulated write success"}
		fsResultBytes2, _ := json.Marshal(fsResultMap2)
		fsResp2 := mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(fsResultBytes2)}},
		}
		if !sendResponse(reqID, fsResp2) {
			t.Errorf("Server simulator: failed to send fs write resp")
			return
		}

		// 10. Expect CallToolRequest (filesystem read), send CallToolResponse
		rawReq, ok = expectRawMessage()
		if !ok {
			t.Errorf("Server simulator: Failed to receive CallToolRequest(fs read)")
			return
		}
		method, _ = rawReq["method"].(string)
		reqID, idOk = rawReq["id"]
		if method != mcp.MethodCallTool || !idOk {
			t.Errorf("Server simulator: Expected CallToolRequest(fs read)")
			return
		}
		log.Printf("Server simulator: Received CallToolRequest(fs read) (ID: %v)", reqID)
		fsResp3 := mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "This is the content of the test file.\nIt has multiple lines."}},
		}
		if !sendResponse(reqID, fsResp3) {
			t.Errorf("Server simulator: failed to send fs read resp")
			return
		}

		// 11. Expect CallToolRequest (filesystem list dir), send CallToolResponse
		rawReq, ok = expectRawMessage()
		if !ok {
			t.Errorf("Server simulator: Failed to receive CallToolRequest(fs list dir)")
			return
		}
		method, _ = rawReq["method"].(string)
		reqID, idOk = rawReq["id"]
		if method != mcp.MethodCallTool || !idOk {
			t.Errorf("Server simulator: Expected CallToolRequest(fs list dir)")
			return
		}
		log.Printf("Server simulator: Received CallToolRequest(fs list dir) (ID: %v)", reqID)
		fsResultMap4 := map[string]interface{}{"files": []interface{}{map[string]interface{}{"name": "my_file.txt", "is_dir": false, "size": 46, "mod_time": "2024-01-01T12:00:00Z"}}} // Example data
		fsResultBytes4, _ := json.Marshal(fsResultMap4)
		fsResp4 := mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(fsResultBytes4)}},
		}
		if !sendResponse(reqID, fsResp4) {
			t.Errorf("Server simulator: failed to send fs list dir resp")
			return
		}

		// 12. Expect CallToolRequest (filesystem read non-existent), send CallToolResponse with error
		rawReq, ok = expectRawMessage()
		if !ok {
			t.Errorf("Server simulator: Failed to receive CallToolRequest(fs read non-existent)")
			return
		}
		method, _ = rawReq["method"].(string)
		reqID, idOk = rawReq["id"]
		if method != mcp.MethodCallTool || !idOk {
			t.Errorf("Server simulator: Expected CallToolRequest(fs read non-existent)")
			return
		}
		log.Printf("Server simulator: Received CallToolRequest(fs read non-existent) (ID: %v)", reqID)
		fsErr5 := mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "File not found at path 'non_existent_file.txt'"}},
			IsError: &isErrTrue,
		}
		if !sendResponse(reqID, fsErr5) {
			t.Errorf("Server simulator: failed to send fs read nf err resp")
			return
		}

		// 13. Expect CallToolRequest (filesystem write outside), send CallToolResponse with error
		rawReq, ok = expectRawMessage()
		if !ok {
			t.Errorf("Server simulator: Failed to receive CallToolRequest(fs write outside)")
			return
		}
		method, _ = rawReq["method"].(string)
		reqID, idOk = rawReq["id"]
		if method != mcp.MethodCallTool || !idOk {
			t.Errorf("Server simulator: Expected CallToolRequest(fs write outside)")
			return
		}
		log.Printf("Server simulator: Received CallToolRequest(fs write outside) (ID: %v)", reqID)
		fsErr6 := mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "path '../outside_sandbox.txt' attempts to escape the sandbox"}},
			IsError: &isErrTrue,
		}
		if !sendResponse(reqID, fsErr6) {
			t.Errorf("Server simulator: failed to send fs write sec err resp")
			return
		}

		// 14. Expect Ping request, send Ping response
		rawReq, ok = expectRawMessage()
		if !ok {
			t.Errorf("Server simulator: Failed to receive PingRequest")
			return
		}
		method, _ = rawReq["method"].(string)
		reqID, idOk = rawReq["id"]
		if method != mcp.MethodPing || !idOk {
			t.Errorf("Server simulator: Expected PingRequest, got method '%s', id present: %v", method, idOk)
			return
		}
		log.Printf("Server simulator: Received PingRequest (ID: %v)", reqID)
		if !sendResponse(reqID, nil) { // Ping response has null result
			t.Errorf("Server simulator: failed to send ping resp")
			return
		}

		// Server simulator done
		log.Println("Server simulator finished.")

	}()

	// Run client logic in a separate goroutine
	clientWg.Add(1)
	go func() {
		defer clientWg.Done()
		// Create client with its end of the pipe
		// Use NewClientWithConnection to provide the test connection
		client := mcp.NewClientWithConnection(testClientName, clientConn)
		// Run the client logic (which now internally calls Connect, etc.)
		clientErr = runClientLogic(testClientName) // Pass only name now
	}()

	// Wait for client and server simulator to finish
	clientWg.Wait()
	serverWg.Wait()

	// Assert results
	if clientErr != nil {
		t.Errorf("Client logic failed unexpectedly: %v", clientErr)
	}
}

// Helper function to get a pointer to a boolean value.
func BoolPtr(b bool) *bool {
	return &b
}

// Helper function to get a pointer to a string value.
func StringPtr(s string) *string {
	return &s
}
