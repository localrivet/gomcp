// Package http provides an HTTP implementation of the MCP transport.
//
// This package implements the Transport interface using HTTP,
// suitable for applications requiring JSON-RPC communication.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/localrivet/gomcp/transport"
)

// DefaultShutdownTimeout is the default timeout for graceful shutdown
const DefaultShutdownTimeout = 10 * time.Second

// DefaultAPIPath is the default endpoint path for HTTP API
const DefaultAPIPath = "/api"

// Transport implements the transport.Transport interface for HTTP
type Transport struct {
	transport.BaseTransport
	addr          string
	server        *http.Server
	client        *http.Client
	asyncHandlers map[string]AsyncMessageHandler
	pathPrefix    string // Optional prefix for endpoint paths (e.g., "/mcp")
	apiPath       string // Path for the HTTP API endpoint
	mu            sync.RWMutex
}

// AsyncMessageHandler is a function that handles asynchronous JSON-RPC notifications
type AsyncMessageHandler func(message []byte)

// NewTransport creates a new HTTP transport
func NewTransport(addr string) *Transport {
	return &Transport{
		addr:          addr,
		client:        &http.Client{Timeout: 30 * time.Second},
		asyncHandlers: make(map[string]AsyncMessageHandler),
		pathPrefix:    "", // Empty by default
		apiPath:       DefaultAPIPath,
	}
}

// SetPathPrefix sets a prefix for all endpoint paths
// For example, SetPathPrefix("/mcp") will result in endpoint like "/mcp/api"
func (t *Transport) SetPathPrefix(prefix string) *Transport {
	// Ensure the prefix starts with a slash if not empty
	if prefix != "" && !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	t.pathPrefix = prefix
	return t
}

// SetAPIPath sets the path for the HTTP API endpoint
func (t *Transport) SetAPIPath(path string) *Transport {
	// Ensure the path starts with a slash
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	t.apiPath = path
	return t
}

// GetFullAPIPath returns the complete path for the HTTP API endpoint
func (t *Transport) GetFullAPIPath() string {
	if t.pathPrefix == "" {
		return t.apiPath
	}
	return t.pathPrefix + t.apiPath
}

// Initialize initializes the transport
func (t *Transport) Initialize() error {
	// Nothing special to initialize for HTTP
	return nil
}

// Start starts the transport
func (t *Transport) Start() error {
	// Create a new HTTP server
	mux := http.NewServeMux()

	// Register the API endpoint at the configured path
	mux.HandleFunc(t.GetFullAPIPath(), t.handleHTTPRequest)

	t.server = &http.Server{
		Addr:    t.addr,
		Handler: mux,
	}

	// Start the server in a goroutine
	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the transport
func (t *Transport) Stop() error {
	if t.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
		defer cancel()
		return t.server.Shutdown(ctx)
	}
	return nil
}

// Send sends a JSON-RPC request to a specified endpoint
func (t *Transport) Send(message []byte) error {
	// Parse the message to extract method for potential async handling
	var jsonRPCRequest struct {
		Jsonrpc string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
		Id      interface{}     `json:"id,omitempty"`
	}

	if err := json.Unmarshal(message, &jsonRPCRequest); err != nil {
		return fmt.Errorf("invalid JSON-RPC message: %w", err)
	}

	// Create the full URL with API path
	url := t.addr

	// Make sure the URL includes the API path
	if !strings.Contains(url, t.apiPath) {
		// Add API path to the URL
		if !strings.HasSuffix(url, "/") {
			url += "/"
		}
		url = strings.TrimSuffix(url, "/") + t.GetFullAPIPath()
	}

	// Create a new HTTP request with proper reader
	req, err := http.NewRequest("POST", url, bytes.NewReader(message))
	if err != nil {
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// For notifications (no ID), we don't need to read the response
	if jsonRPCRequest.Id == nil {
		return nil
	}

	// Read the response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Try to process the response - HandleMessage will return an error if no handler is set
	_, err = t.HandleMessage(responseBody)
	if err != nil && err.Error() != "no message handler set" {
		return err
	}

	return nil
}

// Receive is not directly applicable for HTTP transport (HTTP is request/response based)
func (t *Transport) Receive() ([]byte, error) {
	return nil, errors.New("receive operation not supported for HTTP transport")
}

// RegisterAsyncHandler registers a handler for asynchronous message processing
func (t *Transport) RegisterAsyncHandler(method string, handler AsyncMessageHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.asyncHandlers[method] = handler
}

// GetAddr returns the transport's address
func (t *Transport) GetAddr() string {
	return t.addr
}

// handleHTTPRequest handles incoming HTTP requests
func (t *Transport) handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests for JSON-RPC
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse JSON-RPC request to determine if it's a notification
	var jsonRPCRequest struct {
		Jsonrpc string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
		Id      interface{}     `json:"id,omitempty"`
	}

	if err := json.Unmarshal(body, &jsonRPCRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Validate JSON-RPC version
	if jsonRPCRequest.Jsonrpc != "2.0" {
		w.WriteHeader(http.StatusBadRequest)
		jsonError := map[string]interface{}{
			"jsonrpc": "2.0",
			"error": map[string]interface{}{
				"code":    -32600,
				"message": "Invalid Request",
			},
			"id": nil,
		}
		if jsonRPCRequest.Id != nil {
			jsonError["id"] = jsonRPCRequest.Id
		}
		json.NewEncoder(w).Encode(jsonError)
		return
	}

	// Handle the request based on whether it's a notification (async) or a regular request (sync)
	if jsonRPCRequest.Id == nil {
		// Asynchronous notification
		t.mu.RLock()
		handler, ok := t.asyncHandlers[jsonRPCRequest.Method]
		t.mu.RUnlock()

		if ok {
			go handler(body)
			w.WriteHeader(http.StatusAccepted)
			return
		}

		// Try the general handler
		response, err := t.HandleMessage(body)
		if err == nil && response != nil {
			w.WriteHeader(http.StatusAccepted)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
		return
	}

	// Synchronous request - use the general message handler
	response, err := t.HandleMessage(body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		jsonError := map[string]interface{}{
			"jsonrpc": "2.0",
			"error": map[string]interface{}{
				"code":    -32603,
				"message": "Internal error",
				"data":    err.Error(),
			},
			"id": jsonRPCRequest.Id,
		}
		json.NewEncoder(w).Encode(jsonError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}
