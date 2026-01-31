package events

import "time"

// ===== SPECIFICATION NORMALIZATION =====

// SpecificationNormalizationStartedPayload is the payload for SpecificationNormalizationStarted.
type SpecificationNormalizationStartedPayload struct {
	OriginalSpec FeatureSpecification `json:"original_spec"`
	StartedAt    time.Time            `json:"started_at"`
}

// SpecificationNormalizedPayload is the payload for SpecificationNormalized.
type SpecificationNormalizedPayload struct {
	OriginalDescription string                `json:"original_description"`
	Normalized          NormalizedSpecification `json:"normalized"`
	LLMInfo             LLMProcessingInfo     `json:"llm_info"`
	CompletedAt         time.Time             `json:"completed_at"`
}

// NormalizedSpecification is a structured, clarified specification.
type NormalizedSpecification struct {
	// ProblemStatement is a clear, concise problem statement.
	ProblemStatement string `json:"problem_statement"`

	// BehaviorChanges are expected behavior changes.
	BehaviorChanges []BehaviorChange `json:"behavior_changes"`

	// Components are identified components to modify.
	Components []ComponentReference `json:"components"`

	// AcceptanceCriteria are explicit acceptance criteria.
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria"`

	// Risks are potential risks identified.
	Risks []Risk `json:"risks"`

	// Complexity is the estimated complexity.
	Complexity ComplexityAssessment `json:"complexity"`
}

// BehaviorChange describes an expected behavior change.
type BehaviorChange struct {
	Component       string     `json:"component"`
	CurrentBehavior string     `json:"current_behavior"`
	DesiredBehavior string     `json:"desired_behavior"`
	ChangeType      ChangeType `json:"change_type"`
}

// ChangeType categorizes changes.
type ChangeType string

const (
	ChangeTypeAdd      ChangeType = "add"
	ChangeTypeModify   ChangeType = "modify"
	ChangeTypeRemove   ChangeType = "remove"
	ChangeTypeRefactor ChangeType = "refactor"
)

// ComponentReference references a component.
type ComponentReference struct {
	Name    string        `json:"name"`
	Path    string        `json:"path"`
	Type    ComponentType `json:"type"`
	Purpose string        `json:"purpose"`
}

// ComponentType categorizes components.
type ComponentType string

const (
	ComponentTypeFile     ComponentType = "file"
	ComponentTypeFunction ComponentType = "function"
	ComponentTypeClass    ComponentType = "class"
	ComponentTypeModule   ComponentType = "module"
	ComponentTypePackage  ComponentType = "package"
	ComponentTypeConfig   ComponentType = "config"
	ComponentTypeTest     ComponentType = "test"
)

// AcceptanceCriterion is a testable acceptance criterion.
type AcceptanceCriterion struct {
	Criterion          string `json:"criterion"`
	Testable           bool   `json:"testable"`
	VerificationMethod string `json:"verification_method"`
}

// Risk is an identified risk.
type Risk struct {
	Description string    `json:"description"`
	Level       RiskLevel `json:"level"`
	Mitigation  string    `json:"mitigation"`
}

// RiskLevel indicates risk severity.
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// ComplexityAssessment estimates implementation complexity.
type ComplexityAssessment struct {
	Level                   ComplexityLevel `json:"level"`
	EstimatedSteps          int             `json:"estimated_steps"`
	EstimatedFiles          int             `json:"estimated_files"`
	EstimatedDurationMinutes int             `json:"estimated_duration_minutes"`
	Rationale               string          `json:"rationale"`
}

// ComplexityLevel indicates complexity.
type ComplexityLevel string

const (
	ComplexityLevelTrivial     ComplexityLevel = "trivial"
	ComplexityLevelSimple      ComplexityLevel = "simple"
	ComplexityLevelModerate    ComplexityLevel = "moderate"
	ComplexityLevelComplex     ComplexityLevel = "complex"
	ComplexityLevelVeryComplex ComplexityLevel = "very_complex"
)

// LLMProcessingInfo contains LLM invocation details.
type LLMProcessingInfo struct {
	Model        string `json:"model"`
	Function     string `json:"function"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	LatencyMS    int64  `json:"latency_ms"`
	Provider     string `json:"provider,omitempty"`
}

// SpecificationNormalizationFailedPayload is the payload for SpecificationNormalizationFailed.
type SpecificationNormalizationFailedPayload struct {
	ErrorCode    string            `json:"error_code"`
	ErrorMessage string            `json:"error_message"`
	Retryable    bool              `json:"retryable"`
	ErrorContext map[string]string `json:"error_context,omitempty"`
	FailedAt     time.Time         `json:"failed_at"`
}

// ===== IMPACT ANALYSIS =====

// ImpactAnalysisStartedPayload is the payload for ImpactAnalysisStarted.
type ImpactAnalysisStartedPayload struct {
	NormalizedSpecEventID string    `json:"normalized_spec_event_id"`
	FilesToAnalyze        []string  `json:"files_to_analyze"`
	StartedAt             time.Time `json:"started_at"`
}

// ImpactAnalysisCompletedPayload is the payload for ImpactAnalysisCompleted.
type ImpactAnalysisCompletedPayload struct {
	DirectImpacts   []FileImpact         `json:"direct_impacts"`
	IndirectImpacts []FileImpact         `json:"indirect_impacts"`
	DependencyGraph DependencyGraph      `json:"dependency_graph"`
	TestCoverage    TestCoverageAnalysis `json:"test_coverage"`
	Metrics         ImpactMetrics        `json:"metrics"`
	CompletedAt     time.Time            `json:"completed_at"`
}

// FileImpact describes impact on a file.
type FileImpact struct {
	FilePath        string     `json:"file_path"`
	ImpactType      ImpactType `json:"impact_type"`
	Confidence      float64    `json:"confidence"` // 0.0 to 1.0
	Rationale       string     `json:"rationale"`
	AffectedSymbols []string   `json:"affected_symbols,omitempty"`
}

// ImpactType categorizes file impacts.
type ImpactType string

const (
	ImpactTypeCreate     ImpactType = "create"
	ImpactTypeModify     ImpactType = "modify"
	ImpactTypeDelete     ImpactType = "delete"
	ImpactTypeRename     ImpactType = "rename"
	ImpactTypeDependency ImpactType = "dependency" // May need changes due to dependencies
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

// NodeType categorizes dependency nodes.
type NodeType string

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

// EdgeType categorizes dependency edges.
type EdgeType string

const (
	EdgeTypeImports    EdgeType = "imports"
	EdgeTypeCalls      EdgeType = "calls"
	EdgeTypeExtends    EdgeType = "extends"
	EdgeTypeImplements EdgeType = "implements"
	EdgeTypeUses       EdgeType = "uses"
)

// TestCoverageAnalysis analyzes test coverage for affected code.
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

// CoverageGap identifies a gap in test coverage.
type CoverageGap struct {
	FilePath string `json:"file_path"`
	Symbol   string `json:"symbol"`
	Reason   string `json:"reason"`
}

// TestRecommendation recommends a test to add.
type TestRecommendation struct {
	TargetFile   string `json:"target_file"`
	TargetSymbol string `json:"target_symbol"`
	TestType     string `json:"test_type"` // unit, integration, e2e
	Description  string `json:"description"`
}

// ImpactMetrics contains impact analysis statistics.
type ImpactMetrics struct {
	FilesAnalyzed       int   `json:"files_analyzed"`
	DirectImpactCount   int   `json:"direct_impact_count"`
	IndirectImpactCount int   `json:"indirect_impact_count"`
	TestsAffected       int   `json:"tests_affected"`
	AnalysisDurationMS  int64 `json:"analysis_duration_ms"`
}

// ImpactAnalysisFailedPayload is the payload for ImpactAnalysisFailed.
type ImpactAnalysisFailedPayload struct {
	ErrorCode    string            `json:"error_code"`
	ErrorMessage string            `json:"error_message"`
	Retryable    bool              `json:"retryable"`
	ErrorContext map[string]string `json:"error_context,omitempty"`
	FailedAt     time.Time         `json:"failed_at"`
}
