// Package stdio provides a Transport implementation that uses standard input/output.
package stdio

import (
	"bufio"
	"bytes" // Needed for byte buffer
	"context"
	"encoding/json" // Needed for json.Valid
	"errors"        // Added for error checking
	"fmt"
	"io"
	"log" // Added import for default logger
	"os"
	"strings" // Added for error checking
	"sync"

	// "time" // No longer needed directly in ReceiveWithContext

	"github.com/localrivet/gomcp/protocol" // For ErrorPayload, ErrorCodeParseError
	"github.com/localrivet/gomcp/types"    // For types.Transport, types.Logger
)

// StdioTransport implements the Transport interface using standard input/output.
// It reads messages from stdin and writes messages to stdout.
type StdioTransport struct {
	reader     io.Reader // Use raw reader
	writer     io.Writer
	writeMutex sync.Mutex
	logger     types.Logger
	closed     bool
	closeMutex sync.Mutex

	// Store original streams for potential closing
	rawReader io.Reader
	rawWriter io.Writer
}

// NewStdioTransport creates a new StdioTransport with default options.
func NewStdioTransport() *StdioTransport {
	return NewStdioTransportWithOptions(types.TransportOptions{})
}

// NewStdioTransportWithOptions creates a new StdioTransport with the specified options.
func NewStdioTransportWithOptions(opts types.TransportOptions) *StdioTransport {
	return NewStdioTransportWithReadWriter(os.Stdin, os.Stdout, opts)
}

// NewStdioTransportWithReadWriter creates a new StdioTransport using the provided reader/writer.
func NewStdioTransportWithReadWriter(reader io.Reader, writer io.Writer, opts types.TransportOptions) *StdioTransport {
	logger := opts.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}
	logger.Info("Creating new StdioTransport")

	return &StdioTransport{
		reader:    reader,
		writer:    writer,
		logger:    logger,
		rawReader: reader,
		rawWriter: writer,
		closed:    false,
	}
}

// Send writes a message to the underlying writer (stdout).
// It ensures the message ends with a newline and handles locking and flushing.
func (t *StdioTransport) Send(data []byte) error {
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
	// Ensure data ends with exactly one newline
	data = bytes.TrimRight(data, "\n")
	data = append(data, '\n')

	t.logger.Debug("StdioTransport Send: %s", string(data))

	_, err := t.writer.Write(data)
	if err != nil {
		// Check if it's a pipe closed error, which might be expected
		if errors.Is(err, io.ErrClosedPipe) || strings.Contains(err.Error(), "pipe closed") {
			t.logger.Warn("StdioTransport: Attempted to write to closed pipe: %v", err)
			// Close the transport from this end if write fails due to pipe closure
			_ = t.Close()
			return err // Return the original error
		}
		t.logger.Error("StdioTransport: Failed to write message: %v", err)
		return fmt.Errorf("failed to write message: %w", err)
	}

	// Attempt to flush
	if flusher, ok := t.writer.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			t.logger.Warn("StdioTransport: Failed to flush writer: %v", err)
		}
	} else if f, ok := t.writer.(*os.File); ok && (f == os.Stdout || f == os.Stderr) {
		if err := f.Sync(); err != nil {
			t.logger.Warn("StdioTransport: Failed to sync writer (%s): %v", f.Name(), err)
		}
	}

	return nil
}

// Receive reads the next newline-delimited message from the underlying reader (stdin).
// It blocks until a message is received or an error occurs.
func (t *StdioTransport) Receive() ([]byte, error) {
	return t.ReceiveWithContext(context.Background())
}

// ReceiveWithContext reads a message from stdin with context support.
// It runs the blocking read loop in a goroutine and uses select to wait for
// the result, an error, or context cancellation.
func (t *StdioTransport) ReceiveWithContext(ctx context.Context) ([]byte, error) {
	t.closeMutex.Lock()
	if t.closed {
		t.closeMutex.Unlock()
		return nil, fmt.Errorf("transport is closed")
	}
	t.closeMutex.Unlock()

	type result struct {
		data []byte
		err  error
	}
	resultChan := make(chan result, 1)

	// Goroutine to perform the blocking read
	go func() {
		var byteReader io.ByteReader
		if br, ok := t.reader.(io.ByteReader); ok {
			byteReader = br
		} else {
			byteReader = bufio.NewReaderSize(t.reader, 1)
		}

		var lineBuffer bytes.Buffer
		for {
			b, err := byteReader.ReadByte() // Blocking call
			if err != nil {
				// Handle EOF: return data if buffer has content, otherwise return EOF error
				if err == io.EOF {
					if lineBuffer.Len() > 0 {
						t.logger.Warn("StdioTransport: Reached EOF with partial line data.")
						resultChan <- result{data: lineBuffer.Bytes(), err: nil} // Return partial data on EOF
					} else {
						resultChan <- result{err: io.EOF} // Signal clean EOF
					}
				} else {
					// Other read errors
					t.logger.Error("StdioTransport: Error reading byte: %v", err)
					resultChan <- result{err: fmt.Errorf("failed to read message line: %w", err)}
				}
				return // Exit goroutine
			}

			lineBuffer.WriteByte(b)

			// Check if we reached the delimiter
			if b == '\n' {
				lineBytes := lineBuffer.Bytes()
				t.logger.Debug("StdioTransport Received raw line: %s", string(lineBytes))

				// Basic JSON validation
				if !json.Valid(lineBytes) {
					t.logger.Error("StdioTransport: Received invalid JSON: %s", string(lineBytes))
					_ = t.sendParseError("Received invalid JSON") // Attempt to notify other side
					resultChan <- result{err: fmt.Errorf("received invalid JSON")}
				} else {
					resultChan <- result{data: lineBytes, err: nil} // Success
				}
				return // Exit goroutine
			}
			// If context is cancelled during the loop (between ReadByte calls),
			// the next ReadByte might still block, but the select below will catch it.
		}
	}()

	// Wait for result, error, or context cancellation
	select {
	case <-ctx.Done():
		t.logger.Warn("StdioTransport: Receive context canceled: %v", ctx.Err())
		// Attempt to close the transport to potentially unblock the reader goroutine if stuck
		_ = t.Close()
		return nil, ctx.Err()
	case res := <-resultChan:
		return res.data, res.err
	}
}

// Close attempts to close the underlying reader/writer if they implement io.Closer.
func (t *StdioTransport) Close() error {
	t.closeMutex.Lock()
	defer t.closeMutex.Unlock()

	if t.closed {
		return nil // Already closed
	}
	t.closed = true
	t.logger.Info("StdioTransport: Closing...")

	var firstErr error
	// Close writer if possible
	if closer, ok := t.rawWriter.(io.Closer); ok {
		t.logger.Debug("StdioTransport: Closing writer...")
		if err := closer.Close(); err != nil {
			// Ignore pipe closed errors as they are expected if reader closed first
			if !errors.Is(err, io.ErrClosedPipe) && !strings.Contains(err.Error(), "pipe closed") {
				t.logger.Error("StdioTransport: Error closing writer: %v", err)
				firstErr = err
			}
		}
	}
	// Close reader if possible
	if closer, ok := t.rawReader.(io.Closer); ok {
		t.logger.Debug("StdioTransport: Closing reader...")
		if err := closer.Close(); err != nil {
			// Ignore pipe closed errors as they are expected if writer closed first
			if !errors.Is(err, io.ErrClosedPipe) && !strings.Contains(err.Error(), "pipe closed") {
				t.logger.Error("StdioTransport: Error closing reader: %v", err)
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}

	if firstErr == nil {
		t.logger.Info("StdioTransport: Closed successfully.")
	}
	return firstErr
}

// sendParseError is a helper to send a JSON-RPC ParseError.
func (t *StdioTransport) sendParseError(message string) error {
	errResp := protocol.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      nil, // No ID for parse errors before request ID is known
		Error: &protocol.ErrorPayload{
			Code:    protocol.ErrorCodeParseError,
			Message: message,
		},
	}
	jsonData, err := json.Marshal(errResp)
	if err != nil {
		t.logger.Error("StdioTransport: Failed to marshal parse error response: %v", err)
		return err
	}
	// Use Send directly, ignoring potential errors during error reporting
	_ = t.Send(jsonData)
	return nil
}

// --- Default Logger ---

// defaultLogger provides a basic logger implementation if none is provided.
type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, args ...interface{}) { log.Printf("DEBUG: "+msg, args...) }
func (l *defaultLogger) Info(msg string, args ...interface{})  { log.Printf("INFO: "+msg, args...) }
func (l *defaultLogger) Warn(msg string, args ...interface{})  { log.Printf("WARN: "+msg, args...) }
func (l *defaultLogger) Error(msg string, args ...interface{}) { log.Printf("ERROR: "+msg, args...) }

var _ types.Logger = (*defaultLogger)(nil) // Ensure interface compliance
