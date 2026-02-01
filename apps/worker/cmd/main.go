package main

import (
	"context"
	"net/http"

	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/util"

	appconfig "github.com/antinvestor/builder/apps/worker/config"
	"github.com/antinvestor/builder/apps/worker/service/events"
	"github.com/antinvestor/builder/apps/worker/service/queue"
	"github.com/antinvestor/builder/apps/worker/service/repository"
	internalevents "github.com/antinvestor/builder/internal/events"
	"github.com/antinvestor/builder/internal/llm"
)

// LLM configuration defaults.
const defaultMaxOutputTokens = 16384

func main() {
	ctx := context.Background()

	// Initialize configuration
	cfg, err := config.LoadWithOIDC[appconfig.WorkerConfig](ctx)
	if err != nil {
		util.Log(ctx).With("err", err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServiceName = "feature_worker"
	}

	// Create service with Frame
	ctx, svc := frame.NewServiceWithContext(ctx, frame.WithConfig(&cfg), frame.WithDatastore())
	defer svc.Stop(ctx)
	log := svc.Log(ctx)

	// Get managers
	dbManager := svc.DatastoreManager()
	evtsMan := svc.EventsManager()
	qMan := svc.QueueManager()

	// Handle database migration
	if handleDatabaseMigration(ctx, dbManager, cfg) {
		return
	}

	// Get database pool and setup repositories
	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)
	executionRepo := repository.NewExecutionRepository(ctx, dbPool)
	workspaceRepo := repository.NewWorkspaceRepository(ctx, dbPool)

	// ==========================================================================
	// Setup Services
	// ==========================================================================

	repoService := repository.NewService(&cfg, workspaceRepo)
	bamlClient := setupBAMLClient(ctx, &cfg)

	// ==========================================================================
	// Setup Workspace Cleanup Service
	// ==========================================================================

	workspaceCleanup := repository.NewWorkspaceCleanupService(
		workspaceRepo,
		cfg.WorkspaceBasePath,
		cfg.MaxWorkspaceAgeHours,
	)
	workspaceCleanup.Start(ctx)
	defer workspaceCleanup.Stop()

	// Build service options
	serviceOptions := buildServiceOptions(&cfg, executionRepo, evtsMan, qMan, repoService, bamlClient)

	// Initialize and run service
	svc.Init(ctx, serviceOptions...)
	log.Info("Starting feature worker service...")
	if err = svc.Run(ctx, ""); err != nil {
		log.WithError(err).Fatal("could not run server")
	}
}

func buildServiceOptions(
	cfg *appconfig.WorkerConfig,
	executionRepo repository.ExecutionRepository,
	evtsMan events.EventsEmitter,
	qMan events.QueueManager,
	repoService *repository.RepositoryService,
	bamlClient events.BAMLClient,
) []frame.Option {
	return []frame.Option{
		frame.WithHTTPHandler(setupHealthEndpoints()),
		// Publishers
		frame.WithRegisterPublisher(cfg.QueueFeatureResultName, cfg.QueueFeatureResultURI),
		frame.WithRegisterPublisher(cfg.QueueReviewRequestName, cfg.QueueReviewRequestURI),
		frame.WithRegisterPublisher(cfg.QueueExecutionRequestName, cfg.QueueExecutionRequestURI),
		frame.WithRegisterPublisher(cfg.QueueRetryLevel1Name, cfg.QueueRetryLevel1URI),
		frame.WithRegisterPublisher(cfg.QueueDLQName, cfg.QueueDLQURI),
		// Subscribers
		frame.WithRegisterSubscriber(
			cfg.QueueFeatureRequestName,
			cfg.QueueFeatureRequestURI,
			queue.NewFeatureRequestHandler(cfg, executionRepo, evtsMan),
		),
		// Event handlers
		frame.WithRegisterEvents(
			events.NewRepositoryCheckoutEvent(cfg, repoService, evtsMan),
			events.NewPatchGenerationEvent(cfg, bamlClient, repoService, evtsMan),
			events.NewFeatureCompletionEvent(cfg, executionRepo, qMan),
			events.NewFeatureFailureEvent(cfg, executionRepo, qMan, evtsMan),
		),
	}
}

func setupHealthEndpoints() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy","service":"worker"}`))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready","service":"worker"}`))
	})
	return mux
}

func handleDatabaseMigration(
	ctx context.Context,
	dbManager datastore.Manager,
	cfg appconfig.WorkerConfig,
) bool {
	if cfg.DoDatabaseMigrate() {
		err := repository.Migrate(ctx, dbManager, cfg.GetDatabaseMigrationPath())
		if err != nil {
			util.Log(ctx).WithError(err).Fatal("could not migrate")
		}
		return true
	}
	return false
}

func setupBAMLClient(ctx context.Context, cfg *appconfig.WorkerConfig) events.BAMLClient {
	log := util.Log(ctx)

	// Create LLM client configuration
	llmCfg := llm.ClientConfig{
		AnthropicAPIKey: cfg.AnthropicAPIKey,
		OpenAIAPIKey:    cfg.OpenAIAPIKey,
		GoogleAPIKey:    cfg.GoogleAPIKey,
		DefaultProvider: llm.Provider(cfg.DefaultLLMProvider),
		DefaultModel:    llm.ModelClaudeSonnet,
		TimeoutSeconds:  cfg.LLMTimeoutSeconds,
		MaxRetries:      cfg.LLMMaxRetries,
		MaxOutputTokens: defaultMaxOutputTokens,
		Temperature:     0.0,
	}

	// Try to create real LLM client
	bamlClient, err := llm.NewBAMLClient(llmCfg)
	if err != nil {
		log.WithError(err).Warn("failed to create LLM client, using stub")
		return &bamlClientStub{cfg: cfg}
	}

	log.Info("LLM client initialized",
		"provider", cfg.DefaultLLMProvider,
	)

	return &bamlClientAdapter{client: bamlClient}
}

// bamlClientAdapter adapts llm.BAMLClient to events.BAMLClient.
type bamlClientAdapter struct {
	client *llm.BAMLClient
}

// GeneratePatch implements events.BAMLClient.
func (a *bamlClientAdapter) GeneratePatch(
	ctx context.Context,
	req *events.GeneratePatchRequest,
) (*events.GeneratePatchResponse, error) {
	// Convert events.GeneratePatchRequest to llm.GeneratePatchRequest
	llmReq := &llm.GeneratePatchRequest{
		ExecutionID: req.ExecutionID.String(),
		Specification: llm.FeatureSpecification{
			Title:              req.Specification.Title,
			Description:        req.Specification.Description,
			AcceptanceCriteria: req.Specification.AcceptanceCriteria,
			PathHints:          req.Specification.PathHints,
			AdditionalContext:  req.Specification.AdditionalContext,
			Category:           llm.FeatureCategory(req.Specification.Category),
		},
		WorkspacePath:      req.WorkspacePath,
		RepositoryContext:  req.RepositoryContext,
		IterationNumber:    req.IterationNumber,
		FeedbackFromReview: req.FeedbackFromReview,
	}

	// Convert previous patches
	for _, p := range req.PreviousPatches {
		llmReq.PreviousPatches = append(llmReq.PreviousPatches, llm.Patch{
			FilePath:   p.FilePath,
			OldContent: p.OldContent,
			NewContent: p.NewContent,
			Action:     string(p.Action),
		})
	}

	// Call the LLM client
	resp, err := a.client.GeneratePatch(ctx, llmReq)
	if err != nil {
		return nil, err
	}

	// Convert llm.GeneratePatchResponse to events.GeneratePatchResponse
	evtResp := &events.GeneratePatchResponse{
		CommitMessage: resp.CommitMessage,
		TokensUsed:    resp.TokensUsed,
	}

	// Convert patches
	for _, p := range resp.Patches {
		evtResp.Patches = append(evtResp.Patches, events.Patch{
			FilePath:   p.FilePath,
			OldContent: p.OldContent,
			NewContent: p.NewContent,
			Action:     internalevents.FileAction(p.Action),
		})
	}

	return evtResp, nil
}

// bamlClientStub is a fallback stub when LLM is not configured.
type bamlClientStub struct {
	cfg *appconfig.WorkerConfig
}

// GeneratePatch implements events.BAMLClient.
func (c *bamlClientStub) GeneratePatch(
	_ context.Context,
	_ *events.GeneratePatchRequest,
) (*events.GeneratePatchResponse, error) {
	return &events.GeneratePatchResponse{
		Patches:       []events.Patch{},
		CommitMessage: "Generated by BAML (stub)",
		TokensUsed:    0,
	}, nil
}
