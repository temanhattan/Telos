package model

// MachineProfile represents the identity and hardware characteristics of a
// specific computing environment at a specific point in time. It is created
// during discovery, owned by the discovery engine, and referenced by every
// environment manifest to correlate backups from the same machine.
//
// The embedded MachineFingerprint anchors this profile to a stable hardware
// identity. MachineProfile itself carries no business logic; compatibility
// assessment and profile matching are the responsibility of higher-level
// services.
type MachineProfile struct {
	ID                  MachineID
	Hostname            string
	OSFamily            OSFamily
	Architecture        CPUArchitecture
	CPUModel            string
	CPUCores            uint16
	MemoryBytes         uint64
	PrimaryStorageBytes uint64
	Fingerprint         MachineFingerprint
	FirstSeen           Timestamp
	LastSeen            Timestamp
}
