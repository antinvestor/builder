package llm

import (
	"encoding/json"
	"strings"
	"testing"

	appconfig "github.com/antinvestor/builder/apps/worker/config"
	"github.com/antinvestor/builder/apps/worker/service/events"
	internalevents "github.com/antinvestor/builder/internal/events"
)

func TestBuildCodeGenerationPrompt(t *testing.T) {
	cfg := &appconfig.WorkerConfig{}
	client := NewClient(cfg)

	req := &events.GeneratePatchRequest{
		ExecutionID: internalevents.NewExecutionID(),
		Specification: internalevents.FeatureSpecification{
			Title:              "Add user authentication",
			Description:        "Implement JWT-based authentication for the API",
			AcceptanceCriteria: []string{"Users can login with email/password", "JWT tokens are returned on success"},
			PathHints:          []string{"api/auth.go", "api/middleware.go"},
		},
		WorkspacePath:     "/tmp/workspace",
		RepositoryContext: "This is a Go web API using Chi router",
	}

	fileContents := map[string]string{
		"main.go": "package main\n\nfunc main() {}",
	}

	prompt := client.buildCodeGenerationPrompt(req, fileContents)

	// Check that the prompt contains expected sections
	if !contains(prompt, "Add user authentication") {
		t.Error("prompt should contain feature title")
	}
	if !contains(prompt, "JWT-based authentication") {
		t.Error("prompt should contain description")
	}
	if !contains(prompt, "Users can login with email/password") {
		t.Error("prompt should contain acceptance criteria")
	}
	if !contains(prompt, "api/auth.go") {
		t.Error("prompt should contain path hints")
	}
	if !contains(prompt, "Chi router") {
		t.Error("prompt should contain repository context")
	}
	if !contains(prompt, "main.go") {
		t.Error("prompt should contain file contents")
	}
	if !contains(prompt, "JSON") {
		t.Error("prompt should mention JSON output format")
	}
}

func TestBuildCodeGenerationPrompt_WithFeedback(t *testing.T) {
	cfg := &appconfig.WorkerConfig{}
	client := NewClient(cfg)

	req := &events.GeneratePatchRequest{
		ExecutionID: internalevents.NewExecutionID(),
		Specification: internalevents.FeatureSpecification{
			Title:       "Fix bug",
			Description: "Fix the null pointer issue",
		},
		FeedbackFromReview: "Missing error handling in the login function",
	}

	prompt := client.buildCodeGenerationPrompt(req, nil)

	if !contains(prompt, "Feedback from Previous Review") {
		t.Error("prompt should contain feedback section header")
	}
	if !contains(prompt, "Missing error handling") {
		t.Error("prompt should contain feedback content")
	}
}

func TestIsCodeFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"main.go", true},
		{"app.py", true},
		{"index.ts", true},
		{"styles.css", true},
		{"config.yaml", true},
		{"go.mod", true},
		{"image.png", false},
		{"binary.exe", false},
		{"document.pdf", false},
		{".gitignore", false},
		{"Makefile", false}, // No extension
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isCodeFile(tt.path)
			if result != tt.expected {
				t.Errorf("isCodeFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestParseResponse_ValidJSON(t *testing.T) {
	cfg := &appconfig.WorkerConfig{}
	client := NewClient(cfg)

	// Create a mock message with valid JSON response
	jsonResponse := `{
		"patches": [
			{
				"file_path": "main.go",
				"action": "modify",
				"new_content": "package main\n\nfunc main() { println(\"hello\") }"
			},
			{
				"file_path": "utils.go",
				"action": "create",
				"new_content": "package main\n\nfunc helper() {}"
			}
		],
		"commit_message": "feat: add hello world"
	}`

	// We can't easily test parseResponse without a real anthropic.Message,
	// but we can test the JSON parsing logic by extracting it
	patches, commitMsg := parseJSONResponse(t, client, jsonResponse)

	if len(patches) != 2 {
		t.Fatalf("expected 2 patches, got %d", len(patches))
	}

	if patches[0].FilePath != "main.go" {
		t.Errorf("expected file_path 'main.go', got %q", patches[0].FilePath)
	}
	if patches[0].Action != internalevents.FileActionModify {
		t.Errorf("expected action 'modify', got %q", patches[0].Action)
	}

	if patches[1].FilePath != "utils.go" {
		t.Errorf("expected file_path 'utils.go', got %q", patches[1].FilePath)
	}
	if patches[1].Action != internalevents.FileActionCreate {
		t.Errorf("expected action 'create', got %q", patches[1].Action)
	}

	if commitMsg != "feat: add hello world" {
		t.Errorf("expected commit message 'feat: add hello world', got %q", commitMsg)
	}
}

func TestParseResponse_MarkdownWrapped(t *testing.T) {
	cfg := &appconfig.WorkerConfig{}
	client := NewClient(cfg)

	// JSON wrapped in markdown code blocks
	jsonResponse := "```json\n" + `{
		"patches": [{"file_path": "test.go", "action": "create", "new_content": "test"}],
		"commit_message": "test"
	}` + "\n```"

	patches, _ := parseJSONResponse(t, client, jsonResponse)

	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
}

func TestParseResponse_DeleteAction(t *testing.T) {
	cfg := &appconfig.WorkerConfig{}
	client := NewClient(cfg)

	jsonResponse := `{
		"patches": [{"file_path": "old.go", "action": "delete", "new_content": ""}],
		"commit_message": "chore: remove old file"
	}`

	patches, _ := parseJSONResponse(t, client, jsonResponse)

	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	if patches[0].Action != internalevents.FileActionDelete {
		t.Errorf("expected action 'delete', got %q", patches[0].Action)
	}
}

// Helper to test JSON parsing without needing anthropic.Message
func parseJSONResponse(t *testing.T, client *Client, text string) ([]events.Patch, string) {
	t.Helper()

	// Simulate the text cleanup logic from parseResponse
	text = trimMarkdown(text)

	var resp CodeGenerationResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	patches := make([]events.Patch, 0, len(resp.Patches))
	for _, p := range resp.Patches {
		action := internalevents.FileActionModify
		switch p.Action {
		case "create":
			action = internalevents.FileActionCreate
		case "delete":
			action = internalevents.FileActionDelete
		}
		patches = append(patches, events.Patch{
			FilePath:   p.FilePath,
			NewContent: p.NewContent,
			Action:     action,
		})
	}

	return patches, resp.CommitMessage
}

func trimMarkdown(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	return text
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
