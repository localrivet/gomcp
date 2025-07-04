package server

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientSession_Env(t *testing.T) {
	tests := []struct {
		name     string
		session  *ClientSession
		expected map[string]string
	}{
		{
			name:     "nil session returns empty map",
			session:  nil,
			expected: map[string]string{},
		},
		{
			name: "session with environment variables",
			session: &ClientSession{
				ClientInfo: ClientInfo{
					Env: map[string]string{
						"PROJECT_ROOT": "/workspace/project",
						"DEBUG":        "true",
						"NODE_ENV":     "development",
					},
				},
			},
			expected: map[string]string{
				"PROJECT_ROOT": "/workspace/project",
				"DEBUG":        "true",
				"NODE_ENV":     "development",
			},
		},
		{
			name: "session with empty environment variables",
			session: &ClientSession{
				ClientInfo: ClientInfo{
					Env: map[string]string{},
				},
			},
			expected: map[string]string{},
		},
		{
			name: "session with nil environment variables",
			session: &ClientSession{
				ClientInfo: ClientInfo{
					Env: nil,
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.session.Env()

			if tt.expected == nil && result != nil {
				t.Errorf("expected nil, got %v", result)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d items, got %d", len(tt.expected), len(result))
				return
			}

			for key, expectedValue := range tt.expected {
				if actualValue, exists := result[key]; !exists || actualValue != expectedValue {
					t.Errorf("expected %s=%s, got %s=%s", key, expectedValue, key, actualValue)
				}
			}
		})
	}
}

func TestClientSession_Roots(t *testing.T) {
	tests := []struct {
		name     string
		session  *ClientSession
		expected []string
	}{
		{
			name:     "nil session returns empty slice",
			session:  nil,
			expected: []string{},
		},
		{
			name: "session with workspace roots",
			session: &ClientSession{
				ClientInfo: ClientInfo{
					Roots: []string{
						"/workspace/project1",
						"/workspace/project2",
						"/home/user/docs",
					},
				},
			},
			expected: []string{
				"/workspace/project1",
				"/workspace/project2",
				"/home/user/docs",
			},
		},
		{
			name: "session with empty roots",
			session: &ClientSession{
				ClientInfo: ClientInfo{
					Roots: []string{},
				},
			},
			expected: []string{},
		},
		{
			name: "session with nil roots",
			session: &ClientSession{
				ClientInfo: ClientInfo{
					Roots: nil,
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.session.Roots()

			if tt.expected == nil && result != nil {
				t.Errorf("expected nil, got %v", result)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d roots, got %d", len(tt.expected), len(result))
				return
			}

			for i, expectedRoot := range tt.expected {
				if result[i] != expectedRoot {
					t.Errorf("expected root[%d]=%s, got %s", i, expectedRoot, result[i])
				}
			}
		})
	}
}

func TestClientSession_Capabilities(t *testing.T) {
	tests := []struct {
		name     string
		session  *ClientSession
		expected SamplingCapabilities
	}{
		{
			name:     "nil session returns empty capabilities",
			session:  nil,
			expected: SamplingCapabilities{},
		},
		{
			name: "session with full capabilities",
			session: &ClientSession{
				ClientInfo: ClientInfo{
					SamplingCaps: SamplingCapabilities{
						Supported:    true,
						TextSupport:  true,
						ImageSupport: true,
						AudioSupport: true,
					},
				},
			},
			expected: SamplingCapabilities{
				Supported:    true,
				TextSupport:  true,
				ImageSupport: true,
				AudioSupport: true,
			},
		},
		{
			name: "session with limited capabilities",
			session: &ClientSession{
				ClientInfo: ClientInfo{
					SamplingCaps: SamplingCapabilities{
						Supported:    true,
						TextSupport:  true,
						ImageSupport: false,
						AudioSupport: false,
					},
				},
			},
			expected: SamplingCapabilities{
				Supported:    true,
				TextSupport:  true,
				ImageSupport: false,
				AudioSupport: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.session.Capabilities()

			if result.Supported != tt.expected.Supported {
				t.Errorf("expected Supported=%v, got %v", tt.expected.Supported, result.Supported)
			}
			if result.TextSupport != tt.expected.TextSupport {
				t.Errorf("expected TextSupport=%v, got %v", tt.expected.TextSupport, result.TextSupport)
			}
			if result.ImageSupport != tt.expected.ImageSupport {
				t.Errorf("expected ImageSupport=%v, got %v", tt.expected.ImageSupport, result.ImageSupport)
			}
			if result.AudioSupport != tt.expected.AudioSupport {
				t.Errorf("expected AudioSupport=%v, got %v", tt.expected.AudioSupport, result.AudioSupport)
			}
		})
	}
}

func TestSessionManager_CreateSessionWithClientData(t *testing.T) {
	sm := NewSessionManager()

	clientInfo := ClientInfo{
		SamplingSupported: true,
		SamplingCaps: SamplingCapabilities{
			Supported:    true,
			TextSupport:  true,
			ImageSupport: true,
			AudioSupport: false,
		},
		ProtocolVersion: "2024-11-05",
		Env: map[string]string{
			"PROJECT_ROOT": "/workspace/test-project",
			"DEBUG":        "true",
		},
		Roots: []string{
			"/workspace/test-project",
			"/workspace/docs",
		},
	}

	session := sm.CreateSession(clientInfo, "2024-11-05")

	// Test session was created
	if session == nil {
		t.Fatal("expected session to be created, got nil")
	}

	// Test session ID is set
	if session.ID == "" {
		t.Error("expected session ID to be set")
	}

	// Test protocol version
	if session.ProtocolVersion != "2024-11-05" {
		t.Errorf("expected protocol version '2024-11-05', got '%s'", session.ProtocolVersion)
	}

	// Test environment variables access via session
	envVars := session.Env()
	if envVars["PROJECT_ROOT"] != "/workspace/test-project" {
		t.Errorf("expected PROJECT_ROOT='/workspace/test-project', got '%s'", envVars["PROJECT_ROOT"])
	}
	if envVars["DEBUG"] != "true" {
		t.Errorf("expected DEBUG='true', got '%s'", envVars["DEBUG"])
	}

	// Test workspace roots access via session
	roots := session.Roots()
	expectedRoots := []string{"/workspace/test-project", "/workspace/docs"}
	if len(roots) != len(expectedRoots) {
		t.Errorf("expected %d roots, got %d", len(expectedRoots), len(roots))
	}
	for i, expectedRoot := range expectedRoots {
		if roots[i] != expectedRoot {
			t.Errorf("expected root[%d]='%s', got '%s'", i, expectedRoot, roots[i])
		}
	}

	// Test capabilities access via session
	caps := session.Capabilities()
	if !caps.Supported {
		t.Error("expected sampling to be supported")
	}
	if !caps.TextSupport {
		t.Error("expected text support to be enabled")
	}
	if caps.AudioSupport {
		t.Error("expected audio support to be disabled for 2024-11-05")
	}
}

func TestContextSessionAccess(t *testing.T) {
	// Create a session with test data
	session := &ClientSession{
		ID:              "test-session-123",
		ProtocolVersion: "2024-11-05",
		ClientInfo: ClientInfo{
			Env: map[string]string{
				"PROJECT_ROOT": "/workspace/my-project",
				"NODE_ENV":     "test",
			},
			Roots: []string{
				"/workspace/my-project",
			},
			SamplingCaps: SamplingCapabilities{
				Supported:    true,
				TextSupport:  true,
				ImageSupport: true,
				AudioSupport: false,
			},
		},
	}

	// Create context with session
	ctx := &Context{
		Session: session,
	}

	// Test accessing environment variables through context
	envVars := ctx.Session.Env()
	if projectRoot := envVars["PROJECT_ROOT"]; projectRoot != "/workspace/my-project" {
		t.Errorf("expected PROJECT_ROOT='/workspace/my-project', got '%s'", projectRoot)
	}

	// Test accessing workspace roots through context
	roots := ctx.Session.Roots()
	if len(roots) != 1 || roots[0] != "/workspace/my-project" {
		t.Errorf("expected roots=['/workspace/my-project'], got %v", roots)
	}

	// Test accessing capabilities through context
	caps := ctx.Session.Capabilities()
	if !caps.TextSupport {
		t.Error("expected text support to be enabled")
	}

	// Test session metadata access
	if ctx.Session.ID != "test-session-123" {
		t.Errorf("expected session ID='test-session-123', got '%s'", ctx.Session.ID)
	}

	if ctx.Session.ProtocolVersion != "2024-11-05" {
		t.Errorf("expected protocol version='2024-11-05', got '%s'", ctx.Session.ProtocolVersion)
	}
}

func TestToolAccessPatterns(t *testing.T) {
	// Simulate a tool handler accessing session data
	toolHandler := func(ctx *Context) error {
		// Tool needs project root from environment
		envVars := ctx.Session.Env()
		projectRoot, exists := envVars["PROJECT_ROOT"]
		if !exists {
			return fmt.Errorf("PROJECT_ROOT not found in session environment")
		}

		// Tool needs workspace roots
		roots := ctx.Session.Roots()
		if len(roots) == 0 {
			return fmt.Errorf("no workspace roots found in session")
		}

		// Tool checks client capabilities
		caps := ctx.Session.Capabilities()
		if !caps.TextSupport {
			return fmt.Errorf("client does not support text")
		}

		// All checks passed
		t.Logf("Tool successfully accessed: PROJECT_ROOT=%s, roots=%v, textSupport=%v",
			projectRoot, roots, caps.TextSupport)
		return nil
	}

	// Create test context with session data
	ctx := &Context{
		Session: &ClientSession{
			ClientInfo: ClientInfo{
				Env: map[string]string{
					"PROJECT_ROOT": "/workspace/test",
				},
				Roots: []string{
					"/workspace/test",
				},
				SamplingCaps: SamplingCapabilities{
					Supported:   true,
					TextSupport: true,
				},
			},
		},
	}

	// Test the tool handler
	if err := toolHandler(ctx); err != nil {
		t.Errorf("tool handler failed: %v", err)
	}
}

// TestCompleteSessionFlow tests the entire initialization and session access flow
func TestCompleteSessionFlow(t *testing.T) {
	// Create a server instance with HTTP transport to test the HTTP-based client data flow
	server := NewServer("test-server").AsHTTP("localhost:0").(*serverImpl)
	server.sessionManager = NewSessionManager()

	// Simulate a REAL client initialize request (like Cursor sends)
	initializeRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities": map[string]interface{}{
				"tools":     true,
				"prompts":   false,
				"resources": false,
				"logging":   false,
				"roots": map[string]interface{}{
					"listChanged": true, // Client supports roots capability
				},
			},
			"clientInfo": map[string]interface{}{
				"name":    "cursor-vscode",
				"version": "1.0.0",
			},
		},
	}

	// Convert to JSON and create context
	requestBytes, err := json.Marshal(initializeRequest)
	assert.NoError(t, err)

	// Create context for initialize
	initCtx, err := NewContext(context.Background(), requestBytes, server)
	assert.NoError(t, err)

	// Process initialization (this should create and set defaultSession)
	result, err := server.ProcessInitialize(initCtx)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify defaultSession was created (MCP compliant: env/roots are empty until proper requests)
	assert.NotNil(t, server.defaultSession)

	// For MCP compliance: environment and roots start empty and get populated properly
	// - Environment variables come from transport headers (HTTP) or process env (stdio)
	// - Workspace roots come from roots/list requests only
	env := server.defaultSession.Env()
	roots := server.defaultSession.Roots()
	assert.True(t, len(env) == 0, "Environment should be empty initially (comes from headers)")
	assert.True(t, len(roots) == 0, "Roots should be empty initially (comes from roots/list)")

	// Verify that needsRootFetch was set since client supports roots capability
	assert.True(t, server.needsRootFetch.Load(), "Server should be marked to fetch roots via roots/list")

	// Now simulate a tool call request
	toolCallRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "test_tool",
			"arguments": map[string]interface{}{},
		},
	}

	// Convert to JSON and create context for tool call
	toolRequestBytes, err := json.Marshal(toolCallRequest)
	assert.NoError(t, err)

	// Create context for tool call
	toolCtx, err := NewContext(context.Background(), toolRequestBytes, server)
	assert.NoError(t, err)

	// ✅ VERIFY: The tool context should have access to the session
	assert.NotNil(t, toolCtx.Session, "Context should have session attached")

	// At this point, env and roots are still empty because we haven't simulated the roots/list response
	// In a real implementation, the server would send roots/list request and get the response
	toolEnv := toolCtx.Session.Env()
	toolRoots := toolCtx.Session.Roots()
	assert.True(t, len(toolEnv) == 0, "Tool context environment should be empty/nil initially")
	assert.True(t, len(toolRoots) == 0, "Tool context roots should be empty/nil initially")

	t.Logf("✅ Complete session flow working: ctx.Session.Env() = %v", toolCtx.Session.Env())
	t.Logf("✅ Complete session flow working: ctx.Session.Roots() = %v", toolCtx.Session.Roots())
	t.Logf("✅ Server correctly detected client roots capability and marked needsRootFetch = %v", server.needsRootFetch.Load())
}
