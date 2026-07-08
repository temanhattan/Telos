package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerLoad(t *testing.T) {
	tmpDir := t.TempDir()

	sysFile := filepath.Join(tmpDir, "system.yaml")
	userFile := filepath.Join(tmpDir, "user.yaml")

	sysYAML := `
storage:
  output_dir: /system/backups
crypto:
  kdf_memory: 2048
plugins:
  io.aers.storage.s3:
    bucket: system-bucket
`

	userYAML := `
storage:
  output_dir: /user/backups
logging:
  level: debug
plugins:
  io.aers.storage.s3:
    bucket: user-bucket
    region: us-east-1
`

	if err := os.WriteFile(sysFile, []byte(sysYAML), 0644); err != nil {
		t.Fatalf("Failed to write sys file: %v", err)
	}
	if err := os.WriteFile(userFile, []byte(userYAML), 0644); err != nil {
		t.Fatalf("Failed to write user file: %v", err)
	}

	cliOverrides := map[string]any{
		"logging": map[string]any{
			"level": "warn",
		},
	}

	opts := LoadOptions{
		SystemConfigFile: sysFile,
		UserConfigFile:   userFile,
		CLIOverrides:     cliOverrides,
	}

	manager := NewManager()
	profile, err := manager.Load(opts)
	if err != nil {
		t.Fatalf("Expected load to succeed, got: %v", err)
	}

	cfg := profile.Config()

	// Validate merging semantics
	if cfg.Storage.OutputDir != "/user/backups" {
		t.Errorf("Expected output_dir to be /user/backups, got %s", cfg.Storage.OutputDir)
	}
	if cfg.Crypto.KDFMemory != 2048 {
		t.Errorf("Expected KDFMemory to be 2048, got %d", cfg.Crypto.KDFMemory)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("Expected log level to be warn (CLI override), got %s", cfg.Logging.Level)
	}

	// Validate plugin config merge
	s3Plugin := profile.PluginConfig("io.aers.storage.s3")
	if s3Plugin == nil {
		t.Fatalf("Expected s3 plugin config to be parsed")
	}
	if s3Plugin["bucket"] != "user-bucket" {
		t.Errorf("Expected plugin bucket to be user-bucket, got %v", s3Plugin["bucket"])
	}
	if s3Plugin["region"] != "us-east-1" {
		t.Errorf("Expected plugin region to be us-east-1, got %v", s3Plugin["region"])
	}

	// Validate Source tracking
	if src := profile.Source("storage.output_dir"); src != SourceUserFile {
		t.Errorf("Expected storage.output_dir to come from UserFile, got %s", src)
	}
	if src := profile.Source("crypto.kdf_memory"); src != SourceSystemFile {
		t.Errorf("Expected crypto.kdf_memory to come from SystemFile, got %s", src)
	}
	if src := profile.Source("logging.level"); src != SourceCLI {
		t.Errorf("Expected logging.level to come from CLI, got %s", src)
	}
	if src := profile.Source("discovery.timeout"); src != SourceDefault {
		t.Errorf("Expected discovery.timeout to come from Default, got %s", src)
	}
}

func TestStrictDecodingRejectsUnknownKeys(t *testing.T) {
	tmpDir := t.TempDir()
	sysFile := filepath.Join(tmpDir, "system.yaml")

	// 'unknown_field' should fail strict decoding in a core domain
	sysYAML := `
storage:
  output_dir: /system/backups
  unknown_field: true
`
	if err := os.WriteFile(sysFile, []byte(sysYAML), 0644); err != nil {
		t.Fatalf("Failed to write sys file: %v", err)
	}

	manager := NewManager()
	_, err := manager.Load(LoadOptions{SystemConfigFile: sysFile})
	if err == nil {
		t.Fatalf("Expected load to fail due to unknown field")
	}
}

func TestStrictDecodingAcceptsUnknownPluginKeys(t *testing.T) {
	tmpDir := t.TempDir()
	sysFile := filepath.Join(tmpDir, "system.yaml")

	// Plugins map should accept arbitrary keys safely
	sysYAML := `
plugins:
  io.aers.discovery.apt:
    foo: bar
`
	if err := os.WriteFile(sysFile, []byte(sysYAML), 0644); err != nil {
		t.Fatalf("Failed to write sys file: %v", err)
	}

	manager := NewManager()
	profile, err := manager.Load(LoadOptions{SystemConfigFile: sysFile})
	if err != nil {
		t.Fatalf("Expected load to succeed for unknown plugin keys, got error: %v", err)
	}

	pluginCfg := profile.PluginConfig("io.aers.discovery.apt")
	if pluginCfg["foo"] != "bar" {
		t.Errorf("Expected plugin key 'foo' to be 'bar'")
	}
}

func TestValidationFailuresAccumulate(t *testing.T) {
	manager := NewManager()
	cliOverrides := map[string]any{
		"logging": map[string]any{
			"level": "potato",
		},
		"crypto": map[string]any{
			"algorithm": "banana",
		},
		"storage": map[string]any{
			"retention_policy": -5,
		},
	}
	_, err := manager.Load(LoadOptions{CLIOverrides: cliOverrides})
	if err == nil {
		t.Fatalf("Expected load to fail due to schema validation")
	}

	configErr, ok := err.(*ConfigError)
	if !ok {
		t.Fatalf("Expected error of type *ConfigError")
	}
	
	if len(configErr.Errors) != 3 {
		t.Fatalf("Expected exactly 3 validation errors, got %d", len(configErr.Errors))
	}
}

func TestProfileImmutability(t *testing.T) {
	manager := NewManager()
	profile, err := manager.Load(LoadOptions{})
	if err != nil {
		t.Fatalf("Expected successful load: %v", err)
	}

	// Fetch config and mutate it
	cfg1 := profile.Config()
	cfg1.Logging.Level = "debug"

	if profile.Config().Logging.Level == "debug" {
		t.Errorf("Profile was mutated by altering Config struct field")
	}

	// Mutate nested plugin map
	pluginMap := profile.PluginConfig("test-plugin")
	if pluginMap == nil {
		// Mock a plugin map into it for testing
		cliOverrides := map[string]any{
			"plugins": map[string]any{
				"apt": map[string]any{
					"timeout": 30,
				},
			},
		}
		profile, _ = manager.Load(LoadOptions{CLIOverrides: cliOverrides})
	}

	cfg2 := profile.Config()
	if cfg2.Plugins == nil {
		t.Fatalf("Plugins map shouldn't be nil")
	}

	cfg2.Plugins["apt"]["timeout"] = 999

	if profile.Config().Plugins["apt"]["timeout"] == 999 {
		t.Errorf("Profile was mutated by altering nested plugin map")
	}
}

func TestNestedMergeBehavior(t *testing.T) {
	tmpDir := t.TempDir()

	sysFile := filepath.Join(tmpDir, "system.yaml")
	userFile := filepath.Join(tmpDir, "user.yaml")

	sysYAML := `
plugins:
  apt:
    timeout: 30
    retries: 2
`

	userYAML := `
plugins:
  apt:
    timeout: 10
`

	os.WriteFile(sysFile, []byte(sysYAML), 0644)
	os.WriteFile(userFile, []byte(userYAML), 0644)

	manager := NewManager()
	profile, err := manager.Load(LoadOptions{
		SystemConfigFile: sysFile,
		UserConfigFile:   userFile,
	})
	if err != nil {
		t.Fatalf("Expected successful load: %v", err)
	}

	aptCfg := profile.PluginConfig("apt")
	if aptCfg == nil {
		t.Fatalf("Expected apt plugin to be loaded")
	}

	if aptCfg["timeout"] != 10 {
		t.Errorf("Expected timeout=10, got %v", aptCfg["timeout"])
	}
	if aptCfg["retries"] != 2 {
		t.Errorf("Expected retries=2 (nested merge), got %v", aptCfg["retries"])
	}
}
