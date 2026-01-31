package config

import (
	"github.com/pitabwire/frame/config"
)

// GatewayConfig defines configuration for the gateway service.
// The gateway is a lightweight HTTP API that receives feature requests
// and publishes them to the message queue for processing.
type GatewayConfig struct {
	config.ConfigurationDefault

	// ==========================================================================
	// Feature Request Queue (outgoing to workers)
	// ==========================================================================

	// QueueFeatureRequestName is the name of the feature request queue.
	QueueFeatureRequestName string `envDefault:"feature.requests" env:"QUEUE_FEATURE_REQUEST_NAME"`

	// QueueFeatureRequestURI is the URI of the feature request queue.
	QueueFeatureRequestURI string `envDefault:"mem://feature.requests" env:"QUEUE_FEATURE_REQUEST_URI"`

	// ==========================================================================
	// Feature Result Queue (incoming from workers)
	// ==========================================================================

	// QueueFeatureResultName is the name of the feature result queue.
	QueueFeatureResultName string `envDefault:"feature.results" env:"QUEUE_FEATURE_RESULT_NAME"`

	// QueueFeatureResultURI is the URI of the feature result queue.
	QueueFeatureResultURI string `envDefault:"mem://feature.results" env:"QUEUE_FEATURE_RESULT_URI"`

	// ==========================================================================
	// Rate Limiting
	// ==========================================================================

	// RateLimitRequestsPerMinute limits requests per minute per client.
	RateLimitRequestsPerMinute int `envDefault:"60" env:"RATE_LIMIT_REQUESTS_PER_MINUTE"`

	// RateLimitBurstSize is the burst size for rate limiting.
	RateLimitBurstSize int `envDefault:"10" env:"RATE_LIMIT_BURST_SIZE"`

	// ==========================================================================
	// Request Validation
	// ==========================================================================

	// MaxSpecificationSize is the maximum size of a feature specification in bytes.
	MaxSpecificationSize int `envDefault:"1048576" env:"MAX_SPECIFICATION_SIZE"` // 1MB

	// AllowedRepositoryHosts are the allowed Git hosts for repositories.
	AllowedRepositoryHosts string `envDefault:"github.com,gitlab.com,bitbucket.org" env:"ALLOWED_REPOSITORY_HOSTS"`
}
