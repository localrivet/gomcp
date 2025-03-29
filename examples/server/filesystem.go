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

const fileSystemSandbox = "./fs_sandbox" // Restrict operations to this directory

// fileSystemToolDefinition defines the file system tool.
var fileSystemToolDefinition = mcp.ToolDefinition{
	Name:        "filesystem",
	Description: fmt.Sprintf("Performs file operations (list, read, write) within the '%s' sandbox directory.", fileSystemSandbox),
	InputSchema: mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]mcp.PropertyDetail{
			"operation": {Type: "string", Description: "The operation to perform ('list_files', 'read_file', 'write_file')."},
			"path":      {Type: "string", Description: "The relative path within the sandbox directory."},
			"content":   {Type: "string", Description: "The content to write (only for 'write_file')."},
		},
		Required: []string{"operation", "path"}, // Content is only required for write
	},
	OutputSchema: mcp.ToolOutputSchema{
		Type:        "object", // Output varies based on operation
		Description: "The result of the file operation (list of files, file content, or success message).",
		// Note: A more precise output schema could use oneOf based on the operation.
	},
}

// Helper to safely join the sandbox path with the user-provided relative path.
// Returns the absolute path and an error if the path tries to escape the sandbox.
func getSafePath(relativePath string) (string, error) {
	// Ensure sandbox exists
	if err := os.MkdirAll(fileSystemSandbox, 0755); err != nil {
		return "", fmt.Errorf("failed to create sandbox directory '%s': %w", fileSystemSandbox, err)
	}

	// Get absolute path of sandbox
	sandboxAbs, err := filepath.Abs(fileSystemSandbox)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for sandbox: %w", err)
	}

	// Clean the relative path to prevent tricks like '..' or absolute paths
	// filepath.Clean resolves '..' but doesn't prevent starting with '/'
	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("absolute paths are not allowed: '%s'", relativePath)
	}
	cleanedRelativePath := filepath.Clean(relativePath)
	// Double check for '..' after cleaning, though Join should handle it.
	if strings.Contains(cleanedRelativePath, "..") {
		return "", fmt.Errorf("path cannot contain '..': '%s'", relativePath)
	}

	// Join the sandbox path and the cleaned relative path
	joinedPath := filepath.Join(sandboxAbs, cleanedRelativePath)

	// Get the absolute version of the joined path
	finalAbsPath, err := filepath.Abs(joinedPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for target: %w", err)
	}

	// CRITICAL SECURITY CHECK: Ensure the final path is still within the sandbox
	// Check prefix AND ensure it's not exactly the sandbox path if op needs a file/subdir
	if !strings.HasPrefix(finalAbsPath, sandboxAbs) {
		return "", fmt.Errorf("path '%s' attempts to escape the sandbox directory '%s'", relativePath, fileSystemSandbox)
	}

	return finalAbsPath, nil
}

// executeFileSystem performs the requested file operation.
func executeFileSystem(args map[string]interface{}) (interface{}, *mcp.ErrorPayload) {
	opArg, okOp := args["operation"]
	pathArg, okPath := args["path"]

	if !okOp || !okPath {
		return nil, &mcp.ErrorPayload{Code: "InvalidArgument", Message: "Missing required arguments (operation, path)"}
	}

	operation, okOp := opArg.(string)
	relativePath, okPath := pathArg.(string)

	if !okOp || !okPath {
		return nil, &mcp.ErrorPayload{Code: "InvalidArgument", Message: "Invalid argument types (operation and path must be strings)"}
	}

	// Get and validate the absolute path within the sandbox
	safePath, err := getSafePath(relativePath)
	if err != nil {
		log.Printf("Path validation failed: %v", err)
		return nil, &mcp.ErrorPayload{Code: "SecurityViolation", Message: err.Error()}
	}
	log.Printf("Operating on safe path: %s (relative: %s)", safePath, relativePath)

	switch operation {
	case "list_files":
		files, err := os.ReadDir(safePath)
		if err != nil {
			log.Printf("Error listing files in '%s': %v", safePath, err)
			return nil, &mcp.ErrorPayload{Code: "OperationFailed", Message: fmt.Sprintf("Failed to list files at path '%s': %v", relativePath, err)}
		}
		var fileInfos []map[string]interface{}
		for _, file := range files {
			info, errInfo := file.Info()
			// Handle error getting file info, skip if problematic
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
		return map[string]interface{}{"files": fileInfos}, nil

	case "read_file":
		// Ensure it's not a directory
		info, err := os.Stat(safePath)
		if err != nil {
			log.Printf("Error stating file '%s': %v", safePath, err)
			return nil, &mcp.ErrorPayload{Code: "OperationFailed", Message: fmt.Sprintf("Failed to access path '%s': %v", relativePath, err)}
		}
		if info.IsDir() {
			return nil, &mcp.ErrorPayload{Code: "OperationFailed", Message: fmt.Sprintf("Path '%s' is a directory, cannot read", relativePath)}
		}

		contentBytes, err := os.ReadFile(safePath)
		if err != nil {
			log.Printf("Error reading file '%s': %v", safePath, err)
			return nil, &mcp.ErrorPayload{Code: "OperationFailed", Message: fmt.Sprintf("Failed to read file '%s': %v", relativePath, err)}
		}
		// Limit file size read? For now, read all.
		return map[string]interface{}{"content": string(contentBytes)}, nil

	case "write_file":
		contentArg, okContent := args["content"]
		if !okContent {
			return nil, &mcp.ErrorPayload{Code: "InvalidArgument", Message: "Missing required argument 'content' for write_file operation"}
		}
		content, okContent := contentArg.(string)
		if !okContent {
			return nil, &mcp.ErrorPayload{Code: "InvalidArgument", Message: "Invalid argument type for 'content' (must be string)"}
		}

		// Ensure parent directory exists
		parentDir := filepath.Dir(safePath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			log.Printf("Error creating parent directory '%s': %v", parentDir, err)
			return nil, &mcp.ErrorPayload{Code: "OperationFailed", Message: fmt.Sprintf("Failed to create directory structure for path '%s': %v", relativePath, err)}
		}

		// Check if path is trying to write to the sandbox root itself
		sandboxAbs, _ := filepath.Abs(fileSystemSandbox) // Error already checked in getSafePath
		if safePath == sandboxAbs {
			return nil, &mcp.ErrorPayload{Code: "OperationFailed", Message: "Cannot write directly to the sandbox root directory"}
		}

		err := os.WriteFile(safePath, []byte(content), 0644)
		if err != nil {
			log.Printf("Error writing file '%s': %v", safePath, err)
			return nil, &mcp.ErrorPayload{Code: "OperationFailed", Message: fmt.Sprintf("Failed to write file '%s': %v", relativePath, err)}
		}
		return map[string]interface{}{"status": "success", "message": fmt.Sprintf("Successfully wrote to '%s'", relativePath)}, nil

	default:
		return nil, &mcp.ErrorPayload{Code: "InvalidArgument", Message: fmt.Sprintf("Invalid operation '%s'. Use 'list_files', 'read_file', or 'write_file'.", operation)}
	}
}
