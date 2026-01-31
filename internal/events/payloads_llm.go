package events

import "time"

// ===== LLM INVOCATION =====

// LLMInvocationStartedPayload is the payload for LLMInvocationStarted.
type LLMInvocationStartedPayload struct {
	// InvocationID uniquely identifies this LLM call.
	InvocationID string `json:"invocation_id"`

	// Provider is the LLM provider.
	Provider LLMProvider `json:"provider"`

	// Model is the model being invoked.
	Model string `json:"model"`

	// Function is the BAML function being called.
	Function string `json:"function"`

	// Purpose describes why this call is being made.
	Purpose LLMPurpose `json:"purpose"`

	// EstimatedInputTokens is estimated input tokens.
	EstimatedInputTokens int `json:"estimated_input_tokens"`

	// MaxOutputTokens is the max output tokens requested.
	MaxOutputTokens int `json:"max_output_tokens"`

	// Temperature is the sampling temperature.
	Temperature float64 `json:"temperature"`

	// StepNumber is the step number (if applicable).
	StepNumber int `json:"step_number,omitempty"`

	// StartedAt is when invocation started.
	StartedAt time.Time `json:"started_at"`
}

// LLMProvider identifies the LLM provider.
type LLMProvider string

const (
	LLMProviderAnthropic LLMProvider = "anthropic"
	LLMProviderOpenAI    LLMProvider = "openai"
	LLMProviderGoogle    LLMProvider = "google"
	LLMProviderMistral   LLMProvider = "mistral"
	LLMProviderLocal     LLMProvider = "local"
)

// LLMPurpose categorizes LLM invocation purposes.
type LLMPurpose string

const (
	LLMPurposeNormalization   LLMPurpose = "normalization"    // Specification normalization
	LLMPurposeImpactAnalysis  LLMPurpose = "impact_analysis"  // Impact analysis
	LLMPurposePlanning        LLMPurpose = "planning"         // Plan generation
	LLMPurposeCodeGeneration  LLMPurpose = "code_generation"  // Code generation
	LLMPurposeTestGeneration  LLMPurpose = "test_generation"  // Test generation
	LLMPurposeCodeReview      LLMPurpose = "code_review"      // Code review
	LLMPurposeIteration       LLMPurpose = "iteration"        // Iteration/fix
	LLMPurposeCommitMessage   LLMPurpose = "commit_message"   // Commit message generation
)

// LLMInvocationCompletedPayload is the payload for LLMInvocationCompleted.
type LLMInvocationCompletedPayload struct {
	// InvocationID matches the started event.
	InvocationID string `json:"invocation_id"`

	// Provider is the LLM provider.
	Provider LLMProvider `json:"provider"`

	// Model is the model that was invoked.
	Model string `json:"model"`

	// Function is the BAML function called.
	Function string `json:"function"`

	// Purpose is the invocation purpose.
	Purpose LLMPurpose `json:"purpose"`

	// Usage contains token usage.
	Usage LLMUsage `json:"usage"`

	// LatencyMS is total latency.
	LatencyMS int64 `json:"latency_ms"`

	// TimeToFirstTokenMS is time to first token (streaming).
	TimeToFirstTokenMS int64 `json:"time_to_first_token_ms,omitempty"`

	// StopReason is why generation stopped.
	StopReason LLMStopReason `json:"stop_reason"`

	// ValidationPassed indicates if output validation passed.
	ValidationPassed bool `json:"validation_passed"`

	// ValidationErrors are validation errors if any.
	ValidationErrors []string `json:"validation_errors,omitempty"`

	// CacheHit indicates if response was cached.
	CacheHit bool `json:"cache_hit"`

	// RequestID is the provider's request ID.
	RequestID string `json:"request_id,omitempty"`

	// CompletedAt is when invocation completed.
	CompletedAt time.Time `json:"completed_at"`
}

// LLMUsage contains token usage statistics.
type LLMUsage struct {
	// InputTokens is input tokens used.
	InputTokens int `json:"input_tokens"`

	// OutputTokens is output tokens generated.
	OutputTokens int `json:"output_tokens"`

	// TotalTokens is total tokens.
	TotalTokens int `json:"total_tokens"`

	// CacheReadTokens is tokens read from cache.
	CacheReadTokens int `json:"cache_read_tokens,omitempty"`

	// CacheWriteTokens is tokens written to cache.
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`

	// CostUSD is estimated cost in USD.
	CostUSD float64 `json:"cost_usd,omitempty"`
}

// LLMStopReason indicates why generation stopped.
type LLMStopReason string

const (
	LLMStopReasonEndTurn    LLMStopReason = "end_turn"    // Natural completion
	LLMStopReasonMaxTokens  LLMStopReason = "max_tokens"  // Hit token limit
	LLMStopReasonStopSequence LLMStopReason = "stop_sequence" // Hit stop sequence
	LLMStopReasonToolUse    LLMStopReason = "tool_use"    // Tool use requested
	LLMStopReasonContentFilter LLMStopReason = "content_filter" // Content filter triggered
)

// LLMInvocationFailedPayload is the payload for LLMInvocationFailed.
type LLMInvocationFailedPayload struct {
	// InvocationID matches the started event.
	InvocationID string `json:"invocation_id"`

	// Provider is the LLM provider.
	Provider LLMProvider `json:"provider"`

	// Model is the model that was invoked.
	Model string `json:"model"`

	// Function is the BAML function called.
	Function string `json:"function"`

	// Purpose is the invocation purpose.
	Purpose LLMPurpose `json:"purpose"`

	// ErrorCode categorizes the error.
	ErrorCode LLMErrorCode `json:"error_code"`

	// ErrorMessage is the error message.
	ErrorMessage string `json:"error_message"`

	// Retryable indicates if the error is retryable.
	Retryable bool `json:"retryable"`

	// RetryAfterMS is suggested retry delay.
	RetryAfterMS int64 `json:"retry_after_ms,omitempty"`

	// PartialUsage is token usage before failure.
	PartialUsage *LLMUsage `json:"partial_usage,omitempty"`

	// RequestID is the provider's request ID.
	RequestID string `json:"request_id,omitempty"`

	// ErrorContext provides additional context.
	ErrorContext map[string]string `json:"error_context,omitempty"`

	// FailedAt is when invocation failed.
	FailedAt time.Time `json:"failed_at"`
}

// LLMErrorCode categorizes LLM errors.
type LLMErrorCode string

const (
	LLMErrorRateLimit      LLMErrorCode = "rate_limit"      // Rate limited
	LLMErrorQuota          LLMErrorCode = "quota"           // Quota exceeded
	LLMErrorOverloaded     LLMErrorCode = "overloaded"      // Service overloaded
	LLMErrorTimeout        LLMErrorCode = "timeout"         // Request timed out
	LLMErrorContextLength  LLMErrorCode = "context_length"  // Input too long
	LLMErrorInvalidRequest LLMErrorCode = "invalid_request" // Invalid request
	LLMErrorAuth           LLMErrorCode = "auth"            // Authentication failed
	LLMErrorNetwork        LLMErrorCode = "network"         // Network error
	LLMErrorContentFilter  LLMErrorCode = "content_filter"  // Content policy violation
	LLMErrorServerError    LLMErrorCode = "server_error"    // Provider server error
	LLMErrorParsing        LLMErrorCode = "parsing"         // Failed to parse response
	LLMErrorValidation     LLMErrorCode = "validation"      // Output validation failed
)

// ===== LLM AGGREGATED METRICS =====

// LLMMetricsSnapshot contains aggregated LLM metrics for a feature execution.
type LLMMetricsSnapshot struct {
	// TotalInvocations is total LLM calls made.
	TotalInvocations int `json:"total_invocations"`

	// SuccessfulInvocations is successful calls.
	SuccessfulInvocations int `json:"successful_invocations"`

	// FailedInvocations is failed calls.
	FailedInvocations int `json:"failed_invocations"`

	// TotalInputTokens is total input tokens.
	TotalInputTokens int `json:"total_input_tokens"`

	// TotalOutputTokens is total output tokens.
	TotalOutputTokens int `json:"total_output_tokens"`

	// TotalLatencyMS is cumulative latency.
	TotalLatencyMS int64 `json:"total_latency_ms"`

	// AverageLatencyMS is average latency per call.
	AverageLatencyMS int64 `json:"average_latency_ms"`

	// TotalCostUSD is estimated total cost.
	TotalCostUSD float64 `json:"total_cost_usd"`

	// ByPurpose breaks down metrics by purpose.
	ByPurpose map[LLMPurpose]LLMPurposeMetrics `json:"by_purpose,omitempty"`

	// ByModel breaks down metrics by model.
	ByModel map[string]LLMModelMetrics `json:"by_model,omitempty"`
}

// LLMPurposeMetrics contains metrics for a specific purpose.
type LLMPurposeMetrics struct {
	Invocations  int     `json:"invocations"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	LatencyMS    int64   `json:"latency_ms"`
	CostUSD      float64 `json:"cost_usd"`
}

// LLMModelMetrics contains metrics for a specific model.
type LLMModelMetrics struct {
	Invocations  int     `json:"invocations"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	LatencyMS    int64   `json:"latency_ms"`
	CostUSD      float64 `json:"cost_usd"`
}
