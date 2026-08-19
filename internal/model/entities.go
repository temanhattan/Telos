package model

// This file implements the behavior-free domain vocabulary specified in
// docs/04_Data_Model.md. Validation, serialization, and workflow behavior
// belong to the subsystems that consume these structures.

type ImportanceLevel string

const (
	ImportanceCritical    ImportanceLevel = "critical"
	ImportanceRecommended ImportanceLevel = "recommended"
	ImportanceOptional    ImportanceLevel = "optional"
	ImportanceTransient   ImportanceLevel = "transient"
)

type Reproducibility string

const (
	Reproducible  Reproducibility = "reproducible"
	Irreplaceable Reproducibility = "irreplaceable"
)

type CaptureMethod string

const (
	CaptureReference CaptureMethod = "reference"
	CaptureCopy      CaptureMethod = "copy"
	CaptureExport    CaptureMethod = "export"
)

type OperationStatus string

const (
	StatusPending   OperationStatus = "pending"
	StatusCompleted OperationStatus = "completed"
	StatusPartial   OperationStatus = "partial"
	StatusFailed    OperationStatus = "failed"
	StatusAborted   OperationStatus = "aborted"
)

type ApprovalDecision string

const (
	ApprovalApproved ApprovalDecision = "approved"
	ApprovalRejected ApprovalDecision = "rejected"
	ApprovalModified ApprovalDecision = "modified"
)

type ClassificationSource string

const (
	ClassificationDeterministic ClassificationSource = "deterministic"
	ClassificationPlugin        ClassificationSource = "plugin"
	ClassificationAI            ClassificationSource = "ai"
)

type DiffSeverity string

const (
	DiffExpected   DiffSeverity = "expected"
	DiffAcceptable DiffSeverity = "acceptable"
	DiffConcerning DiffSeverity = "concerning"
	DiffCritical   DiffSeverity = "critical"
)

// Annotation is a judgment attached to discovery data without changing facts.
type Annotation struct {
	Importance      ImportanceLevel
	Intent          string
	Category        string
	Reproducibility Reproducibility
	Source          ClassificationSource
	Confidence      *float64
	Reasoning       string
}

type Platform struct {
	OSFamily     OSFamily
	OSID         string
	OSVersion    Version
	Architecture CPUArchitecture
	Kernel       string
	Machine      MachineProfile
}

type Package struct {
	Manager    string
	Name       string
	Version    Version
	Source     string
	Explicit   bool
	Annotation Annotation
}

type Service struct {
	Name        string
	Supervisor  string
	Status      string
	ConfigPaths []string
	Annotation  Annotation
}

type UserConfiguration struct {
	Path       string
	Type       string
	IsDefault  bool
	Annotation Annotation
}

type CredentialReference struct {
	Type        string
	Path        string
	Fingerprint string
	DetectedAt  Timestamp
	Annotation  Annotation
}

type EnvironmentVariable struct {
	Scope      string
	Name       string
	Value      string
	Annotation Annotation
}

type ScheduledTask struct {
	Scheduler  string
	Schedule   string
	Command    string
	Annotation Annotation
}

type NetworkConfiguration struct {
	Interfaces    []NetworkInterface
	DNSConfig     string
	VPNProfiles   []string
	FirewallRules []string
	Annotation    Annotation
}

type NetworkInterface struct {
	Name      string
	Addresses []string
	Gateway   string
}

type CloudMetadata struct {
	Provider      string
	InstanceID    string
	Region        string
	AttachedRoles []string
	Annotation    Annotation
}

type UserData struct {
	Path            string
	SizeBytes       uint64
	LastModified    Timestamp
	IsIrreplaceable bool
	Annotation      Annotation
}

// EnvironmentManifest is the immutable record produced by discovery once sealed.
type EnvironmentManifest struct {
	Metadata       EnvironmentManifestMetadata
	Machine        MachineProfile
	Platform       Platform
	Packages       []Package
	Services       []Service
	UserConfig     []UserConfiguration
	Credentials    []CredentialReference
	Environment    []EnvironmentVariable
	ScheduledTasks []ScheduledTask
	Network        NetworkConfiguration
	CloudMetadata  CloudMetadata
	UserData       []UserData
	Completeness   map[string]OperationStatus
	SealedAt       *Timestamp
}

type CaptureAction struct {
	ID                  string
	Category            string
	Method              CaptureMethod
	SourcePath          string
	Reason              string
	EstimatedSizeBytes  uint64
	Importance          ImportanceLevel
	CredentialIsolation bool
}

type PlanExclusion struct {
	ManifestEntryRef string
	Reason           string
}
type PlanWarning struct {
	ID       string
	Severity string
	Message  string
}

type BackupPlan struct {
	ID                 PlanID
	Version            Version
	CreatedAt          Timestamp
	SourceManifestID   ManifestID
	EstimatedSizeBytes uint64
	Actions            []CaptureAction
	Exclusions         []PlanExclusion
	Warnings           []PlanWarning
	Approval           *UserApproval
}

type RestoreAction struct {
	ID               string
	Type             string
	TargetPath       string
	Dependencies     []string
	Rollback         string
	Destructive      bool
	RequiresApproval bool
}

type RestorePhase struct {
	Name    string
	Order   uint8
	Actions []RestoreAction
}
type RestoreConflict struct {
	ID                  string
	Description         string
	SuggestedResolution string
	Severity            string
}

type RestorePlan struct {
	ID                 PlanID
	Version            Version
	CreatedAt          Timestamp
	TargetOSFamily     OSFamily
	TargetArchitecture CPUArchitecture
	SourceArchiveID    ArchiveID
	SourceManifestID   ManifestID
	Phases             []RestorePhase
	Conflicts          []RestoreConflict
	Warnings           []PlanWarning
	Approval           *UserApproval
}

type CapturedArtifact struct {
	ID                  ArtifactID
	PlanActionID        string
	RelativePath        string
	SourcePath          string
	Method              CaptureMethod
	Category            string
	ContentHash         string
	HashAlgorithm       HashAlgorithm
	SizeBytes           uint64
	CredentialIsolation bool
	CapturedAt          Timestamp
	Status              OperationStatus
}

type BackupManifestEntry struct {
	ArtifactID   ArtifactID
	RelativePath string
	Hash         string
	SizeBytes    uint64
}
type BackupManifest struct {
	ID            BackupManifestID
	ArchiveID     ArchiveID
	HashAlgorithm HashAlgorithm
	ArtifactCount uint64
	Entries       []BackupManifestEntry
	SealedAt      Timestamp
}

type Archive struct {
	ID               ArchiveID
	CreatedAt        Timestamp
	Label            string
	SourceHostname   string
	SourceMachineID  MachineID
	ManifestID       ManifestID
	ManifestVersion  Version
	AERSVersion      Version
	SizeBytes        uint64
	Encryption       EncryptionAlgorithm
	Signature        string
	StorageLocations []StorageLocation
}

type StorageLocation struct {
	ID         StorageLocationID
	Backend    string
	Identifier string
	CreatedAt  Timestamp
}

type DiscoveryResult struct {
	Category       string
	SourcePluginID PluginID
	Status         OperationStatus
	EntryCount     uint64
	FailureReason  string
	Duration       Timestamp
}

type ClassificationResult struct {
	ManifestID     ManifestID
	EntryReference string
	Annotation     Annotation
	CreatedAt      Timestamp
}

type RestoreActionOutcome struct {
	ActionID     string
	Status       OperationStatus
	Error        string
	UserResponse string
}
type RestoreCheckpoint struct {
	LastSuccessfulActionID string
	Resumable              bool
	UpdatedAt              Timestamp
}
type RestoreResult struct {
	ID          RestoreResultID
	PlanID      PlanID
	ArchiveID   ArchiveID
	StartedAt   Timestamp
	CompletedAt *Timestamp
	Status      OperationStatus
	Actions     []RestoreActionOutcome
	Checkpoint  RestoreCheckpoint
}

type DiffEntry struct {
	Section  string
	Change   string
	Before   any
	After    any
	Severity DiffSeverity
}
type VerificationReport struct {
	ID                 VerificationReportID
	OriginalManifestID ManifestID
	RestoredManifestID ManifestID
	RestoreResultID    RestoreResultID
	Differences        []DiffEntry
	Passed             bool
	CreatedAt          Timestamp
}

type UserApproval struct {
	ID            ApprovalID
	PlanID        PlanID
	PlanType      string
	Decision      ApprovalDecision
	DecidedAt     Timestamp
	Modifications []string
}

type Plugin struct {
	ID       PluginID
	Manifest PluginManifest
}
type PluginManifest struct {
	Name             string
	Version          Version
	Author           string
	Description      string
	InterfaceVersion Version
	Type             string
	OSFamilies       []OSFamily
	Capabilities     []string
	FilesystemRead   []string
	FilesystemWrite  []string
	NetworkAllowed   bool
	Subprocesses     []string
	TrustLevel       string
	Signature        string
}

type ConfigurationProfile struct {
	ID            ConfigurationProfileID
	SchemaVersion Version
	LoadedAt      Timestamp
}
