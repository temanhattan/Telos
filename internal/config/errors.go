package config

import (
	"fmt"
	"strings"
)

// ValidationError represents a single configuration validation failure.
// It carries the field path, a human-readable message, and optionally
// the offending value — providing enough context for the user to fix
// the issue without guessing (FR-11.4).
type ValidationError struct {
	// Field is the dotted path of the offending field (e.g., "storage.output_dir").
	Field string
	// Message is a human-readable description of the validation failure.
	Message string
	// Value is the actual value that failed validation, if available.
	Value any
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.Value != nil {
		return fmt.Sprintf("config validation: %s: %s (got: %v)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("config validation: %s: %s", e.Field, e.Message)
}

// ConfigError represents one or more configuration validation failures.
// It aggregates all errors discovered during validation so the user can
// fix them all at once rather than one at a time (FR-11.4).
type ConfigError struct {
	Errors []ValidationError
}

// Error implements the error interface.
func (e *ConfigError) Error() string {
	if len(e.Errors) == 0 {
		return "config validation: no errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "config validation failed with %d errors:", len(e.Errors))
	for _, err := range e.Errors {
		fmt.Fprintf(&b, "\n  - %s", err.Error())
	}
	return b.String()
}

// HasErrors returns true if there are any validation errors.
func (e *ConfigError) HasErrors() bool {
	return len(e.Errors) > 0
}

// Warning represents a non-fatal configuration issue discovered during
// business validation. Warnings do not prevent the system from starting
// but indicate potential runtime problems.
type Warning struct {
	// Field is the dotted path of the field with the issue.
	Field string
	// Message is a human-readable description of the issue.
	Message string
}
