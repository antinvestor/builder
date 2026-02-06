package events

import "time"

// ===== FEATURE EXECUTION INITIALIZATION =====

// FeatureExecutionInitializedPayload is the payload for FeatureExecutionInitialized.
type FeatureExecutionInitializedPayload struct {
	ExecutionID ExecutionID          `json:"execution_id"`
	Spec        FeatureSpecification `json:"spec"`
	Repository  RepositoryContext    `json:"repository"`
	Constraints ExecutionConstraints `json:"constraints"`
	Request     RequestMetadata      `json:"request"`
}

// FeatureSpecification defines the feature to be built.
type FeatureSpecification struct {
	// Title is a human-readable title for the feature.
	Title string `json:"title"`

	// Description is the detailed feature description (for LLM context).
	Description string `json:"description"`

	// AcceptanceCriteria are optional specific criteria to satisfy.
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`

	// PathHints are optional file/path hints from user.
	PathHints []string `json:"path_hints,omitempty"`

	// AdditionalContext is user-provided context (docs, examples, etc.).
	AdditionalContext string `json:"additional_context,omitempty"`

	// Category is the feature category for routing/prioritization.
	Category FeatureCategory `json:"category"`
}

// FeatureCategory categorizes features.
type FeatureCategory string

const (
	FeatureCategoryUnspecified   FeatureCategory = ""
	FeatureCategoryNewFeature    FeatureCategory = "new_feature"
	FeatureCategoryBugFix        FeatureCategory = "bug_fix"
	FeatureCategoryRefactor      FeatureCategory = "refactor"
	FeatureCategoryDocumentation FeatureCategory = "documentation"
	FeatureCategoryTest          FeatureCategory = "test"
	FeatureCategoryDependency    FeatureCategory = "dependency"
)

// RepositoryContext provides repository information.
type RepositoryContext struct {
	// RepositoryID is the repository identifier in our system.
	RepositoryID string `json:"repository_id"`

	// RemoteURL is the git remote URL (provider-agnostic).
	RemoteURL string `json:"remote_url"`

	// TargetBranch is the branch to build upon.
	TargetBranch string `json:"target_branch"`

	// BaseCommitSHA is the specific commit to base on (HEAD if empty).
	BaseCommitSHA string `json:"base_commit_sha,omitempty"`

	// FeatureBranchName is the name for feature branch (auto-generated if empty).
	FeatureBranchName string `json:"feature_branch_name,omitempty"`
}

// ExecutionConstraints define execution boundaries.
type ExecutionConstraints struct {
	// MaxSteps limits the number of implementation steps.
	MaxSteps int `json:"max_steps"`

	// TimeoutSeconds is the total execution timeout.
	TimeoutSeconds int `json:"timeout_seconds"`

	// MaxLLMTokens limits the LLM tokens to consume.
	MaxLLMTokens int `json:"max_llm_tokens"`

	// AllowedPaths are glob patterns for allowed modifications.
	AllowedPaths []string `json:"allowed_paths,omitempty"`

	// ForbiddenPaths are glob patterns for forbidden modifications.
	ForbiddenPaths []string `json:"forbidden_paths,omitempty"`

	// Verification specifies verification requirements.
	Verification VerificationRequirements `json:"verification"`
}

// VerificationRequirements specify what verification is needed.
type VerificationRequirements struct {
	RequireBuildSuccess    bool    `json:"require_build_success"`
	RequireTestSuccess     bool    `json:"require_test_success"`
	RequireLintSuccess     bool    `json:"require_lint_success"`
	MinimumCoveragePercent float64 `json:"minimum_coverage_percent"`
	RequireSecurityScan    bool    `json:"require_security_scan"`
}

// RequestMetadata contains request information.
type RequestMetadata struct {
	// RequestedBy identifies who initiated the request.
	RequestedBy string `json:"requested_by"`

	// RequestedAt is when the request was made.
	RequestedAt time.Time `json:"requested_at"`

	// RequestSource identifies the request source (api, ui, ci, etc.).
	RequestSource string `json:"request_source"`

	// Priority is the priority override.
	Priority Priority `json:"priority"`
}

// ===== FEATURE EXECUTION COMPLETED =====

// FeatureExecutionCompletedPayload is the payload for FeatureExecutionCompleted.
type FeatureExecutionCompletedPayload struct {
	// FinalCommit is the final commit on the feature branch.
	FinalCommit CommitInfo `json:"final_commit"`

	// BranchName is the feature branch ready for push.
	BranchName string `json:"branch_name"`

	// Summary contains execution summary.
	Summary ExecutionSummary `json:"summary"`
}

// ExecutionSummary summarizes the execution.
type ExecutionSummary struct {
	StepsCompleted   int   `json:"steps_completed"`
	FilesCreated     int   `json:"files_created"`
	FilesModified    int   `json:"files_modified"`
	FilesDeleted     int   `json:"files_deleted"`
	TotalLinesAdded  int   `json:"total_lines_added"`
	TotalLinesRemoved int   `json:"total_lines_removed"`
	TestsAdded       int   `json:"tests_added"`
	CommitsCreated   int   `json:"commits_created"`
	TotalDurationMS  int64 `json:"total_duration_ms"`
	LLMTokensUsed    int   `json:"llm_tokens_used"`
	IterationCount   int   `json:"iteration_count"`
}

// ===== FEATURE DELIVERED =====

// FeatureDeliveredPayload is the payload for FeatureDelivered.
type FeatureDeliveredPayload struct {
	// BranchName is the created feature branch.
	BranchName string `json:"branch_name"`

	// RemoteRef is the remote reference.
	RemoteRef string `json:"remote_ref"`

	// HeadCommitSHA is the head commit SHA.
	HeadCommitSHA string `json:"head_commit_sha"`

	// Artifacts are references to created artifacts.
	Artifacts []ArtifactReference `json:"artifacts"`

	// Summary is the final delivery summary.
	Summary DeliverySummary `json:"summary"`
}

// ArtifactReference references a created artifact.
type ArtifactReference struct {
	ArtifactID  string `json:"artifact_id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // patch, report, log
	URL         string `json:"url"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
}

// DeliverySummary provides a delivery summary.
type DeliverySummary struct {
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Execution   ExecutionSummary `json:"execution"`
	Tests       *TestResult      `json:"tests,omitempty"`
}

// ===== FEATURE EXECUTION FAILED =====

// FeatureExecutionFailedPayload is the payload for FeatureExecutionFailed.
type FeatureExecutionFailedPayload struct {
	// Classification categorizes the failure.
	Classification FailureClassification `json:"classification"`

	// ErrorCode is the specific error code.
	ErrorCode string `json:"error_code"`

	// ErrorMessage is the error message.
	ErrorMessage string `json:"error_message"`

	// ErrorContext provides additional error context.
	ErrorContext map[string]string `json:"error_context,omitempty"`

	// FailedPhase is the phase where failure occurred.
	FailedPhase ExecutionPhase `json:"failed_phase"`

	// FailedStep is the step where failure occurred (if applicable).
	FailedStep *StepID `json:"failed_step,omitempty"`

	// Recovery provides recovery information.
	Recovery RecoveryInfo `json:"recovery"`
}

// FailureClassification categorizes failures.
type FailureClassification struct {
	Type           FailureType     `json:"type"`
	Severity       FailureSeverity `json:"severity"`
	Retryable      bool            `json:"retryable"`
	UserActionable bool            `json:"user_actionable"`
}

// FailureType categorizes failure types.
type FailureType string

const (
	FailureTypeTransient      FailureType = "transient"      // Network, timeout, rate limit
	FailureTypeDeterministic  FailureType = "deterministic"  // Bad input, auth revoked
	FailureTypeSemantic       FailureType = "semantic"       // LLM output invalid, build failed
	FailureTypeInfrastructure FailureType = "infrastructure" // Worker crash, storage unavailable
)

// FailureSeverity indicates failure severity.
type FailureSeverity string

const (
	FailureSeverityWarning  FailureSeverity = "warning"
	FailureSeverityError    FailureSeverity = "error"
	FailureSeverityCritical FailureSeverity = "critical"
)

// ExecutionPhase identifies execution phases.
type ExecutionPhase string

const (
	ExecutionPhaseInitialization ExecutionPhase = "initialization"
	ExecutionPhaseCheckout       ExecutionPhase = "checkout"
	ExecutionPhaseIndexing       ExecutionPhase = "indexing"
	ExecutionPhaseNormalization  ExecutionPhase = "normalization"
	ExecutionPhaseAnalysis       ExecutionPhase = "analysis"
	ExecutionPhasePlanning       ExecutionPhase = "planning"
	ExecutionPhaseGeneration     ExecutionPhase = "generation"
	ExecutionPhaseVerification   ExecutionPhase = "verification"
	ExecutionPhaseDelivery       ExecutionPhase = "delivery"
)

// RecoveryInfo provides recovery options.
type RecoveryInfo struct {
	CanRetry             bool   `json:"can_retry"`
	CanResume            bool   `json:"can_resume"`
	ResumeFromSequence   uint64 `json:"resume_from_sequence"`
	RecoveryInstructions string `json:"recovery_instructions,omitempty"`
}

// ===== FEATURE EXECUTION ABORTED =====

// FeatureExecutionAbortedPayload is the payload for FeatureExecutionAborted.
type FeatureExecutionAbortedPayload struct {
	// AbortedBy identifies who aborted the execution.
	AbortedBy string `json:"aborted_by"`

	// Reason is the abort reason.
	Reason string `json:"reason"`

	// AbortedAt is when the abort occurred.
	AbortedAt time.Time `json:"aborted_at"`

	// PhaseAtAbort is the phase when aborted.
	PhaseAtAbort ExecutionPhase `json:"phase_at_abort"`

	// StepAtAbort is the step when aborted (if applicable).
	StepAtAbort *StepID `json:"step_at_abort,omitempty"`
}
