# Telos — Project Status

> **Status:** Active
> **Last Updated:** 2026-09-23 (Bounded Stderr and Sandbox Stdin Forwarding)
> **Author:** AI Engineering Review (Antigravity)
> **Audience:** Project maintainers, future contributors, AI sessions
> **Purpose:** Engineering dashboard — understand exactly where the project stands

---

## Table of Contents

1. [Overall Progress](#1-overall-progress)
2. [Engineering Updates](#2-engineering-updates)
3. [Milestone Assessment](#3-milestone-assessment)
4. [Documentation Status](#4-documentation-status)
5. [Subsystem Status](#5-subsystem-status)
6. [ADR Status](#6-adr-status)
7. [Security & Sandbox Status](#7-security--sandbox-status)
8. [Testing Status](#8-testing-status)
9. [Dependency Inventory](#9-dependency-inventory)
10. [Technical Debt](#10-technical-debt)
11. [Known Risks](#11-known-risks)
12. [What Works Today](#12-what-works-today)
13. [Recommended Next Steps](#13-recommended-next-steps)

---

## 1. Overall Progress

**Phase:** Foundation (Phase 1 of 4)
**Completion Estimate:** Scalar percentage removed (previously estimated at ~10%). Progress across the 17 architectural subsystems is uneven and cannot be represented honestly by a single percentage: 5 foundational subsystems (`internal/model` S4 vocabulary, `internal/crypto` S9 primitives, `internal/plugin` S12 host + sandbox, `internal/logger` S16, `internal/config` S17) have working code and thorough unit/integration tests (44 top-level tests passing on Linux), but 12 subsystems (S1, S2, S3, S5, S6, S7, S8, S10, S11, S13, S14, S15) have zero implementation, no end-to-end backup or restore pipeline exists, and no functional CLI command parsing exists. Progress is tracked strictly by subsystem maturity and milestone deliverables.

### What exists

| Category | State |
| ---------- | ------- |
| **Design documentation** | Comprehensive. Six canonical documents (~315,000 bytes) covering vision, requirements, architecture, threat model, data model, and plugin API. One onboarding context document (`08_PROJECT_CONTEXT.md`). Thirteen ADR files (six populated: ADR-0007, ADR-0009, ADR-0010, ADR-0011, ADR-0012, ADR-0013; seven empty stubs). Four gap/audit reports in `docs/reports/`. |
| **Configuration Manager (S17)** | Fully implemented: 5-tier merge, schema validation, business validation, source attribution, deep-copy immutability, plugin config passthrough, bulk error reporting. Tested (6 unit tests). |
| **Logging & Audit (S16)** | Partially implemented: zerolog wrapper with structured JSON output, level mapping, component tagging, correlation ID context, and a custom `audit` severity. Tested (6 unit tests). |
| **Domain model (S4 foundation)** | Implemented in `internal/model`: behavior-free, platform-neutral types for the manifest and its sections, plans/actions, archive and integrity records, plugin metadata, approvals, restore results, and verification reports. Tested (2 unit tests). |
| **Crypto Engine (S9 foundation)** | Implemented in `internal/crypto`: Argon2id key derivation, AES-256-GCM authenticated envelopes, general/credential key-purpose separation, SHA-256 hashing, and malformed-envelope work-factor bounds. Tested (8 unit tests). GPG signing, archive-container serialization, and subsystem integration remain unimplemented. |
| **Plugin Host (S12 skeleton + V1 sandbox)** | Implemented in `internal/plugin`: directory scanning, strict manifest parsing, interface/type/permission validation, duplicate-ID isolation, stable registration snapshots, bounded timed JSON subprocess invocation, and V1 OS-level plugin isolation on Linux (Landlock + seccomp + namespaces, NO_NEW_PRIVS, and resource limits). Enforces offline invariants (rejects Discovery/Capture with network permission), ADR-0012 resource limit defaults (512MB RAM, 10GB file size, MaxProcesses=0), and defensive Policy slice isolation. Tested (17 host tests + 5 sandbox tests; verified in WSL2). Trust verification (signature checking, trusted key handling, TOFU, and package integrity) is documented but **NOT implemented**. Non-Linux platforms use un-sandboxed fallback execution. |
| **CLI entry point** | Skeleton `cmd/telos/main.go` — creates a logger, re-exec sandbox helper entrypoint, and invokes a placeholder demo subsystem (`core.SomeSubsystem`). Argument parsing is not functional. |
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
- No end-to-end tests or benchmarks.

---

## 2. Engineering Updates

### 2026-09-23 Security and Correctness Update

- **Sandbox stdin forwarding fix (`internal/plugin/sandbox/sandbox_linux.go`)**
  - **Commit Hash:** `fd21debbf9339c8dbc83eb7c6b206e1a33c4e721`.
  - **Bug:** The Linux sandbox supervisor serialized `sandboxHelperInput.PluginStdin`, but `RunSandboxHelper` never connected the decoded bytes to the sandboxed plugin's fd 0 before `syscall.Exec`. A live real-sandbox probe confirmed that plugins received empty/EOF stdin regardless of the request body.
  - **Fix:** The helper writes `PluginStdin` to a sealed memfd, applies `F_SEAL_WRITE | F_SEAL_GROW | F_SEAL_SHRINK`, seeks to offset zero, duplicates it onto fd 0, and closes the original descriptor before Landlock and seccomp are applied.
  - **Impact:** Discovery and Capture request bodies are now delivered to sandboxed plugins for the first time. This is a security-relevant correctness fix because sandbox request delivery is part of the plugin execution boundary.
  - **Scope:** This was a pre-existing defect discovered incidentally during Item B work; it was not part of the original stderr-bounding scope.
  - **Tests:** `TestLinuxSandboxPluginStdin` uses the real `NewLinuxSandbox` path and verifies non-empty, nil, explicitly empty, and 4 MiB stdin delivery. The existing Linux integration and stderr truncation tests also pass.

### Completed and Verified

- **Item A: Network Permission Rejection for Discovery and Capture in `load()` (`internal/plugin/host.go`)**
  - **Commit Hash:** `627a19eb766157f31b93f13757b37c82675a1b8c` (implementation), `10063923e9d547587be06aab73a3b70e38cb995f` (tests).
  - **Evidence:** `internal/plugin/host.go:200-202` explicitly rejects manifests declaring `permissions.network: true` for `Discovery` and `Capture` plugins during `load()`, returning an error and preventing registration.
  - **Tests:** `TestNetworkPermissionValidation` (10 subtests covering Discovery rejection, Capture rejection, ClassificationRule rejection, Storage acceptance, Restore acceptance, offline variants, and unknown types) and `TestInvariantN1NoDiscoveryOrCaptureHasNetwork` in `internal/plugin/host_test.go`. All PASS.
- **Item C1: HostOptions Resource Limits Defaults and Policy Wiring (`internal/plugin/host.go`)**
  - **Commit Hash:** `7ae714ae96154a92e78354fb8c3ea8b83267bdfd`.
  - **Evidence:** `internal/plugin/host.go:34-42` defines single-source-of-truth constants matching ADR-0012: `defaultMemoryBytes = 512MB` (536,870,912 bytes), `defaultMaxFileSizeBytes = 10GB` (10,737,418,240 bytes), `defaultMaxProcesses = 0` (disabled). Lines 57-70 add `MemoryBytes`, `MaxFileSizeBytes`, and `MaxProcesses` to `Options`. Lines 129-137 implement normalization rules in `New()` (`<= 0` falls back to default for memory and file size; `< 0` falls back to 0 for MaxProcesses). Lines 364-366 copy normalized values into `sandbox.Policy` in `buildPolicy()`.
  - **Tests:** `TestNewHostDefaultsMatchADR0012`, `TestResourceLimitZeroValueSemantics`, `TestResourceLimitNegativeValueSemantics`, `TestCustomHostOptionsFlowIntoPolicy`, `TestMaxFileSizeBytesEnforcedLinux` (PASS on Linux), `TestDefaultMemoryBytesEnforcedLinux` (PASS on Linux, confirming minimal compiled Go fixture failure under 512MB RLIMIT_AS with "failed to reserve page summary memory").
- **Item C2: Policy Slice Isolation in `buildPolicy` (`internal/plugin/host.go`)**
  - **Commit Hash:** `77c37484a691e8c89bc837eac0b4388a488fba9e`.
  - **Evidence:** `internal/plugin/host.go:352-368` guarantees returned `sandbox.Policy` slices do not alias `Plugin` or `Manifest` slices. `ReadPaths` allocates a fresh slice (`make([]string, 0, 1+len(p.Manifest.FilesystemRead))`), `WritePaths` clones via `slices.Clone(p.Manifest.FilesystemWrite)`, and `Executables` is freshly allocated (`[]string{p.Executable}`).
  - **Tests:** `TestPolicySliceAliasingSpareCapacity` (asserts mutating or appending to returned policy slices does not modify manifest backing arrays even when spare capacity exists), `TestTwoBuildPolicyCallsShareNoBackingArrays` (pointer divergence and behavioral isolation), and `TestConcurrentBuildPolicyUnderRace` (concurrent mutation under race detector). All PASS.
- **Documentation Policy Reconciliation for Restore Plugins**
  - **Commit Hash:** `7fa6f6775ef1c560e2bc7c93075922f3cf78b96a`.
  - **Evidence:** Reconciled `docs/05_Plugin_API.md`, `docs/reports/2026-08-31-V1-First-Party-Plugin-Gap-Audit-Verification.md`, `docs/reports/2026-08-31-V1-First-Party-Plugin-Gap-Audit.md`, and `docs/reports/2026-08-31-V1-First-Party-Plugin-Gap-Design-Resolution.md`. Clarified normative rules: Restore plugins default to `network: false` but may declare `network: true` where explicitly justified (e.g., package installation), whereas Discovery and Capture are strictly offline. Note: Commit 7fa6f67 message says "and host tests" but contains only 4 markdown files (no Go code or test files were included).
- **Item B: Bounded Stderr on Direct `Invoke()` and Real Sandbox Paths**
  - **Status:** DONE.
  - **Commit Hash:** `3f1a55fdfa5c7e88db88b20ba047d40b75535b4d`.
  - **Evidence:** `internal/plugin/host.go` normalizes one authoritative `defaultStderrLimit` and applies `sandbox.BoundedStderr` to direct invocation; `internal/plugin/sandbox/sandbox_linux.go` and `sandbox_other.go` apply the same bounded collector to sandbox execution. Non-positive sandbox limits fail closed.
  - **Invariant-to-test coverage:**

    | Invariant | Requirement | Tests |
    | --- | --- | --- |
    | **S1** | Direct invocation retains at most the configured stderr limit and preserves successful execution when stderr is truncated. | `TestBoundedStderr`, `TestInvokeStderrTruncation` |
    | **S2** | The real Linux sandbox retains at most the configured stderr limit and reports truncation without unbounded growth. | `TestLinuxSandboxStderrTruncation/success_truncated` |
    | **S3** | A plugin that floods stderr and exits non-zero remains a failure, with bounded/truncated stderr reported; invalid limits fail closed. | `TestLinuxSandboxStderrTruncation/failure_truncated`, `TestLinuxSandboxStderrLimitValidation`, `TestFallbackSandboxStderrLimitValidation` |

  - **Verification:** Build, vet, lint, and tests were verified on Windows; the full race-enabled test suite was verified in WSL/Linux. Both platforms passed their applicable tests.

### Partially Done

### Open Issues and Blockers

- **OPEN ISSUE: Untracked RLIMIT_AS experiment material was quarantined outside the repo; the investigation must be redone.**
- **OPEN ISSUE (blocks the APT plugin): Minimal Compiled Go Fixture Startup Failure under RLIMIT_AS 512 MB**
  - **Issue:** A minimal compiled Go fixture fails to start under the ADR-0012 default `RLIMIT_AS` of 512 MB (`fatal error: failed to reserve page summary memory`).
  - **Source:** `TestDefaultMemoryBytesEnforcedLinux` in `internal/plugin/host_test.go:860-924`, which logs `"REPORTED: minimal compiled Go fixture failed under 512 MB RLIMIT_AS default: ..."` and still passes.
  - **State:** The test does not assert startup (it logs the error and returns cleanly without failing). ADR-0012 has not been amended, and the ADR value was intentionally not changed in code (`internal/plugin/host.go:defaultMemoryBytes = 512 * 1024 * 1024`). The investigation and any ADR-0012 amendment have not been done (pending investigation).
  - **Impact:** Blocks the APT plugin and any compiled Go plugin from starting under default resource limits until Go runtime virtual address space reservation is investigated and ADR-0012 is formally amended.

### Planned but NOT Done

- **Item D: Subprocess Permission Handling (ADR-0007)**
  - **Status:** Approved in design (ADR-0007); no implementation code in `host.go` or `sandbox`.
  - **Missing:** Manifest `subprocess` items are currently stored as raw strings in `model.PluginManifest.Subprocesses` without validation. Name validation (rejecting `/` and `\`), `exec.LookPath` resolution to absolute binary paths on the host, appending resolved binaries to `Policy.Executables`, and automatically granting read+execute rights to system library paths (`/lib`, `/lib64`, `/usr/lib`, `/usr/lib64`, `/etc/alternatives`) in `buildPolicy` remain unimplemented.
- **Items E1, E2, E3: Capture Staging Lifecycle (ADR-0013)**
  - **Status:** Approved in design (ADR-0013); no implementation code in repository.
  - **Missing:**
    - E1: Staging allocator in core to create cryptographically random, per-invocation directories under `/var/lib/telos/staging/capture-<uuid>`.
    - E2: `CaptureRequest.staging_location` field definition, host request wiring to map the path to `Policy.WritePaths` and `Policy.ReadPaths`, and cleanup destruction.
    - E3: Validation of `CaptureResponse.artifacts[].artifact_location` ensuring returned paths are strictly relative subpaths within the designated staging location.
- **Item S0: Subprocess Security Analysis Report**
  - **Status:** Missing. No dedicated subprocess security analysis report exists in `docs/reports/`. Process rule requires this analysis to be reviewed prior to implementing Item D.

### Known Open Security Gaps (Re-verified as Still Present)

1. **Subprocess Execution Blocked under Linux Sandbox:**
   Plugins declaring subprocesses (such as `apt` or `dpkg`) cannot execute them under the Landlock sandbox because `buildPolicy` does not append resolved executables or system libraries to the sandbox policy.
2. **Trust Phase Completely Unimplemented:**
   `Plugin.Manifest.Signature` is parsed but ignored (`host.go:215`). No digital signature validation, no trusted key management, no TOFU, and no package integrity verification exist.
4. **Non-Linux Platforms Have Degraded Plugin Isolation:**
   On Windows and macOS, plugins run with standard subprocess execution without Landlock filesystem containment, seccomp syscall filtering, or PID/network namespaces.
5. **No Instantaneous Task-Count Limit (Fork Bomb Defense Deferred to V2):**
   Per ADR-0012, `RLIMIT_NPROC` is disabled by default (`MaxProcesses = 0`) because it is counted across the entire Real UID on the host, does not constrain root, and breaks Go runtime threads. Bounding task counts requires cgroups v2 (`pids.max`), which is deferred to V2. Containment in V1 relies on PID namespace teardown and `SIGKILL` on execution timeout.

### Test and CI Status

Real test execution results verified on 2026-09-19:

- **Windows 11 Execution Environment:**
  - `go build ./...`: **PASS** (exit code 0, 0 warnings)
  - `go vet ./...`: **PASS** (exit code 0, 0 issues)
  - `golangci-lint run`: **PASS** (exit code 0, 0 issues)
  - `go test ./... -count=1`: **PASS** (6 packages ok; 41 tests executed, 39 PASS, 2 SKIP: `TestInvokeUsesJSONProtocol` skips due to Windows shell fixture, `TestMaxFileSizeBytesEnforcedLinux` and `TestDefaultMemoryBytesEnforcedLinux` skip due to Linux-only rlimit requirement)
  - `go test ./... -race -count=1`: **FAIL** (`go: -race requires cgo; enable cgo by setting CGO_ENABLED=1` due to absence of Windows GCC toolchain)
- **Linux WSL2 Execution Environment (Ubuntu 26.04.1 LTS, Kernel 6.18.33.2-microsoft-standard-WSL2 x86_64, Go 1.26.4):**
  - Executed as `root` (to satisfy namespace capability requirements):
    - `go test ./... -race -count=1`: **PASS** (all 6 packages pass with race detector enabled)
      - `telos/internal/config`: ok (1.080s)
      - `telos/internal/crypto`: ok (1.176s)
      - `telos/internal/logger`: ok (1.032s)
      - `telos/internal/model`: ok (1.026s)
      - `telos/internal/plugin`: ok (17.663s)
      - `telos/internal/plugin/sandbox`: ok (10.342s)
    - All 44 top-level tests pass with 0 skips when executed as root.
    - Verified Linux-only runtime enforcement:
      - `TestMaxFileSizeBytesEnforcedLinux`: **PASS** (9.78s) — verified RLIMIT_FSIZE enforcement prevents file writes beyond limit.
      - `TestDefaultMemoryBytesEnforcedLinux`: **PASS** (7.05s) — logs REPORTED and passes; verifies RLIMIT_AS 512MB default is active (minimal compiled Go fixture fails under 512MB RLIMIT_AS default: "failed to reserve page summary memory").
      - `TestBuildSeccompFilterStructure`: **PASS** (0.00s)
      - `TestBuildSeccompFilterClone3Denied`: **PASS** (0.00s)
      - `TestLinuxSandboxIntegration`: **PASS** (7.61s) — Landlock ABI 7, seccomp BPF, and PID/net namespaces verified: `[clone3: ENOSYS clone_newuser: EPERM mount: EPERM unshare: EPERM setns: EPERM read_allowed_file: OK read_allowed_dir: OK write_allowed_file: OK execute_writable: BLOCKED read_denied: BLOCKED]`.
  - Executed as unprivileged user (`te`):
    - The sandbox Linux integration tests run only as root or with capabilities (`CAP_SYS_ADMIN`). They skip under an unprivileged user and in CI (`sandbox_integration_test.go:57: Skipping integration test: CI environment lacks capabilities for CLONE_NEWPID and CLONE_NEWNET (EPERM)`).
- **CI Status (GitHub Actions `.github/workflows/ci.yml`):**
  - CI runs Ubuntu and Windows runners. Race detector, linting, and build jobs pass.
  - The sandbox Linux integration tests run only as root or with capabilities. They skip under an unprivileged user and in CI. Note that CI provides no kernel-level evidence for the sandbox today.

### Decisions Made Today

1. **Strict Offline Invariants for Discovery and Capture:** Manifests requesting `permissions.network: true` for `Discovery` and `Capture` plugin types are rejected during loading (`host.go:load()`).
2. **Restore Plugin Network Policy:** Reconciled documentation across `05_Plugin_API.md` and gap audit reports: Restore plugins default to offline (`network: false`) but are permitted to declare `network: true` if required for package downloads.
3. **ADR-0012 HostOptions and Policy Defaults:** Enforced resource limit defaults matching ADR-0012: `MemoryBytes = 512 MB`, `MaxFileSizeBytes = 10 GB`, `MaxProcesses = 0` (disabled). Explicit non-zero values from `Options` flow cleanly into `Policy`.
4. **Policy Slice Defense-in-Depth:** In `host.go:buildPolicy()`, returned policy slices are strictly decoupled from plugin manifest backing arrays to prevent any downstream mutation or concurrency race conditions from contaminating host registry state.

---

## 3. Milestone Assessment

The roadmap defined in `08_PROJECT_CONTEXT.md` §10 outlines four phases. Below is the current state against each.

### Phase 1 — Foundation

| Milestone | Status | Notes |
| ----------- | -------- | ------- |
| Configuration Manager (S17) — full implementation | ✅ Done | 10 source files, 1 ADR, 6 unit test cases. |
| Logging & Audit (S16) — core implementation | 🟡 Partial | Logger wrapper exists; audit-level logging works. Missing: file output, log rotation, append-only guarantees, audit trail format. |
| Project scaffolding (module, directories) | ✅ Done | `go.mod`, package layout, empty placeholder dirs. |
| CLI Shell (S1) — command framework | ❌ Not started | `main.go` exists but has no argument parsing. |
| Orchestrator (S2) — pipeline skeleton | ❌ Not started | Leaf dependencies exist; pipeline coordinator unwritten. |
| Data model Go types (S4) | ✅ Foundation implemented | `internal/model` contains the behavior-free shared vocabulary (9 Go files, 2 unit tests). Manifest construction, sealing, persistence, and workflow logic remain unimplemented. |

### Phase 2 — Core Pipeline

| Milestone | Status | Notes |
| ----------- | -------- | ------- |
| Discovery Engine (S3) | ❌ Not started | |
| Manifest (S4) | ❌ Not started | Construction and immutability logic not started. |
| Classifier (S5) | ❌ Not started | |
| Planner (S6) | ❌ Not started | |
| Capture Engine (S7) | ❌ Not started | |
| Crypto Engine (S9) | 🟡 Foundation implemented | Authenticated AES-256-GCM envelopes and Argon2id KDF implemented and tested (8 unit tests). GPG signing, archive container serialization, and subsystem integration remain. |
| Storage Backend (S8) | ❌ Not started | |
| Plugin Host (S12) | 🟡 Skeleton + V1 sandbox | Directory scanning, manifest validation, offline network rejection, registration, bounded subprocess protocol, bounded stderr on direct and real sandbox paths, sandbox stdin delivery, ADR-0012 resource limit defaults, Policy slice isolation, and Linux sandbox (Landlock + seccomp + namespaces) implemented. Subprocess translation (ADR-0007), staging lifecycle (ADR-0013), trust verification, and non-Linux sandboxes remain. |
| Restore Engine (S10) | ❌ Not started | |
| Verification Engine (S11) | ❌ Not started | |

### Phase 3 — Intelligence & Automation

| Milestone | Status |
| ----------- | -------- |
| AI Advisory Layer (S13) | ❌ Not started |
| Diff Engine (S14) | ❌ Not started |
| Scheduler (S15) | ❌ Not started |

### Phase 4 — Community & Ecosystem

| Milestone | Status |
| ----------- | -------- |
| Plugin SDK & developer docs | ❌ Not started |
| Plugin registry | ❌ Not started |
| Community plugin support | ❌ Not started |

---

## 4. Documentation Status

### Canonical Documents

| Document | File | Size | Status | Quality |
| ---------- | ------ | ------ | -------- | --------- |
| Vision | `00_Vision.md` | 28,586 B | ✅ Complete | Excellent. Establishes principles and philosophy. |
| Requirements | `01_Requirements.md` | 50,661 B | ✅ Complete | Excellent. FR-1 through FR-14, NFR-1 through NFR-8, DR, EIR. |
| Architecture | `02_Architecture.md` | 60,523 B | ✅ Complete | Excellent. All 17 subsystems defined with boundaries, dependencies, data flows, mermaid diagrams. |
| Threat Model | `03_Threat_Model.md` | 57,015 B | ✅ Complete | Excellent. Full STRIDE analysis, 10 attack surfaces, 7 threat actors, residual risk mapping. |
| Data Model | `04_Data_Model.md` | 62,890 B | ✅ Complete | Excellent. All entities, relationships, lifecycles, identity strategy. |
| Plugin API | `05_Plugin_API.md` | 63,321 B | ✅ Complete | Excellent. Five plugin types, execution model, sandbox, trust model, dispatch rules. |
| Coding Standards | `06_Coding_Standards.md` | 2 B | ❌ Empty | Stub file only. |
| Roadmap | `07_Roadmap.md` | 2 B | ❌ Empty | Stub file only. |
| Project Context | `08_PROJECT_CONTEXT.md` | 21,931 B | ✅ Complete | Good. Navigational onboarding reference. |
| Project Status | `99_Project_Status.md` | Active file | ✅ Complete | Engineering dashboard. |

### Assessment

The six core design documents collectively provide ~315 KB of thorough specification.

**Gaps:**
- `06_Coding_Standards.md` is empty — no Go style guide, naming conventions, or error handling patterns documented.
- `07_Roadmap.md` is empty — formal timeline beyond `08_PROJECT_CONTEXT.md` §10 is unwritten.
- The `updates/` directory contains working artifacts from prior development sessions (architecture reviews, audit reports, implementation plans). These are historical references, not canonical sources of truth.
- Four reports in `docs/reports/` document sandbox remediation, first-party plugin audit verification, and design resolutions.

---

## 5. Subsystem Status

### Implemented

| ID | Subsystem | Package | Files | Lines | Tests | Test Coverage |
| ---- | ----------- | --------- | ------- | ------- | ------- | -------------- |
| **S17** | Configuration Manager | `internal/config` | 11 Go files (10 source, 1 test) | ~600 | 6 test cases in `loader_test.go` | High — covers merge, strict decoding, plugin keys, validation accumulation, immutability, nested merge. |
| **S12** | Plugin Host | `internal/plugin`, `internal/plugin/sandbox` | 10 Go files (6 source, 4 test across host & sandbox) | ~1,500 | 22 tests (17 in `host_test.go`, 5 in `sandbox/`) | High for manifest parsing, offline network checks, resource limits, policy isolation, bounded stderr, stdin delivery, path normalization, and Linux sandbox primitives (Landlock ABI v1-v7, seccomp BPF, namespaces). Verified in Windows and WSL2. Missing: subprocess resolution, staging lifecycle, non-Linux sandboxing, trust verification. |
| **S9** | Crypto Engine (foundation) | `internal/crypto` | 2 Go files (1 source, 1 test) | ~250 | 8 test cases in `crypto_test.go` | High for Argon2id KDF, AES-256-GCM authenticated envelope, key-purpose separation, work-factor limits, and error handling. GPG signing and archive-container integration not started. |
| **S16** | Logging & Audit | `internal/logger` | 2 Go files (1 source, 1 test) | ~120 | 6 test cases in `log_test.go` | Moderate — covers level creation, component tagging, context correlation, log levels, audit severity, JSON output. Missing: file output, log rotation, concurrent usage. |
| **S4** | Domain Model (vocabulary) | `internal/model` | 9 Go files (8 source, 1 test) | ~500 | 2 test cases in `entities_test.go` | Covers data model structs and section representations. Manifest construction, validation, and sealing remain unwritten. |

### Placeholder / Skeleton

| ID | Subsystem | Package | State |
|----|-----------|---------|-------|
| — | SomeSubsystem (demo) | `core/` | 23-line placeholder in `core/some_subsystem.go`. Demonstrates logger injection pattern. Not an architectural subsystem. Referenced by `cmd/telos/main.go`. |

### Not Started

| ID | Subsystem | Planned Package | Blocked By |
| ---- | ----------- | ---------------- | ------------ |
| S1 | CLI Shell | `cmd/telos` or `internal/cli` | S2 (Orchestrator) |
| S2 | Orchestrator | `internal/core` | S3, S6, S7, S9, S10, S16, S17 (leaf deps done) |
| S3 | Discovery Engine | `internal/core` | S12 (Plugin Host) |
| S4 | Environment Manifest | `internal/model` | Domain vocabulary implemented; manifest construction, sealing, and persistence remain. |
| S5 | Classifier | `internal/core` | S4 (Manifest) |
| S6 | Planner | `internal/core` | S4, S5 |
| S7 | Capture Engine | `internal/core` | S6, S9 |
| S8 | Storage Backend | `internal/storage` | S9 |
| S10 | Restore Engine | `internal/core` | S2, S9 |
| S11 | Verification Engine | `internal/core` | S3, S14 |
| S13 | AI Advisory Layer | `internal/core` | S4, S5 |
| S14 | Diff Engine | `internal/core` | S4 |
| S15 | Scheduler | `internal/core` | S2 |

---

## 6. ADR Status

| ADR | Title | File | Status | Size |
| ----- | ------- | ------ | -------- | ------ |
| ADR-0001 | Offline-First | `ADRs/ADR-0001-Offline-First.md` | ❌ **Empty stub** | 0 B |
| ADR-0002 | Plugin Architecture | `ADRs/ADR-0002-Plugin-Architecture.md` | ❌ **Empty stub** | 0 B |
| ADR-0003 | Fresh-OS Restore | `ADRs/ADR-0003-Fresh-OS-Restore.md` | ❌ **Empty stub** | 0 B |
| ADR-0004 | Single Passphrase | `ADRs/ADR-0004-Single-Passphrase.md` | ❌ **Empty stub** | 0 B |
| ADR-0005 | Compatible Machine Policy | `ADRs/ADR-0005-Compatible-Machine-Policy.md` | ❌ **Empty stub** | 0 B |
| ADR-0006 | Directory-Based Plugins | `ADRs/ADR-0006-Directory-Based-Plugins.md` | ❌ **Empty stub** | 0 B |
| ADR-0007 | Subprocess Plugin Execution | `ADRs/ADR-0007-Subprocess-Plugin-Execution.md` | ✅ **Accepted** | 3,542 B |
| ADR-0008 | Full Backup V1 | `ADRs/ADR-0008-Full-Backup-V1.md` | ❌ **Empty stub** | 0 B |
| ADR-0009 | Configuration Manager | `ADRs/ADR-0009-Configuration-Manager.md` | ✅ **Accepted** | 7,457 B |
| ADR-0010 | Crypto Envelope | `ADRs/ADR-0010-Crypto-Envelope.md` | ✅ **Accepted** | 1,775 B |
| ADR-0011 | Plugin Execution Isolation | `ADRs/ADR-0011-Plugin-Execution-Isolation.md` | ✅ **Accepted** | 8,689 B |
| ADR-0012 | Plugin Resource Limits | `ADRs/ADR-0012-Plugin-Resource-Limits.md` | ✅ **Accepted (Revised 2026-09-09)** | 8,116 B |
| ADR-0013 | Capture Staging Lifecycle | `ADRs/ADR-0013-Capture-Staging-Lifecycle.md` | ✅ **Accepted** | 2,326 B |

### Assessment

6 of 13 ADR files are populated with complete Context, Decision, and Consequences sections (ADR-0007, ADR-0009, ADR-0010, ADR-0011, ADR-0012, ADR-0013). 7 ADR files remain empty stubs (ADR-0001 through ADR-0006, ADR-0008).

---

## 7. Security & Sandbox Status

### Workstream A (V1 Sandbox Enforcement)

The V1 Linux sandbox provides OS-level isolation. Verified status of mechanisms:

- **Landlock (filesystem rules):** IMPLEMENTED + RUNTIME VERIFIED (ABI v1–v7 supported, fail-closed baseline)
- **Seccomp-BPF (syscall denylist):** IMPLEMENTED + RUNTIME VERIFIED (`clone3` denied with `ENOSYS`; namespace clones denied with `EPERM`; `mount`, `unshare`, `setns` denied)
- **Namespaces (PID and Network):** IMPLEMENTED + RUNTIME VERIFIED (new PID namespace, loopback DOWN network namespace)
- **`PR_SET_NO_NEW_PRIVS`:** IMPLEMENTED + RUNTIME VERIFIED
- **Resource limits (`prlimit`):** IMPLEMENTED + RUNTIME VERIFIED (RLIMIT_AS 512MB, RLIMIT_FSIZE 10GB, RLIMIT_NPROC 0 per ADR-0012)
- **Output limits and timeouts:** IMPLEMENTED + STRUCTURALLY VERIFIED (4 MB stdout output limit, execution deadline)
- **Path normalization & boundary containment:** IMPLEMENTED + RUNTIME VERIFIED (`internal/plugin/sandbox/paths.go`)
- **Executable confinement:** IMPLEMENTED + RUNTIME VERIFIED
- **Plugin directory default read access:** IMPLEMENTED + STRUCTURALLY VERIFIED
- **Offline network enforcement:** IMPLEMENTED + STRUCTURALLY VERIFIED (`host.go:load()` rejects Discovery and Capture manifests with `network: true`)
- **Policy slice isolation:** IMPLEMENTED + STRUCTURALLY VERIFIED (`host.go:buildPolicy()` isolates `ReadPaths`, `WritePaths`, `Executables`)

### Linux Runtime Verification Evidence

- **Environment:** WSL2 Ubuntu 26.04.1 LTS / Linux kernel 6.18.33.2-microsoft-standard-WSL2, x86_64.
- **Execution Context:** Root execution required for namespace creation (`CLONE_NEWPID`, `CLONE_NEWNET`).
- **Landlock ABI:** Kernel verified supporting Landlock ABI 7.
- **Integration Test Results (`sandbox_integration_test.go`):**
  - `clone3` → `ENOSYS`
  - `clone(CLONE_NEWUSER)` → `EPERM`
  - `mount` → `EPERM`
  - `unshare` → `EPERM`
  - `setns` → `EPERM`
  - allowed regular-file read → succeeds
  - allowed directory read → succeeds
  - allowed regular-file write → succeeds
  - execution through WritePath → blocked (`EACCES`)
  - unauthorized read → blocked (`EACCES`)

### CI Environment Limitations

The sandbox Linux integration tests run only as root or with capabilities (`CAP_SYS_ADMIN`). They skip under an unprivileged user and in CI (returning `EPERM` on `CLONE_NEWPID` / `CLONE_NEWNET`). Note that CI provides no kernel-level evidence for the sandbox today. Full kernel-level verification requires privileged Linux environments or running locally as root.

### WritePaths vs Executables (Verified Invariant)

The V1 model strictly enforces: **WritePaths MUST NOT implicitly grant EXECUTE.**
- `ReadPaths` → read rights in Landlock
- `Executables` → explicit execute rights in Landlock
- `WritePaths` → write and file creation rights, strictly masking out `landlockAccessFSExecute`
Verified by `sandbox_integration_test.go`.

### Trust Phase (Not Implemented)

The documented Trust Phase remains completely unwritten:
- digital signature verification
- package signature validation
- package integrity verification
- trusted key management
- official vs. community plugin trust classification
- TOFU approval and approval persistence
- signature and key rotation handling

---

## 8. Testing Status

### Unit & Subsystem Tests

| Package | Test File | Test Count | Linux (WSL2 as root) | Windows 11 | Notes |
| --------- | ----------- | ------------ | -------------------- | ---------- | ------- |
| `internal/config` | `loader_test.go` | 6 | ✅ 6 PASS | ✅ 6 PASS | Tests full 5-tier merge, strict decoding, plugin keys, error accumulation, immutability, nested merge. |
| `internal/crypto` | `crypto_test.go` | 8 | ✅ 8 PASS | ✅ 8 PASS | Tests round-trip, empty plaintext, salt/nonce entropy, invalid input rejection, passphrase verification, envelope corruption, key separation, and SHA-256 hash. |
| `internal/logger` | `log_test.go` | 6 | ✅ 6 PASS | ✅ 6 PASS | Tests level creation, component tagging, correlation ID context, levels, audit severity, JSON output. |
| `internal/model` | `entities_test.go` | 2 | ✅ 2 PASS | ✅ 2 PASS | Tests manifest discovery sections and archive identity/storage fields. |
| `internal/plugin` | `host_test.go` | 17 | ✅ 17 PASS | ⚠️ 15 PASS, 2 SKIP | Tests discovery registration, manifest validation, offline network rejection, ADR-0012 defaults, slice isolation, race safety, and Linux resource limits. `TestInvokeUsesJSONProtocol` skips on Windows due to shell fixture; `TestMaxFileSizeBytesEnforcedLinux` and `TestDefaultMemoryBytesEnforcedLinux` skip on non-Linux. |
| `internal/plugin/sandbox` | `paths_test.go`, `seccomp_linux_test.go`, `sandbox_integration_test.go` | 5 (Linux), 2 (Win) | ✅ 5 PASS | ✅ 2 PASS | Tests path normalization, policy validation, BPF seccomp filter structure, clone3 denial, and full Linux sandbox integration (Landlock + seccomp + namespaces). Seccomp and integration tests are build-tagged for Linux only. |
| **Total** | | **44 (Linux) / 41 (Win)** | **✅ 44 PASS** | **⚠️ 39 PASS, 2 SKIP** | 100% pass rate on Linux with `-race -count=1`. |

### Test Infrastructure

- CI/CD configured in `.github/workflows/ci.yml` (test on Ubuntu/Windows with race detector on Linux; lint with golangci-lint; build binaries).
- Linting configured in `.golangci.yml` with gosec, govet, staticcheck, revive, gocritic. Runs clean (0 issues).
- Local untracked test binaries (`config.test`, `crypto.test`, etc.) exist in the root working tree from past manual builds, ignored by `.gitignore`.

---

## 9. Dependency Inventory

### Direct Dependencies

| Module | Version | Purpose | Used By |
| -------- | --------- | --------- | --------- |
| `github.com/rs/zerolog` | v1.35.1 | Structured JSON logging | `internal/logger` |
| `golang.org/x/crypto` | v0.55.0 | Argon2id key derivation | `internal/crypto` |
| `golang.org/x/sys` | v0.47.0 | Linux syscalls, Landlock, seccomp BPF | `internal/plugin/sandbox`, `cmd/telos` |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML parsing with strict mode | `internal/config`, `internal/plugin` |

### Indirect Dependencies

| Module | Version | Source |
| -------- | --------- | -------- |
| `github.com/mattn/go-colorable` | v0.1.14 | zerolog transitive |
| `github.com/mattn/go-isatty` | v0.0.20 | zerolog transitive |

### Assessment

Dependency footprint remains minimal and intentional (four direct dependencies). All direct dependencies are promoted in `go.mod`.

---

## 10. Technical Debt

| ID | Severity | Location | Issue | Resolution | Status |
| ---- | ---------- | ---------- | ------- | ------------ | -------- |
| **TD-1** | Medium | `cmd/telos/main.go`, `core/some_subsystem.go` | Placeholder `SomeSubsystem` in `core/` package. Demonstrates demo logging; imported by `main.go`. | Remove `core/some_subsystem.go` and refactor `main.go` when CLI Shell (S1) is implemented. | **OPEN / UNRESOLVED** (Re-verified: `core/some_subsystem.go` still exists and is imported by `main.go`) |
| **TD-2** | Low | `internal/config/env_mapper.go` | Only supports 2 levels of nesting for env vars (`TELOS_LOGGING_LEVEL`). Deeply nested plugin config cannot be mapped from env. | Acceptable for V1; document limitation. | **OPEN** |
| **TD-3** | Low | `internal/logger/log.go:89` | Context correlation ID uses a bare string key rather than a private type. | Define private context key type. | **OPEN** |
| **TD-4** | Low | `internal/logger/log.go:60` | `AuditLevel` mapping in `New()` is misleading (though `Audit()` bypasses it via level-less `.Log()`). | Clean up constructor mapping. | **OPEN** |
| **TD-5** | Medium | `internal/config/defaults.go` | Default config paths are Linux-only (`/var/lib/telos`). | Make platform-aware using `os.UserConfigDir()`. | **OPEN** |
| **TD-6** | Low | `internal/logger/log.go:71-73` | Console writer activated for any `*os.File`. | Add explicit `UseConsole bool` option. | **OPEN** |
| **TD-7** | Info | `docs/ADRs/` | 7 ADR files (0001–0006, 0008) remain empty stubs. | Backfill ADR format from canonical documents. | **PARTIAL** (6 of 13 ADRs now populated: ADR-0007, 0009, 0010, 0011, 0012, 0013) |
| **TD-8** | Info | `docs/06_Coding_Standards.md`, `docs/07_Roadmap.md` | Empty stub documents. | Write before onboarding contributors. | **OPEN** |

---

## 11. Known Risks

| ID | Risk | Impact | Likelihood | Mitigation |
| ---- | ------ | -------- | ----------- | ------------ |
| **R-1** | **Architecture-implementation gap.** Thorough design specifications (~315 KB) with early-stage implementation (~2,500 lines of Go). | High | Medium | Canonical docs serve as source of truth. Discrepancies tracked and resolved via ADRs. |
| **R-2** | **Plugin sandbox enforcement.** V1 Linux sandbox is robust (Landlock + seccomp + namespaces), but non-Linux platforms use degraded fallback execution. CI cannot execute privileged Linux integration tests. | Critical | Medium | Maintain Linux-first security posture. Require WSL2 or privileged runners for pre-release validation. |
| **R-3** | **Crypto implementation correctness.** Crypto Engine handles sensitive backup and credential material. | Critical | Low | Standard algorithms used (Argon2id, AES-256-GCM via `golang.org/x/crypto` and stdlib). Subsystem integration and key management still to be written. |
| **R-4** | **CI/CD environment capabilities.** Namespace creation fails in unprivileged CI runners (`EPERM`). | Low | High | Unit tests, race detector, and linters run in CI; integration tests gated by capability probe. |
| **R-5** | **Single contributor.** Bus factor of 1. | Medium | — | Comprehensive documentation mitigates context loss. |
| **R-6** | **Platform-specific defaults.** Config defaults assume Linux paths (`/var/lib/telos`). | Low | Medium | Add cross-platform path resolution prior to non-Linux release. |

---

## 12. What Works Today

If you clone this repository and run the code today, here is what is actually implemented and verifiable:

### 1. Configuration Management (S17)
Loads and merges configuration across CLI overrides, environment variables, user config, system config, and built-in defaults. Performs strict schema validation and business validation. Exposes an immutable profile with per-setting source attribution.

### 2. Structured Logging & Auditing (S16)
Emits structured JSON logs with severity levels (debug, info, warn, error, audit), component tagging, and correlation ID propagation. `audit` severity provides dedicated logging for security-sensitive events.

### 3. Domain Model Entities (S4 Foundation)
Platform-neutral Go structs in `internal/model` representing the core vocabulary: `EnvironmentManifest` (all 10 discovery sections), `BackupPlan`, `RestorePlan`, `MachineProfile`, `ApprovalGate`, and archive container records.

### 4. Cryptographic Primitives (S9 Foundation)
Generates Argon2id key derivations from passphrases, AES-256-GCM encrypted authenticated envelopes with tamper detection, purpose-specific key separation (`PurposeGeneral` vs `PurposeCredential`), and SHA-256 hashing.

### 5. Plugin Host & Linux Sandbox (S12)
Scans plugin directories, parses manifests, rejects invalid plugin types and semantic versions, rejects `Discovery` and `Capture` plugins declaring `network: true`, normalizes filesystem paths, allocates fresh isolated Policy slices, and applies ADR-0012 resource limit defaults (512MB RAM, 10GB file size). Under Linux, executes plugins inside an OS-level sandbox combining Landlock filesystem isolation, seccomp BPF syscall filtering (`clone3` denied with `ENOSYS`, namespace clones and `mount` denied with `EPERM`), PID and network namespaces (`loopback DOWN`), and `PR_SET_NO_NEW_PRIVS`.

### What you CANNOT do today
- Run `telos backup`, `telos restore`, `telos discover`, or any CLI command (CLI Shell argument parsing is not implemented).
- Execute an end-to-end backup, restore, diff, or verification pipeline (Orchestrator S2 is not implemented).
- Validate plugin digital signatures or TOFU key records (Trust Phase is not implemented).
- Execute plugins requiring external subprocesses under the Linux sandbox (subprocess resolution ADR-0007 is not implemented).
- Capture artifacts to staging directories (staging lifecycle ADR-0013 is not implemented).
- Run compiled Go plugins under default resource limits (OPEN ISSUE: minimal compiled Go fixture fails to start under ADR-0012 default RLIMIT_AS of 512 MB with "fatal error: failed to reserve page summary memory", blocking the APT plugin; pending investigation).

---

## 13. Recommended Next Steps

The remaining work is structured in strict dependency order:

```
[RLIMIT_AS Investigation & ADR-0012 Amendment]
               │
               ▼
[S0: Subprocess Security Analysis Review]
               │
               ▼
[D: Subprocess Permission Handling (ADR-0007)] (needs C, S0)
               │
               ▼
[E1: Staging Allocator (ADR-0013)]
               │
               ▼
[E2: CaptureRequest.StagingLocation & Host Wiring]
               │
               ▼
[E3: CaptureResponse.artifact_location Validation]
               │
               ▼
[S1: CLI Shell & S2: Orchestrator Skeleton]
```

> **Note on Binaries:** A preliminary git history search found no committed .exe/.test files; not independently re-audited.

### Dependency order after completed Item B

Item B is complete. The remaining dependency order is:

```text
RLIMIT_AS Investigation & ADR-0012 Amendment
               │
               ▼
S0: Subprocess Security Analysis Review
               │
               ▼
D: Subprocess Permission Handling (ADR-0007)
               │
               ▼
E1: Staging Allocator (ADR-0013)
               │
               ▼
E2: CaptureRequest.StagingLocation & Host Wiring
               │
               ▼
E3: CaptureResponse.artifact_location Validation
               │
               ▼
S1: CLI Shell & S2: Orchestrator Skeleton
```

### 1. RLIMIT_AS Investigation and ADR-0012 Amendment (Pending)
- **Status:** Open blocker for the APT plugin (pending investigation).
- **Issue:** A minimal compiled Go fixture fails to start under the ADR-0012 default `RLIMIT_AS` of 512 MB (`fatal error: failed to reserve page summary memory`). Source: `TestDefaultMemoryBytesEnforcedLinux` in `internal/plugin/host_test.go`, which logs `"REPORTED"` and still passes. State: the test does not assert startup, ADR-0012 has not been amended, and the ADR value was intentionally not changed in code (`internal/plugin/host.go:defaultMemoryBytes = 512 * 1024 * 1024`). The investigation and any ADR-0012 amendment have not been done (pending investigation).
- **Requirement:** Investigate Go runtime virtual address space reservation requirements versus resident set size (RSS); earlier measurements were inconclusive. Formally amend ADR-0012 to establish a viable memory limit before modifying the code constant.

### 3. Item S0: Subprocess Security Analysis Review
- **Requirement:** Review and complete the subprocess threat analysis (`docs/reports/`) before implementing Item D.
- **Scope:** Evaluate attack surface of granting Landlock read+execute rights on system libraries (`/lib`, `/lib64`, `/usr/lib`, `/usr/lib64`, `/etc/alternatives`) and executing host binaries (e.g., `apt`, `dpkg`).

### 4. Item D: Subprocess Permission Handling (ADR-0007)
- **Prerequisite:** Depends on Item C (Resource limits, completed in commit `7ae714a`) and Item S0 review.
- **Specification:**
  - Validate manifest `permissions.subprocess` entries are strictly basenames (reject `/` and `\`).
  - Resolve basenames to absolute paths on host via `exec.LookPath(name)`.
  - Append resolved paths to `Policy.Executables`.
  - Automatically append system library paths (`/lib`, `/lib64`, `/usr/lib`, `/usr/lib64`, `/etc/alternatives`) with read+execute access in Landlock when subprocesses are declared.

### 5. Items E1 → E2 → E3: Capture Staging Lifecycle (ADR-0013)
- **Sequence:** E1 → E2 → E3.
- **E1 (Staging Allocator):** Implement allocator in core to create cryptographically random temporary directories under `/var/lib/telos/staging/capture-<uuid>` with restricted permissions (`0700`).
- **E2 (Request Wiring & Cleanup):** Extend `CaptureRequest` with `staging_location`, update `Plugin Host` to canonicalize and append staging location to `Policy.WritePaths` and `Policy.ReadPaths`, and ensure unconditional cleanup by caller upon completion or failure.
- **E3 (Artifact Validation):** Validate `CaptureResponse.artifacts[].artifact_location` ensuring returned files are relative paths contained strictly within the assigned staging location (preventing sandbox path traversal escapes).

### 6. Subsystem Implementation Pipeline & Technical Debt Resolution
- **TD-1 remains open:** The placeholder `SomeSubsystem` in `core/some_subsystem.go` remains present and imported by `cmd/telos/main.go`. It must be removed and `main.go` refactored when CLI Shell (S1) is implemented.
- **Subsequent pipeline milestones:**
  - Implement CLI Shell (S1) command parsing framework.
  - Implement Orchestrator (S2) linear pipeline skeleton.
  - Implement Discovery Engine (S3) to aggregate registered plugin responses into `EnvironmentManifest`.
  - Implement Capture Engine (S7) and Storage Backend (S8).
