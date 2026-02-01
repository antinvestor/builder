package review

import (
	"context"
	"encoding/json"
	"fmt"

	appconfig "github.com/antinvestor/builder/apps/reviewer/config"
	"github.com/antinvestor/builder/internal/events"
)

// Language constants used for detection.
const (
	langGo         = "go"
	langPython     = "python"
	langJavaScript = "javascript"
	langTypeScript = "typescript"
	langJava       = "java"
	langRust       = "rust"
	langUnknown    = "unknown"
)

// RequestHandler handles incoming review requests.
type RequestHandler struct {
	cfg                  *appconfig.ReviewerConfig
	securityAnalyzer     SecurityAnalyzer
	architectureAnalyzer ArchitectureAnalyzer
	decisionEngine       DecisionEngine
	killSwitchService    KillSwitchService
	eventsMan            EventsEmitter
}

// NewRequestHandler creates a new review request handler.
func NewRequestHandler(
	cfg *appconfig.ReviewerConfig,
	securityAnalyzer SecurityAnalyzer,
	architectureAnalyzer ArchitectureAnalyzer,
	decisionEngine DecisionEngine,
	killSwitchService KillSwitchService,
	eventsMan EventsEmitter,
) *RequestHandler {
	return &RequestHandler{
		cfg:                  cfg,
		securityAnalyzer:     securityAnalyzer,
		architectureAnalyzer: architectureAnalyzer,
		decisionEngine:       decisionEngine,
		killSwitchService:    killSwitchService,
		eventsMan:            eventsMan,
	}
}

// Handle processes incoming review request messages.
func (h *RequestHandler) Handle(
	ctx context.Context,
	_ map[string]string,
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
			FilePath:     ref.FilePath,
			Action:       events.FileAction(ref.ChangeType),
			DiffContent:  ref.DiffContent,
			LinesAdded:   ref.LinesAdded,
			LinesRemoved: ref.LinesRemoved,
		}
	}
	return patches
}

func (h *RequestHandler) detectLanguage(patches []events.Patch) string {
	// Simple language detection from file extensions.
	for _, patch := range patches {
		ext := getFileExtension(patch.FilePath)
		switch ext {
		case ".go":
			return langGo
		case ".py":
			return langPython
		case ".js", ".jsx":
			return langJavaScript
		case ".ts", ".tsx":
			return langTypeScript
		case ".java":
			return langJava
		case ".rs":
			return langRust
		}
	}
	return langUnknown
}

func (h *RequestHandler) getIterationNumber(request *events.ComprehensiveReviewRequestedPayload) int {
	if request.Context != nil {
		return request.Context.IterationNumber
	}
	return 0
}

func (h *RequestHandler) emitAbort(ctx context.Context, executionID events.ExecutionID, reason string) error {
	return h.eventsMan.Emit(ctx, "feature.review.abort", &events.FeatureAbortRequestedPayload{
		ExecutionID:      executionID,
		AbortReason:      events.AbortReasonKillSwitch,
		AbortDetails:     reason,
		RollbackRequired: true,
	})
}

func (h *RequestHandler) emitDecision(
	ctx context.Context,
	executionID events.ExecutionID,
	decision *DecisionResult,
) error {
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
