//go:build !linux

package main

import "errors"

// runSandboxHelper is a no-op on non-Linux platforms. The sandbox helper
// requires Landlock + seccomp which are Linux-only.
func runSandboxHelper() error {
	return errors.New("sandbox helper not supported on this platform")
}
