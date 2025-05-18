// Package main provides an example of using stdio transport for gomcp
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/localrivet/gomcp/client"
)

func main() {
	// Create a channel to listen for termination signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// For stdio transport, we need to start the server as a separate process
	// that communicates via stdin/stdout pipes
	cmd, err := startServerProcess()
	if err != nil {
		log.Fatalf("Failed to start server process: %v", err)
	}

	// Ensure the server process is killed when the main process exits
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	// Wait a bit for the server to initialize
	time.Sleep(1 * time.Second)

	// Start the client
	go runClient(cmd)

	// Wait for termination signal
	<-signals
	fmt.Println("\nShutdown signal received, exiting...")
}

// startServerProcess starts a separate process for the server
// and returns the exec.Cmd object for controlling it
func startServerProcess() (*exec.Cmd, error) {
	// Create a new executable for the server
	// In a real application, this might be a separate binary
	serverFile := os.Args[0] + ".server"

	// Generate the server executable
	if err := generateServerExecutable(serverFile); err != nil {
		return nil, fmt.Errorf("failed to generate server executable: %w", err)
	}

	// Start the server executable as a separate process
	cmd := exec.Command(serverFile)

	// Connect the server's stdout and stderr to the current process
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// The server will read from stdin and write to stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Start the server process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server process: %w", err)
	}

	fmt.Println("Started server process with PID:", cmd.Process.Pid)

	// Close the stdin pipe since we don't need to write to it from here
	stdin.Close()

	return cmd, nil
}

// generateServerExecutable creates a temporary executable with the server code
func generateServerExecutable(filename string) error {
	// In a real application, this would be a separate binary
	// For this example, we'll create a simple script that runs the server

	// For simplicity, we'll just write a message and exit
	// In a real application, this would be actual server code
	content := `#!/usr/bin/env go run
package main

import (
	"fmt"
	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create a new server with stdio transport
	srv := server.NewServer("stdio-example-server").AsStdio()

	// Register a simple echo tool
	srv.Tool("echo", "Echo the message back", func(ctx *server.Context, args struct {
		Message string ` + "`json:\"message\"`" + `
	}) (map[string]interface{}, error) {
		fmt.Printf("Server received: %s\n", args.Message)
		return map[string]interface{}{
			"message": args.Message,
		}, nil
	})

	// Run the server
	if err := srv.Run(); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
`

	// Write the content to the file
	if err := os.WriteFile(filename, []byte(content), 0755); err != nil {
		return err
	}

	return nil
}

func runClient(cmd *exec.Cmd) {
	// Create a new client connected to the server process's stdin/stdout
	c, err := client.NewClient("stdio-example-client",
		// Additional options would be set here in a real client
		client.WithConnectionTimeout(5*time.Second),
		client.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Call the echo tool
	echoResult, err := c.CallTool("echo", map[string]interface{}{
		"message": "Hello from stdio client!",
	})
	if err != nil {
		log.Fatalf("Echo call failed: %v", err)
	}
	fmt.Printf("Echo result: %v\n", echoResult)

	// Wait a moment to allow printing of results
	time.Sleep(500 * time.Millisecond)
}
