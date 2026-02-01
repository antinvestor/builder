package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pitabwire/util"
)

// Common errors.
var (
	ErrNoAPIKey           = errors.New("no API key configured")
	ErrRateLimited        = errors.New("rate limited")
	ErrQuotaExceeded      = errors.New("quota exceeded")
	ErrContextTooLong     = errors.New("context too long")
	ErrInvalidResponse    = errors.New("invalid response from LLM")
	ErrAllProvidersFailed = errors.New("all providers failed")
)

// Client is the interface for LLM operations.
type Client interface {
	// NormalizeSpec normalizes a feature specification.
	NormalizeSpec(
		ctx context.Context,
		input NormalizeSpecInput,
	) (*NormalizedSpecification, *InvocationResult, error)

	// AnalyzeImpact analyzes the impact of changes.
	AnalyzeImpact(
		ctx context.Context,
		input AnalyzeImpactInput,
	) (*ImpactAnalysisResult, *InvocationResult, error)

	// GeneratePlan generates an implementation plan.
	GeneratePlan(
		ctx context.Context,
		input GeneratePlanInput,
	) (*ImplementationPlan, *InvocationResult, error)

	// GenerateCode generates code for a plan step.
	GenerateCode(
		ctx context.Context,
		input GenerateCodeInput,
	) (*CodeGenerationResult, *InvocationResult, error)

	// GetUsage returns cumulative usage statistics.
	GetUsage() Usage
}

// ProviderClient is the interface for a single LLM provider.
type ProviderClient interface {
	// Complete sends a completion request and returns the response.
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)

	// Provider returns the provider identifier.
	Provider() Provider

	// IsAvailable returns true if the provider is configured.
	IsAvailable() bool
}

// CompletionRequest is a request to the LLM.
type CompletionRequest struct {
	Model          Model
	SystemPrompt   string
	UserPrompt     string
	MaxTokens      int
	Temperature    float64
	ResponseFormat string // "json" or "text"
	Function       Function
	Purpose        Purpose
}

// CompletionResponse is a response from the LLM.
type CompletionResponse struct {
	Content    string
	Usage      Usage
	StopReason string
	RequestID  string
	LatencyMS  int64
	CacheHit   bool
}

// MultiProviderClient implements Client with fallback support.
type MultiProviderClient struct {
	providers     []ProviderClient
	promptBuilder *PromptBuilder
	config        ClientConfig
	totalUsage    Usage
}

// NewMultiProviderClient creates a new multi-provider client.
func NewMultiProviderClient(cfg ClientConfig) (*MultiProviderClient, error) {
	pb, err := NewPromptBuilder()
	if err != nil {
		return nil, fmt.Errorf("create prompt builder: %w", err)
	}

	const numProviders = 3
	providers := make([]ProviderClient, 0, numProviders)

	// Add Anthropic if configured
	if cfg.AnthropicAPIKey != "" {
		providers = append(providers, NewAnthropicClient(cfg.AnthropicAPIKey, cfg))
	}

	// Add OpenAI if configured
	if cfg.OpenAIAPIKey != "" {
		providers = append(providers, NewOpenAIClient(cfg.OpenAIAPIKey, cfg))
	}

	// Add Google if configured
	if cfg.GoogleAPIKey != "" {
		providers = append(providers, NewGoogleClient(cfg.GoogleAPIKey, cfg))
	}

	if len(providers) == 0 {
		return nil, ErrNoAPIKey
	}

	return &MultiProviderClient{
		providers:     providers,
		promptBuilder: pb,
		config:        cfg,
	}, nil
}

// NormalizeSpec implements Client.
//
//nolint:dupl // Similar pattern to other LLM methods, but with different types
func (c *MultiProviderClient) NormalizeSpec(
	ctx context.Context,
	input NormalizeSpecInput,
) (*NormalizedSpecification, *InvocationResult, error) {
	log := util.Log(ctx)

	prompt, err := c.promptBuilder.Build(FunctionNormalizeSpec, input)
	if err != nil {
		return nil, nil, fmt.Errorf("build prompt: %w", err)
	}

	req := &CompletionRequest{
		Model:          c.config.DefaultModel,
		SystemPrompt:   "You are an expert software architect.",
		UserPrompt:     prompt,
		MaxTokens:      c.config.MaxOutputTokens,
		Temperature:    c.config.Temperature,
		ResponseFormat: "json",
		Function:       FunctionNormalizeSpec,
		Purpose:        PurposeNormalization,
	}

	resp, err := c.completeWithFallback(ctx, req)
	if err != nil {
		log.WithError(err).Error("normalize spec failed")
		return nil, nil, err
	}

	var result NormalizedSpecification
	if parseErr := json.Unmarshal([]byte(resp.Content), &result); parseErr != nil {
		log.WithError(parseErr).Error("failed to parse normalized spec")
		return nil, nil, fmt.Errorf("%w: %w", ErrInvalidResponse, parseErr)
	}

	invocation := c.buildInvocationResult(resp, FunctionNormalizeSpec)
	return &result, invocation, nil
}

// AnalyzeImpact implements Client.
//
//nolint:dupl // Similar pattern to other LLM methods, but with different types
func (c *MultiProviderClient) AnalyzeImpact(
	ctx context.Context,
	input AnalyzeImpactInput,
) (*ImpactAnalysisResult, *InvocationResult, error) {
	log := util.Log(ctx)

	prompt, err := c.promptBuilder.Build(FunctionAnalyzeImpact, input)
	if err != nil {
		return nil, nil, fmt.Errorf("build prompt: %w", err)
	}

	req := &CompletionRequest{
		Model:          c.config.DefaultModel,
		SystemPrompt:   "You are an expert code analyst.",
		UserPrompt:     prompt,
		MaxTokens:      c.config.MaxOutputTokens,
		Temperature:    c.config.Temperature,
		ResponseFormat: "json",
		Function:       FunctionAnalyzeImpact,
		Purpose:        PurposeImpactAnalysis,
	}

	resp, err := c.completeWithFallback(ctx, req)
	if err != nil {
		log.WithError(err).Error("analyze impact failed")
		return nil, nil, err
	}

	var result ImpactAnalysisResult
	if parseErr := json.Unmarshal([]byte(resp.Content), &result); parseErr != nil {
		log.WithError(parseErr).Error("failed to parse impact analysis")
		return nil, nil, fmt.Errorf("%w: %w", ErrInvalidResponse, parseErr)
	}

	invocation := c.buildInvocationResult(resp, FunctionAnalyzeImpact)
	return &result, invocation, nil
}

// GeneratePlan implements Client.
func (c *MultiProviderClient) GeneratePlan(
	ctx context.Context,
	input GeneratePlanInput,
) (*ImplementationPlan, *InvocationResult, error) {
	log := util.Log(ctx)

	prompt, err := c.promptBuilder.Build(FunctionGeneratePlan, input)
	if err != nil {
		return nil, nil, fmt.Errorf("build prompt: %w", err)
	}

	// Use a more capable model for planning
	model := ModelClaudeSonnet
	if c.config.DefaultModel == ModelClaudeOpus {
		model = ModelClaudeOpus
	}

	req := &CompletionRequest{
		Model:          model,
		SystemPrompt:   "You are an expert software architect.",
		UserPrompt:     prompt,
		MaxTokens:      c.config.MaxOutputTokens,
		Temperature:    c.config.Temperature,
		ResponseFormat: "json",
		Function:       FunctionGeneratePlan,
		Purpose:        PurposePlanning,
	}

	resp, err := c.completeWithFallback(ctx, req)
	if err != nil {
		log.WithError(err).Error("generate plan failed")
		return nil, nil, err
	}

	var result ImplementationPlan
	if parseErr := json.Unmarshal([]byte(resp.Content), &result); parseErr != nil {
		log.WithError(parseErr).Error("failed to parse implementation plan")
		return nil, nil, fmt.Errorf("%w: %w", ErrInvalidResponse, parseErr)
	}

	invocation := c.buildInvocationResult(resp, FunctionGeneratePlan)
	return &result, invocation, nil
}

// GenerateCode implements Client.
//
//nolint:dupl // Similar pattern to other LLM methods, but with different types
func (c *MultiProviderClient) GenerateCode(
	ctx context.Context,
	input GenerateCodeInput,
) (*CodeGenerationResult, *InvocationResult, error) {
	log := util.Log(ctx)

	prompt, err := c.promptBuilder.Build(FunctionGenerateCode, input)
	if err != nil {
		return nil, nil, fmt.Errorf("build prompt: %w", err)
	}

	req := &CompletionRequest{
		Model:          c.config.DefaultModel,
		SystemPrompt:   "You are an expert software engineer.",
		UserPrompt:     prompt,
		MaxTokens:      c.config.MaxOutputTokens,
		Temperature:    c.config.Temperature,
		ResponseFormat: "json",
		Function:       FunctionGenerateCode,
		Purpose:        PurposeCodeGeneration,
	}

	resp, err := c.completeWithFallback(ctx, req)
	if err != nil {
		log.WithError(err).Error("generate code failed")
		return nil, nil, err
	}

	var result CodeGenerationResult
	if parseErr := json.Unmarshal([]byte(resp.Content), &result); parseErr != nil {
		log.WithError(parseErr).Error("failed to parse code generation result")
		return nil, nil, fmt.Errorf("%w: %w", ErrInvalidResponse, parseErr)
	}

	invocation := c.buildInvocationResult(resp, FunctionGenerateCode)
	return &result, invocation, nil
}

// GetUsage implements Client.
func (c *MultiProviderClient) GetUsage() Usage {
	return c.totalUsage
}

// completeWithFallback tries each provider in order until one succeeds.
func (c *MultiProviderClient) completeWithFallback(
	ctx context.Context,
	req *CompletionRequest,
) (*CompletionResponse, error) {
	log := util.Log(ctx)
	var lastErr error

	for _, provider := range c.providers {
		if !provider.IsAvailable() {
			continue
		}

		log.Debug("trying provider",
			"provider", provider.Provider(),
			"function", req.Function,
		)

		resp, err := c.completeWithRetry(ctx, provider, req)
		if err == nil {
			// Update total usage
			c.totalUsage.InputTokens += resp.Usage.InputTokens
			c.totalUsage.OutputTokens += resp.Usage.OutputTokens
			c.totalUsage.TotalTokens += resp.Usage.TotalTokens
			c.totalUsage.CostUSD += resp.Usage.CostUSD

			return resp, nil
		}

		log.WithError(err).Warn("provider failed, trying next",
			"provider", provider.Provider(),
		)
		lastErr = err

		// Don't retry with other providers for certain errors
		if errors.Is(err, ErrContextTooLong) {
			return nil, err
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrAllProvidersFailed, lastErr)
	}
	return nil, ErrAllProvidersFailed
}

// completeWithRetry retries a single provider request.
func (c *MultiProviderClient) completeWithRetry(
	ctx context.Context,
	provider ProviderClient,
	req *CompletionRequest,
) (*CompletionResponse, error) {
	log := util.Log(ctx)
	var lastErr error

	for attempt := range c.config.MaxRetries {
		resp, err := provider.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Don't retry certain errors
		if errors.Is(err, ErrContextTooLong) ||
			errors.Is(err, ErrQuotaExceeded) {
			return nil, err
		}

		// Exponential backoff
		backoff := time.Duration(1<<attempt) * time.Second
		log.Debug("retrying after error",
			"provider", provider.Provider(),
			"attempt", attempt+1,
			"backoff", backoff,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return nil, lastErr
}

// buildInvocationResult creates an InvocationResult from a response.
func (c *MultiProviderClient) buildInvocationResult(
	resp *CompletionResponse,
	fn Function,
) *InvocationResult {
	return &InvocationResult{
		Provider:    c.config.DefaultProvider,
		Model:       c.config.DefaultModel,
		Function:    fn,
		Usage:       resp.Usage,
		LatencyMS:   resp.LatencyMS,
		StopReason:  resp.StopReason,
		RequestID:   resp.RequestID,
		CacheHit:    resp.CacheHit,
		CompletedAt: time.Now(),
	}
}

// estimateCost estimates the cost of a request in USD.
func estimateCost(provider Provider, model Model, usage Usage) float64 {
	// Pricing per 1M tokens (as of early 2025)
	var inputPrice, outputPrice float64

	switch provider {
	case ProviderAnthropic:
		switch model {
		case ModelClaudeOpus:
			inputPrice, outputPrice = 15.0, 75.0
		case ModelClaudeSonnet:
			inputPrice, outputPrice = 3.0, 15.0
		case ModelClaudeHaiku:
			inputPrice, outputPrice = 0.25, 1.25
		case ModelGPT4o, ModelGeminiFlash:
			// Non-Anthropic models, use default
			inputPrice, outputPrice = 3.0, 15.0
		}
	case ProviderOpenAI:
		switch model {
		case ModelGPT4o:
			inputPrice, outputPrice = 2.5, 10.0
		case ModelClaudeSonnet, ModelClaudeOpus, ModelClaudeHaiku, ModelGeminiFlash:
			// Non-OpenAI models, use default
			inputPrice, outputPrice = 2.5, 10.0
		}
	case ProviderGoogle:
		// Gemini pricing
		inputPrice, outputPrice = 0.075, 0.30
	}

	const tokensPerMillion = 1_000_000.0
	inputCost := float64(usage.InputTokens) / tokensPerMillion * inputPrice
	outputCost := float64(usage.OutputTokens) / tokensPerMillion * outputPrice

	return inputCost + outputCost
}
