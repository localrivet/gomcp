package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/localrivet/gomcp/types"
)

// mockTransport implements types.Transport for testing
type mockTransport struct {
	mu          sync.Mutex
	closed      bool
	sendChan    chan []byte
	receiveChan chan []byte
	errorOnSend error
	errorOnRecv error
	onClose     func() error
	logger      *NilLogger
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		sendChan:    make(chan []byte, 10),
		receiveChan: make(chan []byte, 10),
		logger:      NewNilLogger(),
	}
}

func (m *mockTransport) Send(ctx context.Context, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("transport is closed")
	}

	if m.errorOnSend != nil {
		return m.errorOnSend
	}

	// Simulate context cancellation check
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Simulate non-blocking send
	select {
	case m.sendChan <- data:
		return nil
	default:
		// In a real transport, this might block or return a specific error
		return fmt.Errorf("mock send buffer full")
	}
}

func (m *mockTransport) Receive(ctx context.Context) ([]byte, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, fmt.Errorf("transport is closed")
	}
	m.mu.Unlock()

	if m.errorOnRecv != nil {
		return nil, m.errorOnRecv
	}

	select {
	case data, ok := <-m.receiveChan:
		if !ok {
			return nil, fmt.Errorf("transport is closed")
		}
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// EstablishReceiver is a no-op for the mock transport.
func (m *mockTransport) EstablishReceiver(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("transport is closed")
	}
	// No receiver setup needed for mock
	return nil
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	close(m.sendChan)
	close(m.receiveChan)

	if m.onClose != nil {
		return m.onClose()
	}
	return nil
}

// Test helper methods
func (m *mockTransport) SimulateSend(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("transport is closed")
	}

	select {
	case m.receiveChan <- data:
		return nil
	default:
		return fmt.Errorf("receive buffer full")
	}
}

func (m *mockTransport) SimulateReceive() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, fmt.Errorf("transport is closed")
	}

	select {
	case data := <-m.sendChan:
		return data, nil
	default:
		return nil, fmt.Errorf("no data available")
	}
}

func (m *mockTransport) SetError(sendErr, recvErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorOnSend = sendErr
	m.errorOnRecv = recvErr
}

func (m *mockTransport) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

var _ types.Transport = (*mockTransport)(nil) // Ensure interface compliance
