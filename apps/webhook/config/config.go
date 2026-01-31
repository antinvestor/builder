package config

import (
	"github.com/pitabwire/frame/config"
)

// WebhookConfig defines configuration for the webhook service.
// The webhook service receives GitHub events and publishes them
// to the message queue for processing by workers.
type WebhookConfig struct {
	config.ConfigurationDefault

	// ==========================================================================
	// GitHub Configuration
	// ==========================================================================

	// GitHubWebhookSecret is the secret used to verify GitHub webhook payloads.
	GitHubWebhookSecret string `env:"GITHUB_WEBHOOK_SECRET"`

	// GitHubAppID is the GitHub App ID for authentication.
	GitHubAppID int64 `envDefault:"0" env:"GITHUB_APP_ID"`

	// GitHubAppPrivateKey is the path to the GitHub App private key file.
	GitHubAppPrivateKey string `env:"GITHUB_APP_PRIVATE_KEY_PATH"`

	// GitHubAppInstallationID is the installation ID for the GitHub App.
	GitHubAppInstallationID int64 `envDefault:"0" env:"GITHUB_APP_INSTALLATION_ID"`

	// ==========================================================================
	// Feature Request Queue (outgoing to workers)
	// ==========================================================================

	// QueueFeatureRequestName is the name of the feature request queue.
	QueueFeatureRequestName string `envDefault:"feature.requests" env:"QUEUE_FEATURE_REQUEST_NAME"`

	// QueueFeatureRequestURI is the URI of the feature request queue.
	QueueFeatureRequestURI string `envDefault:"mem://feature.requests" env:"QUEUE_FEATURE_REQUEST_URI"`

	// ==========================================================================
	// GitHub Event Queue (for state change notifications)
	// ==========================================================================

	// QueueGitHubEventName is the name of the GitHub event queue.
	QueueGitHubEventName string `envDefault:"github.events" env:"QUEUE_GITHUB_EVENT_NAME"`

	// QueueGitHubEventURI is the URI of the GitHub event queue.
	QueueGitHubEventURI string `envDefault:"mem://github.events" env:"QUEUE_GITHUB_EVENT_URI"`

	// ==========================================================================
	// Webhook Processing
	// ==========================================================================

	// AllowedRepositories is a comma-separated list of allowed repositories (owner/repo format).
	// If empty, all repositories are allowed.
	AllowedRepositories string `env:"ALLOWED_REPOSITORIES"`

	// EnableIssueProcessing enables processing of issue events.
	EnableIssueProcessing bool `envDefault:"true" env:"ENABLE_ISSUE_PROCESSING"`

	// EnablePRProcessing enables processing of pull request events.
	EnablePRProcessing bool `envDefault:"true" env:"ENABLE_PR_PROCESSING"`

	// EnablePushProcessing enables processing of push events.
	EnablePushProcessing bool `envDefault:"false" env:"ENABLE_PUSH_PROCESSING"`

	// AutoTriggerLabel is the label that triggers automatic feature processing.
	AutoTriggerLabel string `envDefault:"auto-build" env:"AUTO_TRIGGER_LABEL"`

	// RequiredLabels are labels that must be present for processing (comma-separated).
	RequiredLabels string `env:"REQUIRED_LABELS"`
}
