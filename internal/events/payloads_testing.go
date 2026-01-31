package events

import "time"

// =============================================================================
// Test Generation Request
// =============================================================================

// TestGenerationRequestedPayload is the payload for requesting test generation.
type TestGenerationRequestedPayload struct {
	// ExecutionID is the feature execution ID.
	ExecutionID ExecutionID `json:"execution_id"`

	// StepID is the plan step ID that triggered test generation.
	StepID StepID `json:"step_id"`

	// TargetFiles are the implementation files to generate tests for.
	TargetFiles []TestTargetFile `json:"target_files"`

	// TestTypes specifies which types of tests to generate.
	TestTypes []TestType `json:"test_types"`

	// ExistingTests are paths to existing test files for reference.
	ExistingTests []string `json:"existing_tests,omitempty"`

	// CoverageTarget is the target code coverage percentage.
	CoverageTarget float64 `json:"coverage_target"`

	// Context provides additional context for test generation.
	Context *TestGenerationContext `json:"context,omitempty"`

	// RequestedAt is when the request was made.
	RequestedAt time.Time `json:"requested_at"`
}

// TestTargetFile describes a file to generate tests for.
type TestTargetFile struct {
	// Path is the file path.
	Path string `json:"path"`

	// Language is the programming language.
	Language string `json:"language"`

	// Symbols are specific functions/methods to test (empty means all).
	Symbols []string `json:"symbols,omitempty"`

	// ModifiedLines are line numbers that were modified (for targeted tests).
	ModifiedLines []int `json:"modified_lines,omitempty"`
}

// TestGenerationContext provides context for test generation.
type TestGenerationContext struct {
	// Language is the primary programming language.
	Language string `json:"language"`

	// TestFramework is the test framework to use.
	TestFramework string `json:"test_framework"`

	// TestDirectory is the directory where tests should be placed.
	TestDirectory string `json:"test_directory"`

	// TestNamingPattern is the naming pattern for test files.
	TestNamingPattern string `json:"test_naming_pattern,omitempty"`

	// MockingFramework is the mocking framework to use.
	MockingFramework string `json:"mocking_framework,omitempty"`

	// StyleGuide contains test style guidelines.
	StyleGuide string `json:"style_guide,omitempty"`

	// FeatureDescription describes the feature being tested.
	FeatureDescription string `json:"feature_description,omitempty"`

	// AcceptanceCriteria are the acceptance criteria to verify.
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
}

// ===== TEST GENERATION =====

// TestGenerationStartedPayload is the payload for TestGenerationStarted.
type TestGenerationStartedPayload struct {
	ExecutionID ExecutionID `json:"execution_id"`
	StepID      StepID      `json:"step_id"`
	TargetFiles []string    `json:"target_files"`
	TestTypes   []string    `json:"test_types"` // unit, integration, e2e
	StartedAt   time.Time   `json:"started_at"`
}

// TestGenerationCompletedPayload is the payload for TestGenerationCompleted.
type TestGenerationCompletedPayload struct {
	GeneratedTests []GeneratedTest   `json:"generated_tests"`
	LLMInfo        LLMProcessingInfo `json:"llm_info"`
	CompletedAt    time.Time         `json:"completed_at"`
}

// GeneratedTest describes a generated test file.
type GeneratedTest struct {
	FilePath    string   `json:"file_path"`
	TestType    TestType `json:"test_type"`
	TestCount   int      `json:"test_count"`
	TargetFiles []string `json:"target_files"`
	Description string   `json:"description"`
}

// TestType categorizes test types.
type TestType string

const (
	TestTypeUnit        TestType = "unit"
	TestTypeIntegration TestType = "integration"
	TestTypeE2E         TestType = "e2e"
	TestTypeSnapshot    TestType = "snapshot"
	TestTypeProperty    TestType = "property"
)

// TestGenerationFailedPayload is the payload for TestGenerationFailed.
type TestGenerationFailedPayload struct {
	ErrorCode    string            `json:"error_code"`
	ErrorMessage string            `json:"error_message"`
	Retryable    bool              `json:"retryable"`
	ErrorContext map[string]string `json:"error_context,omitempty"`
	FailedAt     time.Time         `json:"failed_at"`
}

// ===== TEST EXECUTION =====

// TestExecutionStartedPayload is the payload for TestExecutionStarted.
type TestExecutionStartedPayload struct {
	TestCommand    string    `json:"test_command"`
	TestFiles      []string  `json:"test_files,omitempty"`
	TimeoutSeconds int       `json:"timeout_seconds"`
	StartedAt      time.Time `json:"started_at"`
}

// TestStatus indicates overall test result.
type TestStatus string

const (
	TestStatusPassed  TestStatus = "passed"
	TestStatusFailed  TestStatus = "failed"
	TestStatusError   TestStatus = "error"   // Test infrastructure error
	TestStatusTimeout TestStatus = "timeout"
	TestStatusSkipped TestStatus = "skipped"
)

// CoverageResult contains code coverage information.
type CoverageResult struct {
	// LineCoverage is line coverage percentage.
	LineCoverage float64 `json:"line_coverage"`

	// BranchCoverage is branch coverage percentage.
	BranchCoverage float64 `json:"branch_coverage,omitempty"`

	// FunctionCoverage is function coverage percentage.
	FunctionCoverage float64 `json:"function_coverage,omitempty"`

	// FileCoverage maps file paths to coverage.
	FileCoverage map[string]FileCoverageResult `json:"file_coverage,omitempty"`

	// UncoveredLines maps files to uncovered line numbers.
	UncoveredLines map[string][]int `json:"uncovered_lines,omitempty"`
}

// FileCoverageResult contains per-file coverage.
type FileCoverageResult struct {
	LineCoverage    float64 `json:"line_coverage"`
	LinesTotal      int     `json:"lines_total"`
	LinesCovered    int     `json:"lines_covered"`
	BranchCoverage  float64 `json:"branch_coverage,omitempty"`
	BranchesTotal   int     `json:"branches_total,omitempty"`
	BranchesCovered int     `json:"branches_covered,omitempty"`
}

// TestExecutionFailedPayload is the payload for TestExecutionFailed.
type TestExecutionFailedPayload struct {
	ExecutionID   ExecutionID       `json:"execution_id"`
	StepID        StepID            `json:"step_id"`
	ErrorCode     string            `json:"error_code"`
	ErrorMessage  string            `json:"error_message"`
	Retryable     bool              `json:"retryable"`
	ErrorContext  map[string]string `json:"error_context,omitempty"`
	PartialOutput string            `json:"partial_output,omitempty"`
	FailedAt      time.Time         `json:"failed_at"`
}

// TestPhase indicates when tests are being run in the feature lifecycle.
type TestPhase string

const (
	// TestPhasePreFeature runs tests before the feature is implemented.
	// These tests should FAIL to prove the feature doesn't exist yet.
	TestPhasePreFeature TestPhase = "pre_feature"

	// TestPhasePostFeature runs tests after the feature is implemented.
	// These tests should PASS to prove the feature works.
	TestPhasePostFeature TestPhase = "post_feature"

	// TestPhaseRegression runs full regression tests.
	TestPhaseRegression TestPhase = "regression"

	// TestPhaseIteration runs tests during iteration/fix cycles.
	TestPhaseIteration TestPhase = "iteration"
)

// MountPath defines a path to mount into the sandbox.
type MountPath struct {
	// HostPath is the path on the host.
	HostPath string `json:"host_path"`

	// ContainerPath is the path in the sandbox.
	ContainerPath string `json:"container_path"`

	// ReadOnly indicates if the mount is read-only.
	ReadOnly bool `json:"read_only"`
}

// =============================================================================
// Failure Classification
// =============================================================================

// TestFailureClassification classifies test failures for appropriate handling.
type TestFailureClassification struct {
	// Category is the failure category.
	Category FailureCategory `json:"category"`

	// Severity is the failure severity.
	Severity FailureSeverity `json:"severity"`

	// Retryable indicates if the failure might succeed on retry.
	Retryable bool `json:"retryable"`

	// RequiresIteration indicates if iteration/fix is needed.
	RequiresIteration bool `json:"requires_iteration"`

	// RequiresRollback indicates if rollback is needed.
	RequiresRollback bool `json:"requires_rollback"`

	// SuggestedAction is the suggested remediation action.
	SuggestedAction FailureAction `json:"suggested_action"`

	// RootCauseAnalysis provides analysis of the failure.
	RootCauseAnalysis string `json:"root_cause_analysis,omitempty"`

	// AffectedComponents are components affected by the failure.
	AffectedComponents []string `json:"affected_components,omitempty"`

	// RelatedFiles are files related to the failure.
	RelatedFiles []string `json:"related_files,omitempty"`
}

// FailureCategory categorizes test failures.
type FailureCategory string

const (
	// FailureCategoryAssertion is an assertion failure (test logic issue).
	FailureCategoryAssertion FailureCategory = "assertion"

	// FailureCategoryCompilation is a compilation/syntax error.
	FailureCategoryCompilation FailureCategory = "compilation"

	// FailureCategoryRuntime is a runtime error (exception, panic).
	FailureCategoryRuntime FailureCategory = "runtime"

	// FailureCategoryTimeout is a timeout failure.
	FailureCategoryTimeout FailureCategory = "timeout"

	// FailureCategoryResource is a resource exhaustion failure.
	FailureCategoryResource FailureCategory = "resource"

	// FailureCategoryEnvironment is an environment/setup issue.
	FailureCategoryEnvironment FailureCategory = "environment"

	// FailureCategoryDependency is a missing/incompatible dependency.
	FailureCategoryDependency FailureCategory = "dependency"

	// FailureCategoryFlaky is a flaky test (intermittent failure).
	FailureCategoryFlaky FailureCategory = "flaky"

	// FailureCategoryRegression is a regression in existing functionality.
	FailureCategoryRegression FailureCategory = "regression"

	// FailureCategoryInfrastructure is a test infrastructure failure.
	FailureCategoryInfrastructure FailureCategory = "infrastructure"
)

// FailureAction is the suggested action for a failure.
type FailureAction string

const (
	// FailureActionRetry retries the test execution.
	FailureActionRetry FailureAction = "retry"

	// FailureActionFixTest fixes the test code.
	FailureActionFixTest FailureAction = "fix_test"

	// FailureActionFixImplementation fixes the implementation code.
	FailureActionFixImplementation FailureAction = "fix_implementation"

	// FailureActionRollback rolls back the changes.
	FailureActionRollback FailureAction = "rollback"

	// FailureActionSkip skips the test (with justification).
	FailureActionSkip FailureAction = "skip"

	// FailureActionManualReview requires manual review.
	FailureActionManualReview FailureAction = "manual_review"

	// FailureActionAbort aborts the feature execution.
	FailureActionAbort FailureAction = "abort"
)

// =============================================================================
// Test Analysis Event (Post-Execution)
// =============================================================================

// TestAnalysisCompletedPayload is emitted after analyzing test results.
type TestAnalysisCompletedPayload struct {
	// ExecutionID is the feature execution ID.
	ExecutionID ExecutionID `json:"execution_id"`

	// StepID is the plan step ID.
	StepID StepID `json:"step_id"`

	// Phase is when the tests were run.
	Phase TestPhase `json:"phase"`

	// TestResult is the raw test result.
	TestResult TestResult `json:"test_result"`

	// Classification is the failure classification (if tests failed).
	Classification *TestFailureClassification `json:"classification,omitempty"`

	// PreFeatureBaseline is the baseline from pre-feature tests.
	PreFeatureBaseline *TestBaseline `json:"pre_feature_baseline,omitempty"`

	// Comparison compares post-feature results to baseline.
	Comparison *TestComparison `json:"comparison,omitempty"`

	// NextAction is the recommended next action.
	NextAction FailureAction `json:"next_action"`

	// AnalyzedAt is when the analysis completed.
	AnalyzedAt time.Time `json:"analyzed_at"`
}

// TestBaseline captures the test state at a point in time.
type TestBaseline struct {
	// TotalTests is the total number of tests.
	TotalTests int `json:"total_tests"`

	// PassingTests are tests that passed.
	PassingTests []string `json:"passing_tests,omitempty"`

	// FailingTests are tests that failed.
	FailingTests []string `json:"failing_tests,omitempty"`

	// SkippedTests are tests that were skipped.
	SkippedTests []string `json:"skipped_tests,omitempty"`

	// Coverage is the coverage at baseline.
	Coverage *CoverageResult `json:"coverage,omitempty"`

	// CapturedAt is when the baseline was captured.
	CapturedAt time.Time `json:"captured_at"`
}

// TestComparison compares test results between phases.
type TestComparison struct {
	// NewlyPassing are tests that started passing.
	NewlyPassing []string `json:"newly_passing,omitempty"`

	// NewlyFailing are tests that started failing (regressions).
	NewlyFailing []string `json:"newly_failing,omitempty"`

	// StillFailing are tests that were failing and still fail.
	StillFailing []string `json:"still_failing,omitempty"`

	// CoverageChange is the change in coverage.
	CoverageChange float64 `json:"coverage_change"`

	// IsValid indicates if post-feature tests meet requirements.
	// True if: new tests pass AND no regressions.
	IsValid bool `json:"is_valid"`

	// ValidationMessage explains the validation result.
	ValidationMessage string `json:"validation_message"`
}

// =============================================================================
// Rollback Signaling
// =============================================================================

// TestRollbackRequestedPayload signals that a rollback is needed.
type TestRollbackRequestedPayload struct {
	// ExecutionID is the feature execution ID.
	ExecutionID ExecutionID `json:"execution_id"`

	// StepID is the step that triggered the rollback.
	StepID StepID `json:"step_id"`

	// Reason is why rollback is requested.
	Reason RollbackReason `json:"reason"`

	// FailedTests are the tests that failed.
	FailedTests []TestCaseResult `json:"failed_tests,omitempty"`

	// Classification is the failure classification.
	Classification *TestFailureClassification `json:"classification,omitempty"`

	// RollbackTo specifies what to rollback to.
	RollbackTo *RollbackTarget `json:"rollback_to,omitempty"`

	// PreserveTests indicates if generated tests should be preserved.
	PreserveTests bool `json:"preserve_tests"`

	// RequestedAt is when the rollback was requested.
	RequestedAt time.Time `json:"requested_at"`
}

// RollbackTargetType specifies the type of rollback target.
type RollbackTargetType string

const (
	// RollbackTargetCommit rolls back to a specific commit.
	RollbackTargetCommit RollbackTargetType = "commit"

	// RollbackTargetStep rolls back to before a specific step.
	RollbackTargetStep RollbackTargetType = "step"

	// RollbackTargetFiles rolls back specific files.
	RollbackTargetFiles RollbackTargetType = "files"

	// RollbackTargetFull performs a full rollback to initial state.
	RollbackTargetFull RollbackTargetType = "full"
)

// TestRollbackCompletedPayload is emitted when rollback completes.
type TestRollbackCompletedPayload struct {
	// ExecutionID is the feature execution ID.
	ExecutionID ExecutionID `json:"execution_id"`

	// StepID is the step that triggered the rollback.
	StepID StepID `json:"step_id"`

	// Success indicates if rollback succeeded.
	Success bool `json:"success"`

	// RolledBackTo describes what was rolled back to.
	RolledBackTo *RollbackTarget `json:"rolled_back_to"`

	// RestoredFiles are files that were restored.
	RestoredFiles []string `json:"restored_files,omitempty"`

	// ErrorMessage is set if rollback failed.
	ErrorMessage string `json:"error_message,omitempty"`

	// CompletedAt is when the rollback completed.
	CompletedAt time.Time `json:"completed_at"`
}

// =============================================================================
// Iteration Request (from Test Failures)
// =============================================================================

// TestIterationRequestedPayload requests iteration to fix test failures.
type TestIterationRequestedPayload struct {
	// ExecutionID is the feature execution ID.
	ExecutionID ExecutionID `json:"execution_id"`

	// StepID is the step to iterate on.
	StepID StepID `json:"step_id"`

	// IterationNumber is the iteration number (1-based).
	IterationNumber int `json:"iteration_number"`

	// MaxIterations is the maximum allowed iterations.
	MaxIterations int `json:"max_iterations"`

	// FailedTests are the tests that need to pass.
	FailedTests []TestCaseResult `json:"failed_tests"`

	// Classification is the failure classification.
	Classification *TestFailureClassification `json:"classification"`

	// PreviousAttempts contains information about previous attempts.
	PreviousAttempts []IterationAttempt `json:"previous_attempts,omitempty"`

	// RequestedAt is when the iteration was requested.
	RequestedAt time.Time `json:"requested_at"`
}

// IterationAttempt records a previous iteration attempt.
type IterationAttempt struct {
	// IterationNumber is the iteration number.
	IterationNumber int `json:"iteration_number"`

	// ChangesApplied describes what changes were made.
	ChangesApplied string `json:"changes_applied"`

	// TestResult is the result of running tests.
	TestResult *TestResult `json:"test_result"`

	// AttemptedAt is when the attempt was made.
	AttemptedAt time.Time `json:"attempted_at"`
}
