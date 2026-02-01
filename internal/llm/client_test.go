//nolint:testpackage // Testing internal functions requires same package
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewMultiProviderClient_NoAPIKey(t *testing.T) {
	_, err := NewMultiProviderClient(ClientConfig{})
	if err == nil {
		t.Error("expected error when no API keys provided")
	}
	if !errors.Is(err, ErrNoAPIKey) {
		t.Errorf("expected ErrNoAPIKey, got %v", err)
	}
}

func TestNewMultiProviderClient_WithAnthropicKey(t *testing.T) {
	client, err := NewMultiProviderClient(ClientConfig{
		AnthropicAPIKey: "test-key",
		DefaultModel:    ModelClaudeSonnet,
		MaxOutputTokens: 4096,
		TimeoutSeconds:  60,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client to be non-nil")
	}
	if len(client.providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(client.providers))
	}
}

func TestNewMultiProviderClient_MultipleProviders(t *testing.T) {
	client, err := NewMultiProviderClient(ClientConfig{
		AnthropicAPIKey: "test-key",
		OpenAIAPIKey:    "test-key",
		GoogleAPIKey:    "test-key",
		DefaultModel:    ModelClaudeSonnet,
		MaxOutputTokens: 4096,
		TimeoutSeconds:  60,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.providers) != 3 {
		t.Errorf("expected 3 providers, got %d", len(client.providers))
	}
}

func TestAnthropicClient_IsAvailable(t *testing.T) {
	client := NewAnthropicClient("test-key", ClientConfig{TimeoutSeconds: 60})
	if !client.IsAvailable() {
		t.Error("expected client to be available")
	}

	client = NewAnthropicClient("", ClientConfig{TimeoutSeconds: 60})
	if client.IsAvailable() {
		t.Error("expected client to be unavailable with empty key")
	}
}

func TestOpenAIClient_IsAvailable(t *testing.T) {
	client := NewOpenAIClient("test-key", ClientConfig{TimeoutSeconds: 60})
	if !client.IsAvailable() {
		t.Error("expected client to be available")
	}

	client = NewOpenAIClient("", ClientConfig{TimeoutSeconds: 60})
	if client.IsAvailable() {
		t.Error("expected client to be unavailable with empty key")
	}
}

func TestGoogleClient_IsAvailable(t *testing.T) {
	client := NewGoogleClient("test-key", ClientConfig{TimeoutSeconds: 60})
	if !client.IsAvailable() {
		t.Error("expected client to be available")
	}

	client = NewGoogleClient("", ClientConfig{TimeoutSeconds: 60})
	if client.IsAvailable() {
		t.Error("expected client to be unavailable with empty key")
	}
}

func TestAnthropicClient_Provider(t *testing.T) {
	client := NewAnthropicClient("test-key", ClientConfig{TimeoutSeconds: 60})
	if client.Provider() != ProviderAnthropic {
		t.Errorf("expected provider %s, got %s", ProviderAnthropic, client.Provider())
	}
}

func TestOpenAIClient_Provider(t *testing.T) {
	client := NewOpenAIClient("test-key", ClientConfig{TimeoutSeconds: 60})
	if client.Provider() != ProviderOpenAI {
		t.Errorf("expected provider %s, got %s", ProviderOpenAI, client.Provider())
	}
}

func TestGoogleClient_Provider(t *testing.T) {
	client := NewGoogleClient("test-key", ClientConfig{TimeoutSeconds: 60})
	if client.Provider() != ProviderGoogle {
		t.Errorf("expected provider %s, got %s", ProviderGoogle, client.Provider())
	}
}

func TestMapModelToOpenAI(t *testing.T) {
	tests := []struct {
		input    Model
		expected string
	}{
		{ModelGPT4o, string(ModelGPT4o)},
		{ModelClaudeSonnet, string(ModelGPT4o)},
		{ModelClaudeOpus, string(ModelGPT4o)},
		{ModelClaudeHaiku, "gpt-4o-mini"},
		{ModelGeminiFlash, "gpt-4o-mini"},
	}

	for _, tt := range tests {
		result := mapModelToOpenAI(tt.input)
		if result != tt.expected {
			t.Errorf("mapModelToOpenAI(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestMapModelToGoogle(t *testing.T) {
	tests := []struct {
		input    Model
		expected string
	}{
		{ModelGeminiFlash, string(ModelGeminiFlash)},
		{ModelClaudeSonnet, "gemini-1.5-pro"},
		{ModelClaudeOpus, "gemini-1.5-pro"},
		{ModelClaudeHaiku, string(ModelGeminiFlash)},
		{ModelGPT4o, string(ModelGeminiFlash)},
	}

	for _, tt := range tests {
		result := mapModelToGoogle(tt.input)
		if result != tt.expected {
			t.Errorf("mapModelToGoogle(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestEstimateCost(t *testing.T) {
	usage := Usage{
		InputTokens:  1000,
		OutputTokens: 500,
	}

	tests := []struct {
		provider Provider
		model    Model
		minCost  float64
		maxCost  float64
	}{
		{ProviderAnthropic, ModelClaudeHaiku, 0.0, 0.01},
		{ProviderAnthropic, ModelClaudeSonnet, 0.0, 0.02},
		{ProviderAnthropic, ModelClaudeOpus, 0.0, 0.1},
		{ProviderOpenAI, ModelGPT4o, 0.0, 0.02},
		{ProviderGoogle, ModelGeminiFlash, 0.0, 0.01},
	}

	for _, tt := range tests {
		cost := estimateCost(tt.provider, tt.model, usage)
		if cost < tt.minCost || cost > tt.maxCost {
			t.Errorf("estimateCost(%s, %s) = %f, expected between %f and %f",
				tt.provider, tt.model, cost, tt.minCost, tt.maxCost)
		}
	}
}

func TestContainsContextLengthError(t *testing.T) {
	tests := []struct {
		msg      string
		expected bool
	}{
		{"context_length exceeded", true},
		{"Too many tokens in request", true},
		{"Maximum context length exceeded", true},
		{"Token limit reached", true},
		{"something else happened", false},
		{"", false},
	}

	for _, tt := range tests {
		result := containsContextLengthError(tt.msg)
		if result != tt.expected {
			t.Errorf("containsContextLengthError(%q) = %v, expected %v",
				tt.msg, result, tt.expected)
		}
	}
}

func TestTruncateContent(t *testing.T) {
	tests := []struct {
		content  string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello\n... [truncated]"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncateContent(tt.content, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateContent(%q, %d) = %q, expected %q",
				tt.content, tt.maxLen, result, tt.expected)
		}
	}
}

func TestAnthropicClient_Complete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type")
		}
		if r.Header.Get("X-Api-Key") == "" {
			t.Error("expected X-Api-Key header")
		}
		if r.Header.Get("Anthropic-Version") == "" {
			t.Error("expected Anthropic-Version header")
		}

		// Return mock response
		resp := anthropicResponse{
			ID:   "msg_test123",
			Type: "message",
			Role: "assistant",
			Content: []anthropicContent{
				{Type: "text", Text: `{"test": "response"}`},
			},
			Model:      "claude-sonnet-4-20250514",
			StopReason: "end_turn",
			Usage: anthropicUsage{
				InputTokens:  100,
				OutputTokens: 50,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client with test server
	client := &AnthropicClient{
		apiKey: "test-key",
		httpClient: &http.Client{
			Transport: &testTransport{
				originalURL: anthropicAPIURL,
				testURL:     server.URL,
			},
		},
		config: ClientConfig{
			MaxOutputTokens: 4096,
			TimeoutSeconds:  60,
		},
	}

	ctx := context.Background()
	resp, err := client.Complete(ctx, &CompletionRequest{
		Model:        ModelClaudeSonnet,
		SystemPrompt: "You are a test assistant.",
		UserPrompt:   "Test prompt",
		MaxTokens:    1000,
		Temperature:  0.0,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != `{"test": "response"}` {
		t.Errorf("unexpected content: %s", resp.Content)
	}
	if resp.RequestID != "msg_test123" {
		t.Errorf("unexpected request ID: %s", resp.RequestID)
	}
	if resp.Usage.InputTokens != 100 {
		t.Errorf("unexpected input tokens: %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 50 {
		t.Errorf("unexpected output tokens: %d", resp.Usage.OutputTokens)
	}
}

func TestAnthropicClient_Complete_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := anthropicError{
			Type: "error",
			Error: anthropicErrorDetail{
				Type:    "rate_limit_error",
				Message: "Rate limit exceeded",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &AnthropicClient{
		apiKey: "test-key",
		httpClient: &http.Client{
			Transport: &testTransport{
				originalURL: anthropicAPIURL,
				testURL:     server.URL,
			},
		},
		config: ClientConfig{
			MaxOutputTokens: 4096,
			TimeoutSeconds:  60,
		},
	}

	ctx := context.Background()
	_, err := client.Complete(ctx, &CompletionRequest{
		Model:      ModelClaudeSonnet,
		UserPrompt: "Test prompt",
	})

	if err == nil {
		t.Fatal("expected error")
	}
}

// testTransport redirects requests to the test server.
type testTransport struct {
	originalURL string
	testURL     string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.testURL[7:] // Strip "http://"
	return http.DefaultTransport.RoundTrip(req)
}

func TestPromptBuilder_Build(t *testing.T) {
	pb, err := NewPromptBuilder()
	if err != nil {
		t.Fatalf("failed to create prompt builder: %v", err)
	}

	input := NormalizeSpecInput{
		Spec: FeatureSpecification{
			Title:              "Add user authentication",
			Description:        "Implement JWT-based authentication",
			AcceptanceCriteria: []string{"Users can log in", "Users can log out"},
		},
		CodebaseContext: "Go project with Gin framework",
		Language:        "go",
	}

	prompt, err := pb.Build(FunctionNormalizeSpec, input)
	if err != nil {
		t.Fatalf("failed to build prompt: %v", err)
	}

	if prompt == "" {
		t.Error("expected non-empty prompt")
	}

	// Check that key elements are present
	if !contains(prompt, "Add user authentication") {
		t.Error("expected prompt to contain title")
	}
	if !contains(prompt, "JWT-based authentication") {
		t.Error("expected prompt to contain description")
	}
}

func TestPromptBuilder_BuildAllFunctions(t *testing.T) {
	pb, err := NewPromptBuilder()
	if err != nil {
		t.Fatalf("failed to create prompt builder: %v", err)
	}

	tests := []struct {
		function Function
		input    any
	}{
		{
			FunctionNormalizeSpec,
			NormalizeSpecInput{
				Spec:            FeatureSpecification{Title: "Test"},
				CodebaseContext: "context",
				Language:        "go",
			},
		},
		{
			FunctionAnalyzeImpact,
			AnalyzeImpactInput{
				NormalizedSpec:   NormalizedSpecification{ProblemStatement: "Test problem"},
				FileContents:     map[string]string{"main.go": "package main"},
				ProjectStructure: "├── main.go",
			},
		},
		{
			FunctionGeneratePlan,
			GeneratePlanInput{
				NormalizedSpec: NormalizedSpecification{ProblemStatement: "Test problem"},
				ImpactAnalysis: ImpactAnalysisResult{},
				FileContents:   map[string]string{},
				ProjectInfo:    "Go project",
			},
		},
		{
			FunctionGenerateCode,
			GenerateCodeInput{
				Step:         PlanStep{Action: "Create file"},
				FileContents: map[string]string{},
				Language:     "go",
			},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.function), func(t *testing.T) {
			prompt, buildErr := pb.Build(tt.function, tt.input)
			if buildErr != nil {
				t.Errorf("failed to build prompt for %s: %v", tt.function, buildErr)
			}
			if prompt == "" {
				t.Errorf("expected non-empty prompt for %s", tt.function)
			}
		})
	}
}

func TestNewBAMLClient_NoAPIKey(t *testing.T) {
	_, err := NewBAMLClient(ClientConfig{})
	if err == nil {
		t.Error("expected error when no API keys provided")
	}
}

func TestNewBAMLClient_Success(t *testing.T) {
	client, err := NewBAMLClient(ClientConfig{
		AnthropicAPIKey: "test-key",
		DefaultModel:    ModelClaudeSonnet,
		MaxOutputTokens: 4096,
		TimeoutSeconds:  60,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client to be non-nil")
	}
}

func TestGetUsage(t *testing.T) {
	client, err := NewMultiProviderClient(ClientConfig{
		AnthropicAPIKey: "test-key",
		DefaultModel:    ModelClaudeSonnet,
		MaxOutputTokens: 4096,
		TimeoutSeconds:  60,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	usage := client.GetUsage()
	if usage.TotalTokens != 0 {
		t.Errorf("expected 0 tokens for new client, got %d", usage.TotalTokens)
	}
}
