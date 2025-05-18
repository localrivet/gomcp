package transport

import (
	"errors"
	"testing"
)

func TestBaseTransport_SetMessageHandler(t *testing.T) {
	bt := &BaseTransport{}

	// Test with nil handler
	if bt.handler != nil {
		t.Errorf("Expected nil handler, got %v", bt.handler)
	}

	// Set a handler
	handler := func(message []byte) ([]byte, error) {
		return message, nil
	}
	bt.SetMessageHandler(handler)

	// Check that handler was set
	if bt.handler == nil {
		t.Errorf("Expected handler to be set, got nil")
	}
}

func TestBaseTransport_HandleMessage(t *testing.T) {
	bt := &BaseTransport{}

	// Test with no handler set
	message := []byte("test message")
	_, err := bt.HandleMessage(message)
	if err == nil {
		t.Error("Expected error when no handler is set, got nil")
	}
	if err != nil && err.Error() != "no message handler set" {
		t.Errorf("Expected 'no message handler set' error, got %v", err)
	}

	// Set a handler that echoes the message
	bt.SetMessageHandler(func(message []byte) ([]byte, error) {
		return message, nil
	})

	// Test with handler set
	response, err := bt.HandleMessage(message)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if string(response) != string(message) {
		t.Errorf("Expected response to be '%s', got '%s'", string(message), string(response))
	}

	// Set a handler that returns an error
	expectedErr := errors.New("handler error")
	bt.SetMessageHandler(func(message []byte) ([]byte, error) {
		return nil, expectedErr
	})

	// Test with error-returning handler
	_, err = bt.HandleMessage(message)
	if err != expectedErr {
		t.Errorf("Expected '%v' error, got '%v'", expectedErr, err)
	}
}
