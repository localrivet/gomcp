package server

import (
	"path/filepath"
)

// Root sets the allowed root paths for the server.
// This method can be called multiple times to add more roots.
// Each path will be normalized to an absolute path.
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
func (s *serverImpl) GetRoots() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid exposing the internal slice
	rootsCopy := make([]string, len(s.roots))
	copy(rootsCopy, s.roots)

	return rootsCopy
}

// IsPathInRoots checks if the given path is within any of the registered roots.
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
