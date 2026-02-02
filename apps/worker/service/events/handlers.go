package events

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pitabwire/util"

	appconfig "github.com/antinvestor/builder/apps/worker/config"
	"github.com/antinvestor/builder/apps/worker/service/repository"
	"github.com/antinvestor/builder/internal/events"
)

// maxBranchNameLength is the maximum length for feature branch names.
const maxBranchNameLength = 50

// shortIDLength is the length of the short execution ID suffix.
const shortIDLength = 8

// slugifyRegexp matches characters that should be replaced in branch names.
var slugifyRegexp = regexp.MustCompile(`[^a-z0-9]+`)

// generateFeatureBranchName creates a feature branch name from the title.
func generateFeatureBranchName(title string, execID events.ExecutionID) string {
	// Convert to lowercase and replace non-alphanumeric with hyphens
	slug := slugifyRegexp.ReplaceAllString(strings.ToLower(title), "-")
	slug = strings.Trim(slug, "-")

	// Truncate if too long
	if len(slug) > maxBranchNameLength {
		slug = slug[:maxBranchNameLength]
		slug = strings.TrimRight(slug, "-")
	}

	// Add execution ID suffix for uniqueness
	shortID := execID.String()
	if len(shortID) > shortIDLength {
		shortID = shortID[:shortIDLength]
	}

	return fmt.Sprintf("feature/%s-%s", slug, shortID)
}

// Emitter emits events.
type Emitter interface {
	Emit(ctx context.Context, eventName string, payload any) error
}

// EventsEmitter is an alias for Emitter for backwards compatibility.
//
//nolint:revive // intentional stutter for backwards compatibility
type EventsEmitter = Emitter

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
	repoService *repository.Service
	eventsMan   Emitter
}

// NewRepositoryCheckoutEvent creates a new repository checkout event handler.
func NewRepositoryCheckoutEvent(
	cfg *appconfig.WorkerConfig,
	repoService *repository.Service,
	eventsMan Emitter,
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
func (h *RepositoryCheckoutEvent) Validate(_ context.Context, _ any) error {
	return nil
}

// Execute processes the repository checkout event.
func (h *RepositoryCheckoutEvent) Execute(ctx context.Context, payload any) error {
	request, ok := payload.(*events.FeatureExecutionInitializedPayload)
	if !ok {
		return errors.New("invalid payload type: expected *FeatureExecutionInitializedPayload")
	}

	// Use execution ID from the request (created by queue handler)
	execID := request.ExecutionID

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
		emitErr := h.eventsMan.Emit(
			ctx,
			string(events.RepositoryCheckoutFailed),
			&events.RepositoryCheckoutFailedPayload{
				ErrorCode:    events.CheckoutErrorNetwork,
				ErrorMessage: err.Error(),
				Retryable:    true,
				FailedAt:     time.Now(),
			},
		)
		if emitErr != nil {
			util.Log(ctx).Warn("failed to emit checkout failure event", "error", emitErr)
		}
		return err
	}

	// Generate feature branch name
	featureBranch := request.Repository.FeatureBranchName
	if featureBranch == "" {
		featureBranch = generateFeatureBranchName(request.Spec.Title, execID)
	}

	// Emit completion with feature spec for downstream handlers
	return h.eventsMan.Emit(
		ctx,
		string(events.RepositoryCheckoutCompleted),
		&events.RepositoryCheckoutCompletedPayload{
			ExecutionID:       execID,
			WorkspacePath:     result.WorkspacePath,
			HeadCommitSHA:     result.CommitSHA,
			BranchName:        result.Branch,
			FeatureBranchName: featureBranch,
			Spec:              request.Spec,
			RepositoryURL:     request.Repository.RemoteURL,
			DurationMS:        result.CheckoutTimeMS,
			CompletedAt:       time.Now(),
		},
	)
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
	repoService *repository.Service
	eventsMan   Emitter
}

// NewPatchGenerationEvent creates a new patch generation event handler.
func NewPatchGenerationEvent(
	cfg *appconfig.WorkerConfig,
	bamlClient BAMLClient,
	repoService *repository.Service,
	eventsMan Emitter,
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
func (h *PatchGenerationEvent) Validate(_ context.Context, _ any) error {
	return nil
}

// patchStats tracks file change statistics.
type patchStats struct {
	filesCreated  int
	filesModified int
	filesDeleted  int
	linesAdded    int
	linesRemoved  int
}

// Execute processes patch generation.
func (h *PatchGenerationEvent) Execute(ctx context.Context, payload any) error {
	log := util.Log(ctx)

	request, ok := payload.(*events.RepositoryCheckoutCompletedPayload)
	if !ok {
		return errors.New("invalid payload type: expected *RepositoryCheckoutCompletedPayload")
	}

	execID := request.ExecutionID
	startTime := time.Now()

	log.Info("starting patch generation",
		"execution_id", execID.String(),
		"feature_branch", request.FeatureBranchName,
	)

	// Phase 1: Setup - emit started, create branch
	if err := h.setupPatchGeneration(ctx, execID, request, startTime); err != nil {
		return err
	}

	// Phase 2: Generate and apply patches
	resp, stats, err := h.generateAndApplyPatches(ctx, execID, request)
	if err != nil {
		return err
	}

	// Phase 3: Commit and push
	commitInfo, err := h.commitAndPush(ctx, execID, request, resp)
	if err != nil {
		return err
	}

	// Phase 4: Emit completion events
	return h.emitCompletionEvents(ctx, execID, request, resp, stats, commitInfo, startTime)
}

// setupPatchGeneration handles initial setup for patch generation.
func (h *PatchGenerationEvent) setupPatchGeneration(
	ctx context.Context,
	execID events.ExecutionID,
	request *events.RepositoryCheckoutCompletedPayload,
	startTime time.Time,
) error {
	log := util.Log(ctx)

	// Emit patch generation started
	if err := h.eventsMan.Emit(ctx, string(events.PatchGenerationStarted), &events.PatchGenerationStartedPayload{
		TotalSteps: 1,
		StartedAt:  startTime,
	}); err != nil {
		return err
	}

	// Create feature branch before making changes
	if err := h.repoService.CreateBranch(ctx, execID, request.FeatureBranchName); err != nil {
		return h.emitGenerationFailure(ctx, execID, "branch_creation", err, events.StepErrorCategoryResource)
	}

	// Emit branch created event
	if err := h.eventsMan.Emit(ctx, string(events.GitBranchCreated), &events.GitBranchCreatedPayload{
		BranchName:    request.FeatureBranchName,
		BaseBranch:    request.BranchName,
		BaseCommitSHA: request.HeadCommitSHA,
		CreatedAt:     time.Now(),
	}); err != nil {
		log.Warn("failed to emit branch created event", "error", err)
	}

	return nil
}

// generateAndApplyPatches generates patches using LLM and applies them.
func (h *PatchGenerationEvent) generateAndApplyPatches(
	ctx context.Context,
	execID events.ExecutionID,
	request *events.RepositoryCheckoutCompletedPayload,
) (*GeneratePatchResponse, *patchStats, error) {
	log := util.Log(ctx)

	// Get repository structure for LLM context
	repoContext, err := h.repoService.GetProjectStructure(ctx, execID)
	if err != nil {
		log.Warn("failed to get project structure", "error", err)
		repoContext = ""
	}

	// Generate patches using BAML/LLM
	resp, err := h.bamlClient.GeneratePatch(ctx, &GeneratePatchRequest{
		ExecutionID:       execID,
		Specification:     request.Spec,
		WorkspacePath:     request.WorkspacePath,
		RepositoryContext: repoContext,
		IterationNumber:   1,
	})
	if err != nil {
		return nil, nil, h.emitGenerationFailure(ctx, execID, "llm_generation", err, events.StepErrorCategoryLLM)
	}

	// Apply patches and collect stats
	stats, err := h.applyPatches(ctx, execID, resp.Patches)
	if err != nil {
		return nil, nil, err
	}

	return resp, stats, nil
}

// applyPatches applies patches and returns statistics.
func (h *PatchGenerationEvent) applyPatches(
	ctx context.Context,
	execID events.ExecutionID,
	patches []Patch,
) (*patchStats, error) {
	log := util.Log(ctx)
	stats := &patchStats{}

	for _, patch := range patches {
		eventsPatch := &events.Patch{
			FilePath:   patch.FilePath,
			Action:     patch.Action,
			OldContent: patch.OldContent,
			NewContent: patch.NewContent,
		}

		if applyErr := h.repoService.ApplyPatch(ctx, execID, eventsPatch); applyErr != nil {
			log.WithError(applyErr).Error("failed to apply patch", "file", patch.FilePath)
			return nil, h.emitGenerationFailure(
				ctx, execID, "patch_application", applyErr, events.StepErrorCategoryResource,
			)
		}

		h.updatePatchStats(stats, &patch)
	}

	return stats, nil
}

// updatePatchStats updates statistics based on patch action.
func (h *PatchGenerationEvent) updatePatchStats(stats *patchStats, patch *Patch) {
	switch patch.Action {
	case events.FileActionCreate:
		stats.filesCreated++
		stats.linesAdded += countLines(patch.NewContent)
	case events.FileActionModify:
		stats.filesModified++
		stats.linesAdded += countLines(patch.NewContent)
		stats.linesRemoved += countLines(patch.OldContent)
	case events.FileActionDelete:
		stats.filesDeleted++
		stats.linesRemoved += countLines(patch.OldContent)
	case events.FileActionRename:
		stats.filesModified++
	}
}

// commitAndPush creates commit and pushes to remote.
func (h *PatchGenerationEvent) commitAndPush(
	ctx context.Context,
	execID events.ExecutionID,
	request *events.RepositoryCheckoutCompletedPayload,
	resp *GeneratePatchResponse,
) (*events.CommitInfo, error) {
	log := util.Log(ctx)

	commitMessage := resp.CommitMessage
	if commitMessage == "" {
		commitMessage = fmt.Sprintf("feat: %s\n\nImplemented via automated feature builder.", request.Spec.Title)
	}

	commitInfo, err := h.repoService.CreateCommit(ctx, execID, commitMessage)
	if err != nil {
		return nil, h.emitGenerationFailure(ctx, execID, "commit_creation", err, events.StepErrorCategoryResource)
	}

	// Emit commit created event
	if emitErr := h.eventsMan.Emit(ctx, string(events.GitCommitCreated), &events.GitCommitCreatedPayload{
		Commit: *commitInfo,
	}); emitErr != nil {
		log.Warn("failed to emit commit created event", "error", emitErr)
	}

	// Push the branch
	if pushErr := h.pushBranch(ctx, execID, request, commitInfo); pushErr != nil {
		return nil, pushErr
	}

	return commitInfo, nil
}

// pushBranch pushes the branch to remote with event emission.
func (h *PatchGenerationEvent) pushBranch(
	ctx context.Context,
	execID events.ExecutionID,
	request *events.RepositoryCheckoutCompletedPayload,
	commitInfo *events.CommitInfo,
) error {
	log := util.Log(ctx)

	if err := h.eventsMan.Emit(ctx, string(events.GitPushStarted), &events.GitPushStartedPayload{
		BranchName:     request.FeatureBranchName,
		RemoteName:     "origin",
		RemoteURL:      request.RepositoryURL,
		LocalCommitSHA: commitInfo.SHA,
		CommitCount:    1,
		StartedAt:      time.Now(),
	}); err != nil {
		log.Warn("failed to emit push started event", "error", err)
	}

	if err := h.repoService.PushBranch(ctx, execID, request.FeatureBranchName); err != nil {
		h.emitPushFailure(ctx, request.FeatureBranchName, err)
		return h.emitGenerationFailure(ctx, execID, "push", err, events.StepErrorCategoryResource)
	}

	return nil
}

// emitPushFailure emits a push failed event.
func (h *PatchGenerationEvent) emitPushFailure(ctx context.Context, branchName string, err error) {
	log := util.Log(ctx)
	errorCode, retryable := classifyPushError(err)
	pushFailErr := h.eventsMan.Emit(ctx, string(events.GitPushFailed), &events.GitPushFailedPayload{
		BranchName:   branchName,
		ErrorCode:    errorCode,
		ErrorMessage: err.Error(),
		Retryable:    retryable,
		FailedAt:     time.Now(),
	})
	if pushFailErr != nil {
		log.Warn("failed to emit push failed event", "error", pushFailErr)
	}
}

// classifyPushError determines the error code and retryability based on error message.
func classifyPushError(err error) (events.GitPushErrorCode, bool) {
	errMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errMsg, "authentication") ||
		strings.Contains(errMsg, "permission denied") ||
		strings.Contains(errMsg, "could not read from remote") ||
		strings.Contains(errMsg, "invalid credentials"):
		return events.GitPushErrorAuth, false

	case strings.Contains(errMsg, "protected branch") ||
		strings.Contains(errMsg, "branch is protected") ||
		strings.Contains(errMsg, "cannot force push"):
		return events.GitPushErrorProtected, false

	case strings.Contains(errMsg, "non-fast-forward") ||
		strings.Contains(errMsg, "rejected") ||
		strings.Contains(errMsg, "failed to push"):
		return events.GitPushErrorRejected, true

	case strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "connection") ||
		strings.Contains(errMsg, "network") ||
		strings.Contains(errMsg, "unable to access"):
		return events.GitPushErrorNetwork, true

	default:
		// Default to network error with retry for unknown errors
		return events.GitPushErrorNetwork, true
	}
}

// emitCompletionEvents emits all completion events.
func (h *PatchGenerationEvent) emitCompletionEvents(
	ctx context.Context,
	execID events.ExecutionID,
	request *events.RepositoryCheckoutCompletedPayload,
	resp *GeneratePatchResponse,
	stats *patchStats,
	commitInfo *events.CommitInfo,
	startTime time.Time,
) error {
	log := util.Log(ctx)
	durationMS := time.Since(startTime).Milliseconds()

	// Emit push completed
	if err := h.eventsMan.Emit(ctx, string(events.GitPushCompleted), &events.GitPushCompletedPayload{
		BranchName:      request.FeatureBranchName,
		RemoteRef:       fmt.Sprintf("refs/heads/%s", request.FeatureBranchName),
		RemoteCommitSHA: commitInfo.SHA,
		CommitsPushed:   1,
		DurationMS:      durationMS,
		CompletedAt:     time.Now(),
	}); err != nil {
		log.Warn("failed to emit push completed event", "error", err)
	}

	// Emit patch generation completed
	if err := h.eventsMan.Emit(ctx, string(events.PatchGenerationCompleted), &events.PatchGenerationCompletedPayload{
		ExecutionID:       execID,
		TotalSteps:        1,
		StepsCompleted:    1,
		TotalFileChanges:  len(resp.Patches),
		FilesCreated:      stats.filesCreated,
		FilesModified:     stats.filesModified,
		FilesDeleted:      stats.filesDeleted,
		TotalLinesAdded:   stats.linesAdded,
		TotalLinesRemoved: stats.linesRemoved,
		Commits:           []events.CommitInfo{*commitInfo},
		FinalCommitSHA:    commitInfo.SHA,
		TotalDurationMS:   durationMS,
		TotalLLMTokens:    resp.TokensUsed,
		CompletedAt:       time.Now(),
	}); err != nil {
		return err
	}

	// Emit feature delivered
	return h.eventsMan.Emit(ctx, string(events.FeatureDelivered), &events.FeatureDeliveredPayload{
		BranchName:    request.FeatureBranchName,
		RemoteRef:     fmt.Sprintf("refs/heads/%s", request.FeatureBranchName),
		HeadCommitSHA: commitInfo.SHA,
		Summary: events.DeliverySummary{
			Title:       request.Spec.Title,
			Description: request.Spec.Description,
			Execution: events.ExecutionSummary{
				StepsCompleted:    1,
				FilesCreated:      stats.filesCreated,
				FilesModified:     stats.filesModified,
				FilesDeleted:      stats.filesDeleted,
				TotalLinesAdded:   stats.linesAdded,
				TotalLinesRemoved: stats.linesRemoved,
				CommitsCreated:    1,
				TotalDurationMS:   durationMS,
				LLMTokensUsed:     resp.TokensUsed,
			},
		},
	})
}

// emitGenerationFailure emits a patch generation step failed event.
func (h *PatchGenerationEvent) emitGenerationFailure(
	ctx context.Context,
	_ events.ExecutionID, // execID reserved for future use
	phase string,
	err error,
	category events.StepErrorCategory,
) error {
	log := util.Log(ctx)

	emitErr := h.eventsMan.Emit(ctx, string(events.PatchGenerationStepFailed), &events.PatchGenerationStepFailedPayload{
		StepNumber:    1,
		ErrorCode:     phase,
		ErrorMessage:  err.Error(),
		ErrorCategory: category,
		Retryable:     true,
		FailedAt:      time.Now(),
	})
	if emitErr != nil {
		log.Warn("failed to emit step failure event", "error", emitErr)
	}

	return fmt.Errorf("%s failed: %w", phase, err)
}

// countLines counts the number of lines in a string.
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return 1 + strings.Count(s, "\n")
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
func (h *FeatureCompletionEvent) Validate(_ context.Context, _ any) error {
	return nil
}

// Execute processes feature completion.
func (h *FeatureCompletionEvent) Execute(ctx context.Context, payload any) error {
	request, ok := payload.(*events.FeatureDeliveredPayload)
	if !ok {
		return errors.New("invalid payload type: expected *FeatureDeliveredPayload")
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
	eventsMan     Emitter
}

// NewFeatureFailureEvent creates a new feature failure event handler.
func NewFeatureFailureEvent(
	cfg *appconfig.WorkerConfig,
	executionRepo repository.ExecutionRepository,
	queueMan QueueManager,
	eventsMan Emitter,
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
func (h *FeatureFailureEvent) Validate(_ context.Context, _ any) error {
	return nil
}

// Execute processes feature failure.
func (h *FeatureFailureEvent) Execute(ctx context.Context, payload any) error {
	request, ok := payload.(*events.FeatureExecutionFailedPayload)
	if !ok {
		return errors.New("invalid payload type: expected *FeatureExecutionFailedPayload")
	}

	// Publish failure result to gateway
	return h.queueMan.Publish(ctx, h.cfg.QueueFeatureResultName, map[string]interface{}{
		"status":        "failed",
		"error_code":    request.ErrorCode,
		"error_message": request.ErrorMessage,
		"failed_phase":  request.FailedPhase,
	})
}
