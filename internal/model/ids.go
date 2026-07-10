package model

// Domain-specific identifier types provide compile-time safety by
// preventing accidental misuse of one entity's ID where another is
// expected. Each type is a distinct named type over UUID, so a
// function accepting ArchiveID cannot be called with a PluginID.

// MachineID uniquely identifies a machine profile.
type MachineID UUID

// ArchiveID uniquely identifies a recovery archive.
type ArchiveID UUID

// PluginID uniquely identifies a registered plugin.
type PluginID UUID

// ManifestID uniquely identifies an environment manifest.
type ManifestID UUID

// PlanID uniquely identifies a restoration plan.
type PlanID UUID

// ArtifactID uniquely identifies an artifact within an archive.
type ArtifactID UUID
