package config

import "time"

// Config represents the complete, structured configuration.
type Config struct {
	Storage        StorageConfig             `yaml:"storage"`
	Crypto         CryptoConfig              `yaml:"crypto"`
	Discovery      DiscoveryConfig           `yaml:"discovery"`
	Classification ClassificationConfig      `yaml:"classification"`
	Scheduling     SchedulingConfig          `yaml:"scheduling"`
	AI             AIConfig                  `yaml:"ai"`
	Logging        LoggingConfig             `yaml:"logging"`
	Plugins        map[string]map[string]any `yaml:"plugins"`
}

// StorageConfig defines settings for the Storage Backend.
type StorageConfig struct {
	OutputDir       string `yaml:"output_dir"`
	RetentionPolicy int    `yaml:"retention_policy"`
	Backend         string `yaml:"backend"`
}

// CryptoConfig defines settings for the Crypto Engine.
type CryptoConfig struct {
	Algorithm string `yaml:"algorithm"`
	KDFMemory int    `yaml:"kdf_memory"`
	KDFTime   int    `yaml:"kdf_time"`
	GPGKeyID  string `yaml:"gpg_key_id"`
}

// DiscoveryConfig defines settings for the Discovery Engine.
type DiscoveryConfig struct {
	PluginDirs    []string      `yaml:"plugin_dirs"`
	Timeout       time.Duration `yaml:"timeout"`
	ExcludedPaths []string      `yaml:"excluded_paths"`
}

// ClassificationConfig defines settings for the Classifier.
type ClassificationConfig struct {
	ImportanceOverrides map[string]string `yaml:"importance_overrides"`
	IncludeRules        []string          `yaml:"include_rules"`
	ExcludeRules        []string          `yaml:"exclude_rules"`
}

// SchedulingConfig defines settings for automated backups.
type SchedulingConfig struct {
	CronSchedule string `yaml:"cron_schedule"`
	AutoApprove  bool   `yaml:"auto_approve"`
}

// AIConfig defines settings for the AI Advisory Layer.
type AIConfig struct {
	Mode     string `yaml:"mode"` // disabled, local, cloud
	Endpoint string `yaml:"endpoint"`
	Model    string `yaml:"model"`
}

// LoggingConfig defines settings for system observability.
type LoggingConfig struct {
	Level      string `yaml:"level"` // debug, info, warn, error, audit
	OutputPath string `yaml:"output_path"`
	AuditLog   string `yaml:"audit_log"`
}
