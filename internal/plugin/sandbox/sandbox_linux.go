//go:build linux

// Package sandbox provides the Linux V1 sandbox backend using Landlock, seccomp-BPF, and namespaces.
//
// Architecture: the sandbox helper is invoked via re-exec of the Telos binary
// with _TELOS_SANDBOX=1. The helper reads a JSON-encoded Policy from stdin,
// applies Landlock + seccomp + namespace restrictions to itself, then exec's
// the plugin executable. This keeps the host process unrestricted.
package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const sandboxEnvVar = "_TELOS_SANDBOX"

// linuxSandbox implements the Sandbox interface using Landlock + seccomp + namespaces.
type linuxSandbox struct {
	selfPath string // absolute path to the Telos binary for re-exec
}

// NewLinuxSandbox creates a sandbox that uses the supervisor/re-exec pattern.
// selfPath must be the absolute path to the running Telos binary.
func NewLinuxSandbox(selfPath string) (Sandbox, error) {
	if selfPath == "" {
		return nil, errors.New("sandbox: selfPath required for re-exec")
	}
	if _, err := os.Stat(selfPath); err != nil {
		return nil, fmt.Errorf("sandbox: stat self: %w", err)
	}
	return &linuxSandbox{selfPath: selfPath}, nil
}

// Exec runs the plugin within OS-level isolation.
func (s *linuxSandbox) Exec(ctx context.Context, executable string, dir string,
	stdin []byte, policy *Policy) (Result, error) {

	if err := policy.Validate(); err != nil {
		return Result{}, fmt.Errorf("sandbox: invalid policy: %w", err)
	}

	// Check Landlock ABI v1 support — fail closed if unavailable.
	abi, err := landlockABIVersion()
	if err != nil || abi < 1 {
		return Result{}, fmt.Errorf(
			"sandbox: Landlock ABI v1 required but unavailable (got abi=%d, err=%v); "+
				"refusing to execute plugin without filesystem isolation", abi, err)
	}

	// Serialize policy for the sandbox helper.
	helperInput := sandboxHelperInput{
		Policy:      *policy,
		Executable:  executable,
		Dir:         dir,
		PluginStdin: stdin,
		LandlockABI: abi,
	}
	policyJSON, err := json.Marshal(helperInput)
	if err != nil {
		return Result{}, fmt.Errorf("sandbox: marshal policy: %w", err)
	}

	// Build timeout context.
	timeout := time.Duration(policy.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Spawn sandbox helper via re-exec.
	cmd := exec.CommandContext(execCtx, s.selfPath) // #nosec G204 -- selfPath originates from os.Executable() in all production callers; os.Stat validates existence but trust derives from the call-site invariant, not from Stat
	cmd.Env = append(os.Environ(), sandboxEnvVar+"=1")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(string(policyJSON) + "\n")

	// Capture output with limits.
	outLimit := policy.OutputLimit
	if outLimit <= 0 {
		outLimit = 4 << 20
	}
	var stdout, stderr limitedBuffer
	stdout.limit = outLimit
	stderr.limit = 1 << 20 // 1 MB for stderr
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set up process group for clean termination.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Add namespace flags for isolation.
	cloneFlags := uintptr(0)
	cloneFlags |= syscall.CLONE_NEWPID // PID namespace isolation
	if !policy.Network {
		cloneFlags |= syscall.CLONE_NEWNET // Network namespace (no connectivity)
	}
	cmd.SysProcAttr.Cloneflags = cloneFlags

	err = cmd.Run()
	if execCtx.Err() != nil {
		// Kill the entire process group on timeout.
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return Result{}, fmt.Errorf("sandbox: plugin timed out after %s", timeout)
	}
	if err != nil {
		return Result{Stdout: stdout.buf, Stderr: stderr.buf},
			fmt.Errorf("sandbox: plugin failed: %w (stderr: %s)",
				err, strings.TrimSpace(string(stderr.buf)))
	}
	return Result{Stdout: stdout.buf, Stderr: stderr.buf}, nil
}

// sandboxHelperInput is the JSON protocol between the supervisor and helper.
type sandboxHelperInput struct {
	Policy      Policy `json:"policy"`
	Executable  string `json:"executable"`
	Dir         string `json:"dir"`
	PluginStdin []byte `json:"plugin_stdin"`
	LandlockABI int    `json:"landlock_abi"`
}

// RunSandboxHelper is the entry point for the re-exec'd sandbox helper process.
// It reads the policy from stdin, applies OS-level restrictions, then exec's the plugin.
// This function never returns on success (it calls syscall.Exec).
func RunSandboxHelper() error {
	// Validate invocation.
	if os.Getenv(sandboxEnvVar) != "1" {
		return errors.New("sandbox helper: invalid invocation (missing env)")
	}

	// Read and validate policy from stdin.
	var input sandboxHelperInput
	dec := json.NewDecoder(os.Stdin)
	if err := dec.Decode(&input); err != nil {
		return fmt.Errorf("sandbox helper: decode policy: %w", err)
	}
	if err := input.Policy.Validate(); err != nil {
		return fmt.Errorf("sandbox helper: invalid policy: %w", err)
	}
	if input.Executable == "" {
		return errors.New("sandbox helper: no executable specified")
	}

	// 1. Set PR_SET_NO_NEW_PRIVS — required before seccomp, prevents setuid escalation.
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("sandbox helper: PR_SET_NO_NEW_PRIVS: %w", err)
	}

	// 2. Apply resource limits via prlimit.
	if err := applyResourceLimits(&input.Policy); err != nil {
		return fmt.Errorf("sandbox helper: resource limits: %w", err)
	}

	// 3. Apply Landlock filesystem restrictions.
	if err := applyLandlock(&input.Policy, input.LandlockABI); err != nil {
		return fmt.Errorf("sandbox helper: landlock: %w", err)
	}

	// 4. Install seccomp filter (must be after Landlock — seccomp may restrict
	//    the landlock_* syscalls themselves, so Landlock must be applied first).
	if err := installSeccomp(); err != nil {
		return fmt.Errorf("sandbox helper: seccomp: %w", err)
	}

	// 5. Exec the plugin. This replaces the sandbox helper process image.
	//    The Landlock + seccomp restrictions are inherited by the exec'd process.
	return syscall.Exec(input.Executable, []string{input.Executable}, os.Environ()) // #nosec G204 -- Executable is passed from the supervisor which validated it against the plugin manifest
}

// landlockABIVersion returns the highest Landlock ABI version supported by the kernel.
func landlockABIVersion() (int, error) {
	abi, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		0, // NULL attr
		0, // 0 size
		1, // LANDLOCK_CREATE_RULESET_VERSION
	)
	if errno != 0 {
		return 0, errno
	}
	return int(abi), nil
}

const (
	landlockAccessFSExecute    = 1 << 0
	landlockAccessFSWriteFile  = 1 << 1
	landlockAccessFSReadFile   = 1 << 2
	landlockAccessFSReadDir    = 1 << 3
	landlockAccessFSRemoveDir  = 1 << 4
	landlockAccessFSRemoveFile = 1 << 5
	landlockAccessFSMakeChar   = 1 << 6
	landlockAccessFSMakeDir    = 1 << 7
	landlockAccessFSMakeReg    = 1 << 8
	landlockAccessFSMakeSock   = 1 << 9
	landlockAccessFSMakeFifo   = 1 << 10
	landlockAccessFSMakeBlock  = 1 << 11
	landlockAccessFSMakeSym    = 1 << 12
	landlockAccessFSRefer      = 1 << 13
	landlockAccessFSTruncate   = 1 << 14
)

// applyLandlock creates and enforces a Landlock ruleset for the given policy.
func applyLandlock(p *Policy, abi int) error {
	// ABI v1 handled access rights (all 13 bits).
	var fsAccess uint64 = (1 << 13) - 1

	// For higher ABIs, add additional access rights.
	if abi >= 2 {
		fsAccess |= landlockAccessFSRefer
	}
	if abi >= 3 {
		fsAccess |= landlockAccessFSTruncate
	}

	type landlockAttr struct {
		AllowedAccessFS  uint64
		AllowedAccessNet uint64
	}

	attr := landlockAttr{AllowedAccessFS: fsAccess}
	attrSize := uint64(16) // 16 is sizeof(landlockAttr)

	fd, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafePointer(&attr)),
		uintptr(attrSize),
		0,
	)
	if errno != 0 {
		return fmt.Errorf("landlock_create_ruleset: %w", errno)
	}
	rulesetFD := int(fd)
	defer func() { _ = unix.Close(rulesetFD) }()

	// Read-only access rights.
	roAccess := uint64(landlockAccessFSReadFile | landlockAccessFSReadDir)
	// Read-write access adds write and directory modification rights, but strictly EXCLUDES execution.
	rwAccess := fsAccess &^ landlockAccessFSExecute
	// Add rules for read paths.
	for _, path := range p.ReadPaths {
		if err := landlockAddPathRule(rulesetFD, path, roAccess); err != nil {
			return fmt.Errorf("landlock add read rule %q: %w", path, err)
		}
	}

	// Add rules for executables (read + execute).
	for _, path := range p.Executables {
		if err := landlockAddPathRule(rulesetFD, path, roAccess|landlockAccessFSExecute); err != nil {
			return fmt.Errorf("landlock add execute rule %q: %w", path, err)
		}
	}

	// Add rules for write paths.
	for _, path := range p.WritePaths {
		if err := landlockAddPathRule(rulesetFD, path, rwAccess); err != nil {
			return fmt.Errorf("landlock add write rule %q: %w", path, err)
		}
	}

	// Enforce the ruleset.
	_, _, errno = unix.Syscall(
		unix.SYS_LANDLOCK_RESTRICT_SELF,
		uintptr(rulesetFD),
		0,
		0,
	)
	if errno != 0 {
		return fmt.Errorf("landlock_restrict_self: %w", errno)
	}

	return nil
}

func landlockAddPathRule(rulesetFD int, path string, accessRights uint64) error {
	fd, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}
	defer func() { _ = unix.Close(fd) }()

	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return fmt.Errorf("fstat %q: %w", path, err)
	}

	if stat.Mode&unix.S_IFMT != unix.S_IFDIR {
		// Valid rights for a regular file.
		const fileAccessMask = landlockAccessFSExecute | landlockAccessFSWriteFile |
			landlockAccessFSReadFile | landlockAccessFSTruncate
		accessRights &= fileAccessMask
	}

	type landlockPathBeneath struct {
		AllowedAccess uint64
		ParentFD      int32
		_             [4]byte // padding
	}

	if fd > 2147483647 || fd < 0 {
		return fmt.Errorf("invalid fd: %d", fd)
	}

	rule := landlockPathBeneath{
		AllowedAccess: accessRights,
		ParentFD:      int32(fd),
	}

	_, _, errno := unix.Syscall6(
		unix.SYS_LANDLOCK_ADD_RULE,
		uintptr(rulesetFD),
		1, // LANDLOCK_RULE_PATH_BENEATH
		uintptr(unsafePointer(&rule)),
		0, 0, 0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// applyResourceLimits sets prlimit constraints on the current process.
func applyResourceLimits(p *Policy) error {
	if p.MemoryBytes > 0 {
		if err := setRlimit(unix.RLIMIT_AS, uint64(p.MemoryBytes)); err != nil {
			return fmt.Errorf("RLIMIT_AS: %w", err)
		}
	}
	if p.MaxProcesses > 0 {
		// RLIMIT_NPROC is a resource limit (fork bomb prevention),
		// NOT a namespace-creation restriction.
		if err := setRlimit(unix.RLIMIT_NPROC, uint64(p.MaxProcesses)); err != nil {
			return fmt.Errorf("RLIMIT_NPROC: %w", err)
		}
	}
	if p.MaxFileSizeBytes > 0 {
		if err := setRlimit(unix.RLIMIT_FSIZE, uint64(p.MaxFileSizeBytes)); err != nil {
			return fmt.Errorf("RLIMIT_FSIZE: %w", err)
		}
	}
	return nil
}

func setRlimit(resource int, value uint64) error {
	rlim := unix.Rlimit{Cur: value, Max: value}
	return unix.Setrlimit(resource, &rlim)
}

// limitedBuffer is a bytes.Buffer with a maximum capacity.
type limitedBuffer struct {
	buf   []byte
	limit int64
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - int64(len(b.buf))
	if remaining <= 0 {
		return 0, errors.New("output limit exceeded")
	}
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

// unsafePointer is a helper to get an unsafe.Pointer from any value.
// This avoids importing unsafe in multiple places.
func unsafePointer[T any](v *T) unsafe.Pointer {
	return unsafe.Pointer(v) // #nosec G103 -- required for syscall argument passing
}
