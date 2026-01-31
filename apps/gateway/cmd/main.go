package main

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	connectInterceptors "github.com/pitabwire/frame/security/interceptors/connect"
	"github.com/pitabwire/util"

	appconfig "github.com/antinvestor/builder/apps/gateway/config"
)

func main() {
	ctx := context.Background()

	// Initialize configuration
	cfg, err := config.LoadWithOIDC[appconfig.GatewayConfig](ctx)
	if err != nil {
		util.Log(ctx).With("err", err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServiceName = "feature_gateway"
	}

	// Create service with Frame - minimal dependencies
	ctx, svc := frame.NewServiceWithContext(
		ctx,
		frame.WithConfig(&cfg),
		frame.WithRegisterServerOauth2Client(),
	)
	defer svc.Stop(ctx)
	log := svc.Log(ctx)

	// Get queue manager for publishing
	qMan := svc.QueueManager()

	// ==========================================================================
	// Register Publishers
	// ==========================================================================

	featureRequestPublisher := frame.WithRegisterPublisher(
		cfg.QueueFeatureRequestName,
		cfg.QueueFeatureRequestURI,
	)

	// ==========================================================================
	// Setup HTTP Server
	// ==========================================================================

	securityMan := svc.SecurityManager()
	authenticator := securityMan.GetAuthenticator(ctx)

	defaultInterceptorList, err := connectInterceptors.DefaultList(ctx, authenticator)
	if err != nil {
		log.WithError(err).Fatal("could not create default interceptors")
	}
	_ = defaultInterceptorList
	_ = connect.WithInterceptors // Silence unused

	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"gateway"}`))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","service":"gateway"}`))
	})

	// Feature submission endpoint
	mux.HandleFunc("/api/v1/features", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// TODO: Parse request and publish to queue
		// This is where the gateway receives feature requests and publishes them
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"accepted","message":"Feature request queued"}`))
	})

	_ = qMan // Will be used in feature endpoint

	// ==========================================================================
	// Initialize Service
	// ==========================================================================

	serviceOptions := []frame.Option{
		frame.WithHTTPHandler(mux),
		featureRequestPublisher,
	}

	svc.Init(ctx, serviceOptions...)

	// ==========================================================================
	// Start the Service
	// ==========================================================================

	log.Info("Starting feature gateway service...")
	err = svc.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("could not run server")
	}
}
