# AERS — Adaptive Environment Recovery System

> **A computer is not just its files.**

AERS is a platform that rebuilds a *working computing environment* — not just the data on it. When you lose or migrate a machine, restoring files gets you your documents back, but not the environment: the packages you'd installed, how they were configured, and the reasoning behind those choices. AERS closes that gap by discovering and understanding an environment first, then treating backup as a byproduct of that understanding — enabling **deterministic environment reconstruction** on a new machine.

**Status:** 🚧 Foundational design and specification phase. Active development has not yet started. This README reflects intended design, not shipped functionality.

---

## Table of Contents
- [Why AERS](#why-aers)
- [How It Works, In Short](#how-it-works-in-short)
- [Core Philosophy](#core-philosophy)
- [Architecture at a Glance](#architecture-at-a-glance)
- [Use Cases](#use-cases)
- [Project Status & Roadmap](#project-status--roadmap)
- [Documentation](#documentation)
- [Getting Involved](#getting-involved)

---

## Why AERS

Imagine your laptop dies, or you're handed a fresh cloud instance and need it to feel like your old one. A traditional backup tool hands you back a pile of files — but you're still the one who has to remember: *Which Python version was this project pinned to? Did I configure that service with a custom flag? Why did I install this obscure package in the first place?*

AERS is built to answer those questions automatically. Instead of blindly copying bytes, it:

1. **Discovers** what's actually on your machine — OS, packages, services, configurations.
2. **Understands** *why* those things are there (a package from a public registry doesn't need to be backed up as a binary blob — just its name and version).
3. **Reconstructs** that environment deterministically on a new machine, the same way, every time.

This makes AERS less like a backup tool and more like an **environment compiler**: input a captured snapshot, output a working system.

## How It Works, In Short

A typical AERS flow looks like this:

```
1. Discover   →  Scan the machine: OS, installed packages, running services, configs
2. Capture    →  Record *intent* (e.g. "nginx 1.24 from apt") not raw binaries where avoidable
3. Secure     →  Encrypt and integrity-verify every artifact before it's stored
4. Store      →  Persist the snapshot to your chosen backend
5. Reconstruct→  On a target machine, replay the snapshot to rebuild the environment
                 (with human approval at every destructive step)
```

**Example scenario:** You're running a lab VM with a custom-configured PostgreSQL instance, three Python virtual environments, and a handful of dotfiles tuning your shell. Your VM host dies. With AERS, a new VM boots up, AERS reads your last snapshot, reinstalls PostgreSQL at the same version with the same config, recreates each virtualenv from its recorded package list, and restores your dotfiles — all without you having to recall the exact setup from memory.

## Core Philosophy

| Principle | What it means |
|---|---|
| **Discovery Before Backup** | AERS fully catalogs the environment (OS, packages, services, configs) *before* capturing any data. |
| **Intent Over Artifacts** | AERS stores *why* something exists, not just *that* it exists. A package available in a public repo is stored as a name + version, not a binary. |
| **Zero-Trust Security** | Every backup artifact is encrypted and integrity-verified before storage. Sensitive credentials are mathematically isolated. |
| **Human in the Loop** | AERS never overwrites, deletes, or modifies data on a target machine without explicit human approval. |

## Architecture at a Glance

AERS uses a pipeline-oriented, plugin-extended architecture with four layers:

- **The Core** — Small, stable, and entirely OS-agnostic. Owns orchestration, cryptography, storage, and verification.
- **The Plugins** — All platform-specific knowledge (OS discovery, package managers, cloud adapters) lives here, not in the core.
- **The Pipelines** — Backup and restore are linear, staged pipelines: data flows predictably from one stage to the next.
- **AI Advisory Layer** *(optional)* — Suggests machine roles, scores artifact importance, and predicts restore conflicts. Always a non-binding advisor, operating under strict deterministic overrides — it can suggest, never decide.

→ Full breakdown in [Architecture](docs/02_Architecture.md).

## Use Cases

- **New Machine Setup** — Reconstruct your entire development environment on a fresh OS install.
- **VM Snapshot & Clone** — Capture the exact state of complex lab environments for easy cloning.
- **Cloud Migration** — Move server environments between providers (e.g., AWS → GCP) by separating portable configs from provider-specific infrastructure.
- **Disaster Recovery** — Get back to work quickly after hardware failure with prioritized, intelligent restoration.
- **Environment Audit** — Compare your current machine state against past snapshots to detect drift or anomalies.

## Project Status & Roadmap

AERS is currently in the **foundational design and specification phase** — the architecture and philosophy above are settled, but no implementation exists yet. There's no installable release, CLI, or usage instructions to give yet; this section will be replaced with real installation and quick-start steps once a working build exists.

See [Roadmap](docs/05_Roadmap.md) for the planned phases of development.

## Documentation

| Doc | What's in it |
|---|---|
| [Vision](docs/00_Vision.md) | The mission, philosophy, and boundaries of the project |
| [Architecture](docs/02_Architecture.md) | System topology, subsystem definitions, and data pipelines |
| [Plugin API](docs/03_Plugin_API.md) | Guidelines for extending AERS |
| [Coding Standards](docs/04_Coding_Standards.md) | Rules for contributing to the codebase |
| [Roadmap](docs/05_Roadmap.md) | Future expansion phases |

## Getting Involved

Since AERS hasn't reached implementation yet, the highest-value way to engage right now is with the design itself:

- Read the [Vision](docs/00_Vision.md) and [Architecture](docs/02_Architecture.md) docs to understand the intended shape of the system.
- Open an issue/discussion with feedback on the pipeline design or plugin API before code is written — design-phase feedback is cheap to act on.
- Once implementation begins, [Coding Standards](docs/04_Coding_Standards.md) will govern contributions — check back there for how to submit changes.

*(Contribution workflow, issue templates, and a CONTRIBUTING.md will be added once active development starts.)*
