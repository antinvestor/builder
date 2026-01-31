package events

// EventType identifies the type of event.
// Format: {domain}.{aggregate}.{action}[.{qualifier}]
type EventType string

// Event type constants organized by category.
const (
	// === LIFECYCLE EVENTS ===

	// FeatureExecutionInitialized marks the start of feature execution.
	FeatureExecutionInitialized EventType = "feature.execution.initialized"

	// FeatureExecutionCompleted marks successful completion of all execution steps.
	FeatureExecutionCompleted EventType = "feature.execution.completed"

	// FeatureDelivered marks the feature branch pushed and ready.
	FeatureDelivered EventType = "feature.execution.delivered"

	// FeatureExecutionFailed marks terminal failure.
	FeatureExecutionFailed EventType = "feature.execution.failed"

	// FeatureExecutionAborted marks user-initiated cancellation.
	FeatureExecutionAborted EventType = "feature.execution.aborted"

	// === REPOSITORY EVENTS ===

	// RepositoryCheckoutStarted indicates clone/fetch beginning.
	RepositoryCheckoutStarted EventType = "repository.checkout.started"

	// RepositoryCheckoutCompleted indicates successful checkout.
	RepositoryCheckoutCompleted EventType = "repository.checkout.completed"

	// RepositoryCheckoutFailed indicates checkout failure.
	RepositoryCheckoutFailed EventType = "repository.checkout.failed"

	// RepositoryIndexingStarted indicates codebase analysis beginning.
	RepositoryIndexingStarted EventType = "repository.indexing.started"

	// RepositoryIndexingCompleted indicates codebase analysis done.
	RepositoryIndexingCompleted EventType = "repository.indexing.completed"

	// RepositoryIndexingFailed indicates indexing failure.
	RepositoryIndexingFailed EventType = "repository.indexing.failed"

	// === SPECIFICATION EVENTS ===

	// SpecificationNormalizationStarted indicates spec processing beginning.
	SpecificationNormalizationStarted EventType = "specification.normalization.started"

	// SpecificationNormalized indicates LLM has processed the spec.
	SpecificationNormalized EventType = "specification.normalization.completed"

	// SpecificationNormalizationFailed indicates spec processing failed.
	SpecificationNormalizationFailed EventType = "specification.normalization.failed"

	// === ANALYSIS EVENTS ===

	// ImpactAnalysisStarted indicates impact analysis beginning.
	ImpactAnalysisStarted EventType = "analysis.impact.started"

	// ImpactAnalysisCompleted indicates impact analysis done.
	ImpactAnalysisCompleted EventType = "analysis.impact.completed"

	// ImpactAnalysisFailed indicates impact analysis failed.
	ImpactAnalysisFailed EventType = "analysis.impact.failed"

	// === PLANNING EVENTS ===

	// PlanGenerationStarted indicates plan generation beginning.
	PlanGenerationStarted EventType = "planning.generation.started"

	// PlanGenerated indicates implementation plan is ready.
	PlanGenerated EventType = "planning.generation.completed"

	// PlanGenerationFailed indicates planning failed.
	PlanGenerationFailed EventType = "planning.generation.failed"

	// PlanValidated indicates plan validation passed.
	PlanValidated EventType = "planning.validation.completed"

	// === PATCH GENERATION EVENTS ===

	// PatchGenerationStarted indicates patch generation phase beginning.
	PatchGenerationStarted EventType = "patch.generation.started"

	// PatchGenerationStepStarted indicates a step is beginning.
	PatchGenerationStepStarted EventType = "patch.generation.step.started"

	// PatchGenerationStepCompleted indicates step generated code.
	PatchGenerationStepCompleted EventType = "patch.generation.step.completed"

	// PatchGenerationStepFailed indicates step failure.
	PatchGenerationStepFailed EventType = "patch.generation.step.failed"

	// PatchGenerationCompleted indicates all patches generated.
	PatchGenerationCompleted EventType = "patch.generation.completed"

	// === TEST EVENTS ===

	// TestGenerationStarted indicates test generation beginning.
	TestGenerationStarted EventType = "test.generation.started"

	// TestGenerationCompleted indicates tests generated.
	TestGenerationCompleted EventType = "test.generation.completed"

	// TestGenerationFailed indicates test generation failed.
	TestGenerationFailed EventType = "test.generation.failed"

	// TestExecutionStarted indicates test run beginning.
	TestExecutionStarted EventType = "test.execution.started"

	// TestExecutionCompleted indicates test run finished.
	TestExecutionCompleted EventType = "test.execution.completed"

	// TestExecutionFailed indicates test execution infrastructure failure.
	TestExecutionFailed EventType = "test.execution.failed"

	// === BUILD EVENTS ===

	// BuildStarted indicates build execution beginning.
	BuildStarted EventType = "build.execution.started"

	// BuildCompleted indicates build finished.
	BuildCompleted EventType = "build.execution.completed"

	// BuildFailed indicates build failure.
	BuildFailed EventType = "build.execution.failed"

	// === REVIEW EVENTS ===

	// ReviewStarted indicates automated review beginning.
	ReviewStarted EventType = "review.analysis.started"

	// ReviewCompleted indicates automated review done.
	ReviewCompleted EventType = "review.analysis.completed"

	// ReviewFailed indicates review analysis failed.
	ReviewFailed EventType = "review.analysis.failed"

	// SecurityScanStarted indicates security scan beginning.
	SecurityScanStarted EventType = "review.security.started"

	// SecurityScanCompleted indicates security scan done.
	SecurityScanCompleted EventType = "review.security.completed"

	// === ITERATION EVENTS ===

	// IterationRequired indicates changes needed before completion.
	IterationRequired EventType = "iteration.required"

	// IterationStarted indicates iteration beginning.
	IterationStarted EventType = "iteration.started"

	// IterationCompleted indicates iteration finished.
	IterationCompleted EventType = "iteration.completed"

	// === ROLLBACK EVENTS ===

	// RollbackInitiated indicates rollback starting.
	RollbackInitiated EventType = "rollback.initiated"

	// RollbackCompleted indicates rollback finished.
	RollbackCompleted EventType = "rollback.completed"

	// RollbackFailed indicates rollback failed.
	RollbackFailed EventType = "rollback.failed"

	// === GIT EVENTS ===

	// GitBranchCreated indicates feature branch created.
	GitBranchCreated EventType = "git.branch.created"

	// GitCommitCreated indicates commit created.
	GitCommitCreated EventType = "git.commit.created"

	// GitPushStarted indicates push to remote beginning.
	GitPushStarted EventType = "git.push.started"

	// GitPushCompleted indicates push succeeded.
	GitPushCompleted EventType = "git.push.completed"

	// GitPushFailed indicates push failed.
	GitPushFailed EventType = "git.push.failed"

	// === RESOURCE EVENTS ===

	// ResourcesAcquired indicates locks/credentials obtained.
	ResourcesAcquired EventType = "resources.acquired"

	// ResourcesReleased indicates cleanup completed.
	ResourcesReleased EventType = "resources.released"

	// SandboxCreated indicates sandbox provisioned.
	SandboxCreated EventType = "sandbox.created"

	// SandboxDestroyed indicates sandbox removed.
	SandboxDestroyed EventType = "sandbox.destroyed"

	// === LLM EVENTS ===

	// LLMInvocationStarted indicates LLM call beginning.
	LLMInvocationStarted EventType = "llm.invocation.started"

	// LLMInvocationCompleted indicates LLM response received.
	LLMInvocationCompleted EventType = "llm.invocation.completed"

	// LLMInvocationFailed indicates LLM call failed.
	LLMInvocationFailed EventType = "llm.invocation.failed"
)

// String returns the string representation.
func (t EventType) String() string {
	return string(t)
}

// Domain returns the domain component of the event type.
func (t EventType) Domain() string {
	s := string(t)
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return s[:i]
		}
	}
	return s
}

// IsLifecycleEvent returns true if this is a lifecycle event.
func (t EventType) IsLifecycleEvent() bool {
	return t.Domain() == "feature"
}

// IsFailureEvent returns true if this event type indicates a failure.
func (t EventType) IsFailureEvent() bool {
	switch t {
	case FeatureExecutionFailed,
		RepositoryCheckoutFailed,
		RepositoryIndexingFailed,
		SpecificationNormalizationFailed,
		ImpactAnalysisFailed,
		PlanGenerationFailed,
		PatchGenerationStepFailed,
		TestGenerationFailed,
		TestExecutionFailed,
		BuildFailed,
		ReviewFailed,
		RollbackFailed,
		GitPushFailed,
		LLMInvocationFailed:
		return true
	default:
		return false
	}
}

// IsTerminalEvent returns true if this event type ends execution.
func (t EventType) IsTerminalEvent() bool {
	switch t {
	case FeatureDelivered, FeatureExecutionFailed, FeatureExecutionAborted:
		return true
	default:
		return false
	}
}

// EventCategory represents a category of events.
type EventCategory string

const (
	CategoryLifecycle     EventCategory = "lifecycle"
	CategoryRepository    EventCategory = "repository"
	CategorySpecification EventCategory = "specification"
	CategoryAnalysis      EventCategory = "analysis"
	CategoryPlanning      EventCategory = "planning"
	CategoryPatch         EventCategory = "patch"
	CategoryTest          EventCategory = "test"
	CategoryBuild         EventCategory = "build"
	CategoryReview        EventCategory = "review"
	CategoryIteration     EventCategory = "iteration"
	CategoryRollback      EventCategory = "rollback"
	CategoryGit           EventCategory = "git"
	CategoryResource      EventCategory = "resource"
	CategoryLLM           EventCategory = "llm"
)

// Category returns the category this event type belongs to.
func (t EventType) Category() EventCategory {
	domain := t.Domain()
	switch domain {
	case "feature":
		return CategoryLifecycle
	case "repository":
		return CategoryRepository
	case "specification":
		return CategorySpecification
	case "analysis":
		return CategoryAnalysis
	case "planning":
		return CategoryPlanning
	case "patch":
		return CategoryPatch
	case "test":
		return CategoryTest
	case "build":
		return CategoryBuild
	case "review":
		return CategoryReview
	case "iteration":
		return CategoryIteration
	case "rollback":
		return CategoryRollback
	case "git":
		return CategoryGit
	case "resources", "sandbox":
		return CategoryResource
	case "llm":
		return CategoryLLM
	default:
		return ""
	}
}

// AllEventTypes returns all defined event types.
func AllEventTypes() []EventType {
	return []EventType{
		// Lifecycle
		FeatureExecutionInitialized,
		FeatureExecutionCompleted,
		FeatureDelivered,
		FeatureExecutionFailed,
		FeatureExecutionAborted,
		// Repository
		RepositoryCheckoutStarted,
		RepositoryCheckoutCompleted,
		RepositoryCheckoutFailed,
		RepositoryIndexingStarted,
		RepositoryIndexingCompleted,
		RepositoryIndexingFailed,
		// Specification
		SpecificationNormalizationStarted,
		SpecificationNormalized,
		SpecificationNormalizationFailed,
		// Analysis
		ImpactAnalysisStarted,
		ImpactAnalysisCompleted,
		ImpactAnalysisFailed,
		// Planning
		PlanGenerationStarted,
		PlanGenerated,
		PlanGenerationFailed,
		PlanValidated,
		// Patch
		PatchGenerationStarted,
		PatchGenerationStepStarted,
		PatchGenerationStepCompleted,
		PatchGenerationStepFailed,
		PatchGenerationCompleted,
		// Test
		TestGenerationStarted,
		TestGenerationCompleted,
		TestGenerationFailed,
		TestExecutionStarted,
		TestExecutionCompleted,
		TestExecutionFailed,
		// Build
		BuildStarted,
		BuildCompleted,
		BuildFailed,
		// Review
		ReviewStarted,
		ReviewCompleted,
		ReviewFailed,
		SecurityScanStarted,
		SecurityScanCompleted,
		// Iteration
		IterationRequired,
		IterationStarted,
		IterationCompleted,
		// Rollback
		RollbackInitiated,
		RollbackCompleted,
		RollbackFailed,
		// Git
		GitBranchCreated,
		GitCommitCreated,
		GitPushStarted,
		GitPushCompleted,
		GitPushFailed,
		// Resources
		ResourcesAcquired,
		ResourcesReleased,
		SandboxCreated,
		SandboxDestroyed,
		// LLM
		LLMInvocationStarted,
		LLMInvocationCompleted,
		LLMInvocationFailed,
	}
}
