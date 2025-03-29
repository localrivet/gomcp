// This file defines the "filesystem" tool for the example MCP server.
// WARNING: This is a simplified example for demonstration purposes only.
// Exposing filesystem operations requires extreme security considerations
// in a real application to prevent unauthorized access or modification.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings" // Needed for HasPrefix
	"time"    // Needed for ModTime format

	mcp "github.com/localrivet/gomcp"
)

// fileSystemSandbox defines the root directory within which all filesystem
// operations by this tool are restricted. This is a crucial security measure.
const fileSystemSandbox = "./fs_sandbox"

// fileSystemToolDefinition defines the structure and schema for the filesystem tool.
var fileSystemToolDefinition = mcp.ToolDefinition{
	Name:        "filesystem",
	Description: fmt.Sprintf("Performs file operations (list, read, write) within the '%s' sandbox directory.", fileSystemSandbox),
	InputSchema: mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]mcp.PropertyDetail{
			"operation": {Type: "string", Description: "The operation to perform ('list_files', 'read_file', 'write_file')."},
			"path":      {Type: "string", Description: "The relative path within the sandbox directory (e.g., 'mydir/myfile.txt', '.')."},
			"content":   {Type: "string", Description: "The content to write (only required for 'write_file')."},
		},
		Required: []string{"operation", "path"}, // Content requirement depends on operation, checked in executeFileSystem
	},
	OutputSchema: mcp.ToolOutputSchema{
		Type:        "object", // Output structure varies based on the operation
		Description: "The result of the file operation. For 'list_files', contains a 'files' array. For 'read_file', contains 'content'. For 'write_file', contains 'status' and 'message'.",
		// Note: A more precise output schema could use oneOf/anyOf based on the operation,
		// but this simpler schema is sufficient for demonstration.
	},
}

// getSafePath resolves the user-provided relative path against the sandbox directory,
// performs security checks to prevent path traversal attacks (e.g., using '..'),
// and returns the final, validated absolute path.
// Returns an error if validation fails or path resolution encounters issues.
func getSafePath(relativePath string) (string, error) {
	// Ensure the sandbox base directory exists.
	if err := os.MkdirAll(fileSystemSandbox, 0755); err != nil {
		return "", fmt.Errorf("failed to create sandbox directory '%s': %w", fileSystemSandbox, err)
	}

	// Get the absolute path of the sandbox for reliable comparison.
	sandboxAbs, err := filepath.Abs(fileSystemSandbox)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for sandbox: %w", err)
	}

	// --- Security Checks on Input Path ---
	// 1. Disallow absolute paths from the client.
	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("absolute paths are not allowed: '%s'", relativePath)
	}
	// 2. Clean the path (resolves ., removes trailing slashes).
	cleanedRelativePath := filepath.Clean(relativePath)
	// 3. Explicitly check for '..' components *after* cleaning, as an extra precaution.
	//    filepath.Join below should also handle this, but defense in depth is good.
	if strings.Contains(cleanedRelativePath, "..") {
		// This check might be overly strict if symlinks within the sandbox are intended,
		// but is safer for a simple example.
		return "", fmt.Errorf("path cannot contain '..' components: '%s'", relativePath)
	}
	// --- End Security Checks ---

	// Join the absolute sandbox path with the cleaned relative path.
	joinedPath := filepath.Join(sandboxAbs, cleanedRelativePath)

	// Get the absolute representation of the final target path.
	finalAbsPath, err := filepath.Abs(joinedPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for target: %w", err)
	}

	// --- CRITICAL SECURITY CHECK ---
	// Ensure the final resolved path is still prefixed by the absolute sandbox path.
	// This prevents escaping the sandbox via various path manipulation techniques.
	if !strings.HasPrefix(finalAbsPath, sandboxAbs) {
		return "", fmt.Errorf("path '%s' attempts to escape the sandbox directory '%s'", relativePath, fileSystemSandbox)
	}

	// Return the validated, absolute path.
	return finalAbsPath, nil
}

// executeFileSystem contains the logic for the filesystem tool.
// It validates arguments, determines the safe path, performs the requested operation,
// and returns the result or an error payload.
func executeFileSystem(args map[string]interface{}) (interface{}, *mcp.ErrorPayload) {
	// --- Argument Extraction and Basic Validation ---
	opArg, okOp := args["operation"]
	pathArg, okPath := args["path"]

	if !okOp || !okPath {
		return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPInvalidArgument, Message: "Missing required arguments (operation, path)"} // Use MCP code
	}

	operation, okOp := opArg.(string)
	relativePath, okPath := pathArg.(string)

	if !okOp || !okPath {
		return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPInvalidArgument, Message: "Invalid argument types (operation and path must be strings)"} // Use MCP code
	}
	// --- End Argument Extraction ---

	// --- Path Validation ---
	// Get the safe, absolute path within the sandbox. This is crucial for security.
	safePath, err := getSafePath(relativePath)
	if err != nil {
		log.Printf("Path validation failed for relative path '%s': %v", relativePath, err)
		// Return the validation error directly to the client.
		return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPSecurityViolation, Message: err.Error()} // Use MCP code
	}
	log.Printf("Operating on safe path: %s (relative: %s)", safePath, relativePath)
	// --- End Path Validation ---

	// --- Operation Dispatch ---
	switch operation {
	case "list_files":
		// Read directory entries.
		files, err := os.ReadDir(safePath)
		if err != nil {
			log.Printf("Error listing files in '%s': %v", safePath, err)
			return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPOperationFailed, Message: fmt.Sprintf("Failed to list files at path '%s': %v", relativePath, err)} // Use MCP code
		}
		// Format the output as a list of file information maps.
		var fileInfos []map[string]interface{}
		for _, file := range files {
			info, errInfo := file.Info()
			// Handle cases where getting info for a specific entry fails.
			if errInfo != nil {
				log.Printf("Warning: could not get info for entry '%s' in '%s': %v", file.Name(), safePath, errInfo)
				continue // Skip this entry
			}
			fileInfos = append(fileInfos, map[string]interface{}{
				"name":     info.Name(),
				"is_dir":   info.IsDir(),
				"size":     info.Size(),
				"mod_time": info.ModTime().Format(time.RFC3339), // Use standard time format
			})
		}
		// Return the list wrapped in a map under the "files" key.
		return map[string]interface{}{"files": fileInfos}, nil

	case "read_file":
		// First, check if the path exists and is actually a file.
		info, err := os.Stat(safePath)
		if err != nil {
			log.Printf("Error stating file '%s': %v", safePath, err)
			// Handle file not found or other access errors.
			if os.IsNotExist(err) {
				return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPResourceNotFound, Message: fmt.Sprintf("File not found at path '%s'", relativePath)} // Use MCP code
			}
			return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPOperationFailed, Message: fmt.Sprintf("Failed to access path '%s': %v", relativePath, err)} // Use MCP code
		}
		// Ensure it's not a directory.
		if info.IsDir() {
			return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPOperationFailed, Message: fmt.Sprintf("Path '%s' is a directory, cannot read", relativePath)} // Use MCP code
		}

		// Read the file content. Consider adding size limits for large files in real apps.
		contentBytes, err := os.ReadFile(safePath)
		if err != nil {
			log.Printf("Error reading file '%s': %v", safePath, err)
			return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPOperationFailed, Message: fmt.Sprintf("Failed to read file '%s': %v", relativePath, err)} // Use MCP code
		}
		// Return content as a string within a map.
		return map[string]interface{}{"content": string(contentBytes)}, nil

	case "write_file":
		// Extract and validate the 'content' argument specifically for write.
		contentArg, okContent := args["content"]
		if !okContent {
			return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPInvalidArgument, Message: "Missing required argument 'content' for write_file operation"} // Use MCP code
		}
		content, okContent := contentArg.(string)
		if !okContent {
			return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPInvalidArgument, Message: "Invalid argument type for 'content' (must be string)"} // Use MCP code
		}

		// Ensure parent directory exists within the sandbox.
		parentDir := filepath.Dir(safePath)
		// Check parent is still within sandbox *before* creating it (defense in depth)
		sandboxAbsForWrite, _ := filepath.Abs(fileSystemSandbox) // Error already checked in getSafePath
		if !strings.HasPrefix(parentDir, sandboxAbsForWrite) {
			return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPSecurityViolation, Message: fmt.Sprintf("Cannot create parent directory outside sandbox for path '%s'", relativePath)} // Use MCP code
		}
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			log.Printf("Error creating parent directory '%s': %v", parentDir, err)
			return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPOperationFailed, Message: fmt.Sprintf("Failed to create directory structure for path '%s': %v", relativePath, err)} // Use MCP code
		}

		// Prevent writing directly to the sandbox root itself.
		if safePath == sandboxAbsForWrite {
			return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPOperationFailed, Message: "Cannot write directly to the sandbox root directory"} // Use MCP code
		}

		// Write the file content.
		err := os.WriteFile(safePath, []byte(content), 0644) // Use standard file permissions
		if err != nil {
			log.Printf("Error writing file '%s': %v", safePath, err)
			return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPOperationFailed, Message: fmt.Sprintf("Failed to write file '%s': %v", relativePath, err)} // Use MCP code
		}
		// Return a success status message.
		return map[string]interface{}{"status": "success", "message": fmt.Sprintf("Successfully wrote %d bytes to '%s'", len(content), relativePath)}, nil

	default:
		// Handle unknown operation strings.
		return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPInvalidArgument, Message: fmt.Sprintf("Invalid operation '%s'. Use 'list_files', 'read_file', or 'write_file'.", operation)} // Use MCP code
	}
	// --- End Operation Dispatch ---
}
