# AERS — Plugin API Specification

> **Status:** Ratified
> **Last Updated:** 2026-07-01
> **Owner:** temanhattan
> **Audience:** All future contributors, AI sessions, and design reviewers
> **Source of Truth:** [00_Vision.md](00_Vision.md), [01_Requirements.md](01_Requirements.md), [02_Architecture.md](02_Architecture.md)

---

## Table of Contents

1. [Purpose](#purpose)
2. [Scope](#scope)
3. [Design Principles](#design-principles)
4. [Plugin Identity Model](#plugin-identity-model)
5. [Interface Versioning](#interface-versioning)
6. [Plugin Packaging](#plugin-packaging)
7. [Plugin Execution Model](#plugin-execution-model)
8. [Plugin Type Contracts](#plugin-type-contracts)
   - [Discovery Plugin](#discovery-plugin)
   - [Classification Rule Plugin](#classification-rule-plugin)
   - [Capture Plugin](#capture-plugin)
   - [Restore Plugin](#restore-plugin)
   - [Storage Plugin](#storage-plugin)
9. [Plugin Lifecycle](#plugin-lifecycle)
10. [Plugin Configuration](#plugin-configuration)
11. [Capability Negotiation](#capability-negotiation)
12. [Sandbox and Permission Model](#sandbox-and-permission-model)
13. [Plugin Trust Model](#plugin-trust-model)
14. [Plugin Signature Verification](#plugin-signature-verification)
15. [Error Handling and Isolation](#error-handling-and-isolation)
16. [Shared Types](#shared-types)
17. [Plugin Dispatch Rules](#plugin-dispatch-rules)
18. [Traceability](#traceability)

---

## Purpose

This document is the authoritative specification of the AERS Plugin API — the contract between the stable core system and the extensible plugin ecosystem.

The plugin boundary is the most important architectural boundary in AERS. It separates the invariant core — orchestration, cryptography, storage, verification — from the variant periphery — OS-specific discovery, package-manager integrations, cloud adapters, and classification rules. The quality of this boundary determines whether AERS achieves its vision of a platform that grows without core modification.

This specification defines **what** the plugin interface requires. It does **not** define implementation details, technology choices, serialization formats, or internal data representations. Those decisions belong to implementation-level design documents.

---

## Scope

This specification covers:

- The plugin identity and metadata model.
- The interface versioning scheme.
- The plugin packaging structure.
- The plugin execution model.
- The contracts for all five plugin types: Discovery, Classification Rule, Capture, Restore, and Storage.
- The plugin lifecycle: discovery, validation, registration, dispatch, and teardown.
- The plugin configuration model.
- The capability negotiation protocol.
- The sandbox and permission model.
- The plugin trust model and signature verification.
- The error handling and isolation protocol.
- The shared type vocabulary.
- The dispatch rules that route requests to plugins.

This specification does **not** cover:

- Implementation language or framework choices.
- Serialization format or transport protocol specifics.
- Test harness, SDK tooling, or developer documentation (deferred to Phase 4 — Community).
- Plugin registry infrastructure (deferred to Phase 4 — Community).

---

## Design Principles

The following principles govern all plugin interface decisions. They are derived from the [Project Vision](00_Vision.md) and the [System Architecture](02_Architecture.md).

### 1. Plugins Are Pure Functions of Their Inputs

A plugin receives a request and returns a response. It does not access global state, other subsystems, or other plugins. The only information available to a plugin is what the Plugin Host explicitly provides in the request.

### 2. Plugins Declare Their Capabilities

At registration time, a plugin declares which operating system families, package managers, cloud providers, or storage protocols it supports, and which plugin type it implements. The Plugin Host uses these declarations to route requests. A plugin never receives a request outside its declared capabilities.

### 3. Plugins Are Versioned

Each plugin declares the plugin interface version it targets. The Plugin Host refuses to load plugins targeting an incompatible interface version. This is a hard compatibility gate, not a soft negotiation.

### 4. Plugins Fail Safely

A plugin crash, timeout, or malformed response results in a logged error and a partial or incomplete result in the affected manifest section — never a core crash. The Plugin Host is the isolation boundary between plugins and the core.

### 5. Plugins Are Sandboxed

Plugins operate within a declared permission scope. They cannot access the filesystem outside their declared paths, cannot make network requests unless explicitly authorized, cannot invoke arbitrary subprocesses, and cannot access cryptographic material or other core subsystems directly.

### 6. Plugins Are Stateless

Plugins do not retain state between invocations. Each request is self-contained. If a plugin requires persistent state (e.g., a cache), it must manage that state through explicitly declared permissions and receive the relevant paths through its validated configuration.

### 7. Plugins Are Trustworthy or Rejected

Every plugin undergoes trust verification before it is permitted to register. Unsigned or untrusted plugins are not loaded without explicit user override. The plugin trust chain is verified before any plugin code executes.

---

## Plugin Identity Model

Every plugin must declare a **plugin manifest** — a static metadata declaration that the Plugin Host (S12) reads at registration time, before any plugin code is executed. The manifest is the plugin's identity card: it tells the Plugin Host what the plugin is, what it does, what it needs, and what interface version it targets.

```
PluginManifest
├── id: string                  — Globally unique identifier (reverse-domain convention: "io.aers.discovery.apt")
├── name: string                — Human-readable display name ("APT Package Discovery")
├── version: SemVer             — Plugin version ("1.2.0")
├── interface_version: int      — Target plugin interface version (e.g., 1)
├── type: PluginType            — One of: Discovery, ClassificationRule, Capture, Restore, Storage
├── capabilities: Capabilities
│   ├── os_families: string[]       — ["linux"], ["windows"], ["darwin"], or combinations
│   ├── package_managers: string[]  — ["apt", "dpkg"], ["winget", "choco"], etc.
│   ├── cloud_providers: string[]   — ["aws"], ["gcp"], ["azure"], etc.
│   ├── discovery_categories: string[] — ["packages", "services", ...] (Discovery plugins only)
│   └── storage_protocols: string[] — ["file", "s3", "gcs", ...] (Storage plugins only)
├── author: string              — Plugin author or organization
├── description: string         — Human-readable description of the plugin's purpose
├── permissions: Permissions
│   ├── filesystem_read: string[]   — Path patterns the plugin requires read access to
│   ├── filesystem_write: string[]  — Path patterns the plugin requires write access to (Restore plugins only)
│   ├── network: bool               — Whether the plugin requires network access
│   └── subprocess: string[]        — Executable names the plugin may invoke (e.g., ["apt", "dpkg-query"])
├── configuration_schema: SchemaDefinition? — Schema describing the plugin's expected configuration (see §Plugin Configuration)
└── signature: SignatureBlock?  — Digital signature over the manifest and package contents (see §Plugin Signature Verification)
```

The manifest is a declarative document. It contains no executable logic. The Plugin Host parses and validates the manifest before any plugin code is loaded or executed.

---

## Interface Versioning

The plugin interface is versioned to enable controlled evolution of the plugin contract without breaking existing plugins.

| Concept | Rule |
|---------|------|
| **Interface version** | A monotonically increasing integer starting at `1`. |
| **Breaking change** | Any change to a plugin type's request schema, response schema, context structure, or behavioral contract constitutes a breaking change and increments the interface version. |
| **Additive changes** | New optional fields in request or response schemas do not constitute breaking changes and do not increment the interface version. Plugins must tolerate unknown fields in requests. |
| **Version declaration** | Every plugin declares its target interface version in its manifest. The Plugin Host checks this value before loading. |
| **Compatibility gate** | The Plugin Host refuses to load any plugin whose `interface_version` does not match the host's supported version. This is a hard rejection, not a negotiation. |
| **Version 1 scope** | Version 1 of AERS supports exactly one plugin interface version. The Plugin Host loads only plugins targeting that version. Backward compatibility adapters for previous interface versions are explicitly deferred to Version 2. |
| **Deprecation policy** | When a new interface version is released in future AERS versions, the previous version enters a deprecation window of at least one major AERS release cycle before support may be dropped. |

---

## Plugin Packaging

Plugins are packaged as **directory-based plugin packages**. Each plugin is a self-contained directory that includes everything the Plugin Host needs to identify, validate, trust, and invoke the plugin.

### Package Structure

```
plugin-directory/
├── manifest             — Plugin manifest file (required)
├── executable(s)        — Plugin executable(s) invoked by the Plugin Host (required)
├── schemas/             — Optional schema definitions for plugin-specific data formats
├── resources/           — Optional resources the plugin requires at runtime
└── docs/                — Optional documentation for plugin users and developers
```

### Package Rules

| Rule | Description |
|------|-------------|
| **Self-contained** | A plugin package must contain everything it needs to operate. It must not reference files outside its own directory, except for paths explicitly declared in its `permissions`. |
| **Manifest at root** | The plugin manifest must be located at the root of the plugin directory. The Plugin Host locates plugins by scanning for manifest files. |
| **No shared state** | Plugin packages must not share files, directories, or state with other plugin packages. Each package is isolated. |
| **Portable** | Plugin packages must not contain absolute paths. All internal references must be relative to the plugin directory root. |
| **Atomic installation** | Installing a plugin means placing its package directory into a configured plugin directory. Removing a plugin means removing its package directory. There is no registration database outside the filesystem. |

---

## Plugin Execution Model

Plugins execute as **isolated subprocesses**. The Plugin Host spawns a plugin process for each invocation, communicates with it through a stable, versioned protocol, and terminates the process when the invocation completes.

### Execution Principles

| Principle | Description |
|-----------|-------------|
| **Process isolation** | Each plugin invocation runs in a separate process. A plugin crash, memory leak, or resource exhaustion does not affect the core system or other plugins. |
| **Protocol-driven communication** | The Plugin Host communicates with plugins through a defined protocol. The protocol specifies the structure of requests and responses. This specification does not prescribe the serialization format or transport mechanism; those are implementation decisions. |
| **Versioned protocol** | The communication protocol is versioned in lockstep with the plugin interface version. A plugin targeting interface version `N` uses protocol version `N`. |
| **Lifecycle boundaries** | A plugin process is started when the Plugin Host dispatches a request and terminated when the response is received or a timeout is reached. Plugins must not assume persistence across invocations. |
| **No direct subsystem access** | Plugin subprocesses have no mechanism to call back into core subsystems. They operate exclusively on the data provided in the request and return their results in the response. |
| **Timeout enforcement** | Every plugin invocation is subject to a configurable timeout. If a plugin does not respond within the timeout, the Plugin Host terminates the subprocess and treats the invocation as a failure. |

---

## Plugin Type Contracts

AERS defines five plugin types. Each type has a specific purpose, a defined consumer within the core, a request structure, a response structure, and a behavioral contract that the plugin must honor.

---

### Discovery Plugin

**Purpose:** Discover environment components for a specific operating system, package manager, service supervisor, or domain.

**Consumer:** S3 — Discovery Engine

#### Request

```
DiscoveryRequest
├── category: DiscoveryCategory          — The category being discovered (packages, services, network, etc.)
├── platform_context: PlatformContext
│   ├── os_family: string                — "linux", "windows", "darwin"
│   ├── os_id: string                    — "ubuntu", "debian", "windows"
│   ├── os_version: string               — "22.04", "12", "11"
│   ├── architecture: string             — "amd64", "arm64"
│   └── kernel: string                   — Kernel version string
└── plugin_config: PluginConfiguration   — Validated, plugin-specific configuration (see §Plugin Configuration)
```

#### Response

```
DiscoveryResponse
├── status: ResponseStatus               — success | partial | failure
├── entries: ManifestEntry[]             — Discovered entries to add to the manifest
│   └── (structure matches the corresponding manifest section schema from S4)
├── warnings: Warning[]                  — Non-fatal issues encountered during discovery
└── error: ErrorDetail?                  — Present only if status is "failure"
```

#### Behavioral Contract

1. Discovery is **read-only**. The plugin MUST NOT modify the source system.
2. Discovery MUST NOT require network access.
3. The plugin MUST return within the configured timeout (default: 60 seconds per category).
4. The plugin MUST handle partial failures internally and return `status: partial` with whatever entries it successfully discovered, rather than failing entirely.
5. The plugin MUST NOT read credential file contents. Credential discovery reports the location and type of credential files — never their contents.
6. The plugin MUST return entries that conform to the manifest section schema for the requested category.

---

### Classification Rule Plugin

**Purpose:** Provide OS-specific or domain-specific classification rules that annotate manifest entries with importance, intent, and reproducibility.

**Consumer:** S5 — Classifier

#### Request

```
ClassificationRequest
├── entry: ManifestEntry                 — The manifest entry to classify
├── entry_section: string                — Which manifest section this entry belongs to ("packages", "services", etc.)
├── machine_context: MachineContext
│   ├── role: string?                    — Machine role if already determined (e.g., "web-server")
│   ├── os_family: string
│   ├── os_id: string
│   └── os_version: string
└── plugin_config: PluginConfiguration   — Validated, plugin-specific configuration
```

#### Response

```
ClassificationResponse
├── status: ResponseStatus               — success | partial | failure
├── annotations: Annotations
│   ├── importance: ImportanceLevel?      — critical | recommended | optional | transient
│   ├── intent: string?                  — Human-readable intent ("reverse proxy", "Python dev toolchain")
│   ├── reproducibility: Reproducibility? — reproducible | irreplaceable
│   ├── category: string?               — Finer-grained category label
│   └── confidence: float?              — 0.0–1.0 confidence in the annotation
├── warnings: Warning[]
└── error: ErrorDetail?
```

#### Behavioral Contract

1. Classification is **annotation-only**. The plugin MUST NOT modify the entry's discovery data. It may only propose annotations.
2. Classification MUST NOT access the filesystem or network.
3. Classification rules provided by plugins sit in the **middle tier** of the classification hierarchy: `Deterministic Rules > Plugin Rules > AI Suggestions`. The Classifier (S5) enforces this ordering; the plugin does not need to be aware of it.
4. If the plugin has no opinion on an entry, it MUST return `status: success` with all annotation fields set to null. Silence is a valid response.
5. The plugin MUST NOT perform resource-intensive operations. Classification is invoked per manifest entry and must be fast.

---

### Capture Plugin

**Purpose:** Export configurations or data in a platform-specific manner during backup execution.

**Consumer:** S7 — Capture Engine

#### Request

```
CaptureRequest
├── action: BackupPlanAction             — The specific backup plan action to execute
│   ├── action_id: string
│   ├── category: string
│   ├── method: CaptureMethod            — reference | copy | export
│   ├── source_path: string?            — Filesystem path (for copy/export methods)
│   └── credential_isolation: bool
├── platform_context: PlatformContext
└── plugin_config: PluginConfiguration   — Validated, plugin-specific configuration
```

#### Response

```
CaptureResponse
├── status: ResponseStatus               — success | failure
├── artifacts: CapturedArtifact[]
│   ├── artifact_id: string
│   ├── content_type: string             — "file", "reference", "export"
│   ├── artifact_location: ArtifactLocation — Where the plugin placed the captured artifact (see below)
│   ├── reference: PackageReference?     — Package name + version + source (for reference capture)
│   │   ├── manager: string
│   │   ├── name: string
│   │   ├── version: string
│   │   └── source: string
│   └── metadata: ArtifactMetadata       — Additional artifact metadata (size, content hash, format)
├── warnings: Warning[]
└── error: ErrorDetail?
```

#### Artifact Ownership

Capture plugins produce artifacts. For reference captures, the artifact is a structured record (package name, version, source) with no binary content. For file copy and configuration export captures, the plugin writes the artifact to a **staging location** designated by the Plugin Host in the request context. The plugin does not return large binary content in-memory; instead, it reports the location of the produced artifact and the core assumes ownership.

The core is then responsible for streaming the staged artifact through the Crypto Engine (S9) into storage (S8). This separation ensures that artifact data flows efficiently through the pipeline without requiring plugins to manage encryption, hashing, or storage.

```
Artifact Flow:
  Plugin writes artifact → Staging location (designated by Plugin Host)
  Core reads from staging → Crypto Engine (hash + encrypt) → Storage Backend
  Core cleans up staging after successful storage
```

The plugin relinquishes ownership of the artifact once it reports its location. The plugin MUST NOT modify or delete a staged artifact after returning its response.

#### Behavioral Contract

1. Capture is **read-only on the source system**. The plugin MUST NOT modify the source.
2. The plugin reads from `source_path` and writes to the designated staging location. It MUST NOT read files outside its declared `permissions.filesystem_read` scope.
3. For `method: export`, the plugin serializes a service or application's configuration into a portable format. The exact export format is plugin-defined but must be documented in the plugin's `schemas/` directory.
4. The plugin MUST NOT perform encryption. It produces plaintext artifacts; the Crypto Engine (S9) handles all encryption.
5. The plugin MUST NOT access credential file contents unless the action has `credential_isolation: true` AND the Plugin Host has explicitly authorized the access for that invocation.
6. The plugin MUST report accurate metadata (size, content hash) for each produced artifact.

---

### Restore Plugin

**Purpose:** Execute platform-specific restore actions on the target machine.

**Consumer:** S10 — Restore Engine

#### Request

```
RestoreRequest
├── action: RestorePlanAction            — The specific restore plan action to execute
│   ├── action_id: string
│   ├── type: RestoreActionType          — install | copy | write | execute | start
│   ├── target_path: string?
│   ├── is_destructive: bool
│   └── rollback_action: RollbackSpec?
├── artifact_location: ArtifactLocation? — Where the core has placed the decrypted artifact for this action
├── platform_context: PlatformContext
└── plugin_config: PluginConfiguration   — Validated, plugin-specific configuration
```

The Restore Engine is responsible for decrypting and staging artifacts before invoking the Restore Plugin. The plugin reads from the provided `artifact_location` — it never handles encrypted data.

#### Response

```
RestoreResponse
├── status: ResponseStatus               — success | failure
├── result: RestoreResult
│   ├── action_id: string
│   ├── outcome: Outcome                 — completed | skipped | failed
│   ├── changes_made: ChangeRecord[]     — What the plugin actually did to the target system
│   │   ├── type: string                 — "file_written", "package_installed", "service_started", etc.
│   │   ├── path: string?
│   │   └── detail: string
│   └── rollback_info: RollbackData?     — Data needed to undo this action if rollback is triggered
├── warnings: Warning[]
└── error: ErrorDetail?
```

#### Behavioral Contract

1. The Restore Plugin is the **only plugin type authorized to write to the target system**.
2. The plugin MUST only write to paths within its declared `permissions.filesystem_write` scope.
3. The plugin MUST only invoke subprocesses listed in its declared `permissions.subprocess` list.
4. The plugin MUST report **every change it makes** in `changes_made` for audit logging by the Logging & Audit Subsystem (S16).
5. If the plugin fails, it MUST return sufficient information in `rollback_info` for the Restore Engine to request a rollback of the failed action.
6. The plugin MUST be **idempotent**: invoking the same restore action twice must detect the existing state and return `outcome: skipped` without causing errors or duplication.
7. The plugin MUST NOT bypass the Approval Gate. It is invoked only after the user has approved the Restore Plan. The plugin has no mechanism to solicit additional approvals.
8. The plugin MUST NOT retain or copy decrypted artifact data beyond what is needed to execute the restore action.

---

### Storage Plugin

**Purpose:** Provide additional storage backends beyond the default local filesystem.

**Consumer:** S8 — Storage Backend

The Storage Plugin interface defines four operations: Write, Read, List, and Delete. Each operation has its own request and response structure.

#### Write Operation

**Request:**

```
StorageWriteRequest
├── archive_id: UUID
├── archive_metadata: ArchiveMetadata
│   ├── created_at: timestamp
│   ├── source_hostname: string
│   ├── label: string?
│   └── size_bytes: int
├── archive_location: ArtifactLocation   — Where the core has placed the encrypted archive for the plugin to read
└── plugin_config: PluginConfiguration   — Validated, plugin-specific configuration (bucket, path, credentials, etc.)
```

**Response:**

```
StorageWriteResponse
├── status: ResponseStatus               — success | failure
├── storage_identifier: string           — Backend-specific location identifier for future retrieval
├── warnings: Warning[]
└── error: ErrorDetail?
```

The Storage Plugin reads the encrypted archive from the location provided by the core and writes it to its target backend. The plugin does not receive the archive as an in-memory blob; it streams from the provided location. The core retains the source until the plugin confirms a successful write.

#### Read Operation

**Request:**

```
StorageReadRequest
├── archive_id: UUID
├── storage_identifier: string           — The identifier returned by the Write operation
├── target_location: ArtifactLocation    — Where the plugin should place the retrieved archive
└── plugin_config: PluginConfiguration
```

**Response:**

```
StorageReadResponse
├── status: ResponseStatus               — success | failure
├── archive_metadata: ArchiveMetadata?   — Metadata about the retrieved archive
├── warnings: Warning[]
└── error: ErrorDetail?
```

The Storage Plugin reads from its backend and writes the encrypted archive to the `target_location` provided by the core. The core then takes ownership and passes the archive to the Crypto Engine for decryption.

#### List Operation

**Request:**

```
StorageListRequest
├── filter: ListFilter?
│   ├── hostname: string?
│   ├── after: timestamp?
│   └── before: timestamp?
└── plugin_config: PluginConfiguration
```

**Response:**

```
StorageListResponse
├── status: ResponseStatus
├── archives: ArchiveMetadata[]
├── warnings: Warning[]
└── error: ErrorDetail?
```

#### Delete Operation

**Request:**

```
StorageDeleteRequest
├── archive_id: UUID
├── storage_identifier: string
└── plugin_config: PluginConfiguration
```

**Response:**

```
StorageDeleteResponse
├── status: ResponseStatus
├── warnings: Warning[]
└── error: ErrorDetail?
```

#### Behavioral Contract

1. The Storage Plugin operates on **encrypted, opaque archives**. It MUST NOT decrypt, inspect, or modify archive contents.
2. The Storage Plugin MUST verify that a write was durable before returning `status: success` (e.g., verifying that data has been persisted to the target medium).
3. Network access is permitted for Storage Plugins targeting remote backends — declared in `permissions.network: true`.
4. The Storage Plugin MUST NOT cache or retain archive data after an operation completes.
5. The Storage Plugin MUST return accurate `ArchiveMetadata` for List operations.
6. Delete operations MUST be idempotent: deleting an already-deleted archive returns `status: success`.

---

## Plugin Lifecycle

The Plugin Host (S12) manages plugins through the following lifecycle phases:

```
┌──────────┐    ┌───────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
│ Discover │ -> │   Trust   │ -> │ Validate │ -> │ Register │ -> │ Dispatch │ -> │ Teardown │
│  (scan)  │    │  (verify) │    │ (schema) │    │ (index)  │    │ (invoke) │    │ (unload) │
└──────────┘    └───────────┘    └──────────┘    └──────────┘    └──────────┘    └──────────┘
```

| Phase | What Happens |
|-------|-------------|
| **Discover** | At startup, the Plugin Host scans all configured plugin directories. Each directory containing a valid plugin manifest is identified as a plugin candidate. |
| **Trust** | The Plugin Host verifies the trust status of each candidate: checking the digital signature, comparing against the trust model (see §Plugin Trust Model), and rejecting unsigned or untrusted plugins according to the configured trust policy. |
| **Validate** | The Plugin Host reads each trusted manifest and validates it: (1) `interface_version` matches the host's supported version, (2) `type` is a known plugin type, (3) all required manifest fields are present and well-formed, (4) `id` is unique across all loaded plugins (no duplicates), (5) `configuration_schema` is valid (if present), (6) declared `permissions` are within acceptable bounds for the plugin type. Invalid plugins are logged and skipped. |
| **Register** | Valid plugins are indexed in an internal capability registry, keyed by `(type, os_family, category)` tuples. This enables efficient lookup during dispatch. The Plugin Host logs the registration of each plugin at `info` severity. |
| **Dispatch** | When a core subsystem needs a plugin, it queries the Plugin Host with a capability selector. The Plugin Host performs capability negotiation (see §Capability Negotiation), selects the appropriate plugin(s), prepares the request with validated configuration, spawns the plugin subprocess, and routes the request through the communication protocol. |
| **Teardown** | At application exit, the Plugin Host terminates any running plugin subprocesses and releases associated resources. Since plugins are stateless, teardown involves no state cleanup — it exists for orderly resource release. |

---

## Plugin Configuration

Plugins receive configuration through the `plugin_config: PluginConfiguration` field present in every request. This replaces arbitrary key-value maps with a typed, validated configuration model.

### Configuration Model

```
PluginConfiguration
├── schema_version: int              — Version of the configuration schema
├── values: ValidatedConfigValues    — Configuration values, validated against the plugin's declared schema
└── source: ConfigSource             — Which configuration source produced these values (for introspection)
```

Each plugin declares a `configuration_schema` in its manifest. This schema defines the structure, types, required fields, defaults, and valid ranges of the plugin's configuration parameters. The schema is declarative and technology-neutral.

### Configuration Flow

```
User configuration file (e.g., ~/.aers/config.yaml)
    └── Plugin-specific section, keyed by plugin ID
                ↓
Configuration Manager (S17) loads configuration from all sources,
    merges according to priority hierarchy, and validates plugin
    configuration against the plugin's declared schema.
                ↓
Plugin Host (S12) receives validated configuration for each plugin.
                ↓
Plugin receives its validated PluginConfiguration in the request.
```

### Configuration Rules

| Rule | Description |
|------|-------------|
| **Schema-validated** | The Configuration Manager validates all plugin configuration against the plugin's declared `configuration_schema` before the Plugin Host dispatches any request. If validation fails, the plugin is not invoked and the error is reported to the user. |
| **No direct file access** | Plugins MUST NOT read configuration files directly from the filesystem. All configuration is received through the `plugin_config` field in the request. |
| **No arbitrary maps** | Plugin configuration is not a generic key-value map. It is a typed structure conforming to the plugin's declared schema. This ensures that configuration errors are caught at validation time, not at runtime. |
| **Defaults in schema** | Default values for optional configuration parameters are declared in the plugin's `configuration_schema`. The Configuration Manager applies defaults before dispatch. |
| **Source transparency** | The `source` field in `PluginConfiguration` indicates which configuration source provided the values (CLI flag, environment variable, user config file, system config file, or built-in default). This supports configuration introspection as required by FR-11.5. |

---

## Capability Negotiation

When the Plugin Host receives a dispatch request from a core subsystem, it performs capability negotiation to select the appropriate plugin(s) for the operation. This protocol ensures that the correct plugin handles each request, that incompatible plugins are never invoked, and that the system can evolve without breaking existing plugins.

### Negotiation Process

The Plugin Host evaluates plugins through the following stages, in order:

| Stage | Criterion | Effect |
|-------|-----------|--------|
| **1. Interface Version** | Does the plugin target the interface version currently supported by the host? | Plugins targeting an unsupported version are excluded. |
| **2. Plugin Type** | Does the plugin implement the type required by the requesting subsystem? | Only plugins of the correct type proceed. |
| **3. Platform Compatibility** | Does the plugin declare support for the current platform's OS family? | Plugins that do not support the current platform are excluded. |
| **4. Capability Match** | Does the plugin declare capabilities matching the specific request? | For Discovery: does it support the requested `discovery_category`? For Capture/Restore: does it support the relevant `package_manager` or domain? For Storage: does it support the required `storage_protocol`? |
| **5. Specificity Ranking** | Among matching plugins, which is the most specific? | When multiple plugins match, the most specific wins (e.g., an `apt`-specific plugin over a generic `linux` plugin). |

### Selection Rules

| Subsystem | Selection Rule |
|-----------|---------------|
| **Discovery Engine (S3)** | All matching plugins are invoked. Results from all applicable plugins are aggregated into the manifest. |
| **Classifier (S5)** | All matching Classification Rule plugins are invoked per manifest entry. Annotations are merged according to the classification hierarchy (deterministic > plugin > AI). |
| **Capture Engine (S7)** | A single best-match plugin handles each capture action. Ambiguity is resolved by specificity ranking. |
| **Restore Engine (S10)** | A single best-match plugin handles each restore action. Ambiguity is resolved by specificity ranking. |
| **Storage Backend (S8)** | A single plugin handles each storage protocol. Multiple storage backends may coexist, but each archive operation targets exactly one backend. |

### Ambiguity Resolution

If capability negotiation produces multiple plugins with equal specificity for a single-selection dispatch (Capture, Restore, Storage), the Plugin Host treats this as a configuration error. The ambiguity is logged at `error` severity, and the dispatch fails. The user must resolve the ambiguity by removing or reconfiguring one of the conflicting plugins.

### Forward Compatibility

Capability negotiation is designed to accommodate future evolution:

- New capability dimensions may be added to the `Capabilities` structure without breaking existing plugins. Plugins that do not declare a new dimension are treated as not supporting it.
- New plugin types may be introduced in future interface versions. The Plugin Host ignores plugin types it does not recognize.
- New discovery categories may be added. Existing plugins that do not declare the new category are simply not dispatched for it.

---

## Sandbox and Permission Model

Plugins operate within a permission sandbox. The sandbox ensures that plugins cannot exceed their declared access scope, protecting the host system and the core from malicious or misbehaving plugins.

### Permission Declaration

Every plugin declares its required permissions in its manifest's `permissions` block. These declarations are the plugin's contract with the runtime environment: the plugin asserts what it needs, and the runtime enforces those boundaries.

| Permission | Description | Default |
|-----------|-------------|---------|
| **Filesystem read** | Path patterns the plugin needs to read from the host filesystem. | Plugin's own directory only. |
| **Filesystem write** | Path patterns the plugin needs to write to the host filesystem. Applicable only to Restore plugins. | None. Only Restore plugins may declare write permissions. |
| **Network** | Whether the plugin requires network access. | Denied. Only Storage plugins targeting remote backends and plugins with explicit justification may declare network access. |
| **Subprocess** | Executable names the plugin may invoke as subprocesses. | None. Only explicitly listed executables may be invoked. |

### Enforcement Principles

1. **Declaration before execution.** The Plugin Host validates all declared permissions during the Validate phase of the plugin lifecycle, before any plugin code executes. Permissions that are inappropriate for the plugin type (e.g., filesystem write for a Discovery plugin) cause validation failure and the plugin is not loaded.

2. **Runtime enforcement.** The runtime environment enforces the declared permissions during plugin execution. The enforcement mechanism is an implementation decision and may vary by platform. This specification does not prescribe the enforcement technology.

3. **Violation handling.** If a plugin attempts to exceed its declared permissions during execution, the attempt is denied, the violation is logged at `audit` severity, and the plugin invocation is treated as a failure. The core continues with remaining plugins.

4. **Least privilege.** Plugins should declare the minimum permissions necessary for their operation. The Plugin Host may reject plugins that declare excessively broad permissions.

### Permission Rules by Plugin Type

| Plugin Type | Filesystem Read | Filesystem Write | Network | Subprocess |
|------------|----------------|-----------------|---------|------------|
| **Discovery** | Declared paths | Not permitted | Not permitted | Declared executables |
| **Classification Rule** | Not permitted | Not permitted | Not permitted | Not permitted |
| **Capture** | Declared paths | Staging location only | Not permitted | Declared executables |
| **Restore** | Declared paths | Declared paths | Not permitted (default) | Declared executables |
| **Storage** | Declared paths | Declared paths | Permitted if declared | Not permitted (default) |

### Cryptographic Material

Plugins NEVER have direct access to cryptographic material. All cryptographic operations (encryption, decryption, hashing, signing, verification) are performed by the Crypto Engine (S9). Plugins receive plaintext artifacts for processing and return plaintext results; the core handles the cryptographic envelope.

### Subsystem Isolation

Plugins cannot call back into core subsystems. They have no mechanism to access the Orchestrator, Crypto Engine, Storage Backend, Configuration Manager, or any other subsystem. Their entire world is the request they receive and the response they return.

---

## Plugin Trust Model

AERS defines three trust levels for plugins. Trust is evaluated during the Trust phase of the plugin lifecycle, before validation or registration occurs. A plugin that does not meet the minimum trust requirements for the configured trust policy is never loaded.

### Trust Levels

| Trust Level | Criteria | Behavior |
|------------|----------|----------|
| **Official** | Plugin is signed with the AERS project signing key. The signature covers both the manifest and the package contents. | Loaded automatically. No user intervention required. |
| **Community** | Plugin is signed with a third-party key that is not the AERS project key. The signature is valid and covers the full package. | Loaded only after explicit user approval. On first encounter, the Plugin Host presents the plugin's identity, author, capabilities, and signing key fingerprint to the user. The user must approve installation before the plugin is registered. Approval is persisted so subsequent startups do not re-prompt. |
| **Unsigned** | Plugin has no digital signature, or the signature is invalid, expired, or does not cover the full package. | **Disabled by default.** The Plugin Host logs the presence of unsigned plugins at `warn` severity but does not load them. The user must explicitly configure an override to permit unsigned plugins. This override is per-plugin, not global, and is logged at `audit` severity when exercised. |

### Trust Policy Configuration

The system's trust policy is configured through the Configuration Manager (S17). The policy defines:

- Whether unsigned plugins are permitted (default: no).
- Per-plugin trust overrides for specific plugin IDs.
- Whether community plugin approvals are persisted across sessions.

### Trust and Security Boundaries

The trust model interacts with the security perimeter defined in the Architecture:

- Official plugins are considered part of the trusted zone.
- Community and unsigned plugins are considered part of the untrusted zone. Even when approved and loaded, they remain subject to full sandbox enforcement.
- Trust level does not affect sandbox permissions. A community plugin cannot declare broader permissions than what is permitted for its plugin type.

---

## Plugin Signature Verification

Every production plugin should carry a digital signature that enables the Plugin Host to verify its integrity and authenticity before loading.

### What Is Verified

| Verification Target | Description |
|---------------------|-------------|
| **Manifest integrity** | The plugin manifest has not been modified since it was signed. |
| **Package integrity** | All files within the plugin package directory match the signed manifest of contents. No files have been added, removed, or modified. |
| **Signature validity** | The digital signature is valid, was produced by a known key, and has not expired. |

### Verification Process

During the Trust phase of the plugin lifecycle:

1. The Plugin Host reads the plugin's `signature` block from its manifest.
2. The Plugin Host verifies the signature against the package contents using the signing key.
3. If the signature is valid and the signing key is recognized as an official key, the plugin is classified as **Official**.
4. If the signature is valid but the signing key is not an official key, the plugin is classified as **Community**.
5. If the signature is absent, invalid, or does not cover the full package, the plugin is classified as **Unsigned**.
6. The trust level is then evaluated against the configured trust policy (see §Plugin Trust Model).

### Verification Failures

A plugin that fails signature verification MUST NOT be loaded. The Plugin Host logs the failure at `audit` severity, including the plugin ID, the nature of the failure (missing signature, invalid signature, tampered contents, expired signature), and the file path. The core continues startup with the remaining plugins.

### Specification Boundaries

This specification defines **what** must be verified, not **how**. The choice of signature algorithm, key format, signature encoding, and verification library are implementation decisions. The specification requires only that the verification provides cryptographic assurance of integrity and authenticity.

---

## Error Handling and Isolation

When a plugin fails, the Plugin Host follows a defined isolation protocol. The protocol ensures that plugin failures are bounded, logged, and reported without affecting the core system or other plugins.

### Failure Modes

| Failure Mode | Plugin Host Response |
|-------------|---------------------|
| **Normal return with `status: success`** | Result forwarded to the calling subsystem. No special handling. |
| **Normal return with `status: partial`** | Partial result forwarded to the calling subsystem. Warnings are logged at `warn` severity. The manifest section is marked as partially complete. |
| **Normal return with `status: failure`** | Error logged at `error` severity. The manifest section (or action) is marked as failed. The core continues with remaining plugins. |
| **Process crash or abnormal termination** | The Plugin Host detects the subprocess termination. Error logged at `error` severity, including the plugin ID and any available diagnostic information. The manifest section is marked as incomplete. The core continues with remaining plugins. |
| **Timeout exceeded** | The Plugin Host terminates the plugin subprocess. Timeout logged at `error` severity, including the plugin ID and the configured timeout value. The manifest section is marked as incomplete. The core continues with remaining plugins. |
| **Malformed response** | The Plugin Host validates every response against the expected response schema for the plugin type. If validation fails, the response is rejected. Validation error logged at `error` severity. The manifest section is marked as incomplete. The core continues with remaining plugins. |
| **Permission violation** | The runtime environment blocks the unauthorized access. Violation logged at `audit` severity. The plugin invocation is treated as a failure. The core continues with remaining plugins. |

### Isolation Guarantees

1. **A plugin failure MUST NEVER crash the core system.** This is Architectural Constraint #6 and NFR-2.4.
2. **A plugin failure MUST NEVER corrupt the Environment Manifest.** If a plugin fails during discovery, the manifest section is marked as incomplete — it is never left in a partially-written state.
3. **A plugin failure MUST NEVER block the pipeline indefinitely.** Timeout enforcement ensures that a hung plugin is terminated within a bounded time.
4. **A plugin failure MUST be visible to the user.** All plugin failures are reported in the operation summary, with the plugin ID, the failure mode, and the affected scope.
5. **A plugin failure in one plugin MUST NOT affect other plugins.** Each plugin runs in its own subprocess. The Plugin Host dispatches remaining plugins regardless of whether a previous plugin failed.

---

## Shared Types

The following types are referenced across multiple plugin type contracts and form the shared vocabulary of the plugin interface. All types are logical definitions — their concrete representation is an implementation decision.

```
PlatformContext
├── os_family: string            — "linux" | "windows" | "darwin"
├── os_id: string                — "ubuntu" | "debian" | "windows" | "fedora" | ...
├── os_version: string           — Distribution or OS version
├── architecture: string         — "amd64" | "arm64"
└── kernel: string               — Kernel or OS build version string

MachineContext
├── role: string?                — Machine role if determined (e.g., "developer-workstation", "web-server")
├── os_family: string
├── os_id: string
└── os_version: string

ArtifactLocation
├── location_type: string        — Describes the kind of location (e.g., "staging-path", "storage-reference")
└── identifier: string           — Location-specific identifier

ArchiveMetadata
├── archive_id: UUID
├── created_at: timestamp
├── source_hostname: string
├── label: string?
├── size_bytes: int
└── os_family: string

PackageReference
├── manager: string              — Package manager name ("apt", "winget", "pip", ...)
├── name: string                 — Package name
├── version: string              — Package version
└── source: string               — Source repository or registry

ArtifactMetadata
├── size_bytes: int              — Size of the artifact in bytes
├── content_hash: string         — Hash of the artifact contents (algorithm specified by core)
└── format: string?              — Optional format identifier for export artifacts

ChangeRecord
├── type: string                 — "file_written", "package_installed", "service_started", etc.
├── path: string?                — Affected filesystem path (if applicable)
└── detail: string               — Human-readable description of the change

RollbackData
├── action_id: string            — The action this rollback data corresponds to
└── instructions: RollbackSpec   — Plugin-defined data needed to reverse the action

Warning
├── code: string                 — Machine-readable warning code
└── message: string              — Human-readable warning message

ErrorDetail
├── code: string                 — Machine-readable error code
└── message: string              — Human-readable error message
```

### Enumerations

```
ResponseStatus: enum
├── success                      — Operation completed fully
├── partial                      — Operation completed with some failures; results are usable
└── failure                      — Operation failed entirely; no usable results

ImportanceLevel: enum
├── critical                     — Must be captured; loss is unrecoverable
├── recommended                  — Should be captured; loss is inconvenient
├── optional                     — May be captured; can be recreated with effort
└── transient                    — Should be skipped; ephemeral or disposable

Reproducibility: enum
├── reproducible                 — Can be reinstalled from a known external source
└── irreplaceable                — Must be backed up verbatim; no external source exists

CaptureMethod: enum
├── reference                    — Store package name + version + source (no binary)
├── copy                         — Include file contents in archive
└── export                       — Serialize via plugin-provided mechanism

RestoreActionType: enum
├── install                      — Install a package via package manager
├── copy                         — Write a file to the target filesystem
├── write                        — Write configuration content to the target
├── execute                      — Run a command on the target
└── start                        — Start a service on the target

Outcome: enum
├── completed                    — Action executed successfully
├── skipped                      — Action was already applied (idempotent detection)
└── failed                       — Action failed

DiscoveryCategory: enum
├── platform
├── packages
├── services
├── user_config
├── credentials
├── environment
├── scheduled_tasks
├── network
├── cloud_metadata
└── user_data

PluginType: enum
├── Discovery
├── ClassificationRule
├── Capture
├── Restore
└── Storage
```

---

## Plugin Dispatch Rules

When a core subsystem needs plugin services, the Plugin Host uses the following dispatch logic:

| Subsystem | Dispatch Key | Selection Rule |
|-----------|-------------|---------------|
| **Discovery Engine (S3)** | `(Discovery, os_family, category)` | **All matching plugins** are invoked. Results are aggregated into the manifest. Multiple plugins may contribute entries to the same category (e.g., `apt` and `snap` both contribute to `packages`). |
| **Classifier (S5)** | `(ClassificationRule, os_family, *)` | **All matching plugins** are invoked per entry. Annotations are merged. Plugin rules sit in the middle tier of the classification hierarchy; the Classifier (S5) enforces ordering. |
| **Capture Engine (S7)** | `(Capture, os_family, action.category)` | **Single best-match plugin.** If multiple plugins match, the most specific is selected (see §Capability Negotiation — Specificity Ranking). |
| **Restore Engine (S10)** | `(Restore, os_family, action.type)` | **Single best-match plugin.** Same specificity rule as Capture. |
| **Storage Backend (S8)** | `(Storage, *, storage_protocol)` | **Single match by storage protocol.** Each storage protocol is handled by exactly one plugin. |

For multi-plugin dispatch (Discovery, Classification), the Plugin Host invokes all matching plugins and returns the aggregated results to the calling subsystem. The calling subsystem is responsible for merging, deduplication, and conflict resolution.

For single-plugin dispatch (Capture, Restore, Storage), the Plugin Host selects exactly one plugin. If no plugin matches, the dispatch fails and the calling subsystem handles the gap (e.g., marking the action as unsupported). If multiple plugins match with equal specificity, this is treated as an ambiguity error (see §Capability Negotiation — Ambiguity Resolution).

---

## Traceability

Every element of this specification is traceable to the foundational documents.

### Traceability to Requirements

| Requirement | Coverage in This Specification |
|------------|-------------------------------|
| FR-12.1 — Plugin discovery and registration | §Plugin Lifecycle (Discover, Validate, Register phases) |
| FR-12.2 — Interface version declaration and enforcement | §Interface Versioning, §Plugin Identity Model (`interface_version` field) |
| FR-12.3 — Capability declaration | §Plugin Identity Model (`capabilities` block), §Capability Negotiation |
| FR-12.4 — Plugin types: Discovery, ClassificationRule, Capture, Restore, Storage | §Plugin Type Contracts (all five types defined) |
| FR-12.5 — Plugins interact through defined interfaces only | §Sandbox and Permission Model (subsystem isolation), §Plugin Execution Model (no callback mechanism) |
| FR-12.6 — Plugin failure isolation | §Error Handling and Isolation (all failure modes and isolation guarantees) |
| FR-12.7 — New support without core modification | §Design Principles (Principle 2), §Capability Negotiation (forward compatibility) |
| FR-12.8 — Core contains zero platform-specific logic | §Design Principles (Principle 1), §Sandbox and Permission Model (subsystem isolation) |
| EIR-2.1 — Clear contract per plugin type | §Plugin Type Contracts (request, response, and behavioral contract for each type) |
| EIR-2.2 — Defined input and output formats | §Plugin Type Contracts (request and response structures), §Shared Types |
| EIR-2.3 — Interface version targeting | §Interface Versioning, §Plugin Identity Model |
| EIR-2.4 — Capability declaration | §Plugin Identity Model (`capabilities` block) |
| EIR-2.5 — Statelessness | §Design Principles (Principle 6), §Plugin Execution Model (lifecycle boundaries) |
| NFR-1.11 — Plugin sandboxing | §Sandbox and Permission Model |
| NFR-2.4 — Plugin failure never crashes core | §Error Handling and Isolation (Isolation Guarantee 1) |
| NFR-5.1 — New distro plugin without core changes | §Capability Negotiation (forward compatibility), §Plugin Packaging (atomic installation) |
| NFR-5.2 — Versioned plugin interface | §Interface Versioning |
| NFR-5.3 — Formal extensibility points | §Plugin Type Contracts (five types = five extensibility points) |

### Traceability to Architecture

| Architecture Element | Coverage in This Specification |
|---------------------|-------------------------------|
| S12 — Plugin Host | §Plugin Lifecycle, §Plugin Dispatch Rules, §Capability Negotiation, §Error Handling and Isolation |
| Plugin Boundary Contract — Principles 1–5 | §Design Principles (mapped 1:1) |
| Plugin Boundary Contract — Interface Summary | §Plugin Type Contracts (expanded with full request/response schemas) |
| Security Perimeter — Plugin sandboxing | §Sandbox and Permission Model, §Plugin Trust Model |
| Architectural Constraint #6 — Plugin failure isolation | §Error Handling and Isolation |
| Extensibility Points table | §Plugin Type Contracts, §Capability Negotiation |

### Traceability to Vision

| Vision Principle | Coverage in This Specification |
|-----------------|-------------------------------|
| Modular Plugin Architecture | Entire document — this specification is the realization of that principle |
| Security Is the Foundation | §Sandbox and Permission Model, §Plugin Trust Model, §Plugin Signature Verification |
| Simplicity Over Cleverness | §Plugin Execution Model (subprocess isolation over complex in-process mechanisms) |
| Transparency Is Non-Negotiable | §Error Handling and Isolation (all failures visible), §Plugin Configuration (source transparency) |

---

> **This document defines the contract between the AERS core and the plugin ecosystem.** Every plugin implementation must conform to the contracts, permissions, and behavioral rules defined here. Every core subsystem that consumes plugins must dispatch through the Plugin Host and honor the isolation guarantees. If a future implementation decision conflicts with this specification, the conflict must be resolved explicitly — either by updating the implementation or by amending this document through formal review.
