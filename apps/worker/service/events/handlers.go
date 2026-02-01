package events

import (
	"context"
	"fmt"
	"time"

	appconfig "github.com/antinvestor/builder/apps/worker/config"
	"github.com/antinvestor/builder/apps/worker/service/repository"
	"github.com/antinvestor/builder/internal/events"
)

// EventsEmitter emits events.
type EventsEmitter interface {
	Emit(ctx context.Context, eventName string, payload any) error
}

// QueueManager manages queue publishing.
type QueueManager interface {
	Publish(ctx context.Context, queueName string, payload any, headers ...map[string]string) error
}

// =============================================================================
// Repository Checkout Event Handler
// =============================================================================

// RepositoryCheckoutEvent handles repository checkout operations.
type RepositoryCheckoutEvent struct {
	cfg         *appconfig.WorkerConfig
	repoService *repository.RepositoryService
	eventsMan   EventsEmitter
}

// NewRepositoryCheckoutEvent creates a new repository checkout event handler.
func NewRepositoryCheckoutEvent(
	cfg *appconfig.WorkerConfig,
	repoService *repository.RepositoryService,
	eventsMan EventsEmitter,
) *RepositoryCheckoutEvent {
	return &RepositoryCheckoutEvent{
		cfg:         cfg,
		repoService: repoService,
		eventsMan:   eventsMan,
	}
}

// Name returns the event name.
func (h *RepositoryCheckoutEvent) Name() string {
	return string(events.FeatureExecutionInitialized)
}

// PayloadType returns the expected payload type.
func (h *RepositoryCheckoutEvent) PayloadType() any {
	return &events.FeatureExecutionInitializedPayload{}
}

// Validate validates the payload.
func (h *RepositoryCheckoutEvent) Validate(ctx context.Context, payload any) error {
	return nil
}

// Execute processes the repository checkout event.
func (h *RepositoryCheckoutEvent) Execute(ctx context.Context, payload any) error {
	request, ok := payload.(*events.FeatureExecutionInitializedPayload)
	if !ok {
		return fmt.Errorf("invalid payload type: expected *FeatureExecutionInitializedPayload")
	}

	// Generate execution ID for this feature build
	execID := events.NewExecutionID()

	// Emit checkout started
	if err := h.eventsMan.Emit(ctx, string(events.RepositoryCheckoutStarted), &events.RepositoryCheckoutStartedPayload{
		RemoteURL: request.Repository.RemoteURL,
		TargetRef: request.Repository.TargetBranch,
		StartedAt: time.Now(),
	}); err != nil {
		return err
	}

	// Perform checkout
	result, err := h.repoService.Checkout(ctx, &repository.CheckoutRequest{
		ExecutionID:   execID,
		RepositoryURL: request.Repository.RemoteURL,
		Branch:        request.Repository.TargetBranch,
		CommitSHA:     request.Repository.BaseCommitSHA,
	})
	if err != nil {
		h.eventsMan.Emit(ctx, string(events.RepositoryCheckoutFailed), &events.RepositoryCheckoutFailedPayload{
			ErrorCode:    events.CheckoutErrorNetwork,
			ErrorMessage: err.Error(),
			Retryable:    true,
			FailedAt:     time.Now(),
		})
		return err
	}

	// Emit completion with execution context for downstream handlers
	return h.eventsMan.Emit(ctx, string(events.RepositoryCheckoutCompleted), &events.RepositoryCheckoutCompletedPayload{
		ExecutionID:   execID,
		Specification: request.Spec,
		RepositoryURL: request.Repository.RemoteURL,
		WorkspacePath: result.WorkspacePath,
		HeadCommitSHA: result.CommitSHA,
		BranchName:    result.Branch,
		DurationMS:    result.CheckoutTimeMS,
		CompletedAt:   time.Now(),
	})
}

// =============================================================================
// Patch Generation Handler
// =============================================================================

// BAMLClient is the interface for BAML LLM operations.
type BAMLClient interface {
	GeneratePatch(ctx context.Context, req *GeneratePatchRequest) (*GeneratePatchResponse, error)
}

// GeneratePatchRequest contains the request for patch generation.
type GeneratePatchRequest struct {
	ExecutionID        events.ExecutionID
	Specification      events.FeatureSpecification
	WorkspacePath      string
	RepositoryContext  string
	PreviousPatches    []Patch
	IterationNumber    int
	FeedbackFromReview string
}

// GeneratePatchResponse contains the response from patch generation.
type GeneratePatchResponse struct {
	Patches       []Patch
	CommitMessage string
	TokensUsed    int
}

// Patch represents a code patch.
type Patch struct {
	FilePath   string
	OldContent string
	NewContent string
	Action     events.FileAction
}

// PatchGenerationEvent handles patch generation operations.
type PatchGenerationEvent struct {
	cfg         *appconfig.WorkerConfig
	bamlClient  BAMLClient
	repoService *repository.RepositoryService
	eventsMan   EventsEmitter
}

// NewPatchGenerationEvent creates a new patch generation event handler.
func NewPatchGenerationEvent(
	cfg *appconfig.WorkerConfig,
	bamlClient BAMLClient,
	repoService *repository.RepositoryService,
	eventsMan EventsEmitter,
) *PatchGenerationEvent {
	return &PatchGenerationEvent{
		cfg:         cfg,
		bamlClient:  bamlClient,
		repoService: repoService,
		eventsMan:   eventsMan,
	}
}

// Name returns the event name.
func (h *PatchGenerationEvent) Name() string {
	return string(events.RepositoryCheckoutCompleted)
}

// PayloadType returns the expected payload type.
func (h *PatchGenerationEvent) PayloadType() any {
	return &events.RepositoryCheckoutCompletedPayload{}
}

// Validate validates the payload.
func (h *PatchGenerationEvent) Validate(ctx context.Context, payload any) error {
	return nil
}

// Execute processes patch generation and completes the full pipeline.
func (h *PatchGenerationEvent) Execute(ctx context.Context, payload any) error {
	request, ok := payload.(*events.RepositoryCheckoutCompletedPayload)
	if !ok {
		return fmt.Errorf("invalid payload type: expected *RepositoryCheckoutCompletedPayload")
	}

	execID := request.ExecutionID

	// Emit patch generation started
	if err := h.eventsMan.Emit(ctx, string(events.PatchGenerationStarted), &events.PatchGenerationStartedPayload{
		TotalSteps: 1,
		StartedAt:  time.Now(),
	}); err != nil {
		return err
	}

	// Get project structure for context
	projectStructure, _ := h.repoService.GetProjectStructure(ctx, execID)

	// Generate patches using BAML/LLM
	resp, err := h.bamlClient.GeneratePatch(ctx, &GeneratePatchRequest{
		ExecutionID:      execID,
		Specification:    request.Specification,
		WorkspacePath:    request.WorkspacePath,
		RepositoryContext: projectStructure,
	})
	if err != nil {
		h.eventsMan.Emit(ctx, string(events.PatchGenerationFailed), &events.PatchGenerationFailedPayload{
			ErrorCode:    "llm_error",
			ErrorMessage: err.Error(),
			Retryable:    true,
			FailedAt:     time.Now(),
		})
		return fmt.Errorf("patch generation failed: %w", err)
	}

	// If no patches generated, emit failure
	if len(resp.Patches) == 0 {
		h.eventsMan.Emit(ctx, string(events.PatchGenerationFailed), &events.PatchGenerationFailedPayload{
			ErrorCode:    "no_patches",
			ErrorMessage: "LLM generated no patches",
			Retryable:    true,
			FailedAt:     time.Now(),
		})
		return fmt.Errorf("no patches generated")
	}

	// Create feature branch
	featureBranch := fmt.Sprintf("feature/%s", execID.Short())
	if err := h.repoService.CreateBranch(ctx, execID, featureBranch); err != nil {
		h.eventsMan.Emit(ctx, string(events.GitBranchCreationFailed), &events.GitBranchCreationFailedPayload{
			BranchName:   featureBranch,
			ErrorCode:    "branch_create_failed",
			ErrorMessage: err.Error(),
			FailedAt:     time.Now(),
		})
		return fmt.Errorf("create feature branch: %w", err)
	}

	// Emit branch created event
	h.eventsMan.Emit(ctx, string(events.GitBranchCreated), &events.GitBranchCreatedPayload{
		BranchName:    featureBranch,
		BaseBranch:    request.BranchName,
		BaseCommitSHA: request.HeadCommitSHA,
		CreatedAt:     time.Now(),
	})

	// Apply patches to workspace
	for _, patch := range resp.Patches {
		internalPatch := &events.Patch{
			FilePath:   patch.FilePath,
			Action:     patch.Action,
			NewContent: patch.NewContent,
		}
		if err := h.repoService.ApplyPatch(ctx, execID, internalPatch); err != nil {
			return fmt.Errorf("apply patch to %s: %w", patch.FilePath, err)
		}
	}

	// Create commit
	commitInfo, err := h.repoService.CreateCommit(ctx, execID, resp.CommitMessage)
	if err != nil {
		return fmt.Errorf("create commit: %w", err)
	}

	// Emit commit created event
	h.eventsMan.Emit(ctx, string(events.GitCommitCreated), &events.GitCommitCreatedPayload{
		Commit: *commitInfo,
	})

	// Push branch to remote
	h.eventsMan.Emit(ctx, string(events.GitPushStarted), &events.GitPushStartedPayload{
		BranchName: featureBranch,
		StartedAt:  time.Now(),
	})

	if err := h.repoService.PushBranch(ctx, execID, featureBranch); err != nil {
		h.eventsMan.Emit(ctx, string(events.GitPushFailed), &events.GitPushFailedPayload{
			BranchName:   featureBranch,
			ErrorCode:    events.GitPushErrorNetwork,
			ErrorMessage: err.Error(),
			Retryable:    true,
			FailedAt:     time.Now(),
		})
		return fmt.Errorf("push branch: %w", err)
	}

	// Emit push completed event
	h.eventsMan.Emit(ctx, string(events.GitPushCompleted), &events.GitPushCompletedPayload{
		BranchName:      featureBranch,
		RemoteRef:       "refs/heads/" + featureBranch,
		RemoteCommitSHA: commitInfo.SHA,
		CommitsPushed:   1,
		CompletedAt:     time.Now(),
	})

	// Emit patch generation completed
	if err := h.eventsMan.Emit(ctx, string(events.PatchGenerationCompleted), &events.PatchGenerationCompletedPayload{
		TotalSteps:     1,
		StepsCompleted: 1,
		TotalLLMTokens: resp.TokensUsed,
		FinalCommitSHA: commitInfo.SHA,
		CompletedAt:    time.Now(),
	}); err != nil {
		return err
	}

	// Emit feature delivered
	return h.eventsMan.Emit(ctx, string(events.FeatureDelivered), &events.FeatureDeliveredPayload{
		BranchName:    featureBranch,
		RemoteRef:     "refs/heads/" + featureBranch,
		HeadCommitSHA: commitInfo.SHA,
		Summary: events.DeliverySummary{
			Title:       request.Specification.Title,
			Description: resp.CommitMessage,
			Execution: events.ExecutionSummary{
				StepsCompleted:  1,
				FilesModified:   len(resp.Patches),
				CommitsCreated:  1,
				LLMTokensUsed:   resp.TokensUsed,
			},
		},
	})
}

// =============================================================================
// Feature Completion Handler
// =============================================================================

// FeatureCompletionEvent handles feature completion.
type FeatureCompletionEvent struct {
	cfg           *appconfig.WorkerConfig
	executionRepo repository.ExecutionRepository
	queueMan      QueueManager
}

// NewFeatureCompletionEvent creates a new feature completion event handler.
func NewFeatureCompletionEvent(
	cfg *appconfig.WorkerConfig,
	executionRepo repository.ExecutionRepository,
	queueMan QueueManager,
) *FeatureCompletionEvent {
	return &FeatureCompletionEvent{
		cfg:           cfg,
		executionRepo: executionRepo,
		queueMan:      queueMan,
	}
}

// Name returns the event name.
func (h *FeatureCompletionEvent) Name() string {
	return string(events.FeatureDelivered)
}

// PayloadType returns the expected payload type.
func (h *FeatureCompletionEvent) PayloadType() any {
	return &events.FeatureDeliveredPayload{}
}

// Validate validates the payload.
func (h *FeatureCompletionEvent) Validate(ctx context.Context, payload any) error {
	return nil
}

// Execute processes feature completion.
func (h *FeatureCompletionEvent) Execute(ctx context.Context, payload any) error {
	request, ok := payload.(*events.FeatureDeliveredPayload)
	if !ok {
		return fmt.Errorf("invalid payload type: expected *FeatureDeliveredPayload")
	}

	// Publish result to gateway
	return h.queueMan.Publish(ctx, h.cfg.QueueFeatureResultName, map[string]interface{}{
		"status":      "completed",
		"branch_name": request.BranchName,
		"commit_sha":  request.HeadCommitSHA,
		"summary":     request.Summary,
	})
}

// =============================================================================
// Feature Failure Handler
// =============================================================================

// FeatureFailureEvent handles feature execution failure.
type FeatureFailureEvent struct {
	cfg           *appconfig.WorkerConfig
	executionRepo repository.ExecutionRepository
	queueMan      QueueManager
	eventsMan     EventsEmitter
}

// NewFeatureFailureEvent creates a new feature failure event handler.
func NewFeatureFailureEvent(
	cfg *appconfig.WorkerConfig,
	executionRepo repository.ExecutionRepository,
	queueMan QueueManager,
	eventsMan EventsEmitter,
) *FeatureFailureEvent {
	return &FeatureFailureEvent{
		cfg:           cfg,
		executionRepo: executionRepo,
		queueMan:      queueMan,
		eventsMan:     eventsMan,
	}
}

// Name returns the event name.
func (h *FeatureFailureEvent) Name() string {
	return string(events.FeatureExecutionFailed)
}

// PayloadType returns the expected payload type.
func (h *FeatureFailureEvent) PayloadType() any {
	return &events.FeatureExecutionFailedPayload{}
}

// Validate validates the payload.
func (h *FeatureFailureEvent) Validate(ctx context.Context, payload any) error {
	return nil
}

// Execute processes feature failure.
func (h *FeatureFailureEvent) Execute(ctx context.Context, payload any) error {
	request, ok := payload.(*events.FeatureExecutionFailedPayload)
	if !ok {
		return fmt.Errorf("invalid payload type: expected *FeatureExecutionFailedPayload")
	}

	// Publish failure result to gateway
	return h.queueMan.Publish(ctx, h.cfg.QueueFeatureResultName, map[string]interface{}{
		"status":        "failed",
		"error_code":    request.ErrorCode,
		"error_message": request.ErrorMessage,
		"failed_phase":  request.FailedPhase,
	})
}
