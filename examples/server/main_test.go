package main

import (
	"context" // Needed for dummy handlers
	"encoding/json"
	"errors" // Import errors package
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath" // Needed for dummy filesystem handler
	"strings"
	"sync"
	"testing"
	"time" // Needed for timeout

	"github.com/localrivet/gomcp"
	// Use google/uuid for request IDs in the test simulation
)

// --- Test Setup ---

// createTestConnections creates a pair of connected in-memory pipes
// suitable for testing client-server interactions without real network I/O.
func createTestConnections() (*gomcp.Connection, *gomcp.Connection) {
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	// Use the standard NewConnection. Closing the pipes handles cleanup.
	serverConn := gomcp.NewConnection(serverReader, serverWriter)
	clientConn := gomcp.NewConnection(clientReader, clientWriter)
	return serverConn, clientConn
}

// Helper to receive and decode a JSON-RPC response within a timeout
func receiveResponse(t *testing.T, conn *gomcp.Connection, expectedID interface{}, timeout time.Duration) (*gomcp.JSONRPCResponse, error) {
	t.Helper()
	rawJSONChan := make(chan []byte)
	errChan := make(chan error, 1)

	go func() {
		raw, err := conn.ReceiveRawMessage()
		if err != nil {
			errChan <- err
			return
		}
		rawJSONChan <- raw
	}()

	select {
	case rawJSON := <-rawJSONChan:
		// log.Printf("Client simulator received raw: %s", string(rawJSON)) // Uncomment for debugging
		var resp gomcp.JSONRPCResponse
		if err := json.Unmarshal(rawJSON, &resp); err != nil {
			// Try unmarshalling as a batch response (array) in case server sends one unexpectedly
			var batchResp []gomcp.JSONRPCResponse
			if errBatch := json.Unmarshal(rawJSON, &batchResp); errBatch == nil && len(batchResp) > 0 {
				// For simplicity in this test, just check the first response in the batch
				resp = batchResp[0]
			} else {
				return nil, fmt.Errorf("failed to unmarshal response JSON (not single or batch): %w. Raw: %s", err, string(rawJSON))
			}
		}

		// Check ID matching (handle potential type differences like int vs float64 from JSON)
		if expectedID != nil && fmt.Sprintf("%v", resp.ID) != fmt.Sprintf("%v", expectedID) {
			return nil, fmt.Errorf("response ID mismatch. Expected: %v, Got: %v", expectedID, resp.ID)
		}
		// Check for protocol-level error BEFORE checking for null result
		if resp.Error != nil {
			return nil, fmt.Errorf("received MCP Error: [%d] %s", resp.Error.Code, resp.Error.Message)
		}
		// Allow null result for Ping, otherwise check if result is present
		if resp.Result == nil && resp.Error == nil {
			// We need method context to definitively say null result is bad,
			// but for this test's flow, only Ping should have a null result.
			// We'll check specific results later.
		}
		return &resp, nil
	case err := <-errChan:
		// Check for expected pipe closure errors which might be okay depending on test stage
		if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") {
			return nil, fmt.Errorf("connection closed while waiting for response (Expected ID: %v): %w", expectedID, err)
		}
		return nil, fmt.Errorf("error receiving response: %w", err)
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for response (Expected ID: %v)", expectedID)
	}
}

// Helper to receive and decode a JSON-RPC message (Request or Notification) within a timeout
func receiveRequestOrNotification(t *testing.T, conn *gomcp.Connection, timeout time.Duration) (map[string]interface{}, error) {
	t.Helper()
	rawJSONChan := make(chan []byte)
	errChan := make(chan error, 1)

	go func() {
		raw, err := conn.ReceiveRawMessage()
		if err != nil {
			errChan <- err
			return
		}
		rawJSONChan <- raw
	}()

	select {
	case rawJSON := <-rawJSONChan:
		// log.Printf("Server simulator received raw: %s", string(rawJSON)) // Uncomment for debugging
		var baseMsg map[string]interface{}
		if err := json.Unmarshal(rawJSON, &baseMsg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal raw message: %w. Raw: %s", err, string(rawJSON))
		}
		// Basic JSON-RPC validation
		if _, ok := baseMsg["jsonrpc"].(string); !ok || baseMsg["jsonrpc"] != "2.0" {
			return nil, fmt.Errorf("invalid or missing jsonrpc version: %v", baseMsg["jsonrpc"])
		}
		if _, ok := baseMsg["method"].(string); !ok {
			// Could be a response, but this helper expects requests/notifications
			return nil, fmt.Errorf("message is not a request or notification (missing method)")
		}
		return baseMsg, nil
	case err := <-errChan:
		// Check for expected pipe closure errors
		if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") {
			return nil, fmt.Errorf("connection closed while waiting for message: %w", err)
		}
		return nil, fmt.Errorf("error receiving message: %w", err)
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for message")
	}
}

// --- Dummy Handlers for Server Simulation (Needed because test is in package main) ---
var testEchoTool = gomcp.Tool{
	Name:        "echo",
	Description: "Test Echo",
	InputSchema: gomcp.ToolInputSchema{Type: "object", Properties: map[string]gomcp.PropertyDetail{"message": {Type: "string"}}, Required: []string{"message"}},
}

func testEchoHandler(ctx context.Context, pt *gomcp.ProgressToken, args map[string]interface{}) ([]gomcp.Content, bool) {
	msg, _ := args["message"].(string)
	return []gomcp.Content{gomcp.TextContent{Type: "text", Text: msg}}, false
}

var testCalculatorTool = gomcp.Tool{
	Name:        "calculator",
	Description: "Test Calc",
	InputSchema: gomcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]gomcp.PropertyDetail{
			"operand1":  {Type: "number"},
			"operand2":  {Type: "number"},
			"operation": {Type: "string", Enum: []interface{}{"add", "subtract", "multiply", "divide"}},
		},
		Required: []string{"operand1", "operand2", "operation"},
	},
}

func testCalculatorHandler(ctx context.Context, pt *gomcp.ProgressToken, args map[string]interface{}) ([]gomcp.Content, bool) {
	op1, ok1 := args["operand1"].(float64)
	op2, ok2 := args["operand2"].(float64)
	opStr, ok3 := args["operation"].(string)
	if !ok1 || !ok2 || !ok3 {
		return []gomcp.Content{gomcp.TextContent{Type: "text", Text: "Missing required arguments"}}, true
	}
	var result float64
	isErrTrue := true // Local variable for error flag pointer
	switch opStr {
	case "add":
		result = op1 + op2
	case "subtract":
		result = op1 - op2
	case "multiply":
		result = op1 * op2
	case "divide":
		if op2 == 0 {
			return []gomcp.Content{gomcp.TextContent{Type: "text", Text: "Division by zero"}}, isErrTrue
		}
		result = op1 / op2
	default:
		return []gomcp.Content{gomcp.TextContent{Type: "text", Text: "Invalid operation"}}, isErrTrue
	}
	return []gomcp.Content{gomcp.TextContent{Type: "text", Text: fmt.Sprintf("%f", result)}}, false
}

// Use a test-specific sandbox directory
const testFileSystemSandbox = "./fs_sandbox_test_server" // Renamed to avoid conflict

var testFileSystemTool = gomcp.Tool{
	Name:        "filesystem",
	Description: fmt.Sprintf("Test FS Tool (Sandbox: %s)", testFileSystemSandbox),
	InputSchema: gomcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]gomcp.PropertyDetail{
			"operation": {Type: "string", Enum: []interface{}{"list_files", "read_file", "write_file"}},
			"path":      {Type: "string"},
			"content":   {Type: "string"},
		},
		Required: []string{"operation", "path"},
	},
}

// Simplified filesystem handler for testing basic calls within a test sandbox
func testFilesystemHandler(ctx context.Context, pt *gomcp.ProgressToken, args map[string]interface{}) ([]gomcp.Content, bool) {
	op, _ := args["operation"].(string)
	relativePath, okPath := args["path"].(string)
	if !okPath {
		return []gomcp.Content{gomcp.TextContent{Type: "text", Text: "Missing path argument"}}, true
	}

	// Basic path validation for test sandbox
	if strings.Contains(relativePath, "..") || filepath.IsAbs(relativePath) {
		return []gomcp.Content{gomcp.TextContent{Type: "text", Text: "Invalid path: contains '..' or is absolute"}}, true
	}
	// Use filepath.Join which cleans the path
	safePath := filepath.Join(testFileSystemSandbox, relativePath)
	// Ensure it's still within the intended sandbox (double check after join)
	absSandbox, _ := filepath.Abs(testFileSystemSandbox) // Ignore error for test simplicity
	absSafePath, _ := filepath.Abs(safePath)
	if !strings.HasPrefix(absSafePath, absSandbox) {
		return []gomcp.Content{gomcp.TextContent{Type: "text", Text: fmt.Sprintf("Path '%s' attempts to escape the sandbox directory '%s'", relativePath, testFileSystemSandbox)}}, true
	}

	isErrTrue := true // Local variable for error flag pointer

	switch op {
	case "list_files":
		files, err := os.ReadDir(safePath)
		if err != nil {
			// If dir doesn't exist yet (e.g., listing root before write), return empty list
			if os.IsNotExist(err) {
				resultMap := map[string]interface{}{"files": []interface{}{}}
				resultBytes, _ := json.Marshal(resultMap)
				return []gomcp.Content{gomcp.TextContent{Type: "text", Text: string(resultBytes)}}, false
			}
			return []gomcp.Content{gomcp.TextContent{Type: "text", Text: fmt.Sprintf("Failed to list files: %v", err)}}, isErrTrue
		}
		var fileInfos []map[string]interface{}
		for _, file := range files {
			info, errInfo := file.Info()
			if errInfo == nil { // Only include files we can get info for
				fileInfos = append(fileInfos, map[string]interface{}{
					"name":     info.Name(),
					"is_dir":   info.IsDir(),
					"size":     info.Size(),
					"mod_time": info.ModTime().Format(time.RFC3339),
				})
			}
		}
		// Return result as JSON string within TextContent
		resultMap := map[string]interface{}{"files": fileInfos}
		resultBytes, _ := json.Marshal(resultMap) // Ignore marshal error for test simplicity
		return []gomcp.Content{gomcp.TextContent{Type: "text", Text: string(resultBytes)}}, false

	case "write_file":
		contentToWrite, okContent := args["content"].(string)
		if !okContent {
			return []gomcp.Content{gomcp.TextContent{Type: "text", Text: "Missing content argument for write_file"}}, isErrTrue
		}
		parentDir := filepath.Dir(safePath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return []gomcp.Content{gomcp.TextContent{Type: "text", Text: fmt.Sprintf("Failed to create parent dir: %v", err)}}, isErrTrue
		}
		err := os.WriteFile(safePath, []byte(contentToWrite), 0644)
		if err != nil {
			return []gomcp.Content{gomcp.TextContent{Type: "text", Text: fmt.Sprintf("Failed to write file: %v", err)}}, isErrTrue
		}
		// Return success message as JSON string within TextContent
		resultMap := map[string]interface{}{"status": "success", "message": fmt.Sprintf("Successfully wrote %d bytes to '%s'", len(contentToWrite), relativePath)}
		resultBytes, _ := json.Marshal(resultMap)
		return []gomcp.Content{gomcp.TextContent{Type: "text", Text: string(resultBytes)}}, false

	case "read_file":
		contentBytes, err := os.ReadFile(safePath)
		if err != nil {
			if os.IsNotExist(err) {
				return []gomcp.Content{gomcp.TextContent{Type: "text", Text: fmt.Sprintf("File not found at path '%s'", relativePath)}}, isErrTrue
			}
			return []gomcp.Content{gomcp.TextContent{Type: "text", Text: fmt.Sprintf("Failed to read file: %v", err)}}, isErrTrue
		}
		// Return file content directly as TextContent
		return []gomcp.Content{gomcp.TextContent{Type: "text", Text: string(contentBytes)}}, false

	default:
		return []gomcp.Content{gomcp.TextContent{Type: "text", Text: fmt.Sprintf("Invalid operation '%s'", op)}}, isErrTrue
	}
}

// --- Test Function ---

// TestExampleServerLogic runs the server logic using the refactored gomcp.Server
// and simulates a basic client interaction using the Connection directly.
func TestExampleServerLogic(t *testing.T) {
	originalOutput := log.Writer()
	// log.SetOutput(os.Stderr) // Enable for debugging test
	log.SetOutput(io.Discard) // Discard logs during test run
	defer log.SetOutput(originalOutput)

	serverConn, clientConn := createTestConnections()
	defer serverConn.Close()
	// clientConn will be closed by the client simulation goroutine

	testServerName := "TestServerLogicServer"
	testClientName := "TestServerLogicClient-Refactored"

	// Clean up sandbox directory before and after test
	_ = os.RemoveAll(testFileSystemSandbox) // Use test-specific sandbox
	defer func() {
		log.SetOutput(originalOutput) // Restore log output before cleanup logs
		log.Printf("Cleaning up test sandbox directory: %s", testFileSystemSandbox)
		_ = os.RemoveAll(testFileSystemSandbox)
	}()

	var serverWg sync.WaitGroup
	var serverErr error // Declare serverErr in the outer scope

	// Run server logic in a goroutine using the gomcp.Server
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		// Create server instance using the new constructor with the test connection
		server := gomcp.NewServerWithConnection(testServerName, serverConn)
		if server == nil {
			serverErr = fmt.Errorf("test setup: NewServerWithConnection returned nil")
			return
		}

		// Register tools using the dummy handlers defined in this test file
		if err := server.RegisterTool(testEchoTool, testEchoHandler); err != nil {
			serverErr = fmt.Errorf("test setup: failed to register echo tool: %w", err)
			serverConn.Close() // Close conn if setup fails
			return
		}
		if err := server.RegisterTool(testCalculatorTool, testCalculatorHandler); err != nil {
			serverErr = fmt.Errorf("test setup: failed to register calculator tool: %w", err)
			serverConn.Close()
			return
		}
		// Create the actual sandbox dir for the test
		if err := os.MkdirAll(testFileSystemSandbox, 0755); err != nil {
			serverErr = fmt.Errorf("test setup: failed to create test sandbox '%s': %w", testFileSystemSandbox, err)
			serverConn.Close()
			return
		}
		if err := server.RegisterTool(testFileSystemTool, testFilesystemHandler); err != nil { // Use the test handler
			serverErr = fmt.Errorf("test setup: failed to register filesystem tool: %w", err)
			serverConn.Close()
			return
		}

		// Run the server's main loop
		log.Println("Server simulator: Starting Run loop...")
		serverErr = server.Run() // Assign to the outer scope serverErr
		// Log server exit reason if not EOF/pipe error
		if serverErr != nil && !errors.Is(serverErr, io.EOF) && !strings.Contains(serverErr.Error(), "pipe") && !strings.Contains(serverErr.Error(), "closed") {
			log.Printf("Server simulator exited with error: %v", serverErr)
		} else {
			log.Printf("Server simulator exited cleanly or due to pipe closure.")
		}
	}()

	// --- Simulate Client Interaction ---
	clientErr := func() error {
		defer clientConn.Close() // Ensure client connection is closed when simulation finishes

		// 1. Send InitializeRequest, wait for InitializeResponse
		log.Println("Client simulator: Sending InitializeRequest...")
		clientCapabilities := gomcp.ClientCapabilities{} // Basic capabilities
		clientInfo := gomcp.Implementation{Name: testClientName, Version: "0.1.0"}
		initReqParams := gomcp.InitializeRequestParams{
			ProtocolVersion: gomcp.CurrentProtocolVersion,
			Capabilities:    clientCapabilities,
			ClientInfo:      clientInfo,
		}
		// Use the correct SendRequest signature: method, params -> returns id, err
		initReqID, err := clientConn.SendRequest(gomcp.MethodInitialize, initReqParams)
		if err != nil {
			return fmt.Errorf("client send initialize req failed: %w", err)
		}
		log.Printf("Client simulator: Sent InitializeRequest (ID: %s)", initReqID)

		initResp, err := receiveResponse(t, clientConn, initReqID, 5*time.Second)
		if err != nil {
			return fmt.Errorf("client recv initialize resp failed: %w", err)
		}
		var initResult gomcp.InitializeResult
		if initResp.Result == nil {
			return fmt.Errorf("client received null result for Initialize")
		}
		if err := gomcp.UnmarshalPayload(initResp.Result, &initResult); err != nil {
			return fmt.Errorf("client failed to unmarshal InitializeResult: %w", err)
		}
		if initResult.ServerInfo.Name != testServerName {
			return fmt.Errorf("unexpected server name: got %s, want %s", initResult.ServerInfo.Name, testServerName)
		}
		log.Printf("Client simulator: Received InitializeResponse, server: %s", initResult.ServerInfo.Name)

		// 2. Send Initialized Notification
		log.Println("Client simulator: Sending InitializedNotification...")
		initParams := gomcp.InitializedNotificationParams{}
		if err := clientConn.SendNotification(gomcp.MethodInitialized, initParams); err != nil {
			log.Printf("Client simulator warning: failed to send InitializedNotification: %v", err)
			// Continue anyway for testing other interactions
		}

		// 3. Send ListToolsRequest, wait for ListToolsResponse
		log.Println("Client simulator: Sending ListToolsRequest...")
		listToolsReqParams := gomcp.ListToolsRequestParams{}
		// Use the correct SendRequest signature
		listToolsReqID, err := clientConn.SendRequest(gomcp.MethodListTools, listToolsReqParams)
		if err != nil {
			return fmt.Errorf("client send list tools req failed: %w", err)
		}
		log.Printf("Client simulator: Sent ListToolsRequest (ID: %s)", listToolsReqID)

		listToolsResp, err := receiveResponse(t, clientConn, listToolsReqID, 5*time.Second)
		if err != nil {
			return fmt.Errorf("client recv list tools resp failed: %w", err)
		}
		var listToolsResult gomcp.ListToolsResult
		if listToolsResp.Result == nil {
			return fmt.Errorf("client received null result for ListTools")
		}
		if err := gomcp.UnmarshalPayload(listToolsResp.Result, &listToolsResult); err != nil {
			return fmt.Errorf("client failed to unmarshal ListToolsResult: %w", err)
		}
		log.Printf("Client simulator: Received ListToolsResponse with %d tools", len(listToolsResult.Tools))
		if len(listToolsResult.Tools) != 3 { // echo, calculator, filesystem
			return fmt.Errorf("expected 3 tools, got %d", len(listToolsResult.Tools))
		}

		// 4. Send CallToolRequest (echo), wait for CallToolResponse
		log.Println("Client simulator: Sending CallToolRequest (echo)...")
		echoMsg := "hello server"
		echoArgs := map[string]interface{}{"message": echoMsg}
		echoReqParams := gomcp.CallToolParams{Name: "echo", Arguments: echoArgs}
		// Use the correct SendRequest signature
		echoReqID, err := clientConn.SendRequest(gomcp.MethodCallTool, echoReqParams)
		if err != nil {
			return fmt.Errorf("client send call tool(echo) req failed: %w", err)
		}
		log.Printf("Client simulator: Sent CallToolRequest(echo) (ID: %s)", echoReqID)

		echoResp, err := receiveResponse(t, clientConn, echoReqID, 5*time.Second)
		if err != nil {
			return fmt.Errorf("client recv call tool(echo) resp failed: %w", err)
		}
		var echoResult gomcp.CallToolResult
		if echoResp.Result == nil {
			return fmt.Errorf("client received null result for CallTool(echo)")
		}
		// Use the custom UnmarshalJSON for CallToolResult by unmarshalling the result field
		resultBytesEcho, err := json.Marshal(echoResp.Result) // Re-marshal the result part
		if err != nil {
			return fmt.Errorf("client failed to re-marshal CallToolResult(echo): %w", err)
		}
		if err := json.Unmarshal(resultBytesEcho, &echoResult); err != nil { // Unmarshal into CallToolResult using its custom method
			return fmt.Errorf("client failed to unmarshal CallToolResult(echo) using custom unmarshaller: %w", err)
		}

		if echoResult.IsError != nil && *echoResult.IsError {
			return fmt.Errorf("echo tool reported an error: %+v", echoResult.Content)
		}
		if len(echoResult.Content) != 1 {
			return fmt.Errorf("echo tool expected 1 content item, got %d", len(echoResult.Content))
		}
		// Check the concrete type after custom unmarshalling
		if textContent, ok := echoResult.Content[0].(gomcp.TextContent); !ok || textContent.Text != echoMsg {
			return fmt.Errorf("echo tool result mismatch: expected '%s', got '%+v' (type %T)", echoMsg, echoResult.Content[0], echoResult.Content[0])
		}
		log.Printf("Client simulator: Received CallToolResponse(echo) successfully.")

		// 5. Send CallToolRequest (calculator add), wait for CallToolResponse
		log.Println("Client simulator: Sending CallToolRequest (calculator add)...")
		calcArgs1 := map[string]interface{}{"operand1": 5.0, "operand2": 7.0, "operation": "add"}
		calcReqParams1 := gomcp.CallToolParams{Name: "calculator", Arguments: calcArgs1}
		// Use the correct SendRequest signature
		calcReqID1, err := clientConn.SendRequest(gomcp.MethodCallTool, calcReqParams1)
		if err != nil {
			return fmt.Errorf("client send calc(add) req failed: %w", err)
		}
		calcResp1, err := receiveResponse(t, clientConn, calcReqID1, 5*time.Second)
		if err != nil {
			return fmt.Errorf("client recv calc(add) resp failed: %w", err)
		}
		var calcResult1 gomcp.CallToolResult
		if calcResp1.Result == nil {
			return fmt.Errorf("client received null result for CallTool(add)")
		}
		// Use custom unmarshaller
		resultBytesCalc1, _ := json.Marshal(calcResp1.Result)
		if err := json.Unmarshal(resultBytesCalc1, &calcResult1); err != nil {
			return fmt.Errorf("client failed to unmarshal CallToolResult(add): %w", err)
		}
		if calcResult1.IsError != nil && *calcResult1.IsError {
			return fmt.Errorf("calc(add) tool reported an error: %+v", calcResult1.Content)
		}
		// Further validation of calcResult1.Content if needed...
		log.Printf("Client simulator: Received CallToolResponse(add) successfully.")

		// 6. Send CallToolRequest (calculator divide by zero), wait for CallToolResponse (expecting tool error)
		log.Println("Client simulator: Sending CallToolRequest (calculator div0)...")
		calcArgs2 := map[string]interface{}{"operand1": 10.0, "operand2": 0.0, "operation": "divide"}
		calcReqParams2 := gomcp.CallToolParams{Name: "calculator", Arguments: calcArgs2}
		// Use the correct SendRequest signature
		calcReqID2, err := clientConn.SendRequest(gomcp.MethodCallTool, calcReqParams2)
		if err != nil {
			return fmt.Errorf("client send calc(div0) req failed: %w", err)
		}
		calcResp2, err := receiveResponse(t, clientConn, calcReqID2, 5*time.Second)
		if err != nil {
			return fmt.Errorf("client recv calc(div0) resp failed: %w", err)
		}
		var calcResult2 gomcp.CallToolResult
		if calcResp2.Result == nil {
			return fmt.Errorf("client received null result for CallTool(div0)")
		}
		// Use custom unmarshaller
		resultBytesCalc2, _ := json.Marshal(calcResp2.Result)
		if err := json.Unmarshal(resultBytesCalc2, &calcResult2); err != nil {
			return fmt.Errorf("client failed to unmarshal CallToolResult(div0): %w", err)
		}
		if calcResult2.IsError == nil || !*calcResult2.IsError {
			return fmt.Errorf("calc(div0) tool should have reported an error, but didn't")
		}
		log.Printf("Client simulator: Received CallToolResponse(div0) with expected error flag.")

		// 7. Send CallToolRequest (calculator missing arg), wait for CallToolResponse (expecting tool error)
		log.Println("Client simulator: Sending CallToolRequest (calculator missing arg)...")
		calcArgs3 := map[string]interface{}{"operand1": 10.0, "operation": "multiply"} // Missing operand2
		calcReqParams3 := gomcp.CallToolParams{Name: "calculator", Arguments: calcArgs3}
		// Use the correct SendRequest signature
		calcReqID3, err := clientConn.SendRequest(gomcp.MethodCallTool, calcReqParams3)
		if err != nil {
			return fmt.Errorf("client send calc(missing arg) req failed: %w", err)
		}
		calcResp3, err := receiveResponse(t, clientConn, calcReqID3, 5*time.Second)
		if err != nil {
			return fmt.Errorf("client recv calc(missing arg) resp failed: %w", err)
		}
		var calcResult3 gomcp.CallToolResult
		if calcResp3.Result == nil {
			return fmt.Errorf("client received null result for CallTool(missing arg)")
		}
		// Use custom unmarshaller
		resultBytesCalc3, _ := json.Marshal(calcResp3.Result)
		if err := json.Unmarshal(resultBytesCalc3, &calcResult3); err != nil {
			return fmt.Errorf("client failed to unmarshal CallToolResult(missing arg): %w", err)
		}
		if calcResult3.IsError == nil || !*calcResult3.IsError {
			return fmt.Errorf("calc(missing arg) tool should have reported an error, but didn't")
		}
		log.Printf("Client simulator: Received CallToolResponse(missing arg) with expected error flag.")

		// 8. Send CallToolRequest (filesystem list), wait for CallToolResponse
		log.Println("Client simulator: Sending CallToolRequest (fs list)...")
		fsArgsList := map[string]interface{}{"operation": "list_files", "path": "."}
		fsListParams := gomcp.CallToolParams{Name: "filesystem", Arguments: fsArgsList}
		// Use the correct SendRequest signature
		fsListReqID, err := clientConn.SendRequest(gomcp.MethodCallTool, fsListParams)
		if err != nil {
			return fmt.Errorf("client send fs(list) req failed: %w", err)
		}
		fsListResp, err := receiveResponse(t, clientConn, fsListReqID, 5*time.Second)
		if err != nil {
			return fmt.Errorf("client recv fs(list) resp failed: %w", err)
		}
		var fsListResult gomcp.CallToolResult
		if fsListResp.Result == nil {
			return fmt.Errorf("client received null result for CallTool(fs list)")
		}
		// Use custom unmarshaller
		resultBytesFsList, _ := json.Marshal(fsListResp.Result)
		if err := json.Unmarshal(resultBytesFsList, &fsListResult); err != nil {
			return fmt.Errorf("client failed to unmarshal CallToolResult(fs list): %w", err)
		}
		if fsListResult.IsError != nil && *fsListResult.IsError {
			return fmt.Errorf("fs(list) tool reported an error: %+v", fsListResult.Content)
		}
		log.Printf("Client simulator: Received CallToolResponse(fs list) successfully.")

		// 9. Send CallToolRequest (filesystem write), wait for CallToolResponse
		log.Println("Client simulator: Sending CallToolRequest (fs write)...")
		fsToolName := "filesystem"
		testFilePath := "test_dir/my_file.txt" // Relative to sandbox
		testFileContent := "This is the content of the test file.\nIt has multiple lines."
		fsArgsWrite := map[string]interface{}{"operation": "write_file", "path": testFilePath, "content": testFileContent}
		fsWriteParams := gomcp.CallToolParams{Name: fsToolName, Arguments: fsArgsWrite}
		// Use the correct SendRequest signature
		fsWriteReqID, err := clientConn.SendRequest(gomcp.MethodCallTool, fsWriteParams)
		if err != nil {
			return fmt.Errorf("client send fs(write) req failed: %w", err)
		}
		fsWriteResp, err := receiveResponse(t, clientConn, fsWriteReqID, 5*time.Second)
		if err != nil {
			return fmt.Errorf("client recv fs(write) resp failed: %w", err)
		}
		var fsWriteResult gomcp.CallToolResult
		if fsWriteResp.Result == nil {
			return fmt.Errorf("client received null result for CallTool(fs write)")
		}
		// Use custom unmarshaller
		resultBytesFsWrite, _ := json.Marshal(fsWriteResp.Result)
		if err := json.Unmarshal(resultBytesFsWrite, &fsWriteResult); err != nil {
			return fmt.Errorf("client failed to unmarshal CallToolResult(fs write): %w", err)
		}
		if fsWriteResult.IsError != nil && *fsWriteResult.IsError {
			return fmt.Errorf("fs(write) tool reported an error: %+v", fsWriteResult.Content)
		}
		log.Printf("Client simulator: Received CallToolResponse(fs write) successfully.")

		// 10. Send CallToolRequest (filesystem read), wait for CallToolResponse
		log.Println("Client simulator: Sending CallToolRequest (fs read)...")
		fsArgsRead := map[string]interface{}{"operation": "read_file", "path": testFilePath}
		fsReadParams := gomcp.CallToolParams{Name: fsToolName, Arguments: fsArgsRead}
		// Use the correct SendRequest signature
		fsReadReqID, err := clientConn.SendRequest(gomcp.MethodCallTool, fsReadParams)
		if err != nil {
			return fmt.Errorf("client send fs(read) req failed: %w", err)
		}
		fsReadResp, err := receiveResponse(t, clientConn, fsReadReqID, 5*time.Second)
		if err != nil {
			return fmt.Errorf("client recv fs(read) resp failed: %w", err)
		}
		var fsReadResult gomcp.CallToolResult
		if fsReadResp.Result == nil {
			return fmt.Errorf("client received null result for CallTool(fs read)")
		}
		// Use the custom UnmarshalJSON for CallToolResult
		tempReadResultBytes, _ := json.Marshal(fsReadResp.Result)
		var tempReadResult gomcp.CallToolResult
		if err := json.Unmarshal(tempReadResultBytes, &tempReadResult); err != nil {
			return fmt.Errorf("client failed to re-unmarshal CallToolResult(fs read): %w", err)
		}
		fsReadResult = tempReadResult // Assign the correctly unmarshalled result

		if fsReadResult.IsError != nil && *fsReadResult.IsError {
			return fmt.Errorf("fs(read) tool reported an error: %+v", fsReadResult.Content)
		}
		if len(fsReadResult.Content) != 1 {
			return fmt.Errorf("fs(read) tool expected 1 content item, got %d", len(fsReadResult.Content))
		}
		// Check the concrete type after custom unmarshalling
		if textContent, ok := fsReadResult.Content[0].(gomcp.TextContent); !ok || textContent.Text != testFileContent {
			return fmt.Errorf("fs(read) content mismatch: expected %q, got '%+v' (type %T)", testFileContent, fsReadResult.Content[0], fsReadResult.Content[0])
		}
		log.Printf("Client simulator: Received CallToolResponse(fs read) successfully.")

		// 11. Send CallToolRequest (filesystem list dir), wait for CallToolResponse
		log.Println("Client simulator: Sending CallToolRequest (fs list dir)...")
		fsArgsListDir := map[string]interface{}{"operation": "list_files", "path": "test_dir"}
		fsListDirParams := gomcp.CallToolParams{Name: fsToolName, Arguments: fsArgsListDir}
		// Use the correct SendRequest signature
		fsListDirReqID, err := clientConn.SendRequest(gomcp.MethodCallTool, fsListDirParams)
		if err != nil {
			return fmt.Errorf("client send fs(list dir) req failed: %w", err)
		}
		fsListDirResp, err := receiveResponse(t, clientConn, fsListDirReqID, 5*time.Second)
		if err != nil {
			return fmt.Errorf("client recv fs(list dir) resp failed: %w", err)
		}
		var fsListDirResult gomcp.CallToolResult
		if fsListDirResp.Result == nil {
			return fmt.Errorf("client received null result for CallTool(fs list dir)")
		}
		// Use the custom UnmarshalJSON for CallToolResult
		tempListDirResultBytes, _ := json.Marshal(fsListDirResp.Result)
		var tempListDirResult gomcp.CallToolResult
		if err := json.Unmarshal(tempListDirResultBytes, &tempListDirResult); err != nil {
			return fmt.Errorf("client failed to re-unmarshal CallToolResult(fs list dir): %w", err)
		}
		fsListDirResult = tempListDirResult // Assign the correctly unmarshalled result

		if fsListDirResult.IsError != nil && *fsListDirResult.IsError {
			return fmt.Errorf("fs(list dir) tool reported an error: %+v", fsListDirResult.Content)
		}
		// Basic check: expect text content containing the filename
		if len(fsListDirResult.Content) != 1 {
			return fmt.Errorf("fs(list dir) tool expected 1 content item, got %d", len(fsListDirResult.Content))
		}
		var listTextContent gomcp.TextContent
		listBytes, _ := json.Marshal(fsListDirResult.Content[0])
		if err := json.Unmarshal(listBytes, &listTextContent); err != nil || listTextContent.Type != "text" {
			return fmt.Errorf("fs(list dir) result content[0] was not TextContent: %T, err: %v", fsListDirResult.Content[0], err)
		}
		if !strings.Contains(listTextContent.Text, "my_file.txt") {
			return fmt.Errorf("fs(list dir) result did not contain expected file 'my_file.txt': %s", listTextContent.Text)
		}
		log.Printf("Client simulator: Received CallToolResponse(fs list dir) successfully.")

		// 12. Send CallToolRequest (filesystem read non-existent), wait for CallToolResponse (expecting tool error)
		log.Println("Client simulator: Sending CallToolRequest (fs read non-existent)...")
		fsArgsReadNF := map[string]interface{}{"operation": "read_file", "path": "non_existent_file.txt"}
		fsReadNFParams := gomcp.CallToolParams{Name: fsToolName, Arguments: fsArgsReadNF}
		// Use the correct SendRequest signature
		fsReadNFReqID, err := clientConn.SendRequest(gomcp.MethodCallTool, fsReadNFParams)
		if err != nil {
			return fmt.Errorf("client send fs(read nf) req failed: %w", err)
		}
		fsReadNFResp, err := receiveResponse(t, clientConn, fsReadNFReqID, 5*time.Second)
		if err != nil {
			return fmt.Errorf("client recv fs(read nf) resp failed: %w", err)
		}
		var fsReadNFResult gomcp.CallToolResult
		if fsReadNFResp.Result == nil {
			return fmt.Errorf("client received null result for CallTool(fs read nf)")
		}
		// Use custom unmarshaller
		resultBytesFsReadNF, _ := json.Marshal(fsReadNFResp.Result)
		if err := json.Unmarshal(resultBytesFsReadNF, &fsReadNFResult); err != nil {
			return fmt.Errorf("client failed to unmarshal CallToolResult(fs read nf): %w", err)
		}
		if fsReadNFResult.IsError == nil || !*fsReadNFResult.IsError {
			return fmt.Errorf("fs(read non-existent) tool should have reported an error, but didn't")
		}
		log.Printf("Client simulator: Received CallToolResponse(fs read non-existent) with expected error flag.")

		// 13. Send CallToolRequest (filesystem write outside), wait for CallToolResponse (expecting tool error)
		log.Println("Client simulator: Sending CallToolRequest (fs write outside)...")
		fsArgsWriteOutside := map[string]interface{}{"operation": "write_file", "path": "../outside_sandbox.txt", "content": "attempt escape"}
		fsWriteOutsideParams := gomcp.CallToolParams{Name: fsToolName, Arguments: fsArgsWriteOutside}
		// Use the correct SendRequest signature
		fsWriteOutsideReqID, err := clientConn.SendRequest(gomcp.MethodCallTool, fsWriteOutsideParams)
		if err != nil {
			return fmt.Errorf("client send fs(write outside) req failed: %w", err)
		}
		fsWriteOutsideResp, err := receiveResponse(t, clientConn, fsWriteOutsideReqID, 5*time.Second)
		if err != nil {
			return fmt.Errorf("client recv fs(write outside) resp failed: %w", err)
		}
		var fsWriteOutsideResult gomcp.CallToolResult
		if fsWriteOutsideResp.Result == nil {
			return fmt.Errorf("client received null result for CallTool(fs write outside)")
		}
		// Use custom unmarshaller
		resultBytesFsWriteOutside, _ := json.Marshal(fsWriteOutsideResp.Result)
		if err := json.Unmarshal(resultBytesFsWriteOutside, &fsWriteOutsideResult); err != nil {
			return fmt.Errorf("client failed to unmarshal CallToolResult(fs write outside): %w", err)
		}
		if fsWriteOutsideResult.IsError == nil || !*fsWriteOutsideResult.IsError {
			return fmt.Errorf("fs(write outside) tool should have reported an error, but didn't")
		}
		log.Printf("Client simulator: Received CallToolResponse(fs write outside) with expected error flag.")

		// 14. Send Ping request, wait for Ping response
		log.Println("Client simulator: Sending PingRequest...")
		// Use the correct SendRequest signature
		pingReqID, err := clientConn.SendRequest(gomcp.MethodPing, nil) // Ping has nil params
		if err != nil {
			return fmt.Errorf("client send ping req failed: %w", err)
		}
		log.Printf("Client simulator: Sent PingRequest (ID: %s)", pingReqID)

		pingResp, err := receiveResponse(t, clientConn, pingReqID, 5*time.Second)
		if err != nil {
			return fmt.Errorf("client recv ping resp failed: %w", err)
		}
		if pingResp.Result != nil { // Ping result should be null
			return fmt.Errorf("ping response result should be null, got: %+v", pingResp.Result)
		}
		log.Printf("Client simulator: Received PingResponse successfully.")

		// Client simulation done
		log.Println("Client simulator finished.")
		return nil
	}() // Execute the client simulation immediately

	// Wait for server to finish (should happen after client closes pipe)
	serverWg.Wait()

	// Assert results
	if clientErr != nil {
		t.Errorf("Client simulation failed: %v", clientErr)
	}
	// Server should exit cleanly with nil error or EOF/pipe error after client disconnects
	if serverErr != nil && !errors.Is(serverErr, io.EOF) && !strings.Contains(serverErr.Error(), "pipe") && !strings.Contains(serverErr.Error(), "closed") {
		t.Errorf("Server logic failed unexpectedly: %v", serverErr)
	} else if serverErr != nil {
		t.Logf("Server exited with expected EOF/pipe error: %v", serverErr)
	} else {
		t.Log("Server exited cleanly (nil error).")
	}
}

// Helper function to get a pointer to a boolean value.
// Needed because IsError is a *bool
func BoolPtr(b bool) *bool {
	return &b
}

// Helper function to get a pointer to a string value.
// Needed for optional Annotations fields
func StringPtr(s string) *string {
	return &s
}
