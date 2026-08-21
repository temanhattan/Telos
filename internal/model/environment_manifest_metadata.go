package model

import "time"

// EnvironmentManifestMetadata contains information describing an
// environment manifest itself. It captures the identity, versioning,
// and provenance of a manifest at creation time.
//
// All fields are immutable once the manifest is created. This struct
// is intended to be embedded as a field inside EnvironmentManifest.
type EnvironmentManifestMetadata struct {
	// ManifestID uniquely identifies this environment manifest.
	ManifestID ManifestID

	// SchemaVersion is the version of the manifest schema used to
	// structure the data within this manifest.
	SchemaVersion Version

	// TelosVersion is the version of Telos that produced this manifest.
	TelosVersion Version

	// CreatedAt is the instant the manifest was created.
	CreatedAt Timestamp

	// DiscoveryDuration is the wall-clock time elapsed during the
	// environment discovery process that produced this manifest.
	DiscoveryDuration time.Duration

	// SourceMachine identifies the machine on which discovery was
	// performed.
	SourceMachine MachineID
}
