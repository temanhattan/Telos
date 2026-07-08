# Title

Configuration Manager Design

# Status

Accepted

# Context

The Configuration Manager (subsystem S17) exists to serve as the unified, definitive source of truth for all operational settings within the AERS system. 

Configuration must be centralized to prevent scattered, inconsistent logic where different subsystems attempt to parse files, resolve environment variables, or read command-line arguments independently. Because AERS operates in a decoupled architecture where independent subsystems rely on configuration (e.g., Storage needs backend paths, Crypto needs algorithms, Discovery needs plugin directories), every subsystem depends on the Configuration Manager.

To guarantee a deterministic and consistent environment, the configuration must be loaded, resolved, and frozen before any other subsystem is initialized. This fulfills the requirement that operational parameters are centrally defined (FR-11) and provides a secure, predictable baseline for system behavior.

# Decision

The following architectural decisions have been made and implemented for the Configuration Manager:

* **Zero Internal Dependencies:** Configuration Manager is a leaf subsystem with zero internal dependencies. It does not import other AERS subsystems.
* **Immutability:** Configuration is immutable after loading.
* **Abstraction:** Configuration is represented by a `Profile` interface abstraction rather than exposing mutable concrete structs. Deep copies are utilized to protect internal slices and maps from modification.
* **Priority Order:** Configuration loading follows a strict 5-tier priority order:
  1. Default (Built-in fallbacks)
  2. System Config (Global `/etc/aers/`)
  3. User Config (Local `~/.config/aers/`)
  4. Environment Variables (`AERS_*`)
  5. CLI (Command-line overrides)
* **Strong Typing:** Core configuration domains (Storage, Crypto, Logging, etc.) are strongly typed within Go structs.
* **Schema-Agnostic Plugins:** Plugin configuration is intentionally schema-agnostic and relies on dynamic mapping: `Plugins map[string]map[string]any`. S17 passes these maps through, but plugin schemas are validated by the Plugin Host (S12), not S17.
* **Source Attribution:** Configuration source attribution (which layer provided a specific value) is tracked and stored separately from the configuration values themselves, accessible via `Profile.Source(path)`.
* **Deep Merging:** Recursive deep merge is required and implemented, ensuring nested configuration structures (like individual plugin settings) merge properly without completely overwriting siblings.
* **Strict Core Parsing:** Unknown keys in the core schema are explicitly rejected to prevent typos and misconfigurations using strict YAML decoding.
* **Dynamic Plugin Parsing:** Unknown plugin keys are accepted, allowing plugins to dictate their own structures without requiring upstream updates to the core S17 schema.
* **Separated Validation:** Validation is separated into two distinct stages:
  - **Schema validation:** Structural correctness, missing fields, valid enums, ranges.
  - **Business validation:** Semantic checks that yield non-fatal warnings (e.g., empty plugin directories or dangerous flags).
* **Pure Configuration Processing:** S17 performs no filesystem existence checks (no `os.Stat`), no directory creation, no package validation, no storage operations, no logging, and no cryptography.
* **Single Responsibility:** S17 is solely responsible for exactly five operations: Read → Merge → Validate → Freeze → Expose.
* **YAML Tooling:** YAML parsing uses `gopkg.in/yaml.v3` with strict decoding (`KnownFields(true)`).
* **Bulk Errors:** Validation accumulates all errors across the entire configuration object before returning, preventing "guess-and-check" trial loops for the user.

# Consequences

**Benefits:**
- **Deterministic configuration:** The immutable, frozen profile ensures that configuration cannot change out from under running subsystems.
- **Reproducible startup:** A clear 5-tier priority with strong schemas means the system predictably boots exactly the same way given the same inputs.
- **Clean dependency graph:** By remaining a leaf node, S17 prevents circular dependencies and can be tested in absolute isolation.
- **Plugin independence:** Using `map[string]map[string]any` means the core does not need to be rebuilt or modified when new plugins with complex configurations are added.
- **Better UX:** Bulk validation errors and strict YAML parsing stop users from experiencing silent failures due to typos.

**Trade-offs:**
- **Performance overhead:** Deep copying maps and slices on every call to `Profile.Config()` introduces a small performance cost and allocation overhead.
- **Deferred plugin validation:** Plugin configurations cannot be statically validated at the S17 level; typos inside a plugin block will pass S17 and must be caught later by S12.
- **Complex merge tracking:** Deep recursive merging and attribution tracking requires significantly more code than a simple struct unmarshal.

# Alternatives Considered

1. **TrackedValue[T] embedded in every field.**
   *Rejected because* it pollutes the configuration domain structs. Every consumer would have to call `.Value()` to access fields, significantly worsening ergonomics. Tracking sources parallel to the data keeps the configuration struct clean.

2. **Performing os.Stat() during configuration loading.**
   *Rejected because* it violates S17's role as a pure leaf dependency and introduces environmental coupling. Verifying if a directory exists or is writable is operational logic that belongs to the subsystem actually using the directory (e.g., Storage Backend).

3. **Flat configuration without source attribution.**
   *Rejected because* without source tracking, debugging conflicting configurations (e.g., trying to figure out if a value came from ENV or CLI) becomes impossible.

4. **Plugin configuration using arbitrary interface{}.**
   *Rejected because* it's too loose. `map[string]map[string]any` enforces that every plugin configuration is fundamentally an object map keyed by the plugin ID, which aligns perfectly with how the Plugin Host passes config blocks to individual plugins.

# Implementation Notes

The implementation satisfies these decisions through the following structures:

- **`config.go`**: Defines the strongly-typed core domains and the `Plugins map[string]map[string]any` dynamic structure.
- **`defaults.go`**: Centralizes the fallback values for the `SourceDefault` tier.
- **`env_mapper.go`**: Projects flat `AERS_X_Y` environment variables into nested maps for proper merging.
- **`merge.go`**: Implements recursive deep merging while populating the separated source attribution map.
- **`schema_validation.go`**: Implements the strict rules for core domains and returns bulk `ConfigError` arrays.
- **`business_validation.go`**: Implements non-fatal semantic checks that result in warnings.
- **`profile.go`**: Encapsulates the configuration in a private `profile` struct and exposes only the read-only `Profile` interface, actively performing deep clones upon access.
- **`loader.go`**: Exposes the `Manager` interface, acting as the pipeline coordinator for reading, merging, validating, and freezing the configuration.

# Related Documents

- [Vision](../00_Vision.md)
- [Requirements](../01_Requirements.md)
- [Architecture](../02_Architecture.md)
- [Data Model](../04_Data_Model.md)
