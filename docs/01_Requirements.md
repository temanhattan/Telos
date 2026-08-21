# Telos — Software Requirements Specification

> **Status:** Draft
> **Last Updated:** 2026-07-01
> **Owner:** temanhattan
> **Audience:** All future contributors, AI sessions, and design reviewers
> **Source of Truth:** [00_Vision.md](00_Vision.md), [02_Architecture.md](02_Architecture.md)

---

## Table of Contents

1. [Purpose](#purpose)
2. [Scope](#scope)
3. [Definitions and Abbreviations](#definitions-and-abbreviations)
4. [Overall Description](#overall-description)
5. [Constraints](#constraints)
6. [Functional Requirements](#functional-requirements)
   - [FR-1: Environment Discovery](#fr-1-environment-discovery)
   - [FR-2: Environment Classification](#fr-2-environment-classification)
   - [FR-3: Backup Planning](#fr-3-backup-planning)
   - [FR-4: Backup Execution](#fr-4-backup-execution)
   - [FR-5: Restore Planning](#fr-5-restore-planning)
   - [FR-6: Restore Execution](#fr-6-restore-execution)
   - [FR-7: Post-Restore Verification](#fr-7-post-restore-verification)
   - [FR-8: Environment Diffing and Audit](#fr-8-environment-diffing-and-audit)
   - [FR-9: Archive Management](#fr-9-archive-management)
   - [FR-10: Scheduling](#fr-10-scheduling)
   - [FR-11: Configuration](#fr-11-configuration)
   - [FR-12: Plugin System](#fr-12-plugin-system)
   - [FR-13: AI Advisory](#fr-13-ai-advisory)
   - [FR-14: User Interface](#fr-14-user-interface)
7. [Non-Functional Requirements](#non-functional-requirements)
   - [NFR-1: Security](#nfr-1-security)
   - [NFR-2: Reliability](#nfr-2-reliability)
   - [NFR-3: Performance](#nfr-3-performance)
   - [NFR-4: Portability](#nfr-4-portability)
   - [NFR-5: Extensibility](#nfr-5-extensibility)
   - [NFR-6: Usability](#nfr-6-usability)
   - [NFR-7: Auditability](#nfr-7-auditability)
   - [NFR-8: Offline Operation](#nfr-8-offline-operation)
8. [Data Requirements](#data-requirements)
   - [DR-1: Environment Manifest](#dr-1-environment-manifest)
   - [DR-2: Backup Archive](#dr-2-backup-archive)
   - [DR-3: Backup and Restore Plans](#dr-3-backup-and-restore-plans)
   - [DR-4: Verification Report](#dr-4-verification-report)
   - [DR-5: Diff Report](#dr-5-diff-report)
   - [DR-6: Audit Log](#dr-6-audit-log)
9. [External Interface Requirements](#external-interface-requirements)
   - [EIR-1: User Interface](#eir-1-user-interface)
   - [EIR-2: Plugin Interface](#eir-2-plugin-interface)
   - [EIR-3: Storage Interface](#eir-3-storage-interface)
   - [EIR-4: Operating System Interface](#eir-4-operating-system-interface)
10. [Version 1 Scope Boundaries](#version-1-scope-boundaries)
11. [Traceability Matrix](#traceability-matrix)

---

## Purpose

This document defines the software requirements for Telos — the Intent-Driven Environment Recovery & Reconstruction System. It specifies **what** the system must accomplish, the conditions it must satisfy, and the constraints it must respect.

This document does **not** specify how the system will be implemented. Implementation details, technology choices, and internal design decisions are the domain of the Architecture document and subsequent design artifacts.

All requirements are derived from the [Project Vision](00_Vision.md) and the [System Architecture](02_Architecture.md). Where architecture decisions have been fixed, they are treated as constraints and reflected in requirements without alteration.

---

## Scope

Telos is an intelligent environment backup and restoration system. It discovers the complete state of a computing environment — operating system, installed software, active services, user configurations, credentials, scheduled tasks, network settings, and user data — and produces a secure, encrypted backup archive from which a functionally equivalent environment can be reconstructed on a compatible target machine.

### In Scope (Version 1)

- Full-backup operations (discovery, planning, capture, encryption, storage).
- Restore operations to compatible target machines.
- Post-restore verification.
- Environment diffing and audit.
- CLI-based user interface.
- Plugin-based extensibility.
- Optional AI-assisted advisory features.
- Scheduled backup automation.

### Out of Scope (Version 1)

- Differential and incremental backups.
- Graphical user interface.
- Full disk imaging or block-level backup.
- Real-time synchronization.
- Multi-user or multi-tenant operation.
- Mobile device backup (iOS, Android).
- Bare-metal operating system provisioning.
- Proprietary application state backup (unless a dedicated plugin exists).
- Cross-platform restoration (e.g., Linux backup restored to Windows).

---

## Definitions and Abbreviations

| Term | Definition |
| ------ | ----------- |
| **Environment** | The complete state of a computing system: its operating system, installed software, configurations, credentials, data, and services. |
| **Environment Manifest** | A structured, machine-readable description of a discovered environment. |
| **Backup Archive** | An encrypted, integrity-verified file containing everything needed to reconstruct an environment. |
| **Backup Plan** | A human-reviewable specification of what will be captured during a backup operation. |
| **Restore Plan** | A human-reviewable specification of what will be performed on a target machine during restoration. |
| **Execution Plan** | The general term for either a Backup Plan or a Restore Plan — any plan generated before an operation that the user must review. |
| **Compatible Machine** | A target machine running the same operating system family and CPU architecture as the source machine. |
| **Approval Gate** | The user-facing checkpoint where an execution plan must receive explicit human approval before any action is taken. |
| **Plugin** | An extension module that provides platform-specific functionality without modifying the core system. |
| **Credential Isolation** | The separation of sensitive materials (private keys, tokens, secrets) into a distinct security domain within the backup archive. |
| **Discovery** | The process of scanning a machine to identify its environment components. |
| **Verification Report** | A structured comparison of a restored environment against the original, classifying differences by severity. |
| **Diff Report** | A structured comparison of two Environment Manifests showing additions, removals, and modifications. |

---

## Overall Description

### Product Perspective

Telos is a standalone application. It is not a component of a larger system. It interacts with the host operating system to discover environment state, with storage media to persist backup archives, and optionally with external package managers during restoration. It has no mandatory dependency on network services.

### User Profile

The primary user is a technically sophisticated individual who manages multiple personal computing environments across operating systems, virtual machines, and cloud instances. The user is comfortable with terminal interfaces, values transparency, and expects backups to be secure enough for storage on untrusted media.

### Operating Environment

Telos must support the following operating environments in Version 1:

| Category | Supported Environments |
| ---------- | ---------------------- |
| **Operating Systems** | Ubuntu, Debian, Windows |
| **Machine Types** | Physical hardware, virtual machines, cloud instances |
| **Storage Media** | Local filesystem, external drives, network-attached storage |

### Assumptions

1. A base operating system is already installed on the target machine before restoration begins.
2. The user has sufficient privileges on the source machine to read the data they wish to back up.
3. The user has sufficient privileges on the target machine to perform the approved restore actions.
4. External package repositories referenced in the backup remain available at restore time (for packages captured by reference).

---

## Constraints

The following constraints are fixed decisions that shape all requirements in this document. They must not be changed or questioned.

| # | Constraint | Rationale |
| --- | ----------- | ----------- |
| C-1 | Version 1 supports restoration only to compatible machines: same operating system family and same CPU architecture. | Establishes a reliable restoration baseline before addressing cross-platform adaptation. |
| C-2 | Restoration assumes a freshly installed operating system on the target machine. | Eliminates the complexity of merging with an existing environment. |
| C-3 | Differential and incremental backups are outside the scope of Version 1. | Focused delivery; these will be introduced in future versions. |
| C-4 | The system uses a single master passphrase for backup security, while internally separating cryptographic material for different security domains. | Balances usability (one passphrase to remember) with defense-in-depth (isolated key material). |
| C-5 | Every backup archive must have a globally unique identifier (UUID), a creation timestamp, and an optional user-defined label. | Enables unambiguous archive identification and human-friendly naming. |
| C-6 | User data selection supports interactive selection during the initial backup and persistent configuration for subsequent backups. | Respects user agency on first run; reduces friction on subsequent runs. |
| C-7 | Environment manifests are stored inside the encrypted backup archive and inside a local encrypted manifest cache. | Enables efficient comparison without decrypting full archives. |
| C-8 | Before executing any backup or restore operation, the system must generate an execution plan that the user can review, modify, approve, or cancel. | Enforces human authority over all operations. |
| C-9 | The application is offline-first. The application itself must operate without Internet connectivity. External package managers may require Internet access during software restoration. | Ensures the tool works in air-gapped and disconnected environments. |
| C-10 | The project follows a plugin-based architecture. All environment-specific functionality is provided through plugins while the core remains platform-independent. | Enables extensibility without core modification. |

---

## Functional Requirements

---

### FR-1: Environment Discovery

The system must discover the complete state of a computing environment before any backup operation.

| ID | Requirement |
| ---- | ------------ |
| FR-1.1 | The system must identify the host operating system family, version, and CPU architecture. |
| FR-1.2 | The system must catalog all installed software packages across all detected package managers, recording the package name, version, source repository, and whether the package was explicitly installed or installed as a dependency. |
| FR-1.3 | The system must identify running services and daemons, including their current status (running, stopped, disabled), startup behavior, and associated configuration file paths. |
| FR-1.4 | The system must detect user configuration files, including shell configurations, editor settings, and desktop environment preferences, and determine whether each file has been modified from its default state. |
| FR-1.5 | The system must detect the presence and location of sensitive credential materials: private keys, public keys, API tokens, and certificates. The system must not read credential file contents during discovery; content capture occurs only during the backup execution phase under credential isolation. |
| FR-1.6 | The system must capture environment variables (system-scope and user-scope), shell aliases, and shell functions. |
| FR-1.7 | The system must discover scheduled tasks, including their schedule expressions and associated commands. |
| FR-1.8 | The system must discover network configuration: interface settings, DNS configuration, VPN profiles, firewall rules, and proxy settings. |
| FR-1.9 | The system must discover cloud instance metadata where applicable: provider identity, instance identifier, region, and attached roles or policies. |
| FR-1.10 | The system must identify user-designated data directories and files marked for inclusion in the backup. |
| FR-1.11 | Discovery must be a read-only operation. The discovery process must never modify the source system. |
| FR-1.12 | Discovery must operate without network access. |
| FR-1.13 | If discovery of any individual category fails, the system must continue discovering remaining categories, mark the failed category as incomplete, and present warnings to the user. |
| FR-1.14 | All discovery results must be assembled into a single, structured Environment Manifest (see [DR-1](#dr-1-environment-manifest)). |

---

### FR-2: Environment Classification

After discovery, the system must classify and annotate the Environment Manifest to support intelligent planning.

| ID | Requirement |
| ---- | ------------ |
| FR-2.1 | The system must determine the overall role of the machine (e.g., developer workstation, web server, database server, cybersecurity lab, general desktop) based on the combination of installed packages, running services, and configuration patterns. |
| FR-2.2 | The system must assign an importance level to each manifest entry: `critical`, `recommended`, `optional`, or `transient`. |
| FR-2.3 | The system must annotate each manifest entry with its inferred intent where determinable (e.g., "reverse proxy," "Python development toolchain," "user shell customization"). |
| FR-2.4 | The system must tag each manifest entry as `reproducible` (can be reinstalled from a known external source) or `irreplaceable` (must be captured verbatim because no external source exists). |
| FR-2.5 | Classification must follow a defined priority hierarchy: deterministic rules take precedence over plugin-provided rules, which take precedence over AI-generated suggestions. |
| FR-2.6 | The user must be able to override any classification decision. |
| FR-2.7 | Classification must add only annotations to the manifest. It must never alter the discovered facts recorded during discovery. |
| FR-2.8 | Classification must function fully without the AI advisory capability. If AI is unavailable or disabled, classification must fall back to deterministic and plugin-provided rules with no loss of core functionality. |

---

### FR-3: Backup Planning

Before any backup operation, the system must generate a Backup Plan for user review.

| ID | Requirement |
| ---- | ------------ |
| FR-3.1 | The system must generate a Backup Plan that lists every artifact to be captured, the method of capture, the estimated size contribution, and the reason for inclusion. |
| FR-3.2 | The system must support three capture methods: (a) reference capture — record the package name, version, and source repository without copying the binary; (b) file copy — include the file contents in the archive; (c) configuration export — serialize a service or application's configuration through a plugin-provided mechanism. |
| FR-3.3 | The Backup Plan must prefer reference capture over file copy for any artifact that is available in a known external repository. |
| FR-3.4 | The Backup Plan must list every manifest entry excluded from the backup and provide the reason for exclusion (e.g., "default configuration — unchanged from OS baseline," "reproducible from external repository"). |
| FR-3.5 | The Backup Plan must flag all credential materials and indicate that they will be captured under credential isolation. |
| FR-3.6 | The Backup Plan must incorporate user-defined include/exclude rules from persistent configuration, overriding classifier annotations where the user has expressed an explicit preference. |
| FR-3.7 | The Backup Plan must provide an estimated total archive size. |
| FR-3.8 | The system must present the Backup Plan to the user through an Approval Gate before execution. The user must be able to approve, reject, or modify the plan. |
| FR-3.9 | On the initial backup, the system must allow the user to interactively select which user data directories and files to include. |
| FR-3.10 | User data selections made during the initial backup must be persisted as configuration so that subsequent backups use the same selections without requiring re-selection. |
| FR-3.11 | The user must be able to modify persisted data selections at any time. |

---

### FR-4: Backup Execution

After the user approves a Backup Plan, the system must execute the plan and produce a backup archive.

| ID | Requirement |
| ---- | ------------ |
| FR-4.1 | The system must execute the approved Backup Plan exactly as specified. It must not capture artifacts not listed in the plan, and must not omit artifacts that are listed. |
| FR-4.2 | The system must compute a cryptographic hash for every captured artifact immediately after reading it from the source. |
| FR-4.3 | Credential materials flagged for credential isolation must be captured into a logically separate segment of the archive, distinct from general configuration data. |
| FR-4.4 | The system must assemble the captured artifacts, the Environment Manifest, the Backup Plan, and the integrity hash manifest into a single structured archive. |
| FR-4.5 | If an individual artifact cannot be captured (e.g., permission denied, file no longer exists), the system must log the failure, mark the entry as failed in the archive, and continue with the remaining artifacts. |
| FR-4.6 | Backup execution must be a read-only operation on the source system. It must never modify the source. |
| FR-4.7 | The system must encrypt the complete archive before writing it to storage. No backup data may be written to storage in plaintext. |
| FR-4.8 | The system must support optional digital signing of backup archives for tamper evidence. |
| FR-4.9 | Every backup archive must be assigned a globally unique identifier (UUID). |
| FR-4.10 | Every backup archive must record its creation timestamp. |
| FR-4.11 | The user must be able to attach an optional label to the backup archive at creation time. |
| FR-4.12 | The Environment Manifest must be stored inside the encrypted backup archive. |
| FR-4.13 | A copy of the Environment Manifest must also be stored in a local encrypted manifest cache to enable efficient comparison without decrypting the full archive. |
| FR-4.14 | The system must report progress to the user during backup execution. |
| FR-4.15 | Version 1 performs full backups only. Differential and incremental backup capabilities are deferred to future versions. |

---

### FR-5: Restore Planning

Before any restore operation, the system must generate a Restore Plan for user review.

| ID | Requirement |
| ---- | ------------ |
| FR-5.1 | The system must retrieve the requested backup archive, verify its digital signature (if signed), decrypt it, and verify the integrity of all artifacts before generating a Restore Plan. If any verification step fails, the system must halt and notify the user. |
| FR-5.2 | The system must generate a Restore Plan that lists every action to be performed on the target machine, organized into ordered phases with dependency-aware sequencing within each phase. |
| FR-5.3 | The Restore Plan must include the following phases in order: (1) Package Installation, (2) Configuration Application, (3) Credential Restoration, (4) Data Restoration, (5) Service Configuration, (6) Environment and Scheduled Task Setup. |
| FR-5.4 | The Restore Plan must identify potential conflicts between the backup state and the target machine's current state (e.g., a package version that is no longer available, a configuration file that already exists). |
| FR-5.5 | The Restore Plan must specify a rollback action for each restore action, defining what to do if that action fails. |
| FR-5.6 | The Restore Plan must mark each action as destructive or non-destructive. |
| FR-5.7 | The system must present the Restore Plan to the user through an Approval Gate before execution. The user must be able to approve, reject, or modify the plan. |
| FR-5.8 | Version 1 supports restoration only to compatible machines: the target must have the same operating system family and CPU architecture as the source. The system must verify compatibility before generating the Restore Plan and refuse to proceed if the target is incompatible. |
| FR-5.9 | Version 1 assumes a freshly installed operating system on the target machine. |

---

### FR-6: Restore Execution

After the user approves a Restore Plan, the system must execute the plan on the target machine.

| ID | Requirement |
| ---- | ------------ |
| FR-6.1 | The system must execute the Restore Plan strictly as approved. It must not execute any action not present in the plan. |
| FR-6.2 | The system must execute restore phases in the defined order: Packages → Configuration → Credentials → Data → Services → Environment and Scheduled Tasks. |
| FR-6.3 | Within each phase, the system must respect dependency ordering: an action must not execute until all actions it depends on have completed successfully. |
| FR-6.4 | The system must delegate platform-specific restore actions (e.g., package installation, service management) to the appropriate plugins. |
| FR-6.5 | For each completed action, the system must record a checkpoint. If the restore is interrupted, subsequent invocations must be able to resume from the last checkpoint without re-executing completed actions. |
| FR-6.6 | If an action fails, the system must offer the user choices: skip the failed action, retry it, roll back the current phase, or abort the restore entirely. |
| FR-6.7 | If a required package is not available in the target's repositories, the system must report the failure. It must not substitute an alternative without user approval. |
| FR-6.8 | Credential materials must be decrypted using a separate security domain from the main archive, requiring appropriate authentication from the user. |
| FR-6.9 | Restore operations must be idempotent within a phase: re-running a phase that has already completed must detect existing state and skip already-applied actions without errors or duplication. |
| FR-6.10 | The system must report progress to the user during restore execution. |
| FR-6.11 | The system must log the outcome of every restore action (success, failure, skipped) for audit purposes. |

---

### FR-7: Post-Restore Verification

After restore execution completes, the system must verify the restored environment.

| ID | Requirement |
| ---- | ------------ |
| FR-7.1 | The system must perform a fresh discovery scan of the target machine after restoration completes. |
| FR-7.2 | The system must compare the post-restore Environment Manifest against the original source Environment Manifest. |
| FR-7.3 | The system must classify each difference into one of the following severity levels: (a) **Expected** — a known difference due to platform adaptation; (b) **Acceptable** — a minor difference that does not affect functionality; (c) **Concerning** — a difference that may indicate an incomplete restore; (d) **Critical** — a difference that indicates a restore failure. |
| FR-7.4 | The system must produce a structured Verification Report that summarizes all differences by severity and presents a pass/fail verdict. |
| FR-7.5 | The pass/fail verdict must be based on configurable tolerance thresholds. |
| FR-7.6 | The Verification Report must be presented to the user. |
| FR-7.7 | The Verification Engine reports problems; it does not fix them. Remediation is a user decision. |

---

### FR-8: Environment Diffing and Audit

The system must support comparing environment states for audit and drift detection.

| ID | Requirement |
| ---- | ------------ |
| FR-8.1 | The system must be able to compare two Environment Manifests and produce a structured Diff Report showing all additions, removals, and modifications, organized by manifest section. |
| FR-8.2 | The system must support comparing the current live environment against the most recent backup manifest, using the local encrypted manifest cache to avoid decrypting the full archive. |
| FR-8.3 | The Diff Report must include a summary: total additions, removals, and modifications by section. |
| FR-8.4 | The system must support filtering the diff by specific manifest sections or importance levels. |

---

### FR-9: Archive Management

The system must provide capabilities for managing backup archives.

| ID | Requirement |
| ---- | ------------ |
| FR-9.1 | The system must support listing all available backup archives with their metadata: UUID, creation timestamp, label (if any), source hostname, and archive size. |
| FR-9.2 | The system must support configurable retention policies that define how many archives to keep or how long to retain them. |
| FR-9.3 | The system must support cleanup of expired archives according to the configured retention policy. |
| FR-9.4 | The system must monitor available storage space and warn the user when space is low. |
| FR-9.5 | The system must verify archive integrity on retrieval: before returning an archive for restore, confirm that the archive's integrity hash matches. |
| FR-9.6 | The default storage location must be the local filesystem. Additional storage backends (external media, network-attached storage) must be supported through the plugin interface. |

---

### FR-10: Scheduling

The system must support time-based automation of backup operations.

| ID | Requirement |
| ---- | ------------ |
| FR-10.1 | The user must be able to define recurring backup schedules. |
| FR-10.2 | The system must evaluate schedule triggers and initiate backup operations at the appropriate times. |
| FR-10.3 | The first execution of a scheduled backup must present the Backup Plan through the Approval Gate for user review. |
| FR-10.4 | Subsequent executions of an unchanged scheduled backup may run unattended if the user has explicitly opted in to auto-approval for that schedule. |
| FR-10.5 | If the machine was unavailable when a scheduled backup was due, the system must run the missed backup when the machine next becomes available. |
| FR-10.6 | The system must ensure that only one backup operation runs at a time. If a scheduled backup is triggered while another is in progress, the system must queue it or skip it according to configuration. |
| FR-10.7 | Scheduling is optional. The system must function fully without any scheduled backups; all operations must be triggerable manually. |

---

### FR-11: Configuration

The system must support flexible configuration through multiple sources.

| ID | Requirement |
| ---- | ------------ |
| FR-11.1 | The system must support configuration from the following sources, listed in descending priority: (1) per-invocation command-line flags, (2) session-level environment variables, (3) persistent per-user configuration file, (4) machine-wide system configuration file, (5) built-in defaults. |
| FR-11.2 | The system must merge configuration from all sources according to the priority hierarchy, with higher-priority sources overriding lower-priority sources. |
| FR-11.3 | The system must validate the merged configuration against a schema: rejecting unknown keys, enforcing required fields, and validating types and value ranges. |
| FR-11.4 | If the configuration is invalid, the system must refuse to start and report the specific validation error. |
| FR-11.5 | The system must support configuration introspection: the user must be able to view the effective merged configuration and understand which source each value came from. |
| FR-11.6 | The system must provide sensible built-in defaults that allow basic operation without any user-provided configuration. |

---

### FR-12: Plugin System

All environment-specific functionality must be provided through plugins.

| ID | Requirement |
| ---- | ------------ |
| FR-12.1 | The system must discover and register plugins from configured directories at startup. |
| FR-12.2 | Each plugin must declare the plugin interface version it targets. The system must refuse to load plugins targeting an incompatible interface version. |
| FR-12.3 | Each plugin must declare its capabilities: which operating system families, package managers, cloud providers, or other domains it supports. |
| FR-12.4 | The system must support the following plugin types: Discovery, Classification Rule, Capture, Restore, and Storage. |
| FR-12.5 | Plugins must interact with the core only through defined plugin interfaces. Plugins must not have direct access to core subsystems, storage, or cryptographic material. |
| FR-12.6 | A plugin failure (crash, timeout, or invalid output) must be isolated. It must never cause the core system to crash. The system must log the failure, report partial results, and continue with remaining plugins. |
| FR-12.7 | The system must support adding new operating system, package manager, or cloud provider support by installing new plugins without modifying any core code. |
| FR-12.8 | The core system must contain zero platform-specific logic. All platform knowledge must reside in plugins. |

---

### FR-13: AI Advisory

The system must support optional AI-assisted advisory features.

| ID | Requirement |
| ---- | ------------ |
| FR-13.1 | The AI advisory capability must provide non-binding suggestions only. It must never execute actions directly. |
| FR-13.2 | Every AI-generated suggestion must be tagged with a confidence score and accompanied by a human-readable explanation of the reasoning. |
| FR-13.3 | All AI suggestions must be overridable by deterministic rules and by user decisions. The trust hierarchy is: Human Decision > Deterministic Logic > AI Suggestion. |
| FR-13.4 | The system must be fully functional with the AI advisory capability disabled. Disabling AI must not degrade any core functionality. |
| FR-13.5 | AI advisory must support the following domains: environment classification suggestions, importance scoring suggestions, anomaly flagging during diff operations, and restore conflict prediction. |
| FR-13.6 | The AI advisory capability must operate locally by default, without network access. Cloud-based AI inference must be opt-in. |
| FR-13.7 | When operating in cloud mode, the AI advisory capability must never transmit backup contents, credential materials, or identifiable file paths. Only anonymized structural metadata may be transmitted. |
| FR-13.8 | All AI reasoning must be logged for auditability. |

---

### FR-14: User Interface

The system must provide a command-line interface as the primary interaction model.

| ID | Requirement |
| ---- | ------------ |
| FR-14.1 | The system must provide a CLI as its sole user interface in Version 1. |
| FR-14.2 | The CLI must support the following top-level operations: discover, backup, restore, verify, diff, schedule, and configuration inspection. |
| FR-14.3 | The CLI must support an interactive output mode (with formatting, color, and progress indicators), a plain output mode (suitable for piping to other tools), and a structured data output mode (for programmatic consumption). |
| FR-14.4 | The CLI must implement the Approval Gate: for any operation that writes, modifies, or deletes data, the CLI must present the execution plan, highlight destructive actions with explicit warnings, and block execution until the user provides one of: approve, reject, or modify. |
| FR-14.5 | The CLI must not contain business logic. It must function solely as a translation layer between the user and the system's internal operations. |

---

## Non-Functional Requirements

---

### NFR-1: Security

| ID | Requirement |
| ---- | ------------ |
| NFR-1.1 | All backup archives must be encrypted before being written to any storage medium. No backup data may be stored in plaintext outside the source machine, including in local staging areas. |
| NFR-1.2 | All backup archives must include a cryptographic integrity manifest listing the hash of every included artifact. |
| NFR-1.3 | Before restore begins, the system must verify the integrity manifest against the archive contents. If any artifact fails verification, the restore must halt and the user must be notified. The system must never silently restore corrupted or tampered data. |
| NFR-1.4 | The system must use a single master passphrase for user authentication. Internally, the system must derive separate cryptographic material for different security domains (general data vs. credential-isolated data). |
| NFR-1.5 | Credential materials (private keys, API tokens, secret keys, database passwords) must receive a higher tier of protection than general configuration files. Credentials must be encrypted with separate cryptographic material and must never be co-mingled with general configuration data in the archive structure. |
| NFR-1.6 | Credentials must never be included in a backup without explicit user acknowledgment. |
| NFR-1.7 | Encryption keys and passphrases must not be stored persistently by the system. They must be received at operation time and discarded when the operation completes. |
| NFR-1.8 | Encryption keys and passphrases must never be transmitted over any network. |
| NFR-1.9 | The system must support optional digital signing of backup archives for tamper evidence and non-repudiation. |
| NFR-1.10 | The system must request only the permissions it needs for the current operation. Discovery must use read-only access. Write access must be requested only during restore, and only for the specific paths being restored. |
| NFR-1.11 | Plugins must operate within a sandbox. They must not access the filesystem outside their declared scope, must not access cryptographic material directly, and must not make network requests unless explicitly authorized. |

---

### NFR-2: Reliability

| ID | Requirement |
| ---- | ------------ |
| NFR-2.1 | If a restore is interrupted (power loss, crash, user abort), the system must be able to resume from the point of interruption without corruption or duplication on the next invocation. |
| NFR-2.2 | Partial failure during discovery or capture must not abort the entire operation. The system must continue with remaining items, mark failures, and report them to the user. |
| NFR-2.3 | Partial failure must never produce an inconsistent state on the target machine. |
| NFR-2.4 | A plugin failure must never crash the core system. Plugin failures must be isolated, logged, and reported as partial results. |
| NFR-2.5 | If an encryption operation fails, the system must halt immediately. No partial archive may be written to storage. |
| NFR-2.6 | If a storage write operation fails, the system must halt immediately and report the failure with diagnostics. |

---

### NFR-3: Performance

| ID | Requirement |
| ---- | ------------ |
| NFR-3.1 | A typical development environment on Ubuntu must be restorable from a Telos backup within 30 minutes, excluding download time for large packages from external repositories. |
| NFR-3.2 | A typical development environment on Windows must be restorable from a Telos backup within 45 minutes, excluding download time for large packages from external repositories. |
| NFR-3.3 | The backup archive for a typical development laptop must be less than 20% of the total disk usage of the source machine. |

---

### NFR-4: Portability

| ID | Requirement |
| ---- | ------------ |
| NFR-4.1 | The core system must contain zero platform-specific code. All platform-specific behavior must be provided through plugins. |
| NFR-4.2 | The core system must be capable of running on any platform supported by the chosen implementation language. |
| NFR-4.3 | Backup archives must be portable: an archive produced on one machine must be restorable on any compatible machine (same OS family and CPU architecture) regardless of where the archive was stored. |

---

### NFR-5: Extensibility

| ID | Requirement |
| ---- | ------------ |
| NFR-5.1 | A plugin for a new Linux distribution must be developable and integrable without modifying any core source files. |
| NFR-5.2 | The plugin interface must be versioned to support backward compatibility as the interface evolves. |
| NFR-5.3 | The system must define formal extensibility points for: new OS support, new package manager support, new storage backends, and new classification rules. All extension must happen through these points. |

---

### NFR-6: Usability

| ID | Requirement |
| ---- | ------------ |
| NFR-6.1 | Every decision the system makes must be explainable. If the system skips a file or includes a directory, the reason must be available to the user. |
| NFR-6.2 | Every execution plan must present its reasoning: why each item is included, excluded, or flagged. |
| NFR-6.3 | The system must provide sensible defaults that allow basic operation without requiring manual configuration. |
| NFR-6.4 | The system must never overwrite, delete, or modify data on a target machine without explicit human approval. |

---

### NFR-7: Auditability

| ID | Requirement |
| ---- | ------------ |
| NFR-7.1 | Every operation the system performs must be logged with sufficient detail to reconstruct the sequence of events after the fact: who initiated the operation, what it did, when it occurred, which artifacts were affected, and whether it succeeded or failed. |
| NFR-7.2 | Log entries must include: timestamp, subsystem identifier, operation type, severity, message, and optional context. |
| NFR-7.3 | Logs must never contain sensitive data: passphrases, private keys, API tokens, or file contents. Sensitive file paths must be logged by basename only. |
| NFR-7.4 | Security-critical events (encryption, decryption, approval decisions, credential access, archive verification outcomes) must be logged at an elevated audit severity level. |
| NFR-7.5 | Logs must be written to durable, append-only local storage. |
| NFR-7.6 | Logs must be available in both human-readable and machine-readable formats. |

---

### NFR-8: Offline Operation

| ID | Requirement |
| ---- | ------------ |
| NFR-8.1 | All discovery, planning, backup, encryption, verification, diffing, and audit operations must function fully without Internet connectivity. |
| NFR-8.2 | Restore operations must function without Internet connectivity, except when restore actions require downloading packages from external repositories via the target system's package managers. |
| NFR-8.3 | The AI advisory capability must support a fully local operation mode that requires no network access. |

---

## Data Requirements

---

### DR-1: Environment Manifest

| ID | Requirement |
| ---- | ------------ |
| DR-1.1 | The Environment Manifest must be a versioned, machine-readable, structured data format. |
| DR-1.2 | The manifest must include the following top-level sections: metadata, platform, packages, services, user configuration, credentials, environment variables, scheduled tasks, network, cloud metadata, and user data. |
| DR-1.3 | The manifest metadata must include: schema version, creation timestamp, source hostname, Telos version, and discovery duration. |
| DR-1.4 | Each manifest entry must support annotations added by the classifier and AI advisory (role, importance, category, intent, confidence scores). |
| DR-1.5 | Once produced by discovery, the manifest's discovery data must be immutable. Subsequent processing (classification, AI advisory) may only add or modify annotations — never alter discovered facts. |
| DR-1.6 | The manifest must be serializable to and deserializable from a portable interchange format. |
| DR-1.7 | The manifest must include a schema version field to enable forward and backward compatibility. |

---

### DR-2: Backup Archive

| ID | Requirement |
| ---- | ------------ |
| DR-2.1 | Each backup archive must contain: captured artifacts, the Environment Manifest, the Backup Plan that produced it, and a cryptographic integrity hash manifest. |
| DR-2.2 | Each archive must have a globally unique identifier (UUID). |
| DR-2.3 | Each archive must record its creation timestamp. |
| DR-2.4 | Each archive must support an optional user-defined label. |
| DR-2.5 | Credential materials must be stored in a logically separate segment from general data within the archive, encrypted with distinct cryptographic material. |
| DR-2.6 | The archive must be encrypted in its entirety before being written to storage. |
| DR-2.7 | The archive must optionally support a digital signature for tamper evidence. |

---

### DR-3: Backup and Restore Plans

| ID | Requirement |
| ---- | ------------ |
| DR-3.1 | Backup Plans must include: plan metadata (version, creation timestamp, source manifest reference, estimated archive size), a list of capture actions (with action ID, category, method, source path, reason for inclusion, estimated size, importance, and credential isolation flag), a list of exclusions (with manifest entry reference and reason), and a list of warnings. |
| DR-3.2 | Restore Plans must include: plan metadata (version, creation timestamp, target OS family, source manifest reference), ordered phases containing ordered actions (with action ID, type, target path, dependencies, rollback action, destructive flag, and approval requirement flag), a list of detected conflicts (with description and suggested resolution), and a list of warnings. |

---

### DR-4: Verification Report

| ID | Requirement |
| ---- | ------------ |
| DR-4.1 | The Verification Report must list every difference between the post-restore manifest and the original manifest, classified by severity: Expected, Acceptable, Concerning, or Critical. |
| DR-4.2 | The report must include a summary of total differences by severity and by manifest section. |
| DR-4.3 | The report must include an overall pass/fail verdict based on configurable tolerance thresholds. |

---

### DR-5: Diff Report

| ID | Requirement |
| ---- | ------------ |
| DR-5.1 | The Diff Report must compare two Environment Manifests and list every difference as an addition, removal, or modification, organized by manifest section. |
| DR-5.2 | For modifications, the report must include the before and after values. |
| DR-5.3 | The report must include a summary of total additions, removals, and modifications by section. |

---

### DR-6: Audit Log

| ID | Requirement |
| ---- | ------------ |
| DR-6.1 | Audit logs must use structured entries with the following fields: timestamp, subsystem, operation, severity, message, and optional context. |
| DR-6.2 | Audit logs must support the following severity levels: debug, info, warn, error, and audit. |
| DR-6.3 | The `audit` severity level must be reserved for security-critical events: encryption, decryption, approval decisions, credential access, and archive verification outcomes. |
| DR-6.4 | Audit logs must never contain sensitive data. |
| DR-6.5 | Audit logs must be written to append-only local storage. |

---

## External Interface Requirements

---

### EIR-1: User Interface

| ID | Requirement |
| ---- | ------------ |
| EIR-1.1 | The user interface in Version 1 is a command-line interface. |
| EIR-1.2 | The CLI must accept subcommands for each top-level operation: discover, backup, restore, verify, diff, schedule, and config. |
| EIR-1.3 | The CLI must support three output modes: interactive (formatted with color and progress), plain (for piping), and structured data (for programmatic consumption). |
| EIR-1.4 | The CLI must implement the Approval Gate for all destructive operations, presenting a plan summary, enumerating destructive actions with warnings, and blocking until the user explicitly responds. |

---

### EIR-2: Plugin Interface

| ID | Requirement |
| ---- | ------------ |
| EIR-2.1 | The plugin interface must define a clear contract for each plugin type: Discovery, Classification Rule, Capture, Restore, and Storage. |
| EIR-2.2 | Each plugin type must define its expected input and output formats. |
| EIR-2.3 | Plugins must declare their target plugin interface version for compatibility enforcement. |
| EIR-2.4 | Plugins must declare their capabilities (supported OS families, package managers, cloud providers). |
| EIR-2.5 | Plugins must be stateless: they receive a request and return a response without retaining state between invocations. |

---

### EIR-3: Storage Interface

| ID | Requirement |
| ---- | ------------ |
| EIR-3.1 | The storage interface must support the following operations: write archive, read archive by identifier, list archives with metadata, and delete archive. |
| EIR-3.2 | The default storage backend must be the local filesystem. |
| EIR-3.3 | Additional storage backends (external media, network-attached storage, cloud storage) must be implementable as storage plugins conforming to the storage interface. |

---

### EIR-4: Operating System Interface

| ID | Requirement |
| ---- | ------------ |
| EIR-4.1 | All interaction with the host operating system (file reads, package manager queries, service queries, credential detection) must occur through plugins. The core system must not interact with the OS directly for any platform-specific operation. |
| EIR-4.2 | Version 1 must provide plugins for Ubuntu, Debian, and Windows. |

---

## Version 1 Scope Boundaries

The following table summarizes what is included and excluded from Version 1, as determined by the fixed architecture decisions and the project Vision.

| Capability | Version 1 Status |
| ----------- | ----------------- |
| Full backup (discover → plan → capture → encrypt → store) | ✅ Included |
| Restore to compatible machine (same OS family, same CPU architecture) | ✅ Included |
| Restoration assumes freshly installed OS | ✅ Included |
| Post-restore verification | ✅ Included |
| Environment diffing and audit | ✅ Included |
| CLI interface | ✅ Included |
| Plugin architecture with stable interface | ✅ Included |
| Ubuntu, Debian, Windows plugins | ✅ Included |
| Encrypted, integrity-verified backup archives | ✅ Included |
| Single master passphrase with internal key separation | ✅ Included |
| Credential isolation | ✅ Included |
| Optional AI advisory features | ✅ Included |
| Scheduled backup automation | ✅ Included |
| Offline-first operation | ✅ Included |
| Archive UUID, timestamp, optional label | ✅ Included |
| Interactive data selection (initial) + persistent config (subsequent) | ✅ Included |
| Local encrypted manifest cache | ✅ Included |
| Execution plan review before every operation | ✅ Included |
| Differential / incremental backups | ❌ Future version |
| Cross-platform restoration | ❌ Future version |
| Graphical user interface | ❌ Future version |
| Multi-user / multi-tenant | ❌ Future version |
| Full disk imaging | ❌ Out of scope |
| Real-time synchronization | ❌ Out of scope |
| Bare-metal OS provisioning | ❌ Out of scope |
| Mobile device backup | ❌ Out of scope |

---

## Traceability Matrix

Every functional requirement traces to one or more principles from the [Project Vision](00_Vision.md) and subsystems from the [System Architecture](02_Architecture.md).

| Requirement Group | Vision Principles | Architecture Subsystems |
| ------------------ | ------------------ | ------------------------ |
| FR-1: Discovery | Discovery Before Backup, Comprehension Precedes Action | S3 (Discovery Engine), S4 (Manifest), S12 (Plugin Host) |
| FR-2: Classification | Intent Over Artifacts, Comprehension Precedes Action | S5 (Classifier), S13 (AI Advisory Layer) |
| FR-3: Backup Planning | Minimum Backup Size Maximum Recoverability, Human Approval Before Destructive Operations, Transparency Is Non-Negotiable | S6 (Planner), S1 (CLI Shell — Approval Gate) |
| FR-4: Backup Execution | Security Is the Foundation, Zero Trust | S7 (Capture Engine), S9 (Crypto Engine), S8 (Storage Backend) |
| FR-5: Restore Planning | Human Approval Before Destructive Operations, Deterministic Restore | S6 (Planner), S9 (Crypto Engine), S1 (CLI Shell — Approval Gate) |
| FR-6: Restore Execution | Deterministic Restore, Failures Must Be Recoverable | S10 (Restore Engine), S12 (Plugin Host) |
| FR-7: Verification | Core Objective 6 (Verify) | S11 (Verification Engine), S3 (Discovery Engine), S14 (Diff Engine) |
| FR-8: Diffing and Audit | Transparency Is Non-Negotiable | S14 (Diff Engine) |
| FR-9: Archive Management | — | S8 (Storage Backend) |
| FR-10: Scheduling | — | S15 (Scheduler) |
| FR-11: Configuration | Convention Over Configuration | S17 (Configuration Manager) |
| FR-12: Plugin System | Modular Plugin Architecture | S12 (Plugin Host) |
| FR-13: AI Advisory | AI Philosophy (tool, not decision-maker) | S13 (AI Advisory Layer) |
| FR-14: User Interface | Transparency Is Non-Negotiable, Human Approval Before Destructive Operations | S1 (CLI Shell) |
| NFR-1: Security | Security Is the Foundation, Zero Trust, Credential Isolation | S9 (Crypto Engine), S7 (Capture Engine) |
| NFR-2: Reliability | Failures Must Be Recoverable | S10 (Restore Engine), S12 (Plugin Host) |
| NFR-3: Performance | Success Criteria 1–3 | All subsystems |
| NFR-4: Portability | Modular Plugin Architecture | Core (S2, S4, S6, S9, S14, S16, S17) |
| NFR-5: Extensibility | Modular Plugin Architecture | S12 (Plugin Host) |
| NFR-6: Usability | Transparency Is Non-Negotiable, Convention Over Configuration | S1 (CLI Shell), S6 (Planner), S17 (Configuration Manager) |
| NFR-7: Auditability | Transparency Is Non-Negotiable, Auditability | S16 (Logging & Audit) |
| NFR-8: Offline Operation | Offline-First | All core subsystems |

---

> **This document defines WHAT Telos must accomplish.** It does not prescribe implementation details or technology choices. Every requirement is traceable to the [Project Vision](00_Vision.md) and consistent with the [System Architecture](02_Architecture.md). If a future design decision conflicts with these requirements, the conflict must be resolved explicitly — either by updating the design or by amending this document through formal review.
