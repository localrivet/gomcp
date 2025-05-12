package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/stretchr/testify/assert"
)

// TestMain is used for setup and teardown for all tests in the package
func TestMain(m *testing.M) {
	// Setup code here if needed

	// Run tests
	exitCode := m.Run()

	// Teardown code here if needed

	os.Exit(exitCode)
}

// testLogger provides a simple logger for tests
func testLogger() logx.Logger {
	return logx.NewDefaultLogger()
}

// setupMockServer creates a mock HTTP server for testing
func setupMockServer(handler http.Handler) *httptest.Server {
	return httptest.NewServer(handler)
}

// setupInMemoryClient creates an in-memory client for testing
func setupInMemoryClient() (Client, error) {
	tools := []protocol.Tool{
		{
			Name:        "test-tool",
			Description: "A test tool",
			InputSchema: protocol.ToolInputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDetail{
					"foo": {
						Type:        "string",
						Description: "A test parameter",
					},
				},
			},
		},
	}

	resources := []protocol.Resource{
		{
			URI:         "test-resource",
			Description: "A test resource",
			Kind:        string(protocol.ResourceKindText),
		},
	}

	prompts := []protocol.Prompt{
		{
			URI:         "test-prompt",
			Description: "A test prompt",
			Messages:    []protocol.PromptMessage{},
		},
	}

	return NewInMemoryClient("test-client", struct {
		Tools     []protocol.Tool
		Resources []protocol.Resource
		Prompts   []protocol.Prompt
	}{
		Tools:     tools,
		Resources: resources,
		Prompts:   prompts,
	}, WithLogger(testLogger()))
}

// createTestContext creates a context with timeout for tests
func createTestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

// TestMCPIsConnected verifies that MCP.IsConnected properly checks all servers
// and returns true if ANY server is connected
func TestMCPIsConnected(t *testing.T) {
	logger := logx.NewDefaultLogger()

	// Create mock clients
	mockClient1 := &mockClientTest{
		logger:      logger,
		isConnected: false,
	}

	mockClient2 := &mockClientTest{
		logger:      logger,
		isConnected: false,
	}

	// Create the MCP struct with the mock clients
	mcp := &MCP{
		Servers: map[string]Client{
			"server1": mockClient1,
			"server2": mockClient2,
		},
		logger: logger,
	}

	// Test 1: No servers connected
	assert.False(t, mcp.IsConnected(), "MCP should report not connected when no servers are connected")

	// Test 2: One server connected
	mockClient1.isConnected = true
	assert.True(t, mcp.IsConnected(), "MCP should report connected when at least one server is connected")

	// Test 3: All servers connected
	mockClient2.isConnected = true
	assert.True(t, mcp.IsConnected(), "MCP should report connected when all servers are connected")

	// Test 4: Only second server connected
	mockClient1.isConnected = false
	assert.True(t, mcp.IsConnected(), "MCP should report connected when only the second server is connected")
}

// mockClient is a simple mock Client implementation for testing
type mockClientTest struct {
	logger      logx.Logger
	isConnected bool
}

func (m *mockClientTest) Connect(ctx context.Context) error {
	m.isConnected = true
	return nil
}

func (m *mockClientTest) Close() error {
	m.isConnected = false
	return nil
}

func (m *mockClientTest) IsConnected() bool {
	return m.isConnected
}

func (m *mockClientTest) Cleanup() {
	m.isConnected = false
}

// Implement other required methods from the Client interface with minimal implementations

func (m *mockClientTest) Run(ctx context.Context) error {
	return nil
}

func (m *mockClientTest) ListTools(ctx context.Context) ([]protocol.Tool, error) {
	return nil, nil
}

func (m *mockClientTest) CallTool(ctx context.Context, name string, args map[string]interface{}, progressCh chan<- protocol.ProgressParams) ([]protocol.Content, error) {
	return nil, nil
}

func (m *mockClientTest) ListResources(ctx context.Context) ([]protocol.Resource, error) {
	return nil, nil
}

func (m *mockClientTest) ReadResource(ctx context.Context, uri string) ([]protocol.ResourceContents, error) {
	return nil, nil
}

func (m *mockClientTest) ListPrompts(ctx context.Context) ([]protocol.Prompt, error) {
	return nil, nil
}

func (m *mockClientTest) GetPrompt(ctx context.Context, name string, args map[string]interface{}) ([]protocol.PromptMessage, error) {
	return nil, nil
}

func (m *mockClientTest) ServerInfo() protocol.Implementation {
	return protocol.Implementation{}
}

func (m *mockClientTest) ServerCapabilities() protocol.ServerCapabilities {
	return protocol.ServerCapabilities{}
}

func (m *mockClientTest) SendRequest(ctx context.Context, method string, params interface{}) (*protocol.JSONRPCResponse, error) {
	return nil, nil
}

func (m *mockClientTest) WithTimeout(timeout time.Duration) Client {
	return m
}

func (m *mockClientTest) WithRetry(maxAttempts int, backoff BackoffStrategy) Client {
	return m
}

func (m *mockClientTest) WithMiddleware(middleware ClientMiddleware) Client {
	return m
}

func (m *mockClientTest) WithAuth(auth AuthProvider) Client {
	return m
}

func (m *mockClientTest) WithLogger(logger logx.Logger) Client {
	m.logger = logger
	return m
}

func (m *mockClientTest) OnNotification(method string, handler NotificationHandler) Client {
	return m
}

func (m *mockClientTest) OnProgress(handler ProgressHandler) Client {
	return m
}

func (m *mockClientTest) OnResourceUpdate(uri string, handler ResourceUpdateHandler) Client {
	return m
}

func (m *mockClientTest) OnLog(handler LogHandler) Client {
	return m
}

func (m *mockClientTest) OnConnectionStatus(handler ConnectionStatusHandler) Client {
	return m
}
