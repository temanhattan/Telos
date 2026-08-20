package config

import (
	"bytes"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// LoadOptions allows overriding configuration sources during loading.
// All properties are optional.
type LoadOptions struct {
	SystemConfigFile string
	UserConfigFile   string
	EnvOverrides     map[string]any
	CLIOverrides     map[string]any
}

// Manager defines the interface for the Configuration Manager subsystem.
// S17 dictates exactly this API boundary to other subsystems.
type Manager interface {
	// Load computes the configuration pipeline, validates it, and sets the active profile.
	Load(opts LoadOptions) (Profile, error)
	// Current returns the currently active configuration profile.
	Current() Profile
}

type manager struct {
	current Profile
}

// NewManager creates a new Configuration Manager instance.
func NewManager() Manager {
	return &manager{}
}

func (m *manager) Current() Profile {
	return m.current
}

// Load loads, merges, validates, and freezes the configuration.
func (m *manager) Load(opts LoadOptions) (Profile, error) {
	// 1. Start with defaults
	base := DefaultConfig()
	sources := make(map[string]Source)

	// Helper for YAML unmarshaling
	readYaml := func(path string) (map[string]any, error) {
		if path == "" {
			return nil, nil
		}
		data, err := os.ReadFile(path) // #nosec G304 -- path is provided by explicit LoadOptions config file settings
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil // Not existing is fine
			}
			return nil, err
		}
		var out map[string]any
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		// We don't enforce Strict mode on the raw unmarshal because we just want to merge.
		// Strict mode happens at the final Config unmarshal in merge().
		if err := decoder.Decode(&out); err != nil && err != io.EOF {
			return nil, err
		}
		return out, nil
	}

	// 2. System config
	sysMap, err := readYaml(opts.SystemConfigFile)
	if err != nil {
		return nil, &ConfigError{Errors: []ValidationError{{Field: "system_config", Message: "failed to read system config", Value: err}}}
	}

	// 3. User config
	userMap, err := readYaml(opts.UserConfigFile)
	if err != nil {
		return nil, &ConfigError{Errors: []ValidationError{{Field: "user_config", Message: "failed to read user config", Value: err}}}
	}

	// 4. Environment
	envOverrides := opts.EnvOverrides
	if envOverrides == nil {
		envOverrides = resolveEnvOverrides()
	}

	// 5. CLI
	cliOverrides := opts.CLIOverrides

	// Merge all
	merged, err := merge(&base, sysMap, userMap, envOverrides, cliOverrides, sources)
	if err != nil {
		return nil, err
	}

	// 6. Validate
	if err := validateSchema(&merged); err != nil {
		return nil, err
	}
	warnings := validateBusiness(&merged)

	// 7. Freeze
	prof := &profile{
		config:        merged,
		sources:       sources,
		schemaVersion: SchemaVersion,
		validatedAt:   time.Now(),
		warnings:      warnings,
	}

	m.current = prof
	return prof, nil
}
