package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings" // Needed for error check
	"sync"
	"testing"
	"time"

	mcp "github.com/localrivet/gomcp"
)

// createTestConnections is a helper from the library tests, duplicated here for simplicity
// or could be exposed from the library if desired.
func createTestConnections() (*mcp.Connection, *mcp.Connection) {
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	serverConn := mcp.NewConnection(serverReader, serverWriter)
	clientConn := mcp.NewConnection(clientReader, clientWriter)
	return serverConn, clientConn
}

// TestExampleServerLogic runs the server logic and simulates a basic client interaction.
func TestExampleServerLogic(t *testing.T) {
	originalOutput := log.Writer()
	log.SetOutput(io.Discard) // Discard logs during test run
	defer log.SetOutput(originalOutput)

	serverConn, clientConn := createTestConnections()
	defer serverConn.Close()
	// Note: We explicitly close clientConn writer in the simulator goroutine

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

	// Run server logic in a goroutine
	go func() {
		defer wg.Done()
		serverErr = runServerLogic(serverConn, testServerName)
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
		// log.Println("Client simulator: Handshake OK") // Keep logs discarded

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
		// log.Println("Client simulator: Tool Def OK")

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
		// log.Println("Client simulator: Echo OK")

		// 4. Close client connection to signal EOF to server
		// The Close method on the Connection will attempt to close the underlying pipe writer.
		log.Println("Client simulator closing connection...")
		if err := clientConn.Close(); err != nil {
			// Log warning, but proceed as the primary check is server behavior
			log.Printf("Client simulator warning: error closing connection: %v", err)
		}

		return nil
	}() // Execute the client simulation immediately

	// Wait for server to finish (should happen after client closes pipe) or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Server completed
	case <-time.After(10 * time.Second): // Timeout
		serverConn.Close() // Attempt to unblock
		t.Fatal("Server logic test timed out")
	}

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
