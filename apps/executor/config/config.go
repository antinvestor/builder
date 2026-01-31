package config

import (
	"github.com/pitabwire/frame/config"
)

// ExecutorConfig defines configuration for the executor service.
// The executor handles sandboxed test execution with isolation.
// This service requires special permissions (Docker socket access).
type ExecutorConfig struct {
	config.ConfigurationDefault

	// ==========================================================================
	// Queue Configuration
	// ==========================================================================

	// Execution request queue (incoming from worker)
	QueueExecutionRequestName string `envDefault:"feature.execution.requests" env:"QUEUE_EXECUTION_REQUEST_NAME"`
	QueueExecutionRequestURI  string `envDefault:"mem://feature.execution.requests" env:"QUEUE_EXECUTION_REQUEST_URI"`

	// Execution result queue (outgoing to worker)
	QueueExecutionResultName string `envDefault:"feature.execution.results" env:"QUEUE_EXECUTION_RESULT_NAME"`
	QueueExecutionResultURI  string `envDefault:"mem://feature.execution.results" env:"QUEUE_EXECUTION_RESULT_URI"`

	// ==========================================================================
	// Sandbox Configuration
	// ==========================================================================

	// SandboxEnabled enables sandbox isolation.
	SandboxEnabled bool `envDefault:"true" env:"SANDBOX_ENABLED"`

	// SandboxType is the sandbox type: docker, gvisor, firecracker.
	SandboxType string `envDefault:"docker" env:"SANDBOX_TYPE"`

	// SandboxImage is the container image for sandbox execution.
	SandboxImage string `envDefault:"feature-sandbox:latest" env:"SANDBOX_IMAGE"`

	// SandboxMemoryLimitMB is the memory limit in MB.
	SandboxMemoryLimitMB int `envDefault:"2048" env:"SANDBOX_MEMORY_LIMIT_MB"`

	// SandboxCPULimit is the CPU limit.
	SandboxCPULimit float64 `envDefault:"2.0" env:"SANDBOX_CPU_LIMIT"`

	// SandboxNetworkEnabled enables network in sandbox.
	SandboxNetworkEnabled bool `envDefault:"false" env:"SANDBOX_NETWORK_ENABLED"`

	// SandboxTimeoutSeconds is the execution timeout.
	SandboxTimeoutSeconds int `envDefault:"300" env:"SANDBOX_TIMEOUT_SECONDS"`

	// ==========================================================================
	// Concurrency
	// ==========================================================================

	// MaxConcurrentExecutions is the maximum concurrent executions.
	MaxConcurrentExecutions int `envDefault:"10" env:"MAX_CONCURRENT_EXECUTIONS"`

	// ==========================================================================
	// Workspace Configuration
	// ==========================================================================

	// WorkspaceBasePath is where workspaces are mounted.
	WorkspaceBasePath string `envDefault:"/var/lib/feature-service/workspaces" env:"WORKSPACE_BASE_PATH"`

	// ==========================================================================
	// Test Runner Configuration
	// ==========================================================================

	// DefaultTestTimeout is the default test timeout in seconds.
	DefaultTestTimeout int `envDefault:"300" env:"DEFAULT_TEST_TIMEOUT"`

	// CoverageEnabled enables coverage collection.
	CoverageEnabled bool `envDefault:"true" env:"COVERAGE_ENABLED"`

	// CoverageThreshold is the minimum coverage percentage.
	CoverageThreshold float64 `envDefault:"70.0" env:"COVERAGE_THRESHOLD"`
}
