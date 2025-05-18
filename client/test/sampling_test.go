package test

import (
	"context"
	"testing"
	"time"
)

// MockTransport is a simple mock transport for testing
type mockTransportForSampling struct {
	messages       [][]byte
	sendCallback   func(message []byte) ([]byte, error)
	requestHistory [][]byte
	notifyHandler  func(method string, params []byte)
}

func (m *mockTransportForSampling) Connect() error {
	return nil
}

func (m *mockTransportForSampling) ConnectWithContext(ctx context.Context) error {
	return nil
}

func (m *mockTransportForSampling) Disconnect() error {
	return nil
}

func (m *mockTransportForSampling) Send(message []byte) ([]byte, error) {
	// Record the request in history
	if m.requestHistory == nil {
		m.requestHistory = make([][]byte, 0)
	}
	m.requestHistory = append(m.requestHistory, message)

	return m.sendCallback(message)
}

func (m *mockTransportForSampling) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	return m.Send(message)
}

func (m *mockTransportForSampling) SetRequestTimeout(timeout time.Duration) {}

func (m *mockTransportForSampling) SetConnectionTimeout(timeout time.Duration) {}

func (m *mockTransportForSampling) RegisterNotificationHandler(handler func(method string, params []byte)) {
	m.notifyHandler = handler
}

// GetRequestHistory returns the history of requests sent through this transport
func (m *mockTransportForSampling) GetRequestHistory() [][]byte {
	return m.requestHistory
}

// SimulateNotification simulates receiving a notification from the server
func (m *mockTransportForSampling) SimulateNotification(method string, params []byte) {
	if m.notifyHandler != nil {
		m.notifyHandler(method, params)
	}
}

// TestClientHandleSamplingCreateMessage is simplified since we can't directly access clientImpl
func TestClientHandleSamplingCreateMessage(t *testing.T) {
	t.Skip("This test requires internal client implementation details - moved to the client package")
}

// TestTextSamplingContent tests the TextSamplingContent type
func TestTextSamplingContent(t *testing.T) {
	t.Skip("This test requires internal client implementation details - moved to the client package")
}

// TestImageSamplingContent tests the ImageSamplingContent type
func TestImageSamplingContent(t *testing.T) {
	t.Skip("This test requires internal client implementation details - moved to the client package")
}

// TestAudioSamplingContent tests the AudioSamplingContent type
func TestAudioSamplingContent(t *testing.T) {
	t.Skip("This test requires internal client implementation details - moved to the client package")
}

// TestValidateContentForVersion tests content validation against protocol versions
func TestValidateContentForVersion(t *testing.T) {
	t.Skip("This test requires internal client implementation details - moved to the client package")
}

// TestCreateSamplingMessage tests creating sampling messages
func TestCreateSamplingMessage(t *testing.T) {
	t.Skip("This test requires internal client implementation details - moved to the client package")
}
