package llm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pitabwire/util"
)

// Content truncation limits.
const (
	maxFileContentLength    = 5000
	maxKeyFileContentLength = 2000
	defaultMaxFileSizeLines = 1000
)

// BAMLClient is the high-level client for BAML operations used by the worker.
// It implements the events.BAMLClient interface.
type BAMLClient struct {
	client Client
	config ClientConfig
}

// NewBAMLClient creates a new BAML client.
func NewBAMLClient(cfg ClientConfig) (*BAMLClient, error) {
	client, err := NewMultiProviderClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create multi-provider client: %w", err)
	}

	return &BAMLClient{
		client: client,
		config: cfg,
	}, nil
}

// GeneratePatchRequest is the request for patch generation.
type GeneratePatchRequest struct {
	ExecutionID        string
	Specification      FeatureSpecification
	WorkspacePath      string
	RepositoryContext  string
	PreviousPatches    []Patch
	IterationNumber    int
	FeedbackFromReview string
}

// GeneratePatchResponse is the response from patch generation.
type GeneratePatchResponse struct {
	Patches       []Patch
	CommitMessage string
	TokensUsed    int
}

// Patch represents a code patch.
type Patch struct {
	FilePath   string
	OldContent string
	NewContent string
	Action     string // "create", "modify", "delete"
}

// GeneratePatch generates code patches for a feature specification.
// This is the main entry point used by the worker service.
//
//nolint:funlen // Pipeline orchestration requires many sequential steps
func (c *BAMLClient) GeneratePatch(
	ctx context.Context,
	req *GeneratePatchRequest,
) (*GeneratePatchResponse, error) {
	log := util.Log(ctx)
	log.Info("starting patch generation",
		"execution_id", req.ExecutionID,
		"workspace", req.WorkspacePath,
	)

	// Step 1: Build codebase context
	codebaseContext := c.buildCodebaseContext(req.WorkspacePath)

	// Detect primary language
	language := c.detectLanguage(req.WorkspacePath)

	// Step 2: Normalize specification
	log.Debug("normalizing specification")
	normalizedSpec, _, err := c.client.NormalizeSpec(ctx, NormalizeSpecInput{
		Spec:            req.Specification,
		CodebaseContext: codebaseContext,
		Language:        language,
	})
	if err != nil {
		return nil, fmt.Errorf("normalize specification: %w", err)
	}
	log.Debug("specification normalized",
		"complexity", normalizedSpec.Complexity.Level,
		"components", len(normalizedSpec.Components),
	)

	// Step 3: Read relevant files for impact analysis
	fileContents := c.readRelevantFiles(ctx, req.WorkspacePath, normalizedSpec.Components)

	// Step 4: Analyze impact
	log.Debug("analyzing impact")
	projectStructure := c.getProjectStructure(req.WorkspacePath)
	impactAnalysis, _, err := c.client.AnalyzeImpact(ctx, AnalyzeImpactInput{
		NormalizedSpec:   *normalizedSpec,
		FileContents:     fileContents,
		ProjectStructure: projectStructure,
	})
	if err != nil {
		return nil, fmt.Errorf("analyze impact: %w", err)
	}
	log.Debug("impact analyzed",
		"direct_impacts", len(impactAnalysis.DirectImpacts),
		"indirect_impacts", len(impactAnalysis.IndirectImpacts),
	)

	// Step 5: Generate implementation plan
	log.Debug("generating implementation plan")
	projectInfo := c.getProjectInfo(req.WorkspacePath, language)
	plan, _, err := c.client.GeneratePlan(ctx, GeneratePlanInput{
		NormalizedSpec: *normalizedSpec,
		ImpactAnalysis: *impactAnalysis,
		FileContents:   fileContents,
		ProjectInfo:    projectInfo,
	})
	if err != nil {
		return nil, fmt.Errorf("generate plan: %w", err)
	}
	log.Info("plan generated",
		"title", plan.Title,
		"steps", len(plan.Steps),
		"estimated_tokens", plan.EstimatedMetrics.EstimatedTokens,
	)

	// Step 6: Execute plan steps and generate code
	var patches []Patch
	var commitMessages []string

	for _, step := range plan.Steps {
		log.Debug("executing plan step",
			"step", step.StepNumber,
			"action", step.Action,
		)

		// Read files needed for this step
		stepFiles := c.readFilesForStep(ctx, req.WorkspacePath, step)

		// Generate code for this step
		codeResult, _, genErr := c.client.GenerateCode(ctx, GenerateCodeInput{
			Step:         step,
			FileContents: stepFiles,
			Language:     language,
			Framework:    c.detectFramework(req.WorkspacePath, language),
			StyleGuide:   "",
			Constraints: CodeConstraints{
				MaxFileSizeLines:      defaultMaxFileSizeLines,
				PreserveFormatting:    true,
				MaintainCompatibility: true,
			},
		})
		if genErr != nil {
			log.WithError(genErr).Warn("failed to generate code for step",
				"step", step.StepNumber,
			)
			continue
		}

		// Convert file changes to patches
		for _, change := range codeResult.FileChanges {
			patch := Patch{
				FilePath:   change.FilePath,
				NewContent: change.Content,
				Action:     string(change.Action),
			}

			// Read old content for modify actions
			if change.Action == FileActionModify {
				oldPath := filepath.Join(req.WorkspacePath, change.FilePath)
				if oldContent, readErr := os.ReadFile(oldPath); readErr == nil {
					patch.OldContent = string(oldContent)
				}
			}

			patches = append(patches, patch)
		}

		commitMessages = append(commitMessages, codeResult.CommitMessage)
	}

	// Build final commit message
	commitMessage := c.buildCommitMessage(req.Specification.Title, commitMessages)

	// Get total tokens used
	usage := c.client.GetUsage()

	log.Info("patch generation completed",
		"patches", len(patches),
		"total_tokens", usage.TotalTokens,
	)

	return &GeneratePatchResponse{
		Patches:       patches,
		CommitMessage: commitMessage,
		TokensUsed:    usage.TotalTokens,
	}, nil
}

// buildCodebaseContext creates a context string from the codebase.
func (c *BAMLClient) buildCodebaseContext(workspacePath string) string {
	var sb strings.Builder

	// Walk the directory and collect key files
	keyFiles := []string{
		"README.md",
		"go.mod",
		"package.json",
		"Cargo.toml",
		"requirements.txt",
		"pyproject.toml",
	}

	sb.WriteString("## Project Overview\n\n")

	for _, file := range keyFiles {
		path := filepath.Join(workspacePath, file)
		if content, err := os.ReadFile(path); err == nil {
			fmt.Fprintf(
				&sb,
				"### %s\n```\n%s\n```\n\n",
				file,
				truncateContent(string(content), maxKeyFileContentLength),
			)
		}
	}

	// Add directory structure
	sb.WriteString("## Directory Structure\n```\n")
	sb.WriteString(c.getProjectStructure(workspacePath))
	sb.WriteString("\n```\n")

	return sb.String()
}

// detectLanguage detects the primary language of the project.
func (c *BAMLClient) detectLanguage(workspacePath string) string {
	// Check for language-specific files
	checks := map[string]string{
		"go.mod":           "go",
		"package.json":     "typescript",
		"Cargo.toml":       "rust",
		"requirements.txt": "python",
		"pyproject.toml":   "python",
		"pom.xml":          "java",
		"build.gradle":     "java",
		"pubspec.yaml":     "dart",
	}

	for file, lang := range checks {
		if _, err := os.Stat(filepath.Join(workspacePath, file)); err == nil {
			return lang
		}
	}

	return "unknown"
}

// detectFramework detects the framework being used.
func (c *BAMLClient) detectFramework(workspacePath, language string) string {
	switch language {
	case "go":
		return c.detectGoFramework(workspacePath)
	case "typescript", "javascript":
		return c.detectJSFramework(workspacePath)
	case "python":
		return c.detectPythonFramework(workspacePath)
	case "dart":
		return "flutter"
	}
	return ""
}

// detectGoFramework detects Go frameworks from go.mod.
func (c *BAMLClient) detectGoFramework(workspacePath string) string {
	content, err := os.ReadFile(filepath.Join(workspacePath, "go.mod"))
	if err != nil {
		return ""
	}
	goMod := string(content)
	switch {
	case strings.Contains(goMod, "github.com/gin-gonic/gin"):
		return "gin"
	case strings.Contains(goMod, "github.com/labstack/echo"):
		return "echo"
	case strings.Contains(goMod, "github.com/pitabwire/frame"):
		return "frame"
	}
	return ""
}

// detectJSFramework detects JavaScript/TypeScript frameworks from package.json.
func (c *BAMLClient) detectJSFramework(workspacePath string) string {
	content, err := os.ReadFile(filepath.Join(workspacePath, "package.json"))
	if err != nil {
		return ""
	}
	pkgJSON := string(content)
	switch {
	case strings.Contains(pkgJSON, "\"react\""):
		return "react"
	case strings.Contains(pkgJSON, "\"vue\""):
		return "vue"
	case strings.Contains(pkgJSON, "\"next\""):
		return "next.js"
	}
	return ""
}

// detectPythonFramework detects Python frameworks from requirements.txt.
func (c *BAMLClient) detectPythonFramework(workspacePath string) string {
	content, err := os.ReadFile(filepath.Join(workspacePath, "requirements.txt"))
	if err != nil {
		return ""
	}
	reqs := string(content)
	switch {
	case strings.Contains(reqs, "django"):
		return "django"
	case strings.Contains(reqs, "flask"):
		return "flask"
	case strings.Contains(reqs, "fastapi"):
		return "fastapi"
	}
	return ""
}

// getProjectStructure returns the project directory structure.
//
//nolint:gocognit // Tree walking naturally has branching complexity
func (c *BAMLClient) getProjectStructure(workspacePath string) string {
	var sb strings.Builder
	maxDepth := 3
	maxEntries := 100
	entries := 0

	var walk func(path string, depth int, prefix string)
	walk = func(path string, depth int, prefix string) {
		if depth > maxDepth || entries > maxEntries {
			return
		}

		dirEntries, err := os.ReadDir(path)
		if err != nil {
			return
		}

		for i, entry := range dirEntries {
			if entries > maxEntries {
				return
			}

			// Skip hidden files and common non-essential directories
			name := entry.Name()
			if strings.HasPrefix(name, ".") ||
				name == "node_modules" ||
				name == "vendor" ||
				name == "__pycache__" ||
				name == "target" ||
				name == "build" {
				continue
			}

			entries++
			isLast := i == len(dirEntries)-1
			connector := "├── "
			if isLast {
				connector = "└── "
			}

			sb.WriteString(prefix + connector + name)
			if entry.IsDir() {
				sb.WriteString("/")
			}
			sb.WriteString("\n")

			if entry.IsDir() {
				newPrefix := prefix + "│   "
				if isLast {
					newPrefix = prefix + "    "
				}
				walk(filepath.Join(path, name), depth+1, newPrefix)
			}
		}
	}

	walk(workspacePath, 0, "")
	return sb.String()
}

// getProjectInfo returns project information.
func (c *BAMLClient) getProjectInfo(workspacePath, language string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Language: %s\n", language)

	framework := c.detectFramework(workspacePath, language)
	if framework != "" {
		fmt.Fprintf(&sb, "Framework: %s\n", framework)
	}

	// Add language-specific info
	if language == "go" {
		if content, err := os.ReadFile(filepath.Join(workspacePath, "go.mod")); err == nil {
			lines := strings.Split(string(content), "\n")
			if len(lines) > 0 {
				fmt.Fprintf(&sb, "Module: %s\n", strings.TrimPrefix(lines[0], "module "))
			}
		}
	}

	return sb.String()
}

// readRelevantFiles reads files relevant to the normalized specification.
func (c *BAMLClient) readRelevantFiles(
	_ context.Context,
	workspacePath string,
	components []ComponentReference,
) map[string]string {
	files := make(map[string]string)

	for _, comp := range components {
		path := filepath.Join(workspacePath, comp.Path)
		if content, err := os.ReadFile(path); err == nil {
			files[comp.Path] = truncateContent(string(content), maxFileContentLength)
		}
	}

	return files
}

// readFilesForStep reads files needed for a plan step.
func (c *BAMLClient) readFilesForStep(
	ctx context.Context,
	workspacePath string,
	step PlanStep,
) map[string]string {
	files := make(map[string]string)

	for _, target := range step.TargetFiles {
		path := filepath.Join(workspacePath, target.Path)
		if content, err := os.ReadFile(path); err == nil {
			files[target.Path] = string(content)
		} else if target.Action == FileActionModify {
			// File should exist for modify, log warning
			util.Log(ctx).Warn("file not found for modify action",
				"path", target.Path,
			)
		}
	}

	return files
}

// buildCommitMessage builds a final commit message from step messages.
func (c *BAMLClient) buildCommitMessage(title string, stepMessages []string) string {
	if len(stepMessages) == 0 {
		return fmt.Sprintf("feat: %s", title)
	}

	if len(stepMessages) == 1 {
		return stepMessages[0]
	}

	// Combine messages
	var sb strings.Builder
	fmt.Fprintf(&sb, "feat: %s\n\n", title)
	for _, msg := range stepMessages {
		fmt.Fprintf(&sb, "- %s\n", msg)
	}

	return sb.String()
}

// truncateContent truncates content to a maximum length.
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "\n... [truncated]"
}
