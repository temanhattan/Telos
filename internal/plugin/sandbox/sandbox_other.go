//go:build !linux

// Package sandbox provides the non-Linux fallback sandbox — application-level enforcement only.
// This provides degraded security with explicit audit-severity warnings.
// No OS-level isolation is available on this platform.
package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	log "telos/internal/logger"
)

// fallbackSandbox implements the Sandbox interface with application-level
// enforcement only. It logs an audit-severity warning on every invocation.
type fallbackSandbox struct {
	logger log.Logger
}

// NewFallbackSandbox creates a sandbox with application-level enforcement only.
// An audit-severity warning is logged indicating degraded security.
func NewFallbackSandbox(logger log.Logger) Sandbox {
	if logger != nil {
		logger.Audit("sandbox: non-Linux platform detected; using degraded fallback " +
			"with application-level enforcement only. No OS-level isolation available.")
	}
	return &fallbackSandbox{logger: logger}
}

// Exec runs the plugin with application-level enforcement (timeout, output limits,
// path validation). No OS-level isolation is applied.
func (s *fallbackSandbox) Exec(ctx context.Context, executable string, dir string,
	stdin []byte, policy *Policy) (Result, error) {

	if err := policy.Validate(); err != nil {
		return Result{}, fmt.Errorf("sandbox: invalid policy: %w", err)
	}

	// Application-level path validation is handled by Policy.Validate().

	// Log degraded enforcement.
	if s.logger != nil {
		s.logger.Audit(fmt.Sprintf("sandbox: executing plugin %q with degraded "+
			"(application-level only) enforcement. Network=%v", executable, policy.Network))
	}

	// Timeout.
	timeout := time.Duration(policy.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, executable)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(stdin)

	outLimit := policy.OutputLimit
	if outLimit <= 0 {
		outLimit = 4 << 20
	}
	var stdout, stderr limitedBufferFallback
	stdout.limit = outLimit
	stderr.limit = 1 << 20
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if execCtx.Err() != nil {
		return Result{}, fmt.Errorf("sandbox: plugin timed out after %s", timeout)
	}
	if err != nil {
		return Result{Stdout: stdout.buf, Stderr: stderr.buf},
			fmt.Errorf("sandbox: plugin failed: %w (stderr: %s)",
				err, strings.TrimSpace(string(stderr.buf)))
	}
	return Result{Stdout: stdout.buf, Stderr: stderr.buf}, nil
}

type limitedBufferFallback struct {
	buf   []byte
	limit int64
}

func (b *limitedBufferFallback) Write(p []byte) (int, error) {
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
