# Telos — Project Status

> **Status:** Active
> **Last Updated:** 2026-08-29 (Repository Audit)
> **Author:** AI Engineering Review (Antigravity)
> **Audience:** Project maintainers, future contributors, AI sessions
> **Purpose:** Engineering dashboard — understand exactly where the project stands

---

## Table of Contents

1. [Overall Progress](#1-overall-progress)
2. [Milestone Assessment](#2-milestone-assessment)
3. [Documentation Status](#3-documentation-status)
4. [Subsystem Status](#4-subsystem-status)
5. [ADR Status](#5-adr-status)
6. [Testing Status](#6-testing-status)
7. [Dependency Inventory](#7-dependency-inventory)
8. [Technical Debt](#8-technical-debt)
9. [Known Risks](#9-known-risks)
10. [What Works Today](#10-what-works-today)
11. [Recommended Next Steps](#11-recommended-next-steps)

---

## 1. Overall Progress

**Phase:** Foundation (Phase 1 of 4)
**Completion Estimate:** ~10% of total V1 scope

The Telos project has completed **extensive, production-grade documentation**, implemented the foundational **S4 domain vocabulary** and **S9 cryptographic primitives**, and now has a functional **S12 Plugin Host skeleton** alongside **S17 Configuration Manager** and **S16 Logging & Audit** foundations. Pipeline orchestration, archive creation, and the functional CLI remain unimplemented.

### What exists

| Category | State |
|----------|-------|
| **Design documentation** | Comprehensive. Six canonical documents (~315,000 bytes) covering vision, requirements, architecture, threat model, data model, and plugin API. One onboarding context document. Eleven ADR files (three populated: ADR-0009 Configuration Manager, ADR-0010 Crypto Envelope, ADR-0011 Plugin Execution Isolation). |
| **Configuration Manager (S17)** | Fully implemented: 5-tier merge, schema validation, business validation, source attribution, deep-copy immutability, plugin config passthrough, bulk error reporting. Tested. |
| **Logging & Audit (S16)** | Partially implemented: zerolog wrapper with structured JSON output, level mapping, component tagging, correlation ID context, and a custom `audit` severity. Tested. |
| **Domain model (S4 foundation)** | Implemented in `internal/model`: behavior-free, platform-neutral types for the manifest and its sections, plans/actions, archive and integrity records, plugin metadata, approvals, restore results, and verification reports. Tested for representative manifest and archive construction. |
| **Crypto Engine (S9 foundation)** | Implemented in `internal/crypto`: Argon2id key derivation, AES-256-GCM authenticated envelopes, general/credential key-purpose separation, SHA-256 hashing, and malformed-envelope work-factor bounds. Tested. GPG signing, archive-container serialization, and subsystem integration remain unimplemented. |
| **Plugin Host (S12 skeleton + V1 sandbox)** | Implemented in `internal/plugin`: directory scanning, strict manifest parsing, interface/type/permission validation, duplicate-ID isolation, stable registration snapshots, bounded timed JSON subprocess invocation, and V1 OS-level plugin isolation on Linux (Landlock + seccomp + namespaces, NO_NEW_PRIVS, and resource limits). Trust verification (signature checking, trusted key handling, TOFU, and package integrity) is documented but **NOT implemented**. The `Signature` field is parsed but ignored. Capability-index dispatch and non-Linux sandbox backends also remain unimplemented. |
| **CLI entry point** | Skeleton `cmd/telos/main.go` — creates a logger and invokes a placeholder subsystem. Not functional. |
| **Everything else** | Empty directories or does not exist. |

### What does NOT exist

- No CLI command parsing (no `cobra`, `urfave/cli`, or equivalent).
- No Orchestrator (S2).
- No Discovery Engine (S3), Manifest (S4), Classifier (S5), or Planner (S6).
- No Capture Engine (S7), Storage Backend (S8), or Crypto Engine (S9).
- No Restore Engine (S10), Verification Engine (S11), or Diff Engine (S14).
- No AI Advisory Layer (S13); Plugin Host (S12) skeleton is implemented but not yet integrated with Discovery.
- No Scheduler (S15).
- No plugin implementations of any type.
- No archive format definition or implementation.
- No integration tests, end-to-end tests, or benchmarks.

---

## 2. Milestone Assessment

The roadmap defined in `08_PROJECT_CONTEXT.md` §10 outlines four phases. Below is the current state against each.

### Phase 1 — Foundation

| Milestone | Status | Notes |
|-----------|--------|-------|
| Configuration Manager (S17) — full implementation | ✅ Done | 10 source files, 1 ADR, 6 unit test cases. |
| Logging & Audit (S16) — core implementation | 🟡 Partial | Logger wrapper exists; audit-level logging works. Missing: file output, log rotation, append-only guarantees, audit trail format. |
| Project scaffolding (module, directories) | ✅ Done | `go.mod`, package layout, empty placeholder dirs. |
| CLI Shell (S1) — command framework | ❌ Not started | `main.go` exists but has no argument parsing. |
| Orchestrator (S2) — pipeline skeleton | ❌ Not started | |
| Data model Go types | ✅ Foundation implemented | `internal/model` now contains the behavior-free shared vocabulary. Manifest construction, sealing, persistence, and workflow logic remain unimplemented. |

### Phase 2 — Core Pipeline

| Milestone | Status |
|-----------|--------|
| Discovery Engine (S3) | ❌ Not started |
| Manifest (S4) | ❌ Not started |
| Classifier (S5) | ❌ Not started |
| Planner (S6) | ❌ Not started |
| Capture Engine (S7) | ❌ Not started |
| Crypto Engine (S9) | ❌ Not started |
| Storage Backend (S8) | ❌ Not started |
| Plugin Host (S12) | 🟡 Skeleton + V1 sandbox | Directory scanning, manifest validation, registration, bounded subprocess protocol, and OS-level plugin isolation (Linux: Landlock + seccomp + namespaces) are implemented. Trust verification, capability-index dispatch, and non-Linux sandbox backends remain. |
| Restore Engine (S10) | ❌ Not started |
| Verification Engine (S11) | ❌ Not started |

### Phase 3 — Intelligence & Automation

| Milestone | Status |
|-----------|--------|
| AI Advisory Layer (S13) | ❌ Not started |
| Diff Engine (S14) | ❌ Not started |
| Scheduler (S15) | ❌ Not started |

### Phase 4 — Community & Ecosystem

| Milestone | Status |
|-----------|--------|
| Plugin SDK & developer docs | ❌ Not started |
| Plugin registry | ❌ Not started |
| Community plugin support | ❌ Not started |

---

## 3. Documentation Status

### Canonical Documents

| Document | File | Size | Status | Quality |
|----------|------|------|--------|---------|
| Vision | `00_Vision.md` | 26,749 B | ✅ Complete | Excellent. Establishes all principles and philosophy. |
| Requirements | `01_Requirements.md` | 50,480 B | ✅ Complete | Excellent. FR-1 through FR-14, NFR-1 through NFR-8, DR, EIR. |
| Architecture | `02_Architecture.md` | 60,399 B | ✅ Complete | Excellent. All 17 subsystems defined with boundaries, dependencies, data flows, mermaid diagrams. |
| Threat Model | `03_Threat_Model.md` | 55,447 B | ✅ Complete | Excellent. Full STRIDE analysis, 10 attack surfaces, 7 threat actors, residual risk mapping. |
| Data Model | `04_Data_Model.md` | 62,861 B | ✅ Complete | Excellent. All entities, relationships, lifecycles, identity strategy. |
| Plugin API | `05_Plugin_API.md` | 57,300 B | ✅ Complete | Excellent. Five plugin types, execution model, sandbox, trust model, dispatch rules. |
| Coding Standards | `06_Coding_Standards.md` | 2 B | ❌ Empty | Stub file only. |
| Roadmap | `07_Roadmap.md` | 2 B | ❌ Empty | Stub file only. |
| Project Context | `08_PROJECT_CONTEXT.md` | 22,157 B | ✅ Complete | Good. Navigational onboarding reference. |

### Assessment

The six core design documents are exceptionally thorough — collectively ~313 KB of specification before a single line of pipeline code exists. This is a significant strength: the architecture, security model, and data model are well-defined and internally consistent.

**Gaps:**
- `06_Coding_Standards.md` is empty — no Go style guide, naming conventions, or error handling patterns documented.
- `07_Roadmap.md` is empty — no formal timeline or phasing beyond what's described in `08_PROJECT_CONTEXT.md` §10.
- The docs do not have a `README.md` (the root `README.md` serves this purpose but is separate from the `docs/` folder).

**Note:** The `updates/` directory contains working artifacts from prior AI-assisted development sessions (architecture reviews, audit reports, implementation plans). These are not canonical documents.

---

## 4. Subsystem Status

### Implemented

| ID | Subsystem | Package | Files | Lines | Tests | Test Coverage |
|----|-----------|---------|-------|-------|-------|--------------|
| **S17** | Configuration Manager | `internal/config` | 10 | ~558 | 6 test cases in `loader_test.go` (260 lines) | Moderate — covers merge, strict decoding, plugin keys, validation accumulation, immutability, nested merge. Missing: env_mapper tests, business_validation edge cases, error type tests. |
| **S12** | Plugin Host | `internal/plugin` | source + sandbox + test | — | 4+ | Core registration, subprocess protocol. Linux seccomp structural tests pass. Runtime Linux integration tests exist but are skipped in unprivileged CI environments. Trust verification is completely unimplemented. |
| **S16** | Logging & Audit | `internal/logger` | 1 | 120 | 5 test cases in `log_test.go` (123 lines) | Moderate — covers level creation, component tagging, context correlation, log levels, audit severity, JSON output. Missing: nil writer edge case, concurrent usage. |

### Placeholder / Skeleton

| ID | Subsystem | Package | State |
|----|-----------|---------|-------|
| — | SomeSubsystem (demo) | `core/` | 23-line placeholder. Demonstrates logger injection pattern. Not an actual subsystem from the architecture. |

### Not Started

| ID | Subsystem | Planned Package | Blocked By |
|----|-----------|----------------|------------|
| S1 | CLI Shell | `cmd/telos` or `internal/cli` | S2 (Orchestrator) |
| S2 | Orchestrator | `internal/core` | S3, S6, S7, S9, S10, S16, S17 (leaf deps done) |
| S3 | Discovery Engine | `internal/core` | S12 (Plugin Host) |
| S4 | Environment Manifest | `internal/model` | Domain vocabulary implemented; manifest construction, sealing, and persistence remain to be implemented. |
| S5 | Classifier | `internal/core` | S4 (Manifest) |
| S6 | Planner | `internal/core` | S4, S5 |
| S7 | Capture Engine | `internal/core` | S6, S9 |
| S8 | Storage Backend | `internal/storage` | S9 |
| S9 | Crypto Engine | `internal/crypto` | Cryptographic primitives and authenticated envelope implemented. GPG signing and integration with capture/storage/restore remain. |
| S10 | Restore Engine | `internal/core` | S2, S9 |
| S11 | Verification Engine | `internal/core` | S3, S14 |
| S12 | Plugin Host | `internal/plugin` | S17 (done), data model types | 
| S13 | AI Advisory Layer | `internal/core` | S4, S5 |
| S14 | Diff Engine | `internal/core` | S4 |
| S15 | Scheduler | `internal/core` | S2 |

---

## 5. ADR Status

| ADR | Title | File | Status |
|-----|-------|------|--------|
| ADR-0001 | Offline-First | `ADRs/ADR-0001-Offline-First.md` | ❌ **Empty** — decision recorded in Vision but not formally documented as ADR |
| ADR-0002 | Plugin Architecture | `ADRs/ADR-0002-Plugin-Architecture.md` | ❌ **Empty** — decision recorded in Architecture but not formally documented as ADR |
| ADR-0003 | Fresh-OS Restore | `ADRs/ADR-0003-Fresh-OS-Restore.md` | ❌ **Empty** |
| ADR-0004 | Single Passphrase | `ADRs/ADR-0004-Single-Passphrase.md` | ❌ **Empty** |
| ADR-0005 | Compatible Machine Policy | `ADRs/ADR-0005-Compatible-Machine-Policy.md` | ❌ **Empty** |
| ADR-0006 | Directory-Based Plugins | `ADRs/ADR-0006-Directory-Based-Plugins.md` | ❌ **Empty** |
| ADR-0007 | Subprocess Plugin Execution | `ADRs/ADR-0007-Subprocess-Plugin-Execution.md` | ❌ **Empty** |
| ADR-0008 | Full Backup V1 | `ADRs/ADR-0008-Full-Backup-V1.md` | ❌ **Empty** |
| ADR-0009 | Configuration Manager | `ADRs/ADR-0009-Configuration-Manager.md` | ✅ **Complete** — 91 lines, well-structured with Context, Decision, Consequences, Alternatives Considered, Implementation Notes |
| ADR-0010 | Crypto Envelope | `ADRs/ADR-0010-Crypto-Envelope.md` | ✅ **Complete** — 47 lines, covers Argon2id + AES-256-GCM envelope design, key-purpose separation, and KDF parameter bounds |
| ADR-0011 | Plugin Execution Isolation | `ADRs/ADR-0011-Plugin-Execution-Isolation.md` | ✅ **Complete** — V1 Linux-first plugin isolation strategy: Landlock + seccomp + namespaces, supervisor/re-exec architecture, fail-closed Landlock baseline, seccomp defense-in-depth framing |

### Assessment

8 of 11 ADR files are empty stubs. The decisions they represent *are* documented in the canonical design documents, but the ADR format (Context → Decision → Consequences → Alternatives Considered) is not captured. ADR-0009, ADR-0010, and ADR-0011 are the models to follow for backfilling the others.

---

## 5.5. Security & Workstream A Status

### Workstream A (V1 Sandbox Remediation)
The V1 Linux sandbox is **implemented in code**, providing OS-level isolation via:
- Landlock (filesystem rules)
- Seccomp-BPF (denylist for `mount`, `ptrace`, `clone3`, `unshare`, etc.)
- Namespaces (PID and Network)
- Resource limits (prlimit for memory, processes, file size)
- `PR_SET_NO_NEW_PRIVS` enforcement
- Output limits and timeouts

**Verification state:** 
Linux seccomp rules are statically verified by tests (`seccomp_linux_test.go`). However, actual runtime sandbox enforcement (`sandbox_integration_test.go`) is currently **skipped in CI** because the GitHub Actions runner lacks the necessary capabilities (`EPERM` when trying to use `CLONE_NEWPID` and `CLONE_NEWNET`). Cross-platform fallbacks are not implemented.

### Trust Phase
Trust verification is **NOT implemented**. Although the documentation describes signature verification, trusted key handling, TOFU (Trust On First Use), official/community plugin verification, and package integrity verification, none of this exists in the codebase. The `Signature` field is parsed from the manifest but no verification occurs.

---

## 6. Testing Status

### Unit Tests

| Package | Test File | Test Count | Passing | Notes |
|---------|-----------|------------|---------|-------|
| `internal/config` | `loader_test.go` | 6 | ✅ Verified passing | Tests: full merge pipeline, strict decoding rejection, plugin key acceptance, validation accumulation, profile immutability, nested merge. |
| `internal/logger` | `log_test.go` | 6 | ✅ Verified passing | Tests: level creation, component tagging, context correlation, log levels, audit severity, JSON output. |
| `internal/model` | `entities_test.go` | 2 | ✅ Verified passing | Tests: manifest discovery section retention, archive identity and storage field retention. |
| `internal/plugin` | `host_test.go`, sandbox tests | 4+ | ✅ 2 pass, 2 skip | Tests: discover registers valid and skips invalid. Linux seccomp structural tests verify BPF programs pass. Runtime integration test is skipped in CI (EPERM on CLONE_NEWPID/NEWNET). Windows invoke tests skipped. |

### Integration Tests

None.

### End-to-End Tests

None.

### Benchmarks

None.

### Test Infrastructure

- No test helpers, fixtures, or shared test utilities.
- CI/CD pipeline configured: `.github/workflows/ci.yml` with test (ubuntu + windows, race detector on Linux), lint (golangci-lint v2), and build jobs.
- **Note:** Linux runtime sandbox integration tests are currently skipped in CI due to missing unprivileged user namespace capabilities (`EPERM` on `CLONE_NEWPID` / `CLONE_NEWNET`).
- No test coverage reporting.
- Linting configured: `.golangci.yml` with gosec, govet, staticcheck, revive, gocritic, and 8 additional linters.

### Missing Test Coverage

| Area | What's Missing |
|------|---------------|
| `internal/config/env_mapper.go` | No dedicated unit tests for environment variable resolution |
| `internal/config/business_validation.go` | No tests for empty plugin dir warning or auto-approve warning |
| `internal/config/errors.go` | No tests for `ValidationError.Error()`, `ConfigError.Error()`, `HasErrors()` |
| `internal/config/sources.go` | No tests for `Source.String()` including the unknown case |
| `internal/logger/log.go` | No tests for `nil` writer fallback, or `AuditLevel` in `New()` |

---

## 7. Dependency Inventory

### Direct Dependencies

| Module | Version | Purpose | Used By |
|--------|---------|---------|---------|
| `github.com/rs/zerolog` | v1.35.1 | Structured JSON logging | `internal/logger` |
| `golang.org/x/crypto` | v0.55.0 | Argon2id key derivation | `internal/crypto` |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML parsing with strict mode | `internal/config` |

### Indirect Dependencies

| Module | Version | Source |
|--------|---------|--------|
| `github.com/mattn/go-colorable` | v0.1.14 | zerolog transitive |
| `github.com/mattn/go-isatty` | v0.0.20 | zerolog transitive |
| `golang.org/x/sys` | v0.47.0 | x/crypto + zerolog transitive |

### Assessment

The dependency footprint is minimal and intentional — three direct dependencies, all well-maintained and widely used. This aligns with the offline-first, minimal-dependency philosophy.

**Future dependencies to anticipate:**
- CLI: a command framework (`cobra`, `urfave/cli`, or custom).
- Archive format: `archive/tar`, `compress/gzip` (stdlib), or a custom format.
- Plugin execution: `os/exec` (stdlib).
- GPG signing: `golang.org/x/crypto/openpgp` or a Go GPG library.

---

## 8. Technical Debt

### Current Debt Items

| ID | Severity | Location | Issue | Resolution |
|----|----------|----------|-------|------------|
| **TD-1** | Medium | `cmd/telos/main.go` | Placeholder `SomeSubsystem` in `core/` package. Not an architectural subsystem. Mixes demo code with production layout. | Remove `core/some_subsystem.go` and refactor `main.go` when CLI Shell (S1) is implemented. |
| **TD-2** | Low | `internal/config/env_mapper.go` | Only supports 2 levels of nesting for env vars (e.g., `TELOS_LOGGING_LEVEL`). Cannot map deeply nested plugin config from env vars. | Acceptable for V1; document the limitation. |
| **TD-3** | Low | `internal/logger/log.go:89` | `context.Value("correlation_id")` uses a bare string key instead of an unexported key type. Violates Go context best practices; risks collisions. | Define a private context key type: `type contextKey struct{}; var correlationIDKey = contextKey{}`. |
| **TD-4** | Low | `internal/logger/log.go:60` | `AuditLevel` maps to `zerolog.InfoLevel`. Audit logs could be filtered out if the logger is set to `WarnLevel` or higher. | Audit should use `zerolog.Log()` (level-less) and always emit, which is actually what `Audit()` does at line 118. The `New()` level mapping on line 60 is misleading but not a bug since `Audit()` uses `.Log()` not `.Info()`. Consider removing the case or adding a comment. |
| **TD-5** | Medium | `internal/config/defaults.go` | Default paths are Linux-only (`/var/lib/telos`, `/var/log/telos`). Windows support would require platform-aware defaults. | Address when adding cross-platform support. Use `os.UserConfigDir()` and `os.UserCacheDir()`. |
| **TD-6** | Low | `internal/logger/log.go:71-73` | Console writer is activated for any `*os.File` writer, including file outputs. Structured JSON would be more appropriate for file outputs. | Add an explicit `UseConsole bool` option to the logger constructor. |
| **TD-7** | Info | ADRs 0001–0008 | Empty stub files. Design decisions exist in canonical docs but not in ADR format. | Backfill ADRs from canonical docs when time permits. ADR-0009 is the template. |
| **TD-8** | Info | `docs/06_Coding_Standards.md`, `docs/07_Roadmap.md` | Empty stub documents. | Write these before onboarding new contributors. |

---

## 9. Known Risks

| ID | Risk | Impact | Likelihood | Mitigation |
|----|------|--------|-----------|------------|
| **R-1** | **Architecture-implementation gap.** 315 KB of design docs with ~700 lines of implementation code. Risk of drift as implementation progresses. | High | Medium | Treat docs as source of truth. Update docs when implementation forces design changes. Use ADRs for deviations. |
| **R-2** | **Plugin sandbox enforcement.** V1 implements OS-level isolation on Linux (Landlock, seccomp, namespaces). Non-Linux platforms have degraded enforcement. Full cross-platform sandbox is V2 scope. | Critical | Medium | V1 Linux sandbox implemented. Residual risk depends on kernel integrity, correct configuration, and implementation correctness. **Note: CI cannot currently verify Linux runtime enforcement.** Independent security review required for definitive risk rating. |
| **R-3** | **Crypto implementation correctness.** The architecture specifies AES-256-GCM, Argon2, SHA-256, and GPG signing. Implementation must use vetted libraries and correct patterns (Security Assumption SA-9). | Critical | Low | Use `golang.org/x/crypto` for Argon2, stdlib `crypto/aes` + `crypto/cipher` for AES-256-GCM, stdlib `crypto/sha256` for hashing. Do not roll custom crypto. |
| **R-4** | **CI/CD.** CI/CD is configured (`.github/workflows/ci.yml`, `.golangci.yml`). Tests, linting, and builds run on push/PR for ubuntu and windows. | Low | Low | Operational. Consider adding test coverage reporting and security scanning. |
| **R-5** | **Single contributor.** Bus factor of 1. All design knowledge lives in docs (good) but implementation velocity is constrained. | Medium | — | The comprehensive docs mitigate knowledge loss. Consider prioritizing an onboarding guide (`06_Coding_Standards.md`). |
| **R-6** | **Platform-specific defaults.** Config defaults assume Linux paths. Windows and macOS users would need to override every path. | Low | Medium | Use Go's `os.UserConfigDir()` and `os.UserCacheDir()` for platform-aware defaults. |

---

## 10. What Works Today

If you clone this repository and run it, here is what you can actually do:

### Configuration Manager

```go
import "telos/internal/config"

mgr := config.NewManager()
profile, err := mgr.Load(config.LoadOptions{
    SystemConfigFile: "/etc/telos/config.yaml",
    UserConfigFile:   "~/.config/telos/config.yaml",
    CLIOverrides: map[string]any{
        "logging": map[string]any{"level": "debug"},
    },
})

cfg := profile.Config()              // Deep copy — safe to mutate
src := profile.Source("logging.level") // → "cli"
meta := profile.Metadata()            // Schema version, warnings
pluginCfg := profile.PluginConfig("io.telos.storage.s3") // Plugin-specific config
```

**Capabilities:** Load YAML files, merge from 5 tiers, validate schema (reject unknown core keys, accept arbitrary plugin keys), validate business rules (warnings), expose immutable profile with source attribution, bulk error reporting.

### Logger

```go
import log "telos/internal/logger"

logger := log.New(log.InfoLevel, os.Stdout)
logger = logger.WithComponent("discovery_engine")
logger.Info("Starting discovery...")
logger.Audit("User approved backup plan")
logger.Error(err, "Plugin timed out")
```

**Capabilities:** Structured JSON output, five severity levels (debug/info/warn/error/audit), component tagging, correlation ID propagation via context, console writer for interactive use.

### What you CANNOT do

- Run `telos backup`, `telos restore`, `telos discover`, or any CLI command.
- Discover, classify, plan, capture, encrypt, store, or restore anything.
- Integrate plugin discovery with the Discovery Engine or execute a full backup pipeline.
- Create, read, or verify any backup archive.

---

## 11. Recommended Next Steps

The following sequence respects the dependency graph defined in `02_Architecture.md` and builds from leaf subsystems inward.

### Immediate (Foundation Completion)

| Priority | Task | Subsystem | Rationale |
|----------|------|-----------|-----------|
| 1 | **Define Go types for core data model entities** | S4 | Every subsystem depends on the shared vocabulary: `EnvironmentManifest`, `BackupPlan`, `RestorePlan`, `MachineProfile`, `DiscoveryResult`, etc. These structs must exist before any pipeline code can be written. |
| 2 | **Implement Crypto Engine (S9)** | S9 | Leaf dependency (depends on nothing internal). Required by Capture Engine, Storage Backend, and Restore Engine. AES-256-GCM encryption, SHA-256 hashing, Argon2 KDF. |
| 3 | **Implement CLI Shell framework (S1)** | S1 | Required for any user-facing functionality. Choose and integrate a command framework. Define top-level commands: `backup`, `restore`, `discover`, `verify`, `diff`, `config show`. |
| 4 | **Implement Discovery Engine (S3)** | S3 | Consume the Plugin Host registry and aggregate isolated plugin responses into discovery results. |
| 5 | ~~**Set up CI/CD pipeline**~~ ✅ Done | — | CI/CD configured: `.github/workflows/ci.yml` with test, lint, and build jobs. |

### Near-Term (Core Pipeline — Phase 2)

| Priority | Task | Subsystem |
|----------|------|-----------|
| 6 | Implement Manifest construction and immutability | S4 |
| 7 | Implement Classifier with deterministic rules | S5 |
| 8 | Implement Planner (backup plan generation) | S6 |
| 9 | Implement Orchestrator (backup pipeline coordination) | S2 |
| 10 | Build first Discovery plugin (e.g., APT packages on Ubuntu) | `plugins/` |
| 11 | Implement Capture Engine (S7) | S7 |
| 12 | Implement Storage Backend — local filesystem (S8) | S8 |
| 13 | Integrate GPG signing and archive-container serialization | S9/S8 |

### Debt Paydown (Parallel)

| Task | Priority |
|------|----------|
| Backfill ADRs 0001–0008 from canonical docs | Low |
| Write `06_Coding_Standards.md` | Medium |
| Write `07_Roadmap.md` | Low |
| Add missing unit tests (env_mapper, business_validation, errors, sources) | Medium |
| Fix context key type in logger (TD-3) | Low |
| Remove placeholder `core/some_subsystem.go` | Low |

---

> **This document is a snapshot.** It reflects the state of the project as of 2026-08-29. Update it after each significant implementation milestone.
