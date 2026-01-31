package events

import "time"

// ===== ITERATION EVENTS =====

// IterationRequiredPayload is the payload for IterationRequired.
type IterationRequiredPayload struct {
	// IterationNumber is the iteration number (1-based).
	IterationNumber int `json:"iteration_number"`

	// Reason is why iteration is needed.
	Reason IterationReason `json:"reason"`

	// Issues are the issues triggering iteration.
	Issues []IterationIssue `json:"issues"`

	// ProposedActions are suggested remediation actions.
	ProposedActions []string `json:"proposed_actions"`

	// MaxIterationsRemaining is iterations remaining before abort.
	MaxIterationsRemaining int `json:"max_iterations_remaining"`

	// RequiredAt is when iteration was determined necessary.
	RequiredAt time.Time `json:"required_at"`
}

// IterationReason categorizes why iteration is needed.
type IterationReason string

const (
	IterationReasonBuildFailed  IterationReason = "build_failed"
	IterationReasonTestsFailed  IterationReason = "tests_failed"
	IterationReasonLintFailed   IterationReason = "lint_failed"
	IterationReasonSecurityIssues IterationReason = "security_issues"
	IterationReasonReviewRejected IterationReason = "review_rejected"
	IterationReasonValidationFailed IterationReason = "validation_failed"
)

// IterationIssue describes an issue triggering iteration.
type IterationIssue struct {
	Type        string `json:"type"`
	FilePath    string `json:"file_path,omitempty"`
	LineNumber  int    `json:"line_number,omitempty"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

// IterationStartedPayload is the payload for IterationStarted.
type IterationStartedPayload struct {
	// IterationNumber is the iteration number.
	IterationNumber int `json:"iteration_number"`

	// Reason is why this iteration was started.
	Reason IterationReason `json:"reason"`

	// TargetIssues are issues being addressed.
	TargetIssues []IterationIssue `json:"target_issues"`

	// Strategy is the iteration strategy.
	Strategy IterationStrategy `json:"strategy"`

	// StartedAt is when iteration started.
	StartedAt time.Time `json:"started_at"`
}

// IterationStrategy describes the approach for this iteration.
type IterationStrategy struct {
	// Approach is the high-level approach.
	Approach IterationApproach `json:"approach"`

	// StepsToRetry are step numbers to retry.
	StepsToRetry []int `json:"steps_to_retry,omitempty"`

	// FilesToFix are files to fix.
	FilesToFix []string `json:"files_to_fix,omitempty"`

	// AdditionalContext is context for the LLM.
	AdditionalContext string `json:"additional_context,omitempty"`
}

// IterationApproach describes the iteration approach.
type IterationApproach string

const (
	IterationApproachFix     IterationApproach = "fix"     // Fix specific issues
	IterationApproachRetry   IterationApproach = "retry"   // Retry failed steps
	IterationApproachReplan  IterationApproach = "replan"  // Generate new plan
	IterationApproachPartial IterationApproach = "partial" // Partial fix, continue
)

// IterationCompletedPayload is the payload for IterationCompleted.
type IterationCompletedPayload struct {
	// IterationNumber is the iteration number.
	IterationNumber int `json:"iteration_number"`

	// Result is the iteration result.
	Result IterationResult `json:"result"`

	// IssuesResolved are issues that were resolved.
	IssuesResolved int `json:"issues_resolved"`

	// IssuesRemaining are issues still remaining.
	IssuesRemaining int `json:"issues_remaining"`

	// Changes are changes made during iteration.
	Changes []FileChange `json:"changes,omitempty"`

	// DurationMS is iteration duration.
	DurationMS int64 `json:"duration_ms"`

	// CompletedAt is when iteration completed.
	CompletedAt time.Time `json:"completed_at"`
}

// IterationResult indicates iteration outcome.
type IterationResult string

const (
	IterationResultSuccess   IterationResult = "success"   // All issues resolved
	IterationResultPartial   IterationResult = "partial"   // Some issues resolved
	IterationResultFailed    IterationResult = "failed"    // No progress made
	IterationResultAborted   IterationResult = "aborted"   // Iteration was aborted
)

// ===== ROLLBACK EVENTS =====

// RollbackInitiatedPayload is the payload for RollbackInitiated.
type RollbackInitiatedPayload struct {
	// Reason is why rollback was initiated.
	Reason RollbackReason `json:"reason"`

	// TargetCommitSHA is the commit to roll back to.
	TargetCommitSHA string `json:"target_commit_sha"`

	// CurrentCommitSHA is the current commit before rollback.
	CurrentCommitSHA string `json:"current_commit_sha"`

	// InitiatedBy identifies who initiated the rollback.
	InitiatedBy string `json:"initiated_by"`

	// InitiatedAt is when rollback was initiated.
	InitiatedAt time.Time `json:"initiated_at"`
}

// RollbackReason categorizes rollback reasons.
type RollbackReason string

const (
	RollbackReasonUserRequested   RollbackReason = "user_requested"
	RollbackReasonMaxIterations   RollbackReason = "max_iterations"
	RollbackReasonUnrecoverable   RollbackReason = "unrecoverable_error"
	RollbackReasonTimeout         RollbackReason = "timeout"
	RollbackReasonResourceLimit   RollbackReason = "resource_limit"
	RollbackReasonSecurityConcern RollbackReason = "security_concern"
)

// RollbackCompletedPayload is the payload for RollbackCompleted.
type RollbackCompletedPayload struct {
	// ResultingCommitSHA is the commit after rollback.
	ResultingCommitSHA string `json:"resulting_commit_sha"`

	// FilesReverted is the number of files reverted.
	FilesReverted int `json:"files_reverted"`

	// CommitsReverted is the number of commits reverted.
	CommitsReverted int `json:"commits_reverted"`

	// DurationMS is rollback duration.
	DurationMS int64 `json:"duration_ms"`

	// CompletedAt is when rollback completed.
	CompletedAt time.Time `json:"completed_at"`
}

// RollbackFailedPayload is the payload for RollbackFailed.
type RollbackFailedPayload struct {
	ErrorCode    string            `json:"error_code"`
	ErrorMessage string            `json:"error_message"`
	Retryable    bool              `json:"retryable"`
	ErrorContext map[string]string `json:"error_context,omitempty"`
	FailedAt     time.Time         `json:"failed_at"`
}
