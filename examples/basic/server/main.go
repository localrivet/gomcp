// examples/basic/server/main.go (Refactored to use AddTool helpers)
// This is the main file for the example MCP server.
package main

import (
	"errors"
	"log"
	"os"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// --- Tool Argument Structs ---

// EchoArgs defines arguments for the echo tool.
type EchoArgs struct {
	Message string `json:"message" description:"The message to echo." required:"true"`
}

// CalculatorArgs is defined in calculator.go
// FileSystemArgs is defined in filesystem.go

// --- Main Function ---
func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Starting Example MCP Server (Basic - Stdio)...")

	logger := logx.NewLogger("basic-server")

	serverName := "GoMultiToolServer-Basic"
	srv := server.NewServer(serverName, server.WithLogger(logger))

	// Register the 'echo' tool using AddTool helper
	err := server.AddTool(
		srv,
		"echo",
		"Echoes back the provided message.",
		// Handler function defined inline
		func(args EchoArgs) (protocol.Content, error) {
			log.Printf("Executing echo tool with message: %s", args.Message)
			if args.Message == "" {
				return nil, errors.New("message cannot be empty")
			}
			successContent := protocol.TextContent{Type: "text", Text: args.Message}
			return successContent, nil
		},
	)
	if err != nil {
		log.Fatalf("Failed to register echo tool: %v", err)
	}

	// Register the 'calculator' tool using AddTool helper and existing handler
	err = server.AddTool(
		srv,
		"calculator",
		"Performs basic arithmetic operations (add, subtract, multiply, divide).",
		executeCalculator,
	)
	if err != nil {
		log.Fatalf("Failed to register calculator tool: %v", err)
	}

	// Register the 'filesystem' tool using AddTool helper and existing handler
	err = server.AddTool(
		srv,
		"filesystem",
		"Performs file operations (list, read, write) within the './fs_sandbox' directory.",
		executeFileSystem,
	)
	if err != nil {
		log.Fatalf("Failed to register filesystem tool: %v", err)
	}

	log.Println("Server setup complete. Listening on stdio...")
	// Start the server using the stdio transport. This blocks.
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
	log.Println("Server shutdown complete.")
}
