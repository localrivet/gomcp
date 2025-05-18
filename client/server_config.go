// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ServerConfig represents a complete MCP server configuration file
type ServerConfig struct {
	MCPServers map[string]ServerDefinition `json:"mcpServers"`
}

// ServerDefinition defines how to launch and connect to an MCP server
type ServerDefinition struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
}

// MCPServer represents a running MCP server process with a connected client
type MCPServer struct {
	Name   string
	Client Client
	cmd    *exec.Cmd
}

// ServerRegistry manages a collection of MCP servers loaded from configuration
type ServerRegistry struct {
	servers map[string]*MCPServer
	mu      sync.RWMutex
}

// NewServerRegistry creates a new empty server registry
func NewServerRegistry() *ServerRegistry {
	return &ServerRegistry{
		servers: make(map[string]*MCPServer),
	}
}

// LoadConfig loads a server configuration from a file
func (r *ServerRegistry) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config ServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return r.ApplyConfig(config)
}

// ApplyConfig applies a server configuration by starting servers and connecting clients
func (r *ServerRegistry) ApplyConfig(config ServerConfig) error {
	for name, def := range config.MCPServers {
		if err := r.StartServer(name, def); err != nil {
			return fmt.Errorf("failed to start server %s: %w", name, err)
		}
	}
	return nil
}

// StartServer starts a server from its definition and connects a client to it
func (r *ServerRegistry) StartServer(name string, def ServerDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if server already exists
	if _, exists := r.servers[name]; exists {
		return fmt.Errorf("server %s already exists", name)
	}

	// Create command
	cmd := exec.Command(def.Command, def.Args...)

	// Set environment variables
	env := os.Environ()
	for k, v := range def.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Set up stdio pipes for communication
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Set stderr to go to the parent process stderr for debugging
	cmd.Stderr = os.Stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Create a transport for the client
	transport := &stdioPipeTransport{
		reader: stdoutPipe,
		writer: stdinPipe,
	}

	// Create a logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	logger = logger.With("server", name)

	// Create client options - use the standard WithTransport function
	clientOpts := []Option{
		WithLogger(logger),
		WithTransport(transport),
	}

	// Create the client and connect to the server
	client, err := NewClient(name, clientOpts...)
	if err != nil {
		// Kill the process if client creation fails
		cmd.Process.Kill()
		cmd.Wait()
		return fmt.Errorf("failed to create client for server %s: %w", name, err)
	}

	// Store the server in our registry
	r.servers[name] = &MCPServer{
		Name:   name,
		Client: client,
		cmd:    cmd,
	}

	return nil
}

// GetClient returns the client for a named server
func (r *ServerRegistry) GetClient(name string) (Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	server, exists := r.servers[name]
	if !exists {
		return nil, fmt.Errorf("server %s not found", name)
	}

	return server.Client, nil
}

// GetServerNames returns a list of all server names in the registry
func (r *ServerRegistry) GetServerNames() ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.servers))
	for name := range r.servers {
		names = append(names, name)
	}

	return names, nil
}

// StopServer stops a server by name
func (r *ServerRegistry) StopServer(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	server, exists := r.servers[name]
	if !exists {
		return fmt.Errorf("server %s not found", name)
	}

	// Close the client first
	if err := server.Client.Close(); err != nil {
		return fmt.Errorf("failed to close client: %w", err)
	}

	// Then terminate the process
	if err := server.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	// Wait for the process to exit
	if err := server.cmd.Wait(); err != nil {
		// Ignore the error if it's due to the process being killed
		if !strings.Contains(err.Error(), "killed") {
			return fmt.Errorf("error waiting for process to exit: %w", err)
		}
	}

	// Remove from our registry
	delete(r.servers, name)

	return nil
}

// StopAll stops all servers
func (r *ServerRegistry) StopAll() error {
	r.mu.RLock()
	names := make([]string, 0, len(r.servers))
	for name := range r.servers {
		names = append(names, name)
	}
	r.mu.RUnlock()

	var lastErr error
	for _, name := range names {
		if err := r.StopServer(name); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// stdioPipeTransport implements the Transport interface for stdio pipes
type stdioPipeTransport struct {
	reader         io.Reader
	writer         io.Writer
	requestTimeout time.Duration
	connectTimeout time.Duration
	notifyHandler  func(method string, params []byte)
	connected      bool
	mu             sync.RWMutex
}

func (t *stdioPipeTransport) Connect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connected = true
	return nil
}

func (t *stdioPipeTransport) ConnectWithContext(ctx context.Context) error {
	return t.Connect()
}

func (t *stdioPipeTransport) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connected = false
	return nil
}

func (t *stdioPipeTransport) Send(message []byte) ([]byte, error) {
	return t.SendWithContext(context.Background(), message)
}

func (t *stdioPipeTransport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	t.mu.RLock()
	connected := t.connected
	t.mu.RUnlock()

	if !connected {
		return nil, errors.New("transport not connected")
	}

	// Write message to the writer
	if _, err := t.writer.Write(append(message, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write message: %w", err)
	}

	// Create a channel for the response
	responseCh := make(chan []byte, 1)
	errCh := make(chan error, 1)

	// Read response in a goroutine
	go func() {
		scanner := bufio.NewScanner(t.reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max size

		if scanner.Scan() {
			responseCh <- scanner.Bytes()
		} else if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("error reading response: %w", err)
		} else {
			errCh <- io.EOF
		}
	}()

	// Wait for response or context cancellation
	select {
	case response := <-responseCh:
		return response, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *stdioPipeTransport) SetRequestTimeout(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.requestTimeout = timeout
}

func (t *stdioPipeTransport) SetConnectionTimeout(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connectTimeout = timeout
}

func (t *stdioPipeTransport) RegisterNotificationHandler(handler func(method string, params []byte)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.notifyHandler = handler
}

// Root functions to provide a cleaner API following the PRD guidance
// These will be added to the root gomcp.go file

// NewDefaultLogger creates a simple logger suitable for MCP clients
func NewDefaultLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
