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

// BackupManifestID uniquely identifies an archive integrity manifest.
type BackupManifestID UUID

// RestoreResultID uniquely identifies the outcome of a restore operation.
type RestoreResultID UUID

// VerificationReportID uniquely identifies a verification report.
type VerificationReportID UUID

// ApprovalID uniquely identifies an approval-gate decision.
type ApprovalID UUID

// StorageLocationID uniquely identifies an archive storage location.
type StorageLocationID UUID

// ConfigurationProfileID uniquely identifies a persisted configuration profile.
type ConfigurationProfileID UUID
