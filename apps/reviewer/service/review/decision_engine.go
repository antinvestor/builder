package review

import (
	"context"
	"fmt"
	"strings"

	appconfig "github.com/antinvestor/builder/apps/reviewer/config"
	"github.com/antinvestor/builder/internal/events"
	"github.com/pitabwire/util"
)

// ThresholdDecisionEngine implements comprehensive threshold-based decision making.
type ThresholdDecisionEngine struct {
	cfg *appconfig.ReviewerConfig
}

// NewThresholdDecisionEngine creates a new threshold-based decision engine.
func NewThresholdDecisionEngine(cfg *appconfig.ReviewerConfig) *ThresholdDecisionEngine {
	return &ThresholdDecisionEngine{cfg: cfg}
}

// MakeDecision makes a comprehensive control decision based on thresholds.
func (e *ThresholdDecisionEngine) MakeDecision(ctx context.Context, req *DecisionRequest) (*DecisionResult, error) {
	log := util.Log(ctx)
	log.Debug("making decision",
		"execution_id", req.ExecutionID.String(),
		"iteration", req.IterationNumber,
		"kill_switch_active", req.KillSwitchActive,
	)

	result := &DecisionResult{
		BlockingIssues: []events.ReviewIssue{},
		NextActions:    []events.ReviewNextAction{},
		Warnings:       []string{},
	}

	thresholds := e.getThresholds(req)

	// Check kill switch first - highest priority
	if req.KillSwitchActive {
		return e.createAbortResult(result, events.AbortReasonKillSwitch, "Kill switch is active"), nil
	}

	// Evaluate iteration count
	if iterationResult := e.evaluateIterationCount(req, thresholds, result); iterationResult != nil {
		return iterationResult, nil
	}

	// Evaluate security assessment
	securityIssues, securityBlocking := e.evaluateSecurityAssessment(req, thresholds, result)
	result.BlockingIssues = append(result.BlockingIssues, securityIssues...)

	// Evaluate architecture assessment
	archIssues, archBlocking := e.evaluateArchitectureAssessment(req, thresholds, result)
	result.BlockingIssues = append(result.BlockingIssues, archIssues...)

	// Evaluate test results
	testPassing := e.evaluateTestResults(req, thresholds, result)

	// Calculate risk assessment
	result.RiskAssessment = e.calculateRiskAssessment(req, thresholds)

	// Count issues by severity
	criticalCount, highCount := e.countIssuesBySeverity(req)

	// Apply decision logic
	decision, rationale := e.determineDecision(
		req,
		thresholds,
		securityBlocking,
		archBlocking,
		testPassing,
		criticalCount,
		highCount,
		result,
	)

	result.Decision = decision
	result.Rationale = rationale

	// Generate next actions
	result.NextActions = e.generateNextActions(result, req)

	// Generate iteration guidance if iterating
	if decision == events.ControlDecisionIterate {
		result.IterationGuidance = e.generateIterationGuidance(result, req, thresholds)
	}

	log.Info("decision made",
		"execution_id", req.ExecutionID.String(),
		"decision", decision,
		"blocking_issues", len(result.BlockingIssues),
		"risk_score", result.RiskAssessment.OverallRiskScore,
	)

	return result, nil
}

func (e *ThresholdDecisionEngine) getThresholds(req *DecisionRequest) events.ReviewThresholds {
	// Use request thresholds if provided, otherwise use config
	if req.Thresholds.MaxRiskScore > 0 {
		return req.Thresholds
	}
	return e.cfg.GetReviewThresholds()
}

func (e *ThresholdDecisionEngine) evaluateIterationCount(
	req *DecisionRequest,
	thresholds events.ReviewThresholds,
	result *DecisionResult,
) *DecisionResult {
	if thresholds.MaxIterations > 0 && req.IterationNumber >= thresholds.MaxIterations {
		return e.createAbortResult(
			result,
			events.AbortReasonMaxIterations,
			fmt.Sprintf("Maximum iterations (%d) reached", thresholds.MaxIterations),
		)
	}
	return nil
}

func (e *ThresholdDecisionEngine) evaluateSecurityAssessment(
	req *DecisionRequest,
	thresholds events.ReviewThresholds,
	result *DecisionResult,
) ([]events.ReviewIssue, bool) {
	var blockingIssues []events.ReviewIssue
	hasBlocking := false

	if req.SecurityAssessment == nil {
		return blockingIssues, false
	}

	sec := req.SecurityAssessment

	// Check for secrets if blocking is enabled
	if e.cfg.BlockOnSecrets && len(sec.SecretsDetected) > 0 {
		hasBlocking = true
		for _, secret := range sec.SecretsDetected {
			blockingIssues = append(blockingIssues, events.ReviewIssue{
				ID:          fmt.Sprintf("secret-%s-%d", secret.FilePath, secret.LineNumber),
				Type:        events.ReviewIssueTypeSecurity,
				Severity:    events.ReviewIssueSeverityCritical,
				FilePath:    secret.FilePath,
				LineStart:   secret.LineNumber,
				Title:       fmt.Sprintf("Secret detected: %s", secret.Type),
				Description: secret.Description,
				Suggestion:  "Remove the secret and use environment variables or a secrets manager",
			})
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d secrets detected in code", len(sec.SecretsDetected)))
	}

	// Check vulnerabilities by severity
	for _, vuln := range sec.VulnerabilitiesFound {
		if vuln.Severity == events.VulnerabilitySeverityCritical {
			hasBlocking = true
			blockingIssues = append(blockingIssues, events.ReviewIssue{
				ID:          vuln.ID,
				Type:        events.ReviewIssueTypeSecurity,
				Severity:    events.ReviewIssueSeverityCritical,
				FilePath:    vuln.FilePath,
				LineStart:   vuln.LineStart,
				LineEnd:     vuln.LineEnd,
				Title:       vuln.Title,
				Description: vuln.Description,
				Suggestion:  vuln.Remediation,
			})
		}
	}

	// Check security score against threshold
	securityRiskScore := 100 - sec.OverallSecurityScore
	if thresholds.MaxSecurityRiskScore > 0 && securityRiskScore > thresholds.MaxSecurityRiskScore {
		hasBlocking = true
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Security risk score (%d) exceeds threshold (%d)",
				securityRiskScore, thresholds.MaxSecurityRiskScore))
	}

	// Check if security review is required
	if sec.RequiresSecurityReview && e.cfg.RequireSecurityApproval {
		hasBlocking = true
		result.Warnings = append(result.Warnings, "Security review required: "+sec.SecurityReviewReason)
	}

	return blockingIssues, hasBlocking
}

func (e *ThresholdDecisionEngine) evaluateArchitectureAssessment(
	req *DecisionRequest,
	thresholds events.ReviewThresholds,
	result *DecisionResult,
) ([]events.ReviewIssue, bool) {
	var blockingIssues []events.ReviewIssue
	hasBlocking := false

	if req.ArchitectureAssessment == nil {
		return blockingIssues, false
	}

	arch := req.ArchitectureAssessment

	// Check breaking changes
	if !e.cfg.AllowBreakingChanges && len(arch.BreakingChanges) > thresholds.MaxBreakingChanges {
		hasBlocking = true
		for _, bc := range arch.BreakingChanges {
			blockingIssues = append(blockingIssues, events.ReviewIssue{
				ID:          fmt.Sprintf("breaking-%s-%s", bc.FilePath, bc.Symbol),
				Type:        events.ReviewIssueTypeBug,
				Severity:    events.ReviewIssueSeverityHigh,
				FilePath:    bc.FilePath,
				Title:       fmt.Sprintf("Breaking change: %s", bc.ChangeType),
				Description: bc.Description,
				Suggestion:  bc.MigrationPath,
			})
		}
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d breaking changes detected (max: %d)",
				len(arch.BreakingChanges), thresholds.MaxBreakingChanges))
	}

	// Check architecture score against threshold
	archRiskScore := 100 - arch.OverallArchitectureScore
	if thresholds.MaxArchitectureRiskScore > 0 && archRiskScore > thresholds.MaxArchitectureRiskScore {
		hasBlocking = true
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Architecture risk score (%d) exceeds threshold (%d)",
				archRiskScore, thresholds.MaxArchitectureRiskScore))
	}

	// Check if architecture review is required
	if arch.RequiresArchitectureReview {
		hasBlocking = true
		result.Warnings = append(result.Warnings, "Architecture review required: "+arch.ArchitectureReviewReason)
	}

	// Check for blocked status
	if arch.ArchitectureStatus == events.ArchitectureStatusBlocked {
		hasBlocking = true
		result.Warnings = append(result.Warnings, "Architecture status is blocked")
	}

	return blockingIssues, hasBlocking
}

func (e *ThresholdDecisionEngine) evaluateTestResults(
	req *DecisionRequest,
	thresholds events.ReviewThresholds,
	result *DecisionResult,
) bool {
	if req.TestResult == nil {
		// No test results - consider passing (tests may not have been run yet)
		return true
	}

	testResult := req.TestResult

	// Check if tests passed
	if !testResult.Success {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Tests failing: %d/%d passed",
				testResult.PassedTests, testResult.TotalTests))
		return false
	}

	// Check coverage threshold if configured
	if thresholds.MinTestCoverage > 0 && testResult.Coverage < thresholds.MinTestCoverage {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Test coverage (%.1f%%) below threshold (%.1f%%)",
				testResult.Coverage, thresholds.MinTestCoverage))
		return false
	}

	return true
}

func (e *ThresholdDecisionEngine) calculateRiskAssessment(
	req *DecisionRequest,
	thresholds events.ReviewThresholds,
) events.RiskAssessment {
	ra := events.RiskAssessment{
		RiskFactors:         []events.RiskFactor{},
		Mitigations:         []events.RiskMitigation{},
		AcceptanceThreshold: thresholds.MaxRiskScore,
	}

	var totalScore int
	var factorCount int

	// Security risk
	if req.SecurityAssessment != nil {
		secRisk := 100 - req.SecurityAssessment.OverallSecurityScore
		ra.SecurityRiskScore = secRisk
		totalScore += secRisk
		factorCount++

		if secRisk > 0 {
			ra.RiskFactors = append(ra.RiskFactors, events.RiskFactor{
				Category:     events.RiskCategorySecurity,
				Factor:       "Security assessment score",
				Contribution: secRisk,
			})
		}
	}

	// Architecture risk
	if req.ArchitectureAssessment != nil {
		archRisk := 100 - req.ArchitectureAssessment.OverallArchitectureScore
		ra.ArchitectureRiskScore = archRisk
		totalScore += archRisk
		factorCount++

		if archRisk > 0 {
			ra.RiskFactors = append(ra.RiskFactors, events.RiskFactor{
				Category:     events.RiskCategoryArchitecture,
				Factor:       "Architecture assessment score",
				Contribution: archRisk,
			})
		}

		// Add breaking change risk
		if len(req.ArchitectureAssessment.BreakingChanges) > 0 {
			bcRisk := len(req.ArchitectureAssessment.BreakingChanges) * 20
			if bcRisk > 100 {
				bcRisk = 100
			}
			totalScore += bcRisk
			factorCount++
			ra.RiskFactors = append(ra.RiskFactors, events.RiskFactor{
				Category:     events.RiskCategoryBreakingChange,
				Factor:       fmt.Sprintf("%d breaking changes", len(req.ArchitectureAssessment.BreakingChanges)),
				Contribution: bcRisk,
			})
		}
	}

	// Test coverage risk
	if req.TestResult != nil {
		if !req.TestResult.Success {
			ra.TestRiskScore = 100
			ra.RiskFactors = append(ra.RiskFactors, events.RiskFactor{
				Category:     events.RiskCategoryTestCoverage,
				Factor:       "Tests failing",
				Contribution: 100,
			})
		} else if thresholds.MinTestCoverage > 0 {
			coverageGap := thresholds.MinTestCoverage - req.TestResult.Coverage
			if coverageGap > 0 {
				ra.TestRiskScore = int(coverageGap)
				ra.RiskFactors = append(ra.RiskFactors, events.RiskFactor{
					Category:     events.RiskCategoryTestCoverage,
					Factor:       fmt.Sprintf("Coverage %.1f%% below target %.1f%%", req.TestResult.Coverage, thresholds.MinTestCoverage),
					Contribution: int(coverageGap),
				})
			}
		}
		totalScore += ra.TestRiskScore
		factorCount++
	}

	// Calculate overall score (average of component scores)
	if factorCount > 0 {
		ra.OverallRiskScore = totalScore / factorCount
	}

	// Determine risk level
	ra.RiskLevel = e.calculateRiskLevel(ra.OverallRiskScore)

	// Determine if acceptable for production
	ra.AcceptableForProduction = ra.OverallRiskScore <= thresholds.MaxRiskScore

	return ra
}

func (e *ThresholdDecisionEngine) calculateRiskLevel(score int) events.RiskLevel {
	switch {
	case score >= 80:
		return events.RiskLevelCritical
	case score >= 60:
		return events.RiskLevelHigh
	case score >= 30:
		return events.RiskLevelMedium
	default:
		return events.RiskLevelLow
	}
}

func (e *ThresholdDecisionEngine) countIssuesBySeverity(req *DecisionRequest) (critical, high int) {
	// Count from security assessment
	if req.SecurityAssessment != nil {
		for _, v := range req.SecurityAssessment.VulnerabilitiesFound {
			switch v.Severity {
			case events.VulnerabilitySeverityCritical:
				critical++
			case events.VulnerabilitySeverityHigh:
				high++
			}
		}
		// Secrets are considered critical
		critical += len(req.SecurityAssessment.SecretsDetected)
	}

	// Count from architecture assessment
	if req.ArchitectureAssessment != nil {
		for _, bc := range req.ArchitectureAssessment.BreakingChanges {
			switch bc.Severity {
			case events.ReviewIssueSeverityCritical:
				critical++
			case events.ReviewIssueSeverityHigh:
				high++
			}
		}
	}

	return critical, high
}

func (e *ThresholdDecisionEngine) determineDecision(
	req *DecisionRequest,
	thresholds events.ReviewThresholds,
	securityBlocking bool,
	archBlocking bool,
	testPassing bool,
	criticalCount int,
	highCount int,
	result *DecisionResult,
) (events.ControlDecision, string) {
	var reasons []string

	// Critical issues - immediate abort
	if criticalCount > thresholds.MaxCriticalIssues {
		reasons = append(reasons, fmt.Sprintf("%d critical issues (max: %d)", criticalCount, thresholds.MaxCriticalIssues))
		return events.ControlDecisionAbort,
			fmt.Sprintf("Critical issues exceed threshold: %s", strings.Join(reasons, "; "))
	}

	// High issues check
	if highCount > thresholds.MaxHighIssues {
		reasons = append(reasons, fmt.Sprintf("%d high severity issues (max: %d)", highCount, thresholds.MaxHighIssues))
	}

	// Risk score check
	if result.RiskAssessment.OverallRiskScore > thresholds.MaxRiskScore {
		reasons = append(reasons, fmt.Sprintf("risk score %d exceeds threshold %d",
			result.RiskAssessment.OverallRiskScore, thresholds.MaxRiskScore))
	}

	// Security blocking
	if securityBlocking {
		reasons = append(reasons, "security issues require attention")
	}

	// Architecture blocking
	if archBlocking {
		reasons = append(reasons, "architecture issues require attention")
	}

	// Tests failing
	if !testPassing {
		reasons = append(reasons, "tests are not passing")
	}

	// Determine final decision
	if len(reasons) == 0 {
		// All checks passed
		if len(result.Warnings) > 0 {
			return events.ControlDecisionApproveWithWarnings,
				fmt.Sprintf("Approved with %d warnings", len(result.Warnings))
		}
		return events.ControlDecisionApprove, "All checks passed"
	}

	// Determine if we should iterate or require manual review
	if securityBlocking && e.cfg.RequireSecurityApproval {
		return events.ControlDecisionManualReview,
			fmt.Sprintf("Manual review required: %s", strings.Join(reasons, "; "))
	}

	return events.ControlDecisionIterate,
		fmt.Sprintf("Issues detected requiring iteration: %s", strings.Join(reasons, "; "))
}

func (e *ThresholdDecisionEngine) generateNextActions(result *DecisionResult, req *DecisionRequest) []events.ReviewNextAction {
	var actions []events.ReviewNextAction

	switch result.Decision {
	case events.ControlDecisionApprove, events.ControlDecisionApproveWithWarnings:
		actions = append(actions, events.ReviewNextAction{
			Action:   events.ControlDecisionMarkComplete,
			Details:  "Proceed with feature completion",
			Priority: "high",
		})

	case events.ControlDecisionIterate:
		if len(result.BlockingIssues) > 0 {
			actions = append(actions, events.ReviewNextAction{
				Action:   events.ControlDecisionIterate,
				Target:   "blocking_issues",
				Details:  fmt.Sprintf("Fix %d blocking issues", len(result.BlockingIssues)),
				Priority: "immediate",
			})
		}

		if req.TestResult != nil && !req.TestResult.Success {
			actions = append(actions, events.ReviewNextAction{
				Action:   events.ControlDecisionIterate,
				Target:   "tests",
				Details:  "Fix failing tests",
				Priority: "high",
			})
		}

	case events.ControlDecisionAbort:
		actions = append(actions, events.ReviewNextAction{
			Action:   events.ControlDecisionRollback,
			Details:  "Roll back changes due to critical issues",
			Priority: "immediate",
		})

	case events.ControlDecisionManualReview:
		actions = append(actions, events.ReviewNextAction{
			Action:   events.ControlDecisionManualReview,
			Details:  "Await human review before proceeding",
			Priority: "immediate",
		})
	}

	return actions
}

func (e *ThresholdDecisionEngine) generateIterationGuidance(
	result *DecisionResult,
	req *DecisionRequest,
	thresholds events.ReviewThresholds,
) *events.IterationGuidance {
	guidance := &events.IterationGuidance{
		Priority:  []string{},
		MustFix:   []string{},
		ShouldFix: []string{},
		MayIgnore: []string{},
	}

	// Categorize blocking issues
	for _, issue := range result.BlockingIssues {
		switch issue.Severity {
		case events.ReviewIssueSeverityCritical:
			guidance.MustFix = append(guidance.MustFix, fmt.Sprintf("[%s] %s: %s", issue.Severity, issue.FilePath, issue.Title))
			guidance.Priority = append(guidance.Priority, issue.ID)
		case events.ReviewIssueSeverityHigh:
			guidance.ShouldFix = append(guidance.ShouldFix, fmt.Sprintf("[%s] %s: %s", issue.Severity, issue.FilePath, issue.Title))
		default:
			guidance.MayIgnore = append(guidance.MayIgnore, fmt.Sprintf("[%s] %s: %s", issue.Severity, issue.FilePath, issue.Title))
		}
	}

	// Add test failures to must-fix
	if req.TestResult != nil && !req.TestResult.Success {
		guidance.MustFix = append(guidance.MustFix, "Fix failing tests")
		guidance.Priority = append([]string{"tests"}, guidance.Priority...)
	}

	// Add context
	remainingIterations := thresholds.MaxIterations - req.IterationNumber
	guidance.Context = fmt.Sprintf("Iteration %d of %d. %d iterations remaining.",
		req.IterationNumber+1, thresholds.MaxIterations, remainingIterations)

	return guidance
}

func (e *ThresholdDecisionEngine) createAbortResult(
	result *DecisionResult,
	reason events.AbortReason,
	details string,
) *DecisionResult {
	result.Decision = events.ControlDecisionAbort
	result.Rationale = fmt.Sprintf("Abort: %s - %s", reason, details)
	result.RiskAssessment = events.RiskAssessment{
		OverallRiskScore:        100,
		RiskLevel:               events.RiskLevelCritical,
		AcceptableForProduction: false,
	}
	result.NextActions = []events.ReviewNextAction{
		{
			Action:   events.ControlDecisionRollback,
			Details:  details,
			Priority: "immediate",
		},
	}
	return result
}
