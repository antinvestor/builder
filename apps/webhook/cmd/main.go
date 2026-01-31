package main

import (
	"context"
	"net/http"

	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/util"

	appconfig "github.com/antinvestor/builder/apps/webhook/config"
	"github.com/antinvestor/builder/apps/webhook/service/handlers"
)

func main() {
	ctx := context.Background()

	// Initialize configuration
	cfg, err := config.LoadWithOIDC[appconfig.WebhookConfig](ctx)
	if err != nil {
		util.Log(ctx).With("err", err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServiceName = "feature_webhook"
	}

	// Create service with Frame - minimal dependencies
	ctx, svc := frame.NewServiceWithContext(
		ctx,
		frame.WithConfig(&cfg),
	)
	defer svc.Stop(ctx)
	log := svc.Log(ctx)

	// ==========================================================================
	// Register Publishers
	// ==========================================================================

	featureRequestPublisher := frame.WithRegisterPublisher(
		cfg.QueueFeatureRequestName,
		cfg.QueueFeatureRequestURI,
	)

	githubEventPublisher := frame.WithRegisterPublisher(
		cfg.QueueGitHubEventName,
		cfg.QueueGitHubEventURI,
	)

	// ==========================================================================
	// Setup HTTP Server
	// ==========================================================================

	qMan := svc.QueueManager()
	webhookHandler := handlers.NewWebhookHandler(&cfg, qMan)

	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy","service":"webhook"}`))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready","service":"webhook"}`))
	})

	// GitHub webhook endpoint
	mux.HandleFunc("/webhooks/github", webhookHandler.HandleGitHubWebhook)

	// ==========================================================================
	// Initialize Service
	// ==========================================================================

	serviceOptions := []frame.Option{
		frame.WithHTTPHandler(mux),
		featureRequestPublisher,
		githubEventPublisher,
	}

	svc.Init(ctx, serviceOptions...)

	// ==========================================================================
	// Start the Service
	// ==========================================================================

	log.Info("Starting webhook service...")
	err = svc.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("could not run server")
	}
}
