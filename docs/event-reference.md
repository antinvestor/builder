# builder Event Reference

## Overview

builder uses event sourcing as its primary state management pattern. All state changes are captured as immutable events in a partitioned event log. This document catalogs all event types, their payloads, and causality relationships.

---

## Event Schema

### Base Event Structure

```protobuf
message Event {
    // Identity
    string event_id = 1;              // UUID v7 (time-ordered)
    string feature_execution_id = 2;  // Partition key
    string event_type = 3;            // Event type identifier

    // Ordering
    uint64 sequence_number = 4;       // Per-feature monotonic counter
    google.protobuf.Timestamp timestamp = 5;

    // Causality
    string causation_id = 6;          // Event that directly caused this event
    string correlation_id = 7;        // Root request ID (spans entire feature)

    // Payload
    bytes payload = 8;                // Protobuf-encoded payload

    // Metadata
    EventMetadata metadata = 9;
}

message EventMetadata {
    string producer_id = 1;           // Service/worker that emitted
    string schema_version = 2;        // Payload schema version (semver)
    map<string, string> tags = 3;     // Additional context tags
}
```

### Go Type Definition

```go
type Event struct {
    EventID            string            `json:"event_id"`
    FeatureExecutionID string            `json:"feature_execution_id"`
    EventType          string            `json:"event_type"`
    SequenceNumber     uint64            `json:"sequence_number"`
    Timestamp          time.Time         `json:"timestamp"`
    CausationID        string            `json:"causation_id"`
    CorrelationID      string            `json:"correlation_id"`
    Payload            json.RawMessage   `json:"payload"`
    Metadata           EventMetadata     `json:"metadata"`
}

type EventMetadata struct {
    ProducerID    string            `json:"producer_id"`
    SchemaVersion string            `json:"schema_version"`
    Tags          map[string]string `json:"tags"`
}
```

---

## Event Catalog

### Event Type Hierarchy

```
feature.events
├── lifecycle
│   ├── feature.requested
│   ├── feature.delivered
│   ├── feature.failed
│   └── feature.cancelled
├── analysis
│   ├── analysis.started
│   ├── repository.clone.started
│   ├── repository.clone.completed
│   ├── codebase.analyzed
│   ├── dependency.graph.built
│   ├── scope.identified
│   ├── analysis.completed
│   └── analysis.failed
├── planning
│   ├── planning.started
│   ├── context.prepared
│   ├── llm.invocation.started
│   ├── llm.response.received
│   ├── plan.parsed
│   ├── plan.validated
│   ├── plan.step.defined
│   ├── plan.generated
│   └── planning.failed
├── execution
│   ├── execution.started
│   ├── step.started
│   ├── code.change.generated
│   ├── file.modified
│   ├── syntax.validated
│   ├── local.commit.created
│   ├── step.completed
│   ├── step.failed
│   ├── execution.completed
│   └── execution.failed
├── verification
│   ├── verification.started
│   ├── sandbox.created
│   ├── build.started
│   ├── build.completed
│   ├── tests.started
│   ├── tests.completed
│   ├── static.analysis.started
│   ├── static.analysis.completed
│   ├── sandbox.destroyed
│   ├── verification.passed
│   └── verification.failed
└── git
    ├── branch.created
    ├── commit.created
    ├── push.started
    ├── push.completed
    └── push.failed
```

---

## Lifecycle Events

### feature.requested

Emitted when a new feature execution is submitted.

**Producer:** Feature Service (API)

**Payload:**
```go
type FeatureRequestedPayload struct {
    Spec            FeatureSpec       `json:"spec"`
    CorrelationID   string            `json:"correlation_id"`
    IdempotencyKey  string            `json:"idempotency_key,omitempty"`
    TenantID        string            `json:"tenant_id"`
    RequestedBy     string            `json:"requested_by"`
    RequestedAt     time.Time         `json:"requested_at"`
}

type FeatureSpec struct {
    Title           string            `json:"title"`
    Description     string            `json:"description"`
    RepositoryID    string            `json:"repository_id"`
    TargetBranch    string            `json:"target_branch"`
    FeatureBranch   string            `json:"feature_branch,omitempty"`
    Constraints     FeatureConstraints `json:"constraints"`
    Properties      map[string]string `json:"properties,omitempty"`
}
```

**Transitions:** `PENDING` → `ANALYZING`

**Next Events:** `analysis.started`

---

### feature.delivered

Emitted when a feature is successfully delivered.

**Producer:** Feature Worker

**Payload:**
```go
type FeatureDeliveredPayload struct {
    BranchName      string    `json:"branch_name"`
    CommitSHA       string    `json:"commit_sha"`
    CommitCount     int       `json:"commit_count"`
    FilesChanged    int       `json:"files_changed"`
    LinesAdded      int       `json:"lines_added"`
    LinesRemoved    int       `json:"lines_removed"`
    PatchArtifactID string    `json:"patch_artifact_id"`
    Summary         string    `json:"summary"`
    Duration        Duration  `json:"duration"`
    DeliveredAt     time.Time `json:"delivered_at"`
}
```

**Transitions:** `COMPLETED` (terminal)

**Causation:** `push.completed`

---

### feature.failed

Emitted when a feature execution fails terminally.

**Producer:** Feature Worker

**Payload:**
```go
type FeatureFailedPayload struct {
    ErrorCode       string            `json:"error_code"`
    ErrorMessage    string            `json:"error_message"`
    ErrorDetails    map[string]any    `json:"error_details,omitempty"`
    FailedPhase     string            `json:"failed_phase"`
    FailedStep      *int              `json:"failed_step,omitempty"`
    Retryable       bool              `json:"retryable"`
    RetryCount      int               `json:"retry_count"`
    FailedAt        time.Time         `json:"failed_at"`
}
```

**Transitions:** `FAILED` (terminal)

**Causation:** Any failure event (`analysis.failed`, `planning.failed`, etc.)

---

### feature.cancelled

Emitted when a feature execution is cancelled by user request.

**Producer:** Feature Service (API)

**Payload:**
```go
type FeatureCancelledPayload struct {
    Reason          string    `json:"reason"`
    CancelledBy     string    `json:"cancelled_by"`
    CancelledAt     time.Time `json:"cancelled_at"`
    WasPending      bool      `json:"was_pending"`
    InterruptedStep *int      `json:"interrupted_step,omitempty"`
}
```

**Transitions:** `CANCELLED` (terminal)

---

## Analysis Events

### analysis.started

Emitted when the analysis phase begins.

**Producer:** Feature Worker

**Payload:**
```go
type AnalysisStartedPayload struct {
    RepositoryID    string    `json:"repository_id"`
    RepositoryURL   string    `json:"repository_url"`
    TargetBranch    string    `json:"target_branch"`
    WorkspacePath   string    `json:"workspace_path"`
    StartedAt       time.Time `json:"started_at"`
}
```

**Transitions:** `PENDING` → `ANALYZING`

**Causation:** `feature.requested`

---

### repository.clone.started

Emitted when git clone operation begins.

**Producer:** Feature Worker

**Payload:**
```go
type RepositoryCloneStartedPayload struct {
    RepositoryURL   string    `json:"repository_url"`
    TargetRef       string    `json:"target_ref"`
    WorkspacePath   string    `json:"workspace_path"`
    CloneDepth      int       `json:"clone_depth"`
    StartedAt       time.Time `json:"started_at"`
}
```

---

### repository.clone.completed

Emitted when git clone completes successfully.

**Producer:** Feature Worker

**Payload:**
```go
type RepositoryCloneCompletedPayload struct {
    HeadCommit      string        `json:"head_commit"`
    BranchName      string        `json:"branch_name"`
    CloneSize       int64         `json:"clone_size_bytes"`
    FileCount       int           `json:"file_count"`
    Duration        time.Duration `json:"duration"`
    CompletedAt     time.Time     `json:"completed_at"`
}
```

**Causation:** `repository.clone.started`

---

### codebase.analyzed

Emitted when codebase structure analysis completes.

**Producer:** Code Analysis Service

**Payload:**
```go
type CodebaseAnalyzedPayload struct {
    Language        string            `json:"primary_language"`
    Languages       map[string]int    `json:"languages"`        // language -> file count
    BuildSystem     string            `json:"build_system"`     // npm, go, maven, etc.
    TestFramework   string            `json:"test_framework"`
    FileCount       int               `json:"file_count"`
    DirectoryCount  int               `json:"directory_count"`
    TotalLines      int               `json:"total_lines"`
    Structure       CodebaseStructure `json:"structure"`
    AnalyzedAt      time.Time         `json:"analyzed_at"`
}

type CodebaseStructure struct {
    SourceDirs      []string `json:"source_dirs"`
    TestDirs        []string `json:"test_dirs"`
    ConfigFiles     []string `json:"config_files"`
    EntryPoints     []string `json:"entry_points"`
}
```

---

### dependency.graph.built

Emitted when dependency analysis completes.

**Producer:** Code Analysis Service

**Payload:**
```go
type DependencyGraphBuiltPayload struct {
    NodeCount       int               `json:"node_count"`
    EdgeCount       int               `json:"edge_count"`
    ExternalDeps    int               `json:"external_dependencies"`
    InternalDeps    int               `json:"internal_dependencies"`
    CircularDeps    int               `json:"circular_dependencies"`
    GraphArtifactID string            `json:"graph_artifact_id"`
    BuiltAt         time.Time         `json:"built_at"`
}
```

---

### scope.identified

Emitted when the modification scope is determined.

**Producer:** Code Analysis Service

**Payload:**
```go
type ScopeIdentifiedPayload struct {
    PrimaryFiles    []string          `json:"primary_files"`
    RelatedFiles    []string          `json:"related_files"`
    TestFiles       []string          `json:"test_files"`
    ImpactedModules []string          `json:"impacted_modules"`
    Complexity      string            `json:"complexity"` // low, medium, high
    IdentifiedAt    time.Time         `json:"identified_at"`
}
```

---

### analysis.completed

Emitted when the analysis phase completes successfully.

**Producer:** Feature Worker

**Payload:**
```go
type AnalysisCompletedPayload struct {
    Summary         AnalysisSummary   `json:"summary"`
    Duration        time.Duration     `json:"duration"`
    ContextSize     int               `json:"context_size_tokens"`
    CompletedAt     time.Time         `json:"completed_at"`
}

type AnalysisSummary struct {
    Language        string   `json:"language"`
    BuildSystem     string   `json:"build_system"`
    ScopeSize       int      `json:"scope_file_count"`
    Complexity      string   `json:"complexity"`
    Recommendation  string   `json:"recommendation"`
}
```

**Transitions:** `ANALYZING` → `PLANNING`

---

### analysis.failed

Emitted when the analysis phase fails.

**Producer:** Feature Worker

**Payload:**
```go
type AnalysisFailedPayload struct {
    ErrorCode       string            `json:"error_code"`
    ErrorMessage    string            `json:"error_message"`
    Phase           string            `json:"failed_phase"` // clone, analyze, scope
    Retryable       bool              `json:"retryable"`
    FailedAt        time.Time         `json:"failed_at"`
}
```

**Transitions:** `ANALYZING` → `FAILED`

---

## Planning Events

### planning.started

Emitted when the planning phase begins.

**Producer:** Feature Worker

**Payload:**
```go
type PlanningStartedPayload struct {
    AnalysisSummary AnalysisSummary   `json:"analysis_summary"`
    ContextTokens   int               `json:"context_tokens"`
    Constraints     FeatureConstraints `json:"constraints"`
    StartedAt       time.Time         `json:"started_at"`
}
```

**Transitions:** `ANALYZING` → `PLANNING`

**Causation:** `analysis.completed`

---

### llm.invocation.started

Emitted when an LLM call begins.

**Producer:** LLM Orchestrator

**Payload:**
```go
type LLMInvocationStartedPayload struct {
    InvocationID    string            `json:"invocation_id"`
    Function        string            `json:"function"`     // GeneratePlan, GenerateCode, etc.
    Provider        string            `json:"provider"`     // anthropic, openai, etc.
    Model           string            `json:"model"`
    InputTokens     int               `json:"input_tokens"`
    MaxOutputTokens int               `json:"max_output_tokens"`
    Temperature     float64           `json:"temperature"`
    StartedAt       time.Time         `json:"started_at"`
}
```

---

### llm.response.received

Emitted when an LLM response is received.

**Producer:** LLM Orchestrator

**Payload:**
```go
type LLMResponseReceivedPayload struct {
    InvocationID    string            `json:"invocation_id"`
    OutputTokens    int               `json:"output_tokens"`
    TotalTokens     int               `json:"total_tokens"`
    FinishReason    string            `json:"finish_reason"` // stop, length, etc.
    Duration        time.Duration     `json:"duration"`
    ParseSuccess    bool              `json:"parse_success"`
    ReceivedAt      time.Time         `json:"received_at"`
}
```

**Causation:** `llm.invocation.started`

---

### plan.step.defined

Emitted for each step in the generated plan.

**Producer:** Feature Worker

**Payload:**
```go
type PlanStepDefinedPayload struct {
    StepIndex       int               `json:"step_index"`
    Description     string            `json:"description"`
    StepType        string            `json:"step_type"`
    TargetFiles     []string          `json:"target_files"`
    Dependencies    []int             `json:"dependencies"`
    EstimatedTokens int               `json:"estimated_tokens"`
    DefinedAt       time.Time         `json:"defined_at"`
}
```

---

### plan.generated

Emitted when the complete execution plan is ready.

**Producer:** Feature Worker

**Payload:**
```go
type PlanGeneratedPayload struct {
    TotalSteps      int               `json:"total_steps"`
    AffectedFiles   []string          `json:"affected_files"`
    EstimatedTime   time.Duration     `json:"estimated_duration"`
    Complexity      string            `json:"complexity"`
    Summary         string            `json:"summary"`
    PlanArtifactID  string            `json:"plan_artifact_id"`
    GeneratedAt     time.Time         `json:"generated_at"`
}
```

**Transitions:** `PLANNING` → `EXECUTING`

---

### planning.failed

Emitted when planning fails.

**Producer:** Feature Worker

**Payload:**
```go
type PlanningFailedPayload struct {
    ErrorCode       string            `json:"error_code"`
    ErrorMessage    string            `json:"error_message"`
    LLMFailure      bool              `json:"llm_failure"`
    ValidationError string            `json:"validation_error,omitempty"`
    Retryable       bool              `json:"retryable"`
    FailedAt        time.Time         `json:"failed_at"`
}
```

**Transitions:** `PLANNING` → `FAILED`

---

## Execution Events

### execution.started

Emitted when code execution begins.

**Producer:** Feature Worker

**Payload:**
```go
type ExecutionStartedPayload struct {
    TotalSteps      int               `json:"total_steps"`
    BranchName      string            `json:"branch_name"`
    BaseCommit      string            `json:"base_commit"`
    StartedAt       time.Time         `json:"started_at"`
}
```

**Transitions:** `PLANNING` → `EXECUTING`

**Causation:** `plan.generated`

---

### step.started

Emitted when a plan step begins.

**Producer:** Feature Worker

**Payload:**
```go
type StepStartedPayload struct {
    StepIndex       int               `json:"step_index"`
    Description     string            `json:"description"`
    StepType        string            `json:"step_type"`
    TargetFiles     []string          `json:"target_files"`
    AttemptNumber   int               `json:"attempt_number"`
    StartedAt       time.Time         `json:"started_at"`
}
```

---

### code.change.generated

Emitted when code is generated by LLM.

**Producer:** Feature Worker

**Payload:**
```go
type CodeChangeGeneratedPayload struct {
    StepIndex       int               `json:"step_index"`
    ChangeType      string            `json:"change_type"` // create, modify, delete
    FilePath        string            `json:"file_path"`
    LinesAdded      int               `json:"lines_added"`
    LinesRemoved    int               `json:"lines_removed"`
    GeneratedAt     time.Time         `json:"generated_at"`
}
```

**Causation:** `llm.response.received`

---

### file.modified

Emitted when a file is written to workspace.

**Producer:** Feature Worker

**Payload:**
```go
type FileModifiedPayload struct {
    FilePath        string            `json:"file_path"`
    ChangeType      string            `json:"change_type"` // created, modified, deleted
    PreviousHash    string            `json:"previous_hash,omitempty"`
    NewHash         string            `json:"new_hash"`
    LinesChanged    int               `json:"lines_changed"`
    ModifiedAt      time.Time         `json:"modified_at"`
}
```

---

### local.commit.created

Emitted when a local git commit is created.

**Producer:** Feature Worker

**Payload:**
```go
type LocalCommitCreatedPayload struct {
    StepIndex       int               `json:"step_index"`
    CommitSHA       string            `json:"commit_sha"`
    Message         string            `json:"message"`
    FilesChanged    int               `json:"files_changed"`
    Insertions      int               `json:"insertions"`
    Deletions       int               `json:"deletions"`
    CreatedAt       time.Time         `json:"created_at"`
}
```

---

### step.completed

Emitted when a plan step completes successfully.

**Producer:** Feature Worker

**Payload:**
```go
type StepCompletedPayload struct {
    StepIndex       int               `json:"step_index"`
    CommitSHA       string            `json:"commit_sha"`
    FilesModified   []string          `json:"files_modified"`
    LinesAdded      int               `json:"lines_added"`
    LinesRemoved    int               `json:"lines_removed"`
    Duration        time.Duration     `json:"duration"`
    CompletedAt     time.Time         `json:"completed_at"`
}
```

**Causation:** `step.started`

---

### step.failed

Emitted when a plan step fails.

**Producer:** Feature Worker

**Payload:**
```go
type StepFailedPayload struct {
    StepIndex       int               `json:"step_index"`
    ErrorCode       string            `json:"error_code"`
    ErrorMessage    string            `json:"error_message"`
    FailurePhase    string            `json:"failure_phase"` // generate, apply, validate
    Retryable       bool              `json:"retryable"`
    AttemptNumber   int               `json:"attempt_number"`
    FailedAt        time.Time         `json:"failed_at"`
}
```

**Causation:** `step.started`

---

### execution.completed

Emitted when all execution steps complete.

**Producer:** Feature Worker

**Payload:**
```go
type ExecutionCompletedPayload struct {
    StepsCompleted  int               `json:"steps_completed"`
    TotalCommits    int               `json:"total_commits"`
    FinalCommit     string            `json:"final_commit"`
    FilesChanged    int               `json:"files_changed"`
    LinesAdded      int               `json:"lines_added"`
    LinesRemoved    int               `json:"lines_removed"`
    Duration        time.Duration     `json:"duration"`
    CompletedAt     time.Time         `json:"completed_at"`
}
```

**Transitions:** `EXECUTING` → `VERIFYING`

---

### execution.failed

Emitted when execution fails after exhausting retries.

**Producer:** Feature Worker

**Payload:**
```go
type ExecutionFailedPayload struct {
    FailedStep      int               `json:"failed_step"`
    StepsCompleted  int               `json:"steps_completed"`
    ErrorCode       string            `json:"error_code"`
    ErrorMessage    string            `json:"error_message"`
    FailedAt        time.Time         `json:"failed_at"`
}
```

**Transitions:** `EXECUTING` → `FAILED`

---

## Verification Events

### verification.started

Emitted when verification phase begins.

**Producer:** Feature Worker

**Payload:**
```go
type VerificationStartedPayload struct {
    CommitToVerify  string            `json:"commit_to_verify"`
    RequiresBuild   bool              `json:"requires_build"`
    RequiresTests   bool              `json:"requires_tests"`
    StartedAt       time.Time         `json:"started_at"`
}
```

**Transitions:** `EXECUTING` → `VERIFYING`

**Causation:** `execution.completed`

---

### sandbox.created

Emitted when a sandbox is provisioned.

**Producer:** Sandbox Manager

**Payload:**
```go
type SandboxCreatedPayload struct {
    SandboxID       string            `json:"sandbox_id"`
    Image           string            `json:"image"`
    CPULimit        int               `json:"cpu_limit"`
    MemoryLimitMB   int               `json:"memory_limit_mb"`
    DiskLimitMB     int               `json:"disk_limit_mb"`
    TimeoutSeconds  int               `json:"timeout_seconds"`
    CreatedAt       time.Time         `json:"created_at"`
}
```

---

### build.started

Emitted when build execution begins.

**Producer:** Feature Worker

**Payload:**
```go
type BuildStartedPayload struct {
    SandboxID       string            `json:"sandbox_id"`
    BuildCommand    string            `json:"build_command"`
    WorkingDir      string            `json:"working_dir"`
    Environment     map[string]string `json:"environment"`
    StartedAt       time.Time         `json:"started_at"`
}
```

---

### build.completed

Emitted when build completes.

**Producer:** Feature Worker

**Payload:**
```go
type BuildCompletedPayload struct {
    Success         bool              `json:"success"`
    ExitCode        int               `json:"exit_code"`
    Duration        time.Duration     `json:"duration"`
    OutputLines     int               `json:"output_lines"`
    ErrorLines      int               `json:"error_lines"`
    LogArtifactID   string            `json:"log_artifact_id"`
    CompletedAt     time.Time         `json:"completed_at"`
}
```

**Causation:** `build.started`

---

### tests.started

Emitted when test execution begins.

**Producer:** Feature Worker

**Payload:**
```go
type TestsStartedPayload struct {
    SandboxID       string            `json:"sandbox_id"`
    TestCommand     string            `json:"test_command"`
    TestFramework   string            `json:"test_framework"`
    StartedAt       time.Time         `json:"started_at"`
}
```

---

### tests.completed

Emitted when tests complete.

**Producer:** Feature Worker

**Payload:**
```go
type TestsCompletedPayload struct {
    Success         bool              `json:"success"`
    TotalTests      int               `json:"total_tests"`
    Passed          int               `json:"passed"`
    Failed          int               `json:"failed"`
    Skipped         int               `json:"skipped"`
    Duration        time.Duration     `json:"duration"`
    Coverage        *float64          `json:"coverage_percent,omitempty"`
    ReportArtifactID string           `json:"report_artifact_id"`
    CompletedAt     time.Time         `json:"completed_at"`
}
```

**Causation:** `tests.started`

---

### verification.passed

Emitted when all verification checks pass.

**Producer:** Feature Worker

**Payload:**
```go
type VerificationPassedPayload struct {
    BuildPassed     bool              `json:"build_passed"`
    TestsPassed     bool              `json:"tests_passed"`
    AnalysisPassed  bool              `json:"analysis_passed"`
    TotalTests      int               `json:"total_tests"`
    TestsPassing    int               `json:"tests_passing"`
    Duration        time.Duration     `json:"duration"`
    PassedAt        time.Time         `json:"passed_at"`
}
```

**Transitions:** `VERIFYING` → `COMPLETED`

---

### verification.failed

Emitted when verification fails.

**Producer:** Feature Worker

**Payload:**
```go
type VerificationFailedPayload struct {
    FailedCheck     string            `json:"failed_check"` // build, tests, analysis
    ErrorMessage    string            `json:"error_message"`
    BuildResult     *BuildResult      `json:"build_result,omitempty"`
    TestResult      *TestResult       `json:"test_result,omitempty"`
    Retryable       bool              `json:"retryable"`
    FailedAt        time.Time         `json:"failed_at"`
}
```

**Transitions:** `VERIFYING` → `FAILED` (or retry to `EXECUTING`)

---

### sandbox.destroyed

Emitted when sandbox is cleaned up.

**Producer:** Sandbox Manager

**Payload:**
```go
type SandboxDestroyedPayload struct {
    SandboxID       string            `json:"sandbox_id"`
    Reason          string            `json:"reason"` // completed, failed, timeout
    TotalDuration   time.Duration     `json:"total_duration"`
    DestroyedAt     time.Time         `json:"destroyed_at"`
}
```

---

## Git Events

### branch.created

Emitted when a feature branch is created.

**Producer:** Feature Worker

**Payload:**
```go
type BranchCreatedPayload struct {
    BranchName      string            `json:"branch_name"`
    BaseBranch      string            `json:"base_branch"`
    BaseCommit      string            `json:"base_commit"`
    CreatedAt       time.Time         `json:"created_at"`
}
```

---

### commit.created

Emitted when a commit is created (local or pushed).

**Producer:** Feature Worker

**Payload:**
```go
type CommitCreatedPayload struct {
    CommitSHA       string            `json:"commit_sha"`
    ParentSHA       string            `json:"parent_sha"`
    Message         string            `json:"message"`
    Author          string            `json:"author"`
    FilesChanged    int               `json:"files_changed"`
    Insertions      int               `json:"insertions"`
    Deletions       int               `json:"deletions"`
    IsPushed        bool              `json:"is_pushed"`
    CreatedAt       time.Time         `json:"created_at"`
}
```

---

### push.started

Emitted when push to remote begins.

**Producer:** Feature Worker

**Payload:**
```go
type PushStartedPayload struct {
    BranchName      string            `json:"branch_name"`
    RemoteURL       string            `json:"remote_url"`
    CommitCount     int               `json:"commit_count"`
    HeadCommit      string            `json:"head_commit"`
    StartedAt       time.Time         `json:"started_at"`
}
```

---

### push.completed

Emitted when push succeeds.

**Producer:** Feature Worker

**Payload:**
```go
type PushCompletedPayload struct {
    BranchName      string            `json:"branch_name"`
    RemoteRef       string            `json:"remote_ref"`
    CommitsPushed   int               `json:"commits_pushed"`
    Duration        time.Duration     `json:"duration"`
    CompletedAt     time.Time         `json:"completed_at"`
}
```

**Causation:** `push.started`

---

### push.failed

Emitted when push fails.

**Producer:** Feature Worker

**Payload:**
```go
type PushFailedPayload struct {
    BranchName      string            `json:"branch_name"`
    ErrorCode       string            `json:"error_code"`
    ErrorMessage    string            `json:"error_message"`
    Retryable       bool              `json:"retryable"`
    FailedAt        time.Time         `json:"failed_at"`
}
```

**Causation:** `push.started`

---

## Event Causality Chains

### Successful Feature Execution

```
feature.requested
└── analysis.started
    ├── repository.clone.started
    │   └── repository.clone.completed
    ├── codebase.analyzed
    ├── dependency.graph.built
    ├── scope.identified
    └── analysis.completed
        └── planning.started
            ├── llm.invocation.started
            │   └── llm.response.received
            ├── plan.step.defined (×N)
            └── plan.generated
                └── execution.started
                    └── step.started (step 0)
                        ├── llm.invocation.started
                        │   └── llm.response.received
                        ├── code.change.generated
                        ├── file.modified (×N)
                        ├── syntax.validated
                        ├── local.commit.created
                        └── step.completed
                            └── step.started (step 1)
                                └── ... (repeat for each step)
                                    └── step.completed (final step)
                                        └── execution.completed
                                            └── verification.started
                                                ├── sandbox.created
                                                ├── build.started
                                                │   └── build.completed
                                                ├── tests.started
                                                │   └── tests.completed
                                                ├── sandbox.destroyed
                                                └── verification.passed
                                                    ├── push.started
                                                    │   └── push.completed
                                                    └── feature.delivered
```

### Failed Feature Execution (Step Failure)

```
feature.requested
└── analysis.started
    └── analysis.completed
        └── planning.started
            └── plan.generated
                └── execution.started
                    └── step.started (step 2)
                        ├── llm.invocation.started
                        │   └── llm.response.received
                        ├── code.change.generated
                        └── step.failed (syntax error)
                            └── step.started (step 2, retry 1)
                                └── step.failed (max retries)
                                    └── execution.failed
                                        └── feature.failed
```

---

## Event Handlers

### Handler Interface

```go
// EventHandler processes events of a specific type
type EventHandler interface {
    // Name returns the event type this handler processes
    Name() string

    // PayloadType returns the expected payload type for unmarshaling
    PayloadType() any

    // Validate validates the event payload before processing
    Validate(ctx context.Context, payload any) error

    // Execute processes the event and may emit new events
    Execute(ctx context.Context, payload any) error
}
```

### Handler Registration

```go
// Event handler registration in main.go
svc.Init(ctx,
    frame.WithRegisterEvents(
        // Lifecycle
        events.NewFeatureRequestedHandler(deps),

        // Analysis
        events.NewAnalysisStartedHandler(deps),
        events.NewRepositoryCloneHandler(deps),
        events.NewCodebaseAnalysisHandler(deps),
        events.NewAnalysisCompletedHandler(deps),

        // Planning
        events.NewPlanningStartedHandler(deps),
        events.NewPlanGeneratedHandler(deps),

        // Execution
        events.NewExecutionStartedHandler(deps),
        events.NewStepExecutionHandler(deps),
        events.NewStepCompletedHandler(deps),

        // Verification
        events.NewVerificationStartedHandler(deps),
        events.NewBuildExecutionHandler(deps),
        events.NewTestExecutionHandler(deps),
        events.NewVerificationCompletedHandler(deps),

        // Git
        events.NewBranchPushHandler(deps),
    ),
)
```

---

## Event Retention and Replay

### Retention Policy

| Event Category | Retention | Compaction |
|----------------|-----------|------------|
| Lifecycle | 90 days | None |
| Analysis | 30 days | None |
| Planning | 30 days | None |
| Execution | 30 days | None |
| Verification | 30 days | None |
| Git | 90 days | None |

### Replay Capability

Events support deterministic replay for:
- Crash recovery
- Debugging
- Audit
- State reconstruction

```go
// ReplayEvents replays events from a sequence number
func (w *Worker) ReplayEvents(ctx context.Context, featureID string, fromSeq uint64) error {
    events, err := w.eventStore.GetEvents(ctx, featureID, fromSeq)
    if err != nil {
        return fmt.Errorf("get events: %w", err)
    }

    for _, event := range events {
        if err := w.processEvent(ctx, event); err != nil {
            return fmt.Errorf("process event %s: %w", event.EventID, err)
        }
    }

    return nil
}
```
