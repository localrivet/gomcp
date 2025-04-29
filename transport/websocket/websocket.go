// Package websocket provides a types.Transport implementation using WebSockets.
package websocket

import (
	"context"
	"errors"
	"fmt"
	"io"  // For io.EOF
	"log" // Added for default logger
	"net" // For net.Conn, net.Addr
	"sync"
	"time" // For deadlines

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil" // For reader/writer helpers
	"github.com/localrivet/gomcp/types"
)

// WebSocketTransport implements the Transport interface using a gobwas/ws connection over net.Conn.
type WebSocketTransport struct {
	conn       net.Conn // Underlying network connection
	state      ws.State // Client or Server state for masking
	writeMutex sync.Mutex
	logger     types.Logger
	closed     bool
	closeMutex sync.Mutex
}

// Ensure WebSocketTransport implements types.Transport
// var _ types.Transport = (*WebSocketTransport)(nil) // Will be updated after methods are adjusted

// NewWebSocketTransport creates a new WebSocketTransport wrapping an existing net.Conn.
// The state determines if outgoing frames should be masked (ws.StateClient).
func NewWebSocketTransport(conn net.Conn, state ws.State, opts types.TransportOptions) *WebSocketTransport {
	logger := opts.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	logger.Info("Creating new WebSocketTransport (State: %v) for connection: %s -> %s", state, conn.LocalAddr(), conn.RemoteAddr())

	// TODO: Configure Ping/Pong handlers? Read/Write deadlines?

	return &WebSocketTransport{
		conn:   conn,
		state:  state,
		logger: logger,
		closed: false,
	}
}

// Send writes a message to the WebSocket connection as a TextMessage.
// Assumes data is a complete JSON message. Context is used for cancellation/deadlines.
func (t *WebSocketTransport) Send(ctx context.Context, data []byte) error {
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

	// Note: gobwas/ws writes raw frames. Newline handling should be done by
	// the application layer before calling Send, if required by the protocol framing.
	// MCP uses newline-delimited JSON, so the caller of Send should ensure data ends with '\n'.
	t.logger.Debug("WebSocketTransport Send: %s", string(data))

	// Set write deadline based on context if available
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		if err := t.conn.SetWriteDeadline(deadline); err != nil {
			t.logger.Warn("WebSocketTransport: Failed to set write deadline from context: %v", err)
			// Continue anyway, but log the warning
		}
	} else {
		// Apply a default reasonable deadline if context has none
		defaultDeadline := time.Now().Add(30 * time.Second) // Default 30s
		if err := t.conn.SetWriteDeadline(defaultDeadline); err != nil {
			t.logger.Warn("WebSocketTransport: Failed to set default write deadline: %v", err)
		}
	}

	// Use wsutil.WriteMessage for simplicity (handles framing)
	// Need to specify client/server state for masking
	err := wsutil.WriteMessage(t.conn, t.state, ws.OpText, data)

	// Alternative: Manual frame writing
	// frame := ws.NewTextFrame(data)
	// if t.state == ws.StateClient { // Mask client-to-server frames
	// 	ws.MaskFrameInPlace(frame)
	// }
	// err := ws.WriteFrame(t.conn, frame)

	if err != nil {
		t.logger.Error("WebSocketTransport: Failed to write message: %v", err)
		_ = t.Close() // Attempt to close on write error
		return fmt.Errorf("failed to write websocket message: %w", err)
	}

	// Reset deadline after write attempt (success or failure)
	if err := t.conn.SetWriteDeadline(time.Time{}); err != nil {
		// Don't return error here, just log it. The primary error is the write error (if any).
		t.logger.Warn("WebSocketTransport: Failed to reset write deadline: %v", err)
	}

	t.logger.Debug("WebSocketTransport Send: Write completed.")
	return nil
}

// Receive reads the next message from the WebSocket connection, respecting context.
// This replaces the old Receive and ReceiveWithContext methods.
// Uses read deadlines derived from the context for cancellation.
func (t *WebSocketTransport) Receive(ctx context.Context) ([]byte, error) {
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
			t.logger.Warn("WebSocketTransport: Failed to set read deadline from context: %v", err)
			// If we can't set deadline, context cancellation might not work reliably via this mechanism
		}
		// Ensure deadline is reset when function returns
		defer func() {
			if err := t.conn.SetReadDeadline(time.Time{}); err != nil {
				t.logger.Warn("WebSocketTransport: Failed to reset read deadline: %v", err)
			}
		}()
	}
	// If no deadline, read will block indefinitely until data arrives, connection closes, or Close() is called.

	t.logger.Debug("WebSocketTransport: Attempting ws.ReadFrame()...")

	// Read a single frame
	// Note: For fragmented messages, need wsutil.Reader or loop with ws.ReadHeader/io.ReadFull
	// Assuming MCP messages fit in single frames for now.
	frame, err := ws.ReadFrame(t.conn)
	if err != nil {
		// Check if the error is due to context deadline
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			// Check if the timeout corresponds to our context deadline
			if hasDeadline && time.Now().After(deadline.Add(-time.Millisecond*50)) { // Allow small grace period
				t.logger.Warn("WebSocketTransport: Read timed out due to context deadline.")
				return nil, context.DeadlineExceeded // Return context error
			}
			// Otherwise, it might be a standard network timeout
			t.logger.Error("WebSocketTransport: Read timed out (network): %v", err)
		} else if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
			t.logger.Info("WebSocketTransport: Connection closed while reading: %v", err)
		} else {
			t.logger.Error("WebSocketTransport: Error reading frame: %v", err)
		}
		_ = t.Close()                                                     // Ensure closed state is set on any read error
		return nil, fmt.Errorf("failed to read websocket frame: %w", err) // Return the original error
	}

	t.logger.Debug("WebSocketTransport: ws.ReadFrame() returned OpCode: %v, Len: %d", frame.Header.OpCode, frame.Header.Length)

	// Handle control frames (like Close)
	if frame.Header.OpCode.IsControl() {
		if frame.Header.OpCode == ws.OpClose {
			t.logger.Info("WebSocketTransport: Received Close frame.")
			_ = t.Close()      // Acknowledge close and close self
			return nil, io.EOF // Signal closure to caller
		}
		// Handle Ping/Pong if necessary, or ignore other control frames
		t.logger.Debug("WebSocketTransport: Received control frame: %v", frame.Header.OpCode)
		// Continue reading for the next data frame
		// Recursive call might be dangerous, use a loop instead if strict handling needed
		// Loop to read the next frame instead of recursion
		// This requires restructuring the read logic slightly, or simply returning
		// an indicator that a control frame was processed. For simplicity,
		// let's just log and try reading again in the next call to Receive.
		// A better approach would involve a dedicated read loop goroutine.
		// For now, return a temporary error or nil to prompt caller to retry?
		// Let's return nil, nil and log it. The caller should ideally loop.
		t.logger.Debug("Processed control frame, caller should retry Receive.")
		return nil, nil // Indicate non-data frame received, caller should retry
	}

	// Unmask payload if necessary (client state)
	if frame.Header.Masked {
		ws.Cipher(frame.Payload, frame.Header.Mask, 0)
	}

	// We expect Text messages containing JSON for MCP
	if frame.Header.OpCode != ws.OpText {
		t.logger.Warn("WebSocketTransport: Received non-text message type: %v", frame.Header.OpCode)
		return nil, fmt.Errorf("received unexpected websocket message type: %v", frame.Header.OpCode)
	}

	// Note: MCP uses newline-delimited JSON. ws.ReadFrame reads a whole WebSocket frame.
	// If a frame contains multiple JSON messages, or a JSON message spans multiple frames,
	// this simple implementation will break. A more robust solution would use wsutil.Reader
	// and read until newline, handling fragmentation.
	// For now, assume one JSON message per Text frame.
	data := frame.Payload

	t.logger.Debug("WebSocketTransport Received raw: %s", string(data))

	return data, nil
}

// EstablishReceiver for WebSocketTransport does nothing, as the connection
// is assumed to be established before the transport is created, and reading
// happens within the Receive method. It exists to satisfy the Transport interface.
func (t *WebSocketTransport) EstablishReceiver(ctx context.Context) error {
	t.logger.Debug("WebSocketTransport: EstablishReceiver called (no-op).")
	// Check if already closed
	t.closeMutex.Lock()
	defer t.closeMutex.Unlock()
	if t.closed {
		return fmt.Errorf("transport is closed")
	}
	return nil
}

// Close sends a close frame and closes the underlying WebSocket connection.
func (t *WebSocketTransport) Close() error {
	t.closeMutex.Lock()
	defer t.closeMutex.Unlock()

	if t.closed {
		return nil // Already closed
	}
	t.closed = true
	t.logger.Info("WebSocketTransport: Closing connection: %s -> %s", t.conn.LocalAddr(), t.conn.RemoteAddr())

	// Attempt to send a normal close frame first.
	// Use a short deadline.
	deadline := time.Now().Add(2 * time.Second)
	_ = t.conn.SetWriteDeadline(deadline)
	closeFrame := ws.NewCloseFrame(ws.NewCloseFrameBody(ws.StatusNormalClosure, ""))
	// Mask if client is sending
	if t.state == ws.StateClientSide { // Correct constant name
		ws.MaskFrameInPlace(closeFrame)
	}
	err := ws.WriteFrame(t.conn, closeFrame)
	if err != nil {
		t.logger.Warn("WebSocketTransport: Failed to send close frame: %v", err)
	}
	_ = t.conn.SetWriteDeadline(time.Time{}) // Reset deadline

	// Close the underlying connection regardless.
	err = t.conn.Close() // Use the underlying net.Conn
	if err != nil {
		t.logger.Error("WebSocketTransport: Error closing underlying connection: %v", err)
	} else {
		t.logger.Info("WebSocketTransport: Connection closed successfully.")
	}
	// Return the first significant error encountered during close
	return err
}

// RemoteAddr returns the remote network address.
func (t *WebSocketTransport) RemoteAddr() net.Addr {
	return t.conn.RemoteAddr()
}

// LocalAddr returns the local network address.
func (t *WebSocketTransport) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}

// IsClosed returns true if the transport connection is closed.
func (t *WebSocketTransport) IsClosed() bool {
	t.closeMutex.Lock()
	defer t.closeMutex.Unlock()
	return t.closed
}

// --- Default Logger (Placeholder) ---
type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, args ...interface{}) { log.Printf("DEBUG: "+msg, args...) }
func (l *defaultLogger) Info(msg string, args ...interface{})  { log.Printf("INFO: "+msg, args...) }
func (l *defaultLogger) Warn(msg string, args ...interface{})  { log.Printf("WARN: "+msg, args...) }
func (l *defaultLogger) Error(msg string, args ...interface{}) { log.Printf("ERROR: "+msg, args...) }

var _ types.Transport = (*WebSocketTransport)(nil) // Ensure interface compliance
