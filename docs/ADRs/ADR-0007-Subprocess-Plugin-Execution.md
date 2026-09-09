# Title

Subprocess Permission Semantics and Sandboxed Execution

# Status

Accepted

# Context

The Plugin API allows plugins to declare required subprocesses in their manifest via `permissions.subprocess: string[]`. The documentation provides examples like `["apt", "dpkg-query"]`. However, the Plugin Host was ignoring these declarations because blindly appending these strings to the Landlock sandbox `Policy.Executables` would be unsafe and non-functional.

Under Landlock, granting `landlockAccessFSExecute` on a binary (e.g., `/usr/bin/apt`) is not sufficient for dynamically linked executables. The kernel must load and execute the ELF interpreter (e.g., `/lib64/ld-linux-x86-64.so.2`), and the interpreter must read shared libraries (e.g., `libc.so.6` from `/lib` or `/usr/lib`). If the sandbox strictly denies access to these paths, the `execve` syscall will fail or the process will crash during dynamic linking.

We need a secure semantic translation from the logical capability (`apt`) to the kernel enforcement primitives that satisfy both the binary and its dynamic runtime dependencies, without introducing path traversal vulnerabilities or granting blanket execution rights to all of `/usr/bin`.

# Decision

1. **Manifest Semantics**: The `subprocess` array must contain **exact binary basenames** (e.g., `apt`), not absolute or relative paths.
2. **Validation**: During plugin registration and policy building, the Plugin Host strictly validates that the requested string does not contain any path separators (`/` or `\`). Any violation results in the plugin failing to load.
3. **Resolution**: The Plugin Host utilizes `exec.LookPath(name)` on the host system to resolve the basename to an absolute path (e.g., `/usr/bin/apt`).
   - *Security Note*: `LookPath` depends on the host's `$PATH`. Because the Plugin Host runs as the system user and the plugin has no influence over the host's environment at resolution time, this is secure.
4. **Enforcement (Target Binary)**: The absolute, resolved path is appended to `Policy.Executables` and passed to the sandbox.
5. **Enforcement (Runtime Dependencies)**: If a plugin declares *any* subprocesses, the Plugin Host automatically appends standard Linux system library directories (`/lib`, `/lib64`, `/usr/lib`, `/usr/lib64`, and `/etc/alternatives`) to the sandbox policy.
   - To satisfy the ELF interpreter, these directories are granted both **read and execute** rights in Landlock.
   - This allows the dynamic linker to execute and shared libraries to load, without granting execute rights to general binary directories like `/usr/bin` or `/bin`.

# Consequences

**Benefits:**
- Restores functional subprocess execution for plugins using system binaries like `apt`.
- Handles dynamic linking dependencies securely without requiring complex per-binary `ldd` tracing.
- Prevents path traversal and arbitrary execution vulnerabilities (e.g., `../../../bin/sh`).
- Maintains the abstraction: plugins don't need to know if `apt` is in `/usr/bin/apt` or `/bin/apt`.

**Trade-offs:**
- Plugins declaring subprocesses receive read+execute access to system library paths. While they cannot modify these paths, they could theoretically execute libraries (e.g., executing `libc.so` directly), which poses negligible security risk compared to granting access to `/bin`.
- Plugins cannot execute binaries shipped within their own plugin directory using the `subprocess` field (they would need a different mechanism). Currently, V1 relies solely on system binaries for subprocesses.
