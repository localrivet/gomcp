package gomcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
)

// StdioTransport implements the Transport interface for standard input/output.
type StdioTransport struct {
	conn   *Connection // Uses the existing Connection for stdio read/write
	server *Server     // Reference to the core server logic
	// Stdio assumes a single session, so no complex session management needed.
	// We can use a fixed session ID.
	sessionID string
}

// NewStdioTransport creates a new transport layer using stdio.
func NewStdioTransport() *StdioTransport {
	return &StdioTransport{
		conn:      NewStdioConnection(),
		sessionID: "stdio_session", // Fixed ID for the single stdio session
	}
}

// RegisterServer sets the core server instance.
func (t *StdioTransport) RegisterServer(server *Server) {
	t.server = server
}

// Start begins the main loop for reading messages from stdin.
// This replaces the old server.Run() method for stdio.
func (t *StdioTransport) Start() error {
	if t.server == nil {
		return fmt.Errorf("stdio transport cannot start: server not registered")
	}
	log.Println("StdioTransport: Starting main message loop...")

	// TODO: Handle initialization sequence here?
	// The original handleInitialize used s.conn directly.
	// We need a way to perform the handshake over this transport.
	// For now, assume initialization happens magically before this loop.
	// A proper implementation would need the transport to handle the
	// initialize request/response/initialized notification sequence.
	log.Println("StdioTransport: Skipping initialization sequence (TODO)")

	for {
		rawJSON, err := t.conn.ReceiveRawMessage()
		if err != nil {
			// Use errors.Is for robust EOF check, also check for common pipe errors
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || strings.Contains(err.Error(), "pipe closed") {
				log.Println("StdioTransport: Client disconnected (EOF or pipe error received). Shutting down.")
				t.Disconnect(t.sessionID, err) // Notify server of disconnect
				return nil                     // Clean exit
			}
			// Log and return unexpected errors
			log.Printf("StdioTransport: Error receiving message: %v. Shutting down.", err)
			t.Disconnect(t.sessionID, err) // Notify server of disconnect
			return err                     // Return the actual error
		}

		// Process the message using the core server logic
		// Use context.Background() for now, session context might be needed later
		processErr := t.server.ProcessRequest(context.Background(), t.sessionID, rawJSON)
		if processErr != nil {
			// ProcessRequest logs errors internally if it fails to marshal/send response.
			// If the error indicates a write failure, the connection is likely broken.
			if strings.Contains(processErr.Error(), "write") || strings.Contains(processErr.Error(), "pipe") || strings.Contains(processErr.Error(), "transport") {
				log.Printf("StdioTransport: Error processing request likely due to send failure: %v. Shutting down.", processErr)
				t.Disconnect(t.sessionID, processErr)
				return processErr // Propagate error indicating likely connection issue
			}
			// Log other processing errors but continue the loop
			log.Printf("StdioTransport: Error processing request: %v", processErr)
		}
	}
}

// Stop closes the underlying connection.
func (t *StdioTransport) Stop() error {
	log.Println("StdioTransport: Stopping...")
	if t.conn != nil {
		// Closing stdio might have unintended consequences, but let's try
		err := t.conn.Close()
		if err != nil {
			log.Printf("StdioTransport: Error closing connection: %v", err)
			return err
		}
	}
	return nil
}

// SendMessage sends a raw JSON message via the underlying connection.
func (t *StdioTransport) SendMessage(sessionID string, message []byte) error {
	if sessionID != t.sessionID {
		log.Printf("StdioTransport: Attempted to send message to unknown session %s", sessionID)
		return fmt.Errorf("invalid session ID for stdio transport: %s", sessionID)
	}
	if t.conn == nil {
		return fmt.Errorf("stdio transport connection is nil")
	}
	// Use the new SendRawMessage method on the Connection
	return t.conn.SendRawMessage(message)
}

// Disconnect is called when the connection is lost or closed.
// For stdio, this typically means the loop in Start() exited.
func (t *StdioTransport) Disconnect(sessionID string, reason error) {
	// No specific action needed for stdio disconnect beyond stopping the loop,
	// but we log it. The server core might use this notification later.
	log.Printf("StdioTransport: Disconnect notified for session %s: %v", sessionID, reason)
}

// GetSessionContext returns a background context for the stdio session.
func (t *StdioTransport) GetSessionContext(sessionID string) context.Context {
	if sessionID == t.sessionID {
		// TODO: Implement proper context management if needed for stdio cancellation
		return context.Background()
	}
	return context.Background() // Return background if session ID doesn't match
}

// Helper function to marshal JSONRPCResponse (avoids code duplication)
// TODO: Move this to a shared utility place?
func marshalResponse(id interface{}, result interface{}, errPayload *ErrorPayload) ([]byte, error) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   errPayload,
	}
	return json.Marshal(response)
}
