// Package llm provides LLM client implementations for code generation.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/pitabwire/util"

	appconfig "github.com/antinvestor/builder/apps/worker/config"
	"github.com/antinvestor/builder/apps/worker/service/events"
	internalevents "github.com/antinvestor/builder/internal/events"
)

// Client implements the BAMLClient interface using Anthropic Claude.
type Client struct {
	cfg       *appconfig.WorkerConfig
	anthropic anthropic.Client
}

// NewClient creates a new LLM client.
func NewClient(cfg *appconfig.WorkerConfig) *Client {
	opts := []option.RequestOption{}
	if cfg.AnthropicAPIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.AnthropicAPIKey))
	}

	return &Client{
		cfg:       cfg,
		anthropic: anthropic.NewClient(opts...),
	}
}

// GeneratePatch implements the BAMLClient interface.
func (c *Client) GeneratePatch(ctx context.Context, req *events.GeneratePatchRequest) (*events.GeneratePatchResponse, error) {
	log := util.Log(ctx)

	// Read workspace files to build context
	fileContents, err := c.readWorkspaceFiles(ctx, req.WorkspacePath)
	if err != nil {
		log.WithError(err).Warn("failed to read workspace files")
	}

	// Build the prompt
	prompt := c.buildCodeGenerationPrompt(req, fileContents)

	// Call Claude API with retry
	var response *anthropic.Message
	var totalTokens int

	for attempt := 0; attempt <= c.cfg.LLMMaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Info("retrying LLM request", "attempt", attempt, "backoff", backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		response, err = c.callClaude(ctx, prompt)
		if err == nil {
			break
		}

		log.WithError(err).Warn("LLM request failed", "attempt", attempt)
	}

	if err != nil {
		return nil, fmt.Errorf("LLM request failed after %d attempts: %w", c.cfg.LLMMaxRetries, err)
	}

	// Track tokens
	totalTokens = int(response.Usage.InputTokens + response.Usage.OutputTokens)

	// Parse response into patches
	patches, commitMessage, err := c.parseResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return &events.GeneratePatchResponse{
		Patches:       patches,
		CommitMessage: commitMessage,
		TokensUsed:    totalTokens,
	}, nil
}

// callClaude makes a request to the Claude API.
func (c *Client) callClaude(ctx context.Context, prompt string) (*anthropic.Message, error) {
	timeout := time.Duration(c.cfg.LLMTimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	message, err := c.anthropic.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude4Sonnet20250514,
		MaxTokens: 16384,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
		Temperature: anthropic.Float(0.0),
	})

	if err != nil {
		return nil, err
	}

	return message, nil
}

// readWorkspaceFiles reads relevant files from the workspace.
func (c *Client) readWorkspaceFiles(ctx context.Context, workspacePath string) (map[string]string, error) {
	if workspacePath == "" {
		return nil, nil
	}

	files := make(map[string]string)
	maxFiles := 50
	maxFileSize := int64(100 * 1024) // 100KB per file
	count := 0

	// Walk the workspace and read relevant files
	err := filepath.Walk(workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories and hidden files
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip large files, binary files, and common non-code files
		if info.Size() > maxFileSize {
			return nil
		}
		if !isCodeFile(path) {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Get relative path
		relPath, _ := filepath.Rel(workspacePath, path)
		files[relPath] = string(content)

		count++
		if count >= maxFiles {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// isCodeFile returns true if the file is a code file.
func isCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	codeExtensions := map[string]bool{
		".go":    true,
		".py":    true,
		".js":    true,
		".ts":    true,
		".tsx":   true,
		".jsx":   true,
		".java":  true,
		".kt":    true,
		".rs":    true,
		".c":     true,
		".cpp":   true,
		".h":     true,
		".hpp":   true,
		".cs":    true,
		".rb":    true,
		".php":   true,
		".swift": true,
		".scala": true,
		".md":    true,
		".json":  true,
		".yaml":  true,
		".yml":   true,
		".toml":  true,
		".xml":   true,
		".html":  true,
		".css":   true,
		".sql":   true,
		".sh":    true,
		".bash":  true,
		".mod":   true,
		".sum":   true,
	}
	return codeExtensions[ext]
}

// buildCodeGenerationPrompt builds the prompt for code generation.
func (c *Client) buildCodeGenerationPrompt(req *events.GeneratePatchRequest, fileContents map[string]string) string {
	var sb strings.Builder

	sb.WriteString("You are an expert software engineer implementing code changes.\n\n")

	sb.WriteString("## Task\n")
	sb.WriteString(fmt.Sprintf("Implement the following feature: %s\n\n", req.Specification.Title))
	sb.WriteString(fmt.Sprintf("Description: %s\n\n", req.Specification.Description))

	if len(req.Specification.AcceptanceCriteria) > 0 {
		sb.WriteString("## Acceptance Criteria\n")
		for _, criterion := range req.Specification.AcceptanceCriteria {
			sb.WriteString(fmt.Sprintf("- %s\n", criterion))
		}
		sb.WriteString("\n")
	}

	if len(req.Specification.PathHints) > 0 {
		sb.WriteString("## Target Files (hints)\n")
		for _, path := range req.Specification.PathHints {
			sb.WriteString(fmt.Sprintf("- %s\n", path))
		}
		sb.WriteString("\n")
	}

	if req.RepositoryContext != "" {
		sb.WriteString("## Repository Context\n")
		sb.WriteString(req.RepositoryContext)
		sb.WriteString("\n\n")
	}

	if len(fileContents) > 0 {
		sb.WriteString("## Current Codebase\n")
		for path, content := range fileContents {
			sb.WriteString(fmt.Sprintf("### %s\n```\n%s\n```\n\n", path, content))
		}
	}

	if req.FeedbackFromReview != "" {
		sb.WriteString("## Feedback from Previous Review\n")
		sb.WriteString(req.FeedbackFromReview)
		sb.WriteString("\n\n")
	}

	sb.WriteString(`## Instructions
Generate the code changes needed to implement this feature. Your response MUST be valid JSON matching this exact schema:

{
  "patches": [
    {
      "file_path": "path/to/file.go",
      "action": "create" | "modify" | "delete",
      "new_content": "the full file content for create/modify actions"
    }
  ],
  "commit_message": "feat: description of changes"
}

Important:
1. For "create" action: provide the full file content in new_content
2. For "modify" action: provide the complete new file content (not a diff)
3. For "delete" action: new_content can be empty
4. Follow existing code style and patterns
5. Ensure all code is syntactically correct
6. Use conventional commit format for commit_message

Respond ONLY with the JSON object, no other text.
`)

	return sb.String()
}

// CodeGenerationResponse is the expected JSON response from the LLM.
type CodeGenerationResponse struct {
	Patches []struct {
		FilePath   string `json:"file_path"`
		Action     string `json:"action"`
		NewContent string `json:"new_content"`
	} `json:"patches"`
	CommitMessage string `json:"commit_message"`
}

// parseResponse parses the LLM response into patches.
func (c *Client) parseResponse(message *anthropic.Message) ([]events.Patch, string, error) {
	if len(message.Content) == 0 {
		return nil, "", fmt.Errorf("empty response from LLM")
	}

	// Extract text from response
	var text string
	for _, block := range message.Content {
		if block.Type == "text" {
			text = block.Text
			break
		}
	}

	if text == "" {
		return nil, "", fmt.Errorf("no text content in LLM response")
	}

	// Clean up potential markdown wrapping
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

	// Parse JSON response
	var resp CodeGenerationResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		return nil, "", fmt.Errorf("failed to parse JSON response: %w\nraw response: %s", err, text[:min(500, len(text))])
	}

	// Convert to patches
	patches := make([]events.Patch, 0, len(resp.Patches))
	for _, p := range resp.Patches {
		action := internalevents.FileActionModify
		switch strings.ToLower(p.Action) {
		case "create":
			action = internalevents.FileActionCreate
		case "delete":
			action = internalevents.FileActionDelete
		case "modify":
			action = internalevents.FileActionModify
		}

		patches = append(patches, events.Patch{
			FilePath:   p.FilePath,
			NewContent: p.NewContent,
			Action:     action,
		})
	}

	commitMessage := resp.CommitMessage
	if commitMessage == "" {
		commitMessage = "feat: implement feature changes"
	}

	return patches, commitMessage, nil
}
