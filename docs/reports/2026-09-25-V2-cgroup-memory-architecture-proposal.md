# V2 cgroup v2 Memory Architecture Proposal

**Date:** 2026-09-25
**Status:** Proposed
**Scope:** Plugin resource containment and telemetry for Linux cgroup v2

## 1. Executive Summary

Telos V1 uses `RLIMIT_AS` as a coarse virtual-address-space limit and relies on execution timeouts and process-group cleanup for plugin containment. V2 should add a dedicated cgroup v2 for every plugin invocation and use the cgroup v2 memory controller as the authoritative boundary for resident memory, swap, and memory-pressure behavior.

The proposed architecture is hybrid. Existing per-process limits remain useful for compatibility and early failure, while cgroup v2 provides hierarchical accounting and enforcement for the complete plugin subtree, including descendants and tasks created by helper processes. The supervisor creates and configures the cgroup before starting the plugin, moves the plugin into it, observes memory pressure and OOM events, and removes the cgroup after teardown.

The design uses `memory.high` as the first response to sustained pressure and `memory.max` as the hard ceiling. `memory.swap.max` is configured explicitly so swap cannot silently defeat the memory policy. `memory.oom.group` is enabled so an invocation is treated as one killable workload. Telemetry is collected from `memory.current`, `memory.peak`, `memory.events`, and swap counters, with the peak files read at invocation end.

## 2. Motivation and Problem Statement

A plugin can allocate memory through its own process, a child process, or a helper launched by a child. Process-local limits do not provide a single accounting boundary for that whole workload. Virtual address space is also not the same resource as resident memory: mappings, allocator behavior, file-backed pages, reclaim, and swap can make `RLIMIT_AS` a poor description of actual host memory pressure.

The V2 design must therefore answer four questions:

1. What is the memory budget for one plugin invocation?
2. How is pressure applied before the hard limit is reached?
3. What happens when the workload reaches the hard limit or is OOM-killed?
4. What evidence is retained for audit logs and operator diagnosis?

## 3. Goals

The proposal has these goals:

- Bound memory used by the complete plugin process subtree.
- Keep the supervisor outside the plugin's memory budget.
- Provide a soft-pressure signal before hard failure.
- Prevent swap from becoming an untracked extension of the budget.
- Make OOM behavior deterministic and attributable to one invocation.
- Capture stable end-of-invocation memory and event telemetry.
- Preserve the existing plugin API and work on hosts that support cgroup v2 delegation.
- Fail closed when a requested hard containment policy cannot be installed.

## 4. Non-Goals

This proposal does not define a general host-wide memory manager, a replacement for the kernel OOM killer, or a guarantee that every byte charged to a process is attributable to a single plugin. It does not use cgroups as a filesystem sandbox, a network sandbox, or a task-count limit; those are separate controls.

It also does not make `memory.min` or `memory.low` part of the plugin policy, and it does not require proactive reclaim during normal invocation. Those interfaces are described for completeness and possible future supervisor use.

## 5. Design Principles

The design follows these principles:

- **Contain the workload, not just the executable.** Every descendant must remain in the invocation cgroup.
- **Separate enforcement from observation.** `memory.max` and `memory.swap.max` enforce; telemetry files explain what happened.
- **Prefer explicit policy.** A configured swap policy must be visible in the cgroup rather than inferred from host defaults.
- **Treat pressure as a state transition.** `memory.high` is an actionable event, not merely a second hard limit.
- **Keep the supervisor trusted and outside the budget.** The supervisor must retain enough memory to collect evidence and clean up.
- **Use kernel semantics as the contract.** Behavior is defined by cgroup v2 documentation, not by a particular systemd version or distribution wrapper.

## 6. cgroup v2 Memory Interface

The memory controller is enabled for the invocation cgroup and exposes controller files in its cgroup directory.

- `memory.current` reports the current memory consumption charged to the cgroup. It is read-only and is used for live status and final telemetry.
- `memory.max` is the hard memory limit. A numeric value establishes the limit; `max` removes the limit. Reaching the limit causes reclaim and, if the cgroup cannot reduce usage, an OOM condition inside the cgroup.
- `memory.high` is a best-effort throttle threshold. Crossing it does not invoke the OOM killer. Tasks are throttled and direct reclaim is performed until usage falls or the condition is otherwise resolved.
- `memory.peak` reports the highest memory usage observed since creation or since the last reset. It is **read-write**, not read-only. A write of any non-empty string to the fd resets the peak to current usage for subsequent reads through that same fd. AERS can therefore open it once per plugin invocation, read it at invocation end, and reset it at the next invocation rather than snapshotting before and diffing after. The same rule applies to `memory.swap.peak`.
- `memory.events` reports counters for `low`, `high`, `max`, `oom`, and `oom_kill`. The counters are hierarchical and are suitable for detecting pressure and hard-limit transitions.
- `memory.events.local` reports the corresponding events generated by the cgroup itself rather than descendants. The implementation should prefer local counters when it needs to attribute an event specifically to the invocation cgroup.
- `memory.swap.current` reports current swap usage charged to the cgroup.
- `memory.swap.max` is the hard limit on swap usage. It is independent of `memory.max`; setting it to `0` prohibits additional swap usage by the cgroup, subject to the kernel's accounting and reclaim behavior.
- `memory.swap.peak` reports the peak swap usage and is **read-write**. A write of any non-empty string to the fd resets it to current usage for subsequent reads through that same fd.
- `memory.oom.group` controls OOM victim selection for the cgroup. When set to `1`, the kernel treats the cgroup as an indivisible workload for a cgroup OOM event and kills all eligible tasks in the cgroup together instead of selecting an arbitrary subset.

The supervisor must parse numeric files as bytes and event files as newline-delimited `key value` pairs. Unknown event keys must be ignored for forward compatibility.

### 6.1 Related Interfaces Not Used for Plugin Containment

Two related interfaces are worth recording even though they are not part of the original plugin limit proposal:

- `memory.min` and `memory.low` provide hierarchical memory protection. They are guarantees or preferences that memory in the protected hierarchy will not be reclaimed before less-protected memory; they are not limits. They are not needed for plugin containment, but could matter if AERS later needs to protect the supervisor's own cgroup.
- `memory.reclaim` is a write-only interface for triggering proactive reclaim on a live cgroup, for example `echo 1G > memory.reclaim`. It could support the administrator-manually-intervenes workflow described in Section 9 after a `memory.high` event.

## 7. Proposed Hierarchy and Lifecycle

For each invocation, the supervisor creates a child cgroup below a delegated Telos subtree, for example:

```text
/sys/fs/cgroup/telos/
  supervisor/
  invocations/
    <invocation-id>/
      plugin and descendants
```

The supervisor must create the directory, enable or verify the memory and pids controllers as required by the host policy, write controller settings, and set `memory.oom.group` before starting the plugin. The child process is then attached to the invocation cgroup before it can execute plugin code.

At completion, the supervisor stops the workload, collects telemetry while the files still exist, waits for all descendants to exit, and removes the invocation cgroup. Cleanup is retried only while the cgroup is empty; a non-empty cgroup is a lifecycle failure and must be recorded rather than silently orphaned.

## 8. Limit Configuration

The policy should expose separate values for:

- `MemoryMaxBytes`, mapped to `memory.max`.
- `MemoryHighBytes`, mapped to `memory.high`.
- `MemorySwapMaxBytes`, mapped to `memory.swap.max`.
- `DisableSwap`, a convenience setting that maps to `memory.swap.max = 0`.

`memory.max` must be greater than or equal to `memory.high` when both are finite. A policy with no hard memory limit is permitted only for an explicitly privileged administrative mode; ordinary plugin execution must have a finite limit. The configured swap limit must be recorded alongside the memory limit because the two values describe different resources.

The V1 `RLIMIT_AS` setting may remain enabled as a compatibility guard. It must not be described as equivalent to the cgroup memory limit, and a cgroup configuration error must not be hidden by successful `prlimit` setup.

## 9. Pressure, Throttling, and Administrator Intervention

The normal pressure path is:

1. Usage crosses `memory.high`.
2. The kernel throttles affected tasks and performs reclaim.
3. A `high` counter increment is observed by the supervisor.
4. The supervisor records the event and allows a bounded recovery window.
5. If usage reaches `memory.max`, the hard-limit and OOM behavior is recorded and the invocation is terminated according to the plugin failure contract.

A future administrative workflow may pause or otherwise manage an invocation after a `memory.high` event, inspect its telemetry, and write a reclaim request to `memory.reclaim` on the live cgroup. That is an operator intervention, not an automatic substitute for `memory.max`; the supervisor must still enforce the configured hard limit.

The initial implementation should not automatically lower limits in response to pressure. Limit changes made while a plugin is running can create surprising failures and must be controlled by an explicit policy or administrator action.

## 10. Telemetry and Accounting

At invocation start, the supervisor opens the peak files for the cgroup and records the configured limits. It should also read the initial values of `memory.current`, `memory.swap.current`, and the relevant `memory.events` counters.

At invocation end, while the cgroup still exists, it reads `memory.current`, `memory.peak`, `memory.swap.current`, `memory.swap.peak`, and `memory.events.local`. `memory.peak` is **read-write**, not read-only: writing any non-empty string to the fd resets it to current usage for subsequent reads through that same fd. AERS should therefore open `memory.peak` once per plugin invocation and read the peak at invocation end, rather than snapshotting before execution and diffing afterward. The same applies to `memory.swap.peak`.

Telemetry should include:

- Invocation identifier and plugin identifier.
- Configured `memory.high`, `memory.max`, and `memory.swap.max`.
- Final current memory and swap usage.
- Peak memory and peak swap usage.
- Local `high`, `max`, `oom`, and `oom_kill` deltas.
- Whether the invocation exited normally, timed out, hit a hard limit, or was OOM-killed.
- The kernel or host capability result if cgroup setup was unavailable.

Counters are monotonic within the lifetime of the cgroup. The supervisor should record both raw values and deltas from the initial read when the files are available.

## 11. OOM Behavior

With `memory.oom.group = 1`, a cgroup OOM is treated as an invocation-level failure. The supervisor must not report success merely because the plugin's main process exits while a descendant was killed. It should classify the result as memory exhaustion when `oom_kill` increases or when the process status and event counters jointly indicate a cgroup OOM.

The supervisor must retain enough memory outside the invocation cgroup to read event files, write the audit record, terminate remaining processes, and remove the cgroup. OOM handling must be idempotent because timeout cleanup and kernel OOM cleanup can race.

## 12. Startup and Failure Semantics

Cgroup setup is part of invocation startup, not best-effort telemetry. If the host requires V2 memory containment and the supervisor cannot create the cgroup, enable the controller, write the limits, or attach the process, the plugin must not start.

A deployment may provide an explicitly documented compatibility mode that runs without cgroup memory enforcement. That mode must be opt-in, visible in audit logs, and unavailable to policies that require hard containment. A missing controller, unavailable delegation, permission failure, malformed value, or controller write error must produce a distinct diagnostic.

## 13. Delegation and Host Prerequisites

The host must run a unified cgroup v2 hierarchy and delegate a subtree to the Telos service. The service account needs permission to create child cgroups, write the permitted controller files, attach processes, and remove empty children. The supervisor should not require unrestricted writes to the host root cgroup.

Systemd integration may provide the delegation boundary, but the core policy should operate on the cgroup filesystem contract. Installation and deployment documentation must identify the required controller delegation and explain that cgroup v2 is a host prerequisite for enforced V2 mode.

## 14. Process Attachment and Race Avoidance

The plugin must be placed in the invocation cgroup before it can create descendants. The preferred sequence is to create the cgroup, configure it, start the process with a small parent-controlled handoff, write the child's PID to `cgroup.procs`, and release the child only after attachment succeeds. The implementation must account for the fact that a process can fork before attachment if the handoff is not enforced.

All descendants remain in the same cgroup under normal cgroup semantics. The existing process-group and PID-namespace cleanup remains necessary because cgroups do not by themselves define the full process lifecycle contract used by Telos.

## 15. Interaction with Existing Sandbox Controls

Cgroup memory containment complements, rather than replaces, Landlock, seccomp, namespaces, timeouts, process-group cleanup, and `prlimit`.

- Landlock controls filesystem access.
- Network namespaces and seccomp control network and syscall exposure.
- PID namespaces and process-group cleanup control visibility and teardown.
- `RLIMIT_FSIZE` continues to bound file size.
- `RLIMIT_AS` can provide an early per-process compatibility guard.
- cgroup v2 accounts for the complete invocation and supplies memory pressure and OOM signals.

The order of setup must ensure that sandbox restrictions and cgroup attachment are complete before untrusted plugin code runs.

## 16. Compatibility and Portability

The architecture is Linux-specific because cgroup v2 is a Linux kernel interface. On Windows and macOS, the existing platform-specific resource and isolation behavior remains in effect until equivalent native implementations are designed.

Linux hosts without cgroup v2 or without the required delegation can use the explicitly documented compatibility mode only where policy permits it. Feature detection must inspect the mounted hierarchy and controller files rather than assuming that a distribution name implies support.

## 17. API and Configuration Changes

The plugin API need not expose cgroup file paths. Host configuration should expose resource intent through named fields such as `MemoryHighBytes`, `MemoryMaxBytes`, and `MemorySwapMaxBytes`. Defaults belong in host policy, not plugin manifests, so an untrusted plugin cannot raise its own budget.

The invocation result and audit record should gain a resource outcome containing the configured limits, observed peaks, event counters, and a normalized failure reason. Existing callers that do not inspect the additional result fields remain source-compatible where practical.

## 18. Security Analysis

The design reduces the impact of memory exhaustion by placing all plugin descendants under one kernel-enforced boundary and by making hard-limit and OOM outcomes auditable. `memory.oom.group` prevents partial survival from being mistaken for a clean invocation.

Important residual risks remain: privileged host administrators can alter cgroups; kernel accounting has documented special cases; a plugin can still consume CPU, file descriptors, or disk unless those resources have separate controls; and the supervisor must protect its own cgroup. A cgroup is not a substitute for the existing sandbox perimeter.

The main security failure to avoid is a false claim of enforcement. V2 mode must refuse to start a plugin when the requested cgroup policy was not successfully installed.

## 19. Rollout and Verification Plan

Implementation should proceed in stages:

1. Add a cgroup v2 capability probe and a testable filesystem abstraction.
2. Create and configure an invocation cgroup without changing the default execution path.
3. Attach a fixture process and verify limits, descendants, cleanup, and event parsing.
4. Integrate the lifecycle with plugin execution and audit telemetry.
5. Add Linux integration tests for high pressure, hard-limit OOM, swap policy, timeout cleanup, and supervisor survival.
6. Enable enforced V2 mode through an explicit configuration or deployment profile.

Tests must run on a real cgroup v2 Linux environment with suitable delegation. Unit tests can use a fake cgroup filesystem, but they cannot prove kernel enforcement. Integration tests must verify that `memory.peak` and `memory.swap.peak` reset correctly when written through an already-open descriptor and that the next read reports the post-reset baseline.

## 20. Conclusion

A dedicated cgroup v2 per plugin invocation is the appropriate V2 architecture for enforcing and observing plugin memory usage. The hybrid design preserves existing safeguards while adding hierarchical accounting, soft pressure through `memory.high`, hard containment through `memory.max`, explicit swap control, group OOM behavior, and useful telemetry.

The proposal is intentionally conservative about failure: when enforced V2 mode cannot be configured, the invocation does not start. This keeps the memory policy meaningful and gives Telos a clear path from the V1 partial controls to a complete, auditable Linux resource boundary.

## Verification

The cgroup v2 memory interface semantics (`memory.high`, `memory.max`, `memory.current`, `memory.peak`, `memory.events`, `memory.oom.group`, and swap controls) were verified on 2026-09-25 against the current authoritative kernel documentation at https://docs.kernel.org/admin-guide/cgroup-v2.html rather than relying on secondary sources. All claims in this document were confirmed accurate as of that fetch, with the `memory.peak` writability correction noted above.
