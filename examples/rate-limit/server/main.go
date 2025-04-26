package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/types" // Keep for RateLimiter
	"golang.org/x/time/rate"            // Import for rate limiting
)

// --- Rate Limiter ---
type RateLimiter struct {
	limiters map[string]*rate.Limiter // Map session ID to limiter
	mu       sync.Mutex
	rate     rate.Limit
	burst    int
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     r,
		burst:    b,
	}
}

func (rl *RateLimiter) GetLimiter(sessionID string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	limiter, exists := rl.limiters[sessionID]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[sessionID] = limiter
	}
	return limiter
}

func (rl *RateLimiter) Allow(sessionID string) bool {
	return rl.GetLimiter(sessionID).Allow()
}

// --- Limited Echo Tool ---
var limitedEchoTool = protocol.Tool{
	Name:        "limited-echo",
	Description: "Echoes back message, subject to rate limiting.",
	InputSchema: protocol.ToolInputSchema{
		Type: "object",
		Properties: map[string]protocol.PropertyDetail{
			"message": {Type: "string", Description: "The message to echo."},
		},
		Required: []string{"message"},
	},
}

// limitedEchoHandlerFactory creates a handler that uses the rate limiter.
func limitedEchoHandlerFactory(limiter *RateLimiter, logger types.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, progressToken *protocol.ProgressToken, arguments any) (content []protocol.Content, isError bool) {
		// We need the session ID to apply the correct rate limit.
		// This information isn't directly available in the ToolHandlerFunc signature.
		// A potential solution is to inject session info into the context via a middleware
		// or modify the handler signature (major change).
		// For this example, we'll log a warning and skip rate limiting.
		// A real implementation would need a way to get the session ID here.
		logger.Warn("Rate limiting skipped: Session ID not available in handler context.")

		/* // Ideal rate limiting check (requires session ID):
		sessionID := getSessionIDFromContext(ctx) // Hypothetical function
		if !limiter.Allow(sessionID) {
			logger.Warn("Rate limit exceeded for session %s", sessionID)
			return []protocol.Content{protocol.TextContent{Type: "text", Text: "Rate limit exceeded. Please try again later."}}, true
		}
		*/

		args, ok := arguments.(map[string]interface{})
		if !ok {
			return []protocol.Content{protocol.TextContent{Type: "text", Text: "Invalid arguments"}}, true
		}
		message, ok := args["message"].(string)
		if !ok {
			return []protocol.Content{protocol.TextContent{Type: "text", Text: "Missing 'message' argument"}}, true
		}

		logger.Info("Executing limited-echo for message: %s", message)
		return []protocol.Content{protocol.TextContent{Type: "text", Text: fmt.Sprintf("Limited Echo: %s", message)}}, false
	}
}

// --- Default Logger ---
type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, args ...interface{}) { log.Printf("DEBUG: "+msg, args...) }
func (l *defaultLogger) Info(msg string, args ...interface{})  { log.Printf("INFO: "+msg, args...) }
func (l *defaultLogger) Warn(msg string, args ...interface{})  { log.Printf("WARN: "+msg, args...) }
func (l *defaultLogger) Error(msg string, args ...interface{}) { log.Printf("ERROR: "+msg, args...) }

var _ types.Logger = (*defaultLogger)(nil)

func NewDefaultLogger() *defaultLogger { return &defaultLogger{} }

// --- Main ---
func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	logger := NewDefaultLogger()
	log.Println("Starting Rate Limit Example MCP Server...")

	// Create rate limiter (e.g., 1 request every 2 seconds, burst of 3)
	limiter := NewRateLimiter(rate.Every(2*time.Second), 3)

	// Create server instance
	srv := server.NewServer("GoRateLimitServer", server.WithLogger(logger)) // Use functional option

	// Register tool with rate limiting handler
	// Note: Passing logger here, but handler currently can't use session-specific logger easily
	handler := limitedEchoHandlerFactory(limiter, logger)
	if err := srv.RegisterTool(limitedEchoTool, handler); err != nil {
		log.Fatalf("Failed to register limited-echo tool: %v", err)
	}

	// Run the server using ServeStdio
	log.Println("Server listening on stdio...")
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}

	log.Println("Server shutdown complete.")
}
