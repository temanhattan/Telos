// Package sandbox provides plugin execution isolation.
//
// The Sandbox interface abstracts platform-specific enforcement. The Linux
// backend uses Landlock, seccomp-BPF, and namespaces for kernel-enforced
// isolation. Non-Linux platforms receive a degraded fallback with
// application-level enforcement and explicit audit-severity warnings.
package sandbox

import (
	"context"
	"errors"
)

// Policy describes the security envelope for a plugin invocation.
// Every field must have a clear enforcement mechanism or be explicitly
// unsupported on the current platform.
type Policy struct {
	// Filesystem — enforced by Landlock on Linux, application-level on others.
	ReadPaths  []string // Absolute paths the plugin may read.
	WritePaths []string // Absolute paths the plugin may write (Restore/Storage only).

	// Executables — the plugin's own binary. Enforced via Landlock execute rules.
	Executables []string

	// Network — enforced via network namespace on Linux (CLONE_NEWNET with no interfaces).
	Network bool

	// Resource limits — enforced via prlimit on Linux.
	MemoryBytes      int64 // Max RSS (RLIMIT_AS). 0 = platform default.
	MaxProcesses     int   // Max child processes (RLIMIT_NPROC — resource limit, not namespace restriction). 0 = platform default.
	MaxFileSizeBytes int64 // Max individual file size (RLIMIT_FSIZE). 0 = no limit.

	// Output and timeout — enforced at application level (all platforms).
	OutputLimit int64 // Max stdout bytes.
	TimeoutSec  int   // Execution deadline in seconds.
}

// Validate checks that the policy is structurally valid.
func (p *Policy) Validate() error {
	for _, path := range p.ReadPaths {
		if path == "" {
			return errors.New("sandbox: empty read path")
		}
	}
	for _, path := range p.WritePaths {
		if path == "" {
			return errors.New("sandbox: empty write path")
		}
	}
	if len(p.Executables) == 0 {
		return errors.New("sandbox: no executables declared")
	}
	for _, exe := range p.Executables {
		if exe == "" {
			return errors.New("sandbox: empty executable path")
		}
	}
	return nil
}

// Result holds the output of a sandboxed plugin invocation.
type Result struct {
	Stdout []byte
	Stderr []byte
}

// Sandbox creates and runs isolated plugin subprocesses.
type Sandbox interface {
	// Exec runs the plugin executable within the given policy.
	// The implementation must enforce as many policy constraints as the
	// platform supports and log the enforcement level at audit severity.
	Exec(ctx context.Context, executable string, dir string,
		stdin []byte, policy Policy) (Result, error)
}
