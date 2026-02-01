// Package llm provides LLM client implementations for code generation.
package llm

import "time"

// Provider identifies an LLM provider.
type Provider string

// LLM provider constants.
const (
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
	ProviderGoogle    Provider = "google"
)

// Model identifies an LLM model.
type Model string

// Anthropic model constants.
const (
	ModelClaudeSonnet Model = "claude-sonnet-4-20250514"
	ModelClaudeOpus   Model = "claude-opus-4-20250514"
	ModelClaudeHaiku  Model = "claude-3-5-haiku-20241022"
)

// OpenAI model constants.
const (
	ModelGPT4o Model = "gpt-4o"
)

// Google model constants.
const (
	ModelGeminiFlash Model = "gemini-2.0-flash"
)

// Function identifies a BAML function.
type Function string

// BAML function constants.
const (
	FunctionNormalizeSpec  Function = "NormalizeSpecification"
	FunctionAnalyzeImpact  Function = "AnalyzeImpact"
	FunctionGeneratePlan   Function = "GeneratePlan"
	FunctionGenerateCode   Function = "GenerateCode"
	FunctionReviewCode     Function = "ReviewCode"
	FunctionGenerateTests  Function = "GenerateTests"
	FunctionPlanIteration  Function = "PlanIteration"
	FunctionGenerateCommit Function = "GenerateCommitMessage"
)

// Purpose categorizes LLM invocation purposes.
type Purpose string

// Purpose constants.
const (
	PurposeNormalization  Purpose = "normalization"
	PurposeImpactAnalysis Purpose = "impact_analysis"
	PurposePlanning       Purpose = "planning"
	PurposeCodeGeneration Purpose = "code_generation"
	PurposeTestGeneration Purpose = "test_generation"
	PurposeCodeReview     Purpose = "code_review"
	PurposeIteration      Purpose = "iteration"
	PurposeCommitMessage  Purpose = "commit_message"
)

// FeatureCategory identifies the type of feature.
type FeatureCategory string

// Feature category constants.
const (
	CategoryNewFeature    FeatureCategory = "new_feature"
	CategoryBugFix        FeatureCategory = "bug_fix"
	CategoryRefactor      FeatureCategory = "refactor"
	CategoryDocumentation FeatureCategory = "documentation"
	CategoryTest          FeatureCategory = "test"
	CategoryDependency    FeatureCategory = "dependency"
)

// FeatureSpecification is the input specification for a feature.
type FeatureSpecification struct {
	Title              string          `json:"title"`
	Description        string          `json:"description"`
	AcceptanceCriteria []string        `json:"acceptance_criteria,omitempty"`
	PathHints          []string        `json:"path_hints,omitempty"`
	AdditionalContext  string          `json:"additional_context,omitempty"`
	Category           FeatureCategory `json:"category"`
}

// NormalizedSpecification is the output of spec normalization.
type NormalizedSpecification struct {
	ProblemStatement   string                `json:"problem_statement"`
	BehaviorChanges    []BehaviorChange      `json:"behavior_changes"`
	Components         []ComponentReference  `json:"components"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria"`
	Risks              []Risk                `json:"risks"`
	Complexity         ComplexityAssessment  `json:"complexity"`
}

// BehaviorChange describes a change to component behavior.
type BehaviorChange struct {
	Component       string     `json:"component"`
	CurrentBehavior string     `json:"current_behavior"`
	DesiredBehavior string     `json:"desired_behavior"`
	ChangeType      ChangeType `json:"change_type"`
}

// ChangeType identifies the type of change.
type ChangeType string

// Change type constants.
const (
	ChangeTypeAdd      ChangeType = "add"
	ChangeTypeModify   ChangeType = "modify"
	ChangeTypeRemove   ChangeType = "remove"
	ChangeTypeRefactor ChangeType = "refactor"
)

// ComponentReference identifies a component to modify.
type ComponentReference struct {
	Name    string        `json:"name"`
	Path    string        `json:"path"`
	Type    ComponentType `json:"type"`
	Purpose string        `json:"purpose"`
}

// ComponentType identifies the type of component.
type ComponentType string

// Component type constants.
const (
	ComponentTypeFile     ComponentType = "file"
	ComponentTypeFunction ComponentType = "function"
	ComponentTypeClass    ComponentType = "class"
	ComponentTypeModule   ComponentType = "module"
	ComponentTypePackage  ComponentType = "package"
	ComponentTypeConfig   ComponentType = "config"
	ComponentTypeTest     ComponentType = "test"
)

// AcceptanceCriterion defines a testable acceptance criterion.
type AcceptanceCriterion struct {
	Criterion          string `json:"criterion"`
	Testable           bool   `json:"testable"`
	VerificationMethod string `json:"verification_method"`
}

// Risk describes a potential risk.
type Risk struct {
	Description string    `json:"description"`
	Level       RiskLevel `json:"level"`
	Mitigation  string    `json:"mitigation"`
}

// RiskLevel identifies the severity of a risk.
type RiskLevel string

// Risk level constants.
const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// ComplexityAssessment estimates implementation complexity.
type ComplexityAssessment struct {
	Level          ComplexityLevel `json:"level"`
	EstimatedSteps int             `json:"estimated_steps"`
	EstimatedFiles int             `json:"estimated_files"`
	Rationale      string          `json:"rationale"`
}

// ComplexityLevel identifies the complexity level.
type ComplexityLevel string

// Complexity level constants.
const (
	ComplexityTrivial     ComplexityLevel = "trivial"
	ComplexitySimple      ComplexityLevel = "simple"
	ComplexityModerate    ComplexityLevel = "moderate"
	ComplexityComplex     ComplexityLevel = "complex"
	ComplexityVeryComplex ComplexityLevel = "very_complex"
)

// ImpactAnalysisResult contains impact analysis results.
type ImpactAnalysisResult struct {
	DirectImpacts   []FileImpact         `json:"direct_impacts"`
	IndirectImpacts []FileImpact         `json:"indirect_impacts"`
	DependencyGraph DependencyGraph      `json:"dependency_graph"`
	TestCoverage    TestCoverageAnalysis `json:"test_coverage"`
}

// FileImpact describes the impact on a file.
type FileImpact struct {
	FilePath        string     `json:"file_path"`
	ImpactType      ImpactType `json:"impact_type"`
	Confidence      float64    `json:"confidence"`
	Rationale       string     `json:"rationale"`
	AffectedSymbols []string   `json:"affected_symbols,omitempty"`
}

// ImpactType identifies the type of impact.
type ImpactType string

// Impact type constants.
const (
	ImpactTypeCreate     ImpactType = "create"
	ImpactTypeModify     ImpactType = "modify"
	ImpactTypeDelete     ImpactType = "delete"
	ImpactTypeRename     ImpactType = "rename"
	ImpactTypeDependency ImpactType = "dependency"
)

// DependencyGraph represents code dependencies.
type DependencyGraph struct {
	Nodes []DependencyNode `json:"nodes"`
	Edges []DependencyEdge `json:"edges"`
}

// DependencyNode is a node in the dependency graph.
type DependencyNode struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Symbol   string   `json:"symbol,omitempty"`
	NodeType NodeType `json:"node_type"`
}

// NodeType identifies the type of dependency node.
type NodeType string

// Node type constants.
const (
	NodeTypeFile     NodeType = "file"
	NodeTypeFunction NodeType = "function"
	NodeTypeClass    NodeType = "class"
	NodeTypeVariable NodeType = "variable"
	NodeTypeType     NodeType = "type"
)

// DependencyEdge is an edge in the dependency graph.
type DependencyEdge struct {
	FromID   string   `json:"from_id"`
	ToID     string   `json:"to_id"`
	EdgeType EdgeType `json:"edge_type"`
}

// EdgeType identifies the type of dependency edge.
type EdgeType string

// Edge type constants.
const (
	EdgeTypeImports    EdgeType = "imports"
	EdgeTypeCalls      EdgeType = "calls"
	EdgeTypeExtends    EdgeType = "extends"
	EdgeTypeImplements EdgeType = "implements"
	EdgeTypeUses       EdgeType = "uses"
)

// TestCoverageAnalysis contains test coverage information.
type TestCoverageAnalysis struct {
	CoveringTests   []TestReference      `json:"covering_tests"`
	CoverageGaps    []CoverageGap        `json:"coverage_gaps"`
	Recommendations []TestRecommendation `json:"recommendations"`
}

// TestReference references an existing test.
type TestReference struct {
	FilePath    string   `json:"file_path"`
	TestName    string   `json:"test_name"`
	CoversFiles []string `json:"covers_files"`
}

// CoverageGap identifies missing test coverage.
type CoverageGap struct {
	FilePath string `json:"file_path"`
	Symbol   string `json:"symbol"`
	Reason   string `json:"reason"`
}

// TestRecommendation suggests a new test.
type TestRecommendation struct {
	TargetFile   string `json:"target_file"`
	TargetSymbol string `json:"target_symbol"`
	TestType     string `json:"test_type"`
	Description  string `json:"description"`
}

// ImplementationPlan is the generated implementation plan.
type ImplementationPlan struct {
	PlanID                 string           `json:"plan_id"`
	Title                  string           `json:"title"`
	Summary                string           `json:"summary"`
	Steps                  []PlanStep       `json:"steps"`
	Dependencies           []StepDependency `json:"dependencies"`
	EstimatedMetrics       PlanMetrics      `json:"estimated_metrics"`
	Assumptions            []string         `json:"assumptions,omitempty"`
	AlternativesConsidered []Alternative    `json:"alternatives_considered,omitempty"`
}

// PlanStep is a step in the implementation plan.
type PlanStep struct {
	StepNumber      int              `json:"step_number"`
	Action          string           `json:"action"`
	Rationale       string           `json:"rationale"`
	TargetFiles     []TargetFile     `json:"target_files"`
	Prerequisites   []int            `json:"prerequisites,omitempty"`
	ExpectedOutcome string           `json:"expected_outcome"`
	Verification    StepVerification `json:"verification"`
	EstimatedTokens int              `json:"estimated_tokens"`
}

// TargetFile identifies a file to modify.
type TargetFile struct {
	Path       string         `json:"path"`
	Action     FileAction     `json:"action"`
	ChangeType FileChangeType `json:"change_type"`
	Reason     string         `json:"reason"`
}

// FileAction identifies the action to take on a file.
type FileAction string

// File action constants.
const (
	FileActionCreate FileAction = "create"
	FileActionModify FileAction = "modify"
	FileActionDelete FileAction = "delete"
	FileActionRename FileAction = "rename"
)

// FileChangeType identifies the type of file change.
type FileChangeType string

// File change type constants.
const (
	FileChangeTypeImplementation FileChangeType = "implementation"
	FileChangeTypeTest           FileChangeType = "test"
	FileChangeTypeConfig         FileChangeType = "config"
	FileChangeTypeDocumentation  FileChangeType = "documentation"
	FileChangeTypeDependency     FileChangeType = "dependency"
)

// StepVerification defines how to verify a step.
type StepVerification struct {
	Method      VerificationMethod `json:"method"`
	Command     string             `json:"command,omitempty"`
	Expected    string             `json:"expected,omitempty"`
	Description string             `json:"description"`
}

// VerificationMethod identifies how to verify a step.
type VerificationMethod string

// Verification method constants.
const (
	VerificationSyntaxCheck VerificationMethod = "syntax_check"
	VerificationBuild       VerificationMethod = "build"
	VerificationTest        VerificationMethod = "test"
	VerificationLint        VerificationMethod = "lint"
	VerificationManual      VerificationMethod = "manual"
)

// StepDependency defines a dependency between steps.
type StepDependency struct {
	FromStep       int            `json:"from_step"`
	ToStep         int            `json:"to_step"`
	DependencyType DependencyKind `json:"dependency_type"`
	Reason         string         `json:"reason"`
}

// DependencyKind identifies the type of step dependency.
type DependencyKind string

// Dependency kind constants.
const (
	DependencyStrict DependencyKind = "strict"
	DependencySoft   DependencyKind = "soft"
	DependencyData   DependencyKind = "data"
)

// PlanMetrics contains estimated metrics for a plan.
type PlanMetrics struct {
	TotalSteps          int `json:"total_steps"`
	EstimatedTokens     int `json:"estimated_tokens"`
	EstimatedFilesTotal int `json:"estimated_files_total"`
	EstimatedNewFiles   int `json:"estimated_new_files"`
	EstimatedModified   int `json:"estimated_modified"`
	EstimatedDeleted    int `json:"estimated_deleted"`
}

// Alternative describes an alternative approach.
type Alternative struct {
	Approach    string `json:"approach"`
	Pros        string `json:"pros"`
	Cons        string `json:"cons"`
	WhyRejected string `json:"why_rejected"`
}

// CodeGenerationResult contains generated code changes.
type CodeGenerationResult struct {
	FileChanges   []FileChange `json:"file_changes"`
	CommitMessage string       `json:"commit_message"`
	Notes         string       `json:"notes,omitempty"`
}

// FileChange describes a change to a file.
type FileChange struct {
	FilePath     string     `json:"file_path"`
	Action       FileAction `json:"action"`
	PreviousPath string     `json:"previous_path,omitempty"`
	Content      string     `json:"content,omitempty"`
	Patch        string     `json:"patch,omitempty"`
	Description  string     `json:"description"`
}

// Usage contains token usage statistics.
type Usage struct {
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int     `json:"cache_write_tokens,omitempty"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
}

// InvocationResult is the result of an LLM invocation.
type InvocationResult struct {
	Provider    Provider  `json:"provider"`
	Model       Model     `json:"model"`
	Function    Function  `json:"function"`
	Usage       Usage     `json:"usage"`
	LatencyMS   int64     `json:"latency_ms"`
	StopReason  string    `json:"stop_reason"`
	RequestID   string    `json:"request_id,omitempty"`
	CacheHit    bool      `json:"cache_hit"`
	CompletedAt time.Time `json:"completed_at"`
}

// Default configuration constants.
const (
	defaultTimeoutSeconds  = 120
	defaultMaxRetries      = 3
	defaultMaxOutputTokens = 16384
)

// ClientConfig contains LLM client configuration.
type ClientConfig struct {
	// Provider settings
	AnthropicAPIKey string
	OpenAIAPIKey    string
	GoogleAPIKey    string

	// Defaults
	DefaultProvider Provider
	DefaultModel    Model

	// Timeouts and retries
	TimeoutSeconds int
	MaxRetries     int

	// Token limits
	MaxOutputTokens int
	Temperature     float64
}

// DefaultClientConfig returns default client configuration.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		DefaultProvider: ProviderAnthropic,
		DefaultModel:    ModelClaudeSonnet,
		TimeoutSeconds:  defaultTimeoutSeconds,
		MaxRetries:      defaultMaxRetries,
		MaxOutputTokens: defaultMaxOutputTokens,
		Temperature:     0.0,
	}
}
