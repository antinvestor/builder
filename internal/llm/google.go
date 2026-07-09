package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const googleAPIURLTemplate = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"

// GoogleClient implements ProviderClient for Google AI.
type GoogleClient struct {
	apiKey     string
	httpClient *http.Client
	config     ClientConfig
}

// NewGoogleClient creates a new Google AI client.
func NewGoogleClient(apiKey string, cfg ClientConfig) *GoogleClient {
	return &GoogleClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		},
		config: cfg,
	}
}

// Provider implements ProviderClient.
func (c *GoogleClient) Provider() Provider {
	return ProviderGoogle
}

// IsAvailable implements ProviderClient.
func (c *GoogleClient) IsAvailable() bool {
	return c.apiKey != ""
}

// googleRequest is the request body for Google AI API.
type googleRequest struct {
	Contents          []googleContent        `json:"contents"`
	GenerationConfig  googleGenerationConfig `json:"generationConfig,omitzero"`
	SystemInstruction *googleContent         `json:"systemInstruction,omitempty"`
}

// googleContent is content in the Google AI API.
type googleContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []googlePart `json:"parts"`
}

// googlePart is a part of content.
type googlePart struct {
	Text string `json:"text"`
}

// googleGenerationConfig is the generation configuration.
type googleGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

// googleResponse is the response body from Google AI API.
type googleResponse struct {
	Candidates    []googleCandidate   `json:"candidates"`
	UsageMetadata googleUsageMetadata `json:"usageMetadata"`
}

// googleCandidate is a candidate response.
type googleCandidate struct {
	Content      googleContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

// googleUsageMetadata is usage information from Google AI.
type googleUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// googleError is an error response from Google AI.
type googleError struct {
	Error googleErrorDetail `json:"error"`
}

// googleErrorDetail contains error details.
type googleErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// Complete implements ProviderClient.
func (c *GoogleClient) Complete(
	ctx context.Context,
	req *CompletionRequest,
) (*CompletionResponse, error) {
	start := time.Now()

	// Map model to Google model
	model := mapModelToGoogle(req.Model)

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.config.MaxOutputTokens
	}

	// Build request
	googleReq := googleRequest{
		Contents: []googleContent{
			{
				Role: "user",
				Parts: []googlePart{
					{Text: req.UserPrompt},
				},
			},
		},
		GenerationConfig: googleGenerationConfig{
			MaxOutputTokens: maxTokens,
			Temperature:     req.Temperature,
		},
	}

	// Add system instruction if provided
	if req.SystemPrompt != "" {
		googleReq.SystemInstruction = &googleContent{
			Parts: []googlePart{
				{Text: req.SystemPrompt},
			},
		}
	}

	body, err := json.Marshal(googleReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Build URL with model and API key
	url := fmt.Sprintf(googleAPIURLTemplate, model) + "?key=" + c.apiKey

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Handle error responses
	if httpResp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(httpResp.StatusCode, respBody)
	}

	// Parse successful response
	var googleResp googleResponse
	if unmarshalErr := json.Unmarshal(respBody, &googleResp); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal response: %w", unmarshalErr)
	}

	// Extract content
	var content string
	var finishReason string
	if len(googleResp.Candidates) > 0 {
		candidate := googleResp.Candidates[0]
		if len(candidate.Content.Parts) > 0 {
			content = candidate.Content.Parts[0].Text
		}
		finishReason = candidate.FinishReason
	}

	// Build usage
	usage := Usage{
		InputTokens:  googleResp.UsageMetadata.PromptTokenCount,
		OutputTokens: googleResp.UsageMetadata.CandidatesTokenCount,
		TotalTokens:  googleResp.UsageMetadata.TotalTokenCount,
	}
	usage.CostUSD = estimateCost(ProviderGoogle, req.Model, usage)

	latencyMS := time.Since(start).Milliseconds()

	return &CompletionResponse{
		Content:    content,
		Usage:      usage,
		StopReason: finishReason,
		RequestID:  "", // Google doesn't return a request ID
		LatencyMS:  latencyMS,
		CacheHit:   false,
	}, nil
}

// handleErrorResponse handles Google AI API errors.
func (c *GoogleClient) handleErrorResponse(statusCode int, body []byte) error {
	var errResp googleError
	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("API error (status %d): %s", statusCode, string(body))
	}

	errMsg := errResp.Error.Message
	errStatus := errResp.Error.Status

	switch statusCode {
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ErrRateLimited, errMsg)
	case http.StatusPaymentRequired:
		return fmt.Errorf("%w: %s", ErrQuotaExceeded, errMsg)
	case http.StatusBadRequest:
		if errStatus == "INVALID_ARGUMENT" && containsContextLengthError(errMsg) {
			return fmt.Errorf("%w: %s", ErrContextTooLong, errMsg)
		}
		return fmt.Errorf("bad request: %s", errMsg)
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("authentication failed: %s", errMsg)
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return fmt.Errorf("server error: %s", errMsg)
	default:
		return fmt.Errorf("API error (status %d): %s", statusCode, errMsg)
	}
}

// mapModelToGoogle maps our model constants to Google model names.
func mapModelToGoogle(model Model) string {
	switch model {
	case ModelGeminiFlash:
		return string(ModelGeminiFlash)
	case ModelClaudeSonnet, ModelClaudeOpus:
		// Fallback to Gemini Pro for complex tasks
		return "gemini-1.5-pro"
	case ModelClaudeHaiku, ModelGPT4o:
		// Use flash for faster models
		return string(ModelGeminiFlash)
	default:
		return string(ModelGeminiFlash)
	}
}
