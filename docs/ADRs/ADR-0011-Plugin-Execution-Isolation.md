# Title

Plugin Execution Isolation Strategy

# Status

Accepted

# Context

NFR-1.11 requires plugin sandboxing. Threat E-01 rates sandbox escape as Critical impact. The Plugin API specification (§Sandbox and Permission Model) deliberately leaves the enforcement technology as an implementation decision. The project needs a strategy that provides genuine security guarantees while keeping the architecture extensible to future platforms.

# Decision

V1 uses Linux as the first fully supported sandbox platform, providing OS-level plugin isolation using:

- **Landlock LSM** for kernel-enforced filesystem access control (ABI v1 minimum, fails closed if unavailable).
- **seccomp-BPF** for system call restriction (defense-in-depth denylist — not complete syscall confinement).
- **Linux namespaces** (PID, network) for process and network isolation.
- **seccomp clone/clone3 flag filtering** to prevent namespace creation by plugins (`setns` and `unshare` denied outright; `clone`/`clone3` allowed for normal threading but `CLONE_NEW*` flags denied).
- **`PR_SET_NO_NEW_PRIVS`** for privilege escalation prevention.
- **`prlimit`** for resource constraints (including `RLIMIT_NPROC` for fork bomb prevention — not as a namespace-creation security boundary).
- **Process-group termination** for timeout enforcement.

The Plugin Host uses a platform-agnostic `Sandbox` interface. The Linux backend is the first implementation. Non-Linux platforms receive a degraded fallback with explicit audit-level warnings.

## seccomp Architecture

The seccomp filter uses a **denylist** of dangerous syscalls rather than a strict allowlist, because plugins may be heterogeneous executables (shell scripts, Python, compiled binaries) with diverse syscall requirements.

**This is explicitly defense-in-depth, not a complete syscall confinement boundary.** The term "OS-level isolation" refers to the combined effect of Landlock, namespaces, and seccomp — it must not be read as "full syscall confinement."

For namespace-creation specifically:
- `setns` and `unshare` are denied outright.
- `clone` and `clone3` are allowed for normal thread/process creation, but the BPF filter evaluates their flags and denies calls with any `CLONE_NEW*` flag set.
- `RLIMIT_NPROC` is a resource limit (fork bomb prevention), not a namespace-creation restriction.

## Landlock Baseline

V1 requires Landlock ABI v1 as the minimum security baseline. If the kernel does not support ABI v1, sandboxed plugin invocation **fails closed** — the plugin is not executed, and an `audit`-severity error is logged. Stronger ABIs (v2+) are used when available for above-baseline capabilities, but weaker-than-baseline enforcement never silently passes. `BestEffort()` is not used for core filesystem access control — only for above-baseline capabilities (e.g., "try to restrict truncation with ABI v3, but don't fail if only v1 is available").

## Supervisor/Re-exec Architecture

Landlock restrictions are applied using the supervisor/re-exec pattern: AERS spawns a sandbox helper process (via `_AERS_SANDBOX=1` env var) that applies Landlock + seccomp to itself, then exec's the plugin executable. This keeps the AERS host process unrestricted. The sandbox helper validates its invocation strictly: verifies the env var, reads and validates the Policy from stdin, and fails closed on any malformed input.

## Why Linux-First

1. Linux provides the strongest available unprivileged sandboxing primitives (Landlock, seccomp, namespaces) — all operable without root on modern kernels.
2. The project's long-term deployment targets include servers and cloud environments, which are predominantly Linux.
3. Windows is not currently a product requirement for V1.
4. Building the security model on a platform with strong OS-level primitives validates the isolation architecture properly. A weaker Windows-first approach would produce a V1 that cannot demonstrate real isolation.
5. The platform-agnostic interface ensures a future Windows backend can be added without architectural changes.

# Alternatives Considered

1. **Windows-first constrained execution.**
   *Rejected because* Windows lacks unprivileged sandbox primitives comparable to Landlock/seccomp. V1 would only provide application-level enforcement — not genuine OS isolation.

2. **Cross-platform weak V1.**
   *Rejected because* it produces security theater — the system claims sandboxing but provides only path string checks.

3. **Full cross-platform sandbox V1.**
   *Rejected because* implementing strong isolation on both Linux and Windows simultaneously is too complex for V1.

4. **seccomp allowlist.**
   *Rejected for V1 because* an allowlist is impractical for heterogeneous plugin executables. Would break shell scripts, Python plugins, and other interpreted languages. Reserved for V2 per-plugin-type profiles.

5. **`RLIMIT_NPROC` as namespace restriction.**
   *Rejected because* `RLIMIT_NPROC` is a resource limit, not a security boundary for namespace creation. seccomp flag filtering on clone/clone3 is the correct mechanism.

6. **`BestEffort()` for Landlock baseline.**
   *Rejected because* silent degradation below the security baseline is not acceptable. `BestEffort()` may only be used for above-baseline capabilities.

7. **Linux-first strong isolation.**
   *Accepted because* it proves the security architecture works with genuine kernel-enforced isolation. The clean `Sandbox` interface allows future backends without architectural changes.

# Consequences

**Benefits:**
- V1 provides strong OS-level isolation on Linux ≥5.13 through Landlock, namespaces, and seccomp.
- If the kernel cannot provide the minimum security baseline (Landlock ABI v1), sandboxed plugin invocation fails closed — plugins are not executed.
- The platform-agnostic `Sandbox` interface supports future backends (Windows Job Objects, container-native) without architectural changes.

**Trade-offs:**
- Sandbox escape remains a critical-impact threat; the actual residual risk depends on kernel integrity, correct configuration, and implementation correctness. A definitive risk rating requires independent security review, not merely an implementation plan.
- Windows developers must use WSL2 or a Linux VM for sandbox-enforced plugin testing.
- Non-Linux platforms operate with degraded security and explicit warnings.
- seccomp uses a denylist (defense-in-depth), not complete syscall confinement. This is documented throughout the codebase and specifications.
- Future Windows backend is architecturally supported but not V1 scope.

# Implementation Notes

- `internal/plugin/sandbox/policy.go`: Platform-agnostic `Policy` and `Sandbox` interface types.
- `internal/plugin/sandbox/sandbox_linux.go`: Linux V1 backend implementing the supervisor/re-exec pattern.
- `internal/plugin/sandbox/sandbox_other.go`: Non-Linux fallback with application-level enforcement.
- `internal/plugin/sandbox/seccomp_linux.go`: seccomp BPF filter construction with denylist and clone flag filtering.
- `internal/plugin/sandbox/paths.go`: Path normalization and validation utilities.
- `internal/plugin/host.go`: Integrated with `Sandbox` interface for plugin invocation.
- `cmd/aers/main.go`: Sandbox helper entry point via `_AERS_SANDBOX=1` detection.

# Related Documents

- [Requirements](../01_Requirements.md): NFR-1.11
- [Architecture](../02_Architecture.md): S12 Plugin Host, §Security Perimeter
- [Threat Model](../03_Threat_Model.md): E-01, SA-4, FSI-9
- [Plugin API](../05_Plugin_API.md): §Sandbox and Permission Model
