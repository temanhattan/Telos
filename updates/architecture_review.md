# AERS — Pre-Requirements Architecture Review

> Questions and ambiguities identified in [00_Vision.md](file:///c:/Users/Zeyad/PycharmProjects/AERS/docs/00_Vision.md) and [01_Architecture.md](file:///c:/Users/Zeyad/PycharmProjects/AERS/docs/01_Architecture.md) that must be resolved before writing `00.5_Requirements.md`.

---

## 1. Implementation Language & Runtime

Neither document specifies the implementation language. This is not a minor detail — it fundamentally constrains:

- How plugins are loaded (shared libraries vs. subprocess vs. WASM vs. dynamic import).
- How the Crypto Engine is implemented (bindings to libsodium vs. Go stdlib vs. Python cryptography).
- How the CLI Shell is built (Click, Cobra, clap, etc.).
- How the plugin sandbox works (process isolation vs. in-process guards).
- Whether "zero platform-specific code in core" is enforced by the type system or by convention.

**Question 1:** What is the implementation language for AERS core?

---

## 2. Plugin Isolation Mechanism

The Architecture states plugins are "sandboxed" and "cannot access the filesystem outside their declared scope." But the mechanism is unspecified. The options have drastically different architectural consequences:

| Mechanism | Pros | Cons |
|-----------|------|------|
| **In-process (dynamic import)** | Fast, simple, shared types | Plugin crash = core crash, sandbox is convention only |
| **Subprocess (exec + JSON IPC)** | True isolation, language-agnostic | Slower, serialization overhead, complex error handling |
| **WASM** | Strong sandbox, portable | Limited filesystem access, ecosystem maturity |
| **Shared library (FFI)** | Fast, language-flexible | Unsafe, hard to sandbox |

The Architecture says "a plugin crash does not propagate to the core" — this is only achievable with subprocess or WASM isolation. In-process dynamic imports cannot guarantee this.

**Question 2:** What is the plugin isolation mechanism? Specifically: are plugins in-process or out-of-process?

---

## 3. Credential Isolation — Contradiction

There is a contradiction in the Capture Engine (S7) definition:

- S7 states: *"The Capture Engine does not encrypt. It produces a plaintext archive that is then passed to the Crypto Engine."*
- S7 also states: *"Artifacts flagged with `credential_isolation: true` are captured into a separate, isolated segment of the archive. This segment is encrypted with a distinct key."*

If S7 doesn't encrypt, who encrypts the credential-isolated segment? Two possible interpretations:

1. **S7 segregates, S9 encrypts both segments separately.** The Capture Engine produces two plaintext segments — a general segment and a credential segment — and the Crypto Engine encrypts each with a different key.
2. **S7 does encrypt the credential segment as a special case.** This would violate its own stated boundary.

**Question 3:** Confirm that the Capture Engine produces *two separate plaintext segments* and the Crypto Engine encrypts them independently with different keys. Or define a different flow.

---

## 4. "Compatible Target Machine" — Undefined

The Vision uses the phrase "compatible target machine" repeatedly but never defines what "compatible" means. This is critical for requirements because it sets the boundary of what the Restore Engine must handle.

Possible definitions:

- **Same OS family and version** (Ubuntu 24.04 → Ubuntu 24.04). Strictest. Simplest.
- **Same OS family, any version** (Ubuntu 22.04 → Ubuntu 24.04). Requires version adaptation logic.
- **Same OS family group** (Ubuntu → Debian, Fedora → RHEL). Requires distro adaptation.
- **Any OS** (Ubuntu → Windows). Vision's Phase 3 mentions "cross-platform restore adaptation" — but is this in scope for Phase 1?

**Question 4:** For Phase 1 (Foundation), what is the definition of a "compatible target machine"? Same OS family + version? Same family, any version? Something else?

---

## 5. Restore Target State

The Architecture assumes the Restore Engine writes to a target machine, but doesn't specify the expected starting state:

- **Clean OS install only?** The target must be a freshly installed OS with nothing else on it.
- **Existing environment?** The target may have existing packages, configs, and data. The Restore Engine must handle conflicts (overwrite? merge? skip?).

The Planner's conflict detection suggests existing environments are expected, but this is never stated explicitly. The answer affects nearly every restore requirement.

**Question 5:** Can AERS restore onto a machine that already has an existing environment, or only onto a clean OS install?

---

## 6. Differential & Incremental Backups — Architecture Gap

The Vision lists "Differential and incremental backups" as in-scope. The Architecture defines no mechanism for this:

- How does AERS know what changed since the last backup? Does it store the previous manifest for comparison?
- Does a differential backup produce a standalone archive, or does it depend on a prior full backup for restore?
- Where is the baseline manifest stored between backups? In the Storage Backend alongside the archive?
- The Planner, Capture Engine, and Storage Backend sections make no mention of differential behavior.

**Question 6a:** Are differential backups in scope for Phase 1, or deferred?
**Question 6b:** If in scope — does a differential backup produce a *dependent* archive (requires the full backup to restore) or a *standalone* archive (self-contained but smaller because unchanged data is referenced, not copied)?

---

## 7. Key & Passphrase Management

The Crypto Engine section states keys are "received at operation time and discarded when the operation completes." But the user experience is unspecified:

- **Backup:** User provides passphrase at backup time. Clear.
- **Credential segment:** User provides a *second* passphrase for the isolated segment? Or is it derived from the main passphrase?
- **Restore:** User provides the same passphrase(s). Clear.
- **Scheduled backups:** The Scheduler can run unattended. How does it get the passphrase? Options:
  - Prompt the user at schedule-registration time and cache it in memory for the session — but what about reboots?
  - Read from an OS keychain (macOS Keychain, Windows Credential Manager, GNOME Keyring).
  - Read from a key file on disk — which the Vision's security philosophy would likely reject.

**Question 7a:** Is the credential-isolated segment encrypted with a *separate user-provided passphrase* or *derived from the main passphrase*?
**Question 7b:** How do scheduled (unattended) backups obtain the encryption passphrase?

---

## 8. Archive Identity & Naming

Archives must be retrievable by "identifier" (S8), but the identification scheme is undefined:

- UUID?
- Timestamp + hostname?
- User-provided label?
- Auto-generated name with user-editable alias?

This affects the CLI UX (`aers restore --archive ???`), the Storage Backend's listing behavior, and differential backup chaining.

**Question 8:** What is the archive identification scheme?

---

## 9. User Data Designation

The Discovery Engine has a "User Data" category for "user-designated directories and files marked for inclusion." But the designation mechanism is unspecified:

- Configuration file listing paths (e.g., `include_paths: [~/projects, ~/documents]`)?
- Interactive prompt during discovery (e.g., "I found these large directories — which should I include?")?
- Convention-based defaults (e.g., always include `~/Documents`, `~/Desktop`, `~/Projects`)?

**Question 9:** How does the user designate which data directories to include? Config file only? Interactive discovery? Both?

---

## 10. Config File Format

The Configuration Manager references `~/.aers/config.yaml` and `/etc/aers/config.yaml` — suggesting YAML. But this is an example, not a decision.

**Question 10:** Is YAML the chosen configuration format, or should this be decided?

---

## 11. Manifest Storage Between Operations

The Diff Engine and Verification Engine both require access to *previous* manifests for comparison. But the Architecture doesn't specify where manifests are stored between operations:

- Inside the backup archive only (meaning you must decrypt an archive to diff)?
- Also persisted locally in a manifest store (e.g., `~/.aers/manifests/`)?
- Both?

If manifests are only inside encrypted archives, the `aers verify` and `aers diff` commands become expensive (decrypt, extract manifest, compare, discard). If manifests are stored locally, there's a security question: the manifest contains a complete inventory of the machine — is it sensitive?

**Question 11:** Are Environment Manifests stored locally outside of archives (for fast diff/verify), or only inside encrypted archives?

---

## 12. Approval Gate — Modification Flow

The Approval Gate accepts `approve`, `reject`, or `modify`. The Vision and Architecture describe approve and reject flows, but **modify** is never defined:

- What can the user modify? Individual plan entries (include/exclude)?
- Does modification re-invoke the Planner, or does the user directly edit the plan?
- Is modification interactive (CLI prompts) or file-based (export plan → edit → re-import)?

**Question 12:** What does the `modify` action in the Approval Gate look like? What can the user change, and how?

---

## 13. Offline-First vs. Package Restoration

The Vision states "all operations must work offline." The Architecture's Restore Engine Phase 1 is "Package Installation" — which typically requires downloading packages from the internet (apt, winget, pip).

Two interpretations:

1. **Offline means the AERS tool itself works offline** — but restore may require internet for package downloads. The Vision already acknowledges this: "Network access may be required for downloading packages during restore."
2. **AERS should cache package binaries in the archive** — contradicting "Minimum Backup Size, Maximum Recoverability" (store names, not binaries).

This seems already resolved by the Vision's own caveat, but it should be stated explicitly in requirements to avoid confusion.

**Question 13:** Confirm: "Offline-first" means AERS's own logic runs offline, but restore-time package installation may require network access. Correct?

---

## Summary

| # | Topic | Blocking? |
|---|-------|-----------|
| 1 | Implementation language | **Yes** — affects plugin mechanism, crypto, CLI, sandbox |
| 2 | Plugin isolation mechanism | **Yes** — in-process vs. subprocess changes everything |
| 3 | Credential isolation flow | **Yes** — contradiction must be resolved |
| 4 | "Compatible target" definition | **Yes** — defines restore scope |
| 5 | Restore target state | **Yes** — clean install vs. existing environment |
| 6 | Differential backup mechanism | Partially — can defer, but must decide if in scope |
| 7 | Key/passphrase management | **Yes** — especially for scheduled backups |
| 8 | Archive identification scheme | Moderate — affects CLI UX and storage |
| 9 | User data designation | Moderate — affects discovery and config |
| 10 | Config file format | Low — YAML is a reasonable default |
| 11 | Manifest storage location | Moderate — affects diff/verify performance and security |
| 12 | Approval Gate modification flow | Moderate — can be basic for Phase 1 |
| 13 | Offline-first clarification | Low — Vision already addresses this |
