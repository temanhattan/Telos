# S17 Configuration Manager Implementation

I have successfully implemented the **Configuration Manager (S17)**. Based on the detailed architectural review and your follow-up feedback, the subsystem correctly encapsulates reading, merging, validating, and freezing configuration safely. It acts as a pure leaf dependency without interacting with the OS filesystem for paths or touching any S8 storage logic.

## 1. Summary of Changes

The S17 subsystem has been fully implemented in `internal/config/`. It exposes a clean `Manager` interface that returns an immutable `Profile` interface abstraction. 

It supports the 5-layer priority queue exactly as requested:
1. `SourceDefault`: Sensible code defaults.
2. `SourceSystemFile`: System-wide YAML.
3. `SourceUserFile`: User-local YAML.
4. `SourceEnvVar`: Environment variables.
5. `SourceCLI`: Command line flags.

It strictly validates the known schema correctness while parsing dynamically typed plugin namespaces, accumulating all errors cleanly.

## 2. Design Decisions & Adjustments

Following your deep-dive review, several critical refinements were implemented:

- **Immutability Guarantee (Deep Copy)**: Calling `Profile.Config()` deep-copies all internal slices, and importantly, the `Plugins` map and its children. Mutations to the returned values do not reflect back onto the immutable profile instance.
- **Interface Abstraction**: `Profile` is now exposed strictly as an `interface` (`Config()`, `Source()`, `Metadata()`, `PluginConfig()`), hiding the concrete `profile` struct and ensuring dependencies consume S17 purely through abstractions.
- **Nested Recursive Merges**: `mergeMapAndTrack` correctly drills down into deeply nested maps. A user overriding `plugins.apt.timeout` cleanly merges alongside `plugins.apt.retries` inherited from the system config, avoiding destructive shallow overrides.
- **Strict Core vs Dynamic Plugins**: The final parse utilizes `yaml.NewDecoder` with `KnownFields(true)`. This enforces strict typos on core fields (e.g. `storage.unknown_key` throws), but safely unmarshals arbitrary dynamic structures explicitly scoped beneath `plugins.<id>`.
- **Bulk Validations**: `schema_validation.go` accumulates all `ValidationError` elements continuously into a `ConfigError` array before returning. If a user mangles three independent fields, they receive exactly three precise error paths in a single pass.

## 3. Tests Added

A comprehensive unit test suite in `loader_test.go` verifies all structural edge cases:
- `TestManagerLoad`: Correct merge behavior and source attribution when System, User, and CLI overrides collide.
- `TestProfileImmutability`: Proves that mutating retrieved maps or structs fails to alter the underlying profile state.
- `TestNestedMergeBehavior`: Verifies multi-tier deep YAML merging on dynamic maps.
- `TestStrictDecodingRejectsUnknownKeys`: Ensures rigid typo safety on the core schema.
- `TestStrictDecodingAcceptsUnknownPluginKeys`: Ensures that plugin namespaces remain schema-agnostic.
- `TestValidationFailuresAccumulate`: Proves that a totally malformed config outputs a unified payload of all structural failures in one pass.

## 4. Remaining Limitations

Because S17 is an isolated leaf subsystem, the CLI args handling currently takes an injected `map[string]any` via `LoadOptions`. The top-level package (`cmd/aers/main.go`) will eventually need to map CLI flags to this generic tree map to pump them into the config manager.

## 5. Verification Against Requirements

- **FR-11.1 (Multi-tier load):** Fully integrated via `merge.go` and `loader.go`.
- **FR-11.2 (Immutability):** `Profile` exposes deep copies via interface.
- **FR-11.3 (Strict format):** Covered natively by `gopkg.in/yaml.v3` `KnownFields(true)`.
- **FR-11.4 (Bulk errors):** Accomplished with custom `ConfigError` array accumulation.
- **FR-11.5 (Warnings over Failures):** Segregated perfectly into `validateBusiness()`.
- **FR-11.6 (Built-in Defaults):** Done gracefully in `defaults.go`.
