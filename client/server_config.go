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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

// ProcessInfo tracks spawned processes for comprehensive cleanup
type ProcessInfo struct {
	PID        int
	ServerName string
	Command    string
	StartTime  time.Time
	Children   []int // Child process PIDs
}

// ServerRegistry manages a collection of MCP servers loaded from configuration
type ServerRegistry struct {
	servers map[string]*MCPServer
	logger  *slog.Logger
	mu      sync.RWMutex
	closed  bool // Track if registry has been closed
	ctx     context.Context
	cancel  context.CancelFunc

	// Process tracking for comprehensive cleanup (opt-in)
	spawnedProcesses      map[int]*ProcessInfo
	processMutex          sync.Mutex
	enableProcessTracking bool // Only enable when needed for production use
}

// ServerRegistryOption configures a ServerRegistry
type ServerRegistryOption func(*ServerRegistry)

// WithRegistryLogger sets a logger for the server registry.
// When using stdio-based MCP servers, ensure the logger does not write to stdout/stderr
// to avoid interfering with the JSON-RPC communication.
func WithRegistryLogger(logger *slog.Logger) ServerRegistryOption {
	return func(r *ServerRegistry) {
		r.logger = logger
	}
}

// WithProcessTracking enables comprehensive process tracking and cleanup.
// This is recommended for production environments to prevent process leaks.
// Process tracking adds overhead and should be disabled for tests with mock commands.
func WithProcessTracking() ServerRegistryOption {
	return func(r *ServerRegistry) {
		r.enableProcessTracking = true
	}
}

// NewServerRegistry creates a new empty server registry.
// By default, no logging is enabled to avoid interfering with stdio-based MCP communication.
func NewServerRegistry(opts ...ServerRegistryOption) *ServerRegistry {
	ctx, cancel := context.WithCancel(context.Background())

	r := &ServerRegistry{
		servers:          make(map[string]*MCPServer),
		logger:           nil, // Default to no logging to avoid stdio interference
		closed:           false,
		ctx:              ctx,
		cancel:           cancel,
		spawnedProcesses: make(map[int]*ProcessInfo),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
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
	if len(config.MCPServers) == 0 {
		return nil
	}

	// Use goroutines to start servers concurrently
	type serverResult struct {
		name string
		err  error
	}

	resultCh := make(chan serverResult, len(config.MCPServers))

	// Start all servers concurrently
	for name, def := range config.MCPServers {
		go func(serverName string, serverDef ServerDefinition) {
			err := r.StartServer(serverName, serverDef)
			resultCh <- serverResult{name: serverName, err: err}
		}(name, def)
	}

	// Collect results and check for errors
	var errors []string
	successCount := 0
	for i := 0; i < len(config.MCPServers); i++ {
		result := <-resultCh
		if result.err != nil {
			errors = append(errors, fmt.Sprintf("server %s: %v", result.name, result.err))
		} else {
			successCount++
		}
	}

	// Return error information if any servers failed
	if len(errors) > 0 {
		return fmt.Errorf("failed to start %d/%d servers: %s", len(errors), len(config.MCPServers), strings.Join(errors, "; "))
	}

	return nil
}

// StartServer starts a server from its definition and connects a client to it
func (r *ServerRegistry) StartServer(name string, def ServerDefinition) error {
	// Check if registry is closed
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return fmt.Errorf("cannot start server %s: registry is closed", name)
	}
	r.mu.RUnlock()
	// Create command
	cmd := exec.Command(def.Command, def.Args...)

	// Set environment variables
	env := os.Environ()
	for k, v := range def.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Create a new process group to enable clean killing of child processes
	// This is the recommended approach for handling process trees
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

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

	// Track the spawned process for comprehensive cleanup (if enabled)
	if r.enableProcessTracking {
		r.trackProcess(cmd.Process.Pid, name, def.Command)
	}

	// Create a transport for the client
	transport := &stdioPipeTransport{
		reader: stdoutPipe,
		writer: stdinPipe,
	}

	// Create client options - use the standard WithTransport function
	clientOpts := []Option{
		WithTransport(transport),
		WithConnectionTimeout(5 * time.Second), // Short timeout for server registry to prevent test hangs
	}

	// Add logger if configured
	if r.logger != nil {
		serverLogger := r.logger.With("server", name)
		clientOpts = append(clientOpts, WithLogger(serverLogger))
	}

	// Create the client and connect to the server
	client, err := NewClient(name, clientOpts...)
	if err != nil {
		// Clean up the process if client creation fails
		if killErr := r.terminateProcess(cmd, name); killErr != nil {
			if r.logger != nil {
				r.logger.Error("Failed to clean up process after client creation failure",
					"server", name, "error", killErr)
			}
		}
		return fmt.Errorf("failed to create client for server %s: %w", name, err)
	}

	// Only lock for the brief moment we need to update the map
	r.mu.Lock()
	// Check if server already exists (race condition check)
	if _, exists := r.servers[name]; exists {
		r.mu.Unlock()
		// Clean up the client and process we just created
		client.Close()
		if killErr := r.terminateProcess(cmd, name); killErr != nil {
			if r.logger != nil {
				r.logger.Error("Failed to clean up duplicate server process",
					"server", name, "error", killErr)
			}
		}
		return fmt.Errorf("server %s already exists", name)
	}
	// Store the server in our registry
	r.servers[name] = &MCPServer{
		Name:   name,
		Client: client,
		cmd:    cmd,
	}
	r.mu.Unlock()

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

	// Close the client first to signal graceful shutdown (if client exists)
	if server.Client != nil {
		if err := server.Client.Close(); err != nil {
			if r.logger != nil {
				r.logger.Warn("Failed to close client gracefully", "server", name, "error", err)
			}
		}
	}

	// Remove from registry immediately to prevent double-cleanup
	delete(r.servers, name)

	// Gracefully terminate the process with proper timeout and escalation
	if err := r.terminateProcess(server.cmd, name); err != nil {
		if r.logger != nil {
			r.logger.Error("Failed to terminate server process", "server", name, "error", err)
		}
		return fmt.Errorf("failed to terminate server %s: %w", name, err)
	}

	// Remove from process tracking (if enabled)
	if r.enableProcessTracking && server.cmd != nil && server.cmd.Process != nil {
		r.untrackProcess(server.cmd.Process.Pid)
	}

	return nil
}

// terminateProcess gracefully terminates a process with escalating signals and timeouts
func (r *ServerRegistry) terminateProcess(cmd *exec.Cmd, name string) error {
	if cmd == nil || cmd.Process == nil {
		return nil // Already dead or never started
	}

	// Create an independent context for termination with a reasonable timeout
	// Don't use r.ctx as it might be cancelled during Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if this process has stdin set up (indicates it's a proper MCP server)
	hasStdin := cmd.Stdin != nil

	// Step 1: Try graceful shutdown only if we have stdin (MCP servers)
	gracefulTimeout := 3 * time.Second
	if !hasStdin {
		// For test processes without stdin, use a much shorter timeout
		gracefulTimeout = 100 * time.Millisecond
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	if hasStdin {
		// First close stdin to signal the process to shut down gracefully
		// This is often how MCP servers detect client disconnect
		if stdinCloser, ok := cmd.Stdin.(io.Closer); ok && stdinCloser != nil {
			if err := stdinCloser.Close(); err != nil && r.logger != nil {
				r.logger.Debug("Failed to close stdin", "server", name, "error", err)
			}
		}
	}

	// Wait for graceful shutdown
	select {
	case <-done:
		// Process exited gracefully
		if r.logger != nil {
			r.logger.Debug("Process exited gracefully after stdin close", "server", name)
		}
		return nil
	case <-time.After(gracefulTimeout):
		// Graceful shutdown timeout, proceed to force kill
		if r.logger != nil {
			if hasStdin {
				r.logger.Debug("Graceful shutdown timeout, force killing", "server", name)
			} else {
				r.logger.Debug("Test process - proceeding to force kill", "server", name)
			}
		}
	case <-ctx.Done():
		return fmt.Errorf("termination context cancelled for process %s", name)
	}

	// Step 2: Force kill - use process group if available (Unix-like systems only)
	if runtime.GOOS != "windows" {
		// Try to get the process group ID and kill the entire group
		// But only if this process was created with its own process group
		if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
			// Check if this process is in its own process group (not the test runner's group)
			// If pgid == pid, then this process is the process group leader (created with Setpgid: true)
			if pgid == cmd.Process.Pid {
				if r.logger != nil {
					r.logger.Debug("Killing process group", "server", name, "pid", cmd.Process.Pid, "pgid", pgid)
				}

				// Kill the entire process group with SIGKILL
				// Use -pgid to target the group, not just the process
				if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
					if r.logger != nil {
						r.logger.Debug("Failed to kill process group, falling back to single process kill",
							"server", name, "pgid", pgid, "error", err)
					}
					// Fall back to killing just the main process
					if err := cmd.Process.Kill(); err != nil {
						if strings.Contains(err.Error(), "process already finished") {
							return nil
						}
						return fmt.Errorf("failed to kill process: %w", err)
					}
				}
			} else {
				// Process is not in its own group, just kill the main process
				if r.logger != nil {
					r.logger.Debug("Process not in own group, killing main process only",
						"server", name, "pid", cmd.Process.Pid, "pgid", pgid)
				}
				if err := cmd.Process.Kill(); err != nil {
					if strings.Contains(err.Error(), "process already finished") {
						return nil
					}
					return fmt.Errorf("failed to kill process: %w", err)
				}
			}
		} else {
			// Couldn't get pgid, fall back to killing just the main process
			if r.logger != nil {
				r.logger.Debug("Failed to get process group, killing main process only",
					"server", name, "error", err)
			}
			if err := cmd.Process.Kill(); err != nil {
				if strings.Contains(err.Error(), "process already finished") {
					return nil
				}
				return fmt.Errorf("failed to kill process: %w", err)
			}
		}
	} else {
		// Windows: just kill the main process
		if err := cmd.Process.Kill(); err != nil {
			if strings.Contains(err.Error(), "process already finished") {
				return nil
			}
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	// Step 3: Wait for process death with timeout
	select {
	case err := <-done:
		// Process died (ignore "signal: killed" error since we caused it)
		if err != nil && !strings.Contains(err.Error(), "signal: killed") {
			if r.logger != nil {
				r.logger.Warn("Process wait returned error", "server", name, "error", err)
			}
		}
		return nil
	case <-ctx.Done():
		// Process still not dead after timeout - this is serious
		return fmt.Errorf("process %s did not die after SIGKILL within timeout", name)
	}
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

// Close shuts down the ServerRegistry and ensures all processes are terminated.
// This should be called when the application is shutting down to prevent orphaned processes.
func (r *ServerRegistry) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil // Already closed
	}
	r.closed = true
	r.mu.Unlock()

	// Stop all servers first (this handles graceful client shutdown)
	if err := r.StopAll(); err != nil {
		if r.logger != nil {
			r.logger.Warn("Failed to stop all servers gracefully", "error", err)
		}
	}

	// Perform comprehensive cleanup of all tracked processes and their trees (if enabled)
	if r.enableProcessTracking {
		if err := r.cleanupAllTrackedProcesses(); err != nil {
			if r.logger != nil {
				r.logger.Error("Failed to cleanup tracked processes", "error", err)
			}
			// Still cancel the context even if cleanup fails
			r.cancel()
			return fmt.Errorf("failed to cleanup tracked processes during close: %w", err)
		}
	}

	// Cancel the context after successful shutdown
	r.cancel()

	return nil
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

	// Request/response correlation
	pendingRequests map[int64]chan []byte
	pendingMu       sync.RWMutex
	readerStarted   bool
	readerDone      chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
}

func (t *stdioPipeTransport) Connect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	// Initialize correlation structures
	t.pendingRequests = make(map[int64]chan []byte)
	t.readerDone = make(chan struct{})
	t.ctx, t.cancel = context.WithCancel(context.Background())

	t.connected = true

	// Start the single reader goroutine
	t.startReader()

	return nil
}

func (t *stdioPipeTransport) ConnectWithContext(ctx context.Context) error {
	return t.Connect()
}

func (t *stdioPipeTransport) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	t.connected = false

	// Cancel context to stop reader
	if t.cancel != nil {
		t.cancel()
	}

	// Wait for reader to finish
	if t.readerStarted {
		select {
		case <-t.readerDone:
		case <-time.After(1 * time.Second):
			// Timeout waiting for reader to stop
		}
	}

	// Close all pending requests
	t.pendingMu.Lock()
	for _, ch := range t.pendingRequests {
		close(ch)
	}
	t.pendingRequests = make(map[int64]chan []byte)
	t.pendingMu.Unlock()

	return nil
}

// startReader starts the single reader goroutine that handles all responses
func (t *stdioPipeTransport) startReader() {
	if t.readerStarted {
		return
	}
	t.readerStarted = true

	go func() {
		defer close(t.readerDone)

		scanner := bufio.NewScanner(t.reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max size

		for {
			select {
			case <-t.ctx.Done():
				return
			default:
			}

			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					// Handle scan error - close all pending requests
					t.closeAllPending()
				}
				return
			}

			response := make([]byte, len(scanner.Bytes()))
			copy(response, scanner.Bytes())

			// Parse JSON to extract request ID
			var jsonResp struct {
				ID interface{} `json:"id"`
			}

			if err := json.Unmarshal(response, &jsonResp); err != nil {
				// Not a valid JSON response, might be notification
				if t.notifyHandler != nil {
					t.notifyHandler("", response)
				}
				continue
			}

			// Handle notifications (no ID)
			if jsonResp.ID == nil {
				if t.notifyHandler != nil {
					t.notifyHandler("", response)
				}
				continue
			}

			// Extract request ID
			var requestID int64
			switch id := jsonResp.ID.(type) {
			case float64:
				requestID = int64(id)
			case int64:
				requestID = id
			case int:
				requestID = int64(id)
			default:
				// Invalid ID type, skip
				continue
			}

			// Deliver response to waiting goroutine
			t.pendingMu.RLock()
			ch, exists := t.pendingRequests[requestID]
			t.pendingMu.RUnlock()

			if exists {
				select {
				case ch <- response:
				default:
					// Channel full or closed, skip
				}
			}
		}
	}()
}

// closeAllPending closes all pending request channels
func (t *stdioPipeTransport) closeAllPending() {
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()

	for _, ch := range t.pendingRequests {
		close(ch)
	}
	t.pendingRequests = make(map[int64]chan []byte)
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

	// Parse message to extract request ID
	var jsonMsg struct {
		ID interface{} `json:"id"`
	}

	if err := json.Unmarshal(message, &jsonMsg); err != nil {
		return nil, fmt.Errorf("failed to parse request JSON: %w", err)
	}

	// Handle notifications (no ID) - send and return immediately
	if jsonMsg.ID == nil {
		if _, err := t.writer.Write(append(message, '\n')); err != nil {
			return nil, fmt.Errorf("failed to write notification: %w", err)
		}
		return []byte{}, nil // Empty response for notifications
	}

	// Extract request ID
	var requestID int64
	switch id := jsonMsg.ID.(type) {
	case float64:
		requestID = int64(id)
	case int64:
		requestID = id
	case int:
		requestID = int64(id)
	default:
		return nil, fmt.Errorf("invalid request ID type: %T", jsonMsg.ID)
	}

	// Create response channel and register it
	responseCh := make(chan []byte, 1)

	t.pendingMu.Lock()
	t.pendingRequests[requestID] = responseCh
	t.pendingMu.Unlock()

	// Cleanup function
	defer func() {
		t.pendingMu.Lock()
		delete(t.pendingRequests, requestID)
		t.pendingMu.Unlock()
	}()

	// Write message to the writer
	if _, err := t.writer.Write(append(message, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write message: %w", err)
	}

	// Wait for response or context cancellation
	select {
	case response := <-responseCh:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.ctx.Done():
		return nil, errors.New("transport disconnected")
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

// LoggerOption configures a logger
type LoggerOption func(*loggerConfig)

type loggerConfig struct {
	level  slog.Level
	output io.Writer
}

// WithLogLevel sets the log level
func WithLogLevel(level slog.Level) LoggerOption {
	return func(c *loggerConfig) {
		c.level = level
	}
}

// WithLogOutput sets the output writer
func WithLogOutput(w io.Writer) LoggerOption {
	return func(c *loggerConfig) {
		c.output = w
	}
}

// WithLogFile sets the output to a file (safe for stdio-based MCP communication)
func WithLogFile(filename string) LoggerOption {
	return func(c *loggerConfig) {
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			// If file opening fails, fall back to discard to avoid panics
			c.output = io.Discard
			return
		}
		c.output = file
	}
}

// WithLogDiscard disables all logging output
func WithLogDiscard() LoggerOption {
	return func(c *loggerConfig) {
		c.output = io.Discard
	}
}

// NewLogger creates a logger with the specified options.
// By default, creates a logger that writes to stderr with Info level.
func NewLogger(opts ...LoggerOption) *slog.Logger {
	config := &loggerConfig{
		level:  slog.LevelInfo,
		output: os.Stderr,
	}

	for _, opt := range opts {
		opt(config)
	}

	return slog.New(slog.NewTextHandler(config.output, &slog.HandlerOptions{
		Level: config.level,
	}))
}

// trackProcess adds a process to the tracking list for cleanup
func (r *ServerRegistry) trackProcess(pid int, serverName, command string) {
	r.processMutex.Lock()
	defer r.processMutex.Unlock()

	processInfo := &ProcessInfo{
		PID:        pid,
		ServerName: serverName,
		Command:    command,
		StartTime:  time.Now(),
		Children:   []int{},
	}

	r.spawnedProcesses[pid] = processInfo

	// Discover child processes after a brief delay to allow them to spawn
	go func() {
		time.Sleep(500 * time.Millisecond)
		r.discoverChildProcesses(pid)
	}()

	if r.logger != nil {
		r.logger.Debug("Tracking process", "pid", pid, "server", serverName, "command", command)
	}
}

// discoverChildProcesses finds all child processes of a given parent PID
func (r *ServerRegistry) discoverChildProcesses(parentPID int) {
	// Check if parent process still exists before discovering children
	if !r.processExists(parentPID) {
		if r.logger != nil {
			r.logger.Debug("Parent process no longer exists, skipping child discovery", "pid", parentPID)
		}
		return
	}

	children := r.findChildProcesses(parentPID)

	r.processMutex.Lock()
	defer r.processMutex.Unlock()

	if processInfo, exists := r.spawnedProcesses[parentPID]; exists {
		processInfo.Children = children
		if r.logger != nil && len(children) > 0 {
			r.logger.Debug("Discovered child processes", "parent", parentPID, "children", children)
		}
	}
}

// processExists checks if a process with the given PID exists
func (r *ServerRegistry) processExists(pid int) bool {
	if runtime.GOOS == "windows" {
		// Windows implementation
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid))
		output, err := cmd.Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(output), strconv.Itoa(pid))
	}

	// Unix-like systems - try to send signal 0 to check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// findChildProcesses discovers child processes using system process information
func (r *ServerRegistry) findChildProcesses(parentPID int) []int {
	var children []int

	if runtime.GOOS == "windows" {
		// Windows implementation would go here
		return children
	}

	// Unix-like systems (macOS, Linux)
	cmd := exec.Command("ps", "-eo", "pid,ppid")
	output, err := cmd.Output()
	if err != nil {
		if r.logger != nil {
			r.logger.Debug("Failed to get process list", "error", err)
		}
		return children
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines[1:] { // Skip header
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			pid, err1 := strconv.Atoi(fields[0])
			ppid, err2 := strconv.Atoi(fields[1])

			if err1 == nil && err2 == nil && ppid == parentPID {
				children = append(children, pid)
				// Recursively find grandchildren
				grandchildren := r.findChildProcesses(pid)
				children = append(children, grandchildren...)
			}
		}
	}

	return children
}

// terminateProcessTree terminates a process and all its children
func (r *ServerRegistry) terminateProcessTree(pid int) error {
	// Find all children first
	children := r.findChildProcesses(pid)

	// Terminate children first (depth-first)
	for _, childPID := range children {
		if err := r.terminateProcessTree(childPID); err != nil {
			if r.logger != nil {
				r.logger.Debug("Failed to terminate child process", "pid", childPID, "error", err)
			}
		}
	}

	// Now terminate the parent process
	return r.terminateSingleProcess(pid)
}

// terminateSingleProcess terminates a single process by PID
func (r *ServerRegistry) terminateSingleProcess(pid int) error {
	if runtime.GOOS == "windows" {
		// Windows implementation
		cmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
		return cmd.Run()
	}

	// Unix-like systems
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil // Process doesn't exist
	}

	// Try graceful termination first
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		if strings.Contains(err.Error(), "process already finished") {
			return nil
		}
	} else {
		// Wait briefly for graceful shutdown
		time.Sleep(1 * time.Second)

		// Check if process still exists
		if err := process.Signal(syscall.Signal(0)); err != nil {
			return nil // Process died gracefully
		}
	}

	// Force kill if still alive
	if err := process.Signal(syscall.SIGKILL); err != nil {
		if strings.Contains(err.Error(), "process already finished") {
			return nil
		}
		return fmt.Errorf("failed to kill process %d: %w", pid, err)
	}

	return nil
}

// cleanupAllTrackedProcesses terminates all tracked processes and their trees
func (r *ServerRegistry) cleanupAllTrackedProcesses() error {
	r.processMutex.Lock()
	defer r.processMutex.Unlock()

	if len(r.spawnedProcesses) == 0 {
		return nil
	}

	if r.logger != nil {
		r.logger.Debug("Cleaning up tracked processes", "count", len(r.spawnedProcesses))
	}

	var errors []string

	// Terminate all tracked process trees
	for pid, processInfo := range r.spawnedProcesses {
		if r.logger != nil {
			r.logger.Debug("Terminating process tree", "pid", pid, "server", processInfo.ServerName)
		}

		if err := r.terminateProcessTree(pid); err != nil {
			errors = append(errors, fmt.Sprintf("failed to terminate process tree %d (%s): %v",
				pid, processInfo.ServerName, err))
		}
	}

	// Clear the tracking map
	r.spawnedProcesses = make(map[int]*ProcessInfo)

	if len(errors) > 0 {
		return fmt.Errorf("process cleanup errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// untrackProcess removes a process from tracking
func (r *ServerRegistry) untrackProcess(pid int) {
	r.processMutex.Lock()
	defer r.processMutex.Unlock()

	if processInfo, exists := r.spawnedProcesses[pid]; exists {
		if r.logger != nil {
			r.logger.Debug("Untracking process", "pid", pid, "server", processInfo.ServerName)
		}
		delete(r.spawnedProcesses, pid)
	}
}

// LoadServerConfig loads a server configuration from a file and returns the parsed config.
// This is a standalone function for loading configuration files without creating a registry.
func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}
