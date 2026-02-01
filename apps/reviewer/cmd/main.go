package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/util"

	appconfig "github.com/antinvestor/builder/apps/reviewer/config"
	"github.com/antinvestor/builder/apps/reviewer/service/review"
)

//nolint:funlen // service bootstrap requires setup of all components
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
	decisionEngine := review.NewThresholdDecisionEngine(&cfg)
	killSwitchService := review.NewPersistentKillSwitchService(&cfg, evtsMan)

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
		review.NewRequestHandler(
			&cfg,
			securityAnalyzer,
			architectureAnalyzer,
			decisionEngine,
			killSwitchService,
			evtsMan,
		),
	)

	// ==========================================================================
	// Setup Health Endpoint
	// ==========================================================================

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy","service":"reviewer"}`))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready","service":"reviewer"}`))
	})

	// Kill switch status endpoint
	mux.HandleFunc("/api/v1/killswitch/status", func(w http.ResponseWriter, r *http.Request) {
		status, statusErr := killSwitchService.GetStatus(r.Context())
		if statusErr != nil {
			http.Error(w, statusErr.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if encodeErr := json.NewEncoder(w).Encode(status); encodeErr != nil {
			http.Error(w, "failed to encode status", http.StatusInternalServerError)
		}
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
