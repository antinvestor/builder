package review

import (
	"context"
	"encoding/json"
	"fmt"

	appconfig "service-feature/apps/reviewer/config"
	"service-feature/internal/events"
)

// ReviewRequestHandler handles incoming review requests.
type ReviewRequestHandler struct {
	cfg                  *appconfig.ReviewerConfig
	securityAnalyzer     SecurityAnalyzer
	architectureAnalyzer ArchitectureAnalyzer
	decisionEngine       DecisionEngine
	killSwitchService    KillSwitchService
	eventsMan            EventsEmitter
}

// NewReviewRequestHandler creates a new review request handler.
func NewReviewRequestHandler(
	cfg *appconfig.ReviewerConfig,
	securityAnalyzer SecurityAnalyzer,
	architectureAnalyzer ArchitectureAnalyzer,
	decisionEngine DecisionEngine,
	killSwitchService KillSwitchService,
	eventsMan EventsEmitter,
) *ReviewRequestHandler {
	return &ReviewRequestHandler{
		cfg:                  cfg,
		securityAnalyzer:     securityAnalyzer,
		architectureAnalyzer: architectureAnalyzer,
		decisionEngine:       decisionEngine,
		killSwitchService:    killSwitchService,
		eventsMan:            eventsMan,
	}
}

// Handle processes incoming review request messages.
func (h *ReviewRequestHandler) Handle(
	ctx context.Context,
	headers map[string]string,
	payload []byte,
) error {
	var request events.ComprehensiveReviewRequestedPayload
	if err := json.Unmarshal(payload, &request); err != nil {
		return fmt.Errorf("unmarshal review request: %w", err)
	}

	// Check kill switch first
	active, reason, _ := h.killSwitchService.IsActive(ctx, request.ExecutionID, "")
	if active {
		return h.emitAbort(ctx, request.ExecutionID, string(reason))
	}

	// Convert PatchReferences to Patches for analysis
	patches := convertPatchReferences(request.Patches)

	// Run security analysis
	securityAssessment, err := h.securityAnalyzer.Analyze(ctx, &SecurityAnalysisRequest{
		Patches:  patches,
		Language: h.detectLanguage(patches),
	})
	if err != nil {
		return fmt.Errorf("security analysis failed: %w", err)
	}

	// Run architecture analysis
	architectureAssessment, err := h.architectureAnalyzer.Analyze(ctx, &ArchitectureAnalysisRequest{
		Patches:  patches,
		Language: h.detectLanguage(patches),
	})
	if err != nil {
		return fmt.Errorf("architecture analysis failed: %w", err)
	}

	// Make decision
	thresholds := h.cfg.GetReviewThresholds()
	decision, err := h.decisionEngine.MakeDecision(ctx, &DecisionRequest{
		ExecutionID:            request.ExecutionID,
		ReviewPhase:            request.ReviewPhase,
		SecurityAssessment:     securityAssessment,
		ArchitectureAssessment: architectureAssessment,
		TestResult:             request.TestResults,
		IterationNumber:        h.getIterationNumber(&request),
		Thresholds:             thresholds,
	})
	if err != nil {
		return fmt.Errorf("decision making failed: %w", err)
	}

	// Emit result event
	return h.emitDecision(ctx, request.ExecutionID, decision)
}

func convertPatchReferences(refs []events.PatchReference) []events.Patch {
	patches := make([]events.Patch, len(refs))
	for i, ref := range refs {
		patches[i] = events.Patch{
			FilePath:    ref.FilePath,
			Action:      events.FileAction(ref.ChangeType),
			DiffContent: ref.DiffContent,
			LinesAdded:  ref.LinesAdded,
			LinesRemoved: ref.LinesRemoved,
		}
	}
	return patches
}

func (h *ReviewRequestHandler) detectLanguage(patches []events.Patch) string {
	// Simple language detection from file extensions
	for _, patch := range patches {
		ext := getFileExtension(patch.FilePath)
		switch ext {
		case ".go":
			return "go"
		case ".py":
			return "python"
		case ".js", ".jsx":
			return "javascript"
		case ".ts", ".tsx":
			return "typescript"
		case ".java":
			return "java"
		case ".rs":
			return "rust"
		}
	}
	return "unknown"
}

func (h *ReviewRequestHandler) getIterationNumber(request *events.ComprehensiveReviewRequestedPayload) int {
	if request.Context != nil {
		return request.Context.IterationNumber
	}
	return 0
}

func (h *ReviewRequestHandler) emitAbort(ctx context.Context, executionID events.ExecutionID, reason string) error {
	return h.eventsMan.Emit(ctx, "feature.review.abort", &events.FeatureAbortRequestedPayload{
		ExecutionID:      executionID,
		AbortReason:      events.AbortReasonKillSwitch,
		AbortDetails:     reason,
		RollbackRequired: true,
	})
}

func (h *ReviewRequestHandler) emitDecision(ctx context.Context, executionID events.ExecutionID, decision *DecisionResult) error {
	eventName := "feature.review.completed"
	return h.eventsMan.Emit(ctx, eventName, &events.ComprehensiveReviewCompletedPayload{
		ExecutionID:       executionID,
		Decision:          decision.Decision,
		RiskAssessment:    decision.RiskAssessment,
		BlockingIssues:    decision.BlockingIssues,
		DecisionRationale: decision.Rationale,
		NextActions:       decision.NextActions,
	})
}

func getFileExtension(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' || path[i] == '\\' {
			break
		}
	}
	return ""
}
