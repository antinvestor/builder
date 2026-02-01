package llm

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// PromptBuilder builds prompts for LLM functions.
type PromptBuilder struct {
	templates map[Function]*template.Template
}

// NewPromptBuilder creates a new prompt builder.
func NewPromptBuilder() (*PromptBuilder, error) {
	pb := &PromptBuilder{
		templates: make(map[Function]*template.Template),
	}

	// Register all templates
	templates := map[Function]string{
		FunctionNormalizeSpec:  normalizeSpecTemplate,
		FunctionAnalyzeImpact:  analyzeImpactTemplate,
		FunctionGeneratePlan:   generatePlanTemplate,
		FunctionGenerateCode:   generateCodeTemplate,
		FunctionReviewCode:     reviewCodeTemplate,
		FunctionGenerateTests:  generateTestsTemplate,
		FunctionPlanIteration:  planIterationTemplate,
		FunctionGenerateCommit: generateCommitTemplate,
	}

	for fn, tmpl := range templates {
		t, err := template.New(string(fn)).Funcs(templateFuncs).Parse(tmpl)
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", fn, err)
		}
		pb.templates[fn] = t
	}

	return pb, nil
}

// Build builds a prompt for the given function and data.
func (pb *PromptBuilder) Build(fn Function, data any) (string, error) {
	t, ok := pb.templates[fn]
	if !ok {
		return "", fmt.Errorf("unknown function: %s", fn)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// templateFuncs provides template helper functions.
//
//nolint:gochecknoglobals // Template functions are inherently global
var templateFuncs = template.FuncMap{
	"join": strings.Join,
	"indent": func(indent int, s string) string {
		prefix := strings.Repeat("  ", indent)
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			if line != "" {
				lines[i] = prefix + line
			}
		}
		return strings.Join(lines, "\n")
	},
	"sub": func(a, b int) int {
		return a - b
	},
}

// NormalizeSpecInput is the input for NormalizeSpecification.
type NormalizeSpecInput struct {
	Spec            FeatureSpecification
	CodebaseContext string
	Language        string
}

const normalizeSpecTemplate = `You are an expert software architect analyzing feature requests.

## Task
Analyze the following feature specification and produce a normalized,
structured understanding of what needs to be built.

## Feature Specification
Title: {{.Spec.Title}}
Description: {{.Spec.Description}}
Category: {{.Spec.Category}}
{{- if .Spec.AcceptanceCriteria}}

User-provided Acceptance Criteria:
{{- range .Spec.AcceptanceCriteria}}
- {{.}}
{{- end}}
{{- end}}
{{- if .Spec.PathHints}}

Path Hints: {{join .Spec.PathHints ", "}}
{{- end}}
{{- if .Spec.AdditionalContext}}

Additional Context: {{.Spec.AdditionalContext}}
{{- end}}

## Codebase Context
{{.CodebaseContext}}

## Primary Language
{{.Language}}

## Instructions
1. Create a clear problem statement
2. Identify specific behavior changes needed
3. List components that will be affected
4. Define testable acceptance criteria
5. Identify potential risks
6. Assess the complexity

Be specific and actionable. Reference actual files and components
from the codebase context when possible.

Respond with a JSON object matching this schema:
{
  "problem_statement": "string - clear problem statement",
  "behavior_changes": [
    {
      "component": "string - component name",
      "current_behavior": "string - current behavior",
      "desired_behavior": "string - desired behavior",
      "change_type": "add|modify|remove|refactor"
    }
  ],
  "components": [
    {
      "name": "string - component name",
      "path": "string - file path",
      "type": "file|function|class|module|package|config|test",
      "purpose": "string - why involved"
    }
  ],
  "acceptance_criteria": [
    {
      "criterion": "string - the criterion",
      "testable": true|false,
      "verification_method": "string - how to verify"
    }
  ],
  "risks": [
    {
      "description": "string - risk description",
      "level": "low|medium|high|critical",
      "mitigation": "string - mitigation strategy"
    }
  ],
  "complexity": {
    "level": "trivial|simple|moderate|complex|very_complex",
    "estimated_steps": number,
    "estimated_files": number,
    "rationale": "string - reasoning"
  }
}`

// AnalyzeImpactInput is the input for AnalyzeImpact.
type AnalyzeImpactInput struct {
	NormalizedSpec   NormalizedSpecification
	FileContents     map[string]string
	ProjectStructure string
}

const analyzeImpactTemplate = `You are an expert code analyst performing impact analysis.

## Task
Analyze which files and components will be affected by implementing
this feature specification.

## Normalized Specification
Problem: {{.NormalizedSpec.ProblemStatement}}

Behavior Changes:
{{- range .NormalizedSpec.BehaviorChanges}}
- {{.Component}}: {{.CurrentBehavior}} → {{.DesiredBehavior}} ({{.ChangeType}})
{{- end}}

Target Components:
{{- range .NormalizedSpec.Components}}
- {{.Name}} ({{.Path}}): {{.Purpose}}
{{- end}}

## Project Structure
{{.ProjectStructure}}

## File Contents
{{- range $path, $content := .FileContents}}

### {{$path}}
` + "```" + `
{{$content}}
` + "```" + `
{{- end}}

## Instructions
1. Identify files that will be directly modified
2. Identify files affected indirectly through dependencies
3. Map the dependency relationships
4. Analyze test coverage for affected code
5. Recommend new tests needed

Be thorough but precise. Include confidence scores for each impact.

Respond with a JSON object matching this schema:
{
  "direct_impacts": [
    {
      "file_path": "string",
      "impact_type": "create|modify|delete|rename|dependency",
      "confidence": 0.0-1.0,
      "rationale": "string",
      "affected_symbols": ["string"] (optional)
    }
  ],
  "indirect_impacts": [...same structure...],
  "dependency_graph": {
    "nodes": [
      {
        "id": "string",
        "path": "string",
        "symbol": "string" (optional),
        "node_type": "file|function|class|variable|type"
      }
    ],
    "edges": [
      {
        "from_id": "string",
        "to_id": "string",
        "edge_type": "imports|calls|extends|implements|uses"
      }
    ]
  },
  "test_coverage": {
    "covering_tests": [
      {
        "file_path": "string",
        "test_name": "string",
        "covers_files": ["string"]
      }
    ],
    "coverage_gaps": [
      {
        "file_path": "string",
        "symbol": "string",
        "reason": "string"
      }
    ],
    "recommendations": [
      {
        "target_file": "string",
        "target_symbol": "string",
        "test_type": "string",
        "description": "string"
      }
    ]
  }
}`

// GeneratePlanInput is the input for GeneratePlan.
type GeneratePlanInput struct {
	NormalizedSpec NormalizedSpecification
	ImpactAnalysis ImpactAnalysisResult
	FileContents   map[string]string
	ProjectInfo    string
}

const generatePlanTemplate = `You are an expert software architect creating implementation plans.

## Task
Create a detailed, step-by-step implementation plan for this feature.

## Normalized Specification
Problem: {{.NormalizedSpec.ProblemStatement}}
Complexity: {{.NormalizedSpec.Complexity.Level}}

Acceptance Criteria:
{{- range .NormalizedSpec.AcceptanceCriteria}}
- {{.Criterion}} ({{.VerificationMethod}})
{{- end}}

Risks:
{{- range .NormalizedSpec.Risks}}
- {{.Level}}: {{.Description}} → {{.Mitigation}}
{{- end}}

## Impact Analysis
Direct Impacts:
{{- range .ImpactAnalysis.DirectImpacts}}
- {{.FilePath}} ({{.ImpactType}}): {{.Rationale}}
{{- end}}

Test Coverage:
{{- range .ImpactAnalysis.TestCoverage.CoverageGaps}}
- {{.FilePath}}: {{.Symbol}} - {{.Reason}}
{{- end}}

## Project Info
{{.ProjectInfo}}

## Current File Contents
{{- range $path, $content := .FileContents}}

### {{$path}}
` + "```" + `
{{$content}}
` + "```" + `
{{- end}}

## Instructions
1. Break the implementation into atomic, sequential steps
2. Each step should modify 1-3 files maximum
3. Include verification for each step (build, test, lint)
4. Order steps to minimize dependencies
5. Include test steps after implementation steps
6. Estimate token usage for each step

The plan should be executable without human intervention.

Respond with a JSON object matching this schema:
{
  "plan_id": "string - unique identifier",
  "title": "string - human readable title",
  "summary": "string - executive summary",
  "steps": [
    {
      "step_number": number (1-based),
      "action": "string - what this step does",
      "rationale": "string - why needed",
      "target_files": [
        {
          "path": "string",
          "action": "create|modify|delete|rename",
          "change_type": "implementation|test|config|documentation|dependency",
          "reason": "string"
        }
      ],
      "prerequisites": [number] (optional - step numbers),
      "expected_outcome": "string",
      "verification": {
        "method": "syntax_check|build|test|lint|manual",
        "command": "string" (optional),
        "expected": "string" (optional),
        "description": "string"
      },
      "estimated_tokens": number
    }
  ],
  "dependencies": [
    {
      "from_step": number,
      "to_step": number,
      "dependency_type": "strict|soft|data",
      "reason": "string"
    }
  ],
  "estimated_metrics": {
    "total_steps": number,
    "estimated_tokens": number,
    "estimated_files_total": number,
    "estimated_new_files": number,
    "estimated_modified": number,
    "estimated_deleted": number
  },
  "assumptions": ["string"] (optional),
  "alternatives_considered": [
    {
      "approach": "string",
      "pros": "string",
      "cons": "string",
      "why_rejected": "string"
    }
  ] (optional)
}`

// GenerateCodeInput is the input for GenerateCode.
type GenerateCodeInput struct {
	Step         PlanStep
	FileContents map[string]string
	Language     string
	Framework    string
	StyleGuide   string
	Constraints  CodeConstraints
}

// CodeConstraints defines constraints on code generation.
type CodeConstraints struct {
	MaxFileSizeLines      int      `json:"max_file_size_lines"`
	PreserveFormatting    bool     `json:"preserve_formatting"`
	MaintainCompatibility bool     `json:"maintain_compatibility"`
	AllowedImports        []string `json:"allowed_imports,omitempty"`
	ForbiddenPatterns     []string `json:"forbidden_patterns,omitempty"`
}

const generateCodeTemplate = `You are an expert software engineer implementing code changes.

## Task
Implement step {{.Step.StepNumber}}: {{.Step.Action}}

## Rationale
{{.Step.Rationale}}

## Target Files
{{- range .Step.TargetFiles}}
- {{.Path}} ({{.Action}}): {{.Reason}}
{{- end}}

## Expected Outcome
{{.Step.ExpectedOutcome}}

## Context
Language: {{.Language}}
{{- if .Framework}}
Framework: {{.Framework}}
{{- end}}
{{- if .StyleGuide}}
Style Guide: {{.StyleGuide}}
{{- end}}

## Current File Contents
{{- range $path, $content := .FileContents}}

### {{$path}}
` + "```" + `{{$.Language}}
{{$content}}
` + "```" + `
{{- end}}

## Constraints
- Max file size: {{.Constraints.MaxFileSizeLines}} lines
- Preserve formatting: {{.Constraints.PreserveFormatting}}
- Maintain compatibility: {{.Constraints.MaintainCompatibility}}
{{- if .Constraints.ForbiddenPatterns}}
- Forbidden patterns: {{join .Constraints.ForbiddenPatterns ", "}}
{{- end}}

## Instructions
1. Generate the minimal code changes needed
2. Follow existing code style and patterns
3. Include appropriate comments where non-obvious
4. Ensure code is syntactically correct
5. Suggest an appropriate commit message

Respond with a JSON object matching this schema:
{
  "file_changes": [
    {
      "file_path": "string",
      "action": "create|modify|delete|rename",
      "previous_path": "string" (optional, for rename),
      "content": "string" (full file content for create/modify),
      "patch": "string" (optional unified diff),
      "description": "string - what changed"
    }
  ],
  "commit_message": "string - conventional commit format",
  "notes": "string" (optional)
}`

const reviewCodeTemplate = `You are an expert code reviewer performing thorough code review.

## Task
Review the following code changes for quality, correctness, and security.

## Feature Context
{{.Context.FeatureDescription}}

## Acceptance Criteria
{{- range .Context.AcceptanceCriteria}}
- {{.}}
{{- end}}

## Language
{{.Context.Language}}
{{- if .Context.CodingStandards}}

## Coding Standards
{{.Context.CodingStandards}}
{{- end}}

## Files to Review
{{- range .FilesToReview}}

### {{.Path}}
{{- if .Diff}}
#### Diff
` + "```diff" + `
{{.Diff}}
` + "```" + `
{{- end}}
#### Content
` + "```" + `{{$.Context.Language}}
{{.Content}}
` + "```" + `
{{- end}}

## Instructions
1. Check for bugs and logic errors
2. Identify security vulnerabilities
3. Assess performance implications
4. Review code style and maintainability
5. Verify acceptance criteria are met
6. Calculate quality metrics
7. Provide actionable feedback

Be thorough but constructive. Prioritize issues by severity.

Respond with a JSON object matching this schema:
{
  "overall_score": 0-100,
  "recommendation": "approve|approve_with_minor_changes|request_changes|reject",
  "summary": "string - executive summary",
  "issues": [
    {
      "id": "string",
      "type": "bug|security|performance|maintainability|style|documentation|complexity|dead_code|duplication",
      "severity": "info|low|medium|high|critical",
      "file_path": "string",
      "line_start": number,
      "line_end": number,
      "title": "string",
      "description": "string",
      "suggestion": "string" (optional),
      "code_snippet": "string" (optional)
    }
  ],
  "suggestions": [
    {
      "title": "string",
      "description": "string",
      "file_path": "string" (optional),
      "line_number": number (optional),
      "priority": "low|medium|high"
    }
  ],
  "metrics": {
    "cyclomatic_complexity": number,
    "cognitive_complexity": number,
    "lines_of_code": number,
    "lines_of_comments": number,
    "comment_ratio": number,
    "duplication_percent": number
  }
}`

const generateTestsTemplate = `You are an expert test engineer generating comprehensive tests.

## Task
Generate tests for the following code to achieve {{.CoverageTarget}}%% coverage.

## Target File
{{.TargetFile}}

## Target Content
` + "```" + `
{{.TargetContent}}
` + "```" + `

## Test Framework
{{.TestFramework}}
{{- if .ExistingTests}}

## Existing Tests
` + "```" + `
{{.ExistingTests}}
` + "```" + `
{{- end}}

## Instructions
1. Generate unit tests for all public functions/methods
2. Include edge cases and error conditions
3. Test both happy path and failure scenarios
4. Follow the existing test patterns if available
5. Use descriptive test names
6. Include setup/teardown as needed

Prioritize test quality over quantity.

Respond with a JSON object matching this schema:
{
  "test_file_path": "string",
  "test_content": "string - full test file content",
  "test_cases": [
    {
      "name": "string",
      "description": "string",
      "test_type": "unit|integration|e2e|snapshot|property",
      "target_function": "string"
    }
  ],
  "coverage_estimate": number (0.0-1.0)
}`

const planIterationTemplate = `You are an expert debugger planning how to fix implementation issues.

## Task
Analyze the failures and plan how to fix them.

## Original Plan
{{.OriginalPlan.Title}}
Total Steps: {{len .OriginalPlan.Steps}}
{{- if .FailedStep}}

## Failed Step
Step {{.FailedStep}}: {{index .OriginalPlan.Steps (sub .FailedStep 1) | .Action}}
{{- end}}

## Issues to Address
{{- range .Issues}}
- [{{.Severity}}] {{.Type}}: {{.Description}}
{{- if .FilePath}}
  File: {{.FilePath}}
{{- end}}
{{- if .LineNumber}}
  Line: {{.LineNumber}}
{{- end}}
{{- end}}

## Context
Previous Attempts: {{.Context.PreviousAttempts}}
{{- if .Context.BuildOutput}}

### Build Output
` + "```" + `
{{.Context.BuildOutput}}
` + "```" + `
{{- end}}
{{- if .Context.TestOutput}}

### Test Output
` + "```" + `
{{.Context.TestOutput}}
` + "```" + `
{{- end}}
{{- if .Context.LintOutput}}

### Lint Output
` + "```" + `
{{.Context.LintOutput}}
` + "```" + `
{{- end}}

## Instructions
1. Analyze the root cause of failures
2. Determine the best strategy (fix, retry, replan)
3. Identify specific files that need changes
4. Provide context for the fix

Be specific and actionable. Focus on the root cause.

Respond with a JSON object matching this schema:
{
  "strategy": "fix|retry|replan|partial",
  "steps_to_retry": [number],
  "files_to_fix": ["string"],
  "additional_context": "string" (optional)
}`

const generateCommitTemplate = `You are generating a conventional commit message.

## Task
Generate a commit message following conventional commits format.

## Feature Context
{{.FeatureContext}}

## Step Description
{{.StepDescription}}

## File Changes
{{- range .FileChanges}}
- {{.Action}}: {{.FilePath}} - {{.Description}}
{{- end}}

## Instructions
1. Use conventional commit format (type: subject)
2. Subject line max 72 characters
3. Body should explain "why" not "what"
4. Reference related components

Respond with a JSON object matching this schema:
{
  "subject": "string - max 72 chars",
  "body": "string" (optional),
  "type": "feat|fix|refactor|test|docs|chore|style|perf"
}`
