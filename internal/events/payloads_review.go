package events

import "time"

// ===== BUILD EXECUTION =====

// BuildStartedPayload is the payload for BuildStarted.
type BuildStartedPayload struct {
	BuildCommand   string    `json:"build_command"`
	TimeoutSeconds int       `json:"timeout_seconds"`
	StartedAt      time.Time `json:"started_at"`
}

// BuildCompletedPayload is the payload for BuildCompleted.
type BuildCompletedPayload struct {
	Result      BuildResult `json:"result"`
	CompletedAt time.Time   `json:"completed_at"`
}

// BuildResult contains build execution results.
type BuildResult struct {
	// Status is the build status.
	Status BuildStatus `json:"status"`

	// DurationMS is the build duration.
	DurationMS int64 `json:"duration_ms"`

	// Output is the build output (truncated if large).
	Output string `json:"output,omitempty"`

	// ExitCode is the build command exit code.
	ExitCode int `json:"exit_code"`

	// Warnings is the number of warnings.
	Warnings int `json:"warnings"`

	// Errors is the number of errors.
	Errors int `json:"errors"`

	// Artifacts are build artifacts produced.
	Artifacts []BuildArtifact `json:"artifacts,omitempty"`
}

// BuildStatus indicates build result.
type BuildStatus string

const (
	BuildStatusSuccess BuildStatus = "success"
	BuildStatusFailed  BuildStatus = "failed"
	BuildStatusError   BuildStatus = "error"   // Build infrastructure error
	BuildStatusTimeout BuildStatus = "timeout"
)

// BuildArtifact describes a build artifact.
type BuildArtifact struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Type      string `json:"type"` // binary, library, bundle, etc.
}

// BuildFailedPayload is the payload for BuildFailed.
type BuildFailedPayload struct {
	ErrorCode     string            `json:"error_code"`
	ErrorMessage  string            `json:"error_message"`
	Retryable     bool              `json:"retryable"`
	ErrorContext  map[string]string `json:"error_context,omitempty"`
	PartialOutput string            `json:"partial_output,omitempty"`
	FailedAt      time.Time         `json:"failed_at"`
}

// ===== CODE REVIEW =====

// ReviewStartedPayload is the payload for ReviewStarted.
type ReviewStartedPayload struct {
	FilesToReview []string      `json:"files_to_review"`
	ReviewTypes   []ReviewType  `json:"review_types"`
	StartedAt     time.Time     `json:"started_at"`
}

// ReviewType categorizes review types.
type ReviewType string

const (
	ReviewTypeCodeQuality ReviewType = "code_quality"
	ReviewTypeSecurity    ReviewType = "security"
	ReviewTypePerformance ReviewType = "performance"
	ReviewTypeBestPractice ReviewType = "best_practice"
	ReviewTypeArchitecture ReviewType = "architecture"
)

// ReviewCompletedPayload is the payload for ReviewCompleted.
type ReviewCompletedPayload struct {
	Assessment  ReviewAssessment  `json:"assessment"`
	LLMInfo     LLMProcessingInfo `json:"llm_info"`
	CompletedAt time.Time         `json:"completed_at"`
}

// ReviewAssessment contains the review results.
type ReviewAssessment struct {
	// OverallScore is 0-100 quality score.
	OverallScore int `json:"overall_score"`

	// Recommendation is the overall recommendation.
	Recommendation ReviewRecommendation `json:"recommendation"`

	// Summary is an executive summary.
	Summary string `json:"summary"`

	// Issues are identified issues.
	Issues []ReviewIssue `json:"issues,omitempty"`

	// Suggestions are improvement suggestions.
	Suggestions []ReviewSuggestion `json:"suggestions,omitempty"`

	// Metrics are code quality metrics.
	Metrics CodeQualityMetrics `json:"metrics"`
}

// ReviewRecommendation indicates the review recommendation.
type ReviewRecommendation string

const (
	ReviewRecommendationApprove         ReviewRecommendation = "approve"
	ReviewRecommendationApproveMinor    ReviewRecommendation = "approve_with_minor_changes"
	ReviewRecommendationRequestChanges  ReviewRecommendation = "request_changes"
	ReviewRecommendationReject          ReviewRecommendation = "reject"
)

// ReviewIssue describes an issue found during review.
type ReviewIssue struct {
	// ID uniquely identifies this issue.
	ID string `json:"id"`

	// Type is the issue type.
	Type ReviewIssueType `json:"type"`

	// Severity is the issue severity.
	Severity ReviewIssueSeverity `json:"severity"`

	// FilePath is the file containing the issue.
	FilePath string `json:"file_path"`

	// LineStart is the starting line number.
	LineStart int `json:"line_start"`

	// LineEnd is the ending line number.
	LineEnd int `json:"line_end"`

	// Title is a short title.
	Title string `json:"title"`

	// Description is a detailed description.
	Description string `json:"description"`

	// Suggestion is a suggested fix.
	Suggestion string `json:"suggestion,omitempty"`

	// CodeSnippet is the problematic code.
	CodeSnippet string `json:"code_snippet,omitempty"`
}

// ReviewIssueType categorizes review issues.
type ReviewIssueType string

const (
	ReviewIssueTypeBug            ReviewIssueType = "bug"
	ReviewIssueTypeSecurity       ReviewIssueType = "security"
	ReviewIssueTypePerformance    ReviewIssueType = "performance"
	ReviewIssueTypeMaintainability ReviewIssueType = "maintainability"
	ReviewIssueTypeStyle          ReviewIssueType = "style"
	ReviewIssueTypeDocumentation  ReviewIssueType = "documentation"
	ReviewIssueTypeComplexity     ReviewIssueType = "complexity"
	ReviewIssueTypeDeadCode       ReviewIssueType = "dead_code"
	ReviewIssueTypeDuplication    ReviewIssueType = "duplication"
)

// ReviewIssueSeverity indicates issue severity.
type ReviewIssueSeverity string

const (
	ReviewIssueSeverityInfo     ReviewIssueSeverity = "info"
	ReviewIssueSeverityLow      ReviewIssueSeverity = "low"
	ReviewIssueSeverityMedium   ReviewIssueSeverity = "medium"
	ReviewIssueSeverityHigh     ReviewIssueSeverity = "high"
	ReviewIssueSeverityCritical ReviewIssueSeverity = "critical"
)

// ReviewSuggestion is an improvement suggestion.
type ReviewSuggestion struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	FilePath    string `json:"file_path,omitempty"`
	LineNumber  int    `json:"line_number,omitempty"`
	Priority    string `json:"priority"` // low, medium, high
}

// CodeQualityMetrics contains quality metrics.
type CodeQualityMetrics struct {
	// Complexity metrics
	CyclomaticComplexity float64 `json:"cyclomatic_complexity"`
	CognitiveComplexity  float64 `json:"cognitive_complexity"`

	// Size metrics
	LinesOfCode      int `json:"lines_of_code"`
	LinesOfComments  int `json:"lines_of_comments"`
	CommentRatio     float64 `json:"comment_ratio"`

	// Quality indicators
	DuplicationPercent float64 `json:"duplication_percent"`
	TechnicalDebtHours float64 `json:"technical_debt_hours,omitempty"`
}

// ReviewFailedPayload is the payload for ReviewFailed.
type ReviewFailedPayload struct {
	ErrorCode    string            `json:"error_code"`
	ErrorMessage string            `json:"error_message"`
	Retryable    bool              `json:"retryable"`
	ErrorContext map[string]string `json:"error_context,omitempty"`
	FailedAt     time.Time         `json:"failed_at"`
}

// ===== SECURITY SCAN =====

// SecurityScanStartedPayload is the payload for SecurityScanStarted.
type SecurityScanStartedPayload struct {
	FilesToScan []string  `json:"files_to_scan"`
	ScanTypes   []string  `json:"scan_types"` // sast, secrets, dependencies
	StartedAt   time.Time `json:"started_at"`
}

// SecurityScanCompletedPayload is the payload for SecurityScanCompleted.
type SecurityScanCompletedPayload struct {
	Result      SecurityScanResult `json:"result"`
	CompletedAt time.Time          `json:"completed_at"`
}

// SecurityScanResult contains security scan results.
type SecurityScanResult struct {
	// Status indicates if security issues were found.
	Status SecurityScanStatus `json:"status"`

	// Vulnerabilities are found vulnerabilities.
	Vulnerabilities []Vulnerability `json:"vulnerabilities,omitempty"`

	// SecretsFound are potential secrets/credentials found.
	SecretsFound []SecretFinding `json:"secrets_found,omitempty"`

	// DependencyIssues are dependency vulnerabilities.
	DependencyIssues []DependencyVulnerability `json:"dependency_issues,omitempty"`

	// Summary is an executive summary.
	Summary string `json:"summary"`
}

// SecurityScanStatus indicates scan result.
type SecurityScanStatus string

const (
	SecurityScanStatusClean    SecurityScanStatus = "clean"
	SecurityScanStatusWarnings SecurityScanStatus = "warnings"
	SecurityScanStatusCritical SecurityScanStatus = "critical"
)

// Vulnerability describes a code vulnerability.
type Vulnerability struct {
	ID          string                  `json:"id"`
	Type        VulnerabilityType       `json:"type"`
	Severity    VulnerabilitySeverity   `json:"severity"`
	CWE         string                  `json:"cwe,omitempty"`
	CVSS        float64                 `json:"cvss,omitempty"`
	FilePath    string                  `json:"file_path"`
	LineStart   int                     `json:"line_start"`
	LineEnd     int                     `json:"line_end"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	Remediation string                  `json:"remediation"`
}

// VulnerabilityType categorizes vulnerabilities.
type VulnerabilityType string

const (
	VulnerabilityTypeInjection     VulnerabilityType = "injection"
	VulnerabilityTypeXSS           VulnerabilityType = "xss"
	VulnerabilityTypePathTraversal VulnerabilityType = "path_traversal"
	VulnerabilityTypeInsecureAuth  VulnerabilityType = "insecure_auth"
	VulnerabilityTypeDataExposure  VulnerabilityType = "data_exposure"
	VulnerabilityTypeCrypto        VulnerabilityType = "weak_crypto"
	VulnerabilityTypeSSRF          VulnerabilityType = "ssrf"
	VulnerabilityTypeDeserialization VulnerabilityType = "deserialization"
)

// VulnerabilitySeverity indicates vulnerability severity.
type VulnerabilitySeverity string

const (
	VulnerabilitySeverityLow      VulnerabilitySeverity = "low"
	VulnerabilitySeverityMedium   VulnerabilitySeverity = "medium"
	VulnerabilitySeverityHigh     VulnerabilitySeverity = "high"
	VulnerabilitySeverityCritical VulnerabilitySeverity = "critical"
)

// SecretFinding describes a potential secret found in code.
type SecretFinding struct {
	Type        string `json:"type"` // api_key, password, token, etc.
	FilePath    string `json:"file_path"`
	LineNumber  int    `json:"line_number"`
	Description string `json:"description"`
	Redacted    string `json:"redacted"` // Redacted value for reference
}

// DependencyVulnerability describes a vulnerable dependency.
type DependencyVulnerability struct {
	Package        string                `json:"package"`
	Version        string                `json:"version"`
	Severity       VulnerabilitySeverity `json:"severity"`
	CVE            string                `json:"cve,omitempty"`
	CVSS           float64               `json:"cvss,omitempty"`
	Title          string                `json:"title"`
	Description    string                `json:"description"`
	FixedVersion   string                `json:"fixed_version,omitempty"`
	DependencyPath []string              `json:"dependency_path,omitempty"`
}

// =============================================================================
// REVIEW & CONTROL AGENT PAYLOADS
// =============================================================================

// ===== COMPREHENSIVE REVIEW REQUEST =====

// ComprehensiveReviewRequestedPayload initiates a full review.
type ComprehensiveReviewRequestedPayload struct {
	// ExecutionID is the feature execution ID.
	ExecutionID ExecutionID `json:"execution_id"`

	// StepID is the current step ID (if applicable).
	StepID StepID `json:"step_id,omitempty"`

	// ReviewPhase indicates when the review is happening.
	ReviewPhase ReviewPhase `json:"review_phase"`

	// Patches are the patches to review.
	Patches []PatchReference `json:"patches"`

	// TestResults are the test execution results.
	TestResults *TestResult `json:"test_results,omitempty"`

	// PreviousReviews are previous review results (for iteration).
	PreviousReviews []ReviewReference `json:"previous_reviews,omitempty"`

	// Context provides additional review context.
	Context *ReviewContext `json:"context,omitempty"`

	// RequestedAt is when the review was requested.
	RequestedAt time.Time `json:"requested_at"`
}

// ReviewPhase indicates when in the pipeline the review occurs.
type ReviewPhase string

const (
	// ReviewPhasePatch reviews generated patches before application.
	ReviewPhasePatch ReviewPhase = "patch"

	// ReviewPhasePostImplementation reviews after all patches applied.
	ReviewPhasePostImplementation ReviewPhase = "post_implementation"

	// ReviewPhaseFinal is the final review before completion.
	ReviewPhaseFinal ReviewPhase = "final"

	// ReviewPhaseIteration reviews iteration changes.
	ReviewPhaseIteration ReviewPhase = "iteration"
)

// PatchReference references a patch for review.
type PatchReference struct {
	// PatchID uniquely identifies the patch.
	PatchID string `json:"patch_id"`

	// FilePath is the file being modified.
	FilePath string `json:"file_path"`

	// ChangeType is the type of change.
	ChangeType ChangeType `json:"change_type"`

	// LinesAdded is the number of lines added.
	LinesAdded int `json:"lines_added"`

	// LinesRemoved is the number of lines removed.
	LinesRemoved int `json:"lines_removed"`

	// DiffContent is the unified diff.
	DiffContent string `json:"diff_content,omitempty"`
}

// ReviewReference references a previous review.
type ReviewReference struct {
	// ReviewID is the review identifier.
	ReviewID string `json:"review_id"`

	// Decision is the previous decision.
	Decision ControlDecision `json:"decision"`

	// Timestamp is when the review occurred.
	Timestamp time.Time `json:"timestamp"`
}

// ReviewContext provides context for the review.
type ReviewContext struct {
	// FeatureDescription describes the feature being built.
	FeatureDescription string `json:"feature_description"`

	// AcceptanceCriteria are the acceptance criteria.
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`

	// IterationNumber is the current iteration (0 for initial).
	IterationNumber int `json:"iteration_number"`

	// PreviousIssues are issues from previous reviews.
	PreviousIssues []ReviewIssue `json:"previous_issues,omitempty"`

	// RepositoryContext provides repo information.
	RepositoryContext *RepositoryContext `json:"repository_context,omitempty"`
}

// ===== COMPREHENSIVE REVIEW RESULT =====

// ComprehensiveReviewCompletedPayload is emitted when review completes.
type ComprehensiveReviewCompletedPayload struct {
	// ExecutionID is the feature execution ID.
	ExecutionID ExecutionID `json:"execution_id"`

	// StepID is the step ID (if applicable).
	StepID StepID `json:"step_id,omitempty"`

	// ReviewID uniquely identifies this review.
	ReviewID string `json:"review_id"`

	// Decision is the control decision.
	Decision ControlDecision `json:"decision"`

	// RiskAssessment is the comprehensive risk assessment.
	RiskAssessment RiskAssessment `json:"risk_assessment"`

	// SecurityAssessment is the security-specific assessment.
	SecurityAssessment SecurityAssessment `json:"security_assessment"`

	// ArchitectureAssessment is the architecture-specific assessment.
	ArchitectureAssessment ArchitectureAssessment `json:"architecture_assessment"`

	// QualityAssessment is the code quality assessment.
	QualityAssessment ReviewAssessment `json:"quality_assessment"`

	// Issues are all identified issues.
	Issues []ReviewIssue `json:"issues,omitempty"`

	// BlockingIssues are issues that block approval.
	BlockingIssues []ReviewIssue `json:"blocking_issues,omitempty"`

	// Recommendations are recommendations for improvement.
	Recommendations []ReviewSuggestion `json:"recommendations,omitempty"`

	// DecisionRationale explains the decision.
	DecisionRationale string `json:"decision_rationale"`

	// NextActions are the next actions to take.
	NextActions []ReviewNextAction `json:"next_actions,omitempty"`

	// LLMInfo contains LLM processing details.
	LLMInfo LLMProcessingInfo `json:"llm_info"`

	// CompletedAt is when the review completed.
	CompletedAt time.Time `json:"completed_at"`
}

// ControlDecision is the decision from the review.
type ControlDecision string

const (
	// ControlDecisionApprove approves the changes to continue.
	ControlDecisionApprove ControlDecision = "approve"

	// ControlDecisionApproveWithWarnings approves with non-blocking warnings.
	ControlDecisionApproveWithWarnings ControlDecision = "approve_with_warnings"

	// ControlDecisionIterate requests iteration to fix issues.
	ControlDecisionIterate ControlDecision = "iterate"

	// ControlDecisionRollback requests rollback of changes.
	ControlDecisionRollback ControlDecision = "rollback"

	// ControlDecisionAbort aborts the feature execution entirely.
	ControlDecisionAbort ControlDecision = "abort"

	// ControlDecisionMarkComplete marks the feature as complete.
	ControlDecisionMarkComplete ControlDecision = "mark_complete"

	// ControlDecisionManualReview requires manual human review.
	ControlDecisionManualReview ControlDecision = "manual_review"
)

// ReviewNextAction describes an action to take.
type ReviewNextAction struct {
	// Action is the action type.
	Action ControlDecision `json:"action"`

	// Target is what the action applies to.
	Target string `json:"target,omitempty"`

	// Details provides additional details.
	Details string `json:"details,omitempty"`

	// Priority is the action priority.
	Priority string `json:"priority"` // immediate, high, medium, low
}

// ===== RISK ASSESSMENT =====

// RiskAssessment provides a comprehensive risk evaluation.
type RiskAssessment struct {
	// OverallRiskScore is 0-100 (0=no risk, 100=critical risk).
	OverallRiskScore int `json:"overall_risk_score"`

	// RiskLevel is the categorical risk level.
	RiskLevel RiskLevel `json:"risk_level"`

	// SecurityRiskScore is 0-100 security-specific risk.
	SecurityRiskScore int `json:"security_risk_score"`

	// ArchitectureRiskScore is 0-100 architecture-specific risk.
	ArchitectureRiskScore int `json:"architecture_risk_score"`

	// QualityRiskScore is 0-100 quality-specific risk.
	QualityRiskScore int `json:"quality_risk_score"`

	// TestRiskScore is 0-100 test coverage/quality risk.
	TestRiskScore int `json:"test_risk_score"`

	// ComplexityRiskScore is 0-100 complexity risk.
	ComplexityRiskScore int `json:"complexity_risk_score"`

	// RegressionRiskScore is 0-100 regression risk.
	RegressionRiskScore int `json:"regression_risk_score"`

	// RiskFactors are the factors contributing to risk.
	RiskFactors []RiskFactor `json:"risk_factors"`

	// Mitigations are recommended mitigations.
	Mitigations []RiskMitigation `json:"mitigations,omitempty"`

	// AcceptableForProduction indicates if risk is acceptable.
	AcceptableForProduction bool `json:"acceptable_for_production"`

	// AcceptanceThreshold is the threshold used for decision.
	AcceptanceThreshold int `json:"acceptance_threshold"`
}

// RiskFactor describes a specific risk factor.
type RiskFactor struct {
	// Category is the risk category.
	Category RiskCategory `json:"category"`

	// Factor is a description of the risk factor.
	Factor string `json:"factor"`

	// Contribution is how much this contributes to overall score (0-100).
	Contribution int `json:"contribution"`

	// Evidence provides evidence for this risk.
	Evidence []string `json:"evidence,omitempty"`

	// AffectedFiles are files affected by this risk.
	AffectedFiles []string `json:"affected_files,omitempty"`
}

// RiskCategory categorizes risk factors.
type RiskCategory string

const (
	RiskCategorySecurity       RiskCategory = "security"
	RiskCategoryArchitecture   RiskCategory = "architecture"
	RiskCategoryBreakingChange RiskCategory = "breaking_change"
	RiskCategoryPerformance    RiskCategory = "performance"
	RiskCategoryMaintainability RiskCategory = "maintainability"
	RiskCategoryComplexity     RiskCategory = "complexity"
	RiskCategoryTestCoverage   RiskCategory = "test_coverage"
	RiskCategoryDependency     RiskCategory = "dependency"
	RiskCategoryRegression     RiskCategory = "regression"
	RiskCategoryDataIntegrity  RiskCategory = "data_integrity"
)

// RiskMitigation describes a recommended mitigation.
type RiskMitigation struct {
	// RiskCategory is the risk being mitigated.
	RiskCategory RiskCategory `json:"risk_category"`

	// Mitigation describes the mitigation.
	Mitigation string `json:"mitigation"`

	// Effort is the effort required (low, medium, high).
	Effort string `json:"effort"`

	// Impact is how much this reduces risk (1-100).
	Impact int `json:"impact"`
}

// ===== SECURITY ASSESSMENT =====

// SecurityAssessment provides security-specific evaluation.
type SecurityAssessment struct {
	// OverallSecurityScore is 0-100 (100=secure, 0=critical issues).
	OverallSecurityScore int `json:"overall_security_score"`

	// SecurityStatus indicates security status.
	SecurityStatus SecurityStatus `json:"security_status"`

	// VulnerabilitiesFound are found vulnerabilities.
	VulnerabilitiesFound []Vulnerability `json:"vulnerabilities_found,omitempty"`

	// SecretsDetected are detected secrets/credentials.
	SecretsDetected []SecretFinding `json:"secrets_detected,omitempty"`

	// DependencyVulnerabilities are vulnerable dependencies.
	DependencyVulnerabilities []DependencyVulnerability `json:"dependency_vulnerabilities,omitempty"`

	// SecurityRegressions are security regressions from baseline.
	SecurityRegressions []SecurityRegression `json:"security_regressions,omitempty"`

	// InsecurePatterns are insecure coding patterns detected.
	InsecurePatterns []InsecurePattern `json:"insecure_patterns,omitempty"`

	// AuthorizationIssues are authorization-related issues.
	AuthorizationIssues []AuthorizationIssue `json:"authorization_issues,omitempty"`

	// DataHandlingIssues are data handling concerns.
	DataHandlingIssues []DataHandlingIssue `json:"data_handling_issues,omitempty"`

	// ComplianceIssues are compliance-related issues.
	ComplianceIssues []ComplianceIssue `json:"compliance_issues,omitempty"`

	// RequiresSecurityReview indicates if manual security review is needed.
	RequiresSecurityReview bool `json:"requires_security_review"`

	// SecurityReviewReason explains why security review is needed.
	SecurityReviewReason string `json:"security_review_reason,omitempty"`
}

// SecurityStatus indicates overall security status.
type SecurityStatus string

const (
	SecurityStatusSecure   SecurityStatus = "secure"
	SecurityStatusWarnings SecurityStatus = "warnings"
	SecurityStatusCritical SecurityStatus = "critical"
	SecurityStatusBlocked  SecurityStatus = "blocked" // Cannot proceed
)

// SecurityRegression indicates a security regression.
type SecurityRegression struct {
	// RegressionType is the type of regression.
	RegressionType SecurityRegressionType `json:"regression_type"`

	// Description describes the regression.
	Description string `json:"description"`

	// FilePath is the affected file.
	FilePath string `json:"file_path"`

	// LineNumber is the line number.
	LineNumber int `json:"line_number,omitempty"`

	// Severity is the regression severity.
	Severity VulnerabilitySeverity `json:"severity"`

	// Baseline is what the baseline was.
	Baseline string `json:"baseline,omitempty"`

	// Current is the current state.
	Current string `json:"current,omitempty"`
}

// SecurityRegressionType categorizes security regressions.
type SecurityRegressionType string

const (
	SecurityRegressionRemovedValidation   SecurityRegressionType = "removed_validation"
	SecurityRegressionWeakenedAuth        SecurityRegressionType = "weakened_auth"
	SecurityRegressionExposedData         SecurityRegressionType = "exposed_data"
	SecurityRegressionRemovedEncryption   SecurityRegressionType = "removed_encryption"
	SecurityRegressionWeakenedPermissions SecurityRegressionType = "weakened_permissions"
	SecurityRegressionInsecureDefault     SecurityRegressionType = "insecure_default"
)

// InsecurePattern describes an insecure coding pattern.
type InsecurePattern struct {
	// PatternType is the pattern type.
	PatternType InsecurePatternType `json:"pattern_type"`

	// Description describes the issue.
	Description string `json:"description"`

	// FilePath is the affected file.
	FilePath string `json:"file_path"`

	// LineStart is the starting line.
	LineStart int `json:"line_start"`

	// LineEnd is the ending line.
	LineEnd int `json:"line_end"`

	// CodeSnippet is the problematic code.
	CodeSnippet string `json:"code_snippet,omitempty"`

	// Remediation is the suggested fix.
	Remediation string `json:"remediation"`

	// OWASPID maps to OWASP category.
	OWASPID string `json:"owasp_id,omitempty"`

	// CWE maps to CWE category.
	CWE string `json:"cwe,omitempty"`
}

// InsecurePatternType categorizes insecure patterns.
type InsecurePatternType string

const (
	InsecurePatternSQLInjection       InsecurePatternType = "sql_injection"
	InsecurePatternXSS                InsecurePatternType = "xss"
	InsecurePatternCommandInjection   InsecurePatternType = "command_injection"
	InsecurePatternPathTraversal      InsecurePatternType = "path_traversal"
	InsecurePatternSSRF               InsecurePatternType = "ssrf"
	InsecurePatternOpenRedirect       InsecurePatternType = "open_redirect"
	InsecurePatternInsecureDeserialize InsecurePatternType = "insecure_deserialize"
	InsecurePatternHardcodedCreds     InsecurePatternType = "hardcoded_credentials"
	InsecurePatternWeakCrypto         InsecurePatternType = "weak_crypto"
	InsecurePatternInsecureRandom     InsecurePatternType = "insecure_random"
	InsecurePatternMissingAuth        InsecurePatternType = "missing_auth"
	InsecurePatternMissingValidation  InsecurePatternType = "missing_validation"
	InsecurePatternLogSensitiveData   InsecurePatternType = "log_sensitive_data"
	InsecurePatternInsecureTLS        InsecurePatternType = "insecure_tls"
)

// AuthorizationIssue describes an authorization concern.
type AuthorizationIssue struct {
	// IssueType is the issue type.
	IssueType AuthIssueType `json:"issue_type"`

	// Description describes the issue.
	Description string `json:"description"`

	// FilePath is the affected file.
	FilePath string `json:"file_path"`

	// Endpoint is the affected endpoint (if applicable).
	Endpoint string `json:"endpoint,omitempty"`

	// Severity is the issue severity.
	Severity VulnerabilitySeverity `json:"severity"`
}

// AuthIssueType categorizes authorization issues.
type AuthIssueType string

const (
	AuthIssueMissingAuth       AuthIssueType = "missing_auth"
	AuthIssueWeakAuth          AuthIssueType = "weak_auth"
	AuthIssuePrivilegeEscalation AuthIssueType = "privilege_escalation"
	AuthIssueIDOR              AuthIssueType = "idor"
	AuthIssueBrokenAccessControl AuthIssueType = "broken_access_control"
)

// DataHandlingIssue describes a data handling concern.
type DataHandlingIssue struct {
	// IssueType is the issue type.
	IssueType DataIssueType `json:"issue_type"`

	// Description describes the issue.
	Description string `json:"description"`

	// FilePath is the affected file.
	FilePath string `json:"file_path"`

	// DataType is the type of data affected.
	DataType string `json:"data_type,omitempty"` // PII, credentials, financial, etc.

	// Severity is the issue severity.
	Severity VulnerabilitySeverity `json:"severity"`
}

// DataIssueType categorizes data handling issues.
type DataIssueType string

const (
	DataIssueUnencrypted     DataIssueType = "unencrypted"
	DataIssueOverExposed     DataIssueType = "over_exposed"
	DataIssueMissingMasking  DataIssueType = "missing_masking"
	DataIssuePIILeakage      DataIssueType = "pii_leakage"
	DataIssueInsecureStorage DataIssueType = "insecure_storage"
	DataIssueMissingRetention DataIssueType = "missing_retention"
)

// ComplianceIssue describes a compliance concern.
type ComplianceIssue struct {
	// Standard is the compliance standard.
	Standard ComplianceStandard `json:"standard"`

	// Requirement is the specific requirement.
	Requirement string `json:"requirement"`

	// Description describes the issue.
	Description string `json:"description"`

	// Severity is the issue severity.
	Severity VulnerabilitySeverity `json:"severity"`
}

// ComplianceStandard identifies compliance standards.
type ComplianceStandard string

const (
	ComplianceStandardGDPR     ComplianceStandard = "gdpr"
	ComplianceStandardSOC2     ComplianceStandard = "soc2"
	ComplianceStandardHIPAA    ComplianceStandard = "hipaa"
	ComplianceStandardPCIDSS   ComplianceStandard = "pci_dss"
	ComplianceStandardOWASP    ComplianceStandard = "owasp"
	ComplianceStandardCISBench ComplianceStandard = "cis_benchmark"
)

// ===== ARCHITECTURE ASSESSMENT =====

// ArchitectureAssessment provides architecture-specific evaluation.
type ArchitectureAssessment struct {
	// OverallArchitectureScore is 0-100 architecture quality.
	OverallArchitectureScore int `json:"overall_architecture_score"`

	// ArchitectureStatus indicates architecture compliance status.
	ArchitectureStatus ArchitectureStatus `json:"architecture_status"`

	// BreakingChanges are detected breaking changes.
	BreakingChanges []BreakingChange `json:"breaking_changes,omitempty"`

	// DependencyViolations are dependency rule violations.
	DependencyViolations []DependencyViolation `json:"dependency_violations,omitempty"`

	// LayeringViolations are layering/boundary violations.
	LayeringViolations []LayeringViolation `json:"layering_violations,omitempty"`

	// InterfaceChanges are interface contract changes.
	InterfaceChanges []InterfaceChange `json:"interface_changes,omitempty"`

	// CircularDependencies are circular dependencies introduced.
	CircularDependencies []CircularDependency `json:"circular_dependencies,omitempty"`

	// PatternViolations are design pattern violations.
	PatternViolations []PatternViolation `json:"pattern_violations,omitempty"`

	// APIContractViolations are API contract violations.
	APIContractViolations []APIContractViolation `json:"api_contract_violations,omitempty"`

	// Recommendations are architecture recommendations.
	Recommendations []ArchitectureRecommendation `json:"recommendations,omitempty"`

	// RequiresArchitectureReview indicates if architect review is needed.
	RequiresArchitectureReview bool `json:"requires_architecture_review"`

	// ArchitectureReviewReason explains why review is needed.
	ArchitectureReviewReason string `json:"architecture_review_reason,omitempty"`
}

// ArchitectureStatus indicates architecture compliance status.
type ArchitectureStatus string

const (
	ArchitectureStatusCompliant    ArchitectureStatus = "compliant"
	ArchitectureStatusWarnings     ArchitectureStatus = "warnings"
	ArchitectureStatusViolations   ArchitectureStatus = "violations"
	ArchitectureStatusBlocked      ArchitectureStatus = "blocked"
)

// BreakingChange describes a breaking change.
type BreakingChange struct {
	// ChangeType is the type of breaking change.
	ChangeType BreakingChangeType `json:"change_type"`

	// Description describes the breaking change.
	Description string `json:"description"`

	// FilePath is the affected file.
	FilePath string `json:"file_path"`

	// Symbol is the affected symbol (function, type, etc.).
	Symbol string `json:"symbol,omitempty"`

	// Impact describes the impact.
	Impact string `json:"impact"`

	// AffectedConsumers are affected consumers.
	AffectedConsumers []string `json:"affected_consumers,omitempty"`

	// MigrationPath describes migration path.
	MigrationPath string `json:"migration_path,omitempty"`

	// Severity indicates severity.
	Severity ReviewIssueSeverity `json:"severity"`
}

// BreakingChangeType categorizes breaking changes.
type BreakingChangeType string

const (
	BreakingChangeRemovedAPI       BreakingChangeType = "removed_api"
	BreakingChangeChangedSignature BreakingChangeType = "changed_signature"
	BreakingChangeChangedBehavior  BreakingChangeType = "changed_behavior"
	BreakingChangeRemovedField     BreakingChangeType = "removed_field"
	BreakingChangeChangedType      BreakingChangeType = "changed_type"
	BreakingChangeRenamedSymbol    BreakingChangeType = "renamed_symbol"
	BreakingChangeChangedDefault   BreakingChangeType = "changed_default"
)

// DependencyViolation describes a dependency rule violation.
type DependencyViolation struct {
	// ViolationType is the violation type.
	ViolationType DependencyViolationType `json:"violation_type"`

	// FromModule is the module with the violation.
	FromModule string `json:"from_module"`

	// ToModule is the module being improperly depended on.
	ToModule string `json:"to_module"`

	// FilePath is the file with the violation.
	FilePath string `json:"file_path"`

	// LineNumber is the line number.
	LineNumber int `json:"line_number,omitempty"`

	// Rule is the rule being violated.
	Rule string `json:"rule"`

	// Severity indicates severity.
	Severity ReviewIssueSeverity `json:"severity"`
}

// DependencyViolationType categorizes dependency violations.
type DependencyViolationType string

const (
	DependencyViolationForbidden    DependencyViolationType = "forbidden"
	DependencyViolationCircular     DependencyViolationType = "circular"
	DependencyViolationSkipLayer    DependencyViolationType = "skip_layer"
	DependencyViolationInternalLeak DependencyViolationType = "internal_leak"
)

// LayeringViolation describes a layering/boundary violation.
type LayeringViolation struct {
	// ViolationType is the violation type.
	ViolationType LayeringViolationType `json:"violation_type"`

	// Description describes the violation.
	Description string `json:"description"`

	// SourceLayer is the source layer.
	SourceLayer string `json:"source_layer"`

	// TargetLayer is the target layer.
	TargetLayer string `json:"target_layer"`

	// FilePath is the file with the violation.
	FilePath string `json:"file_path"`

	// Severity indicates severity.
	Severity ReviewIssueSeverity `json:"severity"`
}

// LayeringViolationType categorizes layering violations.
type LayeringViolationType string

const (
	LayeringViolationSkipLayer       LayeringViolationType = "skip_layer"
	LayeringViolationReverseFlow     LayeringViolationType = "reverse_flow"
	LayeringViolationBoundaryBreach  LayeringViolationType = "boundary_breach"
	LayeringViolationDomainLeak      LayeringViolationType = "domain_leak"
)

// InterfaceChange describes a change to an interface/contract.
type InterfaceChange struct {
	// InterfaceName is the interface name.
	InterfaceName string `json:"interface_name"`

	// ChangeType is the type of change.
	ChangeType InterfaceChangeType `json:"change_type"`

	// Description describes the change.
	Description string `json:"description"`

	// FilePath is the file path.
	FilePath string `json:"file_path"`

	// IsBreaking indicates if this is a breaking change.
	IsBreaking bool `json:"is_breaking"`

	// Implementations are known implementations.
	Implementations []string `json:"implementations,omitempty"`
}

// InterfaceChangeType categorizes interface changes.
type InterfaceChangeType string

const (
	InterfaceChangeAddedMethod    InterfaceChangeType = "added_method"
	InterfaceChangeRemovedMethod  InterfaceChangeType = "removed_method"
	InterfaceChangeChangedMethod  InterfaceChangeType = "changed_method"
	InterfaceChangeRenamedMethod  InterfaceChangeType = "renamed_method"
)

// CircularDependency describes a circular dependency.
type CircularDependency struct {
	// Cycle is the dependency cycle (A -> B -> C -> A).
	Cycle []string `json:"cycle"`

	// IntroducedBy is the file that introduced this cycle.
	IntroducedBy string `json:"introduced_by"`

	// IsNew indicates if this is a new circular dependency.
	IsNew bool `json:"is_new"`
}

// PatternViolation describes a design pattern violation.
type PatternViolation struct {
	// PatternName is the pattern being violated.
	PatternName string `json:"pattern_name"`

	// ViolationType is the violation type.
	ViolationType string `json:"violation_type"`

	// Description describes the violation.
	Description string `json:"description"`

	// FilePath is the file path.
	FilePath string `json:"file_path"`

	// Recommendation is the recommended fix.
	Recommendation string `json:"recommendation"`
}

// APIContractViolation describes an API contract violation.
type APIContractViolation struct {
	// Endpoint is the API endpoint.
	Endpoint string `json:"endpoint"`

	// ViolationType is the violation type.
	ViolationType APIViolationType `json:"violation_type"`

	// Description describes the violation.
	Description string `json:"description"`

	// ExpectedBehavior is the expected behavior.
	ExpectedBehavior string `json:"expected_behavior,omitempty"`

	// ActualBehavior is the actual behavior.
	ActualBehavior string `json:"actual_behavior,omitempty"`

	// IsBreaking indicates if this is breaking.
	IsBreaking bool `json:"is_breaking"`
}

// APIViolationType categorizes API violations.
type APIViolationType string

const (
	APIViolationChangedResponse   APIViolationType = "changed_response"
	APIViolationChangedRequest    APIViolationType = "changed_request"
	APIViolationRemovedEndpoint   APIViolationType = "removed_endpoint"
	APIViolationChangedStatusCode APIViolationType = "changed_status_code"
	APIViolationChangedErrorFormat APIViolationType = "changed_error_format"
)

// ArchitectureRecommendation is an architecture improvement suggestion.
type ArchitectureRecommendation struct {
	// Category is the recommendation category.
	Category string `json:"category"`

	// Recommendation is the recommendation.
	Recommendation string `json:"recommendation"`

	// Rationale explains the rationale.
	Rationale string `json:"rationale"`

	// Priority is the priority level.
	Priority string `json:"priority"` // low, medium, high, critical

	// AffectedFiles are affected files.
	AffectedFiles []string `json:"affected_files,omitempty"`
}

// ===== CONTROL EVENTS =====

// FeatureIterationRequestedPayload requests iteration from review.
type FeatureIterationRequestedPayload struct {
	// ExecutionID is the feature execution ID.
	ExecutionID ExecutionID `json:"execution_id"`

	// ReviewID is the review that triggered iteration.
	ReviewID string `json:"review_id"`

	// IterationNumber is the iteration number.
	IterationNumber int `json:"iteration_number"`

	// Issues are issues that need to be fixed.
	Issues []ReviewIssue `json:"issues"`

	// IterationGuidance provides guidance for iteration.
	IterationGuidance *IterationGuidance `json:"iteration_guidance"`

	// MaxRemainingIterations is iterations remaining before abort.
	MaxRemainingIterations int `json:"max_remaining_iterations"`

	// RequestedAt is when iteration was requested.
	RequestedAt time.Time `json:"requested_at"`
}

// IterationGuidance provides guidance for the iteration.
type IterationGuidance struct {
	// Priority indicates what to fix first.
	Priority []string `json:"priority"`

	// MustFix are issues that must be fixed.
	MustFix []string `json:"must_fix"`

	// ShouldFix are issues that should be fixed.
	ShouldFix []string `json:"should_fix,omitempty"`

	// MayIgnore are issues that may be ignored this iteration.
	MayIgnore []string `json:"may_ignore,omitempty"`

	// Context provides additional context for fixing.
	Context string `json:"context,omitempty"`
}

// FeatureAbortRequestedPayload requests feature abort from review.
type FeatureAbortRequestedPayload struct {
	// ExecutionID is the feature execution ID.
	ExecutionID ExecutionID `json:"execution_id"`

	// ReviewID is the review that triggered abort.
	ReviewID string `json:"review_id"`

	// AbortReason is why abort is requested.
	AbortReason AbortReason `json:"abort_reason"`

	// AbortDetails provides details about the abort.
	AbortDetails string `json:"abort_details"`

	// BlockingIssues are the issues causing abort.
	BlockingIssues []ReviewIssue `json:"blocking_issues,omitempty"`

	// RollbackRequired indicates if rollback is needed.
	RollbackRequired bool `json:"rollback_required"`

	// RollbackTarget is what to rollback to.
	RollbackTarget *RollbackTarget `json:"rollback_target,omitempty"`

	// RequestedAt is when abort was requested.
	RequestedAt time.Time `json:"requested_at"`
}

// AbortReason categorizes abort reasons.
type AbortReason string

const (
	AbortReasonSecurityCritical    AbortReason = "security_critical"
	AbortReasonArchitectureBlocked AbortReason = "architecture_blocked"
	AbortReasonMaxIterations       AbortReason = "max_iterations"
	AbortReasonUnrecoverableError  AbortReason = "unrecoverable_error"
	AbortReasonResourceExhausted   AbortReason = "resource_exhausted"
	AbortReasonTimeout             AbortReason = "timeout"
	AbortReasonKillSwitch          AbortReason = "kill_switch"
	AbortReasonManualRequest       AbortReason = "manual_request"
	AbortReasonPolicyViolation     AbortReason = "policy_violation"
)

// FeatureCompleteRequestedPayload requests feature completion.
type FeatureCompleteRequestedPayload struct {
	// ExecutionID is the feature execution ID.
	ExecutionID ExecutionID `json:"execution_id"`

	// ReviewID is the final review ID.
	ReviewID string `json:"review_id"`

	// FinalAssessment is the final assessment summary.
	FinalAssessment *RiskAssessment `json:"final_assessment"`

	// CompletionSummary summarizes the completed feature.
	CompletionSummary string `json:"completion_summary"`

	// Warnings are non-blocking warnings to note.
	Warnings []string `json:"warnings,omitempty"`

	// RequestedAt is when completion was requested.
	RequestedAt time.Time `json:"requested_at"`
}

// ===== KILL SWITCH EVENTS =====

// KillSwitchActivatedPayload is emitted when kill switch is triggered.
type KillSwitchActivatedPayload struct {
	// Scope is the kill switch scope.
	Scope KillSwitchScope `json:"scope"`

	// ExecutionID is the affected execution (if feature-scoped).
	ExecutionID ExecutionID `json:"execution_id,omitempty"`

	// Reason is why the kill switch was activated.
	Reason KillSwitchReason `json:"reason"`

	// ActivatedBy identifies who/what activated the kill switch.
	ActivatedBy string `json:"activated_by"`

	// Details provides additional details.
	Details string `json:"details,omitempty"`

	// AffectedExecutions are affected executions (if global scope).
	AffectedExecutions []ExecutionID `json:"affected_executions,omitempty"`

	// RollbackAll indicates if all changes should be rolled back.
	RollbackAll bool `json:"rollback_all"`

	// ActivatedAt is when the kill switch was activated.
	ActivatedAt time.Time `json:"activated_at"`
}

// KillSwitchScope defines the kill switch scope.
type KillSwitchScope string

const (
	// KillSwitchScopeFeature affects a single feature execution.
	KillSwitchScopeFeature KillSwitchScope = "feature"

	// KillSwitchScopeGlobal affects all feature executions.
	KillSwitchScopeGlobal KillSwitchScope = "global"

	// KillSwitchScopeRepository affects all executions for a repository.
	KillSwitchScopeRepository KillSwitchScope = "repository"
)

// KillSwitchReason categorizes kill switch reasons.
type KillSwitchReason string

const (
	KillSwitchReasonManual           KillSwitchReason = "manual"
	KillSwitchReasonSecurityBreach   KillSwitchReason = "security_breach"
	KillSwitchReasonResourceExhausted KillSwitchReason = "resource_exhausted"
	KillSwitchReasonAnomalyDetected  KillSwitchReason = "anomaly_detected"
	KillSwitchReasonRateLimitExceeded KillSwitchReason = "rate_limit_exceeded"
	KillSwitchReasonSystemOverload   KillSwitchReason = "system_overload"
	KillSwitchReasonPolicyViolation  KillSwitchReason = "policy_violation"
)

// KillSwitchDeactivatedPayload is emitted when kill switch is deactivated.
type KillSwitchDeactivatedPayload struct {
	// Scope is the kill switch scope.
	Scope KillSwitchScope `json:"scope"`

	// ExecutionID is the affected execution (if feature-scoped).
	ExecutionID ExecutionID `json:"execution_id,omitempty"`

	// DeactivatedBy identifies who deactivated the kill switch.
	DeactivatedBy string `json:"deactivated_by"`

	// Reason is why it was deactivated.
	Reason string `json:"reason"`

	// DeactivatedAt is when it was deactivated.
	DeactivatedAt time.Time `json:"deactivated_at"`
}

// KillSwitchStatusPayload provides kill switch status.
type KillSwitchStatusPayload struct {
	// GlobalActive indicates if global kill switch is active.
	GlobalActive bool `json:"global_active"`

	// GlobalReason is the reason if active.
	GlobalReason KillSwitchReason `json:"global_reason,omitempty"`

	// FeatureSwitches are feature-specific kill switches.
	FeatureSwitches map[string]FeatureKillSwitch `json:"feature_switches,omitempty"`

	// RepositorySwitches are repository-specific kill switches.
	RepositorySwitches map[string]bool `json:"repository_switches,omitempty"`

	// AsOf is when this status was captured.
	AsOf time.Time `json:"as_of"`
}

// FeatureKillSwitch represents a feature-specific kill switch.
type FeatureKillSwitch struct {
	// Active indicates if the switch is active.
	Active bool `json:"active"`

	// Reason is the reason if active.
	Reason KillSwitchReason `json:"reason,omitempty"`

	// ActivatedAt is when it was activated.
	ActivatedAt time.Time `json:"activated_at,omitempty"`

	// ActivatedBy is who activated it.
	ActivatedBy string `json:"activated_by,omitempty"`
}

// ===== REVIEW THRESHOLDS & CONFIGURATION =====

// ReviewThresholds configures review decision thresholds.
type ReviewThresholds struct {
	// MaxRiskScore is the maximum acceptable risk score (0-100).
	MaxRiskScore int `json:"max_risk_score"`

	// MaxSecurityRiskScore is max acceptable security risk.
	MaxSecurityRiskScore int `json:"max_security_risk_score"`

	// MaxArchitectureRiskScore is max acceptable architecture risk.
	MaxArchitectureRiskScore int `json:"max_architecture_risk_score"`

	// MinTestCoverage is minimum required test coverage.
	MinTestCoverage float64 `json:"min_test_coverage"`

	// MaxCriticalIssues is max critical issues allowed.
	MaxCriticalIssues int `json:"max_critical_issues"`

	// MaxHighIssues is max high severity issues allowed.
	MaxHighIssues int `json:"max_high_issues"`

	// MaxBreakingChanges is max breaking changes allowed.
	MaxBreakingChanges int `json:"max_breaking_changes"`

	// MaxIterations is max iterations before abort.
	MaxIterations int `json:"max_iterations"`

	// RequireSecurityApproval requires security team approval.
	RequireSecurityApproval bool `json:"require_security_approval"`

	// RequireArchitectApproval requires architect approval.
	RequireArchitectApproval bool `json:"require_architect_approval"`
}

// DefaultReviewThresholds returns conservative default thresholds.
func DefaultReviewThresholds() ReviewThresholds {
	return ReviewThresholds{
		MaxRiskScore:             50,  // Conservative: 50/100 max risk
		MaxSecurityRiskScore:     30,  // Very conservative for security
		MaxArchitectureRiskScore: 40,  // Conservative for architecture
		MinTestCoverage:          70,  // Require 70% coverage
		MaxCriticalIssues:        0,   // Zero tolerance for critical
		MaxHighIssues:            2,   // Allow up to 2 high issues
		MaxBreakingChanges:       0,   // Zero breaking changes
		MaxIterations:            3,   // Max 3 iterations
		RequireSecurityApproval:  true,
		RequireArchitectApproval: false,
	}
}
