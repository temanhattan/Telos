# Title

Capture Staging Lifecycle and Trusted Staging Path

# Status

Accepted

# Context

The V1 documentation (`05_Plugin_API.md`) states that Capture plugins write their produced artifacts to a "staging location designated by the Plugin Host in the request context." It also specifies that the Core Engine (S7) reads from staging, encrypts the artifact, and cleans up staging after successful storage. 

However, the `CaptureRequest` schema did not include a staging location field, and the Plugin Host (S12) did not allocate one. 

A critical security invariant must be preserved: **A plugin-controlled or request-controlled path MUST NOT become a Landlock WritePath without trusted host/core allocation and canonicalization.** Allowing a plugin to specify its own absolute staging path and mapping that to a Landlock WritePath would result in an arbitrary path traversal vulnerability (Sandbox Escape).

# Decision

1. **Schema Update**: `CaptureRequest` is extended to include `staging_location: string`.
2. **Allocation**: The Core Engine (S7) MUST generate a cryptographically random, per-invocation temporary directory under a trusted, system-configured Telos staging root (e.g., `/var/lib/telos/staging/capture-<uuid>`).
3. **Sandbox Mapping**: The Plugin Host (S12) receives `staging_location` in the request. The Host validates that the path is absolute and within the configured Telos root, canonicalizes it, and appends it to the sandbox `Policy.WritePaths` and `Policy.ReadPaths`.
4. **Plugin Behavior**: The plugin writes its artifact into the provided `staging_location`. The plugin returns the *relative filename* (or subpath) of the artifact in `CaptureResponse.artifacts[].artifact_location`.
5. **Cleanup**: The Core Engine (S7) reads the artifact, passes it to the Crypto Engine, and unconditionally destroys the per-invocation staging directory when the operation completes or times out.

# Consequences

**Benefits:**
- Preserves the security invariant: plugins cannot dictate sandbox WritePaths.
- Ensures staging data is isolated per invocation, preventing concurrent plugin collisions.
- Places lifecycle and cleanup responsibility on the Core Engine, matching the architectural vision.

**Trade-offs:**
- Adds slight complexity to the Core Engine which must manage directory creation and cleanup.
