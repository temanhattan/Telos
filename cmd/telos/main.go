// Package main provides the entry point for the Telos CLI.
package main

import (
	"fmt"
	"os"
	"runtime"
	"telos/core"
	log "telos/internal/logger"
)

func main() {
	// Sandbox helper entry point: when _TELOS_SANDBOX=1 is set, this process
	// was re-exec'd as a sandbox helper. Apply OS-level restrictions and exec
	// the plugin. This path never reaches normal Telos logic.
	if os.Getenv("_TELOS_SANDBOX") == "1" {
		if err := runSandboxHelper(); err != nil {
			fmt.Fprintf(os.Stderr, "sandbox helper: %v\n", err)
			os.Exit(126) // Shell convention for "cannot execute"
		}
		// runSandboxHelper calls syscall.Exec on Linux — if we reach here,
		// the platform doesn't support full sandbox (non-Linux).
		os.Exit(0)
	}

	// 1. Create a logger instance.
	logger := log.New(log.DebugLevel, os.Stdout)

	// 2. Log platform sandbox status.
	if runtime.GOOS == "linux" {
		logger.Info("sandbox: Linux detected, OS-level isolation available")
	} else {
		logger.Audit("sandbox: non-Linux platform (" + runtime.GOOS +
			"), plugin isolation degraded to application-level enforcement only")
	}

	// 3. Inject the logger into a subsystem.
	someSubsystem := core.NewSomeSubsystem(logger)

	// 4. Use the subsystem.
	someSubsystem.DoWork()

	logger.Info("Telos application finished.")
}
