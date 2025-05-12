package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/types"
)

// stdioTransport implements ClientTransport using stdio
type stdioTransport struct {
	command       string
	args          []string
	options       *TransportOptions
	logger        logx.Logger
	notifyHandler NotificationHandler

	transport    types.Transport
	cmd          *exec.Cmd
	connected    bool
	connMutex    sync.RWMutex
	responseMap  *sync.Map // map[string]chan *protocol.JSONRPCResponse
	notifyBuffer chan *protocol.JSONRPCNotification

	// Context for managing the process lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(command string, args []string, logger logx.Logger, options ...TransportOption) (ClientTransport, error) {
	// Apply transport options
	opts := DefaultTransportOptions()
	for _, option := range options {
		option(opts)
	}

	t := &stdioTransport{
		command:      command,
		args:         args,
		options:      opts,
		logger:       logger,
		connected:    false,
		responseMap:  &sync.Map{},
		notifyBuffer: make(chan *protocol.JSONRPCNotification, 100),
	}

	// Create root context that will be used to manage the process
	t.ctx, t.cancel = context.WithCancel(context.Background())

	return t, nil
}

// Connect starts the child process and establishes communication
func (t *stdioTransport) Connect(ctx context.Context) error {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()

	if t.connected {
		return NewConnectionError("stdio", "already connected", ErrAlreadyConnected)
	}

	// Create the command
	t.cmd = exec.CommandContext(ctx, t.command, t.args...)

	// Set up stdin/stdout
	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return NewTransportError("stdio", "failed to create stdin pipe", err)
	}

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return NewTransportError("stdio", "failed to create stdout pipe", err)
	}

	// Optionally redirect stderr to our logger
	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return NewTransportError("stdio", "failed to create stderr pipe", err)
	}

	// Create transport options for stdio
	stdinWriter := stdin
	stdoutReader := stdout

	// Create transport options for the stdio transport
	wsOpts := types.TransportOptions{
		Logger: t.logger,
	}

	// Create the stdio transport
	t.logger.Info("Creating stdio transport for command: %s", t.command)
	t.transport = stdio.NewStdioTransportWithReadWriter(stdoutReader, stdinWriter, wsOpts)

	// Start the process
	if err := t.cmd.Start(); err != nil {
		return NewTransportError("stdio", fmt.Sprintf("failed to start process %s", t.command), err)
	}

	// Start a goroutine to capture and log stderr output
	go func() {
		defer stderr.Close()
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				if err != io.EOF {
					t.logger.Error("Error reading stderr: %v", err)
				}
				return
			}
			if n > 0 {
				t.logger.Info("stderr: %s", string(buf[:n]))
			}
		}
	}()

	t.connected = true

	// Start a goroutine to handle process completion
	go func() {
		if err := t.cmd.Wait(); err != nil {
			if ctx.Err() == nil { // Only log if not cancelled by context
				t.logger.Error("Process exited with error: %v", err)
			}
		}
		t.Close() // Ensure transport is closed when process exits
	}()

	// Start a goroutine to handle incoming messages
	go t.receiveLoop()

	t.logger.Info("Stdio transport connected to process %s", t.command)
	return nil
}

// Close terminates the child process and cleans up resources
func (t *stdioTransport) Close() error {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()

	if !t.connected {
		return nil
	}

	// Cancel context to stop all goroutines
	t.cancel()

	// Close the transport
	if err := t.transport.Close(); err != nil {
		t.logger.Error("Failed to close stdio transport: %v", err)
	}

	// Kill the process if it's still running
	if t.cmd != nil && t.cmd.Process != nil {
		// Try to terminate gracefully first
		if err := t.cmd.Process.Signal(os.Interrupt); err != nil {
			t.logger.Warn("Failed to send interrupt signal to process: %v", err)
			// Force kill if graceful termination fails
			if err := t.cmd.Process.Kill(); err != nil {
				t.logger.Error("Failed to kill process: %v", err)
			}
		}
	}

	// Close any pending response channels
	t.responseMap.Range(func(key, value interface{}) bool {
		if ch, ok := value.(chan *protocol.JSONRPCResponse); ok {
			close(ch)
		}
		t.responseMap.Delete(key)
		return true
	})

	t.connected = false
	t.logger.Info("Stdio transport disconnected from process %s", t.command)

	return nil
}

// IsConnected returns true if the transport is connected
func (t *stdioTransport) IsConnected() bool {
	t.connMutex.RLock()
	defer t.connMutex.RUnlock()
	return t.connected && !t.transport.IsClosed()
}

// SendRequest sends a request to the server and waits for a response
func (t *stdioTransport) SendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
	if !t.IsConnected() {
		return nil, NewConnectionError("stdio", "not connected", ErrNotConnected)
	}

	// Marshal the request to JSON
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, NewTransportError("stdio", "failed to marshal request", err)
	}

	// Create a response channel for this request
	responseCh := make(chan *protocol.JSONRPCResponse, 1)
	defer close(responseCh)

	// Register the response channel
	reqID := fmt.Sprintf("%v", req.ID)
	t.responseMap.Store(reqID, responseCh)
	defer t.responseMap.Delete(reqID)

	// Send the request
	// In stdio protocol, newline is required, but the underlying transport should handle this
	if err := t.transport.Send(ctx, reqData); err != nil {
		return nil, NewTransportError("stdio", "failed to send request", err)
	}

	// Wait for response or timeout
	select {
	case resp := <-responseCh:
		return resp, nil
	case <-ctx.Done():
		return nil, NewTimeoutError("SendRequest", t.options.RequestTimeout, ctx.Err())
	}
}

// SendRequestAsync sends a request to the server without waiting for a response
func (t *stdioTransport) SendRequestAsync(ctx context.Context, req *protocol.JSONRPCRequest, responseCh chan<- *protocol.JSONRPCResponse) error {
	if !t.IsConnected() {
		return NewConnectionError("stdio", "not connected", ErrNotConnected)
	}

	// Marshal the request to JSON
	reqData, err := json.Marshal(req)
	if err != nil {
		return NewTransportError("stdio", "failed to marshal request", err)
	}

	// Register the response channel if not nil
	if responseCh != nil {
		reqID := fmt.Sprintf("%v", req.ID)
		t.responseMap.Store(reqID, responseCh)
	}

	// Send the request
	if err := t.transport.Send(ctx, reqData); err != nil {
		// Clean up the response channel registration on error
		if responseCh != nil {
			reqID := fmt.Sprintf("%v", req.ID)
			t.responseMap.Delete(reqID)
		}
		return NewTransportError("stdio", "failed to send request", err)
	}

	return nil
}

// SetNotificationHandler sets the handler for incoming server notifications
func (t *stdioTransport) SetNotificationHandler(handler NotificationHandler) {
	t.notifyHandler = handler
}

// GetTransportType returns the transport type
func (t *stdioTransport) GetTransportType() TransportType {
	return TransportTypeStdio
}

// GetTransportInfo returns transport-specific information
func (t *stdioTransport) GetTransportInfo() map[string]interface{} {
	return map[string]interface{}{
		"command":   t.command,
		"args":      t.args,
		"connected": t.IsConnected(),
		"pid":       t.getPID(),
	}
}

// getPID returns the process ID if available
func (t *stdioTransport) getPID() int {
	t.connMutex.RLock()
	defer t.connMutex.RUnlock()

	if t.cmd != nil && t.cmd.Process != nil {
		return t.cmd.Process.Pid
	}
	return 0
}

// receiveLoop handles incoming messages from the stdio transport
func (t *stdioTransport) receiveLoop() {
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// Create a context with timeout for the receive operation
			ctx, cancel := context.WithTimeout(t.ctx, t.options.ReadTimeout)
			data, err := t.transport.Receive(ctx)
			cancel()

			if err != nil {
				// Check if the context was cancelled or closed intentionally
				if t.ctx.Err() != nil || t.transport.IsClosed() {
					return
				}

				t.logger.Error("Error receiving from stdio transport: %v", err)

				// Check for EOF (process closed its stdout)
				if err == io.EOF {
					t.logger.Warn("Process closed stdout, closing connection")
					t.Close()
					return
				}

				// For non-fatal errors, continue the loop
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Skip empty messages
			if len(data) == 0 {
				continue
			}

			// Try to parse the message as a JSON-RPC response or notification
			var message baseJSONRPCMessage
			if err := json.Unmarshal(data, &message); err != nil {
				t.logger.Error("Failed to parse stdio message: %v", err)
				continue
			}

			// Check if it's a response (has ID)
			if message.ID != nil {
				var response protocol.JSONRPCResponse
				if err := json.Unmarshal(data, &response); err != nil {
					t.logger.Error("Failed to parse stdio response: %v", err)
					continue
				}

				// Find the response channel
				id := fmt.Sprintf("%v", response.ID)
				if ch, ok := t.responseMap.Load(id); ok {
					if respCh, validCh := ch.(chan *protocol.JSONRPCResponse); validCh {
						select {
						case respCh <- &response:
							// Response sent successfully
						default:
							t.logger.Warn("Response channel for ID %v is full or closed", id)
						}
					}
				} else {
					t.logger.Debug("No handler found for response ID: %v", id)
				}
			} else if message.Method != "" {
				// It's a notification
				var notification protocol.JSONRPCNotification
				if err := json.Unmarshal(data, &notification); err != nil {
					t.logger.Error("Failed to parse stdio notification: %v", err)
					continue
				}

				// Process the notification
				if t.notifyHandler != nil {
					if err := t.notifyHandler(&notification); err != nil {
						t.logger.Error("Notification handler error: %v", err)
					}
				} else {
					// Buffer the notification for later processing
					select {
					case t.notifyBuffer <- &notification:
						// Notification buffered successfully
					default:
						t.logger.Warn("Notification buffer is full, dropping notification")
					}
				}
			} else {
				t.logger.Warn("Received message is neither a response nor a notification: %s", string(data))
			}
		}
	}
}
