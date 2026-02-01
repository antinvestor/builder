package main

import (
	"context"
	"net/http"

	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/util"

	appconfig "github.com/antinvestor/builder/apps/gateway/config"
	"github.com/antinvestor/builder/apps/gateway/middleware"
	"github.com/antinvestor/builder/apps/gateway/service/handlers"
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

	// Setup Security and Middleware
	securityMan := svc.SecurityManager()
	authenticator := securityMan.GetAuthenticator(ctx)
	authMiddleware := middleware.NewAuthMiddleware(authenticator)

	rateLimiter := middleware.NewRateLimiter(
		cfg.RateLimitRequestsPerMinute,
		cfg.RateLimitBurstSize,
	)
	defer rateLimiter.Stop()

	log.Info("rate limiter configured",
		"requests_per_minute", cfg.RateLimitRequestsPerMinute,
		"burst_size", cfg.RateLimitBurstSize,
	)

	// Register Publishers
	featureRequestPublisher := frame.WithRegisterPublisher(
		cfg.QueueFeatureRequestName,
		cfg.QueueFeatureRequestURI,
	)

	// Initialize service with publishers before creating handlers
	svc.Init(ctx, featureRequestPublisher)

	// Get queue manager for publishing (after Init registers publishers)
	qMan := svc.QueueManager()

	// Create feature request handler
	featureHandler := handlers.NewFeatureRequestHandler(&cfg, qMan)

	// Setup HTTP Handlers and Routes
	mux := setupRoutes(log, authMiddleware, rateLimiter, featureHandler)

	// Set HTTP handler and run service
	svc.Init(ctx, frame.WithHTTPHandler(mux))

	log.Info("Starting feature gateway service...")
	if err = svc.Run(ctx, ""); err != nil {
		log.WithError(err).Fatal("could not run server")
	}
}

func setupRoutes(
	log *util.LogEntry,
	authMiddleware *middleware.AuthMiddleware,
	rateLimiter *middleware.RateLimiter,
	featureHandler *handlers.FeatureRequestHandler,
) *http.ServeMux {
	mux := http.NewServeMux()

	// Health endpoints - no auth, no rate limiting
	mux.Handle("/health", healthHandler(log))
	mux.Handle("/ready", readyHandler(log))

	// Feature endpoint - requires auth and rate limiting
	mux.Handle("/api/v1/features",
		rateLimiter.Middleware(
			authMiddleware.Middleware(featureHandler),
		),
	)

	log.Info("routes configured",
		"endpoints", []string{"/health", "/ready", "/api/v1/features"},
	)

	return mux
}

func healthHandler(log *util.LogEntry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, writeErr := w.Write([]byte(`{"status":"healthy","service":"gateway"}`)); writeErr != nil {
			log.WithError(writeErr).Error("failed to write health response")
		}
	})
}

func readyHandler(log *util.LogEntry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, writeErr := w.Write([]byte(`{"status":"ready","service":"gateway"}`)); writeErr != nil {
			log.WithError(writeErr).Error("failed to write ready response")
		}
	})
}
