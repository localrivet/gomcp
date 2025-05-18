package server

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/localrivet/gomcp/server"
)

// This file contains examples of how to use the MCP server with sampling configuration.
// It is not intended to be used in production, but rather as a reference.

// ExampleServerWithSamplingConfig shows how to set up a server with custom sampling configuration.
func ExampleServerWithSamplingConfig() {
	// Create a custom sampling configuration
	samplingConfig := &server.SamplingConfig{
		MaxRequestsPerMinute:  60,   // 1 request per second
		MaxConcurrentRequests: 5,    // Max 5 concurrent requests
		MaxTokensPerRequest:   4096, // Max 4096 tokens per request

		DefaultTimeout: 45 * time.Second,
		MaxTimeout:     2 * time.Minute,

		DefaultMaxRetries:    3,
		DefaultRetryInterval: 2 * time.Second,

		EnablePrioritization: true,
		DefaultPriority:      5,

		GracefulDegradation: true,

		ResourceQuota: map[string]int{
			"text":  4096,
			"image": 2048,
			"audio": 1024,
		},

		ProtocolDefaults: map[string]*server.ProtocolSamplingConfig{
			"draft": {
				MaxTokens: 2048,
				SupportedContentTypes: map[string]bool{
					"text":  true,
					"image": true,
				},
				StreamingSupported: false,
			},
			"2024-11-05": {
				MaxTokens: 4096,
				SupportedContentTypes: map[string]bool{
					"text":  true,
					"image": true,
				},
				StreamingSupported: false,
			},
			"2025-03-26": {
				MaxTokens: 8192,
				SupportedContentTypes: map[string]bool{
					"text":  true,
					"image": true,
					"audio": true,
				},
				StreamingSupported: true,
			},
		},
	}

	// Create a logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create a server with the custom sampling configuration
	svr := server.NewServer("example-server",
		server.WithLogger(logger),
		server.WithSamplingConfig(samplingConfig),
	)

	// Configure server further if needed
	svr.
		AsSSE("localhost:3000").
		Tool("example", "An example tool", func(ctx *server.Context) (interface{}, error) {
			return "Hello, world!", nil
		})

	// Example of using the sampling API in a tool handler
	svr.Tool("samplingExample", "Example of sampling API", func(ctx *server.Context) (interface{}, error) {
		// Create messages
		messages := []server.SamplingMessage{
			server.CreateTextSamplingMessage("user", "Tell me about MCP"),
		}

		// Create model preferences
		preferences := server.SamplingModelPreferences{
			Hints: []server.SamplingModelHint{
				{Name: "gpt-4"},
			},
		}

		// Get the sampling controller
		controller, err := ctx.GetSamplingController()
		if err != nil {
			return nil, fmt.Errorf("failed to get sampling controller: %w", err)
		}

		// Use the controller to log request metrics
		ctx.Logger.Info("sampling controller status",
			"concurrentRequests", controller.GetConcurrentRequestCount(),
		)

		// Validate the request
		err = ctx.ValidateSamplingRequest(messages, 1000)
		if err != nil {
			return nil, fmt.Errorf("invalid sampling request: %w", err)
		}

		// Send a high-priority sampling request
		result, err := ctx.RequestSamplingWithPriority(
			messages,
			preferences,
			"Be helpful and concise.",
			1000,
			8, // Higher priority (1-10 scale)
		)

		if err != nil {
			return nil, fmt.Errorf("sampling request failed: %w", err)
		}

		return result, nil
	})

	// In a real application, you would call server.Run() here
	fmt.Println("Server configured with sampling support")

	// Output: Server configured with sampling support
}
