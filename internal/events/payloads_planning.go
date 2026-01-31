package events

import "time"

// ===== PLAN GENERATION =====

// PlanGenerationStartedPayload is the payload for PlanGenerationStarted.
type PlanGenerationStartedPayload struct {
	NormalizedSpecEventID string    `json:"normalized_spec_event_id"`
	ImpactAnalysisEventID string    `json:"impact_analysis_event_id"`
	StartedAt             time.Time `json:"started_at"`
}

// PlanGeneratedPayload is the payload for PlanGenerated.
type PlanGeneratedPayload struct {
	Plan        ImplementationPlan `json:"plan"`
	LLMInfo     LLMProcessingInfo  `json:"llm_info"`
	CompletedAt time.Time          `json:"completed_at"`
}

// ImplementationPlan is the generated implementation plan.
type ImplementationPlan struct {
	// PlanID uniquely identifies this plan version.
	PlanID string `json:"plan_id"`

	// Title is a human-readable title for the implementation.
	Title string `json:"title"`

	// Summary is an executive summary of the changes.
	Summary string `json:"summary"`

	// Steps are the ordered implementation steps.
	Steps []PlanStep `json:"steps"`

	// Dependencies are cross-step dependencies.
	Dependencies []StepDependency `json:"dependencies"`

	// EstimatedMetrics are estimated execution metrics.
	EstimatedMetrics PlanMetrics `json:"estimated_metrics"`

	// Assumptions are assumptions made during planning.
	Assumptions []string `json:"assumptions,omitempty"`

	// AlternativesConsidered are alternative approaches considered.
	AlternativesConsidered []Alternative `json:"alternatives_considered,omitempty"`
}

// PlanStep is a single step in the implementation plan.
type PlanStep struct {
	// StepNumber is the 1-based step number.
	StepNumber int `json:"step_number"`

	// StepID is the unique step identifier.
	StepID StepID `json:"step_id"`

	// Action describes what this step does.
	Action string `json:"action"`

	// Rationale explains why this step is needed.
	Rationale string `json:"rationale"`

	// TargetFiles are the files this step will modify/create.
	TargetFiles []TargetFile `json:"target_files"`

	// Prerequisites are step numbers that must complete first.
	Prerequisites []int `json:"prerequisites,omitempty"`

	// ExpectedOutcome describes the expected result.
	ExpectedOutcome string `json:"expected_outcome"`

	// Verification describes how to verify success.
	Verification StepVerification `json:"verification"`

	// EstimatedTokens is the estimated LLM tokens for this step.
	EstimatedTokens int `json:"estimated_tokens"`
}

// TargetFile describes a file to be modified.
type TargetFile struct {
	Path       string         `json:"path"`
	Action     FileAction     `json:"action"`
	ChangeType FileChangeType `json:"change_type"`
	Reason     string         `json:"reason"`
}

// FileAction describes what action to take on a file.
type FileAction string

const (
	FileActionCreate FileAction = "create"
	FileActionModify FileAction = "modify"
	FileActionDelete FileAction = "delete"
	FileActionRename FileAction = "rename"
)

// FileChangeType categorizes file changes.
type FileChangeType string

const (
	FileChangeTypeImplementation FileChangeType = "implementation"
	FileChangeTypeTest           FileChangeType = "test"
	FileChangeTypeConfig         FileChangeType = "config"
	FileChangeTypeDocumentation  FileChangeType = "documentation"
	FileChangeTypeDependency     FileChangeType = "dependency"
)

// StepVerification describes how to verify a step.
type StepVerification struct {
	Method      VerificationMethod `json:"method"`
	Command     string             `json:"command,omitempty"`
	Expected    string             `json:"expected,omitempty"`
	Description string             `json:"description"`
}

// VerificationMethod is how to verify step success.
type VerificationMethod string

const (
	VerificationMethodSyntaxCheck VerificationMethod = "syntax_check"
	VerificationMethodBuild       VerificationMethod = "build"
	VerificationMethodTest        VerificationMethod = "test"
	VerificationMethodLint        VerificationMethod = "lint"
	VerificationMethodManual      VerificationMethod = "manual"
)

// StepDependency describes a dependency between steps.
type StepDependency struct {
	FromStep       int            `json:"from_step"`
	ToStep         int            `json:"to_step"`
	DependencyType DependencyType `json:"dependency_type"`
	Reason         string         `json:"reason"`
}

// DependencyType categorizes step dependencies.
type DependencyType string

const (
	DependencyTypeStrict DependencyType = "strict" // Must complete before starting
	DependencyTypeSoft   DependencyType = "soft"   // Should complete, but can proceed
	DependencyTypeData   DependencyType = "data"   // Needs data from prior step
)

// PlanMetrics contains plan execution estimates.
type PlanMetrics struct {
	TotalSteps          int `json:"total_steps"`
	EstimatedTokens     int `json:"estimated_tokens"`
	EstimatedFilesTotal int `json:"estimated_files_total"`
	EstimatedNewFiles   int `json:"estimated_new_files"`
	EstimatedModified   int `json:"estimated_modified"`
	EstimatedDeleted    int `json:"estimated_deleted"`
}

// Alternative describes an alternative approach considered.
type Alternative struct {
	Approach    string `json:"approach"`
	Pros        string `json:"pros"`
	Cons        string `json:"cons"`
	WhyRejected string `json:"why_rejected"`
}

// PlanGenerationFailedPayload is the payload for PlanGenerationFailed.
type PlanGenerationFailedPayload struct {
	ErrorCode    string            `json:"error_code"`
	ErrorMessage string            `json:"error_message"`
	Retryable    bool              `json:"retryable"`
	ErrorContext map[string]string `json:"error_context,omitempty"`
	FailedAt     time.Time         `json:"failed_at"`
}

// ===== PLAN VALIDATION =====

// PlanValidatedPayload is the payload for PlanValidated.
type PlanValidatedPayload struct {
	PlanID           string             `json:"plan_id"`
	ValidationResult ValidationResult   `json:"validation_result"`
	Warnings         []ValidationIssue  `json:"warnings,omitempty"`
	ValidatedAt      time.Time          `json:"validated_at"`
}

// ValidationResult indicates validation outcome.
type ValidationResult string

const (
	ValidationResultPassed         ValidationResult = "passed"
	ValidationResultPassedWarnings ValidationResult = "passed_with_warnings"
	ValidationResultFailed         ValidationResult = "failed"
)

// ValidationIssue describes a validation warning or error.
type ValidationIssue struct {
	Code        string           `json:"code"`
	Message     string           `json:"message"`
	Severity    IssueSeverity    `json:"severity"`
	StepNumber  int              `json:"step_number,omitempty"`
	FilePath    string           `json:"file_path,omitempty"`
	Suggestion  string           `json:"suggestion,omitempty"`
}

// IssueSeverity indicates issue severity.
type IssueSeverity string

const (
	IssueSeverityInfo    IssueSeverity = "info"
	IssueSeverityWarning IssueSeverity = "warning"
	IssueSeverityError   IssueSeverity = "error"
)
