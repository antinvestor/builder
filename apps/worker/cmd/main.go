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
	"github.com/antinvestor/builder/apps/worker/service/llm"
	"github.com/antinvestor/builder/apps/worker/service/queue"
	"github.com/antinvestor/builder/apps/worker/service/repository"
)

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
	ctx, svc := frame.NewServiceWithContext(
		ctx,
		frame.WithConfig(&cfg),
		frame.WithDatastore(),
	)
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

	// Get database pool
	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)

	// ==========================================================================
	// Setup Repositories
	// ==========================================================================

	executionRepo := repository.NewExecutionRepository(ctx, dbPool)
	workspaceRepo := repository.NewWorkspaceRepository(ctx, dbPool)

	// ==========================================================================
	// Setup Services
	// ==========================================================================

	repoService := repository.NewRepositoryService(&cfg, workspaceRepo)
	bamlClient := setupBAMLClient(&cfg)

	// ==========================================================================
	// Register Publishers
	// ==========================================================================

	featureResultPublisher := frame.WithRegisterPublisher(
		cfg.QueueFeatureResultName,
		cfg.QueueFeatureResultURI,
	)

	reviewRequestPublisher := frame.WithRegisterPublisher(
		cfg.QueueReviewRequestName,
		cfg.QueueReviewRequestURI,
	)

	executionRequestPublisher := frame.WithRegisterPublisher(
		cfg.QueueExecutionRequestName,
		cfg.QueueExecutionRequestURI,
	)

	retryLevel1Publisher := frame.WithRegisterPublisher(
		cfg.QueueRetryLevel1Name,
		cfg.QueueRetryLevel1URI,
	)

	dlqPublisher := frame.WithRegisterPublisher(
		cfg.QueueDLQName,
		cfg.QueueDLQURI,
	)

	// ==========================================================================
	// Register Subscribers
	// ==========================================================================

	featureRequestSubscriber := frame.WithRegisterSubscriber(
		cfg.QueueFeatureRequestName,
		cfg.QueueFeatureRequestURI,
		queue.NewFeatureRequestHandler(&cfg, executionRepo, evtsMan),
	)

	// ==========================================================================
	// Register Event Handlers
	// ==========================================================================

	eventHandlers := frame.WithRegisterEvents(
		// Repository operations
		events.NewRepositoryCheckoutEvent(&cfg, repoService, evtsMan),

		// Patch generation
		events.NewPatchGenerationEvent(&cfg, bamlClient, repoService, evtsMan),

		// Completion handlers
		events.NewFeatureCompletionEvent(&cfg, executionRepo, qMan),
		events.NewFeatureFailureEvent(&cfg, executionRepo, qMan, evtsMan),
	)

	// ==========================================================================
	// Setup Health Endpoint
	// ==========================================================================

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"worker"}`))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","service":"worker"}`))
	})

	// ==========================================================================
	// Initialize Service
	// ==========================================================================

	serviceOptions := []frame.Option{
		frame.WithHTTPHandler(mux),
		// Publishers
		featureResultPublisher,
		reviewRequestPublisher,
		executionRequestPublisher,
		retryLevel1Publisher,
		dlqPublisher,
		// Subscribers
		featureRequestSubscriber,
		// Event handlers
		eventHandlers,
	}

	svc.Init(ctx, serviceOptions...)

	// ==========================================================================
	// Start the Service
	// ==========================================================================

	log.Info("Starting feature worker service...")
	err = svc.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("could not run server")
	}
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

func setupBAMLClient(cfg *appconfig.WorkerConfig) events.BAMLClient {
	return llm.NewClient(cfg)
}
