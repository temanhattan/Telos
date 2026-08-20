package model

// OSFamily classifies operating system families at the architecture level.
type OSFamily string

// Standard OSFamily constants for system classification.
const (
	OSFamilyLinux   OSFamily = "linux"
	OSFamilyWindows OSFamily = "windows"
	OSFamilyDarwin  OSFamily = "darwin"
	OSFamilyFreeBSD OSFamily = "freebsd"
)

// CPUArchitecture identifies processor instruction set architectures.
type CPUArchitecture string

// Standard CPUArchitecture constants for identifying hardware instruction sets.
const (
	CPUArchAMD64   CPUArchitecture = "amd64"
	CPUArchARM64   CPUArchitecture = "arm64"
	CPUArchARMv7   CPUArchitecture = "armv7"
	CPUArch386     CPUArchitecture = "386"
	CPUArchRISCV64 CPUArchitecture = "riscv64"
)

// HashAlgorithm identifies cryptographic hash functions used for
// integrity verification throughout the system. Only cryptographically
// secure algorithms belong here; non-cryptographic hashes for caching
// or deduplication should use a separate type.
type HashAlgorithm string

// Supported HashAlgorithm constants for cryptographic integrity checks.
const (
	HashSHA256  HashAlgorithm = "sha256"
	HashSHA512  HashAlgorithm = "sha512"
	HashBLAKE2b HashAlgorithm = "blake2b"
	HashBLAKE3  HashAlgorithm = "blake3"
)

// EncryptionAlgorithm identifies symmetric encryption algorithms
// used for securing archives and sensitive data at rest.
type EncryptionAlgorithm string

// Supported EncryptionAlgorithm constants for securing data at rest.
const (
	EncryptionAES256GCM     EncryptionAlgorithm = "aes-256-gcm"
	EncryptionXChaCha20Poly EncryptionAlgorithm = "xchacha20-poly1305"
)
