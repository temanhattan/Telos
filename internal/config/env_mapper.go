package config

import (
	"os"
	"strings"
)

// resolveEnvOverrides checks the environment for variables starting with TELOS_
// and returns a mapped structure matching the YAML layout.
func resolveEnvOverrides() map[string]any {
	overrides := make(map[string]any)
	prefix := "TELOS_"

	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, prefix) {
			continue
		}

		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimPrefix(parts[0], prefix))
		val := parts[1]

		// Map flat key to nested map: e.g. LOGGING_LEVEL -> logging.level
		// Special case: we only support 2 levels of nesting natively for env vars.
		keyParts := strings.SplitN(key, "_", 2)
		if len(keyParts) == 2 {
			domain := keyParts[0]
			param := keyParts[1]

			if _, ok := overrides[domain]; !ok {
				overrides[domain] = make(map[string]any)
			}
			domainMap := overrides[domain].(map[string]any)
			domainMap[param] = val
		}
	}

	return overrides
}
