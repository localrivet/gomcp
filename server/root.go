package server

import (
	"path/filepath"
)

// Root sets the allowed root paths for the server.
// Root paths define the file system boundaries that the server is allowed to access,
// providing a security boundary for file operations. This method can be called
// multiple times to add more roots, and each path will be normalized to prevent path traversal.
//
// Parameters:
//   - paths: One or more file system paths to register as allowed roots
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) Root(paths ...string) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Append the new roots to the existing ones
	for _, path := range paths {
		// Normalize the path (this would handle ".." and "." components)
		normalized := filepath.Clean(path)

		// Check if this root is already registered
		alreadyExists := false
		for _, existingRoot := range s.roots {
			if existingRoot == normalized {
				alreadyExists = true
				break
			}
		}

		// Add the root if it's not already registered
		if !alreadyExists {
			s.roots = append(s.roots, normalized)
			s.logger.Info("added root path", "path", normalized)
		}
	}

	return s
}

// GetRoots returns a copy of the registered root paths.
// This method provides read-only access to the configured root paths
// without exposing the internal slice that could be modified.
//
// Returns:
//   - A slice containing all currently registered root paths
func (s *serverImpl) GetRoots() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid exposing the internal slice
	rootsCopy := make([]string, len(s.roots))
	copy(rootsCopy, s.roots)

	return rootsCopy
}

// IsPathInRoots checks if the given path is within any of the registered roots.
// This security method ensures that file operations can only access paths within
// the authorized boundaries defined by the registered root paths, preventing
// directory traversal attacks and unauthorized file system access.
//
// Parameters:
//   - path: The path to validate against the registered roots
//
// Returns:
//   - true if the path is within at least one registered root, false otherwise
func (s *serverImpl) IsPathInRoots(path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normalizedPath := filepath.Clean(path)

	// If no roots are registered, return false
	if len(s.roots) == 0 {
		return false
	}

	// Check if the path is within any of the registered roots
	for _, root := range s.roots {
		rel, err := filepath.Rel(root, normalizedPath)
		if err == nil && !filepath.IsAbs(rel) && !filepath.HasPrefix(rel, "..") {
			return true
		}
	}

	return false
}
