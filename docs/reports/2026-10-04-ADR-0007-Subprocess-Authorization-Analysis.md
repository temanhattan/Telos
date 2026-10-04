# ADR-0007 Subprocess Authorization Analysis

**Date:** 2026-10-04  
**Status:** S0 analysis; no production or ADR changes  
**Environment:** Ubuntu 26.04.1 LTS under WSL2; kernel `6.18.33.2-microsoft-standard-WSL2` x86_64; Landlock ABI 7; strace 6.19; Go 1.26.4.

## Phase 1 — Project orientation checkpoint

The current status dashboard orders the remaining work as S0 analysis, Item D subprocess translation, then capture staging E1–E3. The host and Linux sandbox are implemented and tested, while trust verification and most product subsystems are not. Item D is explicitly approved in ADR-0007 but absent from code.

**a. Missing implementation.** `internal/plugin/host.go:load()` copies `permissions.subprocess` directly into `model.PluginManifest.Subprocesses`. It does not validate names, resolve them, or reject path input. `buildPolicy()` returns only `Executables: []string{p.Executable}`. There is no subprocess resolver or policy expansion in the host or sandbox.

**b. Manifest validation today.** `load()` rejects subprocess declarations only for `ClassificationRule` plugins, as part of that type’s general “no permissions” check. For other types, values such as empty strings, separators, NUL, duplicates, missing executables, and PATH resolution are unchecked. The raw slice is stored on the manifest.

**c. Current policy.** `buildPolicy()` sets `ReadPaths` to the plugin directory plus normalized `filesystem_read`; `WritePaths` to normalized `filesystem_write`; and `Executables` to the plugin executable alone. No subprocess or system-library grants exist. Subprocess authorization is greenfield. The plugin’s own executable is a separate existing grant.

**d. Surprise/contradiction.** The actual behavior contradicts the risk characterization in ADR-0007’s trade-off: execute permission on library directories is not “negligible” execution authority. WSL tests below successfully started the unlisted `/usr/lib/apt/methods/http` helper under directory-only execute grants. More critically, a shared object written into a sandbox write path ran through `LD_PRELOAD` once the main binary, interpreter, and libraries were readable/executable as needed. The seccomp filter does not close this.

The recent Git history agrees with the status documentation: commit `c0f1e20` sets `defaultMemoryBytes = 0`, following the RLIMIT_AS investigation/proposal commits; `3f1a55f` adds bounded stderr; `fd21deb` fixes sandbox stdin forwarding. `seccomp_linux.go` implements a **denylist**, not an allowlist: listed dangerous calls are denied, while the default action is ALLOW.

## Test setup and platform evidence

Commands were run in WSL2. The kernel and ABI observations were:

```text
$ uname -a
Linux ZED-PC 6.18.33.2-microsoft-standard-WSL2 #1 SMP PREEMPT_DYNAMIC Thu Jun 18 21:54:43 UTC 2026 x86_64 GNU/Linux
$ cat /etc/os-release
PRETTY_NAME="Ubuntu 26.04.1 LTS"
VERSION_ID="26.04"
$ python3 -c 'import ctypes; l=ctypes.CDLL(None,use_errno=True); r=l.syscall(444,0,0,1); print(r,ctypes.get_errno())'
7 0
$ export PATH="$PATH:/usr/local/go/bin"; go version
go version go1.26.4 linux/amd64
```

The C probe and payload sources are in `docs/reports/experiments/adr0007/`; the Go LookPath probe has `//go:build ignore`. No production code, tests, or ADRs were changed. The C harness creates the test payload only after applying its Landlock rules. The policies below are cumulative: (a) binary only; (b) add library-directory read and `/etc` read; (c) add execute on the resolved ELF interpreter file; (d) add execute on the library directories.

## 1. Dynamic and static startup rights

Commands:

```text
$ ldd /usr/bin/true
libc.so.6 => /usr/lib/x86_64-linux-gnu/libc.so.6
/lib64/ld-linux-x86-64.so.2
$ ldd /usr/bin/dpkg-query
libmd.so.0 => /usr/lib/x86_64-linux-gnu/libmd.so.0
libc.so.6 => /usr/lib/x86_64-linux-gnu/libc.so.6
/lib64/ld-linux-x86-64.so.2
$ strace -e trace=execve,openat /usr/bin/true
execve("/usr/bin/true", ...) = 0
openat(..., "/etc/ld.so.preload", O_RDONLY) = -1 ENOENT
openat(..., "/etc/ld.so.cache", O_RDONLY|O_CLOEXEC) = 3
openat(..., "/usr/lib/x86_64-linux-gnu/libc.so.6", O_RDONLY|O_CLOEXEC) = 3
+++ exited with 0 ++++
```

`dpkg-query` additionally loads `libmd.so.0`; `dpkg-query -W dpkg` later tried `/var/lib/dpkg/status`, where the test policy correctly received `EACCES`. Startup itself did not attempt `/proc` or `/sys` reads: `strace -e trace=openat /usr/bin/true | grep -E '/proc|/sys'` produced no matches. This is evidence for these binaries and this Ubuntu image, not every optional library/plugin path.

Minimum observed Landlock rights for ordinary dynamic startup are `EXECUTE|READ_FILE` on the requested binary, `EXECUTE` on its ELF interpreter, and `READ_FILE` on the needed shared objects. An isolated follow-up showed that `EXECUTE` without `READ_FILE` on the main binary still yielded `execve = -1 EACCES` even when the libraries and interpreter were authorized; making the interpreter rule `EXECUTE`-only still succeeded once the main binary had both rights. The repository implementation grants `READ_FILE|READ_DIR` on read paths and `READ_FILE|EXECUTE` on executable files. `/etc/ld.so.cache` was opened for read by the loader. It is not strictly required for this tested startup: with cache access denied, the loader fell back to default library paths and successfully found the shared objects. `/etc/ld.so.preload` was probed but absent (`ENOENT`).

The static contrast was a small `gcc -static` ELF. With only `EXECUTE` on that file, `execve("/tmp/adr0007-static-true", ...) = 0` and it exited 0; there was no interpreter or shared-library open. The file was built in WSL for this experiment and removed afterward.

## 2. Policy matrix

The exact cumulative matrix was run with `strace -f -e trace=execve,openat` against `/usr/bin/true`:

| Variant | Rights added | Result/evidence |
|---|---|---|
| (a) | Exact main binary `EXECUTE|READ_FILE`; no library/interpreter grants | `execve("/usr/bin/true", ...) = -1 EACCES`; exit 126. The kernel cannot execute the ELF interpreter without its execute grant. |
| (b) | (a) + `READ_FILE|READ_DIR` on `/lib*`, `/usr/lib*`, and `/etc` | Same `execve = -1 EACCES`; exit 126. Library reads do not substitute for interpreter execute. |
| (c) | (b) + `EXECUTE` on `/lib64/ld-linux-x86-64.so.2` (the resolved file) | `execve("/usr/bin/true", ...) = 0`; cache and libc opens succeed; exit 0. |
| (d) | (c) + `READ_FILE|READ_DIR|EXECUTE` on `/lib`, `/lib64`, `/usr/lib`, `/usr/lib64` | Startup succeeds, exit 0. This broad execute grant is unnecessary for the observed startup. |

For all variants, the exact executable file has `EXECUTE|READ_FILE`; the experiment shows the binary's `READ_FILE` is required on this ABI. The interpreter rule in (c) was separately tested with `EXECUTE` only and succeeded. If (c) is tested without (b), `execve` succeeds but the loader reports `libc.so.6: cannot open shared object file`; strace shows `openat(...libc.so.6...) = -1 EACCES` and exit 127. This isolates the interpreter grant from library read access.

## 3. Symlinks

On this image `/bin -> usr/bin`, `/usr/bin/true -> gnutrue`, and `/lib64/ld-linux-x86-64.so.2 -> ../lib/x86_64-linux-gnu/ld-linux-x86-64.so.2`. The C rule builder uses `open(O_PATH)`, which follows the final symlink, matching the repository’s `landlockAddPathRule` behavior.

With a rule opened on `/usr/bin/true`, executing `/usr/bin/gnutrue` succeeded (`execve(...)=0`, exit 0). Conversely, a rule opened on `/usr/bin/gnutrue` allowed execution through `/usr/bin/true` (`execve(...)=0`, exit 0). Thus the Landlock rule attaches to the resolved target object; the link spelling is not a separate authorization boundary. Resolution still matters for the string retained in policy and `argv[0]` semantics.

## 4. Directory execute authority and loader trick

**Unlisted helper.** With only execute/read grants on the library directories (no file rule for the target), the probe invoked `/usr/lib/apt/methods/http`:

```text
execve("/usr/lib/apt/methods/http", ...) = 0
100 Capabilities
Send-URI-Encoded: true
Send-Config: true
Pipeline: true
Version: 1.2
+++ exited with 100 ++++
```

Exit 100 is this APT method’s protocol response, and proves the unlisted executable started. The same broad directory rule allowed the ELF interpreter and libraries to load. Execute on a library directory therefore grants execution to unrelated executable files below it, including APT method helpers. This directly disproves the ADR-0007 trade-off statement.

**LD_PRELOAD test.** The probe wrote a tiny ELF shared object into a `WritePaths`-like directory after Landlock restriction. That directory had the same rights as this repository’s `rwAccess`: all handled filesystem rights except `EXECUTE`, which includes `READ_FILE`. The payload constructor exits with status 42. The probe then ran authorized `/usr/bin/true` with `LD_PRELOAD` pointing at the written file.

With the cumulative (c) grants plus that read/write staging path, strace showed:

```text
execve("/usr/bin/true", ...) = 0
openat(..., "/tmp/adr0007-stage/payload.so", O_RDONLY|O_CLOEXEC) = 3
mmap(..., PROT_READ|PROT_EXEC, ...) = ...
+++ exited with 42 ++++
```

With (d) the same payload also ran and exited 42. Variants (a) and (b) could not start the dynamic target: the kernel denied its missing interpreter with `execve = -1 EACCES`. A separate direct `ld.so /path/to/payload.so` attempt was denied when the loader tried `execve(payload.so) = -1 EACCES`; that invocation alone is not the bypass. The demonstrated route is an authorized dynamic binary loading an unexecuted shared object via `LD_PRELOAD`.

This probe did not install the project’s seccomp filter. Source inspection is decisive for the filter question: `buildSeccompFilter()` returns ALLOW by default and only denies its enumerated calls; it neither denies `mmap`/`mprotect` nor restricts `execve`/`execveat`. The traced loader’s `openat` and executable `mmap` operations are not blocked by the existing filter. **The current seccomp filter does not close this gap.**

This route requires the written file to be readable after creation. The current `sandbox_linux.go` write rule includes `READ_FILE` and ABI read rights (`fsAccess &^ EXECUTE`). Any future policy that combines readable writable staging with a dynamic executable/interpreter can reproduce it. Merely withholding `EXECUTE` on the payload is insufficient.

## 5. Inheritance and scope

The Landlock domain is inherited across `fork`/`clone` and `exec`; it is not reset per child. A shell parent was given a file execute rule for `/usr/bin/bash` and library-directory execute. Its trace showed:

```text
execve("/usr/bin/bash", ...) = 0
clone(..., SIGCHLD, ...) = 437
[pid 437] execve("/usr/lib/apt/methods/http", ...) = 0
execve("/usr/bin/true", ...) = -1 EACCES (Permission denied)
```

The child helper inherited the same grants, including the broad library-directory execute authority; an ungranted `/usr/bin/true` remained denied. Consequently, subprocess declarations describe the complete descendant process tree, not just the top-level command. If `apt` needs to invoke `dpkg`, both commands and any other intended transitive executables must be authorized, or an explicitly designed broker/launcher must mediate that graph. Directory-wide execute silently bypasses this manifest scope.

## 6. Name traversal and PATH

The Linux Go 1.26.4 probe ran `exec.LookPath` with `PATH="."`, `"/usr/bin"`, `"relative:/usr/bin"`, and `"relative:."`, and with path-like names. Output:

```text
PATH="." LookPath("landlock_probe")="landlock_probe" err=exec: "landlock_probe": cannot run executable found relative to current directory
PATH="/usr/bin" LookPath("true")="/usr/bin/true" err=<nil>
PATH="relative:/usr/bin" LookPath("true")="/usr/bin/true" err=<nil>
PATH="relative:." LookPath("landlock_probe")="landlock_probe" err=exec: "landlock_probe": cannot run executable found relative to current directory
name="../x" LookPath="" err=exec: "../x": stat ../x: no such file or directory
name="a/b" LookPath="" err=exec: "a/b": stat a/b: no such file or directory
name="a\\b" LookPath="" err=exec: "a\\b": executable file not found in $PATH
name="" LookPath="" err=exec: "": executable file not found in $PATH
name="." LookPath="" err=exec: ".": executable file not found in $PATH
name="true\\x00x" LookPath="" err=exec: "true\\x00x": executable file not found in $PATH
```

The last value contains an embedded NUL (the probe builds it with `string([]byte{0})`); `LookPath` reports only its wrapped not-found error, not the underlying `EINVAL`. `../x` and `a/b` are passed to filesystem stat as path input; `LookPath` does not enforce basename semantics. On Linux, backslash is an ordinary character, but it must still be rejected for platform-independent manifest semantics. A relative PATH hit returns a relative result and `exec.ErrDot`; relative entries that miss can be skipped before an absolute hit. Therefore the current manifest’s raw values are not safe to pass directly to LookPath. Future load-time code must reject empty names, NUL, `/`, and `\\`, and must either reject `exec.ErrDot` or require an absolute PATH result before canonicalization.

## 7. Canonical resolution edge cases for Debian-family hosts

- **`/etc/alternatives`:** names such as `pager` and `editor` can resolve through `/usr/bin/<name> -> /etc/alternatives/<name> -> selected implementation`. The selected program varies with host configuration. Canonicalizing the resolved binary gives Landlock a stable target and avoids execute permission on `/etc/alternatives` as a directory. The manifest should authorize the logical name, while the host records/logs the resolved target.
- **Multi-call binaries:** BusyBox applet names are symlinks to one binary; dispatch can depend on `argv[0]`. Blindly replacing the lookup path with `EvalSymlinks` and using that resolved basename as `argv[0]` can change the selected applet. Resolution must preserve the requested logical invocation name/argv0 while authorizing the underlying inode, or reject this case in V1.
- **Package-manager helpers:** `apt` may launch `dpkg` and method/helper programs from locations such as `/usr/lib/apt/methods/`. Those are transitive subprocess requirements and must be individually declared/resolved if the contract is an executable allowlist. The WSL `/usr/bin/dpkg-query` is a dynamically linked ELF; `ldd` showed `libmd.so.0`, `libc.so.6`, and the interpreter.
- **PATH shadowing/changes:** `LookPath` uses the host process PATH at resolution time. An absolute, canonical result must be captured during registration, and the invocation must execute that captured path rather than re-resolving an untrusted or changed PATH. The resolved file should be checked as a regular executable and its final target must be stable for the policy lifetime.

## Recommended minimal policy

For each declared subprocess and its declared transitive children:

| Path class | Minimum right |
|---|---|
| Exact authorized executable file | `EXECUTE|READ_FILE` (both were needed by this kernel/probe); do not authorize its parent directory for execute. |
| Exact ELF interpreter file resolved from the ELF header | `EXECUTE` (the isolated test succeeded without `READ_FILE`). Do not grant execute on its containing directory. |
| Exact shared-object dependency files | `READ_FILE`; grant `READ_DIR` only to directories needed for pathname traversal. Prefer exact dependency files; if broad ABI-specific library directories are unavoidable, they must be read-only and must not include `EXECUTE`. |
| `/etc/ld.so.cache` | `READ_FILE` only if retained; it was opened by these loaders but the tested loader successfully fell back when it received `EACCES`. |
| `/etc/ld.so.preload` | No grant needed when absent; if host policy requires it, treat it as a sensitive single file and account for the fact it can inject code. |
| `/etc/alternatives` | No directory grant. Resolve the selected link to its concrete target and authorize that file. |
| `/proc`, `/sys` | No startup grant based on these traces; grant only for a measured, documented runtime dependency. |
| Plugin writable/staging path | Grant only required create/write rights. Do **not** combine it with `READ_FILE` where avoidable: current `rwAccess` includes read and the LD_PRELOAD probe demonstrates code loading from such a path. |

Do not give `EXECUTE` to `/lib`, `/lib64`, `/usr/lib`, `/usr/lib64`, `/bin`, `/usr/bin`, or `/etc/alternatives`. Directory execute both exceeds the basename manifest and allowed the undeclared APT helper in the experiment.

## Residual risks

1. The current sandbox uses a seccomp denylist. It does not constrain executable `mmap`, `mprotect`, `execve`, or `execveat`; it does not mitigate the demonstrated preload route.
2. Current write rules include read rights, so a dynamic executable can load a newly written shared object from a read/write grant without Landlock `EXECUTE` on that file. This is empirically demonstrated. Item D must not claim exact executable authorization is a complete code-execution boundary while this remains true.
3. Broad library-directory execute grants let a plugin invoke undeclared helpers and all descendant processes inherit that authority.
4. Environment variables affecting the loader, PATH changes, file replacement between resolution and execution, interpreter/runtime `dlopen` behavior, architecture-specific library paths, and package-manager helper graphs require explicit design. The captured matrix is WSL Ubuntu 26.04.1, x86_64, ABI 7; other Debian-family releases and architectures may differ.
5. This report’s Landlock matrix uses an isolated C reproducer for policy-right combinations, not `InvokeSandboxed()` end-to-end. The project’s denylist filter was source-reviewed but not installed in that reproducer; the relevant syscalls are visibly outside its denylist.

## Proposed ADR-0007 amendment text (proposal only)

> A `subprocess` entry is a logical executable basename, never a path. Registration rejects empty names, embedded NUL, `/`, `\\`, relative PATH results, and unresolved/non-executable targets. The host resolves each entry against a fixed trusted PATH, canonicalizes the target, and stores the resolved absolute file path together with the logical invocation name needed for `argv[0]` semantics. The policy grants execute only on each explicitly resolved executable file and its exact ELF interpreter; it grants read-only access to measured shared-object dependencies and loader metadata. It MUST NOT grant execute on library or binary directories. Declarations cover the complete descendant process tree. These rights do not by themselves prevent code loading via readable writable paths: implementation must separately ensure untrusted writable content is not readable by dynamic loaders or otherwise close/test the `LD_PRELOAD`/executable-mapping route before claiming subprocess authorization is an execution allowlist.

The present ADR statement that library-directory execute has negligible risk should be removed. The measured helper execution and preload behavior are counterevidence.

## Binding scope for Item D

**Item D must:**

- Validate the declaration as basenames before any path lookup; reject all traversal/path and invalid-byte cases above.
- Resolve against a fixed host-controlled absolute PATH, handle Go `exec.ErrDot`, canonicalize the executable target, and preserve alias/`argv[0]` behavior deliberately.
- Add only exact declared executable file targets and exact interpreter targets, plus measured read-only dependencies/metadata. Document how resolved dependencies and the manifest’s descendant-process graph are handled.
- Add explicit tests for allowed declared targets, denied undeclared targets, paths/NUL/PATH cases, symlink resolution, transitive children, and loader-assisted loading from writable content. Do not claim success based on unit policy slices alone.
- Treat the `LD_PRELOAD` result as an open security requirement: coordinate a change to rights on writable paths, environment handling, or another enforceable boundary and prove the fix before advertising the subprocess list as an execution allowlist.

**Item D must not:**

- Append raw manifest strings to `Policy.Executables`.
- grant execute on `/lib*`, `/usr/lib*`, `/bin`, `/usr/bin`, or `/etc/alternatives` to make dynamic linking work.
- Rely on `EXECUTE` being absent from `WritePaths` as proof that written code cannot run.
- Treat subprocess declarations as only top-level commands when descendants inherit the same Landlock domain.
- Modify ADR-0007 until this analysis and proposed amendment are reviewed.
