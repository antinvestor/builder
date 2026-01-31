// =============================================================================
// BAML Function Definitions for Feature Service
// =============================================================================
// These functions define the LLM interactions used throughout the pipeline.
// Each function has typed inputs and outputs for reliable parsing.
// =============================================================================

// -----------------------------------------------------------------------------
// Specification Normalization
// -----------------------------------------------------------------------------

function NormalizeSpecification(
  spec: FeatureSpecification,
  codebase_context: string,
  language: string
) -> NormalizedSpecification {
  client Anthropic
  prompt #"
    You are an expert software architect analyzing feature requests.

    ## Task
    Analyze the following feature specification and produce a normalized,
    structured understanding of what needs to be built.

    ## Feature Specification
    Title: {{ spec.title }}
    Description: {{ spec.description }}
    Category: {{ spec.category }}
    {% if spec.acceptance_criteria %}
    User-provided Acceptance Criteria:
    {% for criterion in spec.acceptance_criteria %}
    - {{ criterion }}
    {% endfor %}
    {% endif %}
    {% if spec.path_hints %}
    Path Hints: {{ spec.path_hints | join(", ") }}
    {% endif %}
    {% if spec.additional_context %}
    Additional Context: {{ spec.additional_context }}
    {% endif %}

    ## Codebase Context
    {{ codebase_context }}

    ## Primary Language
    {{ language }}

    ## Instructions
    1. Create a clear problem statement
    2. Identify specific behavior changes needed
    3. List components that will be affected
    4. Define testable acceptance criteria
    5. Identify potential risks
    6. Assess the complexity

    Be specific and actionable. Reference actual files and components
    from the codebase context when possible.

    {{ ctx.output_format }}
  "#
}

// -----------------------------------------------------------------------------
// Impact Analysis
// -----------------------------------------------------------------------------

function AnalyzeImpact(
  normalized_spec: NormalizedSpecification,
  file_contents: map<string, string>,
  project_structure: string
) -> ImpactAnalysisResult {
  client Anthropic
  prompt #"
    You are an expert code analyst performing impact analysis.

    ## Task
    Analyze which files and components will be affected by implementing
    this feature specification.

    ## Normalized Specification
    Problem: {{ normalized_spec.problem_statement }}

    Behavior Changes:
    {% for change in normalized_spec.behavior_changes %}
    - {{ change.component }}: {{ change.current_behavior }} → {{ change.desired_behavior }} ({{ change.change_type }})
    {% endfor %}

    Target Components:
    {% for component in normalized_spec.components %}
    - {{ component.name }} ({{ component.path }}): {{ component.purpose }}
    {% endfor %}

    ## Project Structure
    {{ project_structure }}

    ## File Contents
    {% for path, content in file_contents %}
    ### {{ path }}
    ```
    {{ content }}
    ```
    {% endfor %}

    ## Instructions
    1. Identify files that will be directly modified
    2. Identify files affected indirectly through dependencies
    3. Map the dependency relationships
    4. Analyze test coverage for affected code
    5. Recommend new tests needed

    Be thorough but precise. Include confidence scores for each impact.

    {{ ctx.output_format }}
  "#
}

// -----------------------------------------------------------------------------
// Plan Generation
// -----------------------------------------------------------------------------

function GeneratePlan(
  normalized_spec: NormalizedSpecification,
  impact_analysis: ImpactAnalysisResult,
  file_contents: map<string, string>,
  project_info: string
) -> ImplementationPlan {
  client Anthropic
  prompt #"
    You are an expert software architect creating implementation plans.

    ## Task
    Create a detailed, step-by-step implementation plan for this feature.

    ## Normalized Specification
    Problem: {{ normalized_spec.problem_statement }}
    Complexity: {{ normalized_spec.complexity.level }}

    Acceptance Criteria:
    {% for criterion in normalized_spec.acceptance_criteria %}
    - {{ criterion.criterion }} ({{ criterion.verification_method }})
    {% endfor %}

    Risks:
    {% for risk in normalized_spec.risks %}
    - {{ risk.level }}: {{ risk.description }} → {{ risk.mitigation }}
    {% endfor %}

    ## Impact Analysis
    Direct Impacts:
    {% for impact in impact_analysis.direct_impacts %}
    - {{ impact.file_path }} ({{ impact.impact_type }}): {{ impact.rationale }}
    {% endfor %}

    Test Coverage:
    {% for gap in impact_analysis.test_coverage.coverage_gaps %}
    - {{ gap.file_path }}: {{ gap.symbol }} - {{ gap.reason }}
    {% endfor %}

    ## Project Info
    {{ project_info }}

    ## Current File Contents
    {% for path, content in file_contents %}
    ### {{ path }}
    ```
    {{ content }}
    ```
    {% endfor %}

    ## Instructions
    1. Break the implementation into atomic, sequential steps
    2. Each step should modify 1-3 files maximum
    3. Include verification for each step (build, test, lint)
    4. Order steps to minimize dependencies
    5. Include test steps after implementation steps
    6. Estimate token usage for each step

    The plan should be executable without human intervention.

    {{ ctx.output_format }}
  "#
}

// -----------------------------------------------------------------------------
// Code Generation
// -----------------------------------------------------------------------------

function GenerateCode(
  request: CodeGenerationRequest
) -> CodeGenerationResult {
  client Anthropic
  prompt #"
    You are an expert software engineer implementing code changes.

    ## Task
    Implement step {{ request.step.step_number }}: {{ request.step.action }}

    ## Rationale
    {{ request.step.rationale }}

    ## Target Files
    {% for target in request.step.target_files %}
    - {{ target.path }} ({{ target.action }}): {{ target.reason }}
    {% endfor %}

    ## Expected Outcome
    {{ request.step.expected_outcome }}

    ## Context
    Language: {{ request.context.language }}
    {% if request.context.framework %}Framework: {{ request.context.framework }}{% endif %}
    {% if request.context.style_guide %}Style Guide: {{ request.context.style_guide }}{% endif %}

    ## Current File Contents
    {% for path, content in request.context.file_contents %}
    ### {{ path }}
    ```{{ request.context.language }}
    {{ content }}
    ```
    {% endfor %}

    ## Constraints
    - Max file size: {{ request.constraints.max_file_size_lines }} lines
    - Preserve formatting: {{ request.constraints.preserve_formatting }}
    - Maintain compatibility: {{ request.constraints.maintain_compatibility }}
    {% if request.constraints.forbidden_patterns %}
    - Forbidden patterns: {{ request.constraints.forbidden_patterns | join(", ") }}
    {% endif %}

    ## Instructions
    1. Generate the minimal code changes needed
    2. Follow existing code style and patterns
    3. Include appropriate comments where non-obvious
    4. Ensure code is syntactically correct
    5. Suggest an appropriate commit message

    {{ ctx.output_format }}
  "#
}

// -----------------------------------------------------------------------------
// Code Review
// -----------------------------------------------------------------------------

function ReviewCode(
  request: ReviewRequest
) -> ReviewAssessment {
  client Anthropic
  prompt #"
    You are an expert code reviewer performing thorough code review.

    ## Task
    Review the following code changes for quality, correctness, and security.

    ## Feature Context
    {{ request.context.feature_description }}

    ## Acceptance Criteria
    {% for criterion in request.context.acceptance_criteria %}
    - {{ criterion }}
    {% endfor %}

    ## Language
    {{ request.context.language }}

    {% if request.context.coding_standards %}
    ## Coding Standards
    {{ request.context.coding_standards }}
    {% endif %}

    ## Files to Review
    {% for file in request.files_to_review %}
    ### {{ file.path }}
    {% if file.diff %}
    #### Diff
    ```diff
    {{ file.diff }}
    ```
    {% endif %}
    #### Content
    ```{{ request.context.language }}
    {{ file.content }}
    ```
    {% endfor %}

    ## Instructions
    1. Check for bugs and logic errors
    2. Identify security vulnerabilities
    3. Assess performance implications
    4. Review code style and maintainability
    5. Verify acceptance criteria are met
    6. Calculate quality metrics
    7. Provide actionable feedback

    Be thorough but constructive. Prioritize issues by severity.

    {{ ctx.output_format }}
  "#
}

// -----------------------------------------------------------------------------
// Test Generation
// -----------------------------------------------------------------------------

function GenerateTests(
  request: TestGenerationRequest
) -> TestGenerationResult {
  client Anthropic
  prompt #"
    You are an expert test engineer generating comprehensive tests.

    ## Task
    Generate tests for the following code to achieve {{ request.coverage_target }}% coverage.

    ## Target File
    {{ request.target_file }}

    ## Target Content
    ```
    {{ request.target_content }}
    ```

    ## Test Framework
    {{ request.test_framework }}

    {% if request.existing_tests %}
    ## Existing Tests
    ```
    {{ request.existing_tests }}
    ```
    {% endif %}

    ## Instructions
    1. Generate unit tests for all public functions/methods
    2. Include edge cases and error conditions
    3. Test both happy path and failure scenarios
    4. Follow the existing test patterns if available
    5. Use descriptive test names
    6. Include setup/teardown as needed

    Prioritize test quality over quantity.

    {{ ctx.output_format }}
  "#
}

// -----------------------------------------------------------------------------
// Iteration Planning
// -----------------------------------------------------------------------------

function PlanIteration(
  request: IterationRequest
) -> IterationPlan {
  client Anthropic
  prompt #"
    You are an expert debugger planning how to fix implementation issues.

    ## Task
    Analyze the failures and plan how to fix them.

    ## Original Plan
    {{ request.original_plan.title }}
    Total Steps: {{ request.original_plan.steps | length }}

    {% if request.failed_step %}
    ## Failed Step
    Step {{ request.failed_step }}: {{ request.original_plan.steps[request.failed_step - 1].action }}
    {% endif %}

    ## Issues to Address
    {% for issue in request.issues %}
    - [{{ issue.severity }}] {{ issue.type }}: {{ issue.description }}
      {% if issue.file_path %}File: {{ issue.file_path }}{% endif %}
      {% if issue.line_number %}Line: {{ issue.line_number }}{% endif %}
    {% endfor %}

    ## Context
    Previous Attempts: {{ request.context.previous_attempts }}

    {% if request.context.build_output %}
    ### Build Output
    ```
    {{ request.context.build_output }}
    ```
    {% endif %}

    {% if request.context.test_output %}
    ### Test Output
    ```
    {{ request.context.test_output }}
    ```
    {% endif %}

    {% if request.context.lint_output %}
    ### Lint Output
    ```
    {{ request.context.lint_output }}
    ```
    {% endif %}

    ## Instructions
    1. Analyze the root cause of failures
    2. Determine the best strategy (fix, retry, replan)
    3. Identify specific files that need changes
    4. Provide context for the fix

    Be specific and actionable. Focus on the root cause.

    {{ ctx.output_format }}
  "#
}

// -----------------------------------------------------------------------------
// Commit Message Generation
// -----------------------------------------------------------------------------

function GenerateCommitMessage(
  request: CommitMessageRequest
) -> CommitMessage {
  client Anthropic
  prompt #"
    You are generating a conventional commit message.

    ## Task
    Generate a commit message following conventional commits format.

    ## Feature Context
    {{ request.feature_context }}

    ## Step Description
    {{ request.step_description }}

    ## File Changes
    {% for change in request.file_changes %}
    - {{ change.action }}: {{ change.file_path }} - {{ change.description }}
    {% endfor %}

    ## Instructions
    1. Use conventional commit format (type: subject)
    2. Subject line max 72 characters
    3. Body should explain "why" not "what"
    4. Reference related components

    {{ ctx.output_format }}
  "#
}
