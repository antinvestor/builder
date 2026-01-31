// =============================================================================
// BAML Type Definitions for Feature Service
// =============================================================================
// These types define the structured inputs and outputs for LLM functions
// used throughout the autonomous feature-building pipeline.
// =============================================================================

// -----------------------------------------------------------------------------
// Specification Normalization Types
// -----------------------------------------------------------------------------

class FeatureSpecification {
  title string @description("Human-readable title for the feature")
  description string @description("Detailed feature description")
  acceptance_criteria string[]? @description("Optional specific criteria to satisfy")
  path_hints string[]? @description("Optional file/path hints from user")
  additional_context string? @description("User-provided context (docs, examples)")
  category FeatureCategory @description("Feature category for routing")
}

enum FeatureCategory {
  NewFeature @alias("new_feature") @description("Entirely new functionality")
  BugFix @alias("bug_fix") @description("Fix for existing bug")
  Refactor @alias("refactor") @description("Code restructuring without behavior change")
  Documentation @alias("documentation") @description("Documentation updates")
  Test @alias("test") @description("Test additions or updates")
  Dependency @alias("dependency") @description("Dependency updates")
}

class NormalizedSpecification {
  problem_statement string @description("Clear, concise problem statement")
  behavior_changes BehaviorChange[] @description("Expected behavior changes")
  components ComponentReference[] @description("Identified components to modify")
  acceptance_criteria AcceptanceCriterion[] @description("Explicit acceptance criteria")
  risks Risk[] @description("Potential risks identified")
  complexity ComplexityAssessment @description("Estimated complexity")
}

class BehaviorChange {
  component string @description("Component being changed")
  current_behavior string @description("Current behavior description")
  desired_behavior string @description("Desired behavior after change")
  change_type ChangeType @description("Type of change")
}

enum ChangeType {
  Add @alias("add")
  Modify @alias("modify")
  Remove @alias("remove")
  Refactor @alias("refactor")
}

class ComponentReference {
  name string @description("Component name")
  path string @description("File path")
  type ComponentType @description("Component type")
  purpose string @description("Why this component is involved")
}

enum ComponentType {
  File @alias("file")
  Function @alias("function")
  Class @alias("class")
  Module @alias("module")
  Package @alias("package")
  Config @alias("config")
  Test @alias("test")
}

class AcceptanceCriterion {
  criterion string @description("The acceptance criterion")
  testable bool @description("Whether this can be automatically tested")
  verification_method string @description("How to verify this criterion")
}

class Risk {
  description string @description("Risk description")
  level RiskLevel @description("Risk severity level")
  mitigation string @description("How to mitigate this risk")
}

enum RiskLevel {
  Low @alias("low")
  Medium @alias("medium")
  High @alias("high")
  Critical @alias("critical")
}

class ComplexityAssessment {
  level ComplexityLevel @description("Complexity level")
  estimated_steps int @description("Estimated number of implementation steps")
  estimated_files int @description("Estimated number of files to modify")
  rationale string @description("Reasoning for complexity assessment")
}

enum ComplexityLevel {
  Trivial @alias("trivial") @description("< 30 minutes, single file")
  Simple @alias("simple") @description("30-60 minutes, 2-3 files")
  Moderate @alias("moderate") @description("1-3 hours, 4-6 files")
  Complex @alias("complex") @description("3-8 hours, 7-15 files")
  VeryComplex @alias("very_complex") @description("8+ hours, 15+ files")
}

// -----------------------------------------------------------------------------
// Impact Analysis Types
// -----------------------------------------------------------------------------

class ImpactAnalysisResult {
  direct_impacts FileImpact[] @description("Files directly affected by the change")
  indirect_impacts FileImpact[] @description("Files indirectly affected via dependencies")
  dependency_graph DependencyGraph @description("Code dependency relationships")
  test_coverage TestCoverageAnalysis @description("Test coverage analysis")
}

class FileImpact {
  file_path string @description("Path to the affected file")
  impact_type ImpactType @description("Type of impact")
  confidence float @description("Confidence score 0.0-1.0")
  rationale string @description("Why this file is affected")
  affected_symbols string[]? @description("Specific symbols affected")
}

enum ImpactType {
  Create @alias("create") @description("New file to create")
  Modify @alias("modify") @description("Existing file to modify")
  Delete @alias("delete") @description("File to delete")
  Rename @alias("rename") @description("File to rename")
  Dependency @alias("dependency") @description("May need changes due to dependencies")
}

class DependencyGraph {
  nodes DependencyNode[] @description("Nodes in the dependency graph")
  edges DependencyEdge[] @description("Edges connecting nodes")
}

class DependencyNode {
  id string @description("Unique node identifier")
  path string @description("File path")
  symbol string? @description("Symbol name if applicable")
  node_type NodeType @description("Type of node")
}

enum NodeType {
  File @alias("file")
  Function @alias("function")
  Class @alias("class")
  Variable @alias("variable")
  Type @alias("type")
}

class DependencyEdge {
  from_id string @description("Source node ID")
  to_id string @description("Target node ID")
  edge_type EdgeType @description("Type of dependency")
}

enum EdgeType {
  Imports @alias("imports")
  Calls @alias("calls")
  Extends @alias("extends")
  Implements @alias("implements")
  Uses @alias("uses")
}

class TestCoverageAnalysis {
  covering_tests TestReference[] @description("Tests that cover affected code")
  coverage_gaps CoverageGap[] @description("Gaps in test coverage")
  recommendations TestRecommendation[] @description("Recommended new tests")
}

class TestReference {
  file_path string @description("Test file path")
  test_name string @description("Test function/method name")
  covers_files string[] @description("Files this test covers")
}

class CoverageGap {
  file_path string @description("File with coverage gap")
  symbol string @description("Uncovered symbol")
  reason string @description("Why this needs coverage")
}

class TestRecommendation {
  target_file string @description("File to test")
  target_symbol string @description("Symbol to test")
  test_type string @description("Type of test (unit, integration, e2e)")
  description string @description("What the test should verify")
}

// -----------------------------------------------------------------------------
// Plan Generation Types
// -----------------------------------------------------------------------------

class ImplementationPlan {
  plan_id string @description("Unique plan identifier")
  title string @description("Human-readable plan title")
  summary string @description("Executive summary of changes")
  steps PlanStep[] @description("Ordered implementation steps")
  dependencies StepDependency[] @description("Cross-step dependencies")
  estimated_metrics PlanMetrics @description("Estimated execution metrics")
  assumptions string[]? @description("Assumptions made during planning")
  alternatives_considered Alternative[]? @description("Alternative approaches considered")
}

class PlanStep {
  step_number int @description("1-based step number")
  action string @description("What this step does")
  rationale string @description("Why this step is needed")
  target_files TargetFile[] @description("Files this step will modify")
  prerequisites int[]? @description("Step numbers that must complete first")
  expected_outcome string @description("Expected result of this step")
  verification StepVerification @description("How to verify success")
  estimated_tokens int @description("Estimated LLM tokens for this step")
}

class TargetFile {
  path string @description("File path")
  action FileAction @description("Action to take")
  change_type FileChangeType @description("Type of change")
  reason string @description("Why this file is targeted")
}

enum FileAction {
  Create @alias("create")
  Modify @alias("modify")
  Delete @alias("delete")
  Rename @alias("rename")
}

enum FileChangeType {
  Implementation @alias("implementation")
  Test @alias("test")
  Config @alias("config")
  Documentation @alias("documentation")
  Dependency @alias("dependency")
}

class StepVerification {
  method VerificationMethod @description("Verification method")
  command string? @description("Command to run if applicable")
  expected string? @description("Expected outcome")
  description string @description("Description of verification")
}

enum VerificationMethod {
  SyntaxCheck @alias("syntax_check")
  Build @alias("build")
  Test @alias("test")
  Lint @alias("lint")
  Manual @alias("manual")
}

class StepDependency {
  from_step int @description("Dependent step number")
  to_step int @description("Required step number")
  dependency_type DependencyKind @description("Type of dependency")
  reason string @description("Why this dependency exists")
}

enum DependencyKind {
  Strict @alias("strict") @description("Must complete before starting")
  Soft @alias("soft") @description("Should complete, but can proceed")
  Data @alias("data") @description("Needs data from prior step")
}

class PlanMetrics {
  total_steps int @description("Total number of steps")
  estimated_tokens int @description("Total estimated LLM tokens")
  estimated_files_total int @description("Total files affected")
  estimated_new_files int @description("New files to create")
  estimated_modified int @description("Files to modify")
  estimated_deleted int @description("Files to delete")
}

class Alternative {
  approach string @description("Alternative approach description")
  pros string @description("Advantages of this approach")
  cons string @description("Disadvantages of this approach")
  why_rejected string @description("Why this was not chosen")
}

// -----------------------------------------------------------------------------
// Code Generation Types
// -----------------------------------------------------------------------------

class CodeGenerationRequest {
  step PlanStep @description("The step to execute")
  context CodeContext @description("Context for code generation")
  constraints CodeConstraints @description("Constraints on generation")
}

class CodeContext {
  file_contents map<string, string> @description("Contents of relevant files")
  project_structure string @description("Project structure overview")
  language string @description("Primary programming language")
  framework string? @description("Framework being used")
  style_guide string? @description("Code style guidelines")
}

class CodeConstraints {
  max_file_size_lines int @description("Max lines per file")
  preserve_formatting bool @description("Whether to preserve existing formatting")
  maintain_compatibility bool @description("Whether to maintain API compatibility")
  allowed_imports string[]? @description("Allowed import patterns")
  forbidden_patterns string[]? @description("Patterns to avoid")
}

class CodeGenerationResult {
  file_changes FileChange[] @description("Changes to apply")
  commit_message string @description("Suggested commit message")
  notes string? @description("Additional notes about the changes")
}

class FileChange {
  file_path string @description("Path to the file")
  action FileAction @description("Action taken")
  previous_path string? @description("Previous path if renamed")
  content string? @description("New content for create/modify")
  patch string? @description("Unified diff patch")
  description string @description("Description of changes")
}

// -----------------------------------------------------------------------------
// Code Review Types
// -----------------------------------------------------------------------------

class ReviewRequest {
  files_to_review FileToReview[] @description("Files to review")
  context ReviewContext @description("Review context")
}

class FileToReview {
  path string @description("File path")
  content string @description("File content")
  diff string? @description("Diff if available")
}

class ReviewContext {
  feature_description string @description("What the feature does")
  acceptance_criteria string[] @description("Criteria to verify")
  language string @description("Programming language")
  coding_standards string? @description("Coding standards reference")
}

class ReviewAssessment {
  overall_score int @description("Quality score 0-100")
  recommendation ReviewRecommendation @description("Overall recommendation")
  summary string @description("Executive summary")
  issues ReviewIssue[] @description("Issues found")
  suggestions ReviewSuggestion[] @description("Improvement suggestions")
  metrics CodeQualityMetrics @description("Quality metrics")
}

enum ReviewRecommendation {
  Approve @alias("approve")
  ApproveMinor @alias("approve_with_minor_changes")
  RequestChanges @alias("request_changes")
  Reject @alias("reject")
}

class ReviewIssue {
  id string @description("Issue identifier")
  type ReviewIssueType @description("Issue type")
  severity ReviewIssueSeverity @description("Issue severity")
  file_path string @description("File containing the issue")
  line_start int @description("Starting line number")
  line_end int @description("Ending line number")
  title string @description("Issue title")
  description string @description("Detailed description")
  suggestion string? @description("Suggested fix")
  code_snippet string? @description("Problematic code")
}

enum ReviewIssueType {
  Bug @alias("bug")
  Security @alias("security")
  Performance @alias("performance")
  Maintainability @alias("maintainability")
  Style @alias("style")
  Documentation @alias("documentation")
  Complexity @alias("complexity")
  DeadCode @alias("dead_code")
  Duplication @alias("duplication")
}

enum ReviewIssueSeverity {
  Info @alias("info")
  Low @alias("low")
  Medium @alias("medium")
  High @alias("high")
  Critical @alias("critical")
}

class ReviewSuggestion {
  title string @description("Suggestion title")
  description string @description("Detailed description")
  file_path string? @description("Related file path")
  line_number int? @description("Related line number")
  priority string @description("Priority (low, medium, high)")
}

class CodeQualityMetrics {
  cyclomatic_complexity float @description("Average cyclomatic complexity")
  cognitive_complexity float @description("Average cognitive complexity")
  lines_of_code int @description("Total lines of code")
  lines_of_comments int @description("Total comment lines")
  comment_ratio float @description("Comment to code ratio")
  duplication_percent float @description("Code duplication percentage")
}

// -----------------------------------------------------------------------------
// Test Generation Types
// -----------------------------------------------------------------------------

class TestGenerationRequest {
  target_file string @description("File to generate tests for")
  target_content string @description("Content of the target file")
  existing_tests string? @description("Existing test content if any")
  test_framework string @description("Test framework to use")
  coverage_target float @description("Target coverage percentage")
}

class TestGenerationResult {
  test_file_path string @description("Path for the test file")
  test_content string @description("Generated test code")
  test_cases TestCase[] @description("Individual test cases generated")
  coverage_estimate float @description("Estimated coverage")
}

class TestCase {
  name string @description("Test case name")
  description string @description("What this test verifies")
  test_type TestType @description("Type of test")
  target_function string @description("Function being tested")
}

enum TestType {
  Unit @alias("unit")
  Integration @alias("integration")
  E2E @alias("e2e")
  Snapshot @alias("snapshot")
  Property @alias("property")
}

// -----------------------------------------------------------------------------
// Iteration Types
// -----------------------------------------------------------------------------

class IterationRequest {
  original_plan ImplementationPlan @description("Original implementation plan")
  failed_step int? @description("Step that failed")
  issues IterationIssue[] @description("Issues to address")
  context IterationContext @description("Context for iteration")
}

class IterationIssue {
  type string @description("Issue type")
  file_path string? @description("Related file")
  line_number int? @description("Related line")
  description string @description("Issue description")
  severity string @description("Issue severity")
}

class IterationContext {
  build_output string? @description("Build output if applicable")
  test_output string? @description("Test output if applicable")
  lint_output string? @description("Lint output if applicable")
  previous_attempts int @description("Number of previous attempts")
}

class IterationPlan {
  strategy IterationStrategy @description("Iteration strategy")
  steps_to_retry int[] @description("Steps to retry")
  files_to_fix string[] @description("Files to fix")
  additional_context string? @description("Context for fixing")
}

enum IterationStrategy {
  Fix @alias("fix") @description("Fix specific issues")
  Retry @alias("retry") @description("Retry failed steps")
  Replan @alias("replan") @description("Generate new plan")
  Partial @alias("partial") @description("Partial fix, continue")
}

// -----------------------------------------------------------------------------
// Commit Message Types
// -----------------------------------------------------------------------------

class CommitMessageRequest {
  file_changes FileChange[] @description("Changes in this commit")
  feature_context string @description("Feature being implemented")
  step_description string @description("What this step accomplished")
}

class CommitMessage {
  subject string @description("Commit subject line (max 72 chars)")
  body string? @description("Commit body with details")
  type CommitType @description("Commit type")
}

enum CommitType {
  Feat @alias("feat") @description("New feature")
  Fix @alias("fix") @description("Bug fix")
  Refactor @alias("refactor") @description("Code refactoring")
  Test @alias("test") @description("Adding tests")
  Docs @alias("docs") @description("Documentation")
  Chore @alias("chore") @description("Maintenance tasks")
  Style @alias("style") @description("Code style changes")
  Perf @alias("perf") @description("Performance improvement")
}
