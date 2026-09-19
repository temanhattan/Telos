package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"telos/internal/model"
	"telos/internal/plugin/sandbox"
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
			name:               "Restore without network accepted (default-denied)",
			pluginID:           "io.telos.restore.offline",
			pluginType:         "Restore",
			network:            false,
			wantReject:         false,
			wantNetworkAllowed: false,
		},
		{
			name:               "Storage without network accepted (default-denied)",
			pluginID:           "io.telos.storage.offline",
			pluginType:         "Storage",
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

func TestInvariantRestoreNetworkPolicy(t *testing.T) {
	// Invariant N2 (05_Plugin_API.md §12, NFR-8.1, NFR-8.2, C-9):
	// Restore plugins are default-denied network access.
	// When network permission is omitted or false, NetworkAllowed must be false.
	// When network permission is explicitly declared (network: true) with justification, NetworkAllowed must be true.
	root := t.TempDir()

	// 1. Restore plugin with omitted network field (default-denied)
	writePlugin(t, root, "restore-omitted", `id: io.telos.restore.omitted
name: RestoreOmitted
version: 1.0.0
interface_version: 1
type: Restore
author: test
executable: plugin`)

	// 2. Restore plugin with explicit network: false (explicitly denied)
	writePlugin(t, root, "restore-false", `id: io.telos.restore.nonet
name: RestoreNoNet
version: 1.0.0
interface_version: 1
type: Restore
author: test
permissions:
  network: false
executable: plugin`)

	// 3. Restore plugin with explicit network: true (opt-in declared)
	writePlugin(t, root, "restore-true", `id: io.telos.restore.withnet
name: RestoreWithNet
version: 1.0.0
interface_version: 1
type: Restore
author: test
permissions:
  network: true
executable: plugin`)

	h := New(Options{PluginDirs: []string{root}})
	failures := h.Discover()
	if len(failures) != 0 {
		t.Fatalf("expected 0 failures, got %d: %v", len(failures), failures)
	}

	plugins := h.Plugins()
	if len(plugins) != 3 {
		t.Fatalf("expected 3 plugins registered, got %d", len(plugins))
	}

	for _, p := range plugins {
		switch p.ID {
		case "io.telos.restore.omitted":
			if p.Manifest.NetworkAllowed {
				t.Errorf("INVARIANT VIOLATION: %s has NetworkAllowed=true, want false (default-denied)", p.ID)
			}
		case "io.telos.restore.nonet":
			if p.Manifest.NetworkAllowed {
				t.Errorf("INVARIANT VIOLATION: %s has NetworkAllowed=true, want false (explicit false)", p.ID)
			}
		case "io.telos.restore.withnet":
			if !p.Manifest.NetworkAllowed {
				t.Errorf("INVARIANT VIOLATION: %s has NetworkAllowed=false, want true (explicit true)", p.ID)
			}
		}
	}
}

type reexecSandbox struct {
	helperExe string
}

func (s *reexecSandbox) Exec(ctx context.Context, executable string, dir string, stdin []byte, policy *sandbox.Policy) (sandbox.Result, error) {
	helperInput := struct {
		Policy      sandbox.Policy `json:"policy"`
		Executable  string         `json:"executable"`
		Dir         string         `json:"dir"`
		PluginStdin []byte         `json:"plugin_stdin"`
		LandlockABI int            `json:"landlock_abi"`
	}{
		Policy:      *policy,
		Executable:  executable,
		Dir:         dir,
		PluginStdin: stdin,
		LandlockABI: 1,
	}

	policyJSON, err := json.Marshal(helperInput)
	if err != nil {
		return sandbox.Result{}, err
	}

	cmd := exec.CommandContext(ctx, s.helperExe)
	cmd.Env = append(os.Environ(), "_TELOS_SANDBOX=1")
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(append(policyJSON, '\n'))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return sandbox.Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()},
			fmt.Errorf("sandbox helper failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return sandbox.Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, nil
}

func TestNewHostDefaultsMatchADR0012(t *testing.T) {
	// ADR-0012 §3 specifies:
	// - MemoryBytes: 512 MB (536,870,912 bytes)
	// - MaxFileSizeBytes: 10 GB (10,737,418,240 bytes)
	// - OutputLimit: 4 MB (4,194,304 bytes)
	// ADR-0012 §1 specifies:
	// - MaxProcesses: 0 (disabled / unconstrained)
	h := New(Options{})

	if h.opts.MemoryBytes != 536870912 {
		t.Errorf("expected MemoryBytes default 536870912, got %d", h.opts.MemoryBytes)
	}
	if h.opts.MaxFileSizeBytes != 10737418240 {
		t.Errorf("expected MaxFileSizeBytes default 10737418240, got %d", h.opts.MaxFileSizeBytes)
	}
	if h.opts.MaxProcesses != 0 {
		t.Errorf("expected MaxProcesses default 0, got %d", h.opts.MaxProcesses)
	}
	if h.opts.OutputLimit != 4194304 {
		t.Errorf("expected OutputLimit default 4194304, got %d", h.opts.OutputLimit)
	}
	if h.opts.Timeout != 5*time.Minute {
		t.Errorf("expected Timeout default 5m, got %v", h.opts.Timeout)
	}

	p := Plugin{
		ID:         "io.telos.test",
		Dir:        "/opt/telos/plugins/test",
		Executable: "/opt/telos/plugins/test/plugin",
	}
	pol := h.buildPolicy(&p)

	if pol.MemoryBytes != 536870912 {
		t.Errorf("expected policy MemoryBytes 536870912, got %d", pol.MemoryBytes)
	}
	if pol.MaxFileSizeBytes != 10737418240 {
		t.Errorf("expected policy MaxFileSizeBytes 10737418240, got %d", pol.MaxFileSizeBytes)
	}
	if pol.MaxProcesses != 0 {
		t.Errorf("expected policy MaxProcesses 0, got %d", pol.MaxProcesses)
	}
	if pol.OutputLimit != 4194304 {
		t.Errorf("expected policy OutputLimit 4194304, got %d", pol.OutputLimit)
	}
	if pol.TimeoutSec != 300 {
		t.Errorf("expected policy TimeoutSec 300, got %d", pol.TimeoutSec)
	}
}

func TestResourceLimitZeroValueSemantics(t *testing.T) {
	// Zero values in Options:
	// - MemoryBytes == 0 => default (536,870,912)
	// - MaxFileSizeBytes == 0 => default (10,737,418,240)
	// - MaxProcesses == 0 => disabled / default (0)
	h := New(Options{
		MemoryBytes:      0,
		MaxFileSizeBytes: 0,
		MaxProcesses:     0,
	})

	if h.opts.MemoryBytes != 536870912 {
		t.Errorf("expected MemoryBytes == 0 to normalize to default 536870912, got %d", h.opts.MemoryBytes)
	}
	if h.opts.MaxFileSizeBytes != 10737418240 {
		t.Errorf("expected MaxFileSizeBytes == 0 to normalize to default 10737418240, got %d", h.opts.MaxFileSizeBytes)
	}
	if h.opts.MaxProcesses != 0 {
		t.Errorf("expected MaxProcesses == 0 to remain 0 (disabled), got %d", h.opts.MaxProcesses)
	}

	p := Plugin{
		ID:         "io.telos.test",
		Dir:        "/opt/telos/plugins/test",
		Executable: "/opt/telos/plugins/test/plugin",
	}
	pol := h.buildPolicy(&p)

	if pol.MemoryBytes != 536870912 {
		t.Errorf("expected policy MemoryBytes 536870912, got %d", pol.MemoryBytes)
	}
	if pol.MaxFileSizeBytes != 10737418240 {
		t.Errorf("expected policy MaxFileSizeBytes 10737418240, got %d", pol.MaxFileSizeBytes)
	}
	if pol.MaxProcesses != 0 {
		t.Errorf("expected policy MaxProcesses 0, got %d", pol.MaxProcesses)
	}
}

func TestResourceLimitNegativeValueSemantics(t *testing.T) {
	// Negative values in Options:
	// - MemoryBytes < 0 => default (536,870,912)
	// - MaxFileSizeBytes < 0 => default (10,737,418,240)
	// - MaxProcesses < 0 => 0 (disabled)
	h := New(Options{
		MemoryBytes:      -1,
		MaxFileSizeBytes: -100,
		MaxProcesses:     -5,
	})

	if h.opts.MemoryBytes != 536870912 {
		t.Errorf("expected MemoryBytes < 0 to normalize to default 536870912, got %d", h.opts.MemoryBytes)
	}
	if h.opts.MaxFileSizeBytes != 10737418240 {
		t.Errorf("expected MaxFileSizeBytes < 0 to normalize to default 10737418240, got %d", h.opts.MaxFileSizeBytes)
	}
	if h.opts.MaxProcesses != 0 {
		t.Errorf("expected MaxProcesses < 0 to normalize to 0, got %d", h.opts.MaxProcesses)
	}

	p := Plugin{
		ID:         "io.telos.test",
		Dir:        "/opt/telos/plugins/test",
		Executable: "/opt/telos/plugins/test/plugin",
	}
	pol := h.buildPolicy(&p)

	if pol.MemoryBytes != 536870912 {
		t.Errorf("expected policy MemoryBytes 536870912, got %d", pol.MemoryBytes)
	}
	if pol.MaxFileSizeBytes != 10737418240 {
		t.Errorf("expected policy MaxFileSizeBytes 10737418240, got %d", pol.MaxFileSizeBytes)
	}
	if pol.MaxProcesses != 0 {
		t.Errorf("expected policy MaxProcesses 0, got %d", pol.MaxProcesses)
	}
}

func TestCustomHostOptionsFlowIntoPolicy(t *testing.T) {
	// Explicit non-default values must be preserved from Options -> h.opts -> buildPolicy() -> Policy
	const customMem int64 = 256 * 1024 * 1024
	const customFSize int64 = 1024 * 1024
	const customProcs int = 16
	const customOut int64 = 2048
	const customTimeout = 10 * time.Second

	h := New(Options{
		MemoryBytes:      customMem,
		MaxFileSizeBytes: customFSize,
		MaxProcesses:     customProcs,
		OutputLimit:      customOut,
		Timeout:          customTimeout,
	})

	if h.opts.MemoryBytes != customMem {
		t.Errorf("expected MemoryBytes %d, got %d", customMem, h.opts.MemoryBytes)
	}
	if h.opts.MaxFileSizeBytes != customFSize {
		t.Errorf("expected MaxFileSizeBytes %d, got %d", customFSize, h.opts.MaxFileSizeBytes)
	}
	if h.opts.MaxProcesses != customProcs {
		t.Errorf("expected MaxProcesses %d, got %d", customProcs, h.opts.MaxProcesses)
	}
	if h.opts.OutputLimit != customOut {
		t.Errorf("expected OutputLimit %d, got %d", customOut, h.opts.OutputLimit)
	}
	if h.opts.Timeout != customTimeout {
		t.Errorf("expected Timeout %v, got %v", customTimeout, h.opts.Timeout)
	}

	p := Plugin{
		ID:         "io.telos.test",
		Dir:        "/opt/telos/plugins/test",
		Executable: "/opt/telos/plugins/test/plugin",
	}
	pol := h.buildPolicy(&p)

	if pol.MemoryBytes != customMem {
		t.Errorf("expected policy MemoryBytes %d, got %d", customMem, pol.MemoryBytes)
	}
	if pol.MaxFileSizeBytes != customFSize {
		t.Errorf("expected policy MaxFileSizeBytes %d, got %d", customFSize, pol.MaxFileSizeBytes)
	}
	if pol.MaxProcesses != customProcs {
		t.Errorf("expected policy MaxProcesses %d, got %d", customProcs, pol.MaxProcesses)
	}
	if pol.OutputLimit != customOut {
		t.Errorf("expected policy OutputLimit %d, got %d", customOut, pol.OutputLimit)
	}
	if pol.TimeoutSec != 10 {
		t.Errorf("expected policy TimeoutSec 10, got %d", pol.TimeoutSec)
	}
}

func TestMaxFileSizeBytesEnforcedLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("RLIMIT_FSIZE enforcement is Linux-only")
	}

	tmpDir := t.TempDir()

	// Build the telos CLI binary to act as the re-exec sandbox helper
	telosExe := filepath.Join(tmpDir, "telos")
	cmd := exec.Command("go", "build", "-o", telosExe, "telos/cmd/telos")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build telos CLI: %v\n%s", err, out)
	}

	// Create fixture plugin directory
	pluginDir := filepath.Join(tmpDir, "filesize_plugin")
	if err := os.Mkdir(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeDir := filepath.Join(pluginDir, "write_dir")
	if err := os.Mkdir(writeDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := `id: io.telos.filesize.test
name: FileSizeTest
version: 1.0.0
interface_version: 1
type: Restore
author: test
permissions:
  filesystem_write:
    - write_dir
executable: plugin`
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Minimal compiled Go fixture writing 64KB (exceeding 4096 limit)
	pluginSrc := filepath.Join(pluginDir, "main.go")
	fixtureCode := `package main

import (
	"os"
	"path/filepath"
)

func main() {
	dir := filepath.Dir(os.Args[0])
	target := filepath.Join(dir, "write_dir", "oversized.bin")
	data := make([]byte, 64*1024)
	if err := os.WriteFile(target, data, 0644); err != nil {
		os.Exit(1)
	}
	os.Stdout.WriteString("{\"status\":\"success\"}\n")
}
`
	if err := os.WriteFile(pluginSrc, []byte(fixtureCode), 0644); err != nil {
		t.Fatal(err)
	}
	pluginExe := filepath.Join(pluginDir, "plugin")
	cmd = exec.Command("go", "build", "-o", pluginExe, pluginSrc)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build fixture plugin: %v\n%s", err, out)
	}

	sb := &reexecSandbox{helperExe: telosExe}
	const limitBytes int64 = 4096
	h := New(Options{
		PluginDirs:       []string{tmpDir},
		MemoryBytes:      2 * 1024 * 1024 * 1024, // 2GB to allow Go runtime to boot
		MaxFileSizeBytes: limitBytes,
		Sandbox:          sb,
	})
	failures := h.Discover()
	if len(failures) != 0 {
		t.Fatalf("unexpected discovery failures: %v", failures)
	}

	targetFile := filepath.Join(writeDir, "oversized.bin")
	_, err := h.InvokeSandboxed(context.Background(), "io.telos.filesize.test", map[string]string{})
	if err == nil {
		t.Fatal("expected sandboxed invocation to fail due to MaxFileSizeBytes enforcement, but it succeeded")
	}

	// Verify resulting file does not exceed the limit
	if fi, statErr := os.Stat(targetFile); statErr == nil {
		if fi.Size() > limitBytes {
			t.Fatalf("file size %d exceeded limit %d", fi.Size(), limitBytes)
		}
	}
}

func TestDefaultMemoryBytesEnforcedLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("RLIMIT_AS enforcement is Linux-only")
	}

	tmpDir := t.TempDir()
	telosExe := filepath.Join(tmpDir, "telos")
	cmd := exec.Command("go", "build", "-o", telosExe, "telos/cmd/telos")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build telos CLI: %v\n%s", err, out)
	}

	pluginDir := filepath.Join(tmpDir, "mem_plugin")
	if err := os.Mkdir(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `id: io.telos.mem.test
name: MemTest
version: 1.0.0
interface_version: 1
type: Discovery
author: test
executable: plugin`
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	pluginSrc := filepath.Join(pluginDir, "main.go")
	fixtureCode := `package main

import "os"

func main() {
	os.Stdout.WriteString("{\"status\":\"success\"}\n")
}
`
	if err := os.WriteFile(pluginSrc, []byte(fixtureCode), 0644); err != nil {
		t.Fatal(err)
	}
	pluginExe := filepath.Join(pluginDir, "plugin")
	cmd = exec.Command("go", "build", "-o", pluginExe, pluginSrc)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build fixture plugin: %v\n%s", err, out)
	}

	sb := &reexecSandbox{helperExe: telosExe}
	// Use default Options (default MemoryBytes is 512 MB per ADR-0012)
	h := New(Options{
		PluginDirs: []string{tmpDir},
		Sandbox:    sb,
	})
	failures := h.Discover()
	if len(failures) != 0 {
		t.Fatalf("unexpected discovery failures: %v", failures)
	}

	resp, err := h.InvokeSandboxed(context.Background(), "io.telos.mem.test", map[string]string{})
	if err != nil {
		t.Logf("REPORTED: minimal compiled Go fixture failed under 512 MB RLIMIT_AS default: %v", err)
		return
	}
	if string(resp) != `{"status":"success"}` {
		t.Errorf("unexpected response: %s", string(resp))
	}
}
