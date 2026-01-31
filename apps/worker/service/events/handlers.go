package events

import (
	"context"
	"fmt"
	"time"

	appconfig "service-feature/apps/worker/config"
	"service-feature/apps/worker/service/repository"
	"service-feature/internal/events"
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

	// Emit completion
	return h.eventsMan.Emit(ctx, string(events.RepositoryCheckoutCompleted), &events.RepositoryCheckoutCompletedPayload{
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

// Execute processes patch generation.
func (h *PatchGenerationEvent) Execute(ctx context.Context, payload any) error {
	// Emit patch generation started
	if err := h.eventsMan.Emit(ctx, string(events.PatchGenerationStarted), &events.PatchGenerationStartedPayload{
		TotalSteps: 1,
		StartedAt:  time.Now(),
	}); err != nil {
		return err
	}

	// Generate patches using BAML/LLM
	resp, err := h.bamlClient.GeneratePatch(ctx, &GeneratePatchRequest{
		ExecutionID: events.NewExecutionID(),
	})
	if err != nil {
		return err
	}

	// Emit patch generation completed
	return h.eventsMan.Emit(ctx, string(events.PatchGenerationCompleted), &events.PatchGenerationCompletedPayload{
		TotalSteps:     1,
		StepsCompleted: 1,
		TotalLLMTokens: resp.TokensUsed,
		FinalCommitSHA: "stub-commit",
		CompletedAt:    time.Now(),
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
