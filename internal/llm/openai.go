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

const openaiAPIURL = "https://api.openai.com/v1/chat/completions"

// OpenAIClient implements ProviderClient for OpenAI.
type OpenAIClient struct {
	apiKey     string
	httpClient *http.Client
	config     ClientConfig
}

// NewOpenAIClient creates a new OpenAI client.
func NewOpenAIClient(apiKey string, cfg ClientConfig) *OpenAIClient {
	return &OpenAIClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		},
		config: cfg,
	}
}

// Provider implements ProviderClient.
func (c *OpenAIClient) Provider() Provider {
	return ProviderOpenAI
}

// IsAvailable implements ProviderClient.
func (c *OpenAIClient) IsAvailable() bool {
	return c.apiKey != ""
}

// openaiRequest is the request body for OpenAI API.
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

// openaiMessage is a message in the OpenAI API.
type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiResponse is the response body from OpenAI API.
type openaiResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

// openaiChoice is a choice in the OpenAI response.
type openaiChoice struct {
	Index        int           `json:"index"`
	Message      openaiMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// openaiUsage is usage information from OpenAI.
type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// openaiError is an error response from OpenAI.
type openaiError struct {
	Error openaiErrorDetail `json:"error"`
}

// openaiErrorDetail contains error details.
type openaiErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// Complete implements ProviderClient.
func (c *OpenAIClient) Complete(
	ctx context.Context,
	req *CompletionRequest,
) (*CompletionResponse, error) {
	start := time.Now()

	// Map model to OpenAI model
	model := mapModelToOpenAI(req.Model)
	if model == "" {
		model = string(ModelGPT4o)
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.config.MaxOutputTokens
	}

	// Build messages
	messages := []openaiMessage{}
	if req.SystemPrompt != "" {
		messages = append(messages, openaiMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}
	messages = append(messages, openaiMessage{
		Role:    "user",
		Content: req.UserPrompt,
	})

	openaiReq := openaiRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		openaiAPIURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

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
	var openaiResp openaiResponse
	if unmarshalErr := json.Unmarshal(respBody, &openaiResp); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal response: %w", unmarshalErr)
	}

	// Extract content
	var content string
	var finishReason string
	if len(openaiResp.Choices) > 0 {
		content = openaiResp.Choices[0].Message.Content
		finishReason = openaiResp.Choices[0].FinishReason
	}

	// Build usage
	usage := Usage{
		InputTokens:  openaiResp.Usage.PromptTokens,
		OutputTokens: openaiResp.Usage.CompletionTokens,
		TotalTokens:  openaiResp.Usage.TotalTokens,
	}
	usage.CostUSD = estimateCost(ProviderOpenAI, req.Model, usage)

	latencyMS := time.Since(start).Milliseconds()

	return &CompletionResponse{
		Content:    content,
		Usage:      usage,
		StopReason: finishReason,
		RequestID:  openaiResp.ID,
		LatencyMS:  latencyMS,
		CacheHit:   false,
	}, nil
}

// handleErrorResponse handles OpenAI API errors.
func (c *OpenAIClient) handleErrorResponse(statusCode int, body []byte) error {
	var errResp openaiError
	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("API error (status %d): %s", statusCode, string(body))
	}

	errMsg := errResp.Error.Message
	errCode := errResp.Error.Code

	switch statusCode {
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ErrRateLimited, errMsg)
	case http.StatusPaymentRequired:
		return fmt.Errorf("%w: %s", ErrQuotaExceeded, errMsg)
	case http.StatusBadRequest:
		if errCode == "context_length_exceeded" {
			return fmt.Errorf("%w: %s", ErrContextTooLong, errMsg)
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

// mapModelToOpenAI maps our model constants to OpenAI model names.
func mapModelToOpenAI(model Model) string {
	switch model {
	case ModelGPT4o:
		return string(ModelGPT4o)
	case ModelClaudeSonnet, ModelClaudeOpus:
		// Fallback to GPT-4o for Claude models
		return string(ModelGPT4o)
	case ModelClaudeHaiku, ModelGeminiFlash:
		// Use faster model for Haiku/Flash-equivalent
		return "gpt-4o-mini"
	}
	return string(ModelGPT4o)
}
