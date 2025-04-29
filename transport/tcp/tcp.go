// Package tcp provides a types.Transport implementation using TCP sockets.
package tcp

import (
	"bufio"
	"bytes" // Needed for byte buffer
	"context"
	"encoding/json"
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

	// Use a ByteReader if the underlying reader supports it, otherwise wrap
	var byteReader io.ByteReader
	if br, ok := t.reader.(io.ByteReader); ok {
		byteReader = br
	} else {
		// Wrap in bufio.Reader ONLY for ByteReader interface, limited buffer
		// This is less efficient for TCP but provides the ByteReader interface
		byteReader = bufio.NewReaderSize(t.reader, 1)
	}

	resultCh := make(chan struct {
		data []byte
		err  error
	}, 1)

	// Goroutine for the blocking read loop
	go func() {
		var lineBuffer bytes.Buffer
		var readErr error
		startTime := time.Now()
		readTimeout := 1 * time.Minute // Safety timeout for the read loop itself

		t.logger.Debug("TCPTransport: Goroutine: Starting ReadByte loop...")
		for {
			// Check for internal timeout
			if time.Since(startTime) > readTimeout {
				readErr = fmt.Errorf("internal ReadByte loop timeout after %v", readTimeout)
				break
			}
			// Read one byte
			b, err := byteReader.ReadByte()
			if err != nil {
				readErr = err // Store the error (e.g., EOF)
				if readErr == io.EOF && lineBuffer.Len() > 0 {
					t.logger.Warn("TCPTransport: ReadByte loop: Reached EOF with partial line data.")
					readErr = nil // Treat as success for the partial line
				} else if readErr == io.EOF {
					t.logger.Info("TCPTransport: ReadByte loop: Received EOF.")
				} else {
					t.logger.Error("TCPTransport: ReadByte loop: Error reading byte: %v", err)
				}
				break // Exit loop on any error or EOF
			}

			lineBuffer.WriteByte(b)

			if b == '\n' {
				break // Exit loop once newline is found
			}
		}
		t.logger.Debug("TCPTransport: Goroutine: ReadByte loop finished.")

		if readErr != nil && readErr != io.EOF {
			resultCh <- struct {
				data []byte
				err  error
			}{nil, fmt.Errorf("failed to read message line: %w", readErr)}
			return
		}

		lineBytes := lineBuffer.Bytes()
		if len(lineBytes) == 0 && readErr == io.EOF {
			resultCh <- struct {
				data []byte
				err  error
			}{nil, io.EOF}
			return
		}

		t.logger.Debug("TCPTransport Received raw line: %s", string(lineBytes))

		if !json.Valid(lineBytes) {
			t.logger.Error("TCPTransport: Received invalid JSON: %s", string(lineBytes))
			// Pass background context to internal error reporting
			_ = t.sendParseError(context.Background(), "Received invalid JSON")
			resultCh <- struct {
				data []byte
				err  error
			}{nil, fmt.Errorf("received invalid JSON")}
			return
		}

		resultCh <- struct {
			data []byte
			err  error
		}{lineBytes, nil}
	}()

	// Wait for result or context cancellation
	select {
	case result := <-resultCh:
		return result.data, result.err
	case <-ctx.Done():
		t.logger.Warn("TCPTransport: Receive operation canceled: %v", ctx.Err())
		_ = t.Close() // Close connection to attempt to unblock reader goroutine
		return nil, ctx.Err()
	}
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
		Error: &protocol.ErrorPayload{Code: protocol.ErrorCodeParseError, Message: message},
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
