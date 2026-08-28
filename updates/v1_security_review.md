# V1 Security & Architecture — Documentation-First Adversarial Review

## Phase 0: Discovery
**Documentation and Source Files Reviewed:**
- `docs/00_Vision.md`
- `docs/01_Requirements.md`
- `docs/02_Architecture.md`
- `docs/03_Threat_Model.md`
- `docs/05_Plugin_API.md`
- `internal/plugin/host.go`
- `internal/plugin/sandbox/paths.go`
- `internal/plugin/sandbox/policy.go`
- `internal/plugin/sandbox/sandbox_linux.go`
- `internal/plugin/sandbox/sandbox_other.go`
- `internal/plugin/sandbox/seccomp_linux.go`

## Phase 1: Security Model
**Goal:** The stated goal of the V1 Sandbox is to securely isolate untrusted plugins from the core system. The Plugin Host (S12) coordinates plugin discovery and execution, but relies on the sandbox to enforce strict boundaries. Plugins must not be able to read or write files outside their declared paths, access the network without explicit permission, invoke arbitrary subprocesses, or escalate privileges on the host system.

## Phase 2: Attacker Model
**Capabilities:** The attacker is assumed to control a malicious or compromised plugin package (manifest and executable). They can attempt to exploit sandbox weaknesses to escape their declared permission scope, exfiltrate data, consume excessive host resources, or return malformed JSON responses to exploit parsing vulnerabilities in the host. The attacker does NOT have prior access to the host machine or Telos configuration files.

## Phase 3: Trust Boundaries
**Boundary:** Plugin Host ↔ Plugins (TB-3).
- **Data Crossing:** Validated requests are sent to the plugin via stdin; responses are read via stdout.
- **Enforcement:** Subprocess isolation, OS-level sandbox enforcement (Landlock, seccomp, namespaces), permission declaration validation, and output limits. The host does not trust the plugin and validates all incoming responses.

## Phase 4: Manifest Security
**Validation Checks (`host.go`):**
- Ensures `interface_version` matches the host.
- Validates that `type` is one of the recognized plugin types.
- Restricts capabilities based on type (e.g., Classification plugins cannot request filesystem, network, or subprocess permissions).
- Ensures `filesystem_write` is only requested by `Restore` or `Storage` plugins.
- Resolves the `executable` path to ensure it is relative to the plugin directory and does not contain traversals (`..`).

## Phase 5: Landlock
**Enforcement (`sandbox_linux.go`):**
- Operates on a fail-closed paradigm: if Landlock ABI v1 is unavailable, execution is refused.
- Applies `RO` (read-only) rules for all paths in `ReadPaths` and `Executables`.
- Applies `RW` (read-write) rules for all paths in `WritePaths`.
- ABI v1 handles core filesystem access; ABI v2/v3 (if available) add `REFER` and `TRUNCATE` restrictions.

## Phase 6: Seccomp
**Filter (`seccomp_linux.go`):**
- Implements a **denylist**, functioning as a defense-in-depth measure rather than full syscall confinement.
- Explicitly blocks dangerous syscalls like `ptrace`, `mount`, `reboot`, `setns`, and `unshare`.
- Checks `clone` and `clone3` flags to block any namespace-creation variants (`CLONE_NEW*`).
- Violations return `EPERM` rather than raising `SIGKILL`, allowing for graceful error handling.

## Phase 7: Namespaces
**Network & PID:**
- Enforced via `SysProcAttr.Cloneflags`.
- `CLONE_NEWPID` isolates the PID namespace, preventing the plugin from seeing or signaling host processes.
- `CLONE_NEWNET` is applied when `policy.Network` is `false`, removing all network connectivity (loopback is down).

## Phase 8: Privilege/Process
**Limits & Escalation Prevention:**
- Invokes `unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)` prior to seccomp to block `setuid` escalation.
- Uses `prlimit` to enforce resource boundaries: `RLIMIT_AS` for memory, `RLIMIT_NPROC` for process limits (fork bombs), and `RLIMIT_FSIZE` for maximum file size.
- Implements a bounded execution timeout and byte-limited stdout/stderr writers (`limitedWriter`) to prevent resource exhaustion.

## Phase 9: Executable Trust
**Safe Invocation:**
- The plugin executable is identified during manifest loading and strictly bound to the plugin's directory.
- The Linux sandbox uses a supervisor/helper pattern: the Telos binary re-execs itself with `_TELOS_SANDBOX=1`, reads the policy from `stdin`, applies restrictions, and then uses `syscall.Exec` to replace the helper image with the plugin executable.

## Phase 10: FS R/W
**Read vs Write paths enforcement:**
- Manifest permissions are mapped directly to `Policy.ReadPaths` and `Policy.WritePaths`.
- Landlock applies independent access rights for RO (read/execute) and RW (write/remove/make).

## Phase 11: TOCTOU
**Symlinks and Race Conditions:**
- Because Landlock evaluates rules based on underlying inodes rather than string paths, it inherently mitigates many TOCTOU vulnerabilities associated with path resolution during execution.
- However, the current Linux implementation does not resolve symlinks or normalize paths before handing them to the kernel (see Phase 15).

## Phase 12: Symlink/Hardlink
**Normalization and Traversal:**
- The `paths.go` file contains robust `NormalizePath` and `ValidatePaths` functions designed to resolve symlinks, verify absolute paths, and catch `..` traversals.

## Phase 13: Cross-platform
**Fallback Behavior (`sandbox_other.go`):**
- Non-Linux platforms use application-level enforcement only.
- Validates paths using `paths.go`, limits output, and enforces timeouts.
- Does **not** provide OS-level isolation (no syscall filtering, no filesystem enforcement, no namespaces).
- Emits an `audit` severity warning to log the degraded security state.

## Phase 14: Security Invariants
**Non-negotiable Rules:**
- The sandbox must fail closed on Linux if Landlock ABI v1 is absent.
- `PR_SET_NO_NEW_PRIVS` must be applied before installing the seccomp filter.
- The seccomp filter must be applied *after* Landlock setup, to avoid interfering with Landlock's required syscalls.
- Manifest permissions that conflict with the plugin's type must cause immediate load failure.

## Phase 15: Doc vs Implementation Audit
**Critical Discrepancies Identified:**
1. **Missing Path Validation on Linux:** The documentation states that "The Plugin Host validates all declared permissions", and `paths.go` exists to enforce this. However, `ValidatePaths` is **only called in `sandbox_other.go`**. The Linux implementation (`sandbox_linux.go`) and the Host (`host.go`) **never** call `ValidatePaths` or `NormalizePath`. Consequently, a plugin on Linux can declare relative paths or paths with `..` traversals, and they will be passed directly to Landlock without application-level normalization or validation.

## Phase 16: Lint-Fix Review
**Context:**
- The code properly documents security-relevant `#nosec` pragmas (e.g., `G204` for `exec.CommandContext` and `G103` for `unsafe.Pointer`).
- No lint failures directly caused the issues identified, but the unused `NormalizePath` function and isolated use of `ValidatePaths` point to an architectural gap that linters would not inherently flag as a security vulnerability.

## Phase 17: Adversarial Matrix
**STRIDE Analysis for V1 Sandbox:**
- **Spoofing:** Mitigated by duplicate ID checks and (future) signature verification.
- **Tampering:** Partially mitigated by Landlock, but exposed to path traversal vulnerabilities due to the missing path validation on Linux (see Phase 15).
- **Repudiation:** Sandboxed actions, degraded fallbacks, and violations are logged at `audit` severity.
- **Information Disclosure:** Network and PID namespaces prevent sniffing; seccomp prevents `ptrace`.
- **Denial of Service:** Mitigated by `prlimit` and application-level timeouts/output truncation.
- **Elevation of Privilege:** Landlock, seccomp, `CLONE_NEW*` blocking, and `NO_NEW_PRIVS` form a robust defense, assuming kernel integrity.

## Phase 18: Decisions Required
**Questions for the Architect / Security Team:**
1. **Path Validation Remediation:** Should `ValidatePaths` be integrated directly into `host.go` (e.g., within `buildPolicy`) so that all platforms benefit from path normalization and traversal checks before reaching the sandbox backends?
2. **Seccomp Evolution:** Given the reliance on a denylist for V1, what is the roadmap for introducing strict allowlist seccomp profiles for each specific plugin type?
3. **Symlink Resolution:** Should `NormalizePath` be applied to all manifest-declared paths prior to executing Landlock rules to prevent TOCTOU symlink manipulation within the plugin directory?

## Phase 19: STOP: DO NOT IMPLEMENT
**Conclusion:**
This concludes the documentation-first adversarial review of the Telos V1 Sandbox. In strict adherence to the instructions, **no implementation or code modification has been performed.** The findings, specifically the architectural gap in Linux path validation, are recorded here for review by the security engineering team.
