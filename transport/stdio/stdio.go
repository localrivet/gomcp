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
// It reads messages from stdin using a dedicated reader goroutine and writes messages to stdout.
type StdioTransport struct {
	reader     io.Reader // Underlying reader (e.g., os.Stdin)
	writer     io.Writer // Underlying writer (e.g., os.Stdout)
	writeMutex sync.Mutex
	logger     types.Logger
	closed     bool
	closeMutex sync.Mutex

	// Store original streams for potential closing
	rawReader io.Reader
	rawWriter io.Writer

	// For persistent reader goroutine
	messageChan     chan messageOrError // Channel for read messages/errors
	startReaderOnce sync.Once           // Ensures reader goroutine starts only once
	readerCtx       context.Context     // Context to manage reader goroutine lifecycle
	readerCancel    context.CancelFunc  // Function to cancel the reader context
}

// messageOrError holds either a received message or an error from the reader goroutine.
type messageOrError struct {
	data []byte
	err  error
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

	// Keep original writer for potential closing
	rawWriter := writer

	// Wrap stdout/stderr with a buffered writer for reliable flushing
	if f, ok := writer.(*os.File); ok && (f == os.Stdout || f == os.Stderr) {
		writer = bufio.NewWriter(writer)
	}

	// Create a context to manage the reader goroutine's lifecycle
	readerCtx, readerCancel := context.WithCancel(context.Background())

	t := &StdioTransport{
		reader:       reader,
		writer:       writer, // This might be the buffered writer now
		logger:       logger,
		rawReader:    reader,
		rawWriter:    rawWriter, // Store the original writer
		closed:       false,
		messageChan:  make(chan messageOrError, 1), // Buffered channel
		readerCtx:    readerCtx,
		readerCancel: readerCancel,
	}

	// Automatically start the reader goroutine when the transport is created.
	// This is generally safe as StdioTransport usually implies a dedicated process.
	// Alternatively, use startReaderOnce in ReceiveWithContext if lazy start is needed.
	go t.startReader()

	return t
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

	// Attempt to flush if the writer supports it (like bufio.Writer)
	if flusher, ok := t.writer.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			t.logger.Warn("StdioTransport: Failed to flush writer: %v", err)
		}
	}

	return nil
}

// Receive reads the next newline-delimited message from the underlying reader (stdin).
// It blocks until a message is received or an error occurs.
func (t *StdioTransport) Receive() ([]byte, error) {
	return t.ReceiveWithContext(context.Background())
}

// startReader launches the persistent goroutine that reads from stdin,
// validates messages, and sends them to the messageChan.
func (t *StdioTransport) startReader() {
	defer close(t.messageChan) // Ensure channel is closed when reader exits

	scanner := bufio.NewScanner(t.reader)
	// Configure scanner if needed (e.g., buffer size)
	// const maxMessageSize = 1024 * 1024 // 1MB example
	// buf := make([]byte, maxMessageSize)
	// scanner.Buffer(buf, maxMessageSize)

	t.logger.Info("StdioTransport: Reader goroutine started.")

	for {
		// Check for cancellation signal before blocking on Scan
		select {
		case <-t.readerCtx.Done():
			t.logger.Info("StdioTransport: Reader goroutine stopping due to context cancellation.")
			// Send the cancellation error to ensure ReceiveWithContext unblocks if waiting
			// Avoid sending if channel is already potentially closed or blocked
			// A non-blocking send attempt might be safer here, but complicates logic.
			// Relying on channel closure is generally sufficient.
			return
		default:
			// Continue to Scan
		}

		// Blocking call to read the next line
		scanned := scanner.Scan()

		if !scanned {
			// Scan returned false, check for errors or EOF
			err := scanner.Err()
			if err != nil {
				// Check if the error is due to the reader being closed (expected on Close())
				// or context cancellation during scan
				if errors.Is(err, os.ErrClosed) || errors.Is(err, io.ErrClosedPipe) || strings.Contains(err.Error(), "file already closed") || errors.Is(err, context.Canceled) {
					t.logger.Info("StdioTransport: Reader goroutine stopping due to closed reader or context cancellation: %v", err)
					// Don't send error, signal clean closure via closed channel
				} else {
					t.logger.Error("StdioTransport: Scanner error: %v", err)
					// Attempt to send the error non-blockingly in case channel is full/closed
					select {
					case t.messageChan <- messageOrError{err: fmt.Errorf("scanner error: %w", err)}:
					default:
						t.logger.Warn("StdioTransport: Failed to send scanner error to channel (full or closed).")
					}
				}
			} else {
				// If Scan returned false and scanner.Err() is nil, it's EOF
				t.logger.Info("StdioTransport: Reached EOF.")
				// Attempt to send EOF non-blockingly
				select {
				case t.messageChan <- messageOrError{err: io.EOF}: // Signal clean EOF
				default:
					t.logger.Warn("StdioTransport: Failed to send EOF to channel (full or closed).")
				}
			}
			return // Exit reader loop
		}

		// Got a line
		lineBytes := scanner.Bytes() // Get the line bytes (without newline)
		// Make a copy because scanner.Bytes() buffer can be overwritten by next Scan()
		lineCopy := make([]byte, len(lineBytes))
		copy(lineCopy, lineBytes)

		// Append the newline character back for consistency.
		lineCopy = append(lineCopy, '\n')

		t.logger.Debug("StdioTransport Received raw line: %s", string(lineCopy))

		// Basic JSON validation
		if !json.Valid(lineCopy) {
			t.logger.Error("StdioTransport: Received invalid JSON: %s", string(lineCopy))
			_ = t.sendParseError("Received invalid JSON") // Attempt to notify other side
			// Don't send this invalid message to the channel, just log and continue reading.
			continue
		}

		// Send the valid message
		select {
		case t.messageChan <- messageOrError{data: lineCopy, err: nil}:
			// Message sent successfully
		case <-t.readerCtx.Done():
			t.logger.Info("StdioTransport: Reader goroutine stopping due to context cancellation while sending.")
			return
		}
	}
}

// ReceiveWithContext reads the next available message from the internal channel,
// waiting if necessary. It respects the provided context for cancellation.
func (t *StdioTransport) ReceiveWithContext(ctx context.Context) ([]byte, error) {
	// Note: Reader goroutine is started in the constructor.

	t.closeMutex.Lock()
	isClosed := t.closed
	t.closeMutex.Unlock()
	// Check if closed *before* waiting on channel. Avoids race if Close happens
	// between this check and the select statement. The select handles closure
	// during the wait.
	if isClosed {
		return nil, fmt.Errorf("transport is closed")
	}

	select {
	case <-ctx.Done():
		t.logger.Warn("StdioTransport: Receive context canceled: %v", ctx.Err())
		return nil, ctx.Err()
	case msg, ok := <-t.messageChan:
		if !ok {
			// Channel closed, means reader stopped (EOF, error, or Close called).
			t.logger.Info("StdioTransport: Message channel closed.")
			// Ensure transport state reflects closure if not already set
			t.closeMutex.Lock()
			alreadyClosed := t.closed
			if !alreadyClosed {
				t.closed = true // Mark as closed if channel closure is the first indication
			}
			t.closeMutex.Unlock()
			// Return EOF or a specific closed error
			return nil, io.EOF // Consistent with reader goroutine sending EOF
		}
		// Return the message data or error received from the channel
		if msg.err != nil && !errors.Is(msg.err, io.EOF) { // Don't log EOF as an error here
			t.logger.Error("StdioTransport: Received error from reader channel: %v", msg.err)
		}
		// If EOF is received, mark transport as closed
		if errors.Is(msg.err, io.EOF) {
			t.closeMutex.Lock()
			t.closed = true
			t.closeMutex.Unlock()
		}
		return msg.data, msg.err
	}
}

// Close signals the reader goroutine to stop and attempts to close the underlying reader/writer.
func (t *StdioTransport) Close() error {
	t.closeMutex.Lock()
	if t.closed {
		t.closeMutex.Unlock()
		return nil // Already closed
	}
	// Mark as closed immediately inside the lock
	t.closed = true
	t.logger.Info("StdioTransport: Closing...")
	t.closeMutex.Unlock() // Unlock after marking closed

	// Signal the reader goroutine to stop by canceling its context
	// This should happen before closing the rawReader to avoid race conditions
	// where the reader goroutine tries to read from a closed reader.
	t.readerCancel()

	// Closing the rawReader is crucial to unblock the scanner.Scan() call
	// in the reader goroutine if it's currently blocked.
	var firstErr error
	if closer, ok := t.rawReader.(io.Closer); ok {
		t.logger.Debug("StdioTransport: Closing reader...")
		if err := closer.Close(); err != nil {
			// Ignore errors that indicate the reader is already closed or pipe broken,
			// as these might be expected consequences of concurrent closure or the other end closing.
			if !errors.Is(err, os.ErrClosed) && !errors.Is(err, io.ErrClosedPipe) && !strings.Contains(err.Error(), "file already closed") {
				t.logger.Error("StdioTransport: Error closing reader: %v", err)
				firstErr = err // Record the first significant error
			}
		}
	}

	// Close writer
	if closer, ok := t.rawWriter.(io.Closer); ok {
		t.logger.Debug("StdioTransport: Closing writer...")
		if err := closer.Close(); err != nil {
			if !errors.Is(err, os.ErrClosed) && !errors.Is(err, io.ErrClosedPipe) && !strings.Contains(err.Error(), "file already closed") {
				t.logger.Error("StdioTransport: Error closing writer: %v", err)
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}

	// Drain the message channel after closing reader/writer and canceling context.
	// This helps ensure ReceiveWithContext doesn't block indefinitely if called after Close.
	go func() {
		for range t.messageChan {
			// Discard any messages buffered or sent before closure was fully processed
		}
		t.logger.Debug("StdioTransport: Message channel drained.")
	}()

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
