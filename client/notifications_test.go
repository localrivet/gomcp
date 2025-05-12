package client

import (
	"testing"

	"github.com/localrivet/gomcp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock handler to capture notifications
type mockNotificationHandler struct {
	lastMethod       string
	lastNotification *protocol.JSONRPCNotification
	lastProgress     *protocol.ProgressParams
	lastResourceURI  string
	lastLogLevel     protocol.LoggingLevel
	lastLogMessage   string
	lastConnected    bool
	callCount        int
}

func (m *mockNotificationHandler) handle(notification *protocol.JSONRPCNotification) error {
	m.lastMethod = notification.Method
	m.lastNotification = notification
	m.callCount++
	return nil
}

func (m *mockNotificationHandler) handleProgress(progress *protocol.ProgressParams) error {
	m.lastProgress = progress
	m.callCount++
	return nil
}

func (m *mockNotificationHandler) handleResourceUpdate(uri string) error {
	m.lastResourceURI = uri
	m.callCount++
	return nil
}

func (m *mockNotificationHandler) handleLog(level protocol.LoggingLevel, message string) error {
	m.lastLogLevel = level
	m.lastLogMessage = message
	m.callCount++
	return nil
}

func (m *mockNotificationHandler) handleConnectionStatus(connected bool) error {
	m.lastConnected = connected
	m.callCount++
	return nil
}

func TestHandleProgress(t *testing.T) {
	// Create mock handler
	mock := &mockNotificationHandler{}

	// Create a client implementation for testing
	client := &clientImpl{
		progressHandlers:       []ProgressHandler{mock.handleProgress},
		notificationHandlers:   make(map[string][]NotificationHandler),
		resourceUpdateHandlers: make(map[string][]ResourceUpdateHandler),
		logHandlers:            []LogHandler{},
		connectionHandlers:     []ConnectionStatusHandler{},
	}

	// Create progress notification params
	progressParams := &protocol.ProgressParams{
		Token:   "progress-1",
		Value:   50,
		Message: protocol.StringPtr("Test Progress"),
	}

	// Process the notification directly with handleProgress
	err := client.handleProgress(*progressParams)
	require.NoError(t, err)

	// Verify the handler was called with correct params
	assert.Equal(t, 1, mock.callCount)
	assert.Equal(t, progressParams, mock.lastProgress)
	assert.Equal(t, "progress-1", mock.lastProgress.Token)
	assert.Equal(t, 50, mock.lastProgress.Value)
}

func TestHandleResourceUpdate(t *testing.T) {
	// Create mock handler
	mock := &mockNotificationHandler{}

	// Create a client implementation for testing
	resourceURI := "resource-uri"
	client := &clientImpl{
		progressHandlers:     []ProgressHandler{},
		notificationHandlers: make(map[string][]NotificationHandler),
		resourceUpdateHandlers: map[string][]ResourceUpdateHandler{
			resourceURI: {mock.handleResourceUpdate},
		},
		logHandlers:        []LogHandler{},
		connectionHandlers: []ConnectionStatusHandler{},
	}

	// Process the notification directly with handleResourceUpdate
	err := client.handleResourceUpdate(resourceURI)
	require.NoError(t, err)

	// Verify the handler was called with correct params
	assert.Equal(t, 1, mock.callCount)
	assert.Equal(t, resourceURI, mock.lastResourceURI)

	// Test with different resource URI (should not trigger handler)
	otherURI := "other-resource"

	// Reset call count
	mock.callCount = 0
	mock.lastResourceURI = ""

	// Process the notification with different URI
	err = client.handleResourceUpdate(otherURI)
	require.NoError(t, err)

	// Verify the handler was not called
	assert.Equal(t, 0, mock.callCount)
	assert.Equal(t, "", mock.lastResourceURI)
}

func TestHandleLog(t *testing.T) {
	// Create mock handler
	mock := &mockNotificationHandler{}

	// Create a client implementation for testing
	client := &clientImpl{
		progressHandlers:       []ProgressHandler{},
		notificationHandlers:   make(map[string][]NotificationHandler),
		resourceUpdateHandlers: make(map[string][]ResourceUpdateHandler),
		logHandlers:            []LogHandler{mock.handleLog},
		connectionHandlers:     []ConnectionStatusHandler{},
	}

	// Create log notification params
	level := protocol.LogLevelInfo
	message := "Test log message"

	// Process the notification directly with handleLog
	err := client.handleLog(level, message)
	require.NoError(t, err)

	// Verify the handler was called with correct params
	assert.Equal(t, 1, mock.callCount)
	assert.Equal(t, level, mock.lastLogLevel)
	assert.Equal(t, message, mock.lastLogMessage)
}

func TestHandleConnectionStatus(t *testing.T) {
	// Create mock handler
	mock := &mockNotificationHandler{}

	// Create a client implementation for testing
	client := &clientImpl{
		progressHandlers:       []ProgressHandler{},
		notificationHandlers:   make(map[string][]NotificationHandler),
		resourceUpdateHandlers: make(map[string][]ResourceUpdateHandler),
		logHandlers:            []LogHandler{},
		connectionHandlers:     []ConnectionStatusHandler{mock.handleConnectionStatus},
	}

	// Test with connected=true
	connected := true

	// Process the notification directly with handleConnectionStatus
	err := client.handleConnectionStatus(connected)
	require.NoError(t, err)

	// Verify the handler was called with correct params
	assert.Equal(t, 1, mock.callCount)
	assert.Equal(t, connected, mock.lastConnected)

	// Test with connected=false
	connected = false

	// Reset call count
	mock.callCount = 0
	mock.lastConnected = true

	// Process the notification
	err = client.handleConnectionStatus(connected)
	require.NoError(t, err)

	// Verify the handler was called with correct params
	assert.Equal(t, 1, mock.callCount)
	assert.Equal(t, connected, mock.lastConnected)
}

func TestClientNotificationRegistration(t *testing.T) {
	// Create a client implementation for testing
	client := &clientImpl{
		progressHandlers:       []ProgressHandler{},
		notificationHandlers:   make(map[string][]NotificationHandler),
		resourceUpdateHandlers: make(map[string][]ResourceUpdateHandler),
		logHandlers:            []LogHandler{},
		connectionHandlers:     []ConnectionStatusHandler{},
	}

	// Create mock handlers
	mockNotification := func(n *protocol.JSONRPCNotification) error { return nil }
	mockProgress := func(p *protocol.ProgressParams) error { return nil }
	mockResource := func(uri string) error { return nil }
	mockLog := func(level protocol.LoggingLevel, msg string) error { return nil }
	mockConnection := func(connected bool) error { return nil }

	// Register handlers
	client.OnNotification("custom", mockNotification)
	client.OnProgress(mockProgress)
	client.OnResourceUpdate("test-uri", mockResource)
	client.OnLog(mockLog)
	client.OnConnectionStatus(mockConnection)

	// Verify handlers were registered
	assert.Len(t, client.notificationHandlers["custom"], 1)
	assert.Len(t, client.progressHandlers, 1)
	assert.Len(t, client.resourceUpdateHandlers["test-uri"], 1)
	assert.Len(t, client.logHandlers, 1)
	assert.Len(t, client.connectionHandlers, 1)
}
