// This file defines the "filesystem" tool for the example MCP server.
// WARNING: This is a simplified example for demonstration purposes only.
// Exposing filesystem operations requires extreme security considerations
// in a real application to prevent unauthorized access or modification.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings" // Needed for HasPrefix
	"time"    // Needed for ModTime format

	"github.com/localrivet/gomcp/protocol" // Use protocol package
	"github.com/localrivet/gomcp/server"   // Needed for ToolHandlerFunc type matching
)

// fileSystemSandbox defines the root directory within which all filesystem
// operations by this tool are restricted. This is a crucial security measure.
const fileSystemSandbox = "./fs_sandbox"

// fileSystemToolDefinition defines the structure and schema for the filesystem tool.
var fileSystemToolDefinition = protocol.Tool{ // Use protocol.Tool
	Name:        "filesystem",
	Description: fmt.Sprintf("Performs file operations (list, read, write) within the '%s' sandbox directory.", fileSystemSandbox),
	InputSchema: protocol.ToolInputSchema{ // Use protocol.ToolInputSchema
		Type: "object",
		Properties: map[string]protocol.PropertyDetail{ // Use protocol.PropertyDetail
			"operation": {Type: "string", Description: "The operation to perform ('list_files', 'read_file', 'write_file')."},
			"path":      {Type: "string", Description: "The relative path within the sandbox directory (e.g., 'mydir/myfile.txt', '.')."},
			"content":   {Type: "string", Description: "The content to write (only required for 'write_file')."},
		},
		Required: []string{"operation", "path"},
	},
	// Annotations: protocol.ToolAnnotations{}, // Optional
}

// getSafePath resolves the user-provided relative path against the sandbox directory,
// performs security checks to prevent path traversal attacks (e.g., using '..'),
// and returns the final, validated absolute path.
// Returns an error if validation fails or path resolution encounters issues.
func getSafePath(relativePath string) (string, error) {
	if err := os.MkdirAll(fileSystemSandbox, 0755); err != nil {
		return "", fmt.Errorf("failed to create sandbox directory '%s': %w", fileSystemSandbox, err)
	}

	sandboxAbs, err := filepath.Abs(fileSystemSandbox)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for sandbox: %w", err)
	}

	// --- Security Checks on Input Path ---
	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("absolute paths are not allowed: '%s'", relativePath)
	}
	cleanedRelativePath := filepath.Clean(relativePath)
	if strings.Contains(cleanedRelativePath, "..") {
		return "", fmt.Errorf("path cannot contain '..' components: '%s'", relativePath)
	}
	// --- End Security Checks ---

	joinedPath := filepath.Join(sandboxAbs, cleanedRelativePath)

	finalAbsPath, err := filepath.Abs(joinedPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for target: %w", err)
	}

	// --- CRITICAL SECURITY CHECK ---
	if !strings.HasPrefix(finalAbsPath, sandboxAbs) {
		return "", fmt.Errorf("path '%s' attempts to escape the sandbox directory '%s'", relativePath, fileSystemSandbox)
	}

	return finalAbsPath, nil
}

// executeFileSystem contains the logic for the filesystem tool.
// It validates arguments, determines the safe path, performs the requested operation,
// and returns the result content and isError status.
// It now matches the ToolHandlerFunc signature.
func executeFileSystem(ctx context.Context, progressToken *protocol.ProgressToken, args any) (content []protocol.Content, isError bool) { // Use protocol types
	// Helper to create error response content
	newErrorContent := func(msg string) []protocol.Content { // Use protocol.Content
		return []protocol.Content{protocol.TextContent{Type: "text", Text: msg}} // Use protocol.TextContent
	}

	// --- Argument Extraction and Basic Validation ---
	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return newErrorContent("Invalid arguments for filesystem tool (expected object)"), true
	}

	opArg, okOp := argsMap["operation"]
	pathArg, okPath := argsMap["path"]

	if !okOp || !okPath {
		return newErrorContent("Missing required arguments (operation, path)"), true
	}

	operation, okOp := opArg.(string)
	relativePath, okPath := pathArg.(string)

	if !okOp || !okPath {
		return newErrorContent("Invalid argument types (operation and path must be strings)"), true
	}
	// --- End Argument Extraction ---

	// --- Path Validation ---
	safePath, err := getSafePath(relativePath)
	if err != nil {
		log.Printf("Path validation failed for relative path '%s': %v", relativePath, err)
		return newErrorContent(err.Error()), true
	}
	log.Printf("Operating on safe path: %s (relative: %s)", safePath, relativePath)
	// --- End Path Validation ---

	// --- Operation Dispatch ---
	switch operation {
	case "list_files":
		files, err := os.ReadDir(safePath)
		if err != nil {
			log.Printf("Error listing files in '%s': %v", safePath, err)
			return newErrorContent(fmt.Sprintf("Failed to list files at path '%s': %v", relativePath, err)), true
		}
		var fileInfos []map[string]interface{}
		for _, file := range files {
			info, errInfo := file.Info()
			if errInfo != nil {
				log.Printf("Warning: could not get info for entry '%s' in '%s': %v", file.Name(), safePath, errInfo)
				continue
			}
			fileInfos = append(fileInfos, map[string]interface{}{
				"name":     info.Name(),
				"is_dir":   info.IsDir(),
				"size":     info.Size(),
				"mod_time": info.ModTime().Format(time.RFC3339),
			})
		}
		resultBytes, _ := json.Marshal(map[string]interface{}{"files": fileInfos})
		return []protocol.Content{protocol.TextContent{Type: "text", Text: string(resultBytes)}}, false // Use protocol types

	case "read_file":
		info, err := os.Stat(safePath)
		if err != nil {
			log.Printf("Error stating file '%s': %v", safePath, err)
			if os.IsNotExist(err) {
				return newErrorContent(fmt.Sprintf("File not found at path '%s'", relativePath)), true
			}
			return newErrorContent(fmt.Sprintf("Failed to access path '%s': %v", relativePath, err)), true
		}
		if info.IsDir() {
			return newErrorContent(fmt.Sprintf("Path '%s' is a directory, cannot read", relativePath)), true
		}

		contentBytes, err := os.ReadFile(safePath)
		if err != nil {
			log.Printf("Error reading file '%s': %v", safePath, err)
			return newErrorContent(fmt.Sprintf("Failed to read file '%s': %v", relativePath, err)), true
		}
		return []protocol.Content{protocol.TextContent{Type: "text", Text: string(contentBytes)}}, false // Use protocol types

	case "write_file":
		contentArg, okContent := argsMap["content"]
		if !okContent {
			return newErrorContent("Missing required argument 'content' for write_file operation"), true
		}
		contentStr, okContent := contentArg.(string) // Renamed variable to avoid shadowing
		if !okContent {
			return newErrorContent("Invalid argument type for 'content' (must be string)"), true
		}

		parentDir := filepath.Dir(safePath)
		sandboxAbsForWrite, _ := filepath.Abs(fileSystemSandbox)
		if !strings.HasPrefix(parentDir, sandboxAbsForWrite) {
			return newErrorContent(fmt.Sprintf("Cannot create parent directory outside sandbox for path '%s'", relativePath)), true
		}
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			log.Printf("Error creating parent directory '%s': %v", parentDir, err)
			return newErrorContent(fmt.Sprintf("Failed to create directory structure for path '%s': %v", relativePath, err)), true
		}

		if safePath == sandboxAbsForWrite {
			return newErrorContent("Cannot write directly to the sandbox root directory"), true
		}

		err := os.WriteFile(safePath, []byte(contentStr), 0644) // Use contentStr
		if err != nil {
			log.Printf("Error writing file '%s': %v", safePath, err)
			return newErrorContent(fmt.Sprintf("Failed to write file '%s': %v", relativePath, err)), true
		}
		resultMap := map[string]interface{}{"status": "success", "message": fmt.Sprintf("Successfully wrote %d bytes to '%s'", len(contentStr), relativePath)} // Use contentStr
		resultBytes, _ := json.Marshal(resultMap)
		return []protocol.Content{protocol.TextContent{Type: "text", Text: string(resultBytes)}}, false // Use protocol types

	default:
		return newErrorContent(fmt.Sprintf("Invalid operation '%s'. Use 'list_files', 'read_file', or 'write_file'.", operation)), true
	}
	// --- End Operation Dispatch ---
}

// Ensure executeFileSystem matches the expected handler type
var _ server.ToolHandlerFunc = executeFileSystem
