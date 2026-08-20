// Package config provides configuration parsing, validation, and merging.
package config

// validateBusiness applies semantic checks to a structurally valid config.
// Since Configuration Manager operates early and may not have filesystem
// access to verify everything (e.g., storage isn't mounted yet), we
// only produce warnings here. Hard checks happen in the subsystems.
func validateBusiness(cfg *Config) []Warning {
	var warnings []Warning

	// Empty plugin directories isn't a schema error, but might mean nothing is discovered.
	if len(cfg.Discovery.PluginDirs) == 0 {
		warnings = append(warnings, Warning{
			Field:   "discovery.plugin_dirs",
			Message: "no plugin directories specified; discovery will yield no results",
		})
	}

	// AutoApprove is dangerous
	if cfg.Scheduling.AutoApprove {
		warnings = append(warnings, Warning{
			Field:   "scheduling.auto_approve",
			Message: "auto_approve is enabled; system will execute plans without human verification",
		})
	}

	return warnings
}
