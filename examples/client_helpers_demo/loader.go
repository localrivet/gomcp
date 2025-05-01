package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
)

// DefaultHandshakeTimeout is the default timeout for the MCP handshake.
const DefaultHandshakeTimeout = 10 * time.Second

// DefaultProtocolVersion is used when connecting to external servers without explicit version info.
// TODO: Should this be configurable?
const DefaultProtocolVersion = "2024-11-05"

// ManagedClient holds a client connection and its associated managed process.
type ManagedClient struct {
	Client  *client.Client
	Command *exec.Cmd // For stdio-launched processes
}

// LoadServersFromConfig takes a configuration, launches local servers or connects to external ones,
// and returns connected clients along with their process handles (if managed).
func LoadServersFromConfig(ctx context.Context, config client.MCPConfig) (map[string]*ManagedClient, error) {
	loadedClients := make(map[string]*ManagedClient)
	var multiErr error // To collect multiple errors if any

	// Define default protocol version pointer once
	protoVersion := DefaultProtocolVersion

	for name, serverCfg := range config.MCPServers {
		log.Printf("Processing configuration for server: %s", name)

		var clt *client.Client
		var cmd *exec.Cmd // Store the command for managed processes
		var err error
		commandPath := serverCfg.Command
		args := serverCfg.Args
		// TODO: Populate other options from serverCfg if added later
		options := client.ClientOptions{}

		switch {
		case strings.HasPrefix(commandPath, "@sse:"):
			fullURL := strings.TrimPrefix(commandPath, "@sse:")
			parsedURL, err := url.Parse(fullURL)
			if err != nil {
				err = fmt.Errorf("invalid SSE URL for '%s': %w", name, err)
				break
			}
			baseURL := parsedURL.Scheme + "://" + parsedURL.Host
			basePath := parsedURL.Path
			log.Printf("Configuring external SSE client for %s (BaseURL: %s, BasePath: %s)", name, baseURL, basePath)

			// Set preferred protocol version in options
			options.PreferredProtocolVersion = &protoVersion

			clt, err = client.NewSSEClient(name, baseURL, basePath, options)
			if err == nil {
				connCtx, connCancel := context.WithTimeout(ctx, DefaultHandshakeTimeout)
				err = clt.Connect(connCtx)
				connCancel()
			}

		case strings.HasPrefix(commandPath, "@ws:"):
			fullURL := strings.TrimPrefix(commandPath, "@ws:")
			// WebSocket URLs are opaque, treat the whole thing as the address needed by the WS client constructor
			// (Assuming NewWebSocketClient handles the full ws:// or wss:// URL)
			log.Printf("Configuring external WebSocket client for %s at address: %s", name, fullURL)

			// Set preferred protocol version in options
			options.PreferredProtocolVersion = &protoVersion

			// Assuming NewWebSocketClient takes the full URL directly as the third argument ('address')
			// Let's double-check its signature if possible, but proceed with this assumption for now.
			clt, err = client.NewWebSocketClient(name, DefaultProtocolVersion /*<- This arg might be wrong*/, fullURL, options)
			if err != nil {
				// Check if the error indicates wrong number of args
				log.Printf("DEBUG: Potential arg mismatch for NewWebSocketClient? Error: %v", err)
			}
			if err == nil {
				connCtx, connCancel := context.WithTimeout(ctx, DefaultHandshakeTimeout)
				err = clt.Connect(connCtx)
				connCancel()
			}

		case strings.HasPrefix(commandPath, "@docker:"):
			imageName := strings.TrimPrefix(commandPath, "@docker:")
			if imageName == "" {
				err = fmt.Errorf("docker image name missing after @docker: for server '%s'", name)
				break
			}
			log.Printf("Configuring Docker container client for %s using image: %s", name, imageName)
			dockerArgs := []string{"run", "-i", "--rm"}
			for key, val := range serverCfg.Env {
				dockerArgs = append(dockerArgs, "-e", fmt.Sprintf("%s=%s", key, val))
			}
			dockerArgs = append(dockerArgs, imageName)
			dockerArgs = append(dockerArgs, args...)
			commandPath = "docker"
			args = dockerArgs
			log.Printf("Preparing Docker command: %s %v", commandPath, args)
			fallthrough

		case strings.HasPrefix(commandPath, "@stdio:"):
			fullCmdString := strings.TrimPrefix(commandPath, "@stdio:")
			if fullCmdString == "" {
				err = fmt.Errorf("command missing after @stdio: for server '%s'", name)
				break // Break from switch
			}
			// Basic command parsing (assumes space separation, doesn't handle quotes well)
			// TODO: Use a more robust shell word splitting library if needed
			parts := strings.Fields(fullCmdString)
			actualCommand := parts[0]
			actualArgs := []string{}
			if len(parts) > 1 {
				actualArgs = parts[1:]
			}
			// Combine with args from config? For now, assume @stdio: contains the full command+args.
			// args = append(actualArgs, serverCfg.Args...) // Optional merge strategy

			log.Printf("Launching stdio process client for %s: %s %v", name, actualCommand, actualArgs)

			cmd = exec.CommandContext(ctx, actualCommand, actualArgs...) // Assign to the outer cmd variable

			// Set environment variables (if any defined in config)
			if len(serverCfg.Env) > 0 {
				cmd.Env = os.Environ()
				for key, val := range serverCfg.Env {
					cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
				}
			}

			// Get stdin/stdout pipes
			var stdin io.WriteCloser
			var stdout io.ReadCloser
			stdin, err = cmd.StdinPipe()
			if err != nil {
				err = fmt.Errorf("failed to get stdin pipe for '%s': %w", name, err)
				break
			}
			stdout, err = cmd.StdoutPipe()
			if err != nil {
				err = fmt.Errorf("failed to get stdout pipe for '%s': %w", name, err)
				_ = stdin.Close()
				break
			}

			// Optional: Stderr handling (same as before)
			stderrPipe, stderrErr := cmd.StderrPipe()
			if stderrErr != nil {
				log.Printf("Warning: Failed to get stderr pipe for '%s': %v", name, stderrErr)
			} else {
				go func(pName string, pipe io.ReadCloser) {
					// ... (stderr logging goroutine, same as before)
					defer pipe.Close()
					buf := make([]byte, 1024)
					for {
						n, readErr := pipe.Read(buf)
						if n > 0 {
							log.Printf("[%s-stderr] %s", pName, string(buf[:n]))
						}
						if readErr != nil {
							if readErr != io.EOF {
								log.Printf("Error reading stderr for '%s': %v", pName, readErr)
							}
							break
						}
					}
				}(name, stderrPipe)
			}

			// Start the process
			err = cmd.Start()
			if err != nil {
				err = fmt.Errorf("failed to start process for '%s': %w", name, err)
				_ = stdin.Close()
				_ = stdout.Close()
				break
			}
			log.Printf("Process started for '%s' (PID: %d)", name, cmd.Process.Pid)

			// Use the placeholder function for client creation
			// TODO: Use options from serverCfg if available
			clt, err = newClientFromPipes(name, DefaultProtocolVersion, stdout, stdin, options)
			if err != nil {
				// Error already includes context, just need to handle process cleanup
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				break
			}

			// Connect the client (performs handshake)
			connCtx, connCancel := context.WithTimeout(ctx, DefaultHandshakeTimeout)
			err = clt.Connect(connCtx)
			connCancel()
			if err != nil {
				err = fmt.Errorf("failed to connect stdio client for '%s': %w", name, err)
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				_ = clt.Close() // Close the client side
				break
			}

			// Goroutine to wait for the process to exit (same as before)
			go func(pName string, pCmd *exec.Cmd) {
				// ... (process wait goroutine, same as before)
				waitErr := pCmd.Wait()
				if waitErr != nil { /* log exit error */
				} else { /* log success */
				}
			}(name, cmd)

		default:
			log.Printf("Warning: Skipping server configuration '%s'. Command '%s' is not a recognized type (@stdio:command, @sse:url, @ws:url).", name, commandPath)
			continue
		}

		if err != nil {
			log.Printf("ERROR: Failed to load server '%s': %v", name, err)
			if multiErr == nil {
				multiErr = fmt.Errorf("failed to load server '%s': %w", name, err)
			} else {
				multiErr = fmt.Errorf("%w; failed to load server '%s': %v", multiErr, name, err)
			}
			continue
		}

		if clt != nil {
			loadedClients[name] = &ManagedClient{
				Client:  clt,
				Command: cmd,
			}
			log.Printf("Successfully loaded and connected client for server: %s", name)
		} else {
			log.Printf("Warning: Client for server '%s' is nil despite no error reported.", name)
		}
	}

	return loadedClients, multiErr
}

// newClientFromPipes placeholder... (same as before)
func newClientFromPipes(name string, protocolVersion string, reader io.ReadCloser, writer io.WriteCloser, options client.ClientOptions) (*client.Client, error) {
	log.Printf("Placeholder: Would create client '%s' from pipes here.", name)
	return nil, fmt.Errorf("client creation from pipes not yet implemented (using placeholder for %s)", name)
}

// CallToolByName simplifies calling a tool by handling input/output marshalling.
// InputType: The struct type for the tool's input arguments.
// OutputType: The struct type expected in the tool's result content (assuming JSON).
func CallToolByName[InputType any, OutputType any](
	ctx context.Context,
	clt *client.Client,
	toolName string,
	input InputType, // Use the specific input type
) (*OutputType, error) {

	// 1. Marshal the input struct to JSON bytes
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input for tool '%s': %w", toolName, err)
	}

	// 2. Unmarshal input JSON bytes into the expected map[string]interface{} format for arguments
	var argumentsMap map[string]interface{}
	// Handle case where input might be an empty struct or nil, resulting in "null"
	if string(inputBytes) == "null" || string(inputBytes) == "{}" {
		// If input is effectively empty, send nil or an empty map based on what tools expect
		// Sending nil might be safer if tools check for nil args.
		argumentsMap = nil // Or: argumentsMap = make(map[string]interface{})
	} else {
		err = json.Unmarshal(inputBytes, &argumentsMap)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal input into map for tool '%s': %w", toolName, err)
		}
	}

	// 3. Prepare CallTool parameters
	callParams := protocol.CallToolParams{
		Name:      toolName,
		Arguments: argumentsMap, // Assign the map
	}

	// 4. Call the tool
	log.Printf("Calling tool '%s' with input: %s", toolName, string(inputBytes)) // Log original marshalled input
	result, err := clt.CallTool(ctx, callParams, nil)                            // Assuming no stream handler for now
	if err != nil {
		return nil, fmt.Errorf("failed to call tool '%s': %w", toolName, err)
	}

	// 5. Process the result content (assuming first content block is the desired output)
	if len(result.Content) == 0 {
		var zero OutputType
		if any(zero) == nil { // Check if OutputType is like *SomeStruct or interface{}
			return nil, nil // Return nil, nil if output can be nil
		}
		return nil, fmt.Errorf("tool '%s' returned empty content, expected %T", toolName, *new(OutputType))
	}

	// Assume the first content block holds the result (e.g., TextContent with JSON)
	var outputBytes []byte
	switch content := result.Content[0].(type) {
	case protocol.TextContent:
		outputBytes = []byte(content.Text)
	default:
		return nil, fmt.Errorf("tool '%s' returned unexpected content type: %T", toolName, result.Content[0])
	}

	// 6. Unmarshal the result JSON into the target OutputType struct
	var output OutputType
	err = json.Unmarshal(outputBytes, &output)
	if err != nil {
		log.Printf("Raw output from tool '%s': %s", toolName, string(outputBytes))
		return nil, fmt.Errorf("failed to unmarshal result from tool '%s' into %T: %w", toolName, *new(OutputType), err)
	}

	log.Printf("Successfully called tool '%s' and parsed output.", toolName)
	return &output, nil
}

// TerminateManagedClients closes client connections and terminates associated processes.
func TerminateManagedClients(clients map[string]*ManagedClient) {
	log.Println("Terminating managed clients and processes...")
	for name, mc := range clients {
		if mc.Client != nil {
			log.Printf("Closing client connection for '%s'", name)
			_ = mc.Client.Close()
		}
		if mc.Command != nil && mc.Command.Process != nil {
			log.Printf("Terminating process for '%s' (PID: %d)...", name, mc.Command.Process.Pid)
			// Send SIGTERM first for graceful shutdown
			if err := mc.Command.Process.Signal(syscall.SIGTERM); err != nil {
				// Log error, but still try SIGKILL if SIGTERM fails (e.g., process already gone)
				if !strings.Contains(err.Error(), "process already finished") {
					log.Printf("Failed to send SIGTERM to '%s': %v. Attempting SIGKILL.", name, err)
				}
				if killErr := mc.Command.Process.Kill(); killErr != nil {
					// Log kill error only if it's not due to the process being gone already
					if !strings.Contains(killErr.Error(), "process already finished") {
						log.Printf("Failed to send SIGKILL to '%s': %v", name, killErr)
					}
				}
			} else {
				log.Printf("Sent SIGTERM to '%s'", name)
			}
			// The goroutine started in LoadServersFromConfig will call cmd.Wait()
		} else if mc.Command != nil {
			log.Printf("Process command exists for '%s' but process is nil (already exited?)", name)
		}
	}
	log.Println("Finished terminating managed clients.")
}
