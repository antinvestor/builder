package events

// =============================================================================
// Common Patch Types
// =============================================================================

// Patch represents a code change to be applied.
type Patch struct {
	// FilePath is the path to the file.
	FilePath string `json:"file_path"`

	// OldPath is the previous path (for renames).
	OldPath string `json:"old_path,omitempty"`

	// Action is the file action.
	Action FileAction `json:"action"`

	// OldContent is the original file content.
	OldContent string `json:"old_content,omitempty"`

	// NewContent is the new file content.
	NewContent string `json:"new_content,omitempty"`

	// DiffContent is the unified diff.
	DiffContent string `json:"diff_content,omitempty"`

	// LinesAdded is the number of lines added.
	LinesAdded int `json:"lines_added"`

	// LinesRemoved is the number of lines removed.
	LinesRemoved int `json:"lines_removed"`
}


// =============================================================================
// Security Assessment Helper Types
// =============================================================================

// SecurityVulnerability describes a security vulnerability found in code.
type SecurityVulnerability struct {
	ID          string                `json:"id"`
	Type        VulnerabilityType     `json:"type"`
	Severity    VulnerabilitySeverity `json:"severity"`
	CWE         string                `json:"cwe,omitempty"`
	CVSS        float64               `json:"cvss,omitempty"`
	FilePath    string                `json:"file_path"`
	LineStart   int                   `json:"line_start"`
	LineEnd     int                   `json:"line_end"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	Remediation string                `json:"remediation"`
}

// SecretDetection describes a detected secret in code.
type SecretDetection struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // api_key, password, token, etc.
	FilePath    string `json:"file_path"`
	LineNumber  int    `json:"line_number"`
	Description string `json:"description"`
	Redacted    string `json:"redacted"`
}

// =============================================================================
// Assessment Completion Flags
// =============================================================================

// SecurityAssessment additions - completion flags
func (s *SecurityAssessment) SetPassesReview(passes bool) {
	s.RequiresSecurityReview = !passes
}

// PassesSecurityReview indicates if security review passed.
var _ = func() interface{} {
	type extendedSecurityAssessment struct {
		SecurityAssessment
		PassesSecurityReview bool `json:"passes_security_review"`
	}
	return nil
}()

// =============================================================================
// Control Decision Next Actions
// =============================================================================

// NextAction describes an action to take after a decision.
type NextAction struct {
	// Action is the action type.
	Action string `json:"action"`

	// Target is what the action applies to.
	Target string `json:"target,omitempty"`

	// Details provides additional details.
	Details string `json:"details,omitempty"`

	// Priority is the action priority.
	Priority string `json:"priority"` // immediate, high, medium, low
}

// =============================================================================
// Rollback Types
// =============================================================================

// RollbackTarget describes where to roll back to.
type RollbackTarget struct {
	// CommitSHA is the commit to roll back to.
	CommitSHA string `json:"commit_sha"`

	// Branch is the branch to roll back on.
	Branch string `json:"branch"`

	// Reason is why this target was chosen.
	Reason string `json:"reason"`
}


// =============================================================================
// Test Execution Types
// =============================================================================

// TestExecutionRequestedPayload is the payload for test execution request.
type TestExecutionRequestedPayload struct {
	// ExecutionID is the feature execution ID.
	ExecutionID ExecutionID `json:"execution_id"`

	// Language is the programming language.
	Language string `json:"language"`

	// TestFiles are the test files to run.
	TestFiles []string `json:"test_files"`

	// WorkspacePath is the path to the workspace.
	WorkspacePath string `json:"workspace_path"`
}

// TestExecutionCompletedPayload is the payload for test execution completion.
type TestExecutionCompletedPayload struct {
	// ExecutionID is the feature execution ID.
	ExecutionID ExecutionID `json:"execution_id"`

	// Success indicates if tests passed.
	Success bool `json:"success"`

	// Result contains detailed test results.
	Result *TestResult `json:"result,omitempty"`

	// Error contains error information if failed.
	Error *ExecutionError `json:"error,omitempty"`
}

// TestResult contains test execution results.
type TestResult struct {
	// TotalTests is the total number of tests.
	TotalTests int `json:"total_tests"`

	// PassedTests is the number of passing tests.
	PassedTests int `json:"passed_tests"`

	// FailedTests is the number of failing tests.
	FailedTests int `json:"failed_tests"`

	// SkippedTests is the number of skipped tests.
	SkippedTests int `json:"skipped_tests"`

	// Success indicates overall success.
	Success bool `json:"success"`

	// DurationMs is the total duration in milliseconds.
	DurationMs int64 `json:"duration_ms"`

	// TestCases are individual test case results.
	TestCases []TestCaseResult `json:"test_cases,omitempty"`

	// Coverage is the code coverage percentage.
	Coverage float64 `json:"coverage,omitempty"`
}

// TestCaseResult describes a single test case result.
type TestCaseResult struct {
	Name       string `json:"name"`
	Suite      string `json:"suite,omitempty"`
	Status     string `json:"status"` // passed, failed, skipped
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
	Output     string `json:"output,omitempty"`
}

// ExecutionError describes an execution error.
type ExecutionError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}


