package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/util"

	appconfig "github.com/antinvestor/builder/apps/executor/config"
	"github.com/antinvestor/builder/apps/executor/service/sandbox"
)

func main() {
	ctx := context.Background()

	// Initialize configuration
	cfg, err := config.LoadWithOIDC[appconfig.ExecutorConfig](ctx)
	if err != nil {
		util.Log(ctx).With("err", err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServiceName = "feature_executor"
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

	// ==========================================================================
	// Setup Sandbox Executor
	// ==========================================================================

	sandboxExecutor := sandbox.NewSandboxExecutor(&cfg)
	testRunner := sandbox.NewMultiRunner(&cfg)

	// ==========================================================================
	// Register Publishers
	// ==========================================================================

	executionResultPublisher := frame.WithRegisterPublisher(
		cfg.QueueExecutionResultName,
		cfg.QueueExecutionResultURI,
	)

	// ==========================================================================
	// Register Subscribers
	// ==========================================================================

	executionRequestSubscriber := frame.WithRegisterSubscriber(
		cfg.QueueExecutionRequestName,
		cfg.QueueExecutionRequestURI,
		sandbox.NewExecutionRequestHandler(&cfg, sandboxExecutor, testRunner, evtsMan),
	)

	// ==========================================================================
	// Setup Health Endpoint
	// ==========================================================================

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"executor"}`))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// Check if Docker is available
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","service":"executor"}`))
	})

	// Active executions endpoint
	mux.HandleFunc("/api/v1/executions/active", func(w http.ResponseWriter, r *http.Request) {
		count := sandboxExecutor.ActiveCount()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"active_executions":%d}`, count)
	})

	// ==========================================================================
	// Initialize Service
	// ==========================================================================

	serviceOptions := []frame.Option{
		frame.WithHTTPHandler(mux),
		// Publishers
		executionResultPublisher,
		// Subscribers
		executionRequestSubscriber,
	}

	svc.Init(ctx, serviceOptions...)

	// ==========================================================================
	// Start the Service
	// ==========================================================================

	log.Info("Starting feature executor service...")
	err = svc.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("could not run server")
	}
}
