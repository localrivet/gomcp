// examples/server/main_test.go (Refactored)
package main

import (
	"fmt" // Needed for test setup error formatting
	"io"
	"log"
	"os"
	"strings" // Needed for error check
	"sync"
	"testing"

	// "time" // No longer needed directly in test

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

// TestExampleServerLogic runs the server logic using the refactored mcp.Server
// and simulates a basic client interaction.
func TestExampleServerLogic(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard) // Discard logs during test run
	defer log.SetOutput(originalOutput)

	serverConn, clientConn := createTestConnections()
	defer serverConn.Close()
	// Note: Client simulation closes its connection

	testServerName := "TestServerLogicServer"
	testClientName := "TestServerLogicClient"

	// Clean up sandbox directory before and after test
	// Use the constant defined in filesystem.go (since it's package main)
	_ = os.RemoveAll(fileSystemSandbox)
	defer func() {
		log.SetOutput(originalOutput) // Restore log output before cleanup logs
		log.Printf("Cleaning up test sandbox directory: %s", fileSystemSandbox)
		_ = os.RemoveAll(fileSystemSandbox)
	}()

	var wg sync.WaitGroup
	var serverErr error

	wg.Add(1) // Only waiting for the server now

	// Run server logic in a goroutine using the mcp.Server
	go func() {
		defer wg.Done()
		// Create server instance using the new constructor with the test connection
		server := mcp.NewServerWithConnection(testServerName, serverConn)

		// Register tools (using definitions and handlers from other files in package main)
		// These variables (echoTool, calculatorToolDefinition, fileSystemToolDefinition)
		// and functions (echoHandler, calculatorHandler, filesystemHandler)
		// must be defined in the *.go files within this 'main' package (examples/server/).
		if err := server.RegisterTool(echoTool, echoHandler); err != nil {
			serverErr = fmt.Errorf("test setup: failed to register echo tool: %w", err)
			serverConn.Close() // Close conn if setup fails
			return
		}
		if err := server.RegisterTool(calculatorToolDefinition, calculatorHandler); err != nil {
			serverErr = fmt.Errorf("test setup: failed to register calculator tool: %w", err)
			serverConn.Close()
			return
		}
		if err := server.RegisterTool(fileSystemToolDefinition, filesystemHandler); err != nil {
			serverErr = fmt.Errorf("test setup: failed to register filesystem tool: %w", err)
			serverConn.Close()
			return
		}

		// Run the server's main loop
		serverErr = server.Run()
		// Close connection from server side if it finishes early (e.g., error)
		serverConn.Close()
	}()

	// Simulate client interaction in the main test goroutine
	clientErr := func() error {
		// 1. Handshake
		hsReq := mcp.HandshakeRequestPayload{SupportedProtocolVersions: []string{"1.0"}, ClientName: testClientName}
		if err := clientConn.SendMessage(mcp.MessageTypeHandshakeRequest, hsReq); err != nil {
			return fmt.Errorf("client send hs req failed: %w", err)
		}
		msg, err := clientConn.ReceiveMessage() // Expect HandshakeResponse
		if err != nil {
			return fmt.Errorf("client recv hs resp failed: %w", err)
		}
		if msg.MessageType != mcp.MessageTypeHandshakeResponse {
			return fmt.Errorf("client expected hs resp, got %s", msg.MessageType)
		}

		// 2. Get Tool Definitions
		tdReq := mcp.ToolDefinitionRequestPayload{}
		if err := clientConn.SendMessage(mcp.MessageTypeToolDefinitionRequest, tdReq); err != nil {
			return fmt.Errorf("client send td req failed: %w", err)
		}
		msg, err = clientConn.ReceiveMessage() // Expect ToolDefinitionResponse
		if err != nil {
			return fmt.Errorf("client recv td resp failed: %w", err)
		}
		if msg.MessageType != mcp.MessageTypeToolDefinitionResponse {
			return fmt.Errorf("client expected td resp, got %s", msg.MessageType)
		}

		// 3. Use Echo Tool
		echoArgs := map[string]interface{}{"message": "hello server"}
		utReqEcho := mcp.UseToolRequestPayload{ToolName: "echo", Arguments: echoArgs}
		if err := clientConn.SendMessage(mcp.MessageTypeUseToolRequest, utReqEcho); err != nil {
			return fmt.Errorf("client send echo req failed: %w", err)
		}
		msg, err = clientConn.ReceiveMessage() // Expect UseToolResponse
		if err != nil {
			return fmt.Errorf("client recv echo resp failed: %w", err)
		}
		if msg.MessageType != mcp.MessageTypeUseToolResponse {
			return fmt.Errorf("client expected echo resp, got %s", msg.MessageType)
		}

		// 4. Close client connection to signal EOF to server
		// log.Println("Client simulator closing connection...") // Keep logs discarded
		if err := clientConn.Close(); err != nil {
			// Log warning, but proceed as the primary check is server behavior
			log.Printf("Client simulator warning: error closing connection: %v", err)
		}

		return nil
	}() // Execute the client simulation immediately

	// Wait for server to finish (should happen after client closes pipe) or timeout
	wg.Wait() // Wait directly for the server goroutine

	// Assert results
	if clientErr != nil {
		t.Errorf("Client simulation failed: %v", clientErr)
	}
	// Server should exit cleanly with nil error after client disconnects (EOF)
	if serverErr != nil && !strings.Contains(serverErr.Error(), "EOF") && !strings.Contains(serverErr.Error(), "pipe") {
		t.Errorf("Server logic failed unexpectedly: %v", serverErr)
	} else if serverErr != nil {
		t.Logf("Server exited with expected EOF/pipe error: %v", serverErr)
	} else {
		t.Log("Server exited cleanly (nil error).")
	}
}
