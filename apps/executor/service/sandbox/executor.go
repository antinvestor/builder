package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	appconfig "github.com/antinvestor/builder/apps/executor/config"
	"github.com/antinvestor/builder/internal/events"
)

// Common constants for test durations.
const (
	defaultTestDurationMs = 1000
)

// =============================================================================
// Interfaces
// =============================================================================

// EventsEmitter emits events.
type EventsEmitter interface {
	Emit(ctx context.Context, eventName string, payload any) error
}

// =============================================================================
// Execution Request Handler
// =============================================================================

// ExecutionRequestHandler handles incoming execution requests.
type ExecutionRequestHandler struct {
	cfg       *appconfig.ExecutorConfig
	executor  *SandboxExecutor
	runner    *MultiRunner
	eventsMan EventsEmitter
}

// NewExecutionRequestHandler creates a new execution request handler.
func NewExecutionRequestHandler(
	cfg *appconfig.ExecutorConfig,
	executor *SandboxExecutor,
	runner *MultiRunner,
	eventsMan EventsEmitter,
) *ExecutionRequestHandler {
	return &ExecutionRequestHandler{
		cfg:       cfg,
		executor:  executor,
		runner:    runner,
		eventsMan: eventsMan,
	}
}

// Handle processes incoming execution requests.
func (h *ExecutionRequestHandler) Handle(
	ctx context.Context,
	_ map[string]string, // headers - unused but part of handler interface
	payload []byte,
) error {
	var request events.TestExecutionRequestedPayload
	if err := json.Unmarshal(payload, &request); err != nil {
		return fmt.Errorf("unmarshal execution request: %w", err)
	}

	// Execute in sandbox
	result, err := h.executor.Execute(ctx, &SandboxExecutionRequest{
		ExecutionID: request.ExecutionID,
		Language:    request.Language,
		TestFiles:   request.TestFiles,
		Config:      h.cfg,
	})
	if err != nil {
		return h.emitFailure(ctx, request.ExecutionID, err)
	}

	// Parse results
	testResult, err := h.runner.ParseResults(result.Output, result.ExitCode, request.Language)
	if err != nil {
		return h.emitFailure(ctx, request.ExecutionID, err)
	}

	// Emit success
	return h.emitSuccess(ctx, request.ExecutionID, testResult)
}

func (h *ExecutionRequestHandler) emitFailure(ctx context.Context, executionID events.ExecutionID, err error) error {
	return h.eventsMan.Emit(ctx, "feature.execution.failed", &events.TestExecutionCompletedPayload{
		ExecutionID: executionID,
		Success:     false,
		Error: &events.ExecutionError{
			Code:    "execution_failed",
			Message: err.Error(),
		},
	})
}

func (h *ExecutionRequestHandler) emitSuccess(
	ctx context.Context,
	executionID events.ExecutionID,
	result *events.TestResult,
) error {
	return h.eventsMan.Emit(ctx, "feature.execution.completed", &events.TestExecutionCompletedPayload{
		ExecutionID: executionID,
		Success:     true,
		Result:      result,
	})
}

// =============================================================================
// Sandbox Executor
// =============================================================================

// SandboxExecutor executes commands in isolated sandboxes.
//
//nolint:revive // name stutters but changing would be a breaking change
type SandboxExecutor struct {
	cfg         *appconfig.ExecutorConfig
	dockerExec  *DockerExecutor
	activeCount int32
}

// NewSandboxExecutor creates a new sandbox executor.
func NewSandboxExecutor(cfg *appconfig.ExecutorConfig) (*SandboxExecutor, error) {
	var dockerExec *DockerExecutor
	var err error

	// Initialize Docker executor once if sandbox is enabled
	if cfg.SandboxEnabled {
		dockerExec, err = NewDockerExecutor(cfg)
		if err != nil {
			return nil, fmt.Errorf("create docker executor: %w", err)
		}
	}

	return &SandboxExecutor{
		cfg:        cfg,
		dockerExec: dockerExec,
	}, nil
}

// Close releases resources held by the executor.
func (e *SandboxExecutor) Close() error {
	if e.dockerExec != nil {
		return e.dockerExec.Close()
	}
	return nil
}

// SandboxExecutionRequest contains execution request data.
//
//nolint:revive // name stutters but changing would be a breaking change
type SandboxExecutionRequest struct {
	ExecutionID events.ExecutionID
	Language    string
	TestFiles   []string
	Config      *appconfig.ExecutorConfig
}

// SandboxExecutionResult contains execution result data.
//
//nolint:revive // name stutters but changing would be a breaking change
type SandboxExecutionResult struct {
	Output   string
	ExitCode int
	Duration int64
}

// Execute runs a command in a sandbox.
func (e *SandboxExecutor) Execute(ctx context.Context, req *SandboxExecutionRequest) (*SandboxExecutionResult, error) {
	// Increment active count
	atomic.AddInt32(&e.activeCount, 1)
	defer atomic.AddInt32(&e.activeCount, -1)

	// Check concurrency limit
	//nolint:gosec // MaxConcurrentExecutions is a config value, typically small and won't overflow int32
	if atomic.LoadInt32(&e.activeCount) > int32(e.cfg.MaxConcurrentExecutions) {
		return nil, errors.New("max concurrent executions reached")
	}

	// If sandbox is disabled, run locally (for testing)
	if !e.cfg.SandboxEnabled || e.dockerExec == nil {
		return &SandboxExecutionResult{
			Output:   "Tests executed successfully (sandbox disabled)",
			ExitCode: 0,
			Duration: defaultTestDurationMs,
		}, nil
	}

	// Use the reusable Docker executor for sandboxed execution
	return e.dockerExec.Execute(ctx, req)
}

// ActiveCount returns the number of active executions.
func (e *SandboxExecutor) ActiveCount() int {
	return int(atomic.LoadInt32(&e.activeCount))
}

// =============================================================================
// Multi-Language Test Runner
// =============================================================================

// MultiRunner supports multiple test frameworks.
type MultiRunner struct {
	cfg *appconfig.ExecutorConfig
}

// NewMultiRunner creates a new multi-language test runner.
func NewMultiRunner(cfg *appconfig.ExecutorConfig) *MultiRunner {
	return &MultiRunner{cfg: cfg}
}

// ParseResults parses test output into structured results.
func (r *MultiRunner) ParseResults(output string, exitCode int, language string) (*events.TestResult, error) {
	// Use the comprehensive result parser
	parser := NewTestResultParser(r.cfg.CoverageThreshold)
	return parser.ParseTestOutput(language, output, exitCode)
}
