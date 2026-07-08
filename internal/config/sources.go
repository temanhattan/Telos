package config

import "fmt"

// Source identifies where a configuration value originated.
// Sources are ordered by priority from lowest (SourceDefault) to highest (SourceCLI).
// This ordering matches the 5-tier priority hierarchy defined in
// Architecture §S17 and FR-11.1.
type Source int

const (
	// SourceDefault indicates a built-in default value.
	SourceDefault Source = iota
	// SourceSystemFile indicates the value came from the system configuration file.
	SourceSystemFile
	// SourceUserFile indicates the value came from the user configuration file.
	SourceUserFile
	// SourceEnvVar indicates the value came from an environment variable.
	SourceEnvVar
	// SourceCLI indicates the value came from a command-line flag.
	SourceCLI
)

// String returns the human-readable name of the source.
func (s Source) String() string {
	switch s {
	case SourceDefault:
		return "default"
	case SourceSystemFile:
		return "system-config"
	case SourceUserFile:
		return "user-config"
	case SourceEnvVar:
		return "environment"
	case SourceCLI:
		return "cli"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}
