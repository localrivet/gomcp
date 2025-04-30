// This file defines the "filesystem" tool for the example MCP server.
// WARNING: This is a simplified example for demonstration purposes only.
// Exposing filesystem operations requires extreme security considerations
// in a real application to prevent unauthorized access or modification.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/localrivet/gomcp/protocol"
)

// fileSystemSandbox defines the root directory within which all filesystem
// operations by this tool are restricted. This is a crucial security measure.
const fileSystemSandbox = "./fs_sandbox"

// FileSystemArgs defines the arguments for the filesystem tool.
// Struct tags are used by the AddTool helper to generate the schema.
type FileSystemArgs struct {
	Operation string `json:"operation" description:"The operation to perform ('list_files', 'read_file', 'write_file')." required:"true"`
	Path      string `json:"path" description:"The relative path within the sandbox directory (e.g., 'mydir/myfile.txt', '.')." required:"true"`
	Content   string `json:"content,omitempty" description:"The content to write (only required for 'write_file')."` // Optional
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
// and returns the result content or an error.
// It now matches the signature expected by the server.AddTool helper.
func executeFileSystem(args FileSystemArgs) (protocol.Content, error) {

	// Argument parsing and validation for operation/path is handled by the AddTool helper.

	// --- Path Validation ---
	safePath, err := getSafePath(args.Path)
	if err != nil {
		log.Printf("Path validation failed for relative path '%s': %v", args.Path, err)
		return nil, err
	}
	log.Printf("Operating on safe path: %s (relative: %s)", safePath, args.Path)
	// --- End Path Validation ---

	// --- Operation Dispatch ---
	switch args.Operation {
	case "list_files":
		files, err := os.ReadDir(safePath)
		if err != nil {
			log.Printf("Error listing files in '%s': %v", safePath, err)
			return nil, fmt.Errorf("failed to list files at path '%s': %w", args.Path, err)
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

		// Return single content and nil error on success
		return protocol.TextContent{Type: "text", Text: string(resultBytes)}, nil

	case "read_file":
		info, err := os.Stat(safePath)
		if err != nil {
			log.Printf("Error stating file '%s': %v", safePath, err)
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("file not found at path '%s'", args.Path)
			}
			return nil, fmt.Errorf("failed to access path '%s': %w", args.Path, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("path '%s' is a directory, cannot read", args.Path)
		}

		contentBytes, err := os.ReadFile(safePath)
		if err != nil {
			log.Printf("Error reading file '%s': %v", safePath, err)
			return nil, fmt.Errorf("failed to read file '%s': %w", args.Path, err)
		}
		// Return single content and nil error on success
		return protocol.TextContent{Type: "text", Text: string(contentBytes)}, nil

	case "write_file":
		// Content is optional in the struct, but required for this operation
		if args.Content == "" {
			// Note: AddTool helper doesn't enforce conditional requirements based on other args.
			// We could add a check here, or rely on the client sending it.
			// For simplicity, we'll assume the client sends it when needed.
			// If it's truly required, the struct tag could be `required:"true"`
			// but that would make it required for list/read too.
			// A custom validation hook might be needed for complex cases.
			log.Printf("Warning: 'content' argument missing for write_file operation on path '%s'", args.Path)
			// Proceeding, assuming empty content write might be intended in some cases.
		}

		parentDir := filepath.Dir(safePath)
		sandboxAbsForWrite, _ := filepath.Abs(fileSystemSandbox)
		if !strings.HasPrefix(parentDir, sandboxAbsForWrite) {
			return nil, fmt.Errorf("cannot create parent directory outside sandbox for path '%s'", args.Path)
		}
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			log.Printf("Error creating parent directory '%s': %v", parentDir, err)
			return nil, fmt.Errorf("failed to create directory structure for path '%s': %w", args.Path, err)
		}

		if safePath == sandboxAbsForWrite {
			return nil, errors.New("cannot write directly to the sandbox root directory")
		}

		err := os.WriteFile(safePath, []byte(args.Content), 0644)
		if err != nil {
			log.Printf("Error writing file '%s': %v", safePath, err)
			return nil, fmt.Errorf("failed to write file '%s': %w", args.Path, err)
		}
		resultMap := map[string]interface{}{"status": "success", "message": fmt.Sprintf("Successfully wrote %d bytes to '%s'", len(args.Content), args.Path)} // Use args.Content
		resultBytes, _ := json.Marshal(resultMap)
		// Return single content and nil error on success
		return protocol.TextContent{Type: "text", Text: string(resultBytes)}, nil

	default:
		return nil, fmt.Errorf("invalid operation '%s'. Use 'list_files', 'read_file', or 'write_file'", args.Operation)
	}
	// --- End Operation Dispatch ---
}
