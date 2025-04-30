package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types"
)

// --- Test Setup ---

// createTestConnections creates a pair of connected stdio transports using pipes.
func createTestConnections() (types.Transport, types.Transport) {
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	serverConn := stdio.NewStdioTransportWithReadWriter(serverReader, serverWriter, types.TransportOptions{})
	clientConn := stdio.NewStdioTransportWithReadWriter(clientReader, clientWriter, types.TransportOptions{})
	// Note: Closing the WriteCloser end of the pipe is needed to signal EOF reliably.
	// This function signature might need adjustment if the caller needs the writer.
	_ = clientWriter // Avoid unused variable if not returned
	return serverConn, clientConn
}

// Helper to receive and decode a JSON-RPC response within a timeout
func receiveResponseAsync(ctx context.Context, t *testing.T, transport types.Transport, expectedID interface{}) (*protocol.JSONRPCResponse, error) {
	t.Helper()
	respChan := make(chan *protocol.JSONRPCResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		// Use the updated Receive method from the types.Transport interface
		raw, err := transport.Receive(ctx)
		if err != nil {
			errChan <- err
			return
		}
		var resp protocol.JSONRPCResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			errChan <- fmt.Errorf("failed to unmarshal valid JSON response: %w. Raw: %s", err, string(raw))
			return
		}
		if expectedID != nil && fmt.Sprintf("%v", resp.ID) != fmt.Sprintf("%v", expectedID) {
			errChan <- fmt.Errorf("response ID mismatch. Expected: %v, Got: %v", expectedID, resp.ID)
			return
		}
		if resp.Error != nil {
			errChan <- fmt.Errorf("received MCP Error: [%d] %s", resp.Error.Code, resp.Error.Message)
			return
		}
		respChan <- &resp
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout waiting for response (Expected ID: %v): %w", expectedID, ctx.Err())
	case err := <-errChan:
		if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("connection closed or context done while waiting for response (Expected ID: %v): %w", expectedID, err)
		}
		return nil, err
	case resp := <-respChan:
		return resp, nil
	}
}

// --- Dummy Handlers for Server Simulation ---
// These remain as they might be useful for future, more focused tests.

var testEchoTool = protocol.Tool{
	Name:        "echo",
	Description: "Test Echo",
	InputSchema: protocol.ToolInputSchema{Type: "object", Properties: map[string]protocol.PropertyDetail{"message": {Type: "string"}}, Required: []string{"message"}},
}

func testEchoHandler(ctx context.Context, pt interface{}, args any) ([]protocol.Content, bool) {
	// pt is now interface{}, but not used here
	msg, _ := args.(map[string]interface{})["message"].(string)
	return []protocol.Content{protocol.TextContent{Type: "text", Text: msg}}, false
}

var testCalculatorTool = protocol.Tool{
	Name:        "calculator",
	Description: "Test Calc",
	InputSchema: protocol.ToolInputSchema{
		Type: "object",
		Properties: map[string]protocol.PropertyDetail{
			"operand1":  {Type: "number"},
			"operand2":  {Type: "number"},
			"operation": {Type: "string", Enum: []interface{}{"add", "subtract", "multiply", "divide"}},
		},
		Required: []string{"operand1", "operand2", "operation"},
	},
}

func testCalculatorHandler(ctx context.Context, pt interface{}, args any) ([]protocol.Content, bool) {
	// pt is now interface{}, but not used here
	op1, ok1 := args.(map[string]interface{})["operand1"].(float64)
	op2, ok2 := args.(map[string]interface{})["operand2"].(float64)
	opStr, ok3 := args.(map[string]interface{})["operation"].(string)
	if !ok1 || !ok2 || !ok3 {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Missing required arguments"}}, true
	}
	var result float64
	isErrTrue := true
	switch opStr {
	case "add":
		result = op1 + op2
	case "subtract":
		result = op1 - op2
	case "multiply":
		result = op1 * op2
	case "divide":
		if op2 == 0 {
			return []protocol.Content{protocol.TextContent{Type: "text", Text: "Division by zero"}}, isErrTrue
		}
		result = op1 / op2
	default:
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid operation"}}, isErrTrue
	}
	return []protocol.Content{protocol.TextContent{Type: "text", Text: fmt.Sprintf("%f", result)}}, false
}

const testFileSystemSandbox = "./fs_sandbox_test_server"

var testFileSystemTool = protocol.Tool{
	Name:        "filesystem",
	Description: fmt.Sprintf("Test FS Tool (Sandbox: %s)", testFileSystemSandbox),
	InputSchema: protocol.ToolInputSchema{
		Type: "object",
		Properties: map[string]protocol.PropertyDetail{
			"operation": {Type: "string", Enum: []interface{}{"list_files", "read_file", "write_file"}},
			"path":      {Type: "string"},
			"content":   {Type: "string"},
		},
		Required: []string{"operation", "path"},
	},
}

func testFilesystemHandler(ctx context.Context, pt interface{}, args any) ([]protocol.Content, bool) {
	// pt is now interface{}, but not used here
	op, _ := args.(map[string]interface{})["operation"].(string)
	relativePath, okPath := args.(map[string]interface{})["path"].(string)
	if !okPath {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Missing path argument"}}, true
	}

	if strings.Contains(relativePath, "..") || filepath.IsAbs(relativePath) {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid path: contains '..' or is absolute"}}, true
	}
	safePath := filepath.Join(testFileSystemSandbox, relativePath)
	absSandbox, _ := filepath.Abs(testFileSystemSandbox)
	absSafePath, _ := filepath.Abs(safePath)
	if !strings.HasPrefix(absSafePath, absSandbox) {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: fmt.Sprintf("Path '%s' attempts to escape the sandbox directory '%s'", relativePath, testFileSystemSandbox)}}, true
	}

	isErrTrue := true

	switch op {
	case "list_files":
		files, err := os.ReadDir(safePath)
		if err != nil {
			if os.IsNotExist(err) {
				resultMap := map[string]interface{}{"files": []interface{}{}}
				resultBytes, _ := json.Marshal(resultMap)
				return []protocol.Content{protocol.TextContent{Type: "text", Text: string(resultBytes)}}, false
			}
			return []protocol.Content{protocol.TextContent{Type: "text", Text: fmt.Sprintf("Failed to list files: %v", err)}}, isErrTrue
		}
		var fileInfos []map[string]interface{}
		for _, file := range files {
			info, errInfo := file.Info()
			if errInfo == nil {
				fileInfos = append(fileInfos, map[string]interface{}{
					"name":     info.Name(),
					"is_dir":   info.IsDir(),
					"size":     info.Size(),
					"mod_time": info.ModTime().Format(time.RFC3339),
				})
			}
		}
		resultMap := map[string]interface{}{"files": fileInfos}
		resultBytes, _ := json.Marshal(resultMap)
		return []protocol.Content{protocol.TextContent{Type: "text", Text: string(resultBytes)}}, false

	case "write_file":
		contentToWrite, okContent := args.(map[string]interface{})["content"].(string)
		if !okContent {
			return []protocol.Content{protocol.TextContent{Type: "text", Text: "Missing content argument for write_file"}}, isErrTrue
		}
		parentDir := filepath.Dir(safePath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return []protocol.Content{protocol.TextContent{Type: "text", Text: fmt.Sprintf("Failed to create parent dir: %v", err)}}, isErrTrue
		}
		err := os.WriteFile(safePath, []byte(contentToWrite), 0644)
		if err != nil {
			return []protocol.Content{protocol.TextContent{Type: "text", Text: fmt.Sprintf("Failed to write file: %v", err)}}, isErrTrue
		}
		resultMap := map[string]interface{}{"status": "success", "message": fmt.Sprintf("Successfully wrote %d bytes to '%s'", len(contentToWrite), relativePath)}
		resultBytes, _ := json.Marshal(resultMap)
		return []protocol.Content{protocol.TextContent{Type: "text", Text: string(resultBytes)}}, false

	case "read_file":
		contentBytes, err := os.ReadFile(safePath)
		if err != nil {
			if os.IsNotExist(err) {
				return []protocol.Content{protocol.TextContent{Type: "text", Text: fmt.Sprintf("File not found at path '%s'", relativePath)}}, isErrTrue
			}
			return []protocol.Content{protocol.TextContent{Type: "text", Text: fmt.Sprintf("Failed to read file: %v", err)}}, isErrTrue
		}
		return []protocol.Content{protocol.TextContent{Type: "text", Text: string(contentBytes)}}, false

	default:
		return []protocol.Content{protocol.TextContent{Type: "text", Text: fmt.Sprintf("Invalid operation '%s'", op)}}, isErrTrue
	}
}

// --- Test Function ---

// TestExampleServerLogic runs the server logic using the refactored server.Server
// and simulates a basic client interaction using the stdio transport directly.
func TestExampleServerLogic(t *testing.T) {
	// Create a temporary directory for filesystem tests
	err := os.MkdirAll(testFileSystemSandbox, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testFileSystemSandbox)

	// Set up the test connections
	serverConn, clientConn := createTestConnections()
	defer serverConn.Close()
	defer clientConn.Close()

	// Set up a context with timeout for the entire test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start server in a goroutine
	serverErrChan := make(chan error, 1)
	srv := server.NewServer("TestServer")

	// Register test tools directly
	err = srv.RegisterTool(testEchoTool, func(ctx context.Context, pt interface{}, args any) ([]protocol.Content, bool) {
		return testEchoHandler(ctx, pt, args)
	})
	if err != nil {
		t.Fatalf("Failed to register echo tool: %v", err)
	}

	err = srv.RegisterTool(testCalculatorTool, func(ctx context.Context, pt interface{}, args any) ([]protocol.Content, bool) {
		return testCalculatorHandler(ctx, pt, args)
	})
	if err != nil {
		t.Fatalf("Failed to register calculator tool: %v", err)
	}

	err = srv.RegisterTool(testFileSystemTool, func(ctx context.Context, pt interface{}, args any) ([]protocol.Content, bool) {
		return testFilesystemHandler(ctx, pt, args)
	})
	if err != nil {
		t.Fatalf("Failed to register filesystem tool: %v", err)
	}

	// Create and register the test session
	testSession := &testSession{
		id:        "test-session",
		transport: serverConn,
	}
	err = srv.RegisterSession(testSession)
	if err != nil {
		t.Fatalf("Failed to register session: %v", err)
	}

	// Skip setting up a full server with ServeStdio, and instead handle the communications directly
	go func() {
		// Handle messages in a loop
		for {
			select {
			case <-ctx.Done():
				serverErrChan <- ctx.Err()
				return
			default:
				raw, err := serverConn.Receive(ctx)
				if err != nil {
					if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
						serverErrChan <- nil // Clean shutdown
						return
					}
					serverErrChan <- fmt.Errorf("error receiving message: %v", err)
					return
				}

				// Parse the message to determine if it's a request
				var baseMsg struct {
					JSONRPC string      `json:"jsonrpc"`
					ID      interface{} `json:"id,omitempty"`
					Method  string      `json:"method"`
				}
				if err := json.Unmarshal(raw, &baseMsg); err != nil {
					serverErrChan <- fmt.Errorf("failed to parse incoming message: %v", err)
					continue
				}

				// For 'initialize' method, respond with success manually since we're bypassing normal server setup
				if baseMsg.Method == "initialize" {
					// Create a successful initialize response
					initResult := protocol.InitializeResult{
						ServerInfo: protocol.Implementation{
							Name:    "TestServer",
							Version: "1.0.0",
						},
						Capabilities: protocol.ServerCapabilities{
							Tools: &struct {
								ListChanged bool `json:"listChanged,omitempty"`
							}{
								ListChanged: true,
							},
						},
					}

					// Send the successful response
					resp := protocol.JSONRPCResponse{
						JSONRPC: "2.0",
						ID:      baseMsg.ID,
						Result:  initResult,
					}

					respBytes, _ := json.Marshal(resp)
					if err := serverConn.Send(ctx, respBytes); err != nil {
						serverErrChan <- fmt.Errorf("error sending initialize response: %v", err)
						return
					}

					// If the method is 'initialized', just ignore it (it's a notification)
				} else if baseMsg.Method == "initialized" {
					// No response needed for notifications
				} else {
					// Use server.HandleMessage for all other messages
					responses := srv.HandleMessage(ctx, "test-session", raw)
					for _, resp := range responses {
						if resp == nil {
							continue
						}
						respBytes, err := json.Marshal(resp)
						if err != nil {
							serverErrChan <- fmt.Errorf("error marshaling response: %v", err)
							continue
						}
						if err := serverConn.Send(ctx, respBytes); err != nil {
							serverErrChan <- fmt.Errorf("error sending response: %v", err)
							return
						}
					}
				}
			}
		}
	}()

	// --- Client Side Interactions ---

	// Step 1: Initialize
	initReq := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"clientName": "TestClient",
			"version":    "1.0.0",
		},
	}

	reqBytes, err := json.Marshal(initReq)
	if err != nil {
		t.Fatalf("Failed to marshal initialize request: %v", err)
	}

	err = clientConn.Send(ctx, reqBytes)
	if err != nil {
		t.Fatalf("Failed to send initialize request: %v", err)
	}

	initResp, err := receiveResponseAsync(ctx, t, clientConn, 1)
	if err != nil {
		t.Fatalf("Failed to receive initialize response: %v", err)
	}

	// Verify initialize response was successful
	if initResp.Error != nil {
		t.Fatalf("Initialize request failed: %v", initResp.Error)
	}

	// Step 2: Test Echo Tool
	echoReq := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  protocol.MethodCallTool,
		Params: map[string]interface{}{
			"name": "echo",
			"arguments": map[string]interface{}{
				"message": "Hello, MCP Test!",
			},
		},
	}

	reqBytes, err = json.Marshal(echoReq)
	if err != nil {
		t.Fatalf("Failed to marshal echo request: %v", err)
	}

	err = clientConn.Send(ctx, reqBytes)
	if err != nil {
		t.Fatalf("Failed to send echo request: %v", err)
	}

	echoResp, err := receiveResponseAsync(ctx, t, clientConn, 2)
	if err != nil {
		t.Fatalf("Failed to receive echo response: %v", err)
	}

	// Verify echo response
	var contentArray []map[string]interface{}

	// Try to parse the response result correctly
	switch result := echoResp.Result.(type) {
	case json.RawMessage:
		// Try to parse as a response with content array field
		var respWithContent struct {
			Content []map[string]interface{} `json:"content"`
		}
		if err := json.Unmarshal(result, &respWithContent); err != nil {
			t.Fatalf("Failed to unmarshal echo result: %v", err)
		}
		contentArray = respWithContent.Content
	case map[string]interface{}:
		// Check if the map has a content field
		if contentField, ok := result["content"]; ok {
			if contentArr, ok := contentField.([]interface{}); ok {
				for _, item := range contentArr {
					if contentMap, ok := item.(map[string]interface{}); ok {
						contentArray = append(contentArray, contentMap)
					}
				}
			}
		}
	default:
		t.Fatalf("Unexpected result type: %T", echoResp.Result)
	}

	if len(contentArray) != 1 || contentArray[0]["type"] != "text" || contentArray[0]["text"] != "Hello, MCP Test!" {
		t.Fatalf("Unexpected echo result: %v", contentArray)
	}

	// Step 3: Test Calculator Tool
	calcReq := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  protocol.MethodCallTool,
		Params: map[string]interface{}{
			"name": "calculator",
			"arguments": map[string]interface{}{
				"operand1":  10.0,
				"operand2":  5.0,
				"operation": "multiply",
			},
		},
	}

	reqBytes, err = json.Marshal(calcReq)
	if err != nil {
		t.Fatalf("Failed to marshal calculator request: %v", err)
	}

	err = clientConn.Send(ctx, reqBytes)
	if err != nil {
		t.Fatalf("Failed to send calculator request: %v", err)
	}

	calcResp, err := receiveResponseAsync(ctx, t, clientConn, 3)
	if err != nil {
		t.Fatalf("Failed to receive calculator response: %v", err)
	}

	// Verify calculator response
	var calcContentArray []map[string]interface{}

	// Parse the response
	switch result := calcResp.Result.(type) {
	case json.RawMessage:
		var respWithContent struct {
			Content []map[string]interface{} `json:"content"`
		}
		if err := json.Unmarshal(result, &respWithContent); err != nil {
			t.Fatalf("Failed to unmarshal calculator result: %v", err)
		}
		calcContentArray = respWithContent.Content
	case map[string]interface{}:
		if contentField, ok := result["content"]; ok {
			if contentArr, ok := contentField.([]interface{}); ok {
				for _, item := range contentArr {
					if contentMap, ok := item.(map[string]interface{}); ok {
						calcContentArray = append(calcContentArray, contentMap)
					}
				}
			}
		}
	default:
		t.Fatalf("Unexpected calculator result type: %T", calcResp.Result)
	}

	// The result should be 50.0
	if len(calcContentArray) != 1 || calcContentArray[0]["type"] != "text" {
		t.Fatalf("Unexpected calculator result structure: %v", calcContentArray)
	}

	resultText, ok := calcContentArray[0]["text"].(string)
	if !ok {
		t.Fatalf("Calculator result text is not a string: %v", calcContentArray[0]["text"])
	}

	// The text should contain "50" (could be "50.000000" depending on formatting)
	if !strings.Contains(resultText, "50") {
		t.Fatalf("Calculator result doesn't contain expected value '50': %s", resultText)
	}

	// Cancel context to stop the server goroutine
	cancel()

	// Wait for server to exit
	select {
	case err := <-serverErrChan:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("Server exited with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Server did not exit cleanly")
	}
}

// testSession implements a minimal session for testing
type testSession struct {
	id        string
	transport types.Transport
}

func (s *testSession) SessionID() string { return s.id }
func (s *testSession) SendNotification(notification protocol.JSONRPCNotification) error {
	msg, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	return s.transport.Send(context.Background(), msg)
}
func (s *testSession) SendResponse(response protocol.JSONRPCResponse) error {
	msg, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return s.transport.Send(context.Background(), msg)
}
func (s *testSession) Close() error                                             { return nil }
func (s *testSession) Initialize()                                              {}
func (s *testSession) Initialized() bool                                        { return true }
func (s *testSession) StoreClientCapabilities(caps protocol.ClientCapabilities) {}
func (s *testSession) GetClientCapabilities() protocol.ClientCapabilities {
	return protocol.ClientCapabilities{}
}
func (s *testSession) SetNegotiatedVersion(version string) {}
func (s *testSession) GetNegotiatedVersion() string        { return "1.0" }

// Helper function to get a pointer to a boolean value.
func BoolPtr(b bool) *bool { return &b }

// Helper function to get a pointer to a string value.
func StringPtr(s string) *string { return &s }
