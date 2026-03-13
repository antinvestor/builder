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

const (
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion = "2023-06-01"
)

// AnthropicClient implements ProviderClient for Anthropic.
type AnthropicClient struct {
	apiKey     string
	httpClient *http.Client
	config     ClientConfig
}

// NewAnthropicClient creates a new Anthropic client.
func NewAnthropicClient(apiKey string, cfg ClientConfig) *AnthropicClient {
	return &AnthropicClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		},
		config: cfg,
	}
}

// Provider implements ProviderClient.
func (c *AnthropicClient) Provider() Provider {
	return ProviderAnthropic
}

// IsAvailable implements ProviderClient.
func (c *AnthropicClient) IsAvailable() bool {
	return c.apiKey != ""
}

// anthropicRequest is the request body for Anthropic API.
type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
}

// anthropicMessage is a message in the Anthropic API.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse is the response body from Anthropic API.
type anthropicResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []anthropicContent `json:"content"`
	Model        string             `json:"model"`
	StopReason   string             `json:"stop_reason"`
	StopSequence string             `json:"stop_sequence,omitempty"`
	Usage        anthropicUsage     `json:"usage"`
}

// anthropicContent is content in the Anthropic response.
type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// anthropicUsage is usage information from Anthropic.
type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// anthropicError is an error response from Anthropic.
type anthropicError struct {
	Type  string               `json:"type"`
	Error anthropicErrorDetail `json:"error"`
}

// anthropicErrorDetail contains error details.
type anthropicErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Complete implements ProviderClient.
func (c *AnthropicClient) Complete(
	ctx context.Context,
	req *CompletionRequest,
) (*CompletionResponse, error) {
	start := time.Now()

	// Build request body
	model := string(req.Model)
	if model == "" {
		model = string(ModelClaudeSonnet)
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.config.MaxOutputTokens
	}

	anthropicReq := anthropicRequest{
		Model:       model,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		System:      req.SystemPrompt,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: req.UserPrompt,
			},
		},
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		anthropicAPIURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Api-Key", c.apiKey)
	httpReq.Header.Set("Anthropic-Version", anthropicAPIVersion)

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
	var anthropicResp anthropicResponse
	if unmarshalErr := json.Unmarshal(respBody, &anthropicResp); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal response: %w", unmarshalErr)
	}

	// Extract text content
	var content string
	for _, c := range anthropicResp.Content {
		if c.Type == "text" {
			content = c.Text
			break
		}
	}

	// Build usage
	usage := Usage{
		InputTokens:      anthropicResp.Usage.InputTokens,
		OutputTokens:     anthropicResp.Usage.OutputTokens,
		TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		CacheReadTokens:  anthropicResp.Usage.CacheReadInputTokens,
		CacheWriteTokens: anthropicResp.Usage.CacheCreationInputTokens,
	}
	usage.CostUSD = estimateCost(ProviderAnthropic, req.Model, usage)

	latencyMS := time.Since(start).Milliseconds()

	return &CompletionResponse{
		Content:    content,
		Usage:      usage,
		StopReason: anthropicResp.StopReason,
		RequestID:  anthropicResp.ID,
		LatencyMS:  latencyMS,
		CacheHit:   anthropicResp.Usage.CacheReadInputTokens > 0,
	}, nil
}

// handleErrorResponse handles Anthropic API errors.
func (c *AnthropicClient) handleErrorResponse(statusCode int, body []byte) error {
	var errResp anthropicError
	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("API error (status %d): %s", statusCode, string(body))
	}

	errType := errResp.Error.Type
	errMsg := errResp.Error.Message

	switch statusCode {
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ErrRateLimited, errMsg)
	case http.StatusPaymentRequired:
		return fmt.Errorf("%w: %s", ErrQuotaExceeded, errMsg)
	case http.StatusBadRequest:
		if errType == "invalid_request_error" {
			// Check for context length errors
			if containsContextLengthError(errMsg) {
				return fmt.Errorf("%w: %s", ErrContextTooLong, errMsg)
			}
		}
		return fmt.Errorf("bad request: %s", errMsg)
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed: %s", errMsg)
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return fmt.Errorf("server error: %s", errMsg)
	default:
		return fmt.Errorf("API error (status %d): %s", statusCode, errMsg)
	}
}

// containsContextLengthError checks if an error message indicates context length issues.
func containsContextLengthError(msg string) bool {
	keywords := []string{
		"context_length",
		"too many tokens",
		"maximum context length",
		"token limit",
	}
	for _, kw := range keywords {
		if contains(msg, kw) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

// searchSubstring searches for a substring case-insensitively.
func searchSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldSlice(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

// equalFoldSlice compares two strings case-insensitively.
func equalFoldSlice(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		ca, cb := a[i], b[i]
		if ca != cb {
			// Convert to lowercase
			if ca >= 'A' && ca <= 'Z' {
				ca += 'a' - 'A'
			}
			if cb >= 'A' && cb <= 'Z' {
				cb += 'a' - 'A'
			}
			if ca != cb {
				return false
			}
		}
	}
	return true
}
