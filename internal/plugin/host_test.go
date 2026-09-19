package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"telos/internal/model"
)

func writePlugin(t *testing.T, root, name, manifest string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		manifest = stringReplaceExecutable(manifest)
	}
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\necho '{\"status\":\"success\"}'\n"
	file := "plugin"
	if runtime.GOOS == "windows" {
		body = "@echo {\"status\":\"success\"}"
		file = "plugin.cmd"
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
}

func stringReplaceExecutable(manifest string) string {
	return manifest[:len(manifest)-len("plugin")] + "plugin.cmd"
}

const validManifest = `id: io.telos.discovery.test
name: Test
version: 1.0.0
interface_version: 1
type: Discovery
author: test
executable: plugin`

func TestDiscoverRegistersValidAndSkipsInvalid(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "valid", validManifest)
	writePlugin(t, root, "invalid", `id: io.telos.bad
name: Bad
version: 1.0.0
interface_version: 2
type: Discovery
author: test
executable: plugin`)
	h := New(Options{PluginDirs: []string{root}})
	failures := h.Discover()
	if len(failures) != 1 {
		t.Fatalf("got %d failures", len(failures))
	}
	if got := h.Plugins(); len(got) != 1 || got[0].ID != "io.telos.discovery.test" {
		t.Fatal("valid plugin was not registered")
	}
}

func TestInvokeUsesJSONProtocol(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture differs on Windows")
	}
	root := t.TempDir()
	writePlugin(t, root, "valid", validManifest)
	h := New(Options{PluginDirs: []string{root}})
	h.Discover()
	got, err := h.Invoke(context.Background(), "io.telos.discovery.test", map[string]string{"operation": "discover"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"status":"success"}` {
		t.Fatalf("unexpected response %s", got)
	}
}

func TestBuildPolicy(t *testing.T) {
	h := New(Options{Timeout: 30 * time.Second, OutputLimit: 1024})
	p := Plugin{
		ID:         "io.telos.test",
		Dir:        "/opt/telos/plugins/test",
		Executable: "/opt/telos/plugins/test/plugin",
		Manifest: model.PluginManifest{
			FilesystemRead:  []string{"/etc/hosts"},
			FilesystemWrite: []string{"/tmp/out"},
			NetworkAllowed:  true,
		},
	}

	policy := h.buildPolicy(&p)

	if len(policy.ReadPaths) != 2 {
		t.Fatalf("expected 2 ReadPaths, got %d", len(policy.ReadPaths))
	}
	if policy.ReadPaths[0] != "/opt/telos/plugins/test" {
		t.Errorf("expected first ReadPath to be plugin directory, got %q", policy.ReadPaths[0])
	}
	if policy.ReadPaths[1] != "/etc/hosts" {
		t.Errorf("expected second ReadPath to be /etc/hosts, got %q", policy.ReadPaths[1])
	}

	if len(policy.WritePaths) != 1 || policy.WritePaths[0] != "/tmp/out" {
		t.Errorf("WritePaths mismatch: %v", policy.WritePaths)
	}
	if len(policy.Executables) != 1 || policy.Executables[0] != "/opt/telos/plugins/test/plugin" {
		t.Errorf("Executables mismatch: %v", policy.Executables)
	}
	if !policy.Network {
		t.Error("expected network to be allowed")
	}
	if policy.TimeoutSec != 30 {
		t.Errorf("expected timeout 30, got %d", policy.TimeoutSec)
	}
	if policy.OutputLimit != 1024 {
		t.Errorf("expected output limit 1024, got %d", policy.OutputLimit)
	}
}

func TestExecutablePath(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "plugin")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid executable
	exe := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(exe, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create sub-directory for traversal attempt
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file outside plugin directory
	outside := filepath.Join(root, "outside.sh")
	if err := os.WriteFile(outside, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a symlink escaping the plugin directory
	symlinkEscape := filepath.Join(dir, "escape.sh")
	if err := os.Symlink(outside, symlinkEscape); err != nil {
		t.Logf("Symlink creation failed (expected on some Windows setups): %v", err)
	}

	tests := []struct {
		name      string
		declared  string
		wantError bool
	}{
		{
			name:      "Valid executable",
			declared:  "run.sh",
			wantError: false,
		},
		{
			name:      "Valid nested executable",
			declared:  "sub/../run.sh",
			wantError: true,
		},
		{
			name:      "Absolute path",
			declared:  exe,
			wantError: true,
		},
		{
			name:      "Traversal escape",
			declared:  "../outside.sh",
			wantError: true,
		},
		{
			name:      "Starts with dot dot",
			declared:  "../../plugin/run.sh",
			wantError: true,
		},
		{
			name:      "Nonexistent executable",
			declared:  "missing.sh",
			wantError: true,
		},
		{
			name:      "Directory instead of file",
			declared:  "sub",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executablePath(dir, tt.declared)
			if (err != nil) != tt.wantError {
				t.Errorf("executablePath() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}

	if _, err := os.Stat(symlinkEscape); err == nil {
		t.Run("Symlink escape", func(t *testing.T) {
			_, err := executablePath(dir, "escape.sh")
			if err == nil {
				t.Errorf("Expected error for symlink escape, got nil")
			}
		})
	}
}

// TestBuildPolicyDefaultPluginDirAccess verifies the V1 documentation guarantee:
// "Plugin's own directory → read-only access." (05_Plugin_API.md §Sandbox)
//
// Even when a manifest declares NO additional filesystem_read paths,
// buildPolicy() must include the plugin directory in ReadPaths.
func TestBuildPolicyDefaultPluginDirAccess(t *testing.T) {
	h := New(Options{Timeout: 30 * time.Second, OutputLimit: 1024})
	p := Plugin{
		ID:         "io.telos.minimal",
		Dir:        "/opt/telos/plugins/minimal",
		Executable: "/opt/telos/plugins/minimal/plugin",
		Manifest:   model.PluginManifest{
			// No FilesystemRead declared.
		},
	}

	policy := h.buildPolicy(&p)

	if len(policy.ReadPaths) != 1 {
		t.Fatalf("expected 1 ReadPath (plugin dir only), got %d: %v",
			len(policy.ReadPaths), policy.ReadPaths)
	}
	if policy.ReadPaths[0] != "/opt/telos/plugins/minimal" {
		t.Errorf("expected ReadPaths[0] to be plugin directory, got %q",
			policy.ReadPaths[0])
	}
}

func TestNetworkPermissionValidation(t *testing.T) {
	tests := []struct {
		name               string
		pluginID           string
		pluginType         string
		network            bool
		wantReject         bool
		wantErrSubstrings  []string
		wantNetworkAllowed bool
	}{
		{
			name:               "Discovery with network rejected",
			pluginID:           "io.telos.discovery.net",
			pluginType:         "Discovery",
			network:            true,
			wantReject:         true,
			wantErrSubstrings:  []string{"io.telos.discovery.net", "Discovery", "network permission is not allowed"},
			wantNetworkAllowed: false,
		},
		{
			name:               "Capture with network rejected",
			pluginID:           "io.telos.capture.net",
			pluginType:         "Capture",
			network:            true,
			wantReject:         true,
			wantErrSubstrings:  []string{"io.telos.capture.net", "Capture", "network permission is not allowed"},
			wantNetworkAllowed: false,
		},
		{
			name:               "ClassificationRule with network rejected by existing check",
			pluginID:           "io.telos.classification.net",
			pluginType:         "ClassificationRule",
			network:            true,
			wantReject:         true,
			wantErrSubstrings:  []string{"classification plugins cannot request permissions"},
			wantNetworkAllowed: false,
		},
		{
			name:               "Storage with network accepted",
			pluginID:           "io.telos.storage.net",
			pluginType:         "Storage",
			network:            true,
			wantReject:         false,
			wantNetworkAllowed: true,
		},
		{
			name:               "Restore with network accepted",
			pluginID:           "io.telos.restore.net",
			pluginType:         "Restore",
			network:            true,
			wantReject:         false,
			wantNetworkAllowed: true,
		},
		{
			name:               "Discovery without network accepted",
			pluginID:           "io.telos.discovery.offline",
			pluginType:         "Discovery",
			network:            false,
			wantReject:         false,
			wantNetworkAllowed: false,
		},
		{
			name:               "Capture without network accepted",
			pluginID:           "io.telos.capture.offline",
			pluginType:         "Capture",
			network:            false,
			wantReject:         false,
			wantNetworkAllowed: false,
		},
		{
			name:               "Unknown plugin type with network rejected by existing check",
			pluginID:           "io.telos.unknown.net",
			pluginType:         "UnknownType",
			network:            true,
			wantReject:         true,
			wantErrSubstrings:  []string{`unknown plugin type "UnknownType"`},
			wantNetworkAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()

			netStr := "false"
			if tt.network {
				netStr = "true"
			}
			manifest := fmt.Sprintf(`id: %s
name: Test
version: 1.0.0
interface_version: 1
type: %s
author: test
permissions:
  network: %s
executable: plugin`, tt.pluginID, tt.pluginType, netStr)

			writePlugin(t, root, "testplugin", manifest)

			// Sibling plugin to verify sibling isolation
			siblingID := "io.telos.sibling.valid"
			siblingManifest := fmt.Sprintf(`id: %s
name: Sibling
version: 1.0.0
interface_version: 1
type: Discovery
author: test
executable: plugin`, siblingID)
			writePlugin(t, root, "siblingplugin", siblingManifest)

			h := New(Options{PluginDirs: []string{root}})
			failures := h.Discover()

			if tt.wantReject {
				if len(failures) != 1 {
					t.Fatalf("expected 1 failure, got %d: %v", len(failures), failures)
				}
				for _, sub := range tt.wantErrSubstrings {
					if !strings.Contains(failures[0].Reason, sub) {
						t.Errorf("expected failure reason to contain %q, got %q", sub, failures[0].Reason)
					}
				}

				// Verify rejected plugin is NOT registered, sibling IS registered
				plugins := h.Plugins()
				if len(plugins) != 1 {
					t.Fatalf("expected exactly 1 registered plugin (sibling), got %d: %v", len(plugins), plugins)
				}
				if plugins[0].ID != siblingID {
					t.Errorf("expected registered plugin to be sibling %q, got %q", siblingID, plugins[0].ID)
				}
			} else {
				if len(failures) != 0 {
					t.Fatalf("expected 0 failures, got %d: %v", len(failures), failures)
				}

				plugins := h.Plugins()
				if len(plugins) != 2 {
					t.Fatalf("expected 2 registered plugins, got %d: %v", len(plugins), plugins)
				}

				var found bool
				for _, p := range plugins {
					if p.ID == tt.pluginID {
						found = true
						if p.Manifest.NetworkAllowed != tt.wantNetworkAllowed {
							t.Errorf("expected NetworkAllowed=%v, got %v", tt.wantNetworkAllowed, p.Manifest.NetworkAllowed)
						}
					}
				}
				if !found {
					t.Errorf("plugin %q was not registered", tt.pluginID)
				}
			}

			// Invariant N1 check: no registered Discovery or Capture plugin has network permission.
			for _, p := range h.Plugins() {
				if (p.Manifest.Type == "Discovery" || p.Manifest.Type == "Capture") && p.Manifest.NetworkAllowed {
					t.Errorf("INVARIANT N1 VIOLATION: registered %s plugin %q has network permission", p.Manifest.Type, p.ID)
				}
			}
		})
	}
}

func TestInvariantN1NoDiscoveryOrCaptureHasNetwork(t *testing.T) {
	root := t.TempDir()

	writePlugin(t, root, "discovery-net", `id: io.telos.discovery.malicious
name: DiscoveryNet
version: 1.0.0
interface_version: 1
type: Discovery
author: test
permissions:
  network: true
executable: plugin`)

	writePlugin(t, root, "capture-net", `id: io.telos.capture.malicious
name: CaptureNet
version: 1.0.0
interface_version: 1
type: Capture
author: test
permissions:
  network: true
executable: plugin`)

	h := New(Options{PluginDirs: []string{root}})
	failures := h.Discover()

	if len(failures) != 2 {
		t.Fatalf("expected 2 failures, got %d: %v", len(failures), failures)
	}

	for _, p := range h.Plugins() {
		if (p.Manifest.Type == "Discovery" || p.Manifest.Type == "Capture") && p.Manifest.NetworkAllowed {
			t.Fatalf("INVARIANT N1 VIOLATION: registered %s plugin %q has network permission", p.Manifest.Type, p.ID)
		}
	}
	if len(h.Plugins()) != 0 {
		t.Fatalf("expected 0 registered plugins, got %d: %v", len(h.Plugins()), h.Plugins())
	}
}
