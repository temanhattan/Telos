// Package main provides the entry point for the Telos CLI.
package main

import (
	"os"
	"telos/core"
	log "telos/internal/logger"
)

func main() {
	// 1. Create a logger instance.
	logger := log.New(log.DebugLevel, os.Stdout)

	// 2. Inject the logger into a subsystem.
	someSubsystem := core.NewSomeSubsystem(logger)

	// 3. Use the subsystem.
	someSubsystem.DoWork()

	logger.Info("Telos application finished.")
}
