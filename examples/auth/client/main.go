// Command auth/client demonstrates an MCP client that uses JWT authentication
// to access secure tools. Features include:
//
// Authentication
// -------------
//
//   - JWT tokens via environment variables or fallback to test token
//   - Authentication hooks for request processing
//   - Context-based token management
//
// Client Operations
// ----------------
//
//   - Connecting to an MCP server with SSE transport
//   - Listing available tools
//   - Calling protected tools with and without authentication
//
// Build & run:
//
//	go run .
//
// Set JWT_TOKEN environment variable to use a real token.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/auth"
	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/hooks"
	"github.com/localrivet/gomcp/protocol"
)

// requestToolDefinitions uses the client to request tool definitions.
func requestToolDefinitions(ctx context.Context, clt *client.Client) ([]protocol.Tool, error) {
	log.Println("Sending ListToolsRequest...")
	params := protocol.ListToolsRequestParams{}
	result, err := clt.ListTools(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("ListTools failed: %w", err)
	}
	// TODO: Handle pagination if result.NextCursor is not empty
	log.Printf("Received %d tool definitions", len(result.Tools))
	return result.Tools, nil
}

// useTool sends a CallToolRequest using the client and processes the response.
func useTool(ctx context.Context, clt *client.Client, toolName string, args map[string]interface{}) ([]protocol.Content, error) {
	log.Printf("Sending CallToolRequest for tool '%s'...", toolName)
	reqParams := protocol.CallToolParams{
		Name:      toolName,
		Arguments: args,
	}

	// Call the tool using the client method
	result, err := clt.CallTool(ctx, reqParams, nil)
	if err != nil {
		// This error could be a transport error, timeout, or an MCP error response
		return nil, fmt.Errorf("CallTool '%s' failed: %w", toolName, err)
	}

	// Check if the tool execution itself resulted in an error (IsError flag)
	if result.IsError != nil && *result.IsError {
		errMsg := fmt.Sprintf("Tool '%s' execution reported an error", toolName)
		if len(result.Content) > 0 {
			// Check content type before asserting
			if textContent, ok := result.Content[0].(protocol.TextContent); ok {
				errMsg = fmt.Sprintf("Tool '%s' failed: %s", toolName, textContent.Text)
			} else {
				errMsg = fmt.Sprintf("Tool '%s' failed with non-text error content: %T", toolName, result.Content[0])
			}
		}
		// Return content even on error, as per MCP spec for tool errors
		return result.Content, fmt.Errorf("%s", errMsg)
	}

	log.Printf("Tool '%s' executed successfully.", toolName)
	return result.Content, nil
}

// setupAuthContext adds authentication token to the context
func setupAuthContext(ctx context.Context) context.Context {
	// Get JWT token from environment variable or use a demo token for testing
	token := os.Getenv("JWT_TOKEN")
	if token == "" {
		// For demo purposes only - in production, you'd use a real JWT
		token = "test-token-123"
		log.Println("WARNING: Using demo token. Set JWT_TOKEN env var for real authentication.")
	}

	// Add token to context using auth package helper
	return auth.ContextWithToken(ctx, "Bearer "+token)
}

// createAuthHook creates a hook that adds the auth token to outgoing requests
func createAuthHook() hooks.ClientBeforeSendRequestHook {
	return func(hookCtx hooks.ClientHookContext, req *protocol.JSONRPCRequest) (*protocol.JSONRPCRequest, error) {
		// Get token from context
		token, ok := auth.TokenFromContext(hookCtx.Ctx)
		if !ok || token == "" {
			// No token found, don't modify the request
			return req, nil
		}

		// The hook doesn't have direct access to modify HTTP headers,
		// but we've already put the token in the context, which the
		// transport layer should check when making the actual HTTP request

		return req, nil
	}
}

// runClientLogic creates a client, connects, and executes the example tool calls sequence.
func runClientLogic(ctx context.Context, clientName string) error {
	// ---------------------------------------------------------------------
	// Authentication setup
	// ---------------------------------------------------------------------
	ctx = setupAuthContext(ctx)
	authHook := createAuthHook()

	// ---------------------------------------------------------------------
	// Client creation and connection
	// ---------------------------------------------------------------------
	clt, err := client.NewSSEClient(clientName,
		"http://127.0.0.1:8080", // Base URL
		"/mcp",                  // Base path - typical default for MCP
		client.ClientOptions{
			BeforeSendRequestHooks: []hooks.ClientBeforeSendRequestHook{authHook},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Connect and perform initialization
	log.Println("Connecting to server...")
	err = clt.Connect(ctx)
	if err != nil {
		return fmt.Errorf("client failed to connect: %w", err)
	}
	defer clt.Close()
	log.Printf("Client connected successfully to server: %s (Version: %s)", clt.ServerInfo().Name, clt.ServerInfo().Version)
	log.Printf("Server Capabilities: %+v", clt.ServerCapabilities())

	// ---------------------------------------------------------------------
	// List available tools
	// ---------------------------------------------------------------------
	tools, err := requestToolDefinitions(ctx, clt)
	if err != nil {
		return fmt.Errorf("failed to get tool definitions: %w", err)
	}
	log.Printf("Received %d tool definitions:", len(tools))
	for _, tool := range tools {
		toolJson, _ := json.MarshalIndent(tool, "", "  ")
		fmt.Fprintf(os.Stderr, "%s\n", string(toolJson))
	}

	// ---------------------------------------------------------------------
	// Test secure-echo tool with and without authentication
	// ---------------------------------------------------------------------
	secureEchoToolFound := false
	for _, tool := range tools {
		if tool.Name == "secure-echo" {
			secureEchoToolFound = true
			break
		}
	}
	if secureEchoToolFound {
		log.Println("\n--- Testing Secure Echo Tool ---")
		echoMessage := "Secret message!"
		args := map[string]interface{}{"message": echoMessage}

		// First try without auth context to demonstrate it fails
		unauthCtx := context.Background()
		log.Println("First attempt: Calling secure-echo WITHOUT authentication (should fail)...")
		result, err := useTool(unauthCtx, clt, "secure-echo", args)
		if err != nil {
			log.Printf("Using 'secure-echo' tool without auth failed as expected: %v", err)
		} else {
			log.Printf("WARNING: Tool succeeded without authentication when it should have failed!")
		}

		// Then try with auth context to show it works
		log.Println("Second attempt: Calling secure-echo WITH authentication...")
		result, err = useTool(ctx, clt, "secure-echo", args)
		if err != nil {
			log.Printf("ERROR: Using 'secure-echo' tool with auth failed: %v", err)
		} else {
			log.Printf("Successfully used 'secure-echo' tool with authentication.")
			log.Printf("  Sent: %s", echoMessage)
			log.Printf("  Received Content: %+v", result)
			if len(result) > 0 {
				if textContent, ok := result[0].(protocol.TextContent); ok {
					log.Printf("  Extracted Text: %s", textContent.Text)
					if textContent.Text != echoMessage {
						log.Printf("WARNING: Secure Echo result '%s' did not match sent message '%s'", textContent.Text, echoMessage)
					}
				} else {
					log.Printf("WARNING: Secure Echo result content[0] was not TextContent: %T", result[0])
				}
			} else {
				log.Printf("WARNING: Secure Echo result content was empty!")
			}
		}
	} else {
		log.Println("Could not find 'secure-echo' tool definition from server.")
	}

	log.Println("Client operations finished.")
	return nil
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting JWT Auth Example MCP Client...")

	clientName := "GoAuthClient-JWT"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Run the core client logic
	err := runClientLogic(ctx, clientName)
	if err != nil {
		log.Fatalf("Client exited with error: %v", err)
	}

	log.Println("Client finished successfully.")
}
