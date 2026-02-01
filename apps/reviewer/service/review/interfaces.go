package review

import (
	"context"

	appconfig "github.com/antinvestor/builder/apps/reviewer/config"
	"github.com/antinvestor/builder/internal/events"
)

// Risk score constants.
const (
	perfectScore    = 100
	riskScoreMedium = 50
)

// =============================================================================
// Interfaces
// =============================================================================

// SecurityAnalyzer analyzes code for security issues.
type SecurityAnalyzer interface {
	Analyze(ctx context.Context, req *SecurityAnalysisRequest) (*events.SecurityAssessment, error)
}

// ArchitectureAnalyzer analyzes code for architectural issues.
type ArchitectureAnalyzer interface {
	Analyze(ctx context.Context, req *ArchitectureAnalysisRequest) (*events.ArchitectureAssessment, error)
}

// DecisionEngine makes control decisions.
type DecisionEngine interface {
	MakeDecision(ctx context.Context, req *DecisionRequest) (*DecisionResult, error)
}

// KillSwitchService manages kill switch operations.
type KillSwitchService interface {
	IsActive(
		ctx context.Context,
		executionID events.ExecutionID,
		repositoryID string,
	) (bool, events.KillSwitchReason, events.KillSwitchScope)
	GetStatus(ctx context.Context) (*events.KillSwitchStatusPayload, error)
	ActivateGlobal(
		ctx context.Context,
		reason events.KillSwitchReason,
		activatedBy, details string,
	) error
	DeactivateGlobal(ctx context.Context, deactivatedBy, reason string) error
}

// EventsEmitter emits events.
type EventsEmitter interface {
	Emit(ctx context.Context, eventName string, payload any) error
}

// =============================================================================
// Request/Response Types
// =============================================================================

// SecurityAnalysisRequest contains data for security analysis.
type SecurityAnalysisRequest struct {
	Patches      []events.Patch
	FileContents map[string]string
	Language     string
}

// ArchitectureAnalysisRequest contains data for architecture analysis.
type ArchitectureAnalysisRequest struct {
	Patches          []events.Patch
	FileContents     map[string]string
	BaselineContents map[string]string
	Language         string
}

// DecisionRequest contains data for making a control decision.
type DecisionRequest struct {
	ExecutionID            events.ExecutionID
	ReviewPhase            events.ReviewPhase
	SecurityAssessment     *events.SecurityAssessment
	ArchitectureAssessment *events.ArchitectureAssessment
	TestResult             *events.TestResult
	IterationNumber        int
	Thresholds             events.ReviewThresholds
	KillSwitchActive       bool
}

// DecisionResult contains the decision outcome.
type DecisionResult struct {
	Decision          events.ControlDecision
	RiskAssessment    events.RiskAssessment
	BlockingIssues    []events.ReviewIssue
	Rationale         string
	NextActions       []events.ReviewNextAction
	Warnings          []string
	IterationGuidance *events.IterationGuidance
}

// =============================================================================
// Stub Implementations (Only for components not yet implemented)
// =============================================================================

// Note: PatternSecurityAnalyzer is implemented in security_analyzer.go
// Note: PatternArchitectureAnalyzer is implemented in architecture_analyzer.go

// ConservativeDecisionEngine makes conservative control decisions.
type ConservativeDecisionEngine struct {
	cfg *appconfig.ReviewerConfig
}

// NewConservativeDecisionEngine creates a new decision engine.
func NewConservativeDecisionEngine(cfg *appconfig.ReviewerConfig) *ConservativeDecisionEngine {
	return &ConservativeDecisionEngine{cfg: cfg}
}

// MakeDecision makes a control decision.
func (e *ConservativeDecisionEngine) MakeDecision(
	_ context.Context,
	req *DecisionRequest,
) (*DecisionResult, error) {
	// Stub implementation - approves if no issues
	passesSecurityReview := !req.SecurityAssessment.RequiresSecurityReview
	passesArchitectureReview := !req.ArchitectureAssessment.RequiresArchitectureReview

	if passesSecurityReview && passesArchitectureReview {
		return &DecisionResult{
			Decision: events.ControlDecisionApprove,
			RiskAssessment: events.RiskAssessment{
				OverallRiskScore:        0,
				RiskLevel:               events.RiskLevelLow,
				AcceptableForProduction: true,
			},
			Rationale:   "All checks passed",
			NextActions: []events.ReviewNextAction{},
		}, nil
	}

	return &DecisionResult{
		Decision: events.ControlDecisionIterate,
		RiskAssessment: events.RiskAssessment{
			OverallRiskScore:        riskScoreMedium,
			RiskLevel:               events.RiskLevelMedium,
			AcceptableForProduction: false,
		},
		Rationale: "Issues detected, iteration required",
	}, nil
}

// DefaultKillSwitchService is the default kill switch implementation.
type DefaultKillSwitchService struct {
	cfg       *appconfig.ReviewerConfig
	eventsMan EventsEmitter
}

// NewDefaultKillSwitchService creates a new kill switch service.
func NewDefaultKillSwitchService(cfg *appconfig.ReviewerConfig, eventsMan EventsEmitter) *DefaultKillSwitchService {
	return &DefaultKillSwitchService{
		cfg:       cfg,
		eventsMan: eventsMan,
	}
}

// IsActive checks if kill switch is active.
func (s *DefaultKillSwitchService) IsActive(
	_ context.Context,
	_ events.ExecutionID,
	_ string,
) (bool, events.KillSwitchReason, events.KillSwitchScope) {
	// Stub - kill switch not active
	return false, "", ""
}

// GetStatus returns kill switch status.
func (s *DefaultKillSwitchService) GetStatus(_ context.Context) (*events.KillSwitchStatusPayload, error) {
	return &events.KillSwitchStatusPayload{
		GlobalActive: false,
	}, nil
}

// ActivateGlobal activates global kill switch.
func (s *DefaultKillSwitchService) ActivateGlobal(
	_ context.Context,
	_ events.KillSwitchReason,
	_, _ string,
) error {
	return nil
}

// DeactivateGlobal deactivates global kill switch.
func (s *DefaultKillSwitchService) DeactivateGlobal(_ context.Context, _, _ string) error {
	return nil
}
