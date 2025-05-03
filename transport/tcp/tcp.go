// Package tcp provides a types.Transport implementation using TCP sockets.
package tcp

import (
	"bufio"
	"bytes" // Needed for byte buffer
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log" // Added import for default logger
	"net" // Added for TCP
	"sync"
	"time" // Added for read timeout

	"github.com/localrivet/gomcp/protocol" // For ErrorPayload, ErrorCodeParseError
	"github.com/localrivet/gomcp/types"    // For types.Transport, types.Logger
)

// TCPTransport implements the Transport interface using a net.Conn.
type TCPTransport struct {
	conn       net.Conn  // Underlying TCP connection
	reader     io.Reader // Use raw reader for ReadByte approach
	writer     io.Writer // Use buffered writer
	writeMutex sync.Mutex
	logger     types.Logger
	closed     bool
	closeMutex sync.Mutex

	// Store original connection for closing
	rawConn net.Conn
}

// Ensure TCPTransport implements types.Transport
var _ types.Transport = (*TCPTransport)(nil) // Updated to ensure interface compliance

// NewTCPTransport creates a new TCPTransport wrapping an existing net.Conn.
func NewTCPTransport(conn net.Conn, opts types.TransportOptions) *TCPTransport {
	logger := opts.Logger
	if logger == nil {
		logger = &defaultLogger{} // Use a simple default logger
	}

	bufferSize := 4096 // Default buffer size
	if opts.BufferSize > 0 {
		bufferSize = opts.BufferSize
	}

	logger.Info("Creating new TCPTransport for connection: %s -> %s", conn.LocalAddr(), conn.RemoteAddr())

	return &TCPTransport{
		conn:    conn,                                  // Keep original conn reference if needed, e.g., for Addr()
		reader:  conn,                                  // Use raw conn for ReadByte approach
		writer:  bufio.NewWriterSize(conn, bufferSize), // Use buffered writer
		logger:  logger,
		closed:  false,
		rawConn: conn, // Store raw connection
	}
}

// Send writes a message to the TCP connection, respecting the context.
// Uses newline framing similar to stdio transport.
func (t *TCPTransport) Send(ctx context.Context, data []byte) error {
	// Check context cancellation first
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	t.closeMutex.Lock()
	if t.closed {
		t.closeMutex.Unlock()
		return fmt.Errorf("transport is closed")
	}
	t.closeMutex.Unlock()

	t.writeMutex.Lock()
	defer t.writeMutex.Unlock()

	if len(data) == 0 {
		return fmt.Errorf("cannot send empty message")
	}
	if data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	t.logger.Debug("TCPTransport Send: %s", string(data))

	_, err := t.writer.Write(data)
	if err != nil {
		t.logger.Error("TCPTransport: Failed to write message: %v", err)
		_ = t.Close() // Attempt to close on write error
		return fmt.Errorf("failed to write message: %w", err)
	}

	// Flush the buffered writer
	if flusher, ok := t.writer.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			t.logger.Warn("TCPTransport: Failed to flush writer: %v", err)
			_ = t.Close()
			return fmt.Errorf("failed to flush writer: %w", err)
		}
	}
	t.logger.Debug("TCPTransport Send: Write/Flush completed.") // ADDED LOG
	return nil
}

// Receive reads the next newline-delimited message from the TCP connection, respecting context.
// This replaces the old Receive and ReceiveWithContext methods.
func (t *TCPTransport) Receive(ctx context.Context) ([]byte, error) {
	t.closeMutex.Lock()
	if t.closed {
		t.closeMutex.Unlock()
		return nil, fmt.Errorf("transport is closed")
	}
	t.closeMutex.Unlock()

	// Set read deadline based on context
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		if err := t.conn.SetReadDeadline(deadline); err != nil {
			t.logger.Warn("TCPTransport: Failed to set read deadline from context: %v", err)
			// If we can't set deadline, context cancellation might not work reliably via this mechanism
		}
		// Ensure deadline is reset when function returns
		defer func() {
			if err := t.conn.SetReadDeadline(time.Time{}); err != nil {
				t.logger.Warn("TCPTransport: Failed to reset read deadline: %v", err)
			}
		}()
	} else {
		// If no deadline from context, apply a very long default read deadline
		// to prevent goroutine leaks if connection hangs without Close being called.
		// Set a long deadline instead of infinite to prevent leaks.
		longDeadline := time.Now().Add(24 * time.Hour)
		if err := t.conn.SetReadDeadline(longDeadline); err != nil {
			t.logger.Warn("TCPTransport: Failed to set long default read deadline: %v", err)
		}
		// Ensure deadline is reset when function returns
		defer func() {
			if err := t.conn.SetReadDeadline(time.Time{}); err != nil {
				t.logger.Warn("TCPTransport: Failed to reset long default read deadline: %v", err)
			}
		}()
	}

	// Use a bufio.Reader on the connection for ReadBytes
	// Create a new one each time to avoid state issues if Receive is called concurrently (though it shouldn't be)
	reader := bufio.NewReader(t.conn)

	t.logger.Debug("TCPTransport: Attempting reader.ReadBytes('\n')...")
	lineBytes, err := reader.ReadBytes('\n')

	if err != nil {
		// Check if the error is due to context deadline/timeout
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			// Check if the timeout corresponds to our context deadline (if set)
			if hasDeadline && time.Now().After(deadline.Add(-time.Millisecond*100)) { // Allow grace period
				t.logger.Warn("TCPTransport: Read timed out due to context deadline.")
				return nil, context.DeadlineExceeded // Return context error
			} else if hasDeadline {
				t.logger.Warn("TCPTransport: Read timed out, but doesn't match context deadline exactly.")
				// Fall through to return the netErr
			} else {
				// Likely hit the long default deadline
				t.logger.Error("TCPTransport: Read hit long default timeout (24h): %v", err)
				// Fall through
			}
		} else if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
			t.logger.Info("TCPTransport: Connection closed while reading: %v", err)
		} else {
			t.logger.Error("TCPTransport: Error reading message line: %v", err)
		}
		_ = t.Close()                                                  // Ensure closed state is set on any read error
		return nil, fmt.Errorf("failed to read message line: %w", err) // Return the original error
	}

	// Trim trailing newline if present (ReadBytes includes the delimiter)
	trimmedBytes := bytes.TrimSuffix(lineBytes, []byte("\n"))
	trimmedBytes = bytes.TrimSuffix(trimmedBytes, []byte("\r")) // Also trim CR just in case

	t.logger.Debug("TCPTransport Received raw line (trimmed): %s", string(trimmedBytes))

	if !json.Valid(trimmedBytes) {
		t.logger.Error("TCPTransport: Received invalid JSON: %s", string(trimmedBytes))
		// Send parse error notification (use background context for internal error reporting)
		_ = t.sendParseError(context.Background(), "Received invalid JSON")
		return nil, fmt.Errorf("received invalid JSON")
	}

	return trimmedBytes, nil
}

// Close closes the underlying TCP connection.
func (t *TCPTransport) Close() error {
	t.closeMutex.Lock()
	defer t.closeMutex.Unlock()

	if t.closed {
		return nil // Already closed
	}
	t.closed = true
	t.logger.Info("TCPTransport: Closing connection: %s -> %s", t.conn.LocalAddr(), t.conn.RemoteAddr())
	err := t.rawConn.Close()
	if err != nil {
		t.logger.Error("TCPTransport: Error closing raw connection: %v", err)
	} else {
		t.logger.Info("TCPTransport: Connection closed successfully.")
	}
	return err
}

// IsClosed returns true if the transport connection is closed.
func (t *TCPTransport) IsClosed() bool {
	t.closeMutex.Lock()
	defer t.closeMutex.Unlock()
	return t.closed
}

// RemoteAddr returns the remote network address.
func (t *TCPTransport) RemoteAddr() net.Addr {
	return t.conn.RemoteAddr()
}

// LocalAddr returns the local network address.
func (t *TCPTransport) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}

// sendParseError is a helper to send a JSON-RPC ParseError.
func (t *TCPTransport) sendParseError(ctx context.Context, message string) error {
	errResp := protocol.JSONRPCResponse{
		JSONRPC: "2.0", ID: nil,
		Error: &protocol.ErrorPayload{Code: protocol.CodeParseError, Message: message},
	}
	jsonData, err := json.Marshal(errResp)
	if err != nil {
		t.logger.Error("TCPTransport: Failed to marshal parse error response: %v", err)
		return err // Return the marshal error
	}
	// Use Send directly, ignoring potential errors during error reporting
	// Pass a background context as this is internal error reporting
	_ = t.Send(context.Background(), jsonData)
	return nil
}

// EstablishReceiver for TCPTransport does nothing, as the connection
// is already established when the transport is created, and reading
// happens directly within the Receive method. It exists to satisfy the Transport interface.
func (t *TCPTransport) EstablishReceiver(ctx context.Context) error {
	t.logger.Debug("TCPTransport: EstablishReceiver called (no-op).")
	// Check if already closed
	t.closeMutex.Lock()
	defer t.closeMutex.Unlock()
	if t.closed {
		return fmt.Errorf("transport is closed")
	}
	return nil
}

// --- Default Logger (Copied from stdio for now) ---
type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, args ...interface{}) { log.Printf("DEBUG: "+msg, args...) }
func (l *defaultLogger) Info(msg string, args ...interface{})  { log.Printf("INFO: "+msg, args...) }
func (l *defaultLogger) Warn(msg string, args ...interface{})  { log.Printf("WARN: "+msg, args...) }
func (l *defaultLogger) Error(msg string, args ...interface{}) { log.Printf("ERROR: "+msg, args...) }

// --- TODO ---
// - Consider adding read/write deadlines to net.Conn operations within Send/Receive goroutine
// - Evaluate framing: Newline is simple but less robust than length-prefixing.
