---
title: Configuration Loading
weight: 40 # Fourth example
---

This page details the example found in the `/examples/configuration` directory, demonstrating how to load server settings and tool definitions from external configuration files (JSON, TOML, or YAML) instead of defining them directly in code.

This approach allows for easier management and modification of server capabilities without recompiling the server binary.

## Configuration Server (`examples/configuration/server`)

This example uses the [Viper](https://github.com/spf13/viper) library to read configuration files and then dynamically registers tools based on the loaded definitions.

**Key parts:**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/sse" // Using SSE transport here
	"github.com/localrivet/gomcp/types"
	"github.com/spf13/viper" // For config loading
)

// Generic handler for configured tools (example implementation)
func handleConfiguredTool(toolName string) server.ToolHandlerFunc {
	return func(ctx context.Context, args map[string]interface{}) ([]protocol.Content, error) {
		log.Printf("Executing configured tool '%s' with args: %v", toolName, args)
		// In a real scenario, dispatch to specific logic based on toolName
		responseText := fmt.Sprintf("Executed tool '%s'. Args: %v", toolName, args)
		return []protocol.Content{protocol.TextContent{Type: "text", Text: responseText}}, nil
	}
}

func main() {
	// --- Load Configuration using Viper ---
	viper.SetConfigName("config") // Name of config file (without extension)
	viper.AddConfigPath(".")      // Look in the current directory
	viper.AddConfigPath("../")    // Look in the parent directory (where config.* files are)
	viper.AutomaticEnv()          // Read in environment variables that match

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}
	log.Printf("Using config file: %s", viper.ConfigFileUsed())

	// --- Setup MCP Server ---
	serverInfo := types.Implementation{Name: "config-server", Version: "0.1.0"}
	opts := server.NewServerOptions(serverInfo)
	opts.Capabilities.Tools = &protocol.ToolsCaps{} // Enable tool capability
	srv := server.NewServer(opts)

	// --- Register Tools from Configuration ---
	// Assuming config structure like:
	// tools:
	//   - name: tool1
	//     description: "Description for tool1"
	//     inputSchema: { ... }
	//   - name: tool2
	//     description: "Description for tool2"
	//     inputSchema: { ... }

	var configuredTools []protocol.Tool
	if err := viper.UnmarshalKey("tools", &configuredTools); err != nil {
		log.Fatalf("Unable to decode tools from config: %v", err)
	}

	for _, tool := range configuredTools {
		log.Printf("Registering tool from config: %s", tool.Name)
		// Need a local copy for the closure
		localTool := tool
		if err := srv.RegisterTool(localTool, handleConfiguredTool(localTool.Name)); err != nil {
			log.Printf("WARN: Failed to register tool '%s': %v", localTool.Name, err)
		}
	}

	// --- Setup Transport (SSE+HTTP) and Run ---
	sseServer := sse.NewServer(srv, opts.Logger)
	mux := http.NewServeMux()
	mux.HandleFunc("/events", sseServer.HTTPHandler)
	mux.HandleFunc("/message", srv.HTTPHandler)

	log.Println("Starting config MCP server on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
```

**To Run:**

1. Navigate to `examples/configuration/server`.
2. Ensure one of the config files (`../config.json`, `../config.toml`, `../config.yaml`) exists in the parent directory.
3. Run `go run main.go`. Viper will automatically detect and load the configuration file.

This example demonstrates a powerful pattern for managing complex server setups where capabilities might change frequently or need to be defined outside the main application code.
