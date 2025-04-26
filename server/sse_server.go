package server

import (
	"context" // Added for graceful shutdown
	"net/http"
	"strings"
	"time" // Added for shutdown timeout

	"github.com/localrivet/gomcp/transport/sse" // Import the sse transport package
	// "github.com/localrivet/gomcp/types" // Logger comes from srv
)

// ServeSSE runs the MCP server, handling connections via Server-Sent Events (SSE)
// over HTTP using the implementation from the transport/sse package.
// It listens on the specified network address (e.g., ":8080").
// The basePath argument defines the URL prefix for the SSE and message endpoints
// (e.g., "/mcp", resulting in "/mcp/sse" and "/mcp/message"). If empty, "/" is used.
// It now accepts a context for graceful shutdown.
func ServeSSE(ctx context.Context, srv *Server, addr string, basePath string) error { // Added context parameter
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

	// Create and start the HTTP server explicitly
	httpSrv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Channel to capture ListenAndServe error
	listenErr := make(chan error, 1)

	// Start the server in a goroutine
	go func() {
		logger.Info("HTTP server starting...")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server ListenAndServe error: %v", err)
			listenErr <- err
		} else {
			listenErr <- nil // Indicate graceful shutdown or no error
		}
		close(listenErr)
	}()

	// Wait for context cancellation or listen error
	select {
	case <-ctx.Done():
		logger.Info("Shutdown signal received, shutting down HTTP server...")
		// Create a deadline context for the shutdown.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // 10-second timeout
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP server graceful shutdown error: %v", err)
			return err // Return shutdown error
		}
		logger.Info("HTTP server shutdown complete.")
		// Wait for the ListenAndServe goroutine to finish processing the ErrServerClosed
		return <-listenErr // Return nil if shutdown was graceful, otherwise the original listen error
	case err := <-listenErr:
		// ListenAndServe failed before context cancellation
		if err != nil {
			logger.Error("HTTP server failed to start or encountered an error: %v", err)
		}
		return err // Return the error from ListenAndServe
	}
}
