# S17 — Configuration Manager Implementation Plan

## Goal

Implement the Configuration Manager subsystem (S17) for the AERS project, following all documentation as the source of truth.

---

## Requirements Covered

| Requirement | Coverage |
|-------------|----------|
| **FR-11.1** | 5-tier priority hierarchy: CLI flags > env vars > user config > system config > defaults |
| **FR-11.2** | Merge from all sources with higher-priority overrides |
| **FR-11.3** | Validate merged config: reject unknown keys, enforce required fields, validate types/ranges |
| **FR-11.4** | Refuse to start on invalid config; report specific validation error |
| **FR-11.5** | Configuration introspection: show effective values and which source each came from |
| **FR-11.6** | Sensible built-in defaults for basic operation without user config |
| **NFR-6.3** | Sensible defaults (maps to FR-11.6) |

## Architecture Constraints (from 02_Architecture.md §S17)

- **Layer:** Cross-cutting
- **Depends on:** None (leaf dependency)
- **Depended on by:** S2, S5, S6, S8, S9, S12, S13, S15
- **Read-only:** Does NOT write config files
- **No business logic:** Loads and serves configuration; does not interpret it

## Configuration Domains (from 02_Architecture.md §S17)

| Domain | Parameters |
|--------|-----------|
| **Storage** | Archive output directory, retention policy, storage backend selection |
| **Crypto** | Encryption algorithm, KDF parameters, GPG key ID for signing |
| **Discovery** | Plugin directories, discovery timeout, excluded paths |
| **Classification** | Custom importance overrides, user-defined include/exclude rules |
| **Scheduling** | Backup schedules, auto-approval for recurring schedules |
| **AI** | AI mode (disabled/local/cloud), model path, cloud API endpoint |
| **Logging** | Log level, log output path, audit log rotation |

## Data Model Constraints (from 04_Data_Model.md §Configuration Profile)

- Singleton entity (one per process invocation)
- Created at AERS startup by the Configuration Manager
- Read-only during process lifetime
- Rebuilt from sources on each invocation
- Required Metadata:
  - Effective configuration values
  - Source attribution for each value (CLI, env var, user config, system config, default)
  - Schema version
  - Validation timestamp
  - Validation status

---

## Proposed Changes

### Configuration Manager Package

All files go under `internal/config/` — the dedicated S17 package.

---

#### [NEW] [config.go](file:///c:/Users/Zeyad/PycharmProjects/AERS/internal/config/config.go)

Core types and the primary `Config` struct:

- `Config` struct with all 7 configuration domains as nested structs
- `StorageConfig`, `CryptoConfig`, `DiscoveryConfig`, `ClassificationConfig`, `SchedulingConfig`, `AIConfig`, `LoggingConfig`
- `Source` enum type (CLI, EnvVar, UserFile, SystemFile, Default) for introspection
- `TrackedValue[T]` generic type holding the effective value plus its source attribution
- `SchemaVersion` constant (set to "1.0" for V1)

#### [NEW] [defaults.go](file:///c:/Users/Zeyad/PycharmProjects/AERS/internal/config/defaults.go)

Built-in defaults for all configuration parameters:

- Sensible defaults per FR-11.6 / NFR-6.3
- Each default is a function returning the default `Config` with all sources set to `SourceDefault`

#### [NEW] [loader.go](file:///c:/Users/Zeyad/PycharmProjects/AERS/internal/config/loader.go)

Configuration loading and merging logic:

- `Loader` struct with configurable file paths and CLI overrides
- `Load() (*Profile, error)` — the main entry point
- File loading: reads YAML from user config and system config paths
- Environment variable reading for each supported parameter
- 5-tier merge logic per FR-11.2
- Returns a `Profile` (the Configuration Profile entity from the data model)

#### [NEW] [validate.go](file:///c:/Users/Zeyad/PycharmProjects/AERS/internal/config/validate.go)

Schema validation per FR-11.3 / FR-11.4:

- Validates all fields: types, ranges, required fields
- Returns structured `ValidationError` with field path, expected value, and actual value
- Validates enumeration values (AI mode, log level, encryption algorithm, etc.)
- Validates path patterns and numeric ranges

#### [NEW] [profile.go](file:///c:/Users/Zeyad/PycharmProjects/AERS/internal/config/profile.go)

The `Profile` type — the read-only Configuration Profile entity:

- Wraps the validated `Config`
- Provides read-only accessor methods for each domain
- Carries metadata: schema version, validation timestamp, validation status
- Provides `Introspect()` method returning source attribution per FR-11.5
- Provides `PluginConfig(pluginID string)` accessor for plugin-specific config sections

#### [NEW] [env.go](file:///c:/Users/Zeyad/PycharmProjects/AERS/internal/config/env.go)

Environment variable mapping:

- Maps documented env vars to config fields (e.g., `AERS_LOG_LEVEL`, `AERS_STORAGE_DIR`, `AERS_AI_MODE`)
- Parsing logic for each type (string, int, bool, duration)

---

### Test Files

#### [NEW] [config_test.go](file:///c:/Users/Zeyad/PycharmProjects/AERS/internal/config/config_test.go)

Unit tests for core types.

#### [NEW] [defaults_test.go](file:///c:/Users/Zeyad/PycharmProjects/AERS/internal/config/defaults_test.go)

Tests that defaults produce valid, complete configuration.

#### [NEW] [loader_test.go](file:///c:/Users/Zeyad/PycharmProjects/AERS/internal/config/loader_test.go)

Tests for:

- Loading from YAML files
- 5-tier merge priority
- Environment variable overrides
- CLI flag overrides
- Missing files (graceful handling)
- Malformed YAML (error reporting)

#### [NEW] [validate_test.go](file:///c:/Users/Zeyad/PycharmProjects/AERS/internal/config/validate_test.go)

Tests for:

- Valid configuration passes
- Invalid enum values rejected
- Out-of-range numeric values rejected
- Required fields enforced
- Unknown keys rejected
- Error messages include field path

#### [NEW] [profile_test.go](file:///c:/Users/Zeyad/PycharmProjects/AERS/internal/config/profile_test.go)

Tests for:

- Read-only access
- Source attribution / introspection
- Plugin config access

#### [NEW] [env_test.go](file:///c:/Users/Zeyad/PycharmProjects/AERS/internal/config/env_test.go)

Tests for environment variable parsing.

---

## Design Decisions

1. **Generic `TrackedValue[T]`** — Each config field is a `TrackedValue` that carries both the value and its source. This enables FR-11.5 introspection without a separate parallel data structure.

2. **YAML via `gopkg.in/yaml.v3`** — Standard Go YAML library. Supports strict mode to reject unknown keys per FR-11.3.

3. **No dependency on other AERS subsystems** — S17 is a leaf dependency. It does not import the logger or any other internal package. It returns errors that the caller (Orchestrator) can log.

4. **Plugin configuration via `map[string]map[string]any`** — The `plugins` section in the YAML file is keyed by plugin ID. The Configuration Manager parses and stores it but does not validate plugin-specific schemas (that's S12's job per 05_Plugin_API.md §Plugin Configuration).

5. **Env var naming convention:** `AERS_<DOMAIN>_<PARAM>` (e.g., `AERS_LOG_LEVEL`, `AERS_STORAGE_DIR`).

6. **Strict YAML parsing** — Uses `yaml.v3` `KnownFields(true)` to reject unknown keys, satisfying FR-11.3.

## Verification Plan

### Automated Tests

- `go test ./internal/config/... -v -count=1`
- `go build ./...` — full project compilation check

### Manual Verification

- Verify all FR-11.x requirements are covered by tests
- Verify architecture constraints are respected (no OS-specific logic, no business logic, read-only)
