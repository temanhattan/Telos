# V1 First-Party Plugin Functional & Security Gap Audit

> **Date:** 2026-08-31
> **Type:** Audit Report
> **Focus:** V1 Plugin Host & Sandbox Capability Boundaries

## 1. Scope
This audit evaluates the current V1 plugin implementation against the documented Plugin API and Threat Model. The primary goal is to determine if the Plugin Host and Linux Sandbox correctly enforce the intended capability boundaries, resource limits, and protocol contracts when interacting with realistic first-party plugins.

Third-party plugin trust, signing verification, and community distribution were explicitly out of scope for implementation, though their current state was evaluated.

## 2. Documentation Sources
The following project documentation was reviewed to establish the intended V1 behavior:
- `docs/00_Vision.md`
- `docs/01_Requirements.md`
- `docs/02_Architecture.md`
- `docs/03_Threat_Model.md`
- `docs/05_Plugin_API.md`
- `docs/99_Project_Status.md`

## 3. Implementation Areas Inspected
The audit traced data flow and enforcement mechanisms across the following components:
- `internal/plugin/host.go` (Manifest parsing, validation, and invocation)
- `internal/plugin/host_test.go`
- `internal/plugin/sandbox/policy.go` (Policy definition)
- `internal/plugin/sandbox/sandbox_linux.go` (Landlock, seccomp, namespaces, prlimit enforcement)
- `internal/plugin/sandbox/seccomp_linux.go` (BPF filter construction)
- `internal/plugin/sandbox/sandbox_integration_test.go`

## 4. Capability Matrix

| Plugin Type | Capability | Documented | Implemented | Tested | Result |
|---|---|---|---|---|---|
| **Discovery** | Network | NOT permitted | **Allowed** | No | 🔴 BUG |
| **Discovery** | Subprocess | Declared | **Ignored** | No | 🔴 BUG |
| **Discovery** | FS Read | Declared | Enforced | No | ✅ PASS |
| **Discovery** | FS Write | NOT permitted | Enforced | Yes | ✅ PASS |
| **Classification** | Network | NOT permitted | Enforced | Yes | ✅ PASS |
| **Classification** | Subprocess | NOT permitted | Enforced | Yes | ✅ PASS |
| **Classification** | FS Read | NOT permitted | Enforced | Yes | ✅ PASS |
| **Classification** | FS Write | NOT permitted | Enforced | Yes | ✅ PASS |
| **Capture** | Network | NOT permitted | **Allowed** | No | 🔴 BUG |
| **Capture** | Subprocess | Declared | **Ignored** | No | 🔴 BUG |
| **Capture** | FS Write | Staging only | **Denied entirely** | No | 🔴 BUG |
| **Restore** | Network | Not permitted (default) | Enforced | No | ✅ PASS |
| **Restore** | Subprocess | Declared | **Ignored** | No | 🔴 BUG |
| **Storage** | Network | Declared | Enforced | No | ✅ PASS |

## 5. First-Party Plugin Scenarios
The following realistic scenarios were evaluated via code inspection and adversarial modeling:

- **Classification Fixture**: Operates entirely in-memory. Manifest specifies no capabilities. **Status: Handled correctly.**
- **Storage Fixture**: Requires network access and filesystem read/write. Manifest is parsed and Landlock/network rules applied correctly. **Status: Handled correctly.**
- **Capture Fixture**: Requires read access to the system and write access to a staging location provided dynamically via the Request. **Status: Fails.** The host parses the manifest, correctly rejects `FilesystemWrite`, but fails to grant write access to the staging location in the sandbox policy. The plugin receives `EACCES` from Landlock when writing the artifact.
- **Discovery Fixture**: Requires execution of subprocesses (e.g., `apt`, `dpkg`). **Status: Fails.** The host parses the declared `subprocess` list but drops it when building the sandbox policy (`Executables: []string{p.Executable}`). Landlock blocks execution of any binaries other than the plugin itself.

## 6. Security Tests Performed
Adversarial scenarios were tested against the implementation logic:

1. **Plugin requests capability forbidden for its type:** Implemented correctly for `FilesystemWrite` (except the Capture bug above). **FAILED** for `Network` on Discovery and Capture plugins, which can declare `Network: true` without validation errors.
2. **Plugin declares undeclared filesystem path:** Enforced by Landlock on Linux. 
3. **Plugin attempts relative/symlink traversal:** Prevented by `sandbox.NormalizeManifestPaths` and kernel Landlock handling.
4. **Plugin writes executable payload and attempts execution:** Enforced. Landlock `WritePaths` expressly exclude `landlockAccessFSExecute`.
5. **Plugin invokes unauthorized subprocess:** Enforced, but excessively so (blocks authorized subprocesses too).
6. **Plugin attempts unauthorized networking:** Enforced by network namespaces.
7. **Plugin exceeds output limits:** Enforced via `limitedWriter` and `limitedBuffer`.
8. **Plugin times out:** Enforced via `context.WithTimeout` and process group kill.
9. **Plugin sends malformed protocol responses:** Fails safely (`json.Unmarshal` error).
10. **Plugin attempts resource exhaustion:** **FAILED (Windows/Fallback)**. `host.go` creates an unbounded `bytes.Buffer` for `Stderr`, allowing an adversarial plugin to OOM the host on non-Linux platforms. Linux restricts it to 1MB.

## 7. Results
The V1 Plugin API design is sound, and the Linux sandbox provides a strong OS-level isolation baseline. However, critical integration bugs in `host.go` prevent Capture plugins from working, block all legitimate subprocesses, and allow network access to plugin types that should be strictly offline.

## 8. Functional Gaps
- **Capture Plugin Staging Write Failure**: Capture plugins have no way to write artifacts to their staging location under the Linux sandbox because `Policy.WritePaths` is built statically from the manifest and does not include the dynamic `ArtifactLocation`.
- **Subprocess Declaration Ignored**: The `permissions.subprocess` list from the manifest is parsed but dropped in `buildPolicy`. As a result, Discovery and Restore plugins cannot execute necessary system binaries (e.g., package managers).

## 9. Security Gaps
- **Unauthorized Network Access**: The manifest validation logic in `host.go` does not reject `Network: true` for Discovery and Capture plugins, allowing them to bypass the strict offline requirement.
- **Stderr Memory Exhaustion (Fallback)**: Non-sandboxed invocations (`h.Invoke`) read `Stderr` into an unbounded `bytes.Buffer`, exposing the host to memory exhaustion DOS.

## 10. Test Coverage Gaps
- **Resource Exhaustion Limits**: `prlimit` constraints (memory, file size, process limits) are implemented in `sandbox_linux.go` but are completely untested.
- **Protocol Robustness**: There are no tests verifying behavior for timeouts, oversized responses, malformed JSON, excessive stderr, or sudden stdout closure.
- **Sandbox Execution**: `sandbox_integration_test.go` checks execution denial in writable paths but does not test execution of an allowed subprocess vs a denied subprocess.

## 11. Existing Workstream A Verification Status
The Linux Sandbox (`sandbox_linux.go`, `seccomp_linux.go`, `sandbox_integration_test.go`) was reviewed and its implementation remains untouched and frozen. The existing runtime verification and security boundaries established by Workstream A remain valid. The issues discovered lie primarily in how `host.go` translates manifests into sandbox policies.

## 12. Trust Phase Status
The Trust Phase (signature verification, trust hierarchy, user overrides) is **NOT IMPLEMENTED**. `host.go` defines `Signature` in the manifest structure but does not verify it during the `Discover` or `Validate` phases. 

## 13. Recommendations
1. Fix the Sandbox Policy generation to properly handle dynamic staging locations for Capture plugins.
2. Fix the Subprocess permission translation so authorized binaries are passed to Landlock.
3. Tighten manifest validation for Network capabilities on offline-only plugins.
4. Apply the `limitedBuffer` logic from the sandbox to the non-sandboxed `host.go` fallback.
5. Create a comprehensive suite of protocol robustness and resource limit tests.

## 14. Proposed Next Implementation Steps

| Priority | Problem | Affected Files | Proposed Solution | Required ADR? | Tests Required |
|---|---|---|---|---|---|
| 1 | Capture plugins cannot write to staging (Landlock EACCES) | `internal/plugin/host.go` | Update `InvokeSandboxed` to dynamically append the request's `ArtifactLocation` to the sandbox `Policy.WritePaths` for Capture plugins. | No | Unit tests for policy mutation during invocation |
| 2 | Subprocess lists are ignored | `internal/plugin/host.go` | Update `buildPolicy` to append `p.Manifest.Subprocesses` to `Policy.Executables`. | No | Unit tests for `buildPolicy` |
| 3 | Discovery/Capture Network bypass | `internal/plugin/host.go` | Add validation in `host.go` (`load` function) to reject `Network: true` for Discovery and Capture plugins. | No | Unit tests for `load()` |
| 4 | Stderr unbounded memory growth in `host.go` | `internal/plugin/host.go` | Replace `bytes.Buffer` in `h.Invoke` with `limitedWriter` bounded to 1MB. | No | Unit tests for OOM prevention |
| 5 | Missing Capability & Protocol Tests | `internal/plugin/host_test.go`, `internal/plugin/sandbox/sandbox_integration_test.go` | Implement a test suite in `host_test.go` exercising malformed responses, timeouts, and output limits. Add tests for `prlimit` enforcement in `sandbox_integration_test.go`. | No | Integration tests |
