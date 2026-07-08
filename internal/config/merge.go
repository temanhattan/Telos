package config

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// merge applies multiple configuration sources into a single Config struct,
// while recording the source of each field.
func merge(base Config, sysMap, userMap, envOverrides, cliOverrides map[string]any, sources map[string]Source) (Config, error) {
	var merged map[string]any

	baseBytes, err := yaml.Marshal(base)
	if err != nil {
		return base, err
	}

	if err := yaml.Unmarshal(baseBytes, &merged); err != nil {
		return base, err
	}

	// Track baseline defaults
	trackSources(sources, "", merged, SourceDefault)

	// Sequentially merge each layer and update source tracking
	merged = mergeMapAndTrack(merged, sysMap, sources, SourceSystemFile, "")
	merged = mergeMapAndTrack(merged, userMap, sources, SourceUserFile, "")
	merged = mergeMapAndTrack(merged, envOverrides, sources, SourceEnvVar, "")
	merged = mergeMapAndTrack(merged, cliOverrides, sources, SourceCLI, "")

	// Marshal the merged map back to YAML
	mergedBytes, err := yaml.Marshal(merged)
	if err != nil {
		return base, err
	}

	// Unmarshal back to Config, using strict mode to catch unknown fields
	var finalConfig Config
	decoder := yaml.NewDecoder(bytes.NewReader(mergedBytes))
	decoder.KnownFields(true)
	if err := decoder.Decode(&finalConfig); err != nil {
		return base, fmt.Errorf("strict schema validation failed: %w", err)
	}

	return finalConfig, nil
}

// mergeMapAndTrack merges map 'b' into map 'a' recursively, and records the
// source of overridden keys into the 'sources' map.
func mergeMapAndTrack(a, b map[string]any, sources map[string]Source, src Source, prefix string) map[string]any {
	if a == nil {
		a = make(map[string]any)
	}
	if b == nil {
		return a
	}

	for k, v := range b {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}

		if vMap, ok := v.(map[string]any); ok {
			if aVal, aOk := a[k]; aOk {
				if aMap, aMapOk := aVal.(map[string]any); aMapOk {
					a[k] = mergeMapAndTrack(aMap, vMap, sources, src, path)
					continue
				}
			}
			// If 'a' didn't have a map at this key, just copy it and track recursively
			a[k] = vMap
			trackSources(sources, path, vMap, src)
			continue
		}

		// Leaf node merge
		a[k] = v
		sources[path] = src
	}
	return a
}

// trackSources records the source of each configuration field recursively.
func trackSources(sources map[string]Source, prefix string, overrides map[string]any, src Source) {
	for k, v := range overrides {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		if vMap, ok := v.(map[string]any); ok {
			trackSources(sources, path, vMap, src)
		} else {
			sources[path] = src
		}
	}
}
