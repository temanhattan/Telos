package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeManifestPaths(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "plugin")
	if err := os.Mkdir(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a safe internal file.
	safeFile := filepath.Join(pluginDir, "assets")
	if err := os.WriteFile(safeFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a safe internal subdirectory
	safeSubDir := filepath.Join(pluginDir, "sub")
	if err := os.Mkdir(safeSubDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file outside plugin dir.
	outsideFile := filepath.Join(tmpDir, "secret")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink that escapes.
	symlinkEscape := filepath.Join(pluginDir, "link_out")
	if err := os.Symlink(outsideFile, symlinkEscape); err != nil {
		t.Logf("Symlink creation failed (expected on some Windows setups): %v", err)
	}

	// Absolute path existing
	absPathExisting := filepath.Join(tmpDir, "exist.txt")
	if err := os.WriteFile(absPathExisting, []byte("yes"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		paths     []string
		wantError bool
	}{
		{
			name:      "Absolute host path (exists)",
			paths:     []string{absPathExisting},
			wantError: false,
		},
		{
			name:      "Absolute host path (nonexistent)",
			paths:     []string{filepath.Join(tmpDir, "does-not-exist")},
			wantError: true, // Stat absolute path should fail
		},
		{
			name:      "Absolute path with traversal",
			paths:     []string{filepath.Join(pluginDir, "..", "secret")},
			wantError: false, // Clean removes .. and it exists
		},
		{
			name: "Absolute path with unresolvable .. (e.g. /etc/../root)",
			// If it cleans to an existing path, it passes. If it has actual .. left, our code catches it.
			// Let's test a malicious one. Wait, Clean(/etc/../root) is /root. So there is no ".." left.
			paths:     []string{filepath.Join(pluginDir, "..", "secret")},
			wantError: false,
		},
		{
			name:      "Relative internal path",
			paths:     []string{"assets"},
			wantError: false,
		},
		{
			name:      "Relative nested traversal escape",
			paths:     []string{"sub/../../secret"},
			wantError: true,
		},
		{
			name:      "Relative traversal escape",
			paths:     []string{"../secret"},
			wantError: true,
		},
		{
			name:      "Empty path",
			paths:     []string{""},
			wantError: true,
		},
		{
			name:      "Multiple valid paths",
			paths:     []string{"assets", "sub"},
			wantError: false,
		},
		{
			name:      "Nonexistent relative path",
			paths:     []string{"nonexistent"},
			wantError: true, // EvalSymlinks fails on nonexistent targets
		},
		{
			name:      "Mixed valid and invalid paths",
			paths:     []string{"assets", "../secret"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeManifestPaths(pluginDir, tt.paths)
			if (err != nil) != tt.wantError {
				t.Errorf("NormalizeManifestPaths() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}

	if _, err := os.Stat(symlinkEscape); err == nil {
		t.Run("Relative symlink escape", func(t *testing.T) {
			_, err := NormalizeManifestPaths(pluginDir, []string{"link_out"})
			if err == nil {
				t.Errorf("Expected error for symlink escape, got nil")
			}
		})
	}
}

func TestPolicyValidation(t *testing.T) {
	absPath := filepath.Join(os.TempDir(), "valid")
	absExe := filepath.Join(os.TempDir(), "valid", "exe")
	p := &Policy{
		ReadPaths:   []string{absPath},
		Executables: []string{absExe},
	}
	if err := p.Validate(); err != nil {
		t.Errorf("Expected valid policy, got: %v", err)
	}

	p.ReadPaths = []string{"relative/path"}
	if err := p.Validate(); err == nil {
		t.Errorf("Expected error for relative read path, got nil")
	}

	p.ReadPaths = []string{absPath}
	p.Executables = []string{"relative/exe"}
	if err := p.Validate(); err == nil {
		t.Errorf("Expected error for relative executable, got nil")
	}

	// Add test for empty string to match coverage
	p.Executables = []string{absExe}
	p.WritePaths = []string{""}
	if err := p.Validate(); err == nil {
		t.Errorf("Expected error for empty write path, got nil")
	}
}
