# Telos — Project Context

> **Purpose:** Internal onboarding document for AI agents and new contributors
> **Status:** Active
> **Last Updated:** 2026-09-19
> **Last Verified:** 2026-09-19, commit 77c3748
> **Audience:** AI sessions, developers, and code reviewers working on Telos
> **Canonical Sources:** [00_Vision.md](00_Vision.md), [01_Requirements.md](01_Requirements.md), [02_Architecture.md](02_Architecture.md), [03_Threat_Model.md](03_Threat_Model.md), [04_Data_Model.md](04_Data_Model.md), [05_Plugin_API.md](05_Plugin_API.md)

---

## 1. Project Summary & Concept

**Telos** (Ancient Greek: *τέλος* — intrinsic purpose, end goal, and operational intent) is an intelligent, offline-first, plugin-based system for discovering, understanding, and deterministically reconstructing computing environments.

Unlike traditional backup tools that copy files blindly, Telos comprehends an environment before it captures anything: it discovers the operating system, catalogs installed packages, identifies running services, maps user configurations, detects credential material, and constructs an intent-aware backup plan.

The core value proposition is **deterministic environment reconstruction**: given a compatible target machine and a Telos blueprint, the user recreates a functionally identical environment with confidence, speed, and security.

Telos is designed for a single technically sophisticated user who manages a diverse fleet of personal machines across multiple operating systems, virtual machines, and cloud providers.

---

## 2. Architecture at a Glance

Telos is a **pipeline-oriented, plugin-extended, CLI-driven** system with 17 named subsystems.

### Subsystem Map

| ID | Name | Layer | Role |
| ---- | ------ | ------- | ------ |
| S1 | CLI Shell | Surface | User-facing interface; owns the Approval Gate |
| S2 | Orchestrator | Core | Pipeline coordinator; lifecycle owner for all operations |
| S3 | Discovery Engine | Core | Dispatches discovery requests to plugins; aggregates results into a manifest |
| S4 | Environment Manifest | Core | Canonical data model — the single source of truth for a discovered environment |
| S5 | Classifier | Intelligence | Annotates manifest entries with role, importance, intent, reproducibility |
| S6 | Planner | Core | Translates an annotated manifest into a human-reviewable Backup or Restore Plan |
| S7 | Capture Engine | Core | Executes an approved Backup Plan; produces pre-encryption archive |
| S8 | Storage Backend | Core | Reads/writes encrypted archives to durable storage |
| S9 | Crypto Engine | Core | All cryptographic services: encryption (AES-256), hashing (SHA-256), signing (GPG), KDF (Argon2) |
| S10 | Restore Engine | Core | Executes an approved Restore Plan; only subsystem authorized to write to the target |
| S11 | Verification Engine | Core | Post-restore discovery + diff to verify functional equivalence |
| S12 | Plugin Host | Core | Plugin lifecycle: registration, validation, capability dispatch, isolation |
| S13 | AI Advisory Layer | Intelligence | Non-binding probabilistic suggestions; lowest-priority input in every decision |
| S14 | Diff Engine | Core | Structural comparison of two Environment Manifests |
| S15 | Scheduler | Core | Time-based triggers for automated backups |
| S16 | Logging & Audit | Cross-cutting | Structured, tamper-evident logging; never contains sensitive data |
| S17 | Configuration Manager | Cross-cutting | Loads, validates, merges configuration from 5-tier priority hierarchy |

### Key Structural Properties

- **Pipelines are linear and sequential.** No branches, loops, or hidden side channels. If a stage fails, the pipeline halts.
- **The core contains zero OS-specific logic.** All platform knowledge lives in plugins accessed through S12.
- **Single-threaded by design** in the initial architecture. Future parallelism may occur within stages but never across stages.

---

## 3. Core Pipeline

### Backup Pipeline

```
User → CLI Shell → Orchestrator → Discovery Engine → Manifest → Classifier → (AI Advisory) → Planner → Approval Gate → Capture Engine → Crypto Engine → Storage Backend
```

Stages in order: **Discovery → Classification → Planning → Approval → Capture → Hashing → Encryption → Storage**

### Restore Pipeline

```
User → CLI Shell → Orchestrator → Storage Backend → Crypto Engine (verify + decrypt) → Planner → Approval Gate → Restore Engine (6 phases) → Verification Engine
```

Stages in order: **Archive Retrieval → Decryption → Integrity Verification → Manifest Loading → Restore Planning → Approval → Execution → Post-Restore Verification**

### Restore Execution Phases (Canonical Order)

| Phase | Actions |
| ------- | --------- |
| 1. Package Installation | Install packages via OS package managers |
| 2. Configuration Application | Write configuration files, dot files, shell configs |
| 3. Credential Restoration | Restore SSH keys, GPG keys, API tokens (separate decryption) |
| 4. Data Restoration | Copy user data files and directories |
| 5. Service Configuration | Apply service configs; start services |
| 6. Environment & Scheduled Tasks | Set env vars; install cron jobs and timers |

### Other Pipelines

| Pipeline | Stages |
| ---------- | -------- |
| **Verify** | Discovery (current) → Manifest Loading (baseline) → Diff → Report |
| **Diff** | Manifest Loading (A) → Manifest Loading (B) → Diff → Report |
| **Discover** | Discovery → Classification → Report |

---

## 4. Data Entities and Relationships

### Primary Entities

| Entity | Purpose | Defined In |
| -------- | --------- | ------------ |
| **Machine Profile** | Hardware and OS identity of the source machine | 04_Data_Model §3.1 |
| **Environment Manifest** | Complete structured description of a discovered environment; the single source of truth | 04_Data_Model §3.2 |
| **Backup Plan** | What to capture, how, and why — user-reviewable before execution | 04_Data_Model §3.3 |
| **Backup Archive** | Encrypted, integrity-verified container of captured artifacts + metadata | 04_Data_Model §3.4 |
| **Restore Plan** | What to do on the target machine, in what order — user-reviewable before execution | 04_Data_Model §3.5 |
| **Restore Checkpoint** | Resume state for interrupted restores | 04_Data_Model §3.6 |
| **Verification Report** | Post-restore comparison of restored vs. original, with severity classification | 04_Data_Model §3.7 |
| **Diff Report** | Structural comparison of any two manifests | 04_Data_Model §3.8 |
| **Audit Log Entry** | Structured log record; never contains sensitive data | 04_Data_Model §3.9 |

### Critical Data Rule — Facts vs. Judgments

The data model structurally separates **facts** (immutable discovery data) from **judgments** (annotations added by the Classifier and AI Advisory Layer). Once discovery produces a manifest, its fact fields are frozen. Subsequent processing may only add or modify annotation fields. This ensures the raw discovery record is always preserved and auditable.

### Manifest Sections

The Environment Manifest has 10 content sections, each mirroring a discovery category:

`platform` · `packages` · `services` · `user_config` · `credentials` · `environment` · `scheduled_tasks` · `network` · `cloud_metadata` · `user_data`

Plus a `metadata` header with: `schema_version`, `created_at`, `source_hostname`, `telos_version`, `discovery_duration`.

---

## 5. Plugin Architecture

### Design Principles

1. **The core is a host; plugins are the content.** The core contains zero platform-specific logic.
2. **Plugins are stateless.** Each invocation receives a request and returns a response. No retained state between calls.
3. **Plugins are sandboxed.** No direct filesystem access outside declared scope, no network unless authorized, no access to Crypto Engine or other subsystems.
4. **Plugins fail safely.** A plugin crash, timeout, or malformed response never crashes the core. The failure is isolated, logged, and reported as a partial result.
5. **Plugins are versioned.** Each plugin targets a specific interface version. The Plugin Host refuses incompatible versions.
6. **Plugins are pure functions of their inputs.** No global state, no callbacks, no side channels.

### Plugin Types

| Type | Consumed By | Dispatch Rule |
| ------ | ------------ | --------------- |
| **Discovery** | S3 (Discovery Engine) | All matching plugins invoked; results aggregated |
| **ClassificationRule** | S5 (Classifier) | All matching plugins invoked per entry; annotations merged |
| **Capture** | S7 (Capture Engine) | Single best-match plugin by specificity |
| **Restore** | S10 (Restore Engine) | Single best-match plugin by specificity |
| **Storage** | S8 (Storage Backend) | Single match by storage protocol |

### Plugin Identity

Every plugin declares: `name`, `version`, `interface_version`, `plugin_type`, `os_families`, `capabilities`, and `permissions`.

### Plugin Lifecycle

**Discover → Validate → Register → Dispatch → Invoke → Retire**

The Plugin Host executes plugins as isolated subprocesses. Communication uses a versioned protocol with JSON-serialized request/response messages over stdin/stdout.

### Execution & Sandbox Model

Each plugin invocation is a fresh subprocess executing over a versioned JSON-serialized request/response protocol over stdin/stdout with bounded execution deadlines (default 5 minutes).

#### Implemented Today (V1 Linux Sandbox & Host Invariants)
- **Landlock LSM (ABI v1–v7):** Enforces filesystem isolation on Linux. Restricts access to declared `ReadPaths` (read access) and `WritePaths` (write and creation access, with execution permissions strictly masked out). `Executables` receive explicit execute permissions.
- **Seccomp-BPF Syscall Filtering:** Installs a BPF syscall filter denying `clone3` with `ENOSYS`, denying namespace creation variants (`CLONE_NEWUSER`, `CLONE_NEWPID`, `CLONE_NEWNET`, etc.) with `EPERM`, and denying `mount`, `unshare`, and `setns` with `EPERM`.
- **Linux Namespaces:** Creates a private PID namespace (restricting process visibility and enabling clean subtree teardown) and an unshared network namespace with `loopback DOWN` (blocking network access).
- **Privilege Confinement:** Sets `PR_SET_NO_NEW_PRIVS` to prevent privilege escalation via setuid binaries.
- **Resource Limits via `HostOptions` (ADR-0012):** Normalized options pass into `sandbox.Policy`:
  - Memory: 512 MB virtual address space ceiling (`RLIMIT_AS`) default.
  - File size: 10 GB file creation ceiling (`RLIMIT_FSIZE`) default.
  - Task count: Process limits via `RLIMIT_NPROC` default to 0 (disabled in V1; true task bounding deferred to V2 cgroups v2).
- **Offline Network Rejection:** Manifests declaring `permissions.network: true` for `Discovery` and `Capture` plugins are strictly rejected at load time in `host.go:load()`. Restore plugins default to `network: false`, but may request network access where explicitly justified.
- **Policy Slice Isolation:** `buildPolicy()` allocates fresh slices for `ReadPaths` and `Executables`, and clones `WritePaths` via `slices.Clone`, preventing any slice aliasing or concurrent mutation from affecting the host registry.
- **Non-Linux Fallback:** On Windows and macOS, executions fall back to standard subprocess invocation without kernel sandboxing, with application-level timeouts and audit log warnings.

#### Planned / Not Yet Implemented
- **Subprocess Authorization (ADR-0007):** Will validate manifest `subprocess` names (rejecting slashes), resolve host paths via `exec.LookPath`, append resolved binaries to `Policy.Executables`, and automatically grant Landlock read+execute permissions to system library paths (`/lib`, `/lib64`, `/usr/lib`, `/usr/lib64`, `/etc/alternatives`).
- **Capture Staging Lifecycle (ADR-0013):** Core will allocate cryptographically random temporary directories under `/var/lib/telos/staging/capture-<uuid>` (mode 0700), pass `staging_location` in `CaptureRequest`, append staging paths to `Policy.WritePaths` and `Policy.ReadPaths`, validate that returned artifacts are strictly relative subpaths within staging, and ensure unconditional staging cleanup.
- **Bounded Stderr on Direct Path:** The direct `Invoke()` path currently uses an unbounded `bytes.Buffer` (which will be replaced with a unified `limitedBuffer`), and a single authoritative stderr limit across host and sandbox will be established.
- **RLIMIT_AS Amendment:** ADR-0012 will be formally amended to account for Go 64-bit runtime virtual address space reservation requirements (investigation pending; earlier measurements were inconclusive) before adjusting the default memory limit in code, unblocking compiled Go plugins and the APT plugin.

---

## 6. Security Model

### Foundational Principles

1. **Zero Trust** — every storage medium, network path, and intermediate system is assumed compromised. Archives are encrypted before leaving the source machine.
2. **Credential Isolation** — sensitive materials (SSH keys, GPG keys, API tokens) are encrypted with separate cryptographic material and stored in a structurally isolated archive segment.
3. **Least Privilege** — discovery is read-only; write access is requested only during restore, only for specific paths.
4. **No persistent key storage** — encryption keys and passphrases are received at operation time and discarded when the operation completes. Never transmitted over any network.

### Encryption Architecture

| Layer | Algorithm | Purpose |
| ------- | ----------- | --------- |
| Artifact hashing | SHA-256 | Per-artifact integrity before encryption |
| Archive encryption | AES-256 | Entire archive encrypted before storage |
| Credential segment | AES-256 (separate derived key) | Double-encryption for sensitive materials |
| Key derivation | Argon2 | Derive encryption keys from user passphrase |
| Signing (optional) | GPG | Tamper evidence and non-repudiation |

### Trust Boundary

Plaintext backup data **never** crosses the trust boundary. The Crypto Engine is the sole subsystem that handles raw keys — other subsystems request cryptographic operations but never receive key material.

### Integrity Verification Chain (Restore)

1. Verify GPG signature (if signed)
2. Decrypt archive
3. Verify archive-level hash
4. Verify per-artifact hashes
5. If any step fails → **HALT**. No silent restoration of corrupted data.

---

## 7. Threat Landscape

Defined in [03_Threat_Model.md](03_Threat_Model.md). Key threat categories:

| Category | Examples |
| ---------- | ---------- |
| **Storage compromise** | Unauthorized read/modification of backup archives on disk, USB, or cloud |
| **Plugin threats** | Malicious/compromised plugins attempting data exfiltration, privilege escalation, or path traversal |
| **Credential exposure** | Credential material leaked via logs, error messages, or co-mingling with general data |
| **Restore-time attacks** | Tampered archives injecting malicious configs or packages during restore |
| **Supply chain** | Compromised package repositories delivering malicious packages during restore |
| **Insider threat** | Authorized user or process with access to storage attempting unauthorized decryption |

### STRIDE Coverage

The Threat Model applies STRIDE analysis across all trust boundaries:

- **Spoofing** → GPG signing, archive authentication
- **Tampering** → Integrity hash chains, verification before restore
- **Repudiation** → Audit logs with `audit`-severity for security events
- **Information Disclosure** → Encryption-at-rest, credential isolation, log sanitization
- **Denial of Service** → Plugin timeouts, resource limits, graceful degradation
- **Elevation of Privilege** → Plugin sandboxing, least-privilege execution

---

## 8. Invariants Every Change Must Preserve

These are **non-negotiable** architectural constraints. Any change that violates these must be rejected or the constraint must be formally amended:

1. **The core contains zero OS-specific logic.** All platform knowledge lives in plugins.
2. **No backup data is stored in plaintext outside the source machine.** Including local staging directories.
3. **Every destructive operation passes through the Approval Gate.** Human authority over automation.
4. **The system is fully functional with AI disabled.** AI is advisory, not foundational.
5. **Pipelines are linear and sequential.** No hidden side effects or implicit event loops.
6. **Plugin failures are isolated and never crash the core.** Plugin Host catches all failures.
7. **The Environment Manifest's discovery data (facts) is immutable after creation.** Only annotations may change.
8. **Credentials receive a higher tier of protection than general data.** Separate encryption, separate archive segment.
9. **All operations function fully offline.** Network access only for package downloads during restore and opt-in cloud AI.
10. **Log entries never contain sensitive data.** No passphrases, private keys, API tokens, or file contents.

---

## 9. AI Boundaries

### Trust Hierarchy

```
Human Decision  >  Deterministic Logic  >  AI Suggestion
```

AI is the **lowest-priority input** in every decision chain.

### What AI Can Do

- Suggest machine role labels (e.g., "web server")
- Suggest importance adjustments for manifest entries
- Flag anomalies during diff operations
- Predict restore conflicts

### What AI Cannot Do

- Execute any action, destructive or otherwise
- Override deterministic rules or user decisions
- Access raw file contents (operates on manifest metadata only)
- Transmit backup contents or credentials (even in cloud mode — anonymized structural metadata only)

### Operational Modes

| Mode | Network | Behavior |
| ------ | --------- | ---------- |
| **Disabled** | None | AI layer not loaded; deterministic-only codepaths |
| **Local** | None | On-device inference |
| **Cloud** (opt-in) | Required | API calls with anonymized metadata only |

### Non-Negotiable: The system works without AI

If all AI components are removed, Telos must remain a fully capable backup and restore tool.

---

## 10. V1 Scope Boundaries

### Included in Version 1

- Full backup pipeline: discover → classify → plan → approve → capture → encrypt → store
- Restore to compatible machines (same OS family and CPU architecture)
- Target machine assumed to have a freshly installed OS
- Post-restore verification and environment diffing
- CLI interface (sole UI)
- Plugin architecture with versioned interface
- Plugins for Ubuntu, Debian, and Windows
- Encrypted, signed archives with credential isolation
- Single master passphrase with internal key separation
- Optional AI advisory features (local by default)
- Scheduled backup automation
- Offline-first operation

### Excluded from Version 1

| Capability | Status |
| ----------- | -------- |
| Differential/incremental backups | Future version |
| Cross-platform restoration | Future version |
| Graphical user interface | Future version |
| Multi-user/multi-tenant | Future version |
| Full disk imaging | Out of scope |
| Real-time synchronization | Out of scope |
| Bare-metal OS provisioning | Out of scope |
| Mobile device backup | Out of scope |

---

## 11. Coding Conventions

### Document-Driven Development

Every implementation decision must be traceable to the foundational documents. Before writing code:

1. Verify the feature is specified in [01_Requirements.md](01_Requirements.md).
2. Confirm the responsible subsystem in [02_Architecture.md](02_Architecture.md).
3. If the feature involves plugins, follow the contracts in [05_Plugin_API.md](05_Plugin_API.md).
4. If the feature handles sensitive data, verify compliance with [03_Threat_Model.md](03_Threat_Model.md).

### Subsystem Boundaries Are Law

- **Never** put OS-specific logic in core subsystems. Use plugins.
- **Never** have subsystems bypass the Orchestrator to call each other directly outside the defined dependency graph.
- **Never** have the CLI Shell contain business logic. It is a translation layer.
- **Never** have the Capture Engine decide what to capture. It follows the approved plan.
- **Never** have the Restore Engine execute actions not in the approved plan.
- **Never** have any subsystem other than S9 (Crypto Engine) perform cryptographic operations.

### Plugin Development Rules

- Plugins are stateless: receive a request, return a response.
- Plugins communicate via JSON over stdin/stdout as isolated subprocesses.
- Plugins declare all capabilities and required permissions at registration.
- Plugins must handle their own errors and return structured error responses.
- Plugins must never access the filesystem outside their declared scope.
- Plugins must target a specific interface version.

### Configuration Hierarchy

Priority (highest to lowest):

1. CLI flags (per-invocation)
2. Environment variables (`TELOS_*`, session-level)
3. User config file (`~/.telos/config.yaml`)
4. System config file (`/etc/telos/config.yaml`)
5. Built-in defaults

### Logging Rules

- Use structured entries: `timestamp`, `subsystem`, `operation`, `severity`, `message`, `context`.
- Severity levels: `debug`, `info`, `warn`, `error`, `audit`.
- `audit` is reserved for security-critical events: encryption, decryption, approval decisions, credential access, archive verification.
- **Never** log passphrases, private keys, API tokens, or file contents. Log sensitive file paths by basename only.

---

## 12. What Not To Do

| Don't | Why | Reference |
| ------- | ----- | ----------- |
| Put OS logic in core | Violates the plugin architecture constraint | Architecture §Architectural Constraints #1 |
| Store plaintext backups anywhere | Violates Zero Trust security model | Architecture §Architectural Constraints #2, Threat Model |
| Skip the Approval Gate | Violates human authority principle | Architecture §Architectural Constraints #3, Vision §Guiding Principles #1 |
| Make AI a hard dependency | AI is advisory-only; system must work without it | Architecture §Architectural Constraints #4, Vision §AI Philosophy |
| Add branching/loops to pipelines | Pipelines are linear; side effects are design defects | Architecture §Architectural Constraints #5 |
| Let plugin failures crash core | Plugin isolation is a foundational guarantee | Architecture §Architectural Constraints #6 |
| Mutate manifest discovery data | Facts are immutable; only annotations may be added | Architecture §Architectural Constraints #7, Data Model §Entity Lifecycle |
| Co-mingle credentials with general data | Credential isolation requires structural separation and separate encryption | Architecture §Architectural Constraints #8, Threat Model |
| Require network for core operations | Offline-first is a design constraint, not a preference | Architecture §Architectural Constraints #9, Vision §Guiding Principles #7 |
| Log sensitive data | Logs never contain keys, tokens, passwords, or file contents | Architecture §Architectural Constraints #10, Threat Model |
| Have plugins access Crypto Engine | Plugins are sandboxed; cryptographic material is inaccessible to them | Plugin API §Sandbox and Permission Model |
| Substitute packages without approval | Restore Engine reports failures; it does not guess alternatives | Architecture §S10 Boundaries |

---

## 13. Document Map

| Document | What It Defines | Read When |
| ---------- | ---------------- | ----------- |
| [00_Vision.md](00_Vision.md) | Project purpose, concept & symbolism (*Telos*), design philosophy, guiding principles, AI philosophy, scope, non-goals | Understanding why a design decision exists |
| [01_Requirements.md](01_Requirements.md) | Functional requirements (FR-1 through FR-14), non-functional requirements (NFR-1 through NFR-8), data requirements, external interface requirements, V1 scope | Verifying what the system must accomplish |
| [02_Architecture.md](02_Architecture.md) | 17 subsystem definitions, dependency graph, pipeline definitions, data flows, plugin boundary contract, security perimeter, failure model, architectural constraints | Understanding how the system is structured |
| [03_Threat_Model.md](03_Threat_Model.md) | Threat landscape, STRIDE analysis, trust boundaries, attack vectors, mitigations | Implementing security-sensitive features |
| [04_Data_Model.md](04_Data_Model.md) | Entity definitions, field schemas, relationships, lifecycle rules, serialization formats, validation rules | Working with data structures and schemas |
| [05_Plugin_API.md](05_Plugin_API.md) | Plugin types, identity model, lifecycle, execution model, sandbox/permissions, request/response contracts, dispatch rules, shared types, error handling | Building or modifying plugins |
| [06_Coding_Standards.md](06_Coding_Standards.md) | Coding standards and conventions *(empty stub)* | Contributing new Go source code |
| [07_Roadmap.md](07_Roadmap.md) | Project implementation timeline and phases *(empty stub)* | Reviewing milestone planning |
| [99_Project_Status.md](99_Project_Status.md) | Active engineering status dashboard, test evidence, verified subsystem states, open issues, and next steps | Understanding current implementation state |
| [reports/](reports/) | Verification, remediation, and audit reports (V1 sandbox verification, first-party plugin gap audits) | Reviewing empirical security findings and audit history |
| [ADRs/](ADRs/) | Architecture Decision Records (see index below) | Reviewing binding architectural decisions and rationale |
| **08_PROJECT_CONTEXT.md** (this file) | Synthesized onboarding reference; does not replace any source document | Starting work on Telos for the first time |

### Architecture Decision Records (ADRs)

| ADR | Title | Status |
| --- | ----- | ------ |
| [ADR-0001](ADRs/ADR-0001-Offline-First.md) | Offline-First | ❌ Empty stub |
| [ADR-0002](ADRs/ADR-0002-Plugin-Architecture.md) | Plugin Architecture | ❌ Empty stub |
| [ADR-0003](ADRs/ADR-0003-Fresh-OS-Restore.md) | Fresh-OS Restore | ❌ Empty stub |
| [ADR-0004](ADRs/ADR-0004-Single-Passphrase.md) | Single Passphrase | ❌ Empty stub |
| [ADR-0005](ADRs/ADR-0005-Compatible-Machine-Policy.md) | Compatible Machine Policy | ❌ Empty stub |
| [ADR-0006](ADRs/ADR-0006-Directory-Based-Plugins.md) | Directory-Based Plugins | ❌ Empty stub |
| [ADR-0007](ADRs/ADR-0007-Subprocess-Plugin-Execution.md) | Subprocess Plugin Execution | ✅ Populated / Accepted |
| [ADR-0008](ADRs/ADR-0008-Full-Backup-V1.md) | Full Backup V1 | ❌ Empty stub |
| [ADR-0009](ADRs/ADR-0009-Configuration-Manager.md) | Configuration Manager | ✅ Populated / Accepted |
| [ADR-0010](ADRs/ADR-0010-Crypto-Envelope.md) | Crypto Envelope | ✅ Populated / Accepted |
| [ADR-0011](ADRs/ADR-0011-Plugin-Execution-Isolation.md) | Plugin Execution Isolation | ✅ Populated / Accepted |
| [ADR-0012](ADRs/ADR-0012-Plugin-Resource-Limits.md) | Plugin Resource Limits | ✅ Populated / Accepted |
| [ADR-0013](ADRs/ADR-0013-Capture-Staging-Lifecycle.md) | Capture Staging Lifecycle | ✅ Populated / Accepted |

---

> **This document is a navigational aid, not a source of truth.** If any statement in this document conflicts with the canonical documents listed in the Document Map, the canonical document takes precedence. When in doubt, read the original.
