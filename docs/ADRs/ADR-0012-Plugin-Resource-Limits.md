# Title

Plugin Resource Limits and Process Isolation Defaults

# Status

Accepted (Revised 2026-09-30)

# Context

The V1 sandbox utilizes `prlimit` on Linux to enforce selected resource limits
(`RLIMIT_NPROC`, `RLIMIT_FSIZE`, and optionally `RLIMIT_AS`). The API documentation
and Manifest Schema must define the zero-value semantics for each limit explicitly.

A malicious or buggy plugin could consume excessive CPU, memory, or disk space, degrading host performance (Threat D-02). We must define standard V1 numeric defaults. We must not invent random defaults at implementation time without architectural consent.

### Re-evaluation of `RLIMIT_NPROC`

The original draft of this ADR proposed a default of `MaxProcesses = 32` via `RLIMIT_NPROC` to prevent fork bombs. A technical investigation and empirical verification against the Linux kernel and Go runtime identified that `RLIMIT_NPROC` cannot provide the required security property in the current Telos V1 architecture:

1. **Accounting Unit (Threads vs. Processes):**
   In Linux, `RLIMIT_NPROC` restricts all `task_struct` instances created via `clone()` or `fork()`. Calling `clone()` with `CLONE_THREAD` (used by Go runtime thread creation and `pthread_create`) increments the user process counter. `RLIMIT_NPROC` is a task/thread limit, not merely a child process limit.

2. **Accounting Scope (Global Real UID):**
   `RLIMIT_NPROC` is evaluated against `p->real_cred->user->processes`. It is tracked globally across the entire operating system for the calling process's **Real UID**, not per process tree, process group, or PID namespace. Because Telos V1 does not utilize user namespaces (`CLONE_NEWUSER`) or ephemeral UIDs, the plugin shares the Real UID of the host. If an unprivileged user has existing host processes (shells, background services, editor), those tasks consume the quota, causing the plugin to fail on startup with `EAGAIN`.

3. **Bypass Under Root Execution (`INIT_USER`):**
   In `kernel/fork.c`, `is_rlimit_overlimit()` explicitly skips enforcement if `p->real_cred->user == INIT_USER` (`uid=0`) or if the caller holds `CAP_SYS_ADMIN` or `CAP_SYS_RESOURCE`. Because Telos V1 currently requires root / `CAP_SYS_ADMIN` to create PID and Network namespaces (`CLONE_NEWPID`, `CLONE_NEWNET`), `RLIMIT_NPROC` is a complete no-op in actual execution. It provides zero protection against fork bombs under root.

4. **Go Runtime & Subprocess Incompatibility:**
   The Go runtime dynamically creates OS threads ($M$s) to service logical processors (`GOMAXPROCS`, defaulting to host CPU count) and blocking system calls (e.g., waiting on subprocesses, pipe I/O). If a thread creation fails with `EAGAIN`, Go aborts immediately with `fatal error: runtime: failed to create new OS thread`. Subprocesses declared via ADR-0007 (such as `apt` and `dpkg`) spawn multiple helper daemons, transport workers, signature verifiers, and shell pipelines. A low static limit like 32 causes non-deterministic fatal crashes on high-core machines or active workloads.

5. **Required Security Property vs. `RLIMIT_NPROC` Classification:**
   Telos requires:
   > *"Limit plugin-created tasks/processes without depending on unrelated host processes owned by the same UID."*

   `RLIMIT_NPROC` structurally **cannot** provide this property in the V1 architecture. Therefore, in Telos V1, `RLIMIT_NPROC` is explicitly classified as:
   - **NOT a per-plugin process-count security boundary**
   - **NOT a sandbox isolation primitive**
   - **NOT a fork-bomb containment boundary**
   - **Affected by Real UID accounting** across all host processes
   - **Bypassed for real UID 0 / applicable capabilities** (`CAP_SYS_ADMIN`, `CAP_SYS_RESOURCE`)

Arbitrarily raising the limit (e.g., to 512 or 1024) does not convert it into a security boundary: it remains un-enforced under root and subject to Real UID cross-talk under non-root.

### Architectural Taxonomy of Enforcement Primitives

To prevent category errors during design and implementation, the sandbox distinguishes four orthogonal mechanisms:

- **Resource Limits (`prlimit` / `RLIMIT_*`):** Instantaneous capacity caps on specific OS resources for a process and its descendants (e.g., `RLIMIT_AS` limits address space; `RLIMIT_FSIZE` limits file size).
- **Execution Timeout (`context.WithTimeout` / `TimeoutSec`):** Bounds the temporal wall-clock duration of an execution. It limits how long runaway behavior can persist, but does not limit instantaneous resource consumption during that window.
- **Process-Group Cleanup (`Setpgid: true` + `SIGKILL`, PID namespace teardown):** Guarantees lifecycle teardown of child/orphan tasks when the execution concludes or times out. It prevents orphan task survival after termination, but does not bound instantaneous task concurrency while running.
- **Task-Count Isolation (`cgroups v2 pids.max`):** Instantaneously bounds the number of concurrent tasks (threads and processes) within a dedicated control hierarchy, independent of UID or execution lifetime.

# Decision

1. **Process Limit (`MaxProcesses` / `RLIMIT_NPROC`):**
   - The V1 default shall be **`MaxProcesses = 0` (disabled / unconstrained)**.
   - Setting a non-zero default like `32` via `RLIMIT_NPROC` is explicitly rejected.
   - `RLIMIT_NPROC` remains available only as an **optional administrative compatibility limit** if an operator explicitly configures a positive integer in `HostOptions`. Host documentation must explicitly note that this applies to the entire Real UID on the host and is bypassed if running as root.

2. **Cgroups v2 (`pids.max`) as the Architectural Boundary (Deferred to V2):**
   - The cgroups v2 `pids` controller (`pids.max`) is the proper OS primitive to enforce true per-plugin task-count isolation independent of UID and capable of constraining root.
   - Consistent with the V2 roadmap and Threat Model item FSI-9, cgroups v2 integration is designated as **V2 / future architecture**. It is not required for V1 to prevent adding host system prerequisites (cgroup delegation, systemd integration, root cgroupfs writes).
   A detailed hybrid architecture proposal for this V2 work is recorded in docs/reports/2026-09-25-V2-cgroup-memory-architecture-proposal.md.

3. **Standard V1 Defaults for Other Resources:**
   Existing V1 documentation (`05_Plugin_API.md`, `03_Threat_Model.md`) specifies resource limits without fixed values. The following defaults are accepted for V1:
   - `MemoryBytes` (`RLIMIT_AS`): **0 (disabled by default)**. A positive value remains an optional administrative or plugin-specific virtual-address-space override. V1 does not provide physical-memory (RSS) containment for invocations without an explicit override.
   - `MaxFileSizeBytes` (`RLIMIT_FSIZE`): **10 GB** (10,737,418,240 bytes). Accommodates large backup artifacts during Capture, while bounding disk exhaustion.
   - `OutputLimit` (stdout): **4 MB**. Enforced via `limitedBuffer` to bound JSON response deserialization in memory.
   - `StderrLimit`: **1 MB**. Enforced via `limitedBuffer` to prevent unhandled error streams from causing host OOM.

4. **Memory limit semantics:**
   - `MemoryBytes == 0` disables `RLIMIT_AS`.
   - Negative `MemoryBytes` values normalize to `0` and therefore also disable `RLIMIT_AS`.
   - Positive `MemoryBytes` values are preserved and applied as explicit administrative or plugin-manifest overrides.
   - The Linux sandbox skips `setrlimit(RLIMIT_AS, ...)` when the policy value is non-positive.

5. **Application:**
   The Plugin Host must populate the `sandbox.Policy` with these configuration values on every invocation.

The default change is based on the archived empirical investigation in
`docs/reports/2026-09-25-RLIMIT_AS-empirical-investigation.md` and the V2 cgroup
memory architecture proposal in
`docs/reports/2026-09-25-V2-cgroup-memory-architecture-proposal.md`.

# Consequences

**Benefits:**
- Eliminates a false sense of security: removes an ineffective mechanism that claimed to protect against fork bombs but was completely bypassed under root.
- Prevents brittle, environment-dependent crashes of Go plugins and package managers on developer workstations and multi-core machines.
- Maintains a clean, dependency-free V1 architecture without requiring cgroup delegation setup.
- Retains robust defense-in-depth against runaway processes via process group `SIGKILL` on timeout, PID namespace teardown on exit, and strict execution deadlines.
- Activates valid resource constraints (`RLIMIT_FSIZE`, output caps, and explicit
  `RLIMIT_AS` overrides) to mitigate Threat D-02.

**Trade-offs:**
- During the execution window prior to timeout expiry, V1 does not have an instantaneous task-count ceiling; runaway process generation under root or unconstrained non-root can consume host PID slots until the execution times out and is torn down.
- V1 does not provide physical-memory (RSS) containment for plugins using the
  default `MemoryBytes = 0`. Wall-clock timeout, output bounds, namespace
  teardown, and `RLIMIT_FSIZE` remain active protections, but none of them
  bounds instantaneous RSS. Physical-memory containment is deferred to the V2
  cgroup v2 memory architecture.
- True, instantaneous task-count bounding is deferred to V2 cgroups v2 integration.
