package stdio

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestNewTransport(t *testing.T) {
	// Test the default constructor
	transport := NewTransport()
	if transport.reader == nil {
		t.Error("Expected reader to be initialized, got nil")
	}
	if transport.writer == nil {
		t.Error("Expected writer to be initialized, got nil")
	}
	if transport.done == nil {
		t.Error("Expected done channel to be initialized, got nil")
	}
}

func TestNewTransportWithIO(t *testing.T) {
	// Test with custom IO
	in := strings.NewReader("test input")
	out := new(bytes.Buffer)

	transport := NewTransportWithIO(in, out)
	if transport.reader == nil {
		t.Error("Expected reader to be initialized, got nil")
	}
	if transport.writer == nil {
		t.Error("Expected writer to be initialized, got nil")
	}
}

func TestInitialize(t *testing.T) {
	in := strings.NewReader("")
	out := new(bytes.Buffer)
	transport := NewTransportWithIO(in, out)

	err := transport.Initialize()
	if err != nil {
		t.Errorf("Expected no error on Initialize, got %v", err)
	}
}

func TestSend(t *testing.T) {
	out := new(bytes.Buffer)
	transport := NewTransportWithIO(strings.NewReader(""), out)

	message := []byte("test message")
	err := transport.Send(message)
	if err != nil {
		t.Errorf("Unexpected error on Send: %v", err)
	}

	// Should include a newline by default
	expected := "test message\n"
	if out.String() != expected {
		t.Errorf("Expected output %q, got %q", expected, out.String())
	}

	// Test without newline
	out.Reset()
	transport.SetNewline(false)
	err = transport.Send(message)
	if err != nil {
		t.Errorf("Unexpected error on Send: %v", err)
	}

	expected = "test message"
	if out.String() != expected {
		t.Errorf("Expected output %q, got %q", expected, out.String())
	}
}

func TestReceive(t *testing.T) {
	transport := NewTransport()

	_, err := transport.Receive()
	if err == nil {
		t.Error("Expected error on Receive, got nil")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("Expected 'not implemented' error, got %v", err)
	}
}

func TestReadLoop(t *testing.T) {
	// Create transport with mock IO
	input := "test message\n"
	in := strings.NewReader(input)
	out := new(bytes.Buffer)
	transport := NewTransportWithIO(in, out)

	// Set up a handler that echoes the message
	transport.SetMessageHandler(func(message []byte) ([]byte, error) {
		return message, nil
	})

	// Start the transport
	err := transport.Start()
	if err != nil {
		t.Errorf("Unexpected error on Start: %v", err)
	}

	// Wait a short time for the message to be processed
	time.Sleep(50 * time.Millisecond)

	// Check that the message was echoed
	expected := "test message\n"
	if out.String() != expected {
		t.Errorf("Expected output %q, got %q", expected, out.String())
	}

	// Clean up
	transport.Stop()
}

func TestReadLoopWithError(t *testing.T) {
	// Create transport with mock IO
	input := "test message\n"
	in := strings.NewReader(input)
	out := new(bytes.Buffer)
	transport := NewTransportWithIO(in, out)

	// Set up a handler that returns an error
	expectedErr := errors.New("handler error")
	transport.SetMessageHandler(func(message []byte) ([]byte, error) {
		return nil, expectedErr
	})

	// Start the transport
	transport.Start()

	// Wait a short time for the message to be processed
	time.Sleep(50 * time.Millisecond)

	// Check that no output was produced
	if out.String() != "" {
		t.Errorf("Expected empty output, got %q", out.String())
	}

	// Clean up
	transport.Stop()
}

func TestReadLoopWithEOF(t *testing.T) {
	// Create a reader that immediately returns EOF
	in := &eofReader{}
	out := new(bytes.Buffer)
	transport := NewTransportWithIO(in, out)

	// Start the transport
	transport.Start()

	// Wait a short time for EOF to be detected
	time.Sleep(50 * time.Millisecond)

	// Verify EOF was detected
	if !transport.readEOF {
		t.Error("Expected readEOF to be true")
	}

	// Clean up
	transport.Stop()
}

// eofReader is a mock reader that always returns EOF
type eofReader struct{}

func (r *eofReader) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}
