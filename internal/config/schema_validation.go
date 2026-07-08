package config

import (
	"strings"
)

// validateSchema checks the structural validity of the configuration:
// required fields, valid enums, non-negative numbers, etc.
// This implements the first stage of validation per architectural review.
func validateSchema(cfg *Config) error {
	var errs []ValidationError

	// Storage
	if cfg.Storage.OutputDir == "" {
		errs = append(errs, ValidationError{Field: "storage.output_dir", Message: "cannot be empty"})
	}
	if cfg.Storage.RetentionPolicy <= 0 {
		errs = append(errs, ValidationError{Field: "storage.retention_policy", Message: "must be > 0", Value: cfg.Storage.RetentionPolicy})
	}
	if cfg.Storage.Backend == "" {
		errs = append(errs, ValidationError{Field: "storage.backend", Message: "cannot be empty"})
	}

	// Crypto
	validAlgo := false
	for _, algo := range []string{"aes-256-gcm", "chacha20-poly1305"} {
		if cfg.Crypto.Algorithm == algo {
			validAlgo = true
			break
		}
	}
	if !validAlgo {
		errs = append(errs, ValidationError{Field: "crypto.algorithm", Message: "unsupported algorithm", Value: cfg.Crypto.Algorithm})
	}
	if cfg.Crypto.KDFMemory < 1024 {
		errs = append(errs, ValidationError{Field: "crypto.kdf_memory", Message: "must be at least 1024", Value: cfg.Crypto.KDFMemory})
	}
	if cfg.Crypto.KDFTime < 1 {
		errs = append(errs, ValidationError{Field: "crypto.kdf_time", Message: "must be at least 1", Value: cfg.Crypto.KDFTime})
	}

	// Discovery
	if cfg.Discovery.Timeout <= 0 {
		errs = append(errs, ValidationError{Field: "discovery.timeout", Message: "must be > 0", Value: cfg.Discovery.Timeout})
	}

	// AI
	validAIMode := false
	for _, mode := range []string{"disabled", "local", "cloud"} {
		if cfg.AI.Mode == mode {
			validAIMode = true
			break
		}
	}
	if !validAIMode {
		errs = append(errs, ValidationError{Field: "ai.mode", Message: "must be disabled, local, or cloud", Value: cfg.AI.Mode})
	}
	if cfg.AI.Mode == "cloud" && cfg.AI.Endpoint == "" {
		errs = append(errs, ValidationError{Field: "ai.endpoint", Message: "cannot be empty when ai.mode is cloud"})
	}

	// Logging
	validLogLevel := false
	for _, level := range []string{"debug", "info", "warn", "error", "audit"} {
		if strings.ToLower(cfg.Logging.Level) == level {
			validLogLevel = true
			break
		}
	}
	if !validLogLevel {
		errs = append(errs, ValidationError{Field: "logging.level", Message: "unsupported log level", Value: cfg.Logging.Level})
	}

	if len(errs) > 0 {
		return &ConfigError{Errors: errs}
	}
	return nil
}
