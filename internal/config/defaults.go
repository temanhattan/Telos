package config

import "time"

// DefaultConfig returns a complete Configuration with sensible defaults.
// This fulfills FR-11.6 (built-in defaults for basic operation) and NFR-6.3.
func DefaultConfig() Config {
	return Config{
		Storage: StorageConfig{
			OutputDir:       "/var/lib/aers/backups",
			RetentionPolicy: 5,
			Backend:         "local",
		},
		Crypto: CryptoConfig{
			Algorithm: "aes-256-gcm",
			KDFMemory: 65536,
			KDFTime:   3,
			GPGKeyID:  "", // Disabled by default
		},
		Discovery: DiscoveryConfig{
			PluginDirs: []string{"/var/lib/aers/plugins"},
			Timeout:    5 * time.Minute,
			ExcludedPaths: []string{
				"/tmp", "/var/tmp", "/proc", "/sys", "/dev", "/run",
			},
		},
		Classification: ClassificationConfig{
			ImportanceOverrides: make(map[string]string),
			IncludeRules:        make([]string, 0),
			ExcludeRules:        make([]string, 0),
		},
		Scheduling: SchedulingConfig{
			CronSchedule: "", // Disabled by default
			AutoApprove:  false,
		},
		AI: AIConfig{
			Mode:     "disabled",
			Endpoint: "http://localhost:8080",
			Model:    "local-model",
		},
		Logging: LoggingConfig{
			Level:      "info",
			OutputPath: "/var/log/aers/aers.log",
			AuditLog:   "/var/log/aers/audit.log",
		},
		Plugins: make(map[string]map[string]any),
	}
}
