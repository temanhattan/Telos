package config

import (
	"time"
)

// SchemaVersion defines the current expected configuration schema version.
const SchemaVersion = "1.0"

// Metadata contains information about the loaded configuration profile.
type Metadata struct {
	SchemaVersion string
	ValidatedAt   time.Time
	Warnings      []Warning
}

// Profile represents the immutable, validated configuration profile.
// It is the single source of truth for configuration within the process (FR-11.2).
// Exposing it as an interface protects it from external mutation and
// keeps dependants relying on an abstraction.
type Profile interface {
	// Config returns a deep copy of the underlying configuration.
	// Mutations on this copy will not affect the active profile.
	Config() Config
	// Source returns the source attribution for a specific configuration path.
	Source(path string) Source
	// Metadata returns information about the configuration validation.
	Metadata() Metadata
	// PluginConfig returns a copy of the configuration map for a specific plugin, or nil.
	PluginConfig(pluginID string) map[string]any
}

type profile struct {
	config        Config
	sources       map[string]Source
	schemaVersion string
	validatedAt   time.Time
	warnings      []Warning
}

// Config returns a deep copy of the underlying configuration.
func (p *profile) Config() Config {
	cpy := p.config

	// Slices in Discovery
	if p.config.Discovery.PluginDirs != nil {
		cpy.Discovery.PluginDirs = make([]string, len(p.config.Discovery.PluginDirs))
		copy(cpy.Discovery.PluginDirs, p.config.Discovery.PluginDirs)
	}
	if p.config.Discovery.ExcludedPaths != nil {
		cpy.Discovery.ExcludedPaths = make([]string, len(p.config.Discovery.ExcludedPaths))
		copy(cpy.Discovery.ExcludedPaths, p.config.Discovery.ExcludedPaths)
	}

	// Maps and Slices in Classification
	if p.config.Classification.ImportanceOverrides != nil {
		cpy.Classification.ImportanceOverrides = make(map[string]string)
		for k, v := range p.config.Classification.ImportanceOverrides {
			cpy.Classification.ImportanceOverrides[k] = v
		}
	}
	if p.config.Classification.IncludeRules != nil {
		cpy.Classification.IncludeRules = make([]string, len(p.config.Classification.IncludeRules))
		copy(cpy.Classification.IncludeRules, p.config.Classification.IncludeRules)
	}
	if p.config.Classification.ExcludeRules != nil {
		cpy.Classification.ExcludeRules = make([]string, len(p.config.Classification.ExcludeRules))
		copy(cpy.Classification.ExcludeRules, p.config.Classification.ExcludeRules)
	}

	// Deep copy Plugins map
	if p.config.Plugins != nil {
		cpy.Plugins = make(map[string]map[string]any)
		for pk, pv := range p.config.Plugins {
			if pv != nil {
				cpy.Plugins[pk] = cloneMap(pv)
			} else {
				cpy.Plugins[pk] = nil
			}
		}
	}

	return cpy
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	cpy := make(map[string]any)
	for k, v := range m {
		switch v := v.(type) {
		case map[string]any:
			cpy[k] = cloneMap(v)
		case []any:
			cpy[k] = cloneSlice(v)
		default:
			cpy[k] = v
		}
	}
	return cpy
}

func cloneSlice(s []any) []any {
	if s == nil {
		return nil
	}
	cpy := make([]any, len(s))
	for i, v := range s {
		switch v := v.(type) {
		case map[string]any:
			cpy[i] = cloneMap(v)
		case []any:
			cpy[i] = cloneSlice(v)
		default:
			cpy[i] = v
		}
	}
	return cpy
}

// Source returns the source mapping for a given configuration path.
func (p *profile) Source(path string) Source {
	if src, ok := p.sources[path]; ok {
		return src
	}
	return SourceDefault
}

// Metadata returns the base metadata configuration.
func (p *profile) Metadata() Metadata {
	return Metadata{
		SchemaVersion: p.schemaVersion,
		ValidatedAt:   p.validatedAt,
		Warnings:      p.warnings,
	}
}

// PluginConfig returns the configuration mapping for a specific plugin.
func (p *profile) PluginConfig(pluginID string) map[string]any {
	if p.config.Plugins == nil {
		return nil
	}
	return cloneMap(p.config.Plugins[pluginID])
}
