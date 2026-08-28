# V1 Sandbox & Trust Boundary — Final Implementation Plan

---

## 1. Workstream A — Sandbox Enforcement Fixes

Workstream A is **fully unblocked** and can be implemented immediately. It fixes four confirmed pre-existing bugs in the Linux sandbox enforcement layer without altering the V1 capability model.

---

### A1. Seccomp `Jf:1` Denylist Bypass

**Current state in** [`seccomp_linux.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/sandbox/seccomp_linux.go):

The current `buildSeccompFilter()` (line 98–101) already uses `Jf: 0` for the denylist loop. However, the jump target patching at lines 136–138 only sets `Jt` (jump-true targets) and leaves `Jf` at the initialized `0` value.

**Verification required:** Walk through the generated BPF instruction sequence and verify that:

1. Each denied syscall comparison at instruction index `i` has:
   - `Jt` = offset from `i` to `denyEPERMIdx` (the `RET EPERM` instruction).
   - `Jf` = `0` (fall through to instruction `i+1`, which is the next comparison).
2. After the last denied syscall comparison, the next instruction is the `clone3` check.
3. After the `clone3` check, the next instruction is the `clone` check.
4. After the `clone` check, the next instruction is `RET ALLOW` (default allow).
5. Every `Jt` offset is ≤ 255 (uint8 maximum).
6. No jump can land outside the instruction array.

**Implementation:**
- Add a unit test `TestBuildSeccompFilterInstructions` that:
  - Calls `buildSeccompFilter()`.
  - Verifies the total instruction count matches expectations.
  - Verifies `Jf == 0` for every denylist comparison.
  - Verifies every `Jt` target resolves to the correct `RET` instruction.
  - Verifies the architecture check at instructions 0–3.
  - Verifies `clone3` → `RET ENOSYS`.
  - Verifies `clone` → flag check → conditional `EPERM`/`ALLOW`.
  - Verifies default → `RET ALLOW`.

**File:** [`seccomp_linux.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/sandbox/seccomp_linux.go)
**Test file:** New `seccomp_linux_test.go` (build-tagged `linux`)

---

### A2. Clone3 Handling

**Current state:** `buildSeccompFilter()` at line 103–105 checks `clone3` (syscall 435) and patches its `Jt` at line 142–143 to jump to `denyENOSYSIdx`.

**Security invariant:** `clone3` is unconditionally denied with `ENOSYS`. Classic cBPF cannot inspect the `clone_args` struct pointed to by `clone3`'s first argument (it's a memory pointer, not a register value). Therefore `clone3` cannot be conditionally filtered — it must be unconditionally denied.

**Why `ENOSYS`:** `ENOSYS` tells glibc that the syscall is not supported, causing it to transparently fall back to the older `clone()` syscall, where the flags are in a register and *can* be inspected by cBPF. This is a compatibility behavior, not a security guarantee — the security requirement is simply that `clone3` is denied.

**Verification:** The unit test from A1 must verify that instruction `clone3CheckIdx` has `Jt` pointing to the `RET ENOSYS` instruction.

**No code change needed** — the current implementation already handles this correctly. Only verification via test.

---

### A3. Seccomp Denylist Completeness

**Verify** that the denylist in `deniedSyscalls` (lines 63–83) includes all syscalls documented in [`05_Plugin_API.md`](file:///c:/Users/Zeyad/PycharmProjects/Telos/docs/05_Plugin_API.md) §seccomp-BPF (line 681) and [`ADR-0011`](file:///c:/Users/Zeyad/PycharmProjects/Telos/docs/ADRs/ADR-0011-Plugin-Execution-Isolation.md):

| Documented | Syscall | In denylist? |
|---|---|---|
| ptrace | `SYS_PTRACE` | ✅ |
| mount | `SYS_MOUNT` | ✅ |
| umount2 | `SYS_UMOUNT2` | ✅ |
| reboot | `SYS_REBOOT` | ✅ |
| kexec_load | `SYS_KEXEC_LOAD` | ✅ |
| init_module | `SYS_INIT_MODULE` | ✅ |
| finit_module | `SYS_FINIT_MODULE` | ✅ |
| delete_module | `SYS_DELETE_MODULE` | ✅ |
| pivot_root | `SYS_PIVOT_ROOT` | ✅ |
| chroot | `SYS_CHROOT` | ✅ |
| swapon | `SYS_SWAPON` | ✅ |
| swapoff | `SYS_SWAPOFF` | ✅ |
| setns | `SYS_SETNS` | ✅ |
| unshare | `SYS_UNSHARE` | ✅ |
| acct | `SYS_ACCT` | ✅ |
| settimeofday | `SYS_SETTIMEOFDAY` | ✅ |
| clock_settime | `SYS_CLOCK_SETTIME` | ✅ |
| adjtimex | `SYS_ADJTIMEX` | ✅ |
| kexec_file_load | `320` | ✅ |

**No code change needed** — verify only.

---

### A4. Path Validation

**Current state in** [`paths.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/sandbox/paths.go):

`NormalizeManifestPaths()` (lines 31–62) already implements the correct V1 model:

1. **Relative paths:** Joined to `baseDir`, symlinks resolved via `filepath.EvalSymlinks`, containment verified via `IsSubpath`. Escapes are rejected.
2. **Absolute paths:** Lexically cleaned, `..` traversal rejected, `os.Stat` verifies existence. These are preserved as explicit host filesystem authorizations per V1 design.

**What to verify:**
- That `NormalizeManifestPaths` is called in [`host.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/host.go) lines 161–168 before the paths enter the `Plugin` struct.
- That `buildPolicy()` (line 309–318) passes these already-normalized paths into the `Policy`.
- That `Policy.Validate()` (in [`policy.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/sandbox/policy.go)) enforces absolute-path-only at the sandbox boundary.

**Existing test gaps in** [`paths_test.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/sandbox/paths_test.go):

Add the following test cases:
- Nested traversal: `"foo/../../secret"` → rejected.
- Symlink chain escaping: `internal_link → ../../escape` → rejected.
- Absolute path with `..` after `Clean`: `"/etc/../root"` → verify `Clean` produces `"/root"` and no `..` remains.
- Empty path → rejected.
- Absolute path to nonexistent target → rejected.
- Multiple valid mixed paths → all succeed.

**File:** [`paths.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/sandbox/paths.go) (no changes expected), [`paths_test.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/sandbox/paths_test.go) (new cases).

---

### A5. Executable Validation

**Current state in** [`host.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/host.go) `executablePath()` (lines 176–208):

1. Rejects absolute paths, `..` prefixes, and paths where `filepath.Clean` differs from the declared value.
2. Resolves symlinks via `filepath.EvalSymlinks`.
3. Verifies the resolved target is within `plugin.Dir` via `sandbox.IsSubpath`.
4. Verifies the target exists and is not a directory.

**This is correct.** The executable must be a plugin-internal file. This is distinct from filesystem *permissions* (which may reference absolute host paths) — the executable is the plugin's own code, which must reside inside the trusted package.

**Add tests** for:
- Declared executable with absolute path → rejected.
- Declared executable with `../` → rejected.
- Declared executable with symlink escaping → rejected.
- Valid declared executable → resolved path returned.

**File:** [`host_test.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/host_test.go) (new cases).

---

### A6. Symlink Model

The V1 symlink model, as implemented:

1. **Relative manifest paths:** `filepath.EvalSymlinks` is called. The *resolved target* must remain inside `plugin.Dir`. If a relative symlink inside the plugin directory points outside, the path is rejected at the `IsSubpath` check after resolution.

2. **Absolute host paths:** No symlink resolution is performed (lexical `Clean` only). This is intentional — absolute paths are explicit host authorizations, and the user/signer is responsible for their correctness. Landlock operates on inodes, so it follows symlinks at the kernel level regardless.

3. **Landlock enforcement:** Landlock opens paths with `O_PATH` and operates on the resulting inode. The kernel handles symlink resolution at the filesystem layer. The `NormalizeManifestPaths` function ensures that the path strings passed to Landlock are clean and valid, but Landlock's inode-level enforcement provides the final guarantee.

**Security invariant:** A relative path declared in a plugin manifest cannot grant access to files outside the plugin directory, even via symlink chains. An absolute path is an explicit host authorization and is not constrained to the plugin directory.

**No code change needed.**

---

### A7. selfPath Trust

**Current state:** `NewLinuxSandbox(selfPath string)` in [`sandbox_linux.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/sandbox/sandbox_linux.go) line 35 accepts `selfPath` from its caller.

**Tracing callers:**
- The only production call site constructs `selfPath` from `os.Executable()` (documented intent in the `#nosec` comment at line 83, confirmed by [`sandbox_integration_test.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/sandbox/sandbox_integration_test.go) line 36).
- `os.Executable()` returns the path to the currently running binary, which is a trusted OS-provided value.
- `os.Stat(selfPath)` (line 39) verifies existence but does **not** establish trust — trust comes from `selfPath` originating from `os.Executable()`.

**Action:** Update the `#nosec G204` justification at line 83 to:
```
// #nosec G204 -- selfPath originates from os.Executable() in all production callers; os.Stat validates existence but trust derives from the call-site invariant, not from Stat
```

**File:** [`sandbox_linux.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/sandbox/sandbox_linux.go) (comment-only change).

---

## 2. Workstream B — Trust Phase Design

Workstream B is **blocked** on architectural decisions. This section defines the design questions that must be resolved via ADRs before implementation can begin.

---

### B1. What Exactly Is Signed?

**The circularity problem:**
```
manifest contains signature field
signature covers manifest
→ signature signs itself?
```

**Resolution:** The signature **cannot** cover its own value. The signing process must operate on a canonical representation that **excludes** the `signature` field itself.

**Proposed model (requires ADR):**

1. The signer reads the plugin package.
2. The signer computes a **package digest** by:
   - Enumerating all files in the package directory (excluding the signature file or the `signature` field value).
   - Computing a per-file hash.
   - Constructing a deterministic byte sequence from the sorted list of `(relative_path, hash)` pairs.
   - Computing a root hash over this byte sequence.
3. The signer signs the root hash.
4. The signature is placed in the manifest's `signature` field (or in a separate signature file).

**Open question:** Is the signature stored *inside* the manifest's `signature` field, or as a separate file (e.g., `manifest.sig`)? The current manifest schema (`signature: SignatureBlock?` in the docs, `Signature string` in code) suggests it's embedded in the manifest. If embedded:
- The signed data is the manifest contents with the `signature` field zeroed/excluded, plus the file hashes of all other package files.

> [!IMPORTANT]
> **ADR Required:** Define the exact signing input format. Options:
> - **(a)** Sign the manifest bytes with `signature` field stripped/zeroed, concatenated with sorted file hashes.
> - **(b)** Use a separate signature file; sign the entire manifest plus file hashes.
> - **(c)** Sign a separate "package manifest" file containing file hashes; the `signature` field in `plugin.yaml` references or contains the cryptographic signature of that separate manifest.

---

### B2. Package Canonicalization

The signing input must be **deterministic.** Any variation between the signing machine and the verifying machine must not change the digest.

**Issues to resolve (ADR required):**

| Issue | Resolution Required |
|---|---|
| **File ordering** | Files must be enumerated in a deterministic order (e.g., lexicographic sort of relative paths using `/` separator). |
| **Path separators** | Normalize to `/` regardless of OS. |
| **Relative paths** | All paths are relative to the package root. Absolute paths must not appear. |
| **Symlinks** | Decide: (a) resolve and hash the target, or (b) hash the link target string, or (c) reject packages with symlinks. |
| **Hardlinks** | Hash file content, not inode identity. Two hardlinks to the same file produce the same hash independently. |
| **File permissions/mode bits** | Decide: include in hash or exclude. Including prevents executable bit tampering but creates portability issues across OSes. |
| **Empty directories** | Decide: include in hash or exclude. |
| **Timestamps** | **Exclude.** Timestamps are not deterministic across installations. |
| **Special files** | Device files, sockets, FIFOs → reject or exclude. |
| **Signature file/field** | Excluded from digest (see B1). |
| **`.git`, `.DS_Store`, etc.** | Decide: ignore via exclusion list, or require clean packages. |
| **Duplicate paths** | Impossible in a filesystem; not an issue. |

---

### B3. File Integrity Model

**V1 documentation says** ([`05_Plugin_API.md`](file:///c:/Users/Zeyad/PycharmProjects/Telos/docs/05_Plugin_API.md) line 778):
> *"All files within the plugin package directory match the signed manifest of contents. No files have been added, removed, or modified."*

This implies a **content manifest** — a list of all files with their hashes. The signature covers this content manifest.

**The docs do not specify the mechanism.** The Specification Boundaries section (line 797–798) explicitly states:
> *"This specification defines **what** must be verified, not **how**."*

> [!IMPORTANT]
> **ADR Required:** Choose the integrity model:
> - **(a)** Per-file hashes in a flat sorted list. Simplest. Root hash = `H(sorted_concat(path || ":" || file_hash))`.
> - **(b)** Merkle tree. More complex, enables partial verification. Likely overkill for plugin packages.
> - **(c)** Archive hash. Hash the entire package as a `.tar` or `.zip`. Requires deterministic archive creation, which is notoriously difficult.
>
> **Recommendation:** Option (a). Consistent with the existing `crypto.Hash` (SHA-256) and `crypto.VerifyHash` utilities in [`internal/crypto/crypto.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/crypto/crypto.go).

---

### B4. Signature Algorithm

**V1 documentation** (line 797–798):
> *"The choice of signature algorithm, key format, signature encoding, and verification library are implementation decisions."*

The documentation explicitly delegates this to implementation.

**Existing crypto primitives** in [`internal/crypto/crypto.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/crypto/crypto.go):
- SHA-256 hashing (`crypto.Hash`, `crypto.VerifyHash`)
- AES-256-GCM authenticated encryption
- Argon2id key derivation
- **No digital signature primitives** (no Ed25519, RSA, ECDSA, no `crypto/ed25519` import)

> [!IMPORTANT]
> **ADR Required:** Select a signature algorithm. Candidates:
> - **Ed25519** — 32-byte keys, 64-byte signatures, fast, no configuration parameters, deterministic signing. Used by Go module proxy, Minisign, signify.
> - **ECDSA P-256** — Widely supported, more complex, nondeterministic by default.
> - **RSA-2048+** — Large keys/signatures, well-understood, backward compatible.
>
> The ADR must also define the encoding format (e.g., base64-encoded raw bytes, PEM, JWS).

---

### B5. Key Management

**V1 documentation** ([`05_Plugin_API.md`](file:///c:/Users/Zeyad/PycharmProjects/Telos/docs/05_Plugin_API.md) line 747):
> *"Plugin is signed with the Telos project signing key."*

**Threat model** ([`03_Threat_Model.md`](file:///c:/Users/Zeyad/PycharmProjects/Telos/docs/03_Threat_Model.md) SA-5):
> *"The Telos project signing key is not compromised."*

**Unresolved questions (ADR required):**

| Question | Doc Reference | Status |
|---|---|---|
| Where is the official public key stored? | Not specified | **Decision required** |
| Is it embedded in the binary, loaded from config, or fetched remotely? | Not specified | **Decision required** |
| How is key rotation handled? | Not specified | **Decision required** |
| What is the key fingerprint format? | Line 748 mentions "signing key fingerprint" for TOFU display | **Decision required** |
| Can multiple official keys coexist? (e.g., during rotation) | Not specified | **Decision required** |
| How are community signing keys identified? | Line 748: "third-party key" | **Decision required** — key ID format, discovery |
| Can a plugin be signed by multiple keys? | Not specified | **Decision required** |

---

### B6. Community TOFU

**V1 documentation** ([`05_Plugin_API.md`](file:///c:/Users/Zeyad/PycharmProjects/Telos/docs/05_Plugin_API.md) line 748):
> *"On first encounter, the Plugin Host presents the plugin's identity, author, capabilities, and signing key fingerprint to the user. The user must approve installation before the plugin is registered. Approval is persisted so subsequent startups do not re-prompt."*

**What the user is shown and approves:**
- Plugin identity (`id`, `name`, `version`)
- Author
- Capabilities (declared permissions and capability descriptors)
- Signing key fingerprint

**Unresolved questions (ADR required):**

| Question | Status |
|---|---|
| What is stored in the approval record? | **Decision required** |
| Does the approval bind to the signing key fingerprint? | Implied by docs but not explicit |
| Does the approval bind to the specific package hash? | **Decision required** |
| Does the approval bind to the specific version? | **Decision required** |
| If permissions change in an update, is re-approval required? | **Decision required** |
| If the signing key changes, is re-approval required? | Strongly implied (the approval shows the fingerprint) |
| Where is approval stored? | Line 757 says "persisted"; not specified where |
| Format of the approval store? | **Decision required** |

**Critical update scenario:**

```
v1.0.0: filesystem_read: ["/etc/apt"]     → user approves
v1.1.0: filesystem_read: ["/etc/apt", "/"] → ?????
```

If the approval binds only to `(plugin_id, signing_key)`, version 1.1.0 with expanded permissions is automatically trusted. If the approval binds to `(plugin_id, signing_key, permission_hash)`, re-approval is triggered.

> [!WARNING]
> **This is the most security-critical TOFU design decision.** If permission changes do not trigger re-approval, a supply chain attacker who controls the signing key can silently escalate a community plugin's permissions.

---

### B7. Official Plugin Update Semantics

**V1 documentation** (line 747):
> *"Loaded automatically. No user intervention required."*

**Threat model** ([`03_Threat_Model.md`](file:///c:/Users/Zeyad/PycharmProjects/Telos/docs/03_Threat_Model.md) T-05):
> *"Signature verification occurs at the Trust phase during startup. If the plugin package is modified after installation, a subsequent startup re-verifies the signature and detects the tampering."*

**Documented behavior:**
- Signature is verified on **every startup** (not just first install).
- A valid official signature automatically authorizes the plugin regardless of version, permissions, or capabilities — the trust is bound to the signing key, not to specific permissions.
- The security assumption SA-5 is: "The Telos project signing key is not compromised."

**Implication:** If the official key is trusted, any plugin signed with it is trusted with any permissions it declares. This is the documented V1 model.

**No ADR required** — this is explicitly defined by the documentation.

---

### B8. Community Update Semantics

**Not explicitly defined by documentation.**

The docs say approval is persisted (line 748, 757), but do not define update behavior.

> [!IMPORTANT]
> **ADR Required:** Define community update semantics. The approval record must specify which fields it binds to. Scenarios:
>
> | Scenario | Re-approval? |
> |---|---|
> | Same plugin ID, same key, same permissions, new version | Reasonable: No |
> | Same plugin ID, same key, changed permissions, new version | **Must trigger re-approval** |
> | Same plugin ID, different key | **Must trigger re-approval** |
> | Different plugin ID | New plugin — standard TOFU flow |

---

### B9. Trusted vs. Untrusted Parsing

**Documented lifecycle** ([`05_Plugin_API.md`](file:///c:/Users/Zeyad/PycharmProjects/Telos/docs/05_Plugin_API.md) lines 534–547):

```
Discover → Trust → Validate → Register → Dispatch → Teardown
```

**Trust comes before Validate.** This means:

1. **Discover:** Scan directories, find packages with manifest files. No parsing beyond locating the file.
2. **Trust:** Read the signature. Verify package integrity. Classify as Official/Community/Unsigned. Apply trust policy. **Reject untrusted packages here.** Before this point, the manifest content is untrusted.
3. **Validate:** *Now* parse and validate the full manifest (interface version, type, permissions, schema). This is the first time manifest *content* is trusted.
4. **Register:** Index in capability registry.

**What may be inspected before trust:**
- The `signature` field (or signature file) — needed for verification.
- The list of files in the package — needed for integrity checking.
- Minimal metadata for error reporting (plugin ID, path) — but these must not influence security decisions.

**What must not be trusted before verification:**
- `permissions` (filesystem_read, filesystem_write, network, subprocess)
- `type`
- `capabilities`
- `executable`
- `configuration_schema`

**Implementation change required in** [`host.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/host.go):
Currently, `load()` (line 132) does everything in one pass: parse → validate → return. The Trust Phase must be inserted between Discover and the current load logic.

---

### B10. Trust Failure Semantics

**V1 documentation** ([`05_Plugin_API.md`](file:///c:/Users/Zeyad/PycharmProjects/Telos/docs/05_Plugin_API.md) lines 792–794):
> *"A plugin that fails signature verification MUST NOT be loaded. The Plugin Host logs the failure at `audit` severity."*

All trust failures must **fail closed:**

| Failure | Result | Log Severity |
|---|---|---|
| Missing signature | Unsigned → rejected (default) | `audit` |
| Invalid signature | Unsigned → rejected | `audit` |
| Unknown signing key | Cannot classify → rejected | `audit` |
| Modified manifest (hash mismatch) | Integrity failure → rejected | `audit` |
| Modified executable (hash mismatch) | Integrity failure → rejected | `audit` |
| Added file | Package integrity failure → rejected | `audit` |
| Removed file | Package integrity failure → rejected | `audit` |
| Changed permission (if re-verification required) | Hash mismatch → rejected | `audit` |
| Malformed/unparseable signature | Unsigned → rejected | `audit` |
| Unsupported signature format | Unsigned → rejected | `audit` |

---

## 3. Confirmed Security Findings

---

### Finding 1: Missing Trust Phase

| | |
|---|---|
| **Root Cause** | [`host.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/host.go) `Discover()`/`load()` transitions from Discover directly to Validate+Register, skipping the documented Trust Phase entirely. |
| **Attacker Capability** | Any actor with write access to a configured plugin directory can place an unsigned, unverified plugin that will be automatically loaded and granted its declared permissions. |
| **Security Impact** | Critical. Bypasses the entire V1 trust model. Unsigned plugins are loaded as if trusted. No signature verification, no TOFU approval. |
| **Affected Files** | [`host.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/host.go) lines 104–174 |
| **Remediation** | Implement the Trust Phase (Workstream B). Blocked on ADRs. |
| **Regression Test** | Test that an unsigned plugin is rejected; test that a plugin with an invalid signature is rejected; test that a Community plugin without prior approval is rejected. |

---

### Finding 2: Seccomp Jf:1 Denylist Bypass (Historical)

| | |
|---|---|
| **Root Cause** | An earlier version of `buildSeccompFilter()` used `Jf: 1` in the denylist loop, causing false comparisons to skip the next instruction instead of falling through. |
| **Attacker Capability** | A sandboxed plugin could invoke alternating denied syscalls (`mount`, `swapon`, `settimeofday`, `setns`, `bpf`, `adjtimex`) without receiving `EPERM`. |
| **Security Impact** | Critical. Mount, namespace manipulation, and BPF loading were accessible from within the sandbox. |
| **Affected Files** | [`seccomp_linux.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/sandbox/seccomp_linux.go) |
| **Remediation** | Fixed: `Jf: 0` is now used. Must be verified by unit test (Workstream A). |
| **Regression Test** | `TestBuildSeccompFilterInstructions` verifying `Jf == 0` for all denylist entries. |

---

### Finding 3: Invalid clone3 Argument Inspection (Historical)

| | |
|---|---|
| **Root Cause** | An earlier version attempted to inspect `clone3` flags via cBPF, but `clone3` passes flags in a `struct clone_args*` (memory pointer), which cBPF cannot dereference. |
| **Attacker Capability** | A sandboxed plugin could call `clone3` with `CLONE_NEWUSER` or other namespace flags and the filter would not detect them. |
| **Security Impact** | High. Namespace creation within the sandbox was possible via `clone3`. |
| **Affected Files** | [`seccomp_linux.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/sandbox/seccomp_linux.go) |
| **Remediation** | Fixed: `clone3` is now unconditionally denied with `ENOSYS`. Must be verified by unit test (Workstream A). |
| **Regression Test** | `TestBuildSeccompFilterInstructions` verifying `clone3` → `RET ENOSYS`. Linux integration test verifying `clone3` syscall returns `ENOSYS` within sandbox. |

---

### Finding 4: Linux Path Validation Gap (Historical)

| | |
|---|---|
| **Root Cause** | An earlier version of `buildPolicy()` passed raw manifest strings to Landlock without resolving relative paths or checking for directory traversal. |
| **Attacker Capability** | A plugin could declare `"../../etc/shadow"` as a relative path and receive Landlock access to files outside its plugin directory. |
| **Security Impact** | High. Filesystem sandbox escape via path traversal. |
| **Affected Files** | [`paths.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/sandbox/paths.go), [`host.go`](file:///c:/Users/Zeyad/PycharmProjects/Telos/internal/plugin/host.go) |
| **Remediation** | Fixed: `NormalizeManifestPaths()` now resolves symlinks and verifies containment for relative paths. Must be verified by unit tests (Workstream A). |
| **Regression Test** | `TestNormalizeManifestPaths` with traversal, symlink escape, nested traversal cases. |

---

## 4. Security Invariants

1. **Trust invariant:** A plugin must not be loaded unless it has been cryptographically verified as Official or explicitly approved as Community (or overridden as Unsigned per configuration). *Currently violated — see Finding 1.*

2. **Path invariant:** A relative manifest path, after normalization, must resolve to a location inside the plugin directory. An absolute manifest path is an explicit host authorization, allowed only for genuinely trusted plugins.

3. **Seccomp invariant:** Every denied syscall in the denylist reaches `RET EPERM`. `clone3` unconditionally reaches `RET ENOSYS`. `clone()` with any `CLONE_NEW*` flag reaches `RET EPERM`. Default action is `RET ALLOW`.

4. **Landlock invariant:** Landlock ABI v1 is the minimum baseline. If unavailable, plugin execution fails closed. Landlock rules are derived from `Policy` paths, which originate from normalized manifest declarations of a trusted plugin.

5. **Ordering invariant:** `PR_SET_NO_NEW_PRIVS` → `prlimit` → Landlock → Seccomp → `syscall.Exec(plugin)`.

---

## 5. Test Strategy

### 5A. Unit Tests (Runnable Anywhere)

| Test | File | Build Tag |
|---|---|---|
| `TestBuildSeccompFilterInstructions` | `seccomp_linux_test.go` | `linux` |
| `TestNormalizeManifestPaths` (expanded) | `paths_test.go` | none |
| `TestPolicyValidation` (expanded) | `paths_test.go` | none |
| `TestExecutablePath` | `host_test.go` | none |
| `TestDiscoverRejectsUnsigned` (future, Workstream B) | `host_test.go` | none |

> [!NOTE]
> `seccomp_linux_test.go` uses build tag `linux` because `buildSeccompFilter()` imports `golang.org/x/sys/unix` which only compiles on Linux. The test itself does **not** invoke kernel seccomp — it only inspects the generated BPF instruction array. However, it cannot be compiled on non-Linux platforms due to the import.

### 5B. Cross-Platform Tests

| Test | File | Notes |
|---|---|---|
| `TestNormalizeManifestPaths` | `paths_test.go` | Uses `t.TempDir()`, works on all platforms. Symlink test gracefully skips on Windows if symlinks unavailable. |
| `TestPolicyValidation` | `paths_test.go` | Pure logic, no OS dependencies. |
| `TestDiscoverRegistersValidAndSkipsInvalid` | `host_test.go` | Already cross-platform. |
| `TestInvokeUsesJSONProtocol` | `host_test.go` | Skipped on Windows (shell fixture). |

### 5C. Linux Integration Tests (Require Real Linux Kernel)

| Test | File | Requirements |
|---|---|---|
| `TestLinuxSandboxIntegration` | `sandbox_integration_test.go` | Linux ≥5.13, Landlock ABI v1, build tag `linux`. Compiles a real plugin binary, spawns sandbox, verifies kernel enforcement. |

**These tests must execute on an actual Linux kernel.** A `//go:build linux` tag ensures they compile only on Linux, but the kernel must also support Landlock ABI v1 and seccomp for the tests to be meaningful. Running them in a container or VM that lacks these capabilities will produce skips, not false passes.

**Integration tests to add:**
- `clone3` → verify `ENOSYS` from within sandbox.
- `clone(CLONE_NEWUSER)` → verify `EPERM` from within sandbox.
- `mount()` → verify `EPERM` from within sandbox.
- Read from allowed path → succeeds.
- Read from denied path → fails with permission error.

---

## 6. Recommended ADRs

| ADR | Topic | Blocks |
|---|---|---|
| **ADR-0012** | Plugin Signature Format & Integrity Model | Workstream B (all) |
| **ADR-0013** | Plugin Trust Key Management | Workstream B (B5, B7) |
| **ADR-0014** | Community Plugin TOFU & Update Semantics | Workstream B (B6, B8) |

---

## 7. Dependencies Between Workstreams

```
Workstream A (Sandbox Fixes)          Workstream B (Trust Phase)
├── Seccomp verification ──────────── Independent
├── Path validation tests ─────────── Independent
├── selfPath comment fix ──────────── Independent
│                                     ├── ADR-0012 (Signature Format)
│                                     ├── ADR-0013 (Key Management)
│                                     ├── ADR-0014 (TOFU Semantics)
│                                     ├── Implement package hashing
│                                     ├── Implement signature verification
│                                     ├── Insert Trust Phase into host.go
│                                     └── Implement TOFU prompt + storage
```

**No dependency from A → B.** Workstream A can be implemented and merged immediately.

**Dependency from B → ADRs.** Workstream B cannot begin implementation until ADR-0012, ADR-0013, and ADR-0014 are written and accepted.

---

## 8. What Can Be Implemented Immediately

1. `seccomp_linux_test.go` — BPF instruction verification.
2. Expanded `paths_test.go` — traversal, symlink, edge cases.
3. Expanded `host_test.go` — executable validation edge cases.
4. Updated `#nosec` comment on `selfPath` in `sandbox_linux.go`.
5. Expanded integration test cases in `sandbox_integration_test.go` (for Linux environments).

---

## 9. What Must Remain Blocked

1. Trust Phase implementation in `host.go` — blocked on ADR-0012.
2. Package hashing/signing utilities — blocked on ADR-0012.
3. Trusted key storage — blocked on ADR-0013.
4. Community TOFU prompt and approval persistence — blocked on ADR-0014.
5. Trust-related test cases (invalid signature, modified manifest, unapproved community plugin) — blocked on Trust Phase implementation.
6. Config changes (`Config` struct additions for trust policy) — blocked on ADR-0013/0014.
