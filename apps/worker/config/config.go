package config

import (
	"github.com/pitabwire/frame/config"

	"github.com/antinvestor/builder/internal/events"
)

// WorkerConfig defines configuration for the worker service.
// The worker handles the main feature processing pipeline:
// repository checkout, planning, and patch generation.
type WorkerConfig struct {
	config.ConfigurationDefault

	// ==========================================================================
	// LLM Provider Configuration
	// ==========================================================================

	// AnthropicAPIKey is the API key for Anthropic Claude.
	AnthropicAPIKey string `env:"ANTHROPIC_API_KEY"`

	// OpenAIAPIKey is the API key for OpenAI.
	OpenAIAPIKey string `env:"OPENAI_API_KEY"`

	// GoogleAPIKey is the API key for Google AI.
	GoogleAPIKey string `env:"GOOGLE_API_KEY"`

	// DefaultLLMProvider is the default LLM provider.
	DefaultLLMProvider string `envDefault:"anthropic" env:"DEFAULT_LLM_PROVIDER"`

	// LLMTimeoutSeconds is the timeout for LLM requests.
	LLMTimeoutSeconds int `envDefault:"120" env:"LLM_TIMEOUT_SECONDS"`

	// LLMMaxRetries is the maximum retries for LLM requests.
	LLMMaxRetries int `envDefault:"3" env:"LLM_MAX_RETRIES"`

	// ==========================================================================
	// Repository Configuration
	// ==========================================================================

	// WorkspaceBasePath is the base path for repository workspaces.
	WorkspaceBasePath string `envDefault:"/var/lib/feature-service/workspaces" env:"WORKSPACE_BASE_PATH"`

	// MaxConcurrentClones is the maximum concurrent repository clones.
	MaxConcurrentClones int `envDefault:"10" env:"MAX_CONCURRENT_CLONES"`

	// CloneTimeoutSeconds is the timeout for repository cloning.
	CloneTimeoutSeconds int `envDefault:"300" env:"CLONE_TIMEOUT_SECONDS"`

	// MaxWorkspaceAgeHours is the maximum workspace age before cleanup.
	MaxWorkspaceAgeHours int `envDefault:"24" env:"MAX_WORKSPACE_AGE_HOURS"`

	// ==========================================================================
	// Git Authentication
	// ==========================================================================

	// GitSSHKeyPath is the path to SSH private key.
	GitSSHKeyPath string `env:"GIT_SSH_KEY_PATH"`

	// GitHTTPSUsername is the username for HTTPS authentication.
	GitHTTPSUsername string `env:"GIT_HTTPS_USERNAME"`

	// GitHTTPSPassword is the password/token for HTTPS authentication.
	GitHTTPSPassword string `env:"GIT_HTTPS_PASSWORD"`

	// ==========================================================================
	// Queue Configuration
	// ==========================================================================

	// Feature request queue (incoming)
	QueueFeatureRequestName string `envDefault:"feature.requests" env:"QUEUE_FEATURE_REQUEST_NAME"`
	QueueFeatureRequestURI  string `envDefault:"mem://feature.requests" env:"QUEUE_FEATURE_REQUEST_URI"`

	// Feature result queue (outgoing)
	QueueFeatureResultName string `envDefault:"feature.results" env:"QUEUE_FEATURE_RESULT_NAME"`
	QueueFeatureResultURI  string `envDefault:"mem://feature.results" env:"QUEUE_FEATURE_RESULT_URI"`

	// Internal events queue
	QueueInternalEventsName string `envDefault:"feature.events" env:"QUEUE_INTERNAL_EVENTS_NAME"`
	QueueInternalEventsURI  string `envDefault:"mem://feature.events" env:"QUEUE_INTERNAL_EVENTS_URI"`

	// Review queue (to reviewer service)
	QueueReviewRequestName string `envDefault:"feature.review.requests" env:"QUEUE_REVIEW_REQUEST_NAME"`
	QueueReviewRequestURI  string `envDefault:"mem://feature.review.requests" env:"QUEUE_REVIEW_REQUEST_URI"`

	// Execution queue (to executor service)
	QueueExecutionRequestName string `envDefault:"feature.execution.requests" env:"QUEUE_EXECUTION_REQUEST_NAME"`
	QueueExecutionRequestURI  string `envDefault:"mem://feature.execution.requests" env:"QUEUE_EXECUTION_REQUEST_URI"`

	// Retry queues
	QueueRetryLevel1Name string `envDefault:"feature.events.retry.1" env:"QUEUE_RETRY_L1_NAME"`
	QueueRetryLevel1URI  string `envDefault:"mem://feature.events.retry.1" env:"QUEUE_RETRY_L1_URI"`
	QueueRetryLevel2Name string `envDefault:"feature.events.retry.2" env:"QUEUE_RETRY_L2_NAME"`
	QueueRetryLevel2URI  string `envDefault:"mem://feature.events.retry.2" env:"QUEUE_RETRY_L2_URI"`
	QueueRetryLevel3Name string `envDefault:"feature.events.retry.3" env:"QUEUE_RETRY_L3_NAME"`
	QueueRetryLevel3URI  string `envDefault:"mem://feature.events.retry.3" env:"QUEUE_RETRY_L3_URI"`

	// Dead letter queue
	QueueDLQName string `envDefault:"feature.events.dlq" env:"QUEUE_DLQ_NAME"`
	QueueDLQURI  string `envDefault:"mem://feature.events.dlq" env:"QUEUE_DLQ_URI"`

	// ==========================================================================
	// Execution Limits
	// ==========================================================================

	// MaxConcurrentExecutions is the maximum concurrent feature executions.
	MaxConcurrentExecutions int `envDefault:"50" env:"MAX_CONCURRENT_EXECUTIONS"`

	// MaxStepsPerExecution is the maximum steps per execution.
	MaxStepsPerExecution int `envDefault:"100" env:"MAX_STEPS_PER_EXECUTION"`

	// MaxRetriesPerStep is the maximum retries per step.
	MaxRetriesPerStep int `envDefault:"5" env:"MAX_RETRIES_PER_STEP"`

	// StepTimeoutMinutes is the timeout for a single step.
	StepTimeoutMinutes int `envDefault:"30" env:"STEP_TIMEOUT_MINUTES"`

	// ExecutionTimeoutHours is the timeout for entire execution.
	ExecutionTimeoutHours int `envDefault:"8" env:"EXECUTION_TIMEOUT_HOURS"`

	// ==========================================================================
	// Review Thresholds (for delegating to reviewer)
	// ==========================================================================

	// ReviewThresholds contains review thresholds.
	ReviewThresholds events.ReviewThresholds `json:"review_thresholds"`
}
