package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// Example handlers for resource templates

// Basic resource handler returning a string (like FastMCP's basic resource)
func handleGreeting(ctx *server.Context) (string, error) {
	ctx.Info("Handling greeting request")
	return "Hello from gomcp Resources!", nil
}

// Resource handler returning JSON data (dict in FastMCP)
func handleConfig(ctx *server.Context) (map[string]interface{}, error) {
	ctx.Info("Handling config request")
	return map[string]interface{}{
		"theme":    "dark",
		"version":  "1.2.0",
		"features": []string{"tools", "resources"},
	}, nil
}

// Template with a simple parameter
func handleWeather(ctx *server.Context, city string) (map[string]interface{}, error) {
	ctx.Info("Handling weather request for city: " + city)
	return map[string]interface{}{
		"city":        city,
		"temperature": 22,
		"condition":   "Sunny",
		"unit":        "celsius",
	}, nil
}

// Template with multiple parameters
func handleRepoInfo(ctx *server.Context, owner string, repo string) (map[string]interface{}, error) {
	ctx.Info("Handling repo info request for " + owner + "/" + repo)
	return map[string]interface{}{
		"owner":     owner,
		"name":      repo,
		"full_name": fmt.Sprintf("%s/%s", owner, repo),
		"stars":     120,
		"forks":     48,
	}, nil
}

// Handler with wildcard parameter
func handleDocsResource(ctx *server.Context, path string) (string, error) {
	ctx.Info("Handling docs resource with path: " + path)
	return fmt.Sprintf("Documentation for path: %s", path), nil
}

// Handler with default value parameter
func handleUserProfile(ctx *server.Context, userID string, format string) (interface{}, error) {
	ctx.Info(fmt.Sprintf("Handling user profile for ID: %s in format: %s", userID, format))

	if format == "json" {
		return map[string]interface{}{
			"id":    userID,
			"name":  fmt.Sprintf("User %s", userID),
			"email": fmt.Sprintf("user%s@example.com", userID),
		}, nil
	}

	return fmt.Sprintf("User Profile: ID=%s, Name=User %s, Email=user%s@example.com",
		userID, userID, userID), nil
}

// Complex example of resource handler with multiple parameters and defaults
func handleSearch(ctx *server.Context, query string, maxResults int, includeArchived bool) (map[string]interface{}, error) {
	ctx.Info(fmt.Sprintf("Searching for '%s', max results: %d, include archived: %v", query, maxResults, includeArchived))

	// Simulate search results
	results := []map[string]string{
		{"id": "1", "title": fmt.Sprintf("Result 1 for %s", query)},
		{"id": "2", "title": fmt.Sprintf("Result 2 for %s", query)},
	}

	// Truncate to maxResults
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return map[string]interface{}{
		"query":            query,
		"max_results":      maxResults,
		"include_archived": includeArchived,
		"results":          results,
	}, nil
}

// Tool examples

// Basic calculator tool
func handleAdd(ctx *server.Context, args struct{ A, B int }) (int, error) {
	ctx.Info(fmt.Sprintf("Adding %d + %d", args.A, args.B))
	return args.A + args.B, nil
}

// Tool with progress reporting
func handleProcessItems(ctx *server.Context, args struct{ Items []string }) (map[string]interface{}, error) {
	total := len(args.Items)
	ctx.Info(fmt.Sprintf("Processing %d items", total))

	results := make([]string, 0, total)

	for i, item := range args.Items {
		// Report progress
		ctx.ReportProgress(fmt.Sprintf("Processing item %d of %d", i+1, total), i, total)

		// Process item (simulate work)
		time.Sleep(500 * time.Millisecond)
		results = append(results, item+" (processed)")
	}

	// Report completion
	ctx.ReportProgress("Processing complete", total, total)

	return map[string]interface{}{
		"processed": len(results),
		"results":   results,
	}, nil
}

// Tool with complex input structure
func handleAnalyzeData(ctx *server.Context, args struct {
	Text      string  `json:"text"`
	MaxTokens int     `json:"max_tokens,omitempty"`
	Language  *string `json:"language,omitempty"`
}) (map[string]interface{}, error) {
	ctx.Info("Analyzing text data")

	// Extract language or use default
	language := "english"
	if args.Language != nil {
		language = *args.Language
	}

	// Apply max tokens if specified
	text := args.Text
	if args.MaxTokens > 0 && len(text) > args.MaxTokens {
		text = text[:args.MaxTokens] + "..."
	}

	return map[string]interface{}{
		"text":       text,
		"length":     len(args.Text),
		"language":   language,
		"max_tokens": args.MaxTokens,
	}, nil
}

func main() {
	// Create a temporary directory for example resources
	tempDir, err := os.MkdirTemp("", "gomcp-demo")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create example files
	exampleFile := filepath.Join(tempDir, "example.txt")
	if err := os.WriteFile(exampleFile, []byte("This is an example text file."), 0644); err != nil {
		log.Fatalf("Failed to create example file: %v", err)
	}

	// Create a JSON config file
	configFile := filepath.Join(tempDir, "config.json")
	configData := map[string]interface{}{
		"app_name": "GoMCP Demo",
		"version":  "1.0.0",
		"features": []string{"tools", "resources", "templates"},
		"settings": map[string]interface{}{
			"cache_enabled": true,
			"timeout_ms":    5000,
			"debug":         false,
		},
	}
	configBytes, _ := json.MarshalIndent(configData, "", "  ")
	if err := os.WriteFile(configFile, configBytes, 0644); err != nil {
		log.Fatalf("Failed to create config file: %v", err)
	}

	// Create a docs directory structure
	docsDir := filepath.Join(tempDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		log.Fatalf("Failed to create docs directory: %v", err)
	}

	// Write a sample file in the docs directory
	if err := os.WriteFile(filepath.Join(docsDir, "index.md"),
		[]byte("# Documentation Index\n\nWelcome to the GOMCP API documentation."), 0644); err != nil {
		log.Fatalf("Failed to create docs index file: %v", err)
	}

	// Create a demo server with detailed configuration
	svr := server.NewServer("GOMCP Demo Server").
		// Configure for SSE transport
		AsSSE(":9090", "/mcp").

		// Register resource equivalents to FastMCP examples

		// Basic dynamic resource returning a string
		Resource("resource://greeting",
			server.WithTextContent("Hello from gomcp Resources!"),
			server.WithName("Greeting"),
			server.WithDescription("Provides a simple greeting message."),
			server.WithMimeType("text/plain"),
			server.WithHandler(handleGreeting),
		).

		// Resource returning JSON data
		Resource("data://config",
			server.WithTextContent(`{"theme":"dark","version":"1.2.0","features":["tools","resources"]}`),
			server.WithName("Configuration"),
			server.WithDescription("Provides application configuration as JSON."),
			server.WithMimeType("application/json"),
			server.WithHandler(handleConfig),
		).

		// Static resources
		Resource("file://"+exampleFile,
			server.WithFileContent(exampleFile),
			server.WithName("Example Text File"),
			server.WithDescription("A sample text file."),
			server.WithMimeType("text/plain"),
			server.WithTags("example", "text"),
		).
		Resource("file://"+configFile,
			server.WithFileContent(configFile),
			server.WithName("Config File"),
			server.WithDescription("Application configuration file in JSON format."),
			server.WithMimeType("application/json"),
			server.WithTags("config", "json"),
		).

		// Template with parameter
		Resource("weather://{city}/current",
			server.WithHandler(handleWeather),
			server.WithName("Weather Information"),
			server.WithDescription("Provides weather information for a specific city."),
			server.WithMimeType("application/json"),
		).

		// Template with multiple parameters
		Resource("repos://{owner}/{repo}/info",
			server.WithHandler(handleRepoInfo),
			server.WithName("Repository Information"),
			server.WithDescription("Retrieves information about a GitHub repository."),
			server.WithMimeType("application/json"),
		).

		// Resource with wildcard parameter
		Resource("docs://{path*}",
			server.WithHandler(handleDocsResource),
			server.WithName("Documentation Browser"),
			server.WithDescription("Browse documentation with wildcard path support."),
			server.WithWildcardParam("path"),
			server.WithMimeType("text/plain"),
		).

		// Resource with default value parameter
		Resource("search://{query}",
			server.WithHandler(handleSearch),
			server.WithName("Search"),
			server.WithDescription("Search for resources matching the query string."),
			server.WithDefaultParamValue("maxResults", 10),
			server.WithDefaultParamValue("includeArchived", false),
			server.WithMimeType("application/json"),
		).

		// Multiple URIs for same handler with different configurations
		Resource("users://{userID}/profile",
			server.WithHandler(handleUserProfile),
			server.WithName("User Profile (Default Format)"),
			server.WithDescription("Get user profile in default text format."),
			server.WithDefaultParamValue("format", "text"),
			server.WithMimeType("text/plain"),
		).
		Resource("users://{userID}/profile/json",
			server.WithHandler(handleUserProfile),
			server.WithName("User Profile (JSON)"),
			server.WithDescription("Get user profile in JSON format."),
			server.WithDefaultParamValue("format", "json"),
			server.WithMimeType("application/json"),
		).

		// Directory resource
		Resource("file://"+docsDir,
			server.WithDirectoryListing(docsDir),
			server.WithName("Documentation Files"),
			server.WithDescription("The documentation files directory."),
			server.WithTags("docs", "directory"),
		).

		// Register tools with FastMCP parity

		// Basic calculator tools
		Tool("add", "Add two numbers", handleAdd).
		Tool("process_items", "Process a list of items with progress updates", handleProcessItems).
		Tool("analyze_data", "Analyze text data with optional parameters", handleAnalyzeData).

		// Register prompts
		Prompt("add_two_numbers", "Add two numbers using a tool",
			server.Assistant("You are a helpful assistant that adds two numbers."),
			server.User("What is 2 + 2?"),
		).
		Prompt("search_data", "Search for data matching a query",
			server.Assistant("You are a helpful assistant that can search for information."),
			server.User("Find information about {topic}."),
		).

		// Register roots
		Root(protocol.Root{
			URI:         "file://" + tempDir,
			Kind:        "workspace",
			Name:        "Example Workspace",
			Description: "The root of the example project.",
		})

	// Defer server shutdown
	defer svr.Close()

	// Run the server
	log.Println("Starting GOMCP demo server with SSE transport...")
	log.Println("Server URL: http://localhost:9090/mcp")
	log.Println("Demo server ready with the following resources:")
	log.Println("- Greeting resource: resource://greeting")
	log.Println("- Config resource: data://config")
	log.Println("- Example file: file://" + exampleFile)
	log.Println("- Config file: file://" + configFile)
	log.Println("- Weather template: weather://{city}/current")
	log.Println("- Repository template: repos://{owner}/{repo}/info")
	log.Println("- Documentation wildcard: docs://{path*}")
	log.Println("- Search with defaults: search://{query}")
	log.Println("- User profile (text): users://{userID}/profile")
	log.Println("- User profile (JSON): users://{userID}/profile/json")
	log.Println("- Documentation directory: file://" + docsDir)
	log.Println("\nTry requesting any of these resources or calling tools.")

	// Start a goroutine to add resources dynamically
	go func() {
		// Wait a bit for server to start
		time.Sleep(5 * time.Second)

		// Add dynamic resources after server has started
		log.Println("Adding dynamic resources after server start...")

		// Add a new text resource
		svr.Resource("resource://dynamic-greeting",
			server.WithTextContent("Hello, I was added dynamically!"),
			server.WithName("Dynamic Greeting"),
			server.WithDescription("This resource was added after server startup."),
			server.WithMimeType("text/plain"),
		)
		log.Println("Added dynamic resource: resource://dynamic-greeting")

		// Add a dynamic JSON resource
		dynamicConfig := map[string]interface{}{
			"name":      "Dynamic Config",
			"timestamp": time.Now().Format(time.RFC3339),
			"dynamic":   true,
		}
		jsonBytes, _ := json.MarshalIndent(dynamicConfig, "", "  ")
		svr.Resource("data://dynamic-config",
			server.WithTextContent(string(jsonBytes)),
			server.WithName("Dynamic Configuration"),
			server.WithDescription("This config was added dynamically after server start."),
			server.WithMimeType("application/json"),
		)
		log.Println("Added dynamic resource: data://dynamic-config")

		// Add a dynamic template resource
		svr.Resource("dynamic://{param}/echo",
			server.WithHandler(func(ctx *server.Context, param string) (string, error) {
				return fmt.Sprintf("Echo: %s (from dynamic template)", param), nil
			}),
			server.WithName("Dynamic Echo"),
			server.WithDescription("A template resource added after server startup."),
			server.WithMimeType("text/plain"),
		)
		log.Println("Added dynamic template: dynamic://{param}/echo")
	}()

	if err := svr.Run(); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
