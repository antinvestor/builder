package events

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pitabwire/util"

	appconfig "github.com/antinvestor/builder/apps/worker/config"
	"github.com/antinvestor/builder/apps/worker/service/repository"
	"github.com/antinvestor/builder/internal/events"
)

// Test execution timeout constant.
const defaultTestTimeoutSeconds = 300

// =============================================================================
// Test Execution Request Handler
// =============================================================================

// TestExecutionRequestEvent sends test execution requests to the executor.
type TestExecutionRequestEvent struct {
	cfg       *appconfig.WorkerConfig
	queueMan  QueueManager
	eventsMan Emitter
}

// NewTestExecutionRequestEvent creates a new test execution request event handler.
func NewTestExecutionRequestEvent(
	cfg *appconfig.WorkerConfig,
	queueMan QueueManager,
	eventsMan Emitter,
) *TestExecutionRequestEvent {
	return &TestExecutionRequestEvent{
		cfg:       cfg,
		queueMan:  queueMan,
		eventsMan: eventsMan,
	}
}

// Name returns the event name.
func (h *TestExecutionRequestEvent) Name() string {
	return string(events.PatchGenerationCompleted)
}

// PayloadType returns the expected payload type.
func (h *TestExecutionRequestEvent) PayloadType() any {
	return &events.PatchGenerationCompletedPayload{}
}

// Validate validates the payload.
func (h *TestExecutionRequestEvent) Validate(_ context.Context, _ any) error {
	return nil
}

// Execute sends test execution request to executor queue.
func (h *TestExecutionRequestEvent) Execute(ctx context.Context, payload any) error {
	log := util.Log(ctx)
	request, ok := payload.(*events.PatchGenerationCompletedPayload)
	if !ok {
		return errors.New("invalid payload type: expected *PatchGenerationCompletedPayload")
	}

	log.Info("sending test execution request",
		"execution_id", request.ExecutionID.String(),
		"commit_sha", request.FinalCommitSHA,
	)

	// Emit test execution started event
	// TODO: Make TestCommand, TimeoutSeconds, and Language configurable via WorkerConfig
	if err := h.eventsMan.Emit(ctx, string(events.TestExecutionStarted), &events.TestExecutionStartedPayload{
		TestCommand:    "go test ./...",
		TimeoutSeconds: defaultTestTimeoutSeconds,
		StartedAt:      time.Now(),
	}); err != nil {
		return err
	}

	// Publish test execution request to executor queue
	// TODO: Detect language from workspace or config instead of hardcoding
	return h.queueMan.Publish(ctx, h.cfg.QueueExecutionRequestName, &events.TestExecutionRequestedPayload{
		ExecutionID:   request.ExecutionID,
		Language:      "go",
		TestFiles:     []string{},
		WorkspacePath: "", // Executor will use its own workspace
	})
}

// =============================================================================
// Review Request Handler
// =============================================================================

// ReviewRequestEvent sends review requests to the reviewer service.
type ReviewRequestEvent struct {
	cfg       *appconfig.WorkerConfig
	queueMan  QueueManager
	eventsMan Emitter
}

// NewReviewRequestEvent creates a new review request event handler.
func NewReviewRequestEvent(
	cfg *appconfig.WorkerConfig,
	queueMan QueueManager,
	eventsMan Emitter,
) *ReviewRequestEvent {
	return &ReviewRequestEvent{
		cfg:       cfg,
		queueMan:  queueMan,
		eventsMan: eventsMan,
	}
}

// Name returns the event name.
func (h *ReviewRequestEvent) Name() string {
	return string(events.TestExecutionCompleted)
}

// PayloadType returns the expected payload type.
func (h *ReviewRequestEvent) PayloadType() any {
	return &events.TestExecutionCompletedPayload{}
}

// Validate validates the payload.
func (h *ReviewRequestEvent) Validate(_ context.Context, _ any) error {
	return nil
}

// Execute sends review request if tests passed.
func (h *ReviewRequestEvent) Execute(ctx context.Context, payload any) error {
	log := util.Log(ctx)
	request, ok := payload.(*events.TestExecutionCompletedPayload)
	if !ok {
		return errors.New("invalid payload type: expected *TestExecutionCompletedPayload")
	}

	// Only request review if tests passed
	if !request.Success {
		log.Info("tests failed, skipping review request",
			"execution_id", request.ExecutionID.String(),
		)
		// Emit iteration required event
		// TODO: IterationNumber should be tracked in execution state, not hardcoded
		// TODO: MaxIterationsRemaining should be calculated from tracked iteration count
		return h.eventsMan.Emit(ctx, string(events.IterationRequired), &events.IterationRequiredPayload{
			IterationNumber: 1, // TODO: Track and increment across iterations
			Reason:          events.IterationReasonTestsFailed,
			Issues: []events.IterationIssue{
				{
					Type:        "test_failure",
					Description: "Tests failed and need to be fixed",
					Severity:    "high",
				},
			},
			ProposedActions:        []string{"Fix failing tests", "Check test output for errors"},
			MaxIterationsRemaining: h.cfg.ReviewThresholds.MaxIterations - 1, // TODO: Calculate from tracked count
			RequiredAt:             time.Now(),
		})
	}

	log.Info("sending review request",
		"execution_id", request.ExecutionID.String(),
	)

	// Emit review started event
	if err := h.eventsMan.Emit(ctx, string(events.ReviewStarted), &events.ReviewStartedPayload{
		FilesToReview: []string{},
		ReviewTypes:   []events.ReviewType{events.ReviewTypeSecurity, events.ReviewTypeArchitecture},
		StartedAt:     time.Now(),
	}); err != nil {
		return err
	}

	// Publish review request to reviewer queue
	return h.queueMan.Publish(ctx, h.cfg.QueueReviewRequestName, &events.ComprehensiveReviewRequestedPayload{
		ExecutionID: request.ExecutionID,
		ReviewPhase: events.ReviewPhasePostImplementation,
		TestResults: request.Result,
		RequestedAt: time.Now(),
	})
}

// =============================================================================
// Review Result Handler
// =============================================================================

// ReviewResultEvent handles review results from the reviewer service.
type ReviewResultEvent struct {
	cfg         *appconfig.WorkerConfig
	repoService *repository.Service
	bamlClient  BAMLClient
	queueMan    QueueManager
	eventsMan   Emitter
}

// NewReviewResultEvent creates a new review result event handler.
func NewReviewResultEvent(
	cfg *appconfig.WorkerConfig,
	repoService *repository.Service,
	bamlClient BAMLClient,
	queueMan QueueManager,
	eventsMan Emitter,
) *ReviewResultEvent {
	return &ReviewResultEvent{
		cfg:         cfg,
		repoService: repoService,
		bamlClient:  bamlClient,
		queueMan:    queueMan,
		eventsMan:   eventsMan,
	}
}

// Name returns the event name.
func (h *ReviewResultEvent) Name() string {
	return string(events.ReviewCompleted)
}

// PayloadType returns the expected payload type.
func (h *ReviewResultEvent) PayloadType() any {
	return &events.ComprehensiveReviewCompletedPayload{}
}

// Validate validates the payload.
func (h *ReviewResultEvent) Validate(_ context.Context, _ any) error {
	return nil
}

// Execute processes review results and routes to appropriate handler.
func (h *ReviewResultEvent) Execute(ctx context.Context, payload any) error {
	log := util.Log(ctx)
	request, ok := payload.(*events.ComprehensiveReviewCompletedPayload)
	if !ok {
		return errors.New("invalid payload type: expected *ComprehensiveReviewCompletedPayload")
	}

	log.Info("processing review result",
		"execution_id", request.ExecutionID.String(),
		"decision", request.Decision,
	)

	switch request.Decision {
	case events.ControlDecisionApprove, events.ControlDecisionApproveWithWarnings:
		// Approved - proceed to delivery
		return h.handleApproval(ctx, request)

	case events.ControlDecisionIterate:
		// Iteration required - emit iteration event
		return h.handleIteration(ctx, request)

	case events.ControlDecisionAbort, events.ControlDecisionRollback:
		// Abort - emit failure event
		return h.handleAbort(ctx, request)

	case events.ControlDecisionManualReview:
		// Manual review required - pause and wait
		log.Info("manual review required, pausing execution",
			"execution_id", request.ExecutionID.String(),
		)
		return nil

	case events.ControlDecisionMarkComplete:
		// Mark as complete - proceed to delivery
		log.Info("marking feature as complete",
			"execution_id", request.ExecutionID.String(),
		)
		return h.handleApproval(ctx, request)

	default:
		return fmt.Errorf("unknown decision: %s", request.Decision)
	}
}

func (h *ReviewResultEvent) handleApproval(
	ctx context.Context,
	request *events.ComprehensiveReviewCompletedPayload,
) error {
	log := util.Log(ctx)
	branchName := fmt.Sprintf("feature/%s", request.ExecutionID.String())

	log.Info("review approved, proceeding to delivery",
		"execution_id", request.ExecutionID.String(),
		"branch_name", branchName,
	)

	// Emit git push started
	if err := h.eventsMan.Emit(ctx, string(events.GitPushStarted), &events.GitPushStartedPayload{
		BranchName: branchName,
		RemoteName: "origin",
		StartedAt:  time.Now(),
	}); err != nil {
		return err
	}

	// Push the feature branch using the correct method
	startTime := time.Now()
	pushErr := h.repoService.PushBranch(ctx, request.ExecutionID, branchName)
	if pushErr != nil {
		if emitErr := h.eventsMan.Emit(ctx, string(events.GitPushFailed), &events.GitPushFailedPayload{
			BranchName:   branchName,
			ErrorCode:    events.GitPushErrorNetwork,
			ErrorMessage: pushErr.Error(),
			Retryable:    true,
			FailedAt:     time.Now(),
		}); emitErr != nil {
			log.WithError(emitErr).Error("failed to emit git push failed event")
		}
		return pushErr
	}
	durationMS := time.Since(startTime).Milliseconds()

	// Emit git push completed
	// TODO: repoService.PushBranch should return the remote HEAD commit SHA
	if emitErr := h.eventsMan.Emit(ctx, string(events.GitPushCompleted), &events.GitPushCompletedPayload{
		BranchName:      branchName,
		RemoteRef:       fmt.Sprintf("refs/heads/%s", branchName),
		RemoteCommitSHA: "", // TODO: Populate from repoService.PushBranch return value
		CommitsPushed:   1,
		DurationMS:      durationMS,
		CompletedAt:     time.Now(),
	}); emitErr != nil {
		return emitErr
	}

	// Emit feature delivered
	// TODO: HeadCommitSHA should be the commit SHA returned from the push
	return h.eventsMan.Emit(ctx, string(events.FeatureDelivered), &events.FeatureDeliveredPayload{
		BranchName:    branchName,
		RemoteRef:     fmt.Sprintf("refs/heads/%s", branchName),
		HeadCommitSHA: "", // TODO: Populate from repoService.PushBranch return value
		Artifacts:     []events.ArtifactReference{},
		Summary: events.DeliverySummary{
			Title:       "Feature delivered successfully",
			Description: request.DecisionRationale,
			Execution: events.ExecutionSummary{
				StepsCompleted: 1,
			},
		},
	})
}

func (h *ReviewResultEvent) handleIteration(
	ctx context.Context,
	request *events.ComprehensiveReviewCompletedPayload,
) error {
	log := util.Log(ctx)
	log.Info("iteration required",
		"execution_id", request.ExecutionID.String(),
		"blocking_issues", len(request.BlockingIssues),
	)

	// TODO: IterationNumber should be retrieved from execution state and incremented
	return h.eventsMan.Emit(ctx, string(events.IterationRequired), &events.FeatureIterationRequestedPayload{
		ExecutionID:     request.ExecutionID,
		ReviewID:        request.ReviewID,
		IterationNumber: 1, // TODO: Track and increment iteration count in execution state
		Issues:          request.BlockingIssues,
		IterationGuidance: &events.IterationGuidance{
			MustFix: extractIssueTitles(request.BlockingIssues),
		},
		RequestedAt: time.Now(),
	})
}

func (h *ReviewResultEvent) handleAbort(
	ctx context.Context,
	request *events.ComprehensiveReviewCompletedPayload,
) error {
	log := util.Log(ctx)
	log.Warn("aborting feature execution",
		"execution_id", request.ExecutionID.String(),
		"reason", request.DecisionRationale,
	)

	return h.eventsMan.Emit(ctx, string(events.FeatureExecutionFailed), &events.FeatureExecutionFailedPayload{
		Classification: events.FailureClassification{
			Type:           events.FailureTypeSemantic,
			Severity:       events.FailureSeverityError,
			Retryable:      false,
			UserActionable: true,
		},
		FailedPhase:  events.ExecutionPhaseVerification,
		ErrorCode:    "review_abort",
		ErrorMessage: request.DecisionRationale,
		Recovery: events.RecoveryInfo{
			CanRetry:  false,
			CanResume: false,
		},
	})
}

// =============================================================================
// Iteration Handler
// =============================================================================

// IterationEvent handles iteration requests.
type IterationEvent struct {
	cfg        *appconfig.WorkerConfig
	bamlClient BAMLClient
	eventsMan  Emitter
}

// NewIterationEvent creates a new iteration event handler.
func NewIterationEvent(
	cfg *appconfig.WorkerConfig,
	bamlClient BAMLClient,
	eventsMan Emitter,
) *IterationEvent {
	return &IterationEvent{
		cfg:        cfg,
		bamlClient: bamlClient,
		eventsMan:  eventsMan,
	}
}

// Name returns the event name.
func (h *IterationEvent) Name() string {
	return string(events.IterationRequired)
}

// PayloadType returns the expected payload type.
func (h *IterationEvent) PayloadType() any {
	return &events.FeatureIterationRequestedPayload{}
}

// Validate validates the payload.
func (h *IterationEvent) Validate(_ context.Context, _ any) error {
	return nil
}

// Execute processes iteration request.
// Handles both *FeatureIterationRequestedPayload (from review) and *IterationRequiredPayload (from test failures).
func (h *IterationEvent) Execute(ctx context.Context, payload any) error {
	log := util.Log(ctx)

	// Handle both payload types since IterationRequired can come from different sources
	var executionID events.ExecutionID
	var iterationNumber int
	var issues []events.ReviewIssue
	var reason events.IterationReason

	switch p := payload.(type) {
	case *events.FeatureIterationRequestedPayload:
		executionID = p.ExecutionID
		iterationNumber = p.IterationNumber
		issues = p.Issues
		reason = events.IterationReasonReviewRejected

	case *events.IterationRequiredPayload:
		// Handle iteration from test failures
		// TODO: Need to pass execution ID and track iteration count properly
		iterationNumber = p.IterationNumber
		reason = p.Reason
		// Convert IterationIssues to ReviewIssues for consistent handling
		for _, issue := range p.Issues {
			issues = append(issues, events.ReviewIssue{
				Type:        events.ReviewIssueType(issue.Type),
				Description: issue.Description,
				Severity:    events.ReviewIssueSeverity(issue.Severity),
			})
		}

	default:
		return errors.New(
			"invalid payload type: expected *FeatureIterationRequestedPayload or *IterationRequiredPayload",
		)
	}

	log.Info("starting iteration",
		"execution_id", executionID.String(),
		"iteration_number", iterationNumber,
		"issues", len(issues),
	)

	// Check max iterations
	maxIterations := h.cfg.ReviewThresholds.MaxIterations
	if maxIterations == 0 {
		maxIterations = 3
	}
	if iterationNumber >= maxIterations {
		log.Warn("max iterations reached, aborting",
			"execution_id", executionID.String(),
			"max_iterations", maxIterations,
		)
		return h.eventsMan.Emit(ctx, string(events.FeatureExecutionFailed), &events.FeatureExecutionFailedPayload{
			Classification: events.FailureClassification{
				Type:           events.FailureTypeSemantic,
				Severity:       events.FailureSeverityError,
				Retryable:      false,
				UserActionable: true,
			},
			FailedPhase:  events.ExecutionPhaseGeneration,
			ErrorCode:    "max_iterations_exceeded",
			ErrorMessage: fmt.Sprintf("Maximum iterations (%d) exceeded", maxIterations),
			Recovery: events.RecoveryInfo{
				CanRetry:  false,
				CanResume: false,
			},
		})
	}

	// Convert ReviewIssues to IterationIssues for the IterationStartedPayload
	targetIssues := convertToIterationIssues(issues)

	// Emit iteration started
	if err := h.eventsMan.Emit(ctx, string(events.IterationStarted), &events.IterationStartedPayload{
		IterationNumber: iterationNumber,
		Reason:          reason,
		TargetIssues:    targetIssues,
		Strategy: events.IterationStrategy{
			Approach: events.IterationApproachFix,
		},
		StartedAt: time.Now(),
	}); err != nil {
		return err
	}

	// Build feedback from review issues
	feedback := buildFeedbackFromReviewIssues(issues)

	// Generate new patches with feedback
	resp, err := h.bamlClient.GeneratePatch(ctx, &GeneratePatchRequest{
		ExecutionID:        executionID,
		IterationNumber:    iterationNumber,
		FeedbackFromReview: feedback,
	})
	if err != nil {
		return err
	}

	// Emit patch generation completed to trigger test execution again
	// TODO: After applying patches in repo, get the actual commit SHA and use it here.
	// The bamlClient.GeneratePatch only generates patches; the repo service applies them and creates the commit.
	return h.eventsMan.Emit(ctx, string(events.PatchGenerationCompleted), &events.PatchGenerationCompletedPayload{
		ExecutionID:    executionID,
		TotalSteps:     1,
		StepsCompleted: 1,
		TotalLLMTokens: resp.TokensUsed,
		FinalCommitSHA: "", // TODO: Populate with actual commit SHA after applying patches
		CompletedAt:    time.Now(),
	})
}

// =============================================================================
// Delivery Handler (for direct delivery without full review)
// =============================================================================

// DeliveryEvent handles feature delivery.
type DeliveryEvent struct {
	cfg         *appconfig.WorkerConfig
	repoService *repository.Service
	queueMan    QueueManager
	eventsMan   Emitter
}

// NewDeliveryEvent creates a new delivery event handler.
func NewDeliveryEvent(
	cfg *appconfig.WorkerConfig,
	repoService *repository.Service,
	queueMan QueueManager,
	eventsMan Emitter,
) *DeliveryEvent {
	return &DeliveryEvent{
		cfg:         cfg,
		repoService: repoService,
		queueMan:    queueMan,
		eventsMan:   eventsMan,
	}
}

// Name returns the event name - listens for git push completed.
func (h *DeliveryEvent) Name() string {
	return string(events.GitPushCompleted)
}

// PayloadType returns the expected payload type.
func (h *DeliveryEvent) PayloadType() any {
	return &events.GitPushCompletedPayload{}
}

// Validate validates the payload.
func (h *DeliveryEvent) Validate(_ context.Context, _ any) error {
	return nil
}

// Execute marks execution as complete after successful push.
func (h *DeliveryEvent) Execute(ctx context.Context, payload any) error {
	log := util.Log(ctx)
	request, ok := payload.(*events.GitPushCompletedPayload)
	if !ok {
		return errors.New("invalid payload type: expected *GitPushCompletedPayload")
	}

	log.Info("delivery completed",
		"branch_name", request.BranchName,
		"commit_sha", request.RemoteCommitSHA,
	)

	// Emit feature execution completed
	return h.eventsMan.Emit(ctx, string(events.FeatureExecutionCompleted), &events.FeatureExecutionCompletedPayload{
		BranchName: request.BranchName,
		FinalCommit: events.CommitInfo{
			SHA: request.RemoteCommitSHA,
		},
		Summary: events.ExecutionSummary{
			StepsCompleted: 1,
		},
	})
}

// =============================================================================
// Helper Functions
// =============================================================================

func extractIssueTitles(issues []events.ReviewIssue) []string {
	titles := make([]string, 0, len(issues))
	for _, issue := range issues {
		titles = append(titles, fmt.Sprintf("[%s] %s", issue.Severity, issue.Title))
	}
	return titles
}

func buildFeedbackFromReviewIssues(issues []events.ReviewIssue) string {
	if len(issues) == 0 {
		return ""
	}

	var feedback strings.Builder
	feedback.WriteString("Please fix the following issues:\n\n")
	for i, issue := range issues {
		feedback.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, issue.Severity, issue.Title))
		if issue.Description != "" {
			feedback.WriteString(fmt.Sprintf("   %s\n", issue.Description))
		}
		if issue.Suggestion != "" {
			feedback.WriteString(fmt.Sprintf("   Suggestion: %s\n", issue.Suggestion))
		}
		feedback.WriteString("\n")
	}
	return feedback.String()
}

// convertToIterationIssues converts ReviewIssues to IterationIssues.
func convertToIterationIssues(issues []events.ReviewIssue) []events.IterationIssue {
	result := make([]events.IterationIssue, 0, len(issues))
	for _, issue := range issues {
		result = append(result, events.IterationIssue{
			Type:        string(issue.Type),
			FilePath:    issue.FilePath,
			LineNumber:  issue.LineStart,
			Description: issue.Description,
			Severity:    string(issue.Severity),
		})
	}
	return result
}
