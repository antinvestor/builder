package review

import (
	"context"
	"testing"

	appconfig "github.com/antinvestor/builder/apps/reviewer/config"
	"github.com/antinvestor/builder/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDecisionEngine() *ThresholdDecisionEngine {
	cfg := &appconfig.ReviewerConfig{
		MaxRiskScore:             50,
		MaxSecurityRiskScore:     30,
		MaxArchitectureRiskScore: 40,
		MaxCriticalIssues:        0,
		MaxHighIssues:            2,
		MaxBreakingChanges:       0,
		MaxIterations:            3,
		RequireSecurityApproval:  true,
		BlockOnSecrets:           true,
		AllowBreakingChanges:     false,
	}
	return NewThresholdDecisionEngine(cfg)
}

func newCleanSecurityAssessment() *events.SecurityAssessment {
	return &events.SecurityAssessment{
		OverallSecurityScore:   100,
		SecurityStatus:         events.SecurityStatusSecure,
		VulnerabilitiesFound:   []events.Vulnerability{},
		SecretsDetected:        []events.SecretFinding{},
		RequiresSecurityReview: false,
	}
}

func newCleanArchitectureAssessment() *events.ArchitectureAssessment {
	return &events.ArchitectureAssessment{
		OverallArchitectureScore:   100,
		ArchitectureStatus:         events.ArchitectureStatusCompliant,
		BreakingChanges:            []events.BreakingChange{},
		RequiresArchitectureReview: false,
	}
}

func newPassingTestResult() *events.TestResult {
	return &events.TestResult{
		TotalTests:   10,
		PassedTests:  10,
		FailedTests:  0,
		SkippedTests: 0,
		Success:      true,
		DurationMs:   1000,
		Coverage:     80.0,
	}
}

func TestThresholdDecisionEngine_AllChecksPassed_Approve(t *testing.T) {
	engine := newTestDecisionEngine()
	ctx := context.Background()

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     newCleanSecurityAssessment(),
		ArchitectureAssessment: newCleanArchitectureAssessment(),
		TestResult:             newPassingTestResult(),
		IterationNumber:        0,
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, events.ControlDecisionApprove, result.Decision)
	assert.Equal(t, "All checks passed", result.Rationale)
	assert.Empty(t, result.BlockingIssues)
	assert.True(t, result.RiskAssessment.AcceptableForProduction)
}

func TestThresholdDecisionEngine_KillSwitchActive_Abort(t *testing.T) {
	engine := newTestDecisionEngine()
	ctx := context.Background()

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     newCleanSecurityAssessment(),
		ArchitectureAssessment: newCleanArchitectureAssessment(),
		TestResult:             newPassingTestResult(),
		IterationNumber:        0,
		KillSwitchActive:       true,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, events.ControlDecisionAbort, result.Decision)
	assert.Contains(t, result.Rationale, "Kill switch is active")
	assert.Equal(t, events.RiskLevelCritical, result.RiskAssessment.RiskLevel)
}

func TestThresholdDecisionEngine_MaxIterationsReached_Abort(t *testing.T) {
	engine := newTestDecisionEngine()
	ctx := context.Background()

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     newCleanSecurityAssessment(),
		ArchitectureAssessment: newCleanArchitectureAssessment(),
		TestResult:             newPassingTestResult(),
		IterationNumber:        3, // equals MaxIterations
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, events.ControlDecisionAbort, result.Decision)
	assert.Contains(t, result.Rationale, "Maximum iterations")
}

func TestThresholdDecisionEngine_CriticalIssue_Abort(t *testing.T) {
	engine := newTestDecisionEngine()
	ctx := context.Background()

	secAssessment := newCleanSecurityAssessment()
	secAssessment.VulnerabilitiesFound = []events.Vulnerability{
		{
			ID:          "CVE-2024-1234",
			Type:        events.VulnerabilityTypeInjection,
			Severity:    events.VulnerabilitySeverityCritical,
			FilePath:    "src/handler.go",
			LineStart:   42,
			Title:       "SQL Injection",
			Description: "User input not sanitized",
			Remediation: "Use parameterized queries",
		},
	}

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     secAssessment,
		ArchitectureAssessment: newCleanArchitectureAssessment(),
		TestResult:             newPassingTestResult(),
		IterationNumber:        0,
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, events.ControlDecisionAbort, result.Decision)
	assert.Contains(t, result.Rationale, "Critical issues exceed threshold")
	assert.NotEmpty(t, result.BlockingIssues)
}

func TestThresholdDecisionEngine_SecretsDetected_Blocking(t *testing.T) {
	engine := newTestDecisionEngine()
	ctx := context.Background()

	secAssessment := newCleanSecurityAssessment()
	secAssessment.SecretsDetected = []events.SecretFinding{
		{
			Type:        "api_key",
			FilePath:    "config/settings.go",
			LineNumber:  15,
			Description: "AWS API key detected",
			Redacted:    "AKIA***",
		},
	}

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     secAssessment,
		ArchitectureAssessment: newCleanArchitectureAssessment(),
		TestResult:             newPassingTestResult(),
		IterationNumber:        0,
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	// Secrets count as critical issues
	assert.Equal(t, events.ControlDecisionAbort, result.Decision)
	assert.NotEmpty(t, result.BlockingIssues)
	assert.Equal(t, events.ReviewIssueSeverityCritical, result.BlockingIssues[0].Severity)
}

func TestThresholdDecisionEngine_HighRiskScore_Iterate(t *testing.T) {
	// Config without security approval requirement to allow iterate instead of manual review
	cfg := &appconfig.ReviewerConfig{
		MaxRiskScore:             50,
		MaxSecurityRiskScore:     30,
		MaxArchitectureRiskScore: 40,
		MaxCriticalIssues:        0,
		MaxHighIssues:            2,
		MaxIterations:            3,
		RequireSecurityApproval:  false, // Disable to allow iteration
		BlockOnSecrets:           true,
	}
	engine := NewThresholdDecisionEngine(cfg)
	ctx := context.Background()

	secAssessment := newCleanSecurityAssessment()
	secAssessment.OverallSecurityScore = 30 // Risk = 70, above threshold of 30

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     secAssessment,
		ArchitectureAssessment: newCleanArchitectureAssessment(),
		TestResult:             newPassingTestResult(),
		IterationNumber:        0,
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, events.ControlDecisionIterate, result.Decision)
	assert.Contains(t, result.Rationale, "security issues")
}

func TestThresholdDecisionEngine_BreakingChanges_Iterate(t *testing.T) {
	engine := newTestDecisionEngine()
	ctx := context.Background()

	archAssessment := newCleanArchitectureAssessment()
	archAssessment.BreakingChanges = []events.BreakingChange{
		{
			ChangeType:  events.BreakingChangeRemovedAPI,
			Description: "Removed public API function",
			FilePath:    "api/handler.go",
			Symbol:      "GetUser",
			Impact:      "Breaks all consumers",
			Severity:    events.ReviewIssueSeverityHigh,
		},
	}

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     newCleanSecurityAssessment(),
		ArchitectureAssessment: archAssessment,
		TestResult:             newPassingTestResult(),
		IterationNumber:        0,
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, events.ControlDecisionIterate, result.Decision)
	assert.NotEmpty(t, result.BlockingIssues)
}

func TestThresholdDecisionEngine_TestsFailing_Iterate(t *testing.T) {
	engine := newTestDecisionEngine()
	ctx := context.Background()

	testResult := &events.TestResult{
		TotalTests:   10,
		PassedTests:  7,
		FailedTests:  3,
		SkippedTests: 0,
		Success:      false,
		DurationMs:   1000,
	}

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     newCleanSecurityAssessment(),
		ArchitectureAssessment: newCleanArchitectureAssessment(),
		TestResult:             testResult,
		IterationNumber:        0,
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, events.ControlDecisionIterate, result.Decision)
	assert.Contains(t, result.Rationale, "tests are not passing")
}

func TestThresholdDecisionEngine_LowCoverage_Iterate(t *testing.T) {
	cfg := &appconfig.ReviewerConfig{
		MaxRiskScore:    50,
		MaxIterations:   3,
		ReviewThresholds: events.ReviewThresholds{
			MaxRiskScore:      50,
			MinTestCoverage:   70.0,
			MaxCriticalIssues: 0,
			MaxHighIssues:     2,
			MaxIterations:     3,
		},
	}
	engine := NewThresholdDecisionEngine(cfg)
	ctx := context.Background()

	testResult := &events.TestResult{
		TotalTests:  10,
		PassedTests: 10,
		Success:     true,
		Coverage:    50.0, // Below 70% threshold
	}

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     newCleanSecurityAssessment(),
		ArchitectureAssessment: newCleanArchitectureAssessment(),
		TestResult:             testResult,
		IterationNumber:        0,
		KillSwitchActive:       false,
		Thresholds:             cfg.ReviewThresholds,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, events.ControlDecisionIterate, result.Decision)
	assert.Contains(t, result.Rationale, "tests are not passing")
}

func TestThresholdDecisionEngine_SecurityReviewRequired_ManualReview(t *testing.T) {
	engine := newTestDecisionEngine()
	ctx := context.Background()

	secAssessment := newCleanSecurityAssessment()
	secAssessment.RequiresSecurityReview = true
	secAssessment.SecurityReviewReason = "New authentication flow"

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     secAssessment,
		ArchitectureAssessment: newCleanArchitectureAssessment(),
		TestResult:             newPassingTestResult(),
		IterationNumber:        0,
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, events.ControlDecisionManualReview, result.Decision)
	assert.Contains(t, result.Rationale, "Manual review required")
}

func TestThresholdDecisionEngine_ApproveWithWarnings(t *testing.T) {
	cfg := &appconfig.ReviewerConfig{
		MaxRiskScore:            50,
		MaxIterations:           3,
		RequireSecurityApproval: false, // Disable to allow approval with warnings
		MaxHighIssues:           5,
	}
	engine := NewThresholdDecisionEngine(cfg)
	ctx := context.Background()

	archAssessment := newCleanArchitectureAssessment()
	archAssessment.OverallArchitectureScore = 80 // Risk = 20, within threshold

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     newCleanSecurityAssessment(),
		ArchitectureAssessment: archAssessment,
		TestResult:             newPassingTestResult(),
		IterationNumber:        0,
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	// With 20 architecture risk but within threshold, should approve
	assert.Equal(t, events.ControlDecisionApprove, result.Decision)
}

func TestThresholdDecisionEngine_IterationGuidance_Generated(t *testing.T) {
	engine := newTestDecisionEngine()
	ctx := context.Background()

	testResult := &events.TestResult{
		TotalTests:  10,
		PassedTests: 7,
		FailedTests: 3,
		Success:     false,
	}

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     newCleanSecurityAssessment(),
		ArchitectureAssessment: newCleanArchitectureAssessment(),
		TestResult:             testResult,
		IterationNumber:        1,
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, events.ControlDecisionIterate, result.Decision)
	assert.NotNil(t, result.IterationGuidance)
	assert.Contains(t, result.IterationGuidance.MustFix, "Fix failing tests")
	assert.Contains(t, result.IterationGuidance.Context, "Iteration 2 of 3")
}

func TestThresholdDecisionEngine_NextActions_Generated(t *testing.T) {
	tests := []struct {
		name           string
		decision       events.ControlDecision
		setupRequest   func() *DecisionRequest
		expectedAction events.ControlDecision
	}{
		{
			name:     "approve generates complete action",
			decision: events.ControlDecisionApprove,
			setupRequest: func() *DecisionRequest {
				return &DecisionRequest{
					ExecutionID:            events.NewExecutionID(),
					SecurityAssessment:     newCleanSecurityAssessment(),
					ArchitectureAssessment: newCleanArchitectureAssessment(),
					TestResult:             newPassingTestResult(),
				}
			},
			expectedAction: events.ControlDecisionMarkComplete,
		},
		{
			name:     "abort generates rollback action",
			decision: events.ControlDecisionAbort,
			setupRequest: func() *DecisionRequest {
				return &DecisionRequest{
					ExecutionID:      events.NewExecutionID(),
					KillSwitchActive: true,
				}
			},
			expectedAction: events.ControlDecisionRollback,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newTestDecisionEngine()
			ctx := context.Background()

			result, err := engine.MakeDecision(ctx, tt.setupRequest())

			require.NoError(t, err)
			require.NotEmpty(t, result.NextActions)
			assert.Equal(t, tt.expectedAction, result.NextActions[0].Action)
		})
	}
}

func TestThresholdDecisionEngine_RiskAssessment_Calculated(t *testing.T) {
	engine := newTestDecisionEngine()
	ctx := context.Background()

	secAssessment := newCleanSecurityAssessment()
	secAssessment.OverallSecurityScore = 70 // Security risk = 30

	archAssessment := newCleanArchitectureAssessment()
	archAssessment.OverallArchitectureScore = 80 // Architecture risk = 20

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     secAssessment,
		ArchitectureAssessment: archAssessment,
		TestResult:             newPassingTestResult(),
		IterationNumber:        0,
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	// Average risk = (30 + 20 + 0) / 3 = ~17 (test risk is 0 since passing)
	assert.Less(t, result.RiskAssessment.OverallRiskScore, 50)
	assert.Equal(t, 30, result.RiskAssessment.SecurityRiskScore)
	assert.Equal(t, 20, result.RiskAssessment.ArchitectureRiskScore)
}

func TestThresholdDecisionEngine_RiskLevel_Calculated(t *testing.T) {
	tests := []struct {
		name          string
		securityScore int
		expectedLevel events.RiskLevel
	}{
		{"low risk", 90, events.RiskLevelLow},
		{"medium risk", 60, events.RiskLevelMedium},
		{"high risk", 30, events.RiskLevelHigh},
		{"critical risk", 10, events.RiskLevelCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newTestDecisionEngine()
			ctx := context.Background()

			secAssessment := newCleanSecurityAssessment()
			secAssessment.OverallSecurityScore = tt.securityScore

			req := &DecisionRequest{
				ExecutionID:            events.NewExecutionID(),
				SecurityAssessment:     secAssessment,
				ArchitectureAssessment: newCleanArchitectureAssessment(),
				TestResult:             nil, // No test results
				IterationNumber:        0,
				KillSwitchActive:       false,
			}

			result, err := engine.MakeDecision(ctx, req)

			require.NoError(t, err)
			// With only security assessment, risk level based on security score
			assert.Equal(t, 100-tt.securityScore, result.RiskAssessment.SecurityRiskScore)
		})
	}
}

func TestThresholdDecisionEngine_HighIssuesWithinThreshold_Iterate(t *testing.T) {
	cfg := &appconfig.ReviewerConfig{
		MaxRiskScore:      50,
		MaxCriticalIssues: 0,
		MaxHighIssues:     2,
		MaxIterations:     3,
	}
	engine := NewThresholdDecisionEngine(cfg)
	ctx := context.Background()

	secAssessment := newCleanSecurityAssessment()
	secAssessment.VulnerabilitiesFound = []events.Vulnerability{
		{
			ID:       "vuln-1",
			Severity: events.VulnerabilitySeverityHigh,
			FilePath: "file1.go",
			Title:    "High severity issue 1",
		},
		{
			ID:       "vuln-2",
			Severity: events.VulnerabilitySeverityHigh,
			FilePath: "file2.go",
			Title:    "High severity issue 2",
		},
		{
			ID:       "vuln-3",
			Severity: events.VulnerabilitySeverityHigh,
			FilePath: "file3.go",
			Title:    "High severity issue 3",
		},
	}

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     secAssessment,
		ArchitectureAssessment: newCleanArchitectureAssessment(),
		TestResult:             newPassingTestResult(),
		IterationNumber:        0,
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, events.ControlDecisionIterate, result.Decision)
	assert.Contains(t, result.Rationale, "high severity issues")
}

func TestThresholdDecisionEngine_NilAssessments_Handled(t *testing.T) {
	engine := newTestDecisionEngine()
	ctx := context.Background()

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     nil,
		ArchitectureAssessment: nil,
		TestResult:             nil,
		IterationNumber:        0,
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	// With no assessments, should approve (nothing to block on)
	assert.Equal(t, events.ControlDecisionApprove, result.Decision)
}

func TestThresholdDecisionEngine_UsesRequestThresholds(t *testing.T) {
	cfg := &appconfig.ReviewerConfig{
		MaxRiskScore:  50,
		MaxIterations: 3,
	}
	engine := NewThresholdDecisionEngine(cfg)
	ctx := context.Background()

	// Request with custom thresholds allowing higher risk
	customThresholds := events.ReviewThresholds{
		MaxRiskScore:      80, // Higher threshold
		MaxCriticalIssues: 0,
		MaxIterations:     5,
	}

	secAssessment := newCleanSecurityAssessment()
	secAssessment.OverallSecurityScore = 40 // Risk = 60, would fail default but pass custom

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     secAssessment,
		ArchitectureAssessment: newCleanArchitectureAssessment(),
		TestResult:             newPassingTestResult(),
		IterationNumber:        0,
		KillSwitchActive:       false,
		Thresholds:             customThresholds,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, events.ControlDecisionApprove, result.Decision)
	assert.True(t, result.RiskAssessment.AcceptableForProduction)
}

func TestThresholdDecisionEngine_BlockingIssues_Populated(t *testing.T) {
	engine := newTestDecisionEngine()
	ctx := context.Background()

	secAssessment := newCleanSecurityAssessment()
	secAssessment.SecretsDetected = []events.SecretFinding{
		{
			Type:        "api_key",
			FilePath:    "config.go",
			LineNumber:  10,
			Description: "API key found",
		},
	}
	secAssessment.VulnerabilitiesFound = []events.Vulnerability{
		{
			ID:          "vuln-critical",
			Severity:    events.VulnerabilitySeverityCritical,
			FilePath:    "handler.go",
			LineStart:   20,
			Title:       "Critical vulnerability",
			Description: "Critical issue",
			Remediation: "Fix it",
		},
	}

	req := &DecisionRequest{
		ExecutionID:            events.NewExecutionID(),
		SecurityAssessment:     secAssessment,
		ArchitectureAssessment: newCleanArchitectureAssessment(),
		TestResult:             newPassingTestResult(),
		IterationNumber:        0,
		KillSwitchActive:       false,
	}

	result, err := engine.MakeDecision(ctx, req)

	require.NoError(t, err)
	assert.Len(t, result.BlockingIssues, 2) // 1 secret + 1 critical vulnerability

	// Verify secret is in blocking issues
	hasSecret := false
	hasVuln := false
	for _, issue := range result.BlockingIssues {
		if issue.FilePath == "config.go" {
			hasSecret = true
		}
		if issue.ID == "vuln-critical" {
			hasVuln = true
		}
	}
	assert.True(t, hasSecret, "Secret should be in blocking issues")
	assert.True(t, hasVuln, "Critical vulnerability should be in blocking issues")
}
