package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NormalizePath resolves a path to its absolute, clean, symlink-evaluated form.
func NormalizePath(path string) (string, error) {
	if path == "" {
		return "", errors.New("sandbox: empty path")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("sandbox: relative path %q not allowed", path)
	}
	// Clean and resolve symlinks.
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("sandbox: resolve %q: %w", path, err)
	}
	return filepath.Clean(resolved), nil
}

// ValidatePaths ensures all declared paths are absolute, exist, and contain
// no traversal components after normalization.
func ValidatePaths(declared []string) error {
	for _, p := range declared {
		if p == "" {
			return errors.New("sandbox: empty path in declaration")
		}
		if !filepath.IsAbs(p) {
			return fmt.Errorf("sandbox: relative path %q not allowed", p)
		}
		cleaned := filepath.Clean(p)
		if strings.Contains(cleaned, "..") {
			return fmt.Errorf("sandbox: traversal in cleaned path %q", cleaned)
		}
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("sandbox: path %q: %w", p, err)
		}
	}
	return nil
}

// IsSubpath reports whether child is a sub-path of parent after cleaning.
// Both paths must be absolute.
func IsSubpath(parent, child string) bool {
	parent = filepath.Clean(parent) + string(filepath.Separator)
	child = filepath.Clean(child) + string(filepath.Separator)
	return strings.HasPrefix(child, parent)
}
