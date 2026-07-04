package main

import (
	"AERS/core"
	"AERS/internal/logger"
	"os"
)

func main() {
	// 1. Create a logger instance.
	logger := log.New(log.DebugLevel, os.Stdout)

	// 2. Inject the logger into a subsystem.
	someSubsystem := core.NewSomeSubsystem(logger)

	// 3. Use the subsystem.
	someSubsystem.DoWork()

	logger.Info("AERS application finished.")
}
