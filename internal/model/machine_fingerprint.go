package model

// MachineFingerprint captures the stable hardware identity of a physical
// or virtual machine. It aggregates immutable hardware attributes that
// together produce a unique identity persisting across reboots, OS
// reinstalls, and backup sessions.
//
// MachineFingerprint is a value object owned by the discovery subsystem.
// It is referenced by MachineProfile to anchor a profile to a specific
// piece of hardware. It carries no business logic; interpretation and
// matching are the responsibility of higher-level services.
type MachineFingerprint struct {
	ID               MachineID
	BIOSUUIDRaw      string
	MotherboardSerial string
	SystemSerial     string
	PrimaryMAC       string
	CPUIdentifier    string
	CreatedAt        Timestamp
}
