package main

import (
	"context"
	"net/http"

	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/util"

	appconfig "service-feature/apps/reviewer/config"
	"service-feature/apps/reviewer/service/review"
)

func main() {
	ctx := context.Background()

	// Initialize configuration
	cfg, err := config.LoadWithOIDC[appconfig.ReviewerConfig](ctx)
	if err != nil {
		util.Log(ctx).With("err", err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServiceName = "feature_reviewer"
	}

	// Create service with Frame - minimal dependencies
	ctx, svc := frame.NewServiceWithContext(
		ctx,
		frame.WithConfig(&cfg),
	)
	defer svc.Stop(ctx)
	log := svc.Log(ctx)

	// Get managers
	evtsMan := svc.EventsManager()
	qMan := svc.QueueManager()

	// ==========================================================================
	// Setup Review Components
	// ==========================================================================

	securityAnalyzer := review.NewPatternSecurityAnalyzer(&cfg)
	architectureAnalyzer := review.NewPatternArchitectureAnalyzer(&cfg)
	decisionEngine := review.NewConservativeDecisionEngine(&cfg)
	killSwitchService := review.NewDefaultKillSwitchService(&cfg, evtsMan)

	_ = securityAnalyzer
	_ = architectureAnalyzer
	_ = decisionEngine
	_ = killSwitchService
	_ = qMan

	// ==========================================================================
	// Register Publishers
	// ==========================================================================

	reviewResultPublisher := frame.WithRegisterPublisher(
		cfg.QueueReviewResultName,
		cfg.QueueReviewResultURI,
	)

	controlEventsPublisher := frame.WithRegisterPublisher(
		cfg.QueueControlEventsName,
		cfg.QueueControlEventsURI,
	)

	// ==========================================================================
	// Register Subscribers
	// ==========================================================================

	reviewRequestSubscriber := frame.WithRegisterSubscriber(
		cfg.QueueReviewRequestName,
		cfg.QueueReviewRequestURI,
		review.NewReviewRequestHandler(&cfg, securityAnalyzer, architectureAnalyzer, decisionEngine, killSwitchService, evtsMan),
	)

	// ==========================================================================
	// Setup Health Endpoint
	// ==========================================================================

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"reviewer"}`))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","service":"reviewer"}`))
	})

	// Kill switch status endpoint
	mux.HandleFunc("/api/v1/killswitch/status", func(w http.ResponseWriter, r *http.Request) {
		status, err := killSwitchService.GetStatus(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// TODO: Marshal status to JSON
		_ = status
		w.Write([]byte(`{"global_active":false}`))
	})

	// ==========================================================================
	// Initialize Service
	// ==========================================================================

	serviceOptions := []frame.Option{
		frame.WithHTTPHandler(mux),
		// Publishers
		reviewResultPublisher,
		controlEventsPublisher,
		// Subscribers
		reviewRequestSubscriber,
	}

	svc.Init(ctx, serviceOptions...)

	// ==========================================================================
	// Start the Service
	// ==========================================================================

	log.Info("Starting feature reviewer service...")
	err = svc.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("could not run server")
	}
}
