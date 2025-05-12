package main

import (
	// Added for binary content
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http" // Added for URL parsing
	"os"
	"path/filepath" // Added for tool/template handler reflection
	"sort"
	"strings" // Added for concurrency in handlers
	"sync"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	// For logger interface
	// Added for ClientSession type
	// Added for URI template handling
	// Added for tool argument unmarshalling
)

// LogAdapter wraps a standard logger to implement the types.Logger interface
type LogAdapter struct {
	logger *log.Logger
	level  protocol.LoggingLevel
	mu     sync.Mutex
}

// Debug implements types.Logger Debug method
func (a *LogAdapter) Debug(msg string, args ...interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.IsLevelEnabled(protocol.LogLevelDebug) {
		a.logger.Printf("DEBUG: "+msg, args...)
	}
}

// Info implements types.Logger Info method
func (a *LogAdapter) Info(msg string, args ...interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.IsLevelEnabled(protocol.LogLevelInfo) {
		a.logger.Printf("INFO: "+msg, args...)
	}
}

// Warn implements types.Logger Warn method
func (a *LogAdapter) Warn(msg string, args ...interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.IsLevelEnabled(protocol.LogLevelWarn) {
		a.logger.Printf("WARN: "+msg, args...)
	}
}

// Error implements types.Logger Error method
func (a *LogAdapter) Error(msg string, args ...interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.IsLevelEnabled(protocol.LogLevelError) {
		a.logger.Printf("ERROR: "+msg, args...)
	}
}

// SetLevel implements types.Logger SetLevel method
func (a *LogAdapter) SetLevel(level protocol.LoggingLevel) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.level = level
}

// IsLevelEnabled checks if a particular log level is enabled
func (a *LogAdapter) IsLevelEnabled(level protocol.LoggingLevel) bool {
	return levelToSeverity(a.level) <= levelToSeverity(level)
}

// Helper to map protocol level to an internal severity for comparison
// Higher numbers mean less restrictive (DEBUG > INFO > WARN > ERROR)
func levelToSeverity(level protocol.LoggingLevel) int {
	switch level {
	case protocol.LogLevelDebug:
		return 100 // Most permissive
	case protocol.LogLevelInfo:
		return 80
	case protocol.LogLevelNotice:
		return 70
	case protocol.LogLevelWarn:
		return 50
	case protocol.LogLevelError:
		return 40
	case protocol.LogLevelCritical:
		return 30
	case protocol.LogLevelAlert:
		return 20
	case protocol.LogLevelEmergency:
		return 10 // Least permissive
	default:
		return 80 // Default to INFO level
	}
}

// NewLogAdapter creates a new LogAdapter with the specified log level
func NewLogAdapter(logger *log.Logger, level string) *LogAdapter {
	var logLevel protocol.LoggingLevel
	switch level {
	case "debug":
		logLevel = protocol.LogLevelDebug
	case "info":
		logLevel = protocol.LogLevelInfo
	case "warning", "warn":
		logLevel = protocol.LogLevelWarn
	case "error":
		logLevel = protocol.LogLevelError
	default:
		logLevel = protocol.LogLevelInfo // Default to INFO
	}

	return &LogAdapter{
		logger: logger,
		level:  logLevel,
	}
}

func main() {
	// Configure logging - must be done first
	logDir := "./logs"

	// Create the logs directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("Failed to create logs directory: %v", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	logFileName := fmt.Sprintf("mcp-server-%s.log", timestamp)
	logPath := filepath.Join(logDir, logFileName)

	// Open the log file
	logFile, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close() // Ensure the file is closed when the program exits

	// When using stdio transport, we can ONLY write logs to the file
	// Writing to stdout would corrupt the JSON-RPC communication
	serverLogger := log.New(logFile, "", log.LstdFlags|log.Lshortfile)

	// Redirect standard Go logger to file to capture all logs
	log.SetOutput(logFile)

	// Write to the file that we've started up (can't use stdout)
	serverLogger.Println("Starting Full Featured Server with file logging enabled...")
	serverLogger.Printf("Log file created at: %s", logPath)

	// Set the log level for MCP server - use ERROR to reduce logging
	logLevel := "error" // Use least verbose level for production

	// Create a new MCP server instance
	srv := server.NewServer("FullFeaturedServer")

	// Configure the server with our logger IMMEDIATELY
	logAdapter := NewLogAdapter(serverLogger, logLevel)
	srv.WithLogger(logAdapter)

	// --- Register Resources ---

	// 1. Static Text Resource
	srv.Resource("docs://greeting",
		server.WithTextContent("Hello from the Full Featured Server!"),
		server.WithName("Greeting Message"),
		server.WithDescription("A simple greeting message."),
		server.WithMimeType("text/plain"),
		server.WithTags("documentation", "example", "text"))

	// 2. Static File Resource
	// Create a dummy file for demonstration
	dummyFilePath := filepath.Join(os.TempDir(), "fullserver_dummy.txt")
	if err := os.WriteFile(dummyFilePath, []byte("This is the content of the dummy file."), 0644); err != nil {
		log.Fatalf("Failed to create dummy file: %v", err)
	}
	srv.Resource("file:///temp/dummy.txt",
		server.WithFileContent(dummyFilePath),
		server.WithName("Dummy Text File"),
		server.WithDescription("A static file resource served from a local path."),
		server.WithTags("file", "example"))

	// 3. Static Directory Listing Resource
	// Use the current directory for listing
	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}
	srv.Resource("dir://current",
		server.WithDirectoryListing(currentDir),
		server.WithName("Current Directory Listing"),
		server.WithDescription("Lists the contents of the server's current working directory."),
		server.WithTags("directory", "example"))

	// 4. Static URL Resource
	// Use a well-known public URL for demonstration
	srv.Resource("url://example.com",
		server.WithURLContent("https://example.com"),
		server.WithName("Example Website Content"),
		server.WithDescription("Fetches and serves the content of example.com."),
		server.WithMimeType("text/html"), // Explicitly set MIME type
		server.WithTags("url", "example", "external"))

	// --- Register Resource Templates ---

	// 5. Dynamic Resource Template with Parameters
	// Handler function for the template
	itemHandler := func(ctx *server.Context, itemID string, format string) (string, error) {
		log.Printf("Handling item request for ID: %s, Format: %s", itemID, format)
		// Simulate fetching data based on itemID and format
		data := fmt.Sprintf("Data for item %s in %s format", itemID, format)
		if format == "json" {
			return fmt.Sprintf(`{"id": "%s", "data": "%s"}`, itemID, data), nil
		}
		return data, nil
	}
	srv.Resource("data://items/{itemID}/{format}",
		server.WithHandler(itemHandler),
		server.WithName("Item Data Template"),
		server.WithDescription("Provides data for items based on ID and format."),
		server.WithTags("template", "dynamic", "example"))

	// 6. Dynamic Resource Template with Wildcard Parameter and Default Value
	// Handler function for the template
	pathHandler := func(ctx *server.Context, path string) (string, error) {
		log.Printf("Handling path request for path: %s", path)
		// Simulate reading content from a path
		content := fmt.Sprintf("Content for path: /%s", path)
		return content, nil
	}
	srv.Resource("path://files/{path*}",
		server.WithHandler(pathHandler),
		server.WithDefaultParamValue("path", "index.html"), // Default value for path
		server.WithName("File Path Template"),
		server.WithDescription("Provides content based on a file path, supports wildcards."),
		server.WithTags("template", "dynamic", "wildcard", "example"))

	// --- Register Tools ---

	// 7. Simple Echo Tool
	// Handler function for the tool
	echoToolHandler := func(ctx *server.Context, params json.RawMessage) (interface{}, error) {
		log.Printf("Echo tool received params: %s", string(params))
		// Just echo the received parameters back as the result
		return params, nil
	}
	srv.Tool("echo", "Echoes the input parameters.", echoToolHandler)

	// 8. Tool Demonstrating Context Usage (Reading Resource)
	readResourceToolHandler := func(ctx *server.Context, params struct{ URI string }) (interface{}, error) {
		log.Printf("ReadResource tool received URI: %s", params.URI)
		// Use the context to read another resource
		contents, err := ctx.ReadResource(params.URI)
		if err != nil {
			return nil, fmt.Errorf("failed to read resource %s: %w", params.URI, err)
		}
		// Return the resource contents
		return contents, nil
	}
	srv.Tool("read_resource", "Reads and returns the content of another resource.", readResourceToolHandler)

	// 9. Tool Demonstrating Context Usage (Calling Another Tool)
	callToolToolHandler := func(ctx *server.Context, params protocol.CallToolRequestParams) (interface{}, error) {
		log.Printf("tools/call tool received params: %+v", params)
		// Use the context to call another tool
		output, toolErr, err := ctx.CallTool(params.ToolCall.ToolName, params.ToolCall.Input)
		if err != nil {
			return nil, fmt.Errorf("failed to call tool %s: %w", params.ToolCall.ToolName, err)
		}
		if toolErr != nil {
			return nil, fmt.Errorf("called tool %s returned error: %+v", params.ToolCall.ToolName, toolErr)
		}
		// Return the output of the called tool
		return output, nil
	}
	srv.Tool("call_another_tool", "Calls another tool using the context.", callToolToolHandler)

	// 10. Tool Demonstrating Progress Reporting
	progressToolHandler := func(ctx *server.Context, params struct{ Steps int }) (interface{}, error) {
		log.Printf("Progress tool starting for %d steps", params.Steps)
		for i := 0; i < params.Steps; i++ {
			message := fmt.Sprintf("Step %d of %d", i+1, params.Steps)
			ctx.ReportProgress(message, i+1, params.Steps)
			log.Printf("Progress: %s", message)
			// Simulate work
			// time.Sleep(100 * time.Millisecond) // Uncomment to slow down progress
		}
		log.Println("Progress tool finished")
		return map[string]string{"status": "completed", "steps": fmt.Sprintf("%d", params.Steps)}, nil
	}
	srv.Tool("report_progress", "Demonstrates progress reporting.", progressToolHandler)

	// --- Register Prompts ---

	// 11. Simple Greeting Prompt
	srv.Prompt("greeting_prompt", "A simple prompt for greetings.",
		protocol.PromptMessage{Role: "user", Content: protocol.TextContent{Type: "text", Text: "Hello!"}},
		protocol.PromptMessage{Role: "assistant", Content: protocol.TextContent{Type: "text", Text: "Hi there! How can I help you?"}})

	// 12. Task Description Prompt
	srv.Prompt("task_description_prompt", "A prompt for describing a task.",
		protocol.PromptMessage{Role: "user", Content: protocol.TextContent{Type: "text", Text: "Describe the task."}},
		protocol.PromptMessage{Role: "assistant", Content: protocol.TextContent{Type: "text", Text: "Please provide a detailed description of the task you would like me to perform."}})

	// --- Configure Transport ---

	// Use the Stdio transport for this example
	srv.AsStdio()

	// Run the server
	if runErr := srv.Run(); runErr != nil {
		// Log to file only
		serverLogger.Fatalf("Server failed to run: %v", runErr)
	}
}

// Helper function to infer MIME type (basic implementation)
func inferMimeTypeFromFile(filePath string, data []byte) string {
	// Use http.DetectContentType for basic inference from content
	contentType := http.DetectContentType(data)

	// Fallback to extension-based inference if generic
	if contentType == "application/octet-stream" {
		ext := filepath.Ext(filePath)
		switch ext {
		case ".txt":
			return "text/plain"
		case ".json":
			return "application/json"
		case ".html", ".htm":
			return "text/html"
		case ".css":
			return "text/css"
		case ".js":
			return "application/javascript"
		case ".pdf":
			return "application/pdf"
		case ".png":
			return "image/png"
		case ".jpg", ".jpeg":
			return "image/jpeg"
		case ".gif":
			return "image/gif"
		case ".mp3":
			return "audio/mpeg"
		case ".wav":
			return "audio/wav"
		case ".ogg":
			return "audio/ogg"
		case ".mp4":
			return "video/mp4"
		case ".zip":
			return "application/zip"
		case ".tar":
			return "application/x-tar"
		case ".gz":
			return "application/gzip"
		case ".xml":
			return "application/xml"
		// Add more cases as needed
		default:
			return "application/octet-stream" // Default to generic binary
		}
	}
	return contentType
}

// Helper function to check if MIME type is text-based
func isTextContent(mimeType string) bool {
	return strings.HasPrefix(mimeType, "text/") ||
		mimeType == "application/json" ||
		mimeType == "application/xml" ||
		strings.HasSuffix(mimeType, "+json") || // Handle variations like application/ld+json
		strings.HasSuffix(mimeType, "+xml")
}

// Helper struct for directory listing
type directoryEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"modTime"`
}

// Helper function to generate a directory listing
func generateDirectoryListing(dirPath string) ([]directoryEntry, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	listing := make([]directoryEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			// Log error but continue with other entries
			log.Printf("Warning: Failed to get file info for %s: %v", entry.Name(), err)
			continue
		}
		listing = append(listing, directoryEntry{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}

	// Sort by name for consistent output
	sort.SliceStable(listing, func(i, j int) bool {
		return listing[i].Name < listing[j].Name
	})

	return listing, nil
}

// Helper struct for URL content
type urlContent struct {
	data        []byte
	contentType string
}

// Helper function to fetch URL content
func fetchURLContent(urlStr string) (*urlContent, error) {
	resp, err := http.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch URL %s: received status code %d", urlStr, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read URL response body for %s: %w", urlStr, err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		// Try to detect content type from data if header is missing
		contentType = http.DetectContentType(data)
	}

	return &urlContent{data: data, contentType: contentType}, nil
}
