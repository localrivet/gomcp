package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/google/uuid"
)

// Connection handles the reading and writing of MCP messages over an underlying
// transport. It ensures messages are properly framed (newline-delimited JSON)
// and provides thread-safe writing.
type Connection struct {
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex // Protects concurrent writes to the writer
	// Store the original interfaces for potential closing later if needed
	rawReader io.Reader
	rawWriter io.Writer
}

// NewConnection creates a new MCP Connection using the provided io.Reader and io.Writer.
// This allows using the MCP transport logic with various underlying I/O mechanisms,
// such as network connections, in-memory pipes (for testing), or files.
func NewConnection(reader io.Reader, writer io.Writer) *Connection {
	return &Connection{
		reader:    bufio.NewReader(reader), // Wrap reader in bufio.Reader for ReadString
		writer:    writer,
		rawReader: reader, // Keep original reader
		rawWriter: writer, // Keep original writer
	}
}

// NewStdioConnection creates a new MCP Connection that reads from standard input
// and writes to standard output. This is a convenience function using NewConnection.
func NewStdioConnection() *Connection {
	return NewConnection(os.Stdin, os.Stdout)
}

// Close attempts to close the underlying reader and writer if they implement io.Closer.
// It returns the first error encountered during closing, or nil if both close successfully
// or are not closable.
func (c *Connection) Close() error {
	var err error
	log.Printf("Attempting to close connection...")
	if closer, ok := c.rawWriter.(io.Closer); ok {
		log.Printf("Closing writer...")
		err = closer.Close()
		if err != nil {
			log.Printf("Error closing writer: %v", err)
			// Continue to attempt closing reader even if writer fails
		}
	}
	if closer, ok := c.rawReader.(io.Closer); ok {
		log.Printf("Closing reader...")
		readerErr := closer.Close()
		if readerErr != nil {
			log.Printf("Error closing reader: %v", readerErr)
			// Prioritize returning the first error encountered
			if err == nil {
				err = readerErr
			}
		}
	}
	if err == nil {
		log.Printf("Connection closed successfully (or underlying streams not closable).")
	}
	return err
}

// SendMessage marshals the given payload into a complete MCP Message,
// assigns a new UUID as the MessageID, wraps it with the specified messageType
// and current protocol version, encodes it as JSON, and writes it to the
// connection's writer, followed by a newline character.
// It ensures the underlying writer is flushed if possible.
// Writes are protected by a mutex for concurrent safety.
func (c *Connection) SendMessage(messageType string, payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Construct the full message
	msg := Message{
		ProtocolVersion: CurrentProtocolVersion,
		MessageID:       uuid.NewString(),
		MessageType:     messageType,
		Payload:         payload,
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Log only in non-test environments? Or use a debug flag? For now, always log.
	// Consider using a more structured logger later.
	log.Printf("Sending: %s\n", string(jsonData))

	// MCP messages are newline-delimited JSON
	_, err = fmt.Fprintf(c.writer, "%s\n", jsonData)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	// Attempt to flush the writer if it supports it.
	// This is important for interactive protocols.
	if flusher, ok := c.writer.(interface{ Flush() error }); ok {
		err = flusher.Flush()
		if err != nil {
			// Log warning but don't make it fatal, write might have succeeded partially
			log.Printf("Warning: failed to flush writer: %v", err)
		}
	} else if f, ok := c.writer.(*os.File); ok && (f == os.Stdout || f == os.Stderr) {
		// Specifically sync stdout/stderr if they are the writer
		// Note: bufio.Writer also implements Flush, so the above case handles it.
		// This case handles direct os.Stdout/os.Stderr if not wrapped.
		err = f.Sync()
		if err != nil {
			log.Printf("Warning: failed to sync writer (%s): %v", f.Name(), err)
		}
	}

	return nil // Return nil even if flush/sync warned
}

// ReceiveMessage reads the next newline-delimited JSON line from the connection's reader.
// It attempts to unmarshal the line into a generic Message struct, keeping the
// payload field as json.RawMessage. This allows the caller to inspect the
// MessageType before deciding how to unmarshal the specific payload structure
// using the UnmarshalPayload helper or other means.
// Basic validation is performed to ensure required fields (ProtocolVersion, MessageID, MessageType) are present.
// If reading or initial unmarshalling fails, it attempts to send an MCP Error message back.
// Returns the generic Message pointer and nil error on success, or nil message and error on failure.
// io.EOF is logged but returned as a distinct error to signal graceful closure.
func (c *Connection) ReceiveMessage() (*Message, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			log.Println("Received EOF, connection likely closed by peer.")
		} else {
			log.Printf("Error reading message line: %v\n", err)
		}
		return nil, fmt.Errorf("failed to read message line: %w", err) // Wrap EOF as well
	}

	// Log only in non-test environments?
	log.Printf("Received raw: %s", line)

	var tempMsg struct {
		Message
		RawPayload json.RawMessage `json:"payload"`
	}

	err = json.Unmarshal([]byte(line), &tempMsg)
	if err != nil {
		log.Printf("Error unmarshalling message: %v\nRaw data: %s", err, line)
		// Attempt to send an MCP error message back if possible
		// Be careful not to cause infinite error loops
		if tempMsg.MessageType != MessageTypeError { // Avoid sending error in response to an error
			_ = c.SendMessage(MessageTypeError, ErrorPayload{
				Code:    "InvalidJSON",
				Message: fmt.Sprintf("Failed to unmarshal incoming JSON: %v", err),
			})
		}
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	finalMsg := &Message{
		ProtocolVersion: tempMsg.ProtocolVersion,
		MessageID:       tempMsg.MessageID,
		MessageType:     tempMsg.MessageType,
		Payload:         tempMsg.RawPayload,
	}

	// Log only in non-test environments?
	// log.Printf("Received parsed (generic): %+v\n", finalMsg)

	// Basic validation
	if finalMsg.ProtocolVersion == "" || finalMsg.MessageID == "" || finalMsg.MessageType == "" {
		errMsg := "Received message with missing required fields (ProtocolVersion, MessageID, or MessageType)"
		log.Println(errMsg)
		if finalMsg.MessageType != MessageTypeError { // Avoid sending error in response to an error
			_ = c.SendMessage(MessageTypeError, ErrorPayload{
				Code:    "InvalidMessage",
				Message: errMsg,
			})
		}
		// Use errMsg as an argument to avoid vet error
		return nil, fmt.Errorf("%s", errMsg)
	}

	return finalMsg, nil
}

// UnmarshalPayload is a helper function to unmarshal the payload field from a
// received Message (which should be json.RawMessage as returned by ReceiveMessage)
// into a specific Go struct pointed to by 'target'.
// It returns an error if the payload is not json.RawMessage, is nil, or if
// json.Unmarshal fails.
func UnmarshalPayload(payload interface{}, target interface{}) error {
	rawPayload, ok := payload.(json.RawMessage)
	if !ok {
		// Check if it's already the target type (e.g., if SendMessage sent a pre-marshalled payload - less common)
		// This requires reflection and is more complex, stick to RawMessage assumption for now.
		return fmt.Errorf("payload is not json.RawMessage (type: %T), cannot unmarshal", payload)
	}
	if len(rawPayload) == 0 || string(rawPayload) == "null" { // Check for empty or explicit null
		// Consider if nil payload is valid for the target type.
		// Returning an error is safer default.
		return fmt.Errorf("payload is nil or empty")
	}
	err := json.Unmarshal(rawPayload, target)
	if err != nil {
		// Format the type name separately to avoid vet error with %T and %w
		typeName := fmt.Sprintf("%T", target)
		return fmt.Errorf("failed to unmarshal payload into target type %s: %w", typeName, err)
	}
	return nil
}
