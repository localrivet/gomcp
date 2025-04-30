package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types"
)

// TestClientHelpers tests the helper functions in the client package
func TestClientHelpers(t *testing.T) {
	// Example: Test BoolPtr
	b := true
	bp := BoolPtr(b)
	if bp == nil || *bp != b {
		t.Errorf("BoolPtr failed")
	}

	// Example: Test StringPtr
	s := "hello"
	sp := StringPtr(s)
	if sp == nil || *sp != s {
		t.Errorf("StringPtr failed")
	}
}

// TestClientServerIntegration tests the basic client-server interaction with stdio transport
func TestClientServerIntegration(t *testing.T) {
	// Create pipe connections for bidirectional communication
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	defer serverReader.Close()
	defer clientWriter.Close()
	defer clientReader.Close()
	defer serverWriter.Close()

	// Create transports for client and server
	serverTransport := stdio.NewStdioTransportWithReadWriter(serverReader, serverWriter, types.TransportOptions{})
	clientTransport := stdio.NewStdioTransportWithReadWriter(clientReader, clientWriter, types.TransportOptions{})

	// Create a temporary directory for test files if needed
	tempDir := filepath.Join(os.TempDir(), "mcp_client_test")
	_ = os.RemoveAll(tempDir) // Clean up any previous test runs
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create server with test tools
	srv := server.NewServer("TestServer")

	// Register an echo tool
	err := server.AddTool(
		srv,
		"echo",
		"Echoes back the provided message.",
		func(args struct {
			Message string `json:"message" description:"The message to echo." required:"true"`
		}) (protocol.Content, error) {
			return protocol.TextContent{Type: "text", Text: args.Message}, nil
		},
	)
	if err != nil {
		t.Fatalf("Failed to register echo tool: %v", err)
	}

	// Start server in a goroutine
	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	// Use a channel to signal when server is ready or errors
	serverErrChan := make(chan error, 1)

	go func() {
		session := &testSession{
			id:        "test-session",
			transport: serverTransport,
		}
		if err := srv.RegisterSession(session); err != nil {
			serverErrChan <- err
			return
		}

		for {
			select {
			case <-serverCtx.Done():
				return
			default:
				raw, err := serverTransport.Receive(serverCtx)
				if err != nil {
					if err == io.EOF || err == context.Canceled {
						return
					}
					serverErrChan <- err
					return
				}

				// Parse the message to determine if it's an initialize request
				var baseMsg struct {
					JSONRPC string      `json:"jsonrpc"`
					ID      interface{} `json:"id,omitempty"`
					Method  string      `json:"method"`
				}
				if err := json.Unmarshal(raw, &baseMsg); err != nil {
					serverErrChan <- fmt.Errorf("failed to parse message: %v", err)
					continue
				}

				// Handle initialize request manually
				if baseMsg.Method == "initialize" {
					var initReq protocol.JSONRPCRequest
					if err := json.Unmarshal(raw, &initReq); err != nil {
						serverErrChan <- fmt.Errorf("failed to parse initialize request: %v", err)
						continue
					}

					// Create a successful response
					initResponse := protocol.JSONRPCResponse{
						JSONRPC: "2.0",
						ID:      baseMsg.ID,
						Result: protocol.InitializeResult{
							ProtocolVersion: protocol.CurrentProtocolVersion,
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
						},
					}

					respBytes, err := json.Marshal(initResponse)
					if err != nil {
						serverErrChan <- fmt.Errorf("failed to marshal initialize response: %v", err)
						continue
					}

					if err := serverTransport.Send(serverCtx, respBytes); err != nil {
						serverErrChan <- fmt.Errorf("failed to send initialize response: %v", err)
						return
					}

					// Mark the session as initialized
					session.Initialize()
					continue
				}

				// Use normal server handling for all other messages
				responses := srv.HandleMessage(serverCtx, session.id, raw)
				for _, resp := range responses {
					if resp == nil {
						continue
					}
					respBytes, err := json.Marshal(resp)
					if err != nil {
						serverErrChan <- err
						continue
					}
					if err := serverTransport.Send(serverCtx, respBytes); err != nil {
						serverErrChan <- err
						return
					}
				}
			}
		}
	}()

	// Create a client with the stdio transport
	clt, err := client.NewClient("TestClient", client.ClientOptions{
		Transport: clientTransport,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create a context with timeout for the test
	clientCtx, clientCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer clientCancel()

	// Connect the client (initializes the connection)
	if err := clt.Connect(clientCtx); err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer clt.Close()

	// Verify server info
	serverInfo := clt.ServerInfo()
	if serverInfo.Name != "TestServer" {
		t.Errorf("Expected server name 'TestServer', got '%s'", serverInfo.Name)
	}

	// Get tools
	toolsResult, err := clt.ListTools(clientCtx, protocol.ListToolsRequestParams{})
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}

	// Verify we have the echo tool
	echoFound := false
	for _, tool := range toolsResult.Tools {
		if tool.Name == "echo" {
			echoFound = true
			break
		}
	}
	if !echoFound {
		t.Errorf("Echo tool not found in tools list")
	}

	// Call the echo tool
	testMessage := "Hello, Test!"
	callResult, err := clt.CallTool(clientCtx, protocol.CallToolParams{
		Name: "echo",
		Arguments: map[string]interface{}{
			"message": testMessage,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Failed to call echo tool: %v", err)
	}

	// Verify the response
	if len(callResult.Content) == 0 {
		t.Fatalf("Expected non-empty result content")
	}

	// Check that the response is a text content with our message
	textContent, ok := callResult.Content[0].(protocol.TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, got %T", callResult.Content[0])
	}

	if textContent.Text != testMessage {
		t.Errorf("Expected echo response '%s', got '%s'", testMessage, textContent.Text)
	}
}

// testSession implements the server.ClientSession interface for testing
type testSession struct {
	id          string
	transport   types.Transport
	initialized bool
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
func (s *testSession) Initialize()                                              { s.initialized = true }
func (s *testSession) Initialized() bool                                        { return s.initialized }
func (s *testSession) StoreClientCapabilities(caps protocol.ClientCapabilities) {}
func (s *testSession) GetClientCapabilities() protocol.ClientCapabilities {
	return protocol.ClientCapabilities{}
}
func (s *testSession) SetNegotiatedVersion(version string) {}
func (s *testSession) GetNegotiatedVersion() string        { return "1.0" }
