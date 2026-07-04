package core

import (
	"AERS/internal/logger"
)

// SomeSubsystem represents a subsystem that requires a logger.
type SomeSubsystem struct {
	logger log.Logger
}

// NewSomeSubsystem creates a new SomeSubsystem.
func NewSomeSubsystem(logger log.Logger) *SomeSubsystem {
	return &SomeSubsystem{
		logger: logger.WithComponent("some_subsystem"),
	}
}

// DoWork is an example method that uses the logger.
func (s *SomeSubsystem) DoWork() {
	s.logger.Info("Doing some work...")
}
