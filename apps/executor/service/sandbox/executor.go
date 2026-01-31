package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	appconfig "github.com/antinvestor/builder/apps/executor/config"
	"github.com/antinvestor/builder/internal/events"
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
	headers map[string]string,
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
	testResult, err := h.runner.ParseResults(result.Output, result.ExitCode)
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

func (h *ExecutionRequestHandler) emitSuccess(ctx context.Context, executionID events.ExecutionID, result *events.TestResult) error {
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
type SandboxExecutor struct {
	cfg         *appconfig.ExecutorConfig
	activeCount int32
	mu          sync.RWMutex
}

// NewSandboxExecutor creates a new sandbox executor.
func NewSandboxExecutor(cfg *appconfig.ExecutorConfig) *SandboxExecutor {
	return &SandboxExecutor{
		cfg: cfg,
	}
}

// SandboxExecutionRequest contains execution request data.
type SandboxExecutionRequest struct {
	ExecutionID events.ExecutionID
	Language    string
	TestFiles   []string
	Config      *appconfig.ExecutorConfig
}

// SandboxExecutionResult contains execution result data.
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
	if atomic.LoadInt32(&e.activeCount) > int32(e.cfg.MaxConcurrentExecutions) {
		return nil, fmt.Errorf("max concurrent executions reached")
	}

	// Stub implementation - would execute in Docker/gVisor/Firecracker
	return &SandboxExecutionResult{
		Output:   "Tests executed successfully",
		ExitCode: 0,
		Duration: 1000,
	}, nil
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
func (r *MultiRunner) ParseResults(output string, exitCode int) (*events.TestResult, error) {
	// Stub implementation
	success := exitCode == 0
	return &events.TestResult{
		TotalTests:   1,
		PassedTests:  1,
		FailedTests:  0,
		SkippedTests: 0,
		Success:      success,
		DurationMs:   1000,
		TestCases:    []events.TestCaseResult{},
	}, nil
}
