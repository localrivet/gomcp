package server

import (
	"net/http"
	"strings"
	"time" // Added for http.Server timeouts

	"github.com/localrivet/gomcp/transport/sse" // Import the sse transport package
	// "github.com/localrivet/gomcp/types" // Logger comes from srv
)

// ServeSSE runs the MCP server, handling connections via Server-Sent Events (SSE)
// over HTTP using the implementation from the transport/sse package.
// It listens on the specified network address (e.g., ":8080").
// The basePath argument defines the URL prefix for the SSE and message endpoints
// (e.g., "/mcp", resulting in "/mcp/sse" and "/mcp/message"). If empty, "/" is used.
func ServeSSE(srv *Server, addr string, basePath string) error {
	logger := srv.logger // Use the server's configured logger

	// Normalize base path
	if basePath == "" {
		basePath = "/"
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	// Ensure trailing slash for Handle registration
	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}

	// Create the SSE handler options
	// Note: transport/sse NewSSEServer expects MCPServerLogic, which *Server implements.
	// It also defines its own default endpoints (/sse, /message) if not specified.
	sseOpts := sse.SSEServerOptions{
		Logger:   logger,
		BasePath: basePath, // Pass the normalized base path
		// ContextFunc: // Optional: Add if context modification per request is needed
	}
	sseHandler := sse.NewSSEServer(srv, sseOpts) // srv satisfies the MCPServerLogic interface

	// Create a new ServeMux and register the handler
	mux := http.NewServeMux()
	mux.Handle(basePath, sseHandler) // Handle base path and all sub-paths

	// Print GoMCP Banner
	printBanner()
	logger.Info("Starting MCP server with SSE transport...")
	logger.Info("Listening on: %s", addr)
	logger.Info("SSE Endpoint Base Path: %s (SSE at %ssse, Messages at %smessage)",
		strings.TrimSuffix(basePath, "/"),
		basePath, // Use the basePath which includes trailing slash here
		basePath) // Use the basePath which includes trailing slash here

	// Configure and start the HTTP server explicitly to set timeouts
	// SSE connections need longer timeouts, especially IdleTimeout.
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
		// Set timeouts appropriate for long-lived SSE connections
		// IdleTimeout prevents the server from keeping idle connections open indefinitely.
		// Set a much longer timeout when potentially used with proxies like mcp-remote.
		// ReadHeaderTimeout is also good practice. WriteTimeout might be less critical for SSE.
		IdleTimeout:       10 * time.Minute, // Increased to 10 minutes
		ReadHeaderTimeout: 30 * time.Second, // Increased slightly
		// WriteTimeout: 60 * time.Second, // Optional: Might interrupt long SSE pushes if set too low
	}

	logger.Info("Starting HTTP server with timeouts (Idle: %v, ReadHeader: %v)...", httpServer.IdleTimeout, httpServer.ReadHeaderTimeout)
	err := httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed { // Ignore ErrServerClosed on graceful shutdown
		logger.Error("HTTP server ListenAndServe error: %v", err)
	}
	return err
}
