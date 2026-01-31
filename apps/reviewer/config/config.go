package config

import (
	"github.com/pitabwire/frame/config"

	"service-feature/internal/events"
)

// ReviewerConfig defines configuration for the reviewer service.
// The reviewer handles security analysis, architecture review,
// risk scoring, and control decisions (iterate/abort/complete).
type ReviewerConfig struct {
	config.ConfigurationDefault

	// ==========================================================================
	// Queue Configuration
	// ==========================================================================

	// Review request queue (incoming from worker)
	QueueReviewRequestName string `envDefault:"feature.review.requests" env:"QUEUE_REVIEW_REQUEST_NAME"`
	QueueReviewRequestURI  string `envDefault:"mem://feature.review.requests" env:"QUEUE_REVIEW_REQUEST_URI"`

	// Review result queue (outgoing to worker)
	QueueReviewResultName string `envDefault:"feature.review.results" env:"QUEUE_REVIEW_RESULT_NAME"`
	QueueReviewResultURI  string `envDefault:"mem://feature.review.results" env:"QUEUE_REVIEW_RESULT_URI"`

	// Control events queue (iteration/abort/complete commands)
	QueueControlEventsName string `envDefault:"feature.control" env:"QUEUE_CONTROL_EVENTS_NAME"`
	QueueControlEventsURI  string `envDefault:"mem://feature.control" env:"QUEUE_CONTROL_EVENTS_URI"`

	// ==========================================================================
	// Review Thresholds
	// ==========================================================================

	// ReviewThresholds contains the thresholds for review decisions.
	ReviewThresholds events.ReviewThresholds `json:"review_thresholds"`

	// MaxRiskScore is the maximum acceptable overall risk score.
	MaxRiskScore int `envDefault:"50" env:"MAX_RISK_SCORE"`

	// MaxSecurityRiskScore is the maximum acceptable security risk score.
	MaxSecurityRiskScore int `envDefault:"30" env:"MAX_SECURITY_RISK_SCORE"`

	// MaxArchitectureRiskScore is the maximum acceptable architecture risk score.
	MaxArchitectureRiskScore int `envDefault:"40" env:"MAX_ARCHITECTURE_RISK_SCORE"`

	// MaxCriticalIssues is the maximum critical issues allowed (0 = zero tolerance).
	MaxCriticalIssues int `envDefault:"0" env:"MAX_CRITICAL_ISSUES"`

	// MaxHighIssues is the maximum high severity issues allowed.
	MaxHighIssues int `envDefault:"2" env:"MAX_HIGH_ISSUES"`

	// MaxBreakingChanges is the maximum breaking changes allowed (0 = zero tolerance).
	MaxBreakingChanges int `envDefault:"0" env:"MAX_BREAKING_CHANGES"`

	// MaxIterations is the maximum iterations before abort.
	MaxIterations int `envDefault:"3" env:"MAX_ITERATIONS"`

	// ==========================================================================
	// Security Configuration
	// ==========================================================================

	// RequireSecurityApproval requires security approval for all changes.
	RequireSecurityApproval bool `envDefault:"true" env:"REQUIRE_SECURITY_APPROVAL"`

	// BlockOnSecrets blocks any code containing detected secrets.
	BlockOnSecrets bool `envDefault:"true" env:"BLOCK_ON_SECRETS"`

	// AllowBreakingChanges allows breaking changes (not recommended).
	AllowBreakingChanges bool `envDefault:"false" env:"ALLOW_BREAKING_CHANGES"`

	// ==========================================================================
	// Kill Switch Configuration
	// ==========================================================================

	// KillSwitchEnabled enables the kill switch system.
	KillSwitchEnabled bool `envDefault:"true" env:"KILL_SWITCH_ENABLED"`

	// ErrorRateThreshold is the error rate threshold for auto-triggering.
	ErrorRateThreshold float64 `envDefault:"0.5" env:"ERROR_RATE_THRESHOLD"`

	// MaxConsecutiveFailures is max consecutive failures before kill switch.
	MaxConsecutiveFailures int `envDefault:"5" env:"MAX_CONSECUTIVE_FAILURES"`

	// ResourceUsageThreshold is resource usage threshold for kill switch.
	ResourceUsageThreshold float64 `envDefault:"0.9" env:"RESOURCE_USAGE_THRESHOLD"`
}

// GetReviewThresholds returns the configured review thresholds.
func (c *ReviewerConfig) GetReviewThresholds() events.ReviewThresholds {
	if c.ReviewThresholds.MaxRiskScore == 0 {
		return events.ReviewThresholds{
			MaxRiskScore:             c.MaxRiskScore,
			MaxSecurityRiskScore:     c.MaxSecurityRiskScore,
			MaxArchitectureRiskScore: c.MaxArchitectureRiskScore,
			MaxCriticalIssues:        c.MaxCriticalIssues,
			MaxHighIssues:            c.MaxHighIssues,
			MaxBreakingChanges:       c.MaxBreakingChanges,
			MaxIterations:            c.MaxIterations,
		}
	}
	return c.ReviewThresholds
}
