package model

// This file implements the behavior-free domain vocabulary specified in
// docs/04_Data_Model.md. Validation, serialization, and workflow behavior
// belong to the subsystems that consume these structures.

// ImportanceLevel represents how critical a detected change or anomaly is.
type ImportanceLevel string

// ImportanceLevel constants define the severity spectrum for annotations.
const (
	ImportanceCritical    ImportanceLevel = "critical"
	ImportanceRecommended ImportanceLevel = "recommended"
	ImportanceOptional    ImportanceLevel = "optional"
	ImportanceTransient   ImportanceLevel = "transient"
)

// Reproducibility indicates whether an artifact can be regenerated or must be backed up.
type Reproducibility string

// Reproducibility constants specify if data can be safely discarded or must be preserved.
const (
	Reproducible  Reproducibility = "reproducible"
	Irreplaceable Reproducibility = "irreplaceable"
)

// CaptureMethod defines the mechanism used to stage an artifact for backup.
type CaptureMethod string

// CaptureMethod constants represent the various ways data is extracted and staged.
const (
	CaptureReference CaptureMethod = "reference"
	CaptureCopy      CaptureMethod = "copy"
	CaptureExport    CaptureMethod = "export"
)

// OperationStatus tracks the lifecycle state of long-running core workflows.
type OperationStatus string

// OperationStatus constants define the possible states for discovery and restore tasks.
const (
	StatusPending   OperationStatus = "pending"
	StatusCompleted OperationStatus = "completed"
	StatusPartial   OperationStatus = "partial"
	StatusFailed    OperationStatus = "failed"
	StatusAborted   OperationStatus = "aborted"
)

// ApprovalDecision represents a user's choice at a plan approval gate.
type ApprovalDecision string

// ApprovalDecision constants define the outcome of a user review for a generated plan.
const (
	ApprovalApproved ApprovalDecision = "approved"
	ApprovalRejected ApprovalDecision = "rejected"
	ApprovalModified ApprovalDecision = "modified"
)

// ClassificationSource indicates what subsystem or process generated an annotation.
type ClassificationSource string

// ClassificationSource constants define the origins of metadata classifications.
const (
	ClassificationDeterministic ClassificationSource = "deterministic"
	ClassificationPlugin        ClassificationSource = "plugin"
	ClassificationAI            ClassificationSource = "ai"
)

// DiffSeverity categorizes the risk associated with a state discrepancy.
type DiffSeverity string

// DiffSeverity constants define how concerning a post-restore mismatch is.
const (
	DiffExpected   DiffSeverity = "expected"
	DiffAcceptable DiffSeverity = "acceptable"
	DiffConcerning DiffSeverity = "concerning"
	DiffCritical   DiffSeverity = "critical"
)

// Annotation is a judgment attached to discovery data without changing facts.
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

// Platform describes the underlying operating system and hardware architecture.
type Platform struct {
	OSFamily     OSFamily
	OSID         string
	OSVersion    Version
	Architecture CPUArchitecture
	Kernel       string
	Machine      MachineProfile
}

// Package represents an installed software package and its version.
type Package struct {
	Manager    string
	Name       string
	Version    Version
	Source     string
	Explicit   bool
	Annotation Annotation
}

// Service represents a background daemon or system service and its state.
type Service struct {
	Name        string
	Supervisor  string
	Status      string
	ConfigPaths []string
	Annotation  Annotation
}

// UserConfiguration tracks user-specific application configuration files.
type UserConfiguration struct {
	Path       string
	Type       string
	IsDefault  bool
	Annotation Annotation
}

// CredentialReference points to a credential on disk that requires special isolation.
type CredentialReference struct {
	Type        string
	Path        string
	Fingerprint string
	DetectedAt  Timestamp
	Annotation  Annotation
}

// EnvironmentVariable represents a system or user environment variable.
type EnvironmentVariable struct {
	Scope      string
	Name       string
	Value      string
	Annotation Annotation
}

// ScheduledTask describes a cron job, systemd timer, or scheduled execution.
type ScheduledTask struct {
	Scheduler  string
	Schedule   string
	Command    string
	Annotation Annotation
}

// NetworkConfiguration represents the system's networking state, including interfaces and rules.
type NetworkConfiguration struct {
	Interfaces    []NetworkInterface
	DNSConfig     string
	VPNProfiles   []string
	FirewallRules []string
	Annotation    Annotation
}

// NetworkInterface details a single logical or physical network connection.
type NetworkInterface struct {
	Name      string
	Addresses []string
	Gateway   string
}

// CloudMetadata tracks the instance details when running in a cloud environment.
type CloudMetadata struct {
	Provider      string
	InstanceID    string
	Region        string
	AttachedRoles []string
	Annotation    Annotation
}

// UserData tracks files or directories identified as belonging to user documents.
type UserData struct {
	Path            string
	SizeBytes       uint64
	LastModified    Timestamp
	IsIrreplaceable bool
	Annotation      Annotation
}

// EnvironmentManifest is the immutable record produced by discovery once sealed.
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

// CaptureAction defines a specific extraction task in a BackupPlan.
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

// PlanExclusion documents a discovered artifact that was deliberately left out of a plan.
type PlanExclusion struct {
	ManifestEntryRef string
	Reason           string
}

// PlanWarning conveys non-fatal issues or risks identified during plan generation.
type PlanWarning struct {
	ID       string
	Severity string
	Message  string
}

// BackupPlan is an orchestrator-generated, actionable plan for archiving a manifest.
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

// RestoreAction defines a single, verifiable task in a restore phase.
type RestoreAction struct {
	ID               string
	Type             string
	TargetPath       string
	Dependencies     []string
	Rollback         string
	Destructive      bool
	RequiresApproval bool
}

// RestorePhase groups RestoreActions into an ordered stage of execution.
type RestorePhase struct {
	Name    string
	Order   uint8
	Actions []RestoreAction
}

// RestoreConflict details an issue preventing clean restoration, such as pre-existing files.
type RestoreConflict struct {
	ID                  string
	Description         string
	SuggestedResolution string
	Severity            string
}

// RestorePlan maps an archive's manifest to the current machine state to guide restoration.
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

// CapturedArtifact tracks the status of a single item processed during an archive creation.
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

// BackupManifestEntry records the cryptographic hash and size of a staged artifact.
type BackupManifestEntry struct {
	ArtifactID   ArtifactID
	RelativePath string
	Hash         string
	SizeBytes    uint64
}

// BackupManifest serves as the canonical inventory of an archive's contents.
type BackupManifest struct {
	ID            BackupManifestID
	ArchiveID     ArchiveID
	HashAlgorithm HashAlgorithm
	ArtifactCount uint64
	Entries       []BackupManifestEntry
	SealedAt      Timestamp
}

// Archive describes a completed, encrypted backup payload and its storage location.
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

// StorageLocation identifies a remote or local destination where an archive is persisted.
type StorageLocation struct {
	ID         StorageLocationID
	Backend    string
	Identifier string
	CreatedAt  Timestamp
}

// DiscoveryResult summarizes the outcome of a single plugin's discovery phase.
type DiscoveryResult struct {
	Category       string
	SourcePluginID PluginID
	Status         OperationStatus
	EntryCount     uint64
	FailureReason  string
	Duration       Timestamp
}

// ClassificationResult encapsulates an annotation applied to a manifest entry.
type ClassificationResult struct {
	ManifestID     ManifestID
	EntryReference string
	Annotation     Annotation
	CreatedAt      Timestamp
}

// RestoreActionOutcome tracks the success or failure of a specific restore action.
type RestoreActionOutcome struct {
	ActionID     string
	Status       OperationStatus
	Error        string
	UserResponse string
}

// RestoreCheckpoint enables resuming a partially completed restore plan.
type RestoreCheckpoint struct {
	LastSuccessfulActionID string
	Resumable              bool
	UpdatedAt              Timestamp
}

// RestoreResult records the complete execution history of a RestorePlan.
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

// DiffEntry describes a specific difference found during verification.
type DiffEntry struct {
	Section  string
	Change   string
	Before   any
	After    any
	Severity DiffSeverity
}

// VerificationReport summarizes discrepancies between the expected and actual state.
type VerificationReport struct {
	ID                 VerificationReportID
	OriginalManifestID ManifestID
	RestoredManifestID ManifestID
	RestoreResultID    RestoreResultID
	Differences        []DiffEntry
	Passed             bool
	CreatedAt          Timestamp
}

// UserApproval records the authorization granted to a plan before execution.
type UserApproval struct {
	ID            ApprovalID
	PlanID        PlanID
	PlanType      string
	Decision      ApprovalDecision
	DecidedAt     Timestamp
	Modifications []string
}

// Plugin represents a loaded, validated plugin within the system.
type Plugin struct {
	ID       PluginID
	Manifest PluginManifest
}

// PluginManifest contains the static metadata parsed from a plugin's definition.
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

// ConfigurationProfile represents a loaded instance of the system configuration.
type ConfigurationProfile struct {
	ID            ConfigurationProfileID
	SchemaVersion Version
	LoadedAt      Timestamp
}
