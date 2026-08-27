//go:build linux

package main

import "telos/internal/plugin/sandbox"

// runSandboxHelper applies OS-level restrictions and exec's the plugin.
// On Linux, this calls the full Landlock + seccomp + namespace sandbox.
func runSandboxHelper() error {
	return sandbox.RunSandboxHelper()
}
