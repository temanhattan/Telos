# V1 First-Party Plugin Gap Design Resolution

> **Date:** 2026-08-31
> **Type:** Design Resolution Report
> **Focus:** Architectural decisions and evidence prior to implementation

This report translates the confirmed findings from the V1 capability audit into actionable architectural and implementation designs. No code changes have been made. All findings below are based on strict source and documentation tracing.

## 1. Capture Staging Path

**Classification:** ARCHITECTURAL DECISION REQUIRED / CONFIRMED BUG

* **Documentation Evidence:** `05_Plugin_API.md` (line 338) states: "For file copy and configuration export captures, the plugin writes the artifact to a staging location designated by the Plugin Host in the request context." However, the `CaptureRequest` schema (line 306) omits any such field. The document also states (line 345): "Core reads from staging ... Core cleans up staging after successful storage."
* **Source Evidence:** `host.go` only passes `Manifest.FilesystemWrite` to the sandbox `WritePaths`, which is empty for Capture plugins. Thus, Capture plugins receive `EACCES` when writing artifacts. (CODE VERIFIED)
* **Analysis:** The staging directory's lifecycle is clearly owned by the Core Engine (S7), which reads and cleans it up. The Plugin Host (S12) merely acts as the conduit. The plugin must never control this location.
* **Proposed Safe Solution:** Described in **ADR-0013**. `CaptureRequest` is updated to include `staging_location`. The Core Engine creates a secure, trusted temporary directory. The Plugin Host maps this path to `Policy.WritePaths`. This preserves the invariant that a request-controlled path MUST NOT become a Landlock WritePath without trusted allocation.

## 2. Subprocess Permission Semantics

**Classification:** FUNCTIONAL GAP / ARCHITECTURAL DECISION REQUIRED

* **Documentation Evidence:** `05_Plugin_API.md` (lines 135, 646) specifies `subprocess: string[]` as "Executable names the plugin may invoke (e.g., [\"apt\", \"dpkg-query\"])."
* **Source Evidence:** `host.go` (line 308) ignores `p.Manifest.Subprocesses` when building the policy, explicitly hardcoding `Executables: []string{p.Executable}`. (CODE VERIFIED)
* **Analysis:** Passing `"apt"` directly to Landlock fails because it requires absolute paths. Furthermore, Landlock requires execute permissions for dynamic dependencies (like the ELF interpreter). 
* **Proposed Safe Solution:** Described in **ADR-0007**. The Host validates that requested subprocesses are basenames (no slashes), resolves them via `exec.LookPath()`, and maps the absolute paths to `Policy.Executables`. To satisfy runtime dependencies, the Host also maps standard system libraries (`/lib`, `/lib64`, `/usr/lib`, `/usr/lib64`) to the sandbox with Read and Execute rights.

## 3. Resource Limit Semantics

**Classification:** FUNCTIONAL GAP / ARCHITECTURAL DECISION REQUIRED

* **Documentation Evidence:** `05_Plugin_API.md` (line 696) and `03_Threat_Model.md` (line 598) declare that resource limits (memory, file size, max processes) are enforced via `prlimit`.
* **Source Evidence:** `sandbox_linux.go` implements `applyResourceLimits`. However, `host.go` never populates these fields in the `Policy` struct, leaving them at `0`. (CODE VERIFIED)
* **Analysis:** There is no documentation in the V1 specs specifying what these defaults should be, nor are they exposed in the `HostOptions` configuration or the plugin manifest.
* **Proposed Safe Solution:** Described in **ADR-0012**. We do not invent arbitrary numeric limits without an ADR. ADR-0012 establishes standard V1 Host-level defaults (e.g., 512MB RAM, 32 processes) that the Host will populate into the `sandbox.Policy`.

## 4. Network Validation

**Classification:** SECURITY GAP

* **Documentation Evidence:** `05_Plugin_API.md` strictly prohibits network access for Discovery, Classification, and Capture plugins.
* **Source Evidence:** `host.go:load()` contains validation for `FilesystemWrite` and `ClassificationRule` constraints, but **no validation** exists to reject `Network: true` for `Discovery` or `Capture` plugins. If a Discovery plugin maliciously declares `Network: true`, it is passed to the sandbox and `CLONE_NEWNET` isolation is bypassed. (CODE VERIFIED)

| Plugin Type | Network Allowed by V1 | Current Host Validation | Current Sandbox Behavior | Existing Test | Verdict |
|---|---|---|---|---|---|
| **Discovery** | Denied | Allowed | ALLOWED | No | 🔴 SECURITY GAP (CODE VERIFIED) |
| **Classification** | Denied | Enforced | DENIED | Yes | ✅ TEST VERIFIED |
| **Capture** | Denied | Allowed | ALLOWED | No | 🔴 SECURITY GAP (CODE VERIFIED) |
| **Restore** | Denied (default) | Enforced | DENIED | No | ✅ CODE VERIFIED |
| **Storage** | Declared | Enforced | ALLOWED | No | ✅ CODE VERIFIED |

* **Proposed Safe Solution:** Add the exact validation in `host.go:load()` to return an error if `mf.Permissions.Network` is true for offline-only plugin types.

## 5. Stderr Resource Bound

**Classification:** SECURITY GAP

* **Documentation Evidence:** `08_PROJECT_CONTEXT.md` (line 160) lists "output size cap" as a defense against DoS.
* **Source Evidence:** `host.go:Invoke()` (the non-sandboxed fallback for Windows/macOS) uses an unbounded `bytes.Buffer` for `cmd.Stderr`. An attacker-controlled plugin printing indefinitely to stderr will cause an Out-Of-Memory panic in the host. `sandbox_linux.go:Exec()` correctly bounds stderr to 1MB using `limitedBuffer`. (CODE VERIFIED)
* **Proposed Safe Solution:** Extract `limitedBuffer` from the `sandbox` package into a shared utility (or recreate it in `plugin`) and use it in `h.Invoke()` with a hardcoded 1MB limit for stderr and the `OutputLimit` for stdout.

## 6. Protocol Robustness

**Classification:** TEST GAP

* **Documentation Evidence:** `05_Plugin_API.md` (line 815) specifies that malformed responses must be rejected.
* **Source Evidence:** Code inspection confirms that `host.go` safely truncates oversized output (via `limitedWriter`), kills timed-out processes (`context.WithTimeout`), and handles malformed JSON (`json.Unmarshal`). (CODE VERIFIED)
* **Verdict:** NOT TESTED. Negative test cases simulating timeouts, oversized stdout payloads, and invalid JSON responses are missing.

## Recommended Implementation Order

1.  **Network Validation** (Easy fix: update `host.go:load` with missing validation).
2.  **Stderr Resource Bound** (Extract `limitedBuffer` and apply to `host.go:Invoke`).
3.  **Subprocess Translation** (Update `buildPolicy` to resolve and append allowed subprocesses and system library directories per ADR-0007).
4.  **Resource Limits** (Populate `Policy` fields in `host.go` per ADR-0012).
5.  **Capture Staging Path** (Implement ADR-0013).
6.  **Test Coverage** (Add protocol robustness and capability runtime tests).
