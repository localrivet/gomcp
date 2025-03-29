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

// --- JSON-RPC Sending Methods ---

// sendJSON marshals and sends a generic JSON object, handling locking and flushing.
func (c *Connection) sendJSON(data interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON-RPC message: %w", err)
	}

	log.Printf("Sending JSON-RPC: %s\n", string(jsonData))

	_, err = fmt.Fprintf(c.writer, "%s\n", jsonData)
	if err != nil {
		return fmt.Errorf("failed to write JSON-RPC message: %w", err)
	}

	// Attempt to flush the writer
	if flusher, ok := c.writer.(interface{ Flush() error }); ok {
		flushErr := flusher.Flush()
		if flushErr != nil {
			log.Printf("Warning: failed to flush writer after JSON-RPC send: %v", flushErr)
		}
	} else if f, ok := c.writer.(*os.File); ok && (f == os.Stdout || f == os.Stderr) {
		syncErr := f.Sync()
		if syncErr != nil {
			log.Printf("Warning: failed to sync writer (%s) after JSON-RPC send: %v", f.Name(), syncErr)
		}
	}

	return nil
}

// SendRequest sends a JSON-RPC request.
// It generates a new UUID for the request ID.
func (c *Connection) SendRequest(method string, params interface{}) (string, error) {
	id := uuid.NewString()
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	err := c.sendJSON(req)
	return id, err // Return the generated ID so the caller can match the response
}

// SendResponse sends a successful JSON-RPC response.
func (c *Connection) SendResponse(id interface{}, result interface{}) error {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	return c.sendJSON(resp)
}

// SendErrorResponse sends a JSON-RPC error response.
func (c *Connection) SendErrorResponse(id interface{}, errPayload ErrorPayload) error {
	// Ensure ID is null if it couldn't be determined (e.g., parse error)
	// The caller should handle this logic based on where the error occurred.
	// For simplicity here, we assume id is usually valid when sending an error response.
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &errPayload,
	}
	return c.sendJSON(resp)
}

// SendNotification sends a JSON-RPC notification.
func (c *Connection) SendNotification(method string, params interface{}) error {
	notif := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return c.sendJSON(notif)
}

// --- Receiving Logic ---

// ReceiveRawMessage reads the next newline-delimited JSON line from the connection's reader.
// It performs basic JSON validation.
// Returns the raw JSON message bytes and nil error on success, or nil bytes and error on failure.
// io.EOF is logged but returned as a distinct error to signal graceful closure.
func (c *Connection) ReceiveRawMessage() ([]byte, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			log.Println("Received EOF, connection likely closed by peer.")
		} else {
			log.Printf("Error reading message line: %v\n", err)
		}
		return nil, fmt.Errorf("failed to read message line: %w", err) // Wrap EOF as well
	}

	log.Printf("Received raw: %s", line)

	// Basic JSON validation (check if it's valid JSON at all)
	if !json.Valid([]byte(line)) {
		log.Printf("Received invalid JSON: %s", line)
		// Attempt to send a JSON-RPC error response back if possible
		// Send error with null ID because we can't parse anything
		_ = c.SendErrorResponse(nil, ErrorPayload{
			Code:    ErrorCodeParseError,
			Message: "Received invalid JSON",
		})
		return nil, fmt.Errorf("received invalid JSON")
	}

	return []byte(line), nil
}

// UnmarshalPayload is a helper function to unmarshal the payload field from a
// received JSON-RPC params or result field (which is interface{})
// into a specific Go struct pointed to by 'target'.
// It handles the case where the payload might be nil or needs re-marshalling.
func UnmarshalPayload(payload interface{}, target interface{}) error {
	if payload == nil {
		// Consider if nil payload is valid for the target type.
		// Returning an error is safer default.
		return fmt.Errorf("payload is nil, cannot unmarshal")
	}

	// Re-marshal the interface{} back to JSON bytes
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to re-marshal payload (type %T): %w", payload, err)
	}

	// Check for empty or explicit null JSON after re-marshalling
	if len(payloadBytes) == 0 || string(payloadBytes) == "null" {
		return fmt.Errorf("payload is nil or empty after re-marshalling")
	}

	// Unmarshal the JSON bytes into the target struct
	err = json.Unmarshal(payloadBytes, target)
	if err != nil {
		// Format the type name separately to avoid vet error with %T and %w
		typeName := fmt.Sprintf("%T", target)
		return fmt.Errorf("failed to unmarshal payload into target type %s: %w", typeName, err)
	}
	return nil
}
