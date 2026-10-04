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

Minimum observed Landlock rights for ordinary dynamic startup are `EXECUTE|READ_FILE` on the requested binary, `EXECUTE|READ_FILE` on its ELF interpreter, and `READ_FILE` on the needed shared objects. An isolated follow-up showed that `EXECUTE` without `READ_FILE` on the main binary still yielded `execve = -1 EACCES` even when the libraries and interpreter were authorized; the interpreter `EXECUTE`-only split returned EACCES; adding `READ_FILE` succeeded without any library-directory read rule. The repository implementation grants `READ_FILE|READ_DIR` on read paths and `READ_FILE|EXECUTE` on executable files. `/etc/ld.so.cache` was opened for read by the loader. It is not strictly required for this tested startup: with cache access denied, the loader fell back to default library paths and successfully found the shared objects. `/etc/ld.so.preload` was probed but absent (`ENOENT`).

The static contrast was a small `gcc -static` ELF. With only `EXECUTE` on that file, `execve("/tmp/adr0007-static-true", ...) = 0` and it exited 0; there was no interpreter or shared-library open. The file was built in WSL for this experiment and removed afterward.

## 2. Policy matrix

The cumulative matrix was run with `strace -f -e trace=execve,openat` against `/usr/bin/true`. The isolated interpreter split removes all broad library-directory read grants; `/lib64/ld-linux-x86-64.so.2` resolves to `/usr/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2`.

| Variant | Rights added | Result/evidence |
|---|---|---|
| (a) | Exact main binary `EXECUTE|READ_FILE`; no library/interpreter grants | `execve("/usr/bin/true", ...) = -1 EACCES`; exit 126. The kernel cannot execute the ELF interpreter without its execute grant. |
| (b) | (a) + `READ_FILE|READ_DIR` on `/lib*`, `/usr/lib*`, and `/etc` | Same `execve = -1 EACCES`; exit 126. Library reads do not substitute for interpreter execute. |
| (c) | (b) + `EXECUTE|READ_FILE` on the resolved interpreter file | `execve("/usr/bin/true", ...) = 0`; cache and libc opens succeed; exit 0. |
| (d) | (c) + `READ_FILE|READ_DIR|EXECUTE` on `/lib`, `/lib64`, `/usr/lib`, `/usr/lib64` | Startup succeeds, exit 0. This broad execute grant is unnecessary for the observed startup. |

For all variants, the exact executable file has `EXECUTE|READ_FILE`; the experiment shows the binary's `READ_FILE` is required on this ABI. The interpreter rule in (c) was isolated without library-directory reads: `EXECUTE` only returned `EACCES`; `EXECUTE|READ_FILE` succeeded. If (c) is tested without (b), `execve` succeeds but the loader reports `libc.so.6: cannot open shared object file`; strace shows `openat(...libc.so.6...) = -1 EACCES` and exit 127. This isolates the interpreter grant from library read access.

## 3. Symlinks

On this image `/bin -> usr/bin`, `/usr/bin/true -> gnutrue`, and `/lib64/ld-linux-x86-64.so.2 -> ../lib/x86_64-linux-gnu/ld-linux-x86-64.so.2`. The C rule builder uses `open(O_PATH)`, which follows the final symlink, matching the repository’s `landlockAddPathRule` behavior.

With a rule opened on `/usr/bin/true`, executing `/usr/bin/gnutrue` succeeded (`execve(...)=0`, exit 0). Conversely, a rule opened on `/usr/bin/gnutrue` allowed execution through `/usr/bin/true` (`execve(...)=0`, exit 0). Thus the Landlock rule attaches to the resolved target object; the link spelling is not a separate authorization boundary. Resolution still matters for the string retained in policy and `argv[0]` semantics.

## 4. Directory execute authority and loader trick

**Unlisted helper.** With execute/read grants on library directories but no target file rule, `/usr/lib/apt/methods/http` started (`execve = 0`) and emitted its capabilities protocol, then exited 100. This demonstrates executable-allowlist expansion. It does not demonstrate new containment authority: the child inherits the same Landlock and seccomp domain as the plugin.

**LD_PRELOAD and direct loader.** A C probe wrote a shared object after Landlock into the readable/writable staging path and ran authorized `/usr/bin/true` with `LD_PRELOAD`. The constructor ran without an EXECUTE grant on the staged file. A separate PIE test wrote a PIE after Landlock and invoked `ld.so /tmp/adr0007-stage/payload.so`; it printed `LOADER_PIE_CODE_RAN` and exited 33. `strace` showed `execve(ld.so)=0`, then the loader opened the staged PIE for reading; it did not call `execve` on the staged PIE. Thus the proper ld.so PIE route bypasses Landlock's EXECUTE check.

The project's seccomp filter defaults to ALLOW and source inspection shows it does not block `mmap`/`mprotect` or `execve`/`execveat`. The requested real `linuxSandbox.Exec` helper/preload/ld.so fixture attempts each stopped before fixture startup with `plugin failed: exit status 111 (stderr: permission denied)`. Filtered outcomes remain unverified in this WSL environment.

**Rights available to loaded code.** Both the LD_PRELOAD and PIE payloads measured `/etc/shadow` open as EACCES; AF_INET socket creation succeeded; mount returned EPERM; unshare succeeded in this no-seccomp probe; `PR_GET_NO_NEW_PRIVS=1`, `PR_GET_SECCOMP=0`. The payloads gained no Landlock authority compared with plugin code. These are executable-allowlist fidelity gaps, not demonstrated containment escapes. The current write rule includes READ_FILE (`fsAccess &^ EXECUTE`); ADR-0013 staging pairs readable and writable access, so document this residual under that staging design.

## 5. Inheritance and scope

Landlock is inherited across fork/clone and exec. The shell trace showed the unlisted APT helper starting while an ungranted `/usr/bin/true` remained denied. Descendants retain caller restrictions. Subprocess declarations may define intended helpers, but directory EXECUTE expands that manifest allowlist without adding demonstrated kernel rights.

### dpkg-query read set

Under a traced Landlock policy, `dpkg-query -W dpkg` successfully read `/var/lib/dpkg/status`, `/var/lib/dpkg/updates/`, `/var/lib/dpkg/triggers/File`, and `/var/lib/dpkg/triggers/Unincorp`. Startup also opened `libmd.so.0`, `libc.so.6`, locale data under `/usr/lib/locale/C.utf8`, and `/usr/lib/x86_64-linux-gnu/gconv/gconv-modules.cache`. Absent locale candidates and `/var/lib/dpkg/arch` returned ENOENT. `/usr/share/dpkg` and `/etc/dpkg` were not opened for content in this invocation. A narrow database policy can grant exact status, updates, and trigger paths; a read-only `/var/lib/dpkg` grant is simpler. Parsing status directly needs only one exact READ_FILE grant on `/var/lib/dpkg/status`, with no parent-directory read grant.


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
| Exact ELF interpreter file resolved from the ELF header | `EXECUTE|READ_FILE` (execute-only returned `EACCES`). Do not grant execute on its containing directory. |
| Exact shared-object dependency files | `READ_FILE`; grant `READ_DIR` only to directories needed for pathname traversal. Prefer exact dependency files; if broad ABI-specific library directories are unavoidable, they must be read-only and must not include `EXECUTE`. |
| `/etc/ld.so.cache` | `READ_FILE` only if retained; it was opened by these loaders but the tested loader successfully fell back when it received `EACCES`. |
| `/etc/ld.so.preload` | No grant needed when absent; if host policy requires it, treat it as a sensitive single file and account for the fact it can inject code. |
| `/etc/alternatives` | No directory grant. Resolve the selected link to its concrete target and authorize that file. |
| `/proc`, `/sys` | No startup grant based on these traces; grant only for a measured, documented runtime dependency. |
| Plugin writable/staging path | ADR-0013 pairs staging read/write grants. This lets loader routes consume staged code without Landlock EXECUTE; document this allowlist-fidelity residual. Measurements found no added Landlock authority. |

Do not give `EXECUTE` to `/lib`, `/lib64`, `/usr/lib`, `/usr/lib64`, `/bin`, `/usr/bin`, or `/etc/alternatives`. Directory execute both exceeds the basename manifest and allowed the undeclared APT helper in the experiment.

## Residual risks

1. LD_PRELOAD and direct PIE-through-ld.so execute bytes lacking Landlock EXECUTE. This weakens executable-allowlist fidelity. Tested code gained no extra Landlock authority: `/etc/shadow` remained EACCES, sockets already worked, and mount was EPERM. Unshare succeeded only in the no-seccomp C probe.
2. ADR-0013's readable writable staging enables this loader behavior. It is a documented residual of the staging design, not an established privilege escalation or containment escape. Do not describe exact executable-file grants as a complete code-loading boundary.
3. Broad library-directory EXECUTE starts undeclared helpers, which inherit the same Landlock domain. This is a manifest-fidelity issue absent evidence that the helper itself gains rights unavailable to plugin code.
4. Loader environment, PATH changes, file replacement, dlopen, architecture-specific dependencies, and transitive helper graphs still require explicit design. Results are specific to WSL Ubuntu 26.04.1, x86_64, ABI 7.
5. Real `linuxSandbox.Exec` scenarios were attempted, but all failed before the Go fixture began: helper exit 111, stderr `permission denied`. Actual namespace/NO_NEW_PRIVS/seccomp behavior for these payloads remains unverified.

## Proposed ADR-0007 amendment text (proposal only)


> A `subprocess` entry is a logical executable basename, never a path. Registration rejects empty names, embedded NUL, `/`, `\\`, relative PATH results, and unresolved/non-executable targets. The host resolves each entry against a fixed trusted PATH, canonicalizes the target, and stores the resolved absolute file path together with the logical invocation name needed for `argv[0]` semantics. The policy grants `EXECUTE|READ_FILE` on each explicitly resolved executable and exact ELF interpreter, plus read-only access to measured shared-object dependencies and loader metadata. It MUST NOT grant execute on library or binary directories. Declarations cover the complete descendant process tree and define executable-allowlist fidelity. Readable writable staging (as in ADR-0013) permits LD_PRELOAD and direct ld.so PIE loading without Landlock EXECUTE; experiments found no additional Landlock authority, so document this as a residual rather than an established containment escape. Do not claim the manifest constrains every code-loading path.
The present ADR statement that library-directory execute has negligible risk should be removed. The helper execution is evidence of allowlist expansion; the loader results similarly show code-loading paths outside the executable-file list, without a demonstrated privilege gain.

## Binding scope for Item D

**Item D must:**

- Validate subprocess declarations as basenames before lookup; reject traversal, NUL, and invalid separators.
- Resolve against a fixed absolute PATH, handle `exec.ErrDot`, canonicalize targets, and preserve alias/argv0 semantics.
- Grant exact executable and interpreter targets (`EXECUTE|READ_FILE`) plus measured read-only dependencies/metadata. Document the descendant graph and distinguish command allowlisting from kernel containment.
- Include verification for declared/undeclared helpers, PATH and symlink cases, descendants, and readable staged loader routes. The real Linux sandbox fixture was blocked by helper `permission denied` in this WSL run; do not present C-only results as end-to-end verification.
- Document the ADR-0013 staging interaction: loader-mediated code can run without target EXECUTE, but these experiments found no added Landlock authority. Treat it as residual allowlist-fidelity behavior, not an established open containment requirement.

**Item D must not:**

- Append raw manifest strings to `Policy.Executables`.
- Grant execute on `/lib*`, `/usr/lib*`, `/bin`, `/usr/bin`, or `/etc/alternatives` to make dynamic linking work.
- Treat subprocess declarations as only top-level commands when descendants inherit restrictions.
- Modify ADR-0007 until this analysis and proposed amendment are reviewed.