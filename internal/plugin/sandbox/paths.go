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

// NormalizeManifestPaths canonicalizes paths declared in the manifest.
// Absolute paths are lexically cleaned and preserved as explicit host filesystem authorizations.
// Relative paths are evaluated against baseDir, symlinks are resolved, and the resulting target
// must remain inside baseDir to prevent traversal or static symlink escapes.
func NormalizeManifestPaths(baseDir string, declared []string) ([]string, error) {
	var result []string
	baseDir = filepath.Clean(baseDir)
	for _, p := range declared {
		if p == "" {
			return nil, errors.New("sandbox: empty path in declaration")
		}
		if filepath.IsAbs(p) {
			// Explicit absolute paths are host authorizations. Just normalize them.
			cleaned := filepath.Clean(p)
			if strings.Contains(cleaned, "..") {
				return nil, fmt.Errorf("sandbox: traversal in absolute path %q", cleaned)
			}
			if _, err := os.Stat(cleaned); err != nil {
				return nil, fmt.Errorf("sandbox: stat absolute path %q: %w", p, err)
			}
			result = append(result, cleaned)
		} else {
			// Relative paths are plugin-internal. Resolve symlinks and verify they stay within baseDir.
			fullPath := filepath.Join(baseDir, filepath.Clean(p))
			resolved, err := filepath.EvalSymlinks(fullPath)
			if err != nil {
				return nil, fmt.Errorf("sandbox: resolve %q: %w", p, err)
			}
			resolved = filepath.Clean(resolved)
			if !IsSubpath(baseDir, resolved) {
				return nil, fmt.Errorf("sandbox: relative path %q escapes plugin root", p)
			}
			result = append(result, resolved)
		}
	}
	return result, nil
}

// IsSubpath reports whether child is a sub-path of parent after cleaning.
// Both paths must be absolute.
func IsSubpath(parent, child string) bool {
	if p, err := filepath.EvalSymlinks(parent); err == nil {
		parent = p
	}
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)

	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}
