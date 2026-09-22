//go:build linux

package sandbox_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"telos/internal/plugin/sandbox"
)

// To run this on Linux:
//
//	go test -v ./internal/plugin/sandbox -run TestLinuxSandboxIntegration
func TestLinuxSandboxIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Build the plugin executable from a self-contained Go source file.
	// The plugin exercises sandbox enforcement boundaries (seccomp + Landlock)
	// and reports results as JSON on stdout.
	pluginSrc := filepath.Join(tmpDir, "plugin.go")
	if err := os.WriteFile(pluginSrc, []byte(pluginSource), 0644); err != nil {
		t.Fatal(err)
	}

	pluginExe := filepath.Join(tmpDir, "plugin")
	cmd := exec.Command("go", "build", "-o", pluginExe, pluginSrc)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build plugin: %v\n%s", err, out)
	}

	// Build the real telos CLI so we can use it as the sandbox helper.
	telosExe := filepath.Join(tmpDir, "telos")
	cmd = exec.Command("go", "build", "-o", telosExe, "telos/cmd/telos")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build telos CLI: %v\n%s", err, out)
	}

	// --- ENVIRONMENT CAPABILITY CHECK ---
	// The Linux sandbox requires CLONE_NEWPID and CLONE_NEWNET to isolate the plugin.
	// We probe these specific requirements because unprivileged CI runners (like GitHub Actions)
	// typically lack CAP_SYS_ADMIN and will reject these clone flags with EPERM.
	// This is NOT a sandbox bypass; it safely skips only when the kernel rejects the namespace creation.
	probeCmd := exec.Command("true")
	probeCmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNET,
	}
	if probeErr := probeCmd.Run(); probeErr != nil {
		if errors.Is(probeErr, syscall.EPERM) || errors.Is(probeErr, os.ErrPermission) {
			t.Skipf("Skipping integration test: CI environment lacks capabilities for CLONE_NEWPID and CLONE_NEWNET (EPERM)")
		}
		// If the failure is not EPERM (e.g. invalid arguments, missing true binary, unexpected kernel error),
		// we must not silently hide it.
		t.Fatalf("Unexpected error during namespace capability probe: %v", probeErr)
	}
	// ------------------------------------

	sb, err := sandbox.NewLinuxSandbox(telosExe)
	if err != nil {
		t.Skipf("Sandbox not supported on this kernel: %v", err)
	}

	// Set up allowed and denied test files and directories for Landlock verification.
	if writeErr := os.WriteFile("/tmp/allowed-file", []byte("ok"), 0644); writeErr != nil {
		t.Fatal(writeErr)
	}
	if dirErr := os.Mkdir("/tmp/allowed-dir", 0755); dirErr != nil && !os.IsExist(dirErr) {
		t.Fatal(dirErr)
	}
	if writeErr := os.WriteFile("/tmp/writable-file", []byte("ok"), 0644); writeErr != nil {
		t.Fatal(writeErr)
	}
	if writeErr := os.WriteFile("/tmp/writable-exe", []byte(""), 0755); writeErr != nil {
		t.Fatal(writeErr)
	}
	if writeErr := os.WriteFile("/tmp/denied", []byte("secret"), 0644); writeErr != nil {
		t.Fatal(writeErr)
	}
	defer func() { _ = os.Remove("/tmp/allowed-file") }()
	defer func() { _ = os.RemoveAll("/tmp/allowed-dir") }()
	defer func() { _ = os.Remove("/tmp/writable-file") }()
	defer func() { _ = os.Remove("/tmp/writable-exe") }()
	defer func() { _ = os.Remove("/tmp/denied") }()

	pol := &sandbox.Policy{
		Executables: []string{pluginExe},
		ReadPaths:   []string{tmpDir, "/tmp/allowed-file", "/tmp/allowed-dir"},
		WritePaths:  []string{"/tmp/writable-file", "/tmp/writable-exe"},
		TimeoutSec:  10,
		OutputLimit: 4096,
	}

	res, err := sb.Exec(context.Background(), pluginExe, tmpDir, nil, pol)
	if err != nil {
		t.Fatalf("Exec failed: %v\nstderr: %s", err, res.Stderr)
	}

	var output struct {
		Results string `json:"results"`
	}
	if err := json.Unmarshal(res.Stdout, &output); err != nil {
		t.Fatalf("Malformed output: %v (stdout: %s, stderr: %s)",
			err, res.Stdout, res.Stderr)
	}

	t.Logf("Plugin output: %s", output.Results)

	// Verify seccomp enforcement results.
	seccompChecks := []struct {
		name     string
		expected string
	}{
		{"clone3", "clone3: ENOSYS"},
		{"clone(CLONE_NEWUSER)", "clone_newuser: EPERM"},
		{"mount", "mount: EPERM"},
		{"unshare", "unshare: EPERM"},
		{"setns", "setns: EPERM"},
	}
	for _, check := range seccompChecks {
		if !strings.Contains(output.Results, check.expected) {
			t.Errorf("seccomp %s: expected %q in results, got: %s",
				check.name, check.expected, output.Results)
		}
	}

	// Verify Landlock filesystem enforcement.
	if !strings.Contains(output.Results, "read_allowed_file: OK") {
		t.Errorf("Landlock: expected allowed read file to succeed, got: %s", output.Results)
	}
	if !strings.Contains(output.Results, "read_allowed_dir: OK") {
		t.Errorf("Landlock: expected allowed read dir to succeed, got: %s", output.Results)
	}
	if !strings.Contains(output.Results, "write_allowed_file: OK") {
		t.Errorf("Landlock: expected allowed write file to succeed, got: %s", output.Results)
	}
	if !strings.Contains(output.Results, "execute_writable: BLOCKED") {
		t.Errorf("Landlock: expected execution in WritePath to be blocked, got: %s", output.Results)
	}
	if !strings.Contains(output.Results, "read_denied: BLOCKED") {
		t.Errorf("Landlock: expected denied read to be blocked, got: %s", output.Results)
	}
}

// pluginSource is the Go source for the test plugin binary.
// It uses only standard library packages so it can be compiled as a
// standalone program without external module dependencies.
//
// Each test exercises a specific sandbox enforcement boundary and
// reports the result as a labeled string in the output JSON.
const pluginSource = `package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func main() {
	var results []string

	// 1. Test clone3 (syscall 435) — seccomp should deny with ENOSYS.
	// Classic cBPF cannot inspect clone3's struct clone_args pointer,
	// so the filter unconditionally denies this syscall.
	_, _, err := syscall.RawSyscall(435, 0, 0, 0)
	if err == syscall.ENOSYS {
		results = append(results, "clone3: ENOSYS")
	} else {
		results = append(results, fmt.Sprintf("clone3: %v", err))
	}

	// 2. Test clone(CLONE_NEWUSER) — seccomp should deny with EPERM.
	// The BPF filter inspects clone's flags register and denies CLONE_NEW*.
	const cloneNewUser = 0x10000000
	_, _, err = syscall.RawSyscall(unix.SYS_CLONE, cloneNewUser, 0, 0)
	if err == syscall.EPERM {
		results = append(results, "clone_newuser: EPERM")
	} else {
		results = append(results, fmt.Sprintf("clone_newuser: %v", err))
	}

	// 3. Test mount — seccomp denylist should deny with EPERM.
	mountErr := syscall.Mount("none", "/mnt", "tmpfs", 0, "")
	if mountErr == syscall.EPERM {
		results = append(results, "mount: EPERM")
	} else {
		results = append(results, fmt.Sprintf("mount: %v", mountErr))
	}

	// 4. Test unshare(CLONE_NEWNS) — seccomp denylist should deny with EPERM.
	const cloneNewNS = 0x00020000
	_, _, err = syscall.RawSyscall(unix.SYS_UNSHARE, cloneNewNS, 0, 0)
	if err == syscall.EPERM {
		results = append(results, "unshare: EPERM")
	} else {
		results = append(results, fmt.Sprintf("unshare: %v", err))
	}

	// 5. Test setns — seccomp denylist should deny with EPERM.
	_, _, err = syscall.RawSyscall(unix.SYS_SETNS, 0, 0, 0)
	if err == syscall.EPERM {
		results = append(results, "setns: EPERM")
	} else {
		results = append(results, fmt.Sprintf("setns: %v", err))
	}

	// 6. Test Landlock: read allowed file.
	if _, readErr := os.ReadFile("/tmp/allowed-file"); readErr == nil {
		results = append(results, "read_allowed_file: OK")
	} else {
		results = append(results, fmt.Sprintf("read_allowed_file: %v", readErr))
	}

	// 7. Test Landlock: read allowed directory.
	if f, err := os.Open("/tmp/allowed-dir"); err == nil {
		_, _ = f.Readdirnames(1)
		f.Close()
		results = append(results, "read_allowed_dir: OK")
	} else {
		results = append(results, fmt.Sprintf("read_allowed_dir: %v", err))
	}

	// 8. Test Landlock: write allowed file.
	if writeErr := os.WriteFile("/tmp/writable-file", []byte("test"), 0644); writeErr == nil {
		results = append(results, "write_allowed_file: OK")
	} else {
		results = append(results, fmt.Sprintf("write_allowed_file: %v", writeErr))
	}

	// 8.5 Test Landlock: execution denied in WritePaths.
	myExe, _ := os.Executable()
	myBytes, _ := os.ReadFile(myExe)
	if err := os.WriteFile("/tmp/writable-exe", myBytes, 0755); err == nil {
		cmd := exec.Command("/tmp/writable-exe")
		if err := cmd.Start(); err != nil {
			if strings.Contains(err.Error(), "permission denied") {
				results = append(results, "execute_writable: BLOCKED")
			} else {
				results = append(results, fmt.Sprintf("execute_writable: %v", err))
			}
		} else {
			_ = cmd.Process.Kill()
			results = append(results, "execute_writable: ACCESSIBLE")
		}
	} else {
		results = append(results, fmt.Sprintf("write_exe: %v", err))
	}

	// 9. Test Landlock: read denied path.
	if _, readErr := os.ReadFile("/tmp/denied"); readErr != nil {
		results = append(results, "read_denied: BLOCKED")
	} else {
		results = append(results, "read_denied: ACCESSIBLE")
	}

	// Output as JSON for the test harness.
	fmt.Printf("{\"results\": %q}\n", fmt.Sprintf("%v", results))
}
`

func TestLinuxSandboxStderrLimitValidation(t *testing.T) {
	sb, err := sandbox.NewLinuxSandbox("/usr/bin/env")
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	tests := []struct {
		limit int64
		name  string
	}{
		{limit: 0, name: "zero_limit"},
		{limit: -1, name: "negative_limit"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pol := &sandbox.Policy{
				Executables: []string{"/bin/true"},
				StderrLimit: tc.limit,
			}
			_, err := sb.Exec(context.Background(), "/bin/true", "/", nil, pol)
			if err == nil {
				t.Fatalf("expected error for StderrLimit %d, got nil", tc.limit)
			}
			if !strings.Contains(err.Error(), "strictly positive") {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}
}

const pluginSourceStderr = `package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	// Write 64MB to stderr
	chunk := strings.Repeat("A", 1024*1024)
	for i := 0; i < 64; i++ {
		fmt.Fprint(os.Stderr, chunk)
	}

	in, _ := io.ReadAll(os.Stdin)
	if strings.Contains(string(in), "fail") {
		os.Exit(1)
	}
	fmt.Println("{\"results\": \"ok\"}")
}
`

// To run this on Linux:
//
//	go test -v ./internal/plugin/sandbox -run TestLinuxSandboxStderrTruncation
func TestLinuxSandboxStderrTruncation(t *testing.T) {
	tmpDir := t.TempDir()

	pluginSrc := filepath.Join(tmpDir, "plugin_stderr.go")
	if err := os.WriteFile(pluginSrc, []byte(pluginSourceStderr), 0644); err != nil {
		t.Fatal(err)
	}

	pluginExe := filepath.Join(tmpDir, "plugin_stderr")
	cmd := exec.Command("go", "build", "-o", pluginExe, pluginSrc)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build plugin: %v\n%s", err, out)
	}

	telosExe := filepath.Join(tmpDir, "telos")
	cmd = exec.Command("go", "build", "-o", telosExe, "telos/cmd/telos")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build telos CLI: %v\n%s", err, out)
	}

	probeCmd := exec.Command("true")
	probeCmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNET,
	}
	if probeErr := probeCmd.Run(); probeErr != nil {
		if errors.Is(probeErr, syscall.EPERM) || errors.Is(probeErr, os.ErrPermission) {
			t.Skipf("Skipping integration test: CI environment lacks capabilities for CLONE_NEWPID and CLONE_NEWNET (EPERM)")
		}
		t.Fatalf("Unexpected error during namespace capability probe: %v", probeErr)
	}

	sb, err := sandbox.NewLinuxSandbox(telosExe)
	if err != nil {
		t.Skipf("Sandbox not supported on this kernel: %v", err)
	}

	pol := &sandbox.Policy{
		Executables: []string{pluginExe},
		ReadPaths:   []string{tmpDir},
		TimeoutSec:  10,
		OutputLimit: 4096,
		StderrLimit: 4096,
		// The Go runtime requires a large memory limit (2GB) inside the sandbox to bypass RLIMIT_AS initialization failure.
		MemoryBytes: 2147483648,
	}

	t.Run("success_truncated", func(t *testing.T) {
		res, err := sb.Exec(context.Background(), pluginExe, tmpDir, nil, pol)
		if err != nil {
			t.Fatalf("Expected success, got error: %v", err)
		}
		if !res.StderrTruncated {
			t.Errorf("Expected StderrTruncated to be true")
		}
		if len(res.Stderr) > 4096 {
			t.Errorf("Expected Stderr length to be <= 4096, got %d", len(res.Stderr))
		}
		if !strings.Contains(string(res.Stdout), "results") {
			t.Errorf("Expected stdout JSON, got %q", res.Stdout)
		}
	})

	t.Run("failure_truncated", func(t *testing.T) {
		res, err := sb.Exec(context.Background(), pluginExe, tmpDir, []byte("fail"), pol)
		if err == nil {
			t.Fatalf("Expected error, got success")
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() != 1 {
				t.Errorf("Expected exit code 1, got %d", exitErr.ExitCode())
			}
		} else {
			t.Errorf("Expected ExitError, got %T: %v", err, err)
		}

		if !strings.Contains(err.Error(), "[stderr truncated]") {
			t.Errorf("Expected error to contain '[stderr truncated]', got %v", err)
		}

		if !res.StderrTruncated {
			t.Errorf("Expected StderrTruncated to be true")
		}
		if len(res.Stderr) > 4096 {
			t.Errorf("Expected Stderr length to be <= 4096, got %d", len(res.Stderr))
		}
	})
}

func TestLinuxSandboxPluginStdin(t *testing.T) {
	tmpDir := t.TempDir()
	pluginSrc := filepath.Join(tmpDir, "plugin_stdin.go")
	pluginCode := `package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(2)
	}
	fmt.Printf("%d:%x\n", len(data), sha256.Sum256(data))
}
`
	if err := os.WriteFile(pluginSrc, []byte(pluginCode), 0644); err != nil {
		t.Fatal(err)
	}
	pluginExe := filepath.Join(tmpDir, "plugin_stdin")
	if output, err := exec.Command("go", "build", "-o", pluginExe, pluginSrc).CombinedOutput(); err != nil {
		t.Fatalf("failed to build stdin fixture: %v\n%s", err, output)
	}
	telosExe := filepath.Join(tmpDir, "telos")
	if output, err := exec.Command("go", "build", "-o", telosExe, "telos/cmd/telos").CombinedOutput(); err != nil {
		t.Fatalf("failed to build telos CLI: %v\n%s", err, output)
	}

	probeCmd := exec.Command("true")
	probeCmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNET,
	}
	if probeErr := probeCmd.Run(); probeErr != nil {
		if errors.Is(probeErr, syscall.EPERM) || errors.Is(probeErr, os.ErrPermission) {
			t.Skipf("Skipping integration test: CI environment lacks capabilities for CLONE_NEWPID and CLONE_NEWNET (EPERM)")
		}
		t.Fatalf("Unexpected error during namespace capability probe: %v", probeErr)
	}

	sb, err := sandbox.NewLinuxSandbox(telosExe)
	if err != nil {
		t.Skipf("Sandbox not supported on this kernel: %v", err)
	}
	policy := &sandbox.Policy{
		Executables: []string{pluginExe},
		ReadPaths:   []string{tmpDir},
		TimeoutSec:  10,
		OutputLimit: 4096,
		StderrLimit: 4096,
		MemoryBytes: 2 * 1024 * 1024 * 1024,
	}

	tests := []struct {
		name string
		data []byte
	}{
		{name: "non_empty", data: []byte("discovery-request\n")},
		{name: "nil", data: nil},
		{name: "empty", data: []byte{}},
		{name: "large", data: bytes.Repeat([]byte("x"), 4*1024*1024)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := sb.Exec(context.Background(), pluginExe, tmpDir, tc.data, policy)
			if err != nil {
				t.Fatalf("sandbox exec: %v", err)
			}
			expected := fmt.Sprintf("%d:%x\n", len(tc.data), sha256.Sum256(tc.data))
			if string(result.Stdout) != expected {
				t.Fatalf("expected stdout %q, got %q", expected, result.Stdout)
			}
		})
	}
}
