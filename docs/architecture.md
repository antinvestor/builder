# builder Architecture

## Table of Contents

1. [System Overview](#system-overview)
2. [Component Architecture](#component-architecture)
3. [Feature Execution Lifecycle](#feature-execution-lifecycle)
4. [Concurrency and Isolation Model](#concurrency-and-isolation-model)
5. [Event System Design](#event-system-design)
6. [Git Operations Layer](#git-operations-layer)
7. [LLM Integration](#llm-integration)
8. [Sandbox Execution](#sandbox-execution)
9. [Persistence Strategy](#persistence-strategy)
10. [Failure and Recovery](#failure-and-recovery)
11. [Known Limitations](#known-limitations)

---

## System Overview

builder is an autonomous feature-building platform that uses event-driven architecture to orchestrate the end-to-end process of analyzing codebases, planning implementations, generating code, and verifying changes.

### Current Implementation Status

> **Note:** This document reflects the actual implemented system as of the current version.
> Some features are fully implemented, while others are planned for future releases.

| Feature | Status | Notes |
|---------|--------|-------|
| Gateway API | ✅ Implemented | HTTP entry point with queue publishing |
| Worker Service | ✅ Implemented | LLM pipeline, git operations |
| Reviewer Service | ✅ Implemented | Security review, kill switch |
| Executor Service | ✅ Implemented | Docker sandbox execution |
| Webhook Service | ✅ Implemented | External integrations |
| Multi-provider LLM | ✅ Implemented | Anthropic, OpenAI, Google |
| Queue Infrastructure | ✅ Implemented | NATS/Kafka with retry levels |
| Persistent State | ⚠️ Partial | Database for executions, in-memory for workspace tracking |

### Design Principles

| Principle | Implementation |
|-----------|----------------|
| **Event-Driven** | Queue-based communication between services |
| **Multi-Provider LLM** | Fallback support across Anthropic, OpenAI, Google |
| **Horizontal Scalability** | Stateless services with semaphore-based concurrency |
| **Isolation** | Workspace isolation per execution, Docker sandboxes |
| **Retry with Backoff** | 3-level retry queues with exponential backoff |
| **Observability** | OpenTelemetry integration via Frame framework |

### System Boundaries

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              PLATFORM BOUNDARY                                   │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  CONTROL PLANE                          DATA PLANE                              │
│  ┌─────────────────────────┐            ┌─────────────────────────┐            │
│  │ • Gateway (HTTP API)    │            │ • Worker (LLM Pipeline) │            │
│  │ • Webhook Service       │            │ • Git Operations        │            │
│  │ • Health Endpoints      │            │ • Reviewer (Security)   │            │
│  └─────────────────────────┘            │ • Executor (Sandbox)    │            │
│                                          └─────────────────────────┘            │
│                                                                                  │
│  QUEUE INFRASTRUCTURE                   PERSISTENCE                             │
│  ┌─────────────────────────┐            ┌─────────────────────────┐            │
│  │ • NATS/Kafka            │            │ • PostgreSQL            │            │
│  │ • 3-Level Retry Queues  │            │ • Local Workspace FS    │            │
│  │ • Dead Letter Queues    │            │ • In-Memory State*      │            │
│  └─────────────────────────┘            └─────────────────────────┘            │
│                                                                                  │
│  * Workspace and deduplication tracking are in-memory (see Known Limitations)  │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Component Architecture

### Implemented Services (5 Services)

The platform consists of five independently deployable services:

| Service | Type | Responsibility | Scaling Model | Concurrency Control |
|---------|------|----------------|---------------|---------------------|
| **Gateway** | Stateless | HTTP API, queue publishing | Horizontal | None (stateless) |
| **Worker** | Semi-Stateful | LLM pipeline, git operations, patch generation | Queue depth | Clone semaphore (10), Execution semaphore (50) |
| **Reviewer** | Stateless | Security review, architecture review, kill switch | Horizontal | None |
| **Executor** | Stateful | Docker sandbox, test execution | Careful | Docker socket access required |
| **Webhook** | Stateless | External integrations, callbacks | Horizontal | None |

#### Service Dependencies

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           SERVICE DEPENDENCIES                                │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                               │
│   Gateway ──────────────────► NATS/Kafka ◄─────────────── Webhook            │
│                                   │                                           │
│                    ┌──────────────┼──────────────┐                           │
│                    ▼              ▼              ▼                           │
│                ┌────────┐    ┌────────┐    ┌────────┐                       │
│                │ Worker │    │Reviewer│    │Executor│                       │
│                └───┬────┘    └────────┘    └────────┘                       │
│                    │                                                          │
│         ┌─────────┼─────────┐                                                │
│         ▼         ▼         ▼                                                │
│    PostgreSQL   LLM APIs   Filesystem                                        │
│                                                                               │
│   Required Infrastructure:                                                    │
│   • PostgreSQL - State persistence                                           │
│   • NATS/Kafka - Queue infrastructure                                        │
│   • Docker - Executor sandbox (optional)                                     │
│   • LLM API Keys - Worker (Anthropic, OpenAI, or Google)                    │
│                                                                               │
└──────────────────────────────────────────────────────────────────────────────┘
```

#### Persistence Components (Actual)

| Component | Technology | Purpose | Status |
|-----------|------------|---------|--------|
| **Queue** | NATS/Kafka | Event routing, retry queues | ✅ Implemented |
| **State Store** | PostgreSQL | Execution state, repository registry | ✅ Implemented |
| **Workspace Tracking** | In-Memory Map | Track active workspaces | ⚠️ Non-persistent |
| **Deduplication Store** | In-Memory Map | Request deduplication | ⚠️ Non-persistent |
| **Lock Manager** | Database-backed | Distributed locking | ✅ Implemented |

### Component Interaction Diagram

```
                                 ┌───────────────────────┐
                                 │     External Client   │
                                 └───────────┬───────────┘
                                             │
                                             ▼
┌────────────────────────────────────────────────────────────────────────────────┐
│                              API GATEWAY                                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │
│  │   Auth/TLS   │  │ Rate Limit   │  │   Routing    │  │  Validation  │        │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘        │
└────────────────────────────────────────────────────────────────────────────────┘
                                             │
                    ┌────────────────────────┼────────────────────────┐
                    │                        │                        │
                    ▼                        ▼                        ▼
         ┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐
         │ Feature Service  │    │Repository Service│    │Observability Svc │
         │                  │    │                  │    │                  │
         │ • Create Feature │    │ • Register Repo  │    │ • Query Status   │
         │ • Get Status     │    │ • Update Creds   │    │ • List Events    │
         │ • Cancel Feature │    │ • Validate Access│    │ • Get Metrics    │
         └────────┬─────────┘    └────────┬─────────┘    └──────────────────┘
                  │                       │
                  │                       │
                  ▼                       ▼
         ┌────────────────────────────────────────────────────────────────┐
         │                        EVENT BUS                               │
         │  ┌─────────────────────────────────────────────────────────┐  │
         │  │ Partition 0 │ Partition 1 │ Partition 2 │ ... │ Part N │  │
         │  │ [Feature A] │ [Feature B] │ [Feature C] │     │        │  │
         │  └─────────────────────────────────────────────────────────┘  │
         └────────────────────────────────┬───────────────────────────────┘
                                          │
                    ┌─────────────────────┼─────────────────────┐
                    │                     │                     │
                    ▼                     ▼                     ▼
         ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
         │  Feature Worker  │  │  Feature Worker  │  │  Feature Worker  │
         │    (Pod 0)       │  │    (Pod 1)       │  │    (Pod N)       │
         │                  │  │                  │  │                  │
         │  Partitions:     │  │  Partitions:     │  │  Partitions:     │
         │  [0, 3, 6, ...]  │  │  [1, 4, 7, ...]  │  │  [2, 5, 8, ...]  │
         └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘
                  │                     │                     │
                  └─────────────────────┼─────────────────────┘
                                        │
                    ┌───────────────────┼───────────────────┐
                    │                   │                   │
                    ▼                   ▼                   ▼
         ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
         │ Git Operations   │ │ LLM Orchestrator │ │ Code Analyzer    │
         │ Service          │ │ (BAML Runtime)   │ │ Service          │
         │                  │ │                  │ │                  │
         │ • Clone/Fetch    │ │ • Analyze        │ │ • Parse AST      │
         │ • Branch/Commit  │ │ • Plan           │ │ • Dependency Map │
         │ • Push/Merge     │ │ • Generate       │ │ • Impact Scope   │
         └────────┬─────────┘ └────────┬─────────┘ └────────┬─────────┘
                  │                    │                    │
                  └────────────────────┼────────────────────┘
                                       │
                                       ▼
                            ┌──────────────────┐
                            │ Sandbox Manager  │
                            │                  │
                            │ • Create/Destroy │
                            │ • Execute Build  │
                            │ • Execute Tests  │
                            │ • Capture Output │
                            └────────┬─────────┘
                                     │
                  ┌──────────────────┼──────────────────┐
                  │                  │                  │
                  ▼                  ▼                  ▼
         ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
         │   State Store    │ │  Artifact Store  │ │  Secrets Store   │
         │   (PostgreSQL)   │ │  (S3-compatible) │ │  (Vault)         │
         └──────────────────┘ └──────────────────┘ └──────────────────┘
```

---

## Feature Execution Lifecycle

### State Machine Definition

```
                                      ┌──────────────────┐
                                      │                  │
                                      ▼                  │ retry
┌───────────┐    ┌───────────┐    ┌───────────┐    ┌───────────┐
│           │    │           │    │           │    │           │
│  PENDING  │───▶│ ANALYZING │───▶│ PLANNING  │───▶│ EXECUTING │──┐
│           │    │           │    │           │    │           │  │
└───────────┘    └───────────┘    └───────────┘    └─────┬─────┘  │
                                                         │        │
                      ┌──────────────────────────────────┘        │
                      │                                           │
                      ▼                                           │
               ┌───────────┐    ┌───────────┐    ┌───────────┐   │
               │           │    │           │    │           │   │
               │ VERIFYING │───▶│ COMPLETED │    │  FAILED   │◀──┘
               │           │    │           │    │           │
               └───────────┘    └───────────┘    └───────────┘
                      │                                 ▲
                      │                                 │
                      └─────────────────────────────────┘
```

### State Definitions

#### PENDING
**Entry Event:** `FeatureRequested`

**Activities:**
- Validate feature specification
- Check repository accessibility
- Acquire repository-level lock (prevents concurrent features on same branch)
- Initialize feature execution context

**Exit Events:** `AnalysisStarted` | `FeatureFailed`

**Invariants:**
- Feature specification is syntactically valid
- Repository exists and is accessible
- No conflicting feature execution on same target branch

---

#### ANALYZING
**Entry Event:** `AnalysisStarted`

**Activities:**
- Clone or fetch repository to workspace
- Checkout target branch
- Analyze codebase structure (directory layout, build system, test framework)
- Build dependency graph
- Identify files in scope for modification
- Extract relevant context for LLM

**Exit Events:** `AnalysisCompleted` | `AnalysisFailed`

**Sub-Events:**
```
AnalysisStarted
├── RepositoryCloneStarted
├── RepositoryCloneCompleted
├── CodebaseAnalysisStarted
├── DependencyGraphBuilt
├── ScopeIdentified
└── AnalysisCompleted
```

**Invariants:**
- Workspace is isolated and clean
- Analysis results are deterministic for same commit

---

#### PLANNING
**Entry Event:** `AnalysisCompleted`

**Activities:**
- Invoke LLM to generate implementation plan
- Decompose plan into discrete, ordered steps
- Validate plan feasibility (files exist, dependencies available)
- Estimate resource requirements

**Exit Events:** `PlanGenerated` | `PlanningFailed`

**Sub-Events:**
```
PlanningStarted
├── ContextPrepared
├── LLMInvocationStarted
├── LLMResponseReceived
├── PlanParsed
├── PlanValidated
├── PlanStepDefined (repeated)
└── PlanGenerated
```

**Invariants:**
- Plan contains at least one step
- Each step has clear inputs/outputs
- Steps are topologically ordered by dependencies

---

#### EXECUTING
**Entry Event:** `PlanGenerated`

**Activities:**
- Execute plan steps sequentially
- For each step:
  - Invoke LLM to generate code changes
  - Apply changes to workspace
  - Validate syntax (parse check)
  - Commit to local branch

**Exit Events:** `ExecutionCompleted` | `ExecutionFailed`

**Sub-Events:**
```
ExecutionStarted
├── StepStarted
│   ├── LLMInvocationStarted
│   ├── LLMResponseReceived
│   ├── CodeChangeGenerated
│   ├── FileModified (repeated)
│   ├── SyntaxValidated
│   ├── LocalCommitCreated
│   └── StepCompleted
├── StepStarted (repeated for each step)
└── ExecutionCompleted
```

**Invariants:**
- Steps execute in order
- Each step either completes fully or fails atomically
- Workspace remains in valid state after each step

---

#### VERIFYING
**Entry Event:** `ExecutionCompleted`

**Activities:**
- Build project in sandbox
- Run test suite in sandbox
- Perform static analysis (lint, type check)
- Validate no regressions

**Exit Events:** `VerificationPassed` | `VerificationFailed`

**Sub-Events:**
```
VerificationStarted
├── SandboxCreated
├── BuildStarted
├── BuildCompleted
├── TestsStarted
├── TestsCompleted
├── StaticAnalysisStarted
├── StaticAnalysisCompleted
├── SandboxDestroyed
└── VerificationPassed
```

**Invariants:**
- Sandbox is isolated from host
- Build/test results are deterministic
- Resource limits enforced

---

#### COMPLETED
**Entry Event:** `VerificationPassed`

**Activities:**
- Push branch to remote
- Create artifacts (patch files, documentation)
- Release repository lock
- Emit completion metrics

**Exit Events:** `FeatureDelivered`

**Sub-Events:**
```
CompletionStarted
├── BranchPushStarted
├── BranchPushCompleted
├── ArtifactsCreated
├── LockReleased
└── FeatureDelivered
```

---

#### FAILED
**Entry Event:** Any failure event

**Activities:**
- Record failure context and diagnostics
- Release repository lock
- Clean up workspace
- Emit failure metrics

**Exit Events:** `FeatureFailed`

**Recovery:**
- Transient failures may trigger automatic retry
- Semantic failures may trigger re-planning
- Permanent failures terminate execution

---

### Execution Timeline Example

```
Time ──────────────────────────────────────────────────────────────────────────▶

     │FeatureRequested                                                          │
     ├─────────────────────────────────────────────────────────────────────────▶│
     │                                                                          │
     │  ┌─── PENDING ───┐                                                       │
     │  │ • Validate    │                                                       │
     │  │ • Lock repo   │                                                       │
     │  └───────────────┘                                                       │
     │                   │AnalysisStarted                                       │
     │                   ├─────────────────────────────────────────────────────▶│
     │                   │                                                      │
     │                   │  ┌─── ANALYZING ───────────────────┐                │
     │                   │  │ • Clone repository              │                │
     │                   │  │ • Analyze codebase structure    │                │
     │                   │  │ • Build dependency graph        │                │
     │                   │  └─────────────────────────────────┘                │
     │                   │                                    │AnalysisCompleted
     │                   │                                    ├─────────────────▶
     │                   │                                    │
     │                   │                                    │  ┌─ PLANNING ─┐
     │                   │                                    │  │ • LLM plan │
     │                   │                                    │  │ • Validate │
     │                   │                                    │  └────────────┘
     │                   │                                    │              │PlanGenerated
     │                   │                                    │              ├────────────▶
     │                   │                                    │              │
     │                   │                                    │              │  ┌── EXEC ──────────┐
     │                   │                                    │              │  │ • Step 1: modify │
     │                   │                                    │              │  │ • Step 2: add    │
     │                   │                                    │              │  │ • Step N: update │
     │                   │                                    │              │  └──────────────────┘
     │                   │                                    │              │                    │
     │                   │                                    │              │                    │ExecutionCompleted
     │                   │                                    │              │                    ├─────────────────▶
     │                   │                                    │              │                    │
     │                   │                                    │              │                    │ ┌─ VERIFY ─┐
     │                   │                                    │              │                    │ │ • Build  │
     │                   │                                    │              │                    │ │ • Test   │
     │                   │                                    │              │                    │ │ • Lint   │
     │                   │                                    │              │                    │ └──────────┘
     │                   │                                    │              │                    │            │
     │                   │                                    │              │                    │            │VerificationPassed
     │                   │                                    │              │                    │            ├─────────────▶
     │                   │                                    │              │                    │            │
     │                   │                                    │              │                    │            │ ┌─COMPLETE─┐
     │                   │                                    │              │                    │            │ │• Push    │
     │                   │                                    │              │                    │            │ │• Cleanup │
     │                   │                                    │              │                    │            │ └──────────┘
     │                   │                                    │              │                    │            │           │
     │                   │                                    │              │                    │            │           │FeatureDelivered
     │                   │                                    │              │                    │            │           └─────────────▶
```

---

## Concurrency and Isolation Model

### Isolation Guarantees

| Boundary | Mechanism | Guarantee |
|----------|-----------|-----------|
| **Feature-to-Feature** | Event partitioning | No shared state between features |
| **Workspace** | Filesystem isolation | Each feature has unique workspace path |
| **Git Operations** | Repository-level locking | Prevents branch conflicts |
| **Sandbox** | Container namespaces | Complete process/network isolation |
| **Credentials** | Scoped leases | Credentials bound to specific feature execution |

### Concurrency Rules

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    CONCURRENCY MODEL                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  RULE 1: One worker per partition at any time                           │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  Consumer Group Protocol ensures exclusive partition ownership   │    │
│  │  • Worker heartbeat every 3s                                    │    │
│  │  • Session timeout 30s                                          │    │
│  │  • Rebalance on worker join/leave                               │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  RULE 2: Events for same feature always route to same partition         │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  partition = hash(feature_execution_id) % partition_count       │    │
│  │  • Preserves total ordering per feature                         │    │
│  │  • Enables state machine consistency                            │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  RULE 3: Repository locks prevent concurrent modifications              │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  Lock key: repository_id + target_branch                        │    │
│  │  • Acquired in PENDING state                                    │    │
│  │  • Released in COMPLETED or FAILED state                        │    │
│  │  • TTL: feature execution timeout + buffer                      │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  RULE 4: External service calls are idempotent                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  • Git operations use deterministic branch/commit naming        │    │
│  │  • LLM calls include request ID for deduplication               │    │
│  │  • Sandbox operations are stateless                             │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Worker Assignment

```
┌─────────────────────────────────────────────────────────────────┐
│                 PARTITION ASSIGNMENT                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Given: 64 partitions, 8 workers                                │
│                                                                  │
│  Worker 0: Partitions [0, 8, 16, 24, 32, 40, 48, 56]           │
│  Worker 1: Partitions [1, 9, 17, 25, 33, 41, 49, 57]           │
│  Worker 2: Partitions [2, 10, 18, 26, 34, 42, 50, 58]          │
│  Worker 3: Partitions [3, 11, 19, 27, 35, 43, 51, 59]          │
│  Worker 4: Partitions [4, 12, 20, 28, 36, 44, 52, 60]          │
│  Worker 5: Partitions [5, 13, 21, 29, 37, 45, 53, 61]          │
│  Worker 6: Partitions [6, 14, 22, 30, 38, 46, 54, 62]          │
│  Worker 7: Partitions [7, 15, 23, 31, 39, 47, 55, 63]          │
│                                                                  │
│  On Worker Failure (Worker 3 dies):                             │
│  ─────────────────────────────────────────                      │
│  Worker 0: Partitions [0, 8, 16, 24, 32, 40, 48, 56, 3, 35]    │
│  Worker 1: Partitions [1, 9, 17, 25, 33, 41, 49, 57, 11, 43]   │
│  Worker 2: Partitions [2, 10, 18, 26, 34, 42, 50, 58, 19, 51]  │
│  Worker 4: Partitions [4, 12, 20, 28, 36, 44, 52, 60, 27, 59]  │
│  ...                                                             │
│                                                                  │
│  Rebalancing Strategy: Sticky assignment (minimize partition    │
│  movement to preserve cache locality)                           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Feature Execution Context

Each active feature maintains isolated execution context:

```go
// FeatureExecutionContext holds all state for a single feature execution
type FeatureExecutionContext struct {
    // Identity
    ExecutionID    string    // UUID v4, globally unique
    RepositoryID   string    // Reference to registered repository
    FeatureSpec    FeatureSpec // Original feature specification

    // Workspace
    WorkspacePath  string    // /var/feature/workspaces/{execution_id}
    BranchName     string    // feature/{execution_id_short}
    BaseCommit     string    // SHA of base commit

    // State Machine
    CurrentState   FeatureState
    SequenceCursor uint64    // Last processed event sequence
    StepCursor     uint32    // Current plan step index

    // Plan (populated after PLANNING)
    Plan           *ExecutionPlan

    // Artifacts (populated during execution)
    Commits        []CommitReference
    ArtifactRefs   []ArtifactReference

    // Resources
    CredentialLease  CredentialLease
    RepositoryLock   DistributedLock
    SandboxID        string // Active sandbox (if any)
}
```

---

## Event System Design

### Event Schema

```protobuf
message Event {
    // Identity
    string event_id = 1;              // UUID v7 (time-ordered)
    string feature_execution_id = 2;  // Partition key
    string event_type = 3;            // Enum: FeatureRequested, AnalysisStarted, etc.

    // Ordering
    uint64 sequence_number = 4;       // Per-feature monotonic counter
    google.protobuf.Timestamp timestamp = 5;

    // Causality
    string causation_id = 6;          // Event that caused this event
    string correlation_id = 7;        // Root request ID (spans all events)

    // Payload
    bytes payload = 8;                // Protobuf-encoded, type depends on event_type

    // Metadata
    EventMetadata metadata = 9;
}

message EventMetadata {
    string producer_id = 1;           // Service/worker that emitted
    string schema_version = 2;        // Semver for payload schema
    map<string, string> tags = 3;     // Additional context
}
```

### Event Types

#### Lifecycle Events

| Event | Payload | Description |
|-------|---------|-------------|
| `FeatureRequested` | FeatureSpec | Initial feature submission |
| `FeatureDelivered` | DeliveryReport | Successful completion |
| `FeatureFailed` | FailureReport | Terminal failure |
| `FeatureCancelled` | CancellationReason | User-initiated cancellation |

#### Analysis Events

| Event | Payload | Description |
|-------|---------|-------------|
| `AnalysisStarted` | AnalysisContext | Begin analysis phase |
| `RepositoryCloneStarted` | CloneRequest | Starting git clone |
| `RepositoryCloneCompleted` | CloneResult | Clone finished |
| `CodebaseAnalyzed` | AnalysisResult | Structure analysis complete |
| `DependencyGraphBuilt` | DependencyGraph | Dependencies mapped |
| `AnalysisCompleted` | AnalysisSummary | Analysis phase complete |
| `AnalysisFailed` | AnalysisError | Analysis phase failed |

#### Planning Events

| Event | Payload | Description |
|-------|---------|-------------|
| `PlanningStarted` | PlanningContext | Begin planning phase |
| `LLMInvocationStarted` | LLMRequest | Starting LLM call |
| `LLMResponseReceived` | LLMResponse | LLM response received |
| `PlanStepDefined` | PlanStep | Individual step defined |
| `PlanGenerated` | ExecutionPlan | Complete plan ready |
| `PlanningFailed` | PlanningError | Planning phase failed |

#### Execution Events

| Event | Payload | Description |
|-------|---------|-------------|
| `ExecutionStarted` | ExecutionContext | Begin execution phase |
| `StepStarted` | StepContext | Starting plan step |
| `CodeChangeGenerated` | CodeChange | LLM generated code |
| `FileModified` | FileModification | File written to workspace |
| `LocalCommitCreated` | CommitInfo | Git commit created |
| `StepCompleted` | StepResult | Step finished |
| `StepFailed` | StepError | Step failed |
| `ExecutionCompleted` | ExecutionSummary | All steps complete |
| `ExecutionFailed` | ExecutionError | Execution phase failed |

#### Verification Events

| Event | Payload | Description |
|-------|---------|-------------|
| `VerificationStarted` | VerificationContext | Begin verification |
| `SandboxCreated` | SandboxInfo | Sandbox provisioned |
| `BuildExecuted` | BuildResult | Build completed |
| `TestsExecuted` | TestResult | Tests completed |
| `StaticAnalysisExecuted` | AnalysisResult | Lint/typecheck done |
| `VerificationPassed` | VerificationSummary | All checks passed |
| `VerificationFailed` | VerificationError | Verification failed |
| `SandboxDestroyed` | SandboxCleanup | Sandbox removed |

#### Git Events

| Event | Payload | Description |
|-------|---------|-------------|
| `BranchCreated` | BranchInfo | New branch created |
| `CommitCreated` | CommitInfo | Commit to branch |
| `PushStarted` | PushRequest | Starting push to remote |
| `PushCompleted` | PushResult | Push succeeded |
| `PushFailed` | PushError | Push failed |

### Event Handlers

Following the service-notification pattern, event handlers implement a standard interface:

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

#### Handler Registration

```go
// In main.go
svc.Init(ctx,
    frame.WithRegisterEvents(
        // Lifecycle handlers
        &events.FeatureRequestedHandler{...},
        &events.FeatureCancellationHandler{...},

        // Analysis handlers
        &events.AnalysisStartedHandler{...},
        &events.CodebaseAnalysisHandler{...},

        // Planning handlers
        &events.PlanningHandler{...},
        &events.PlanValidationHandler{...},

        // Execution handlers
        &events.StepExecutionHandler{...},
        &events.CodeGenerationHandler{...},

        // Verification handlers
        &events.BuildExecutionHandler{...},
        &events.TestExecutionHandler{...},

        // Git handlers
        &events.BranchPushHandler{...},
    ),
)
```

---

## Git Operations Layer

### Abstraction Interface

```go
// GitOperations defines the provider-agnostic interface for git operations
type GitOperations interface {
    // Repository lifecycle
    Clone(ctx context.Context, req *CloneRequest) (*Workspace, error)
    Fetch(ctx context.Context, workspace *Workspace, refs []string) error

    // Branch operations
    CreateBranch(ctx context.Context, workspace *Workspace, name, base string) error
    Checkout(ctx context.Context, workspace *Workspace, ref string) error
    DeleteBranch(ctx context.Context, workspace *Workspace, name string) error

    // Working copy operations
    Stage(ctx context.Context, workspace *Workspace, paths []string) error
    Commit(ctx context.Context, workspace *Workspace, req *CommitRequest) (*CommitResult, error)
    Reset(ctx context.Context, workspace *Workspace, ref string, mode ResetMode) error

    // Remote operations
    Push(ctx context.Context, workspace *Workspace, req *PushRequest) error

    // Query operations
    Diff(ctx context.Context, workspace *Workspace, base, head string) (*DiffResult, error)
    Log(ctx context.Context, workspace *Workspace, opts *LogOptions) ([]*CommitInfo, error)
    Status(ctx context.Context, workspace *Workspace) (*StatusResult, error)
    Show(ctx context.Context, workspace *Workspace, ref string) (*CommitDetail, error)

    // Cleanup
    CleanWorkspace(ctx context.Context, workspace *Workspace) error
}
```

### Implementation Strategy

```
┌─────────────────────────────────────────────────────────────────┐
│              GIT ABSTRACTION LAYER                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ GitOperations Interface                                  │    │
│  └────────────────────────┬────────────────────────────────┘    │
│                           │                                      │
│           ┌───────────────┼───────────────┐                     │
│           │               │               │                     │
│           ▼               ▼               ▼                     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │ GitCLI      │  │ Libgit2     │  │ Go-git      │              │
│  │ Adapter     │  │ Adapter     │  │ Adapter     │              │
│  │             │  │             │  │ (Pure Go)   │              │
│  │ Features:   │  │ Features:   │  │ Features:   │              │
│  │ • Full git  │  │ • Fast      │  │ • Portable  │              │
│  │ • Familiar  │  │ • Low-level │  │ • No deps   │              │
│  │ • Shell out │  │ • CGO req'd │  │ • Limited   │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
│                                                                  │
│  Selection: Config-driven, default GitCLI                       │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Credential Handling

```go
// CredentialProvider abstracts credential retrieval
type CredentialProvider interface {
    // GetCredential retrieves credentials for a repository
    GetCredential(ctx context.Context, repoID string) (*Credential, *CredentialLease, error)

    // ReleaseCredential releases a credential lease
    ReleaseCredential(ctx context.Context, lease *CredentialLease) error

    // ValidateCredential checks if credentials are still valid
    ValidateCredential(ctx context.Context, cred *Credential) error
}

// Credential represents git authentication
type Credential struct {
    Type        CredentialType // SSH, Token, OAuth
    SSHKey      []byte         // For SSH auth
    Token       string         // For token auth
    Username    string         // For HTTP auth
    Password    string         // For HTTP auth
    OAuthToken  string         // For OAuth
}

// CredentialLease tracks credential usage
type CredentialLease struct {
    LeaseID     string
    RepoID      string
    FeatureID   string
    ExpiresAt   time.Time
    Renewable   bool
}
```

---

## LLM Integration

### Multi-Provider Client Architecture

The LLM integration uses a multi-provider client with automatic fallback support:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        LLM CLIENT ARCHITECTURE                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │                    MultiProviderClient                              │     │
│  │                                                                     │     │
│  │  • Fallback across providers on failure                            │     │
│  │  • Retry with exponential backoff (default 3 retries)              │     │
│  │  • Usage tracking and cost estimation                              │     │
│  │  • Prompt building from Go templates                               │     │
│  └───────────────────────────┬────────────────────────────────────────┘     │
│                              │                                               │
│              ┌───────────────┼───────────────┐                              │
│              │               │               │                              │
│              ▼               ▼               ▼                              │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐               │
│  │ AnthropicClient │ │   OpenAIClient  │ │  GoogleClient   │               │
│  │                 │ │                 │ │                 │               │
│  │ Models:         │ │ Models:         │ │ Models:         │               │
│  │ • claude-sonnet │ │ • gpt-4o        │ │ • gemini-2.0    │               │
│  │ • claude-opus   │ │ • gpt-4o-mini   │ │                 │               │
│  │ • claude-haiku  │ │                 │ │                 │               │
│  └─────────────────┘ └─────────────────┘ └─────────────────┘               │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### LLM Pipeline Functions

The code generation pipeline consists of four sequential functions:

| Function | Purpose | Input | Output |
|----------|---------|-------|--------|
| **NormalizeSpec** | Parse and structure feature specification | FeatureSpecification + codebase context | NormalizedSpecification |
| **AnalyzeImpact** | Identify affected files and dependencies | NormalizedSpec + file contents | ImpactAnalysisResult |
| **GeneratePlan** | Create step-by-step implementation plan | NormalizedSpec + ImpactAnalysis | ImplementationPlan |
| **GenerateCode** | Generate code changes for each step | PlanStep + file contents | CodeGenerationResult |

### Pipeline Flow (Actual Implementation)

```
Worker Service                           BAMLClient                          MultiProviderClient
     │                                        │                                      │
     │  GeneratePatch(request)                │                                      │
     │───────────────────────────────────────▶│                                      │
     │                                        │                                      │
     │                                        │  1. buildCodebaseContext()           │
     │                                        │  2. detectLanguage()                 │
     │                                        │                                      │
     │                                        │  NormalizeSpec(input)                │
     │                                        │─────────────────────────────────────▶│
     │                                        │                                      │ Build prompt
     │                                        │                                      │ HTTP POST to provider
     │                                        │◀─────────────────────────────────────│
     │                                        │  NormalizedSpecification             │
     │                                        │                                      │
     │                                        │  3. readRelevantFiles()              │
     │                                        │                                      │
     │                                        │  AnalyzeImpact(input)                │
     │                                        │─────────────────────────────────────▶│
     │                                        │◀─────────────────────────────────────│
     │                                        │  ImpactAnalysisResult                │
     │                                        │                                      │
     │                                        │  GeneratePlan(input)                 │
     │                                        │─────────────────────────────────────▶│
     │                                        │◀─────────────────────────────────────│
     │                                        │  ImplementationPlan                  │
     │                                        │                                      │
     │                                        │  FOR EACH step IN plan.Steps:        │
     │                                        │    readFilesForStep()                │
     │                                        │    GenerateCode(step)                │
     │                                        │───────────────────────────────────▶  │
     │                                        │◀───────────────────────────────────  │
     │                                        │    Convert to Patch                  │
     │                                        │                                      │
     │  GeneratePatchResponse                 │                                      │
     │◀───────────────────────────────────────│                                      │
     │  {Patches, CommitMessage, TokensUsed}  │                                      │
```

### Prompt Templates

Prompts are defined as Go templates in `internal/llm/prompts.go`:

```go
// Template functions available
var templateFuncs = template.FuncMap{
    "join":   strings.Join,
    "indent": func(indent int, s string) string { ... },
    "sub":    func(a, b int) int { return a - b },
}

// Example: NormalizeSpec prompt structure
const normalizeSpecPrompt = `
You are analyzing a feature specification for a {{.Language}} codebase.
...
{{range $i, $criterion := .Spec.AcceptanceCriteria}}
{{sub $i 1}}. {{$criterion}}
{{end}}
...
`
```

### Error Handling and Retry

```go
// Retry configuration
const (
    defaultMaxRetries      = 3
    defaultTimeoutSeconds  = 120
)

// Error types that trigger provider fallback
var retryableErrors = []error{
    ErrRateLimited,        // Try next provider
    ErrQuotaExceeded,      // Don't retry (permanent)
    ErrContextTooLong,     // Don't retry (needs truncation)
    ErrAllProvidersFailed, // Terminal failure
}
```

### Configuration

```go
type ClientConfig struct {
    // Provider API keys (at least one required)
    AnthropicAPIKey string
    OpenAIAPIKey    string
    GoogleAPIKey    string

    // Model selection
    DefaultProvider Provider  // "anthropic", "openai", "google"
    DefaultModel    Model     // "claude-sonnet-4-20250514", etc.

    // Limits
    TimeoutSeconds  int       // Default: 120
    MaxRetries      int       // Default: 3
    MaxOutputTokens int       // Default: 16384
    Temperature     float64   // Default: 0.0
}
```

---

## Sandbox Execution

### Sandbox Interface

```go
// SandboxManager manages isolated execution environments
type SandboxManager interface {
    // Create provisions a new sandbox for a feature
    Create(ctx context.Context, req *SandboxRequest) (*Sandbox, error)

    // Execute runs a command in the sandbox
    Execute(ctx context.Context, sandbox *Sandbox, cmd *ExecuteRequest) (*ExecuteResult, error)

    // CopyIn copies files into the sandbox
    CopyIn(ctx context.Context, sandbox *Sandbox, src, dst string) error

    // CopyOut copies files out of the sandbox
    CopyOut(ctx context.Context, sandbox *Sandbox, src, dst string) error

    // Destroy removes the sandbox
    Destroy(ctx context.Context, sandbox *Sandbox) error
}

// SandboxRequest specifies sandbox requirements
type SandboxRequest struct {
    FeatureExecutionID  string
    Image               string              // Base image
    CPULimit            int                 // CPU cores
    MemoryLimitMB       int                 // Memory in MB
    DiskLimitMB         int                 // Disk in MB
    TimeoutSeconds      int                 // Execution timeout
    NetworkPolicy       NetworkPolicy       // Egress rules
    Environment         map[string]string   // Env vars
}
```

### Isolation Controls

```
┌─────────────────────────────────────────────────────────────────┐
│                 SANDBOX ISOLATION MODEL                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ NAMESPACE ISOLATION                                      │    │
│  │                                                          │    │
│  │  • PID namespace    → Process isolation                 │    │
│  │  • Mount namespace  → Filesystem isolation              │    │
│  │  • Network namespace → Network isolation                │    │
│  │  • User namespace   → UID/GID mapping                   │    │
│  │  • IPC namespace    → IPC isolation                     │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ RESOURCE LIMITS (cgroups v2)                             │    │
│  │                                                          │    │
│  │  • cpu.max         → CPU time limit                     │    │
│  │  • memory.max      → Memory limit                       │    │
│  │  • io.max          → Disk I/O limit                     │    │
│  │  • pids.max        → Process count limit                │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ SECURITY CONTROLS                                        │    │
│  │                                                          │    │
│  │  • Seccomp profile → Syscall filtering                  │    │
│  │  • AppArmor/SELinux → MAC enforcement                   │    │
│  │  • Read-only rootfs → Prevent host modification         │    │
│  │  • No capabilities → Drop all Linux capabilities        │    │
│  │  • Non-root user   → Run as unprivileged user           │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ NETWORK POLICY                                           │    │
│  │                                                          │    │
│  │  Default: Deny all                                      │    │
│  │  Whitelist:                                              │    │
│  │  • Package registries (npm, pypi, crates.io)            │    │
│  │  • Git remotes (specified in repository config)         │    │
│  │  • Internal services (LLM, state store)                 │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Persistence Strategy

### Event Store

The event store is the source of truth for all state transitions.

**Technology:** Kafka/Redpanda with compaction disabled

**Schema:**
```sql
-- Logical schema (events stored in Kafka topics)
-- Retention: 7-90 days configurable

Topic: feature.events.{partition}
Key: feature_execution_id
Value: Event (protobuf)

-- Consumer offsets tracked by Kafka consumer group
```

**Guarantees:**
- At-least-once delivery
- Strict ordering per partition
- Configurable retention

### State Store

Materialized projections for fast reads.

**Technology:** PostgreSQL with JSONB

**Schema:**
```sql
-- Feature execution state
CREATE TABLE feature_executions (
    id              UUID PRIMARY KEY,
    repository_id   UUID NOT NULL REFERENCES repositories(id),
    spec            JSONB NOT NULL,
    state           VARCHAR(32) NOT NULL,
    sequence_cursor BIGINT NOT NULL DEFAULT 0,
    plan            JSONB,
    error           JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX idx_feature_executions_repository ON feature_executions(repository_id);
CREATE INDEX idx_feature_executions_state ON feature_executions(state);
CREATE INDEX idx_feature_executions_created ON feature_executions(created_at DESC);

-- Repository registry
CREATE TABLE repositories (
    id              UUID PRIMARY KEY,
    tenant_id       UUID NOT NULL,
    name            VARCHAR(255) NOT NULL,
    url             TEXT NOT NULL,
    default_branch  VARCHAR(255) NOT NULL DEFAULT 'main',
    credential_id   UUID,
    properties      JSONB,
    state           VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_repositories_tenant_name ON repositories(tenant_id, name);

-- Execution steps
CREATE TABLE execution_steps (
    id                  UUID PRIMARY KEY,
    feature_execution_id UUID NOT NULL REFERENCES feature_executions(id),
    step_index          INT NOT NULL,
    description         TEXT NOT NULL,
    state               VARCHAR(32) NOT NULL,
    input               JSONB,
    output              JSONB,
    error               JSONB,
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ
);

CREATE INDEX idx_execution_steps_feature ON execution_steps(feature_execution_id);

-- Distributed locks
CREATE TABLE distributed_locks (
    lock_key        VARCHAR(255) PRIMARY KEY,
    owner_id        VARCHAR(255) NOT NULL,
    feature_id      UUID NOT NULL,
    acquired_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    CONSTRAINT fk_lock_feature FOREIGN KEY (feature_id)
        REFERENCES feature_executions(id) ON DELETE CASCADE
);

CREATE INDEX idx_distributed_locks_expires ON distributed_locks(expires_at);
```

### Artifact Store

Binary artifacts and large outputs.

**Technology:** S3-compatible object storage

**Structure:**
```
bucket: feature-artifacts
├── {feature_execution_id}/
│   ├── patches/
│   │   ├── step-0.patch
│   │   ├── step-1.patch
│   │   └── final.patch
│   ├── analysis/
│   │   ├── dependency-graph.json
│   │   └── scope-analysis.json
│   ├── verification/
│   │   ├── build-output.log
│   │   ├── test-results.xml
│   │   └── coverage-report.html
│   └── metadata.json
```

---

## Failure and Recovery

### Failure Classification

| Category | Examples | Strategy |
|----------|----------|----------|
| **Transient** | Network timeout, rate limit, temporary unavailable | Exponential backoff retry |
| **Deterministic** | Invalid input, auth revoked, repo not found | Immediate fail, no retry |
| **Semantic** | LLM unparseable, build failure, test failure | Contextual retry (re-prompt) |
| **Infrastructure** | Worker crash, DB unavailable, event bus down | Automatic recovery via replay |

### Retry Strategy

```go
// RetryPolicy defines retry behavior
type RetryPolicy struct {
    MaxAttempts     int           // Maximum retry attempts
    BaseDelay       time.Duration // Initial delay
    MaxDelay        time.Duration // Maximum delay cap
    Multiplier      float64       // Exponential multiplier
    JitterFraction  float64       // Random jitter (0-1)
}

// Default policies
var (
    TransientRetryPolicy = RetryPolicy{
        MaxAttempts:    5,
        BaseDelay:      1 * time.Second,
        MaxDelay:       60 * time.Second,
        Multiplier:     2.0,
        JitterFraction: 0.2,
    }

    SemanticRetryPolicy = RetryPolicy{
        MaxAttempts:    3,
        BaseDelay:      5 * time.Second,
        MaxDelay:       30 * time.Second,
        Multiplier:     1.5,
        JitterFraction: 0.1,
    }
)

// CalculateDelay computes the next retry delay
func (p RetryPolicy) CalculateDelay(attempt int) time.Duration {
    delay := float64(p.BaseDelay) * math.Pow(p.Multiplier, float64(attempt))
    if delay > float64(p.MaxDelay) {
        delay = float64(p.MaxDelay)
    }
    jitter := delay * p.JitterFraction * (rand.Float64()*2 - 1)
    return time.Duration(delay + jitter)
}
```

### Crash Recovery

```
┌─────────────────────────────────────────────────────────────────┐
│                 CRASH RECOVERY PROTOCOL                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. DETECTION                                                    │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Consumer group detects missing heartbeat               │    │
│  │  Timeout: 30 seconds                                    │    │
│  └─────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              ▼                                   │
│  2. REBALANCE                                                    │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Partitions reassigned to surviving workers             │    │
│  │  Sticky assignment minimizes movement                   │    │
│  └─────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              ▼                                   │
│  3. STATE RECOVERY                                               │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  For each assigned partition:                           │    │
│  │  a. Load last committed offset                          │    │
│  │  b. Query state store for in-flight features            │    │
│  │  c. Load feature execution context                      │    │
│  │  d. Validate sequence cursor                            │    │
│  └─────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              ▼                                   │
│  4. EVENT REPLAY                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  For each recovered feature:                            │    │
│  │  a. Seek to sequence_cursor + 1                         │    │
│  │  b. Replay events sequentially                          │    │
│  │  c. Apply idempotency guards                            │    │
│  │  d. Resume state machine                                │    │
│  └─────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              ▼                                   │
│  5. NORMAL OPERATION                                             │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Continue processing new events                         │    │
│  │  Emit RecoveryCompleted metric                          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Idempotency Guards

```go
// processEvent handles an event with idempotency guarantees
func (w *Worker) processEvent(ctx context.Context, execCtx *FeatureExecutionContext, event *Event) error {
    // Guard 1: Check if already processed
    if w.isEventProcessed(ctx, execCtx.ExecutionID, event.SequenceNumber) {
        w.logger.Debug("event already processed, skipping",
            "feature_id", execCtx.ExecutionID,
            "sequence", event.SequenceNumber,
        )
        return nil
    }

    // Guard 2: Validate sequence continuity
    if event.SequenceNumber != execCtx.SequenceCursor+1 {
        return fmt.Errorf("sequence gap: expected %d, got %d",
            execCtx.SequenceCursor+1, event.SequenceNumber)
    }

    // Process event
    result, err := w.handleEvent(ctx, execCtx, event)
    if err != nil {
        return fmt.Errorf("handle event: %w", err)
    }

    // Atomic commit: state + offset
    if err := w.commitTransaction(ctx, execCtx.ExecutionID, event.SequenceNumber, result); err != nil {
        return fmt.Errorf("commit transaction: %w", err)
    }

    // Update local cursor
    execCtx.SequenceCursor = event.SequenceNumber

    return nil
}
```

---

## Known Limitations

> **Important:** The following limitations should be addressed before production deployment.

### Critical Issues

| Issue | Component | Impact | Recommended Fix |
|-------|-----------|--------|-----------------|
| **In-Memory Workspace Tracking** | `repository/workspace.go` | Workspace mappings lost on restart; workspaces become orphaned | Implement database-backed workspace repository |
| **In-Memory Deduplication** | `deduplication/store.go` | Duplicate request detection lost on restart; may reprocess requests | Implement Redis or database-backed deduplication store |
| **Lock Polling Without Backoff** | `locks/lock_manager.go` | Aggressive polling (1s) under contention; CPU waste | Add exponential backoff to lock acquisition |
| **No LLM Rate Limiter** | `internal/llm/client.go` | Relies on provider rate limit errors; suboptimal | Add proactive token bucket rate limiter |
| **Cleanup Without Context** | `repository/cleanup.go` | Background goroutines don't respect shutdown signals | Add graceful shutdown with context propagation |

### Concurrency Safeguards (Currently Implemented)

These controls are in place and working:

| Control | Location | Configuration | Default |
|---------|----------|---------------|---------|
| Clone Semaphore | `repository/git.go` | `MaxConcurrentClones` | 10 |
| Execution Semaphore | `executor/service.go` | `MaxConcurrentExecutions` | 50 |
| Distributed Lock | `locks/lock_manager.go` | Database-backed | Per-repository locking |
| Request Deduplication | `deduplication/store.go` | In-memory | ⚠️ Non-persistent |

### Queue Resilience (Implemented)

Three-level retry with dead letter queue:

```
feature.requests         → L1 entry point
feature.requests.retry1  → L2 retry (5 min delay)
feature.requests.retry2  → L3 retry (30 min delay)
feature.requests.dlq     → Dead letter queue (requires manual intervention)
```

### Recommended Pre-Production Checklist

- [ ] **Issue #32:** Implement persistent workspace repository
- [ ] **Issue #18:** Implement persistent deduplication store (existing issue)
- [ ] **Issue #33:** Add exponential backoff to lock manager
- [ ] **Issue #34:** Add proactive LLM rate limiter
- [ ] **Issue #35:** Implement graceful shutdown for cleanup goroutines
- [ ] Configure monitoring/alerting for DLQ depth
- [ ] Load test with target concurrent execution count
- [ ] Configure appropriate resource limits in Kubernetes

---

## Next Steps

- [API Reference](./api-reference.md) - Detailed API documentation
- [Event Reference](./event-reference.md) - Complete event catalog
- [Security Model](./security-model.md) - Security architecture details
- [Deployment Guide](./deployment-guide.md) - Production deployment
- [Operations Guide](./operations-guide.md) - Day-2 operations
