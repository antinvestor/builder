# builder Agent Architecture

This document describes how to build agents for the autonomous feature-building platform using the Frame library primitives.

## Frame Library Integration

The service uses [github.com/pitabwire/frame](https://github.com/pitabwire/frame) for:
- Event handling and emission
- Queue publishing and subscription
- Service initialization and lifecycle
- Worker pool management
- Database operations

## Core Patterns

### 1. Event Handler Pattern

Events implement the Frame event interface with three core methods:

```go
package events

import (
    "context"
)

// EventHandler implements frame's event interface
type EventHandler interface {
    // Name returns the unique event identifier
    Name() string

    // PayloadType returns the type of payload this handler expects
    PayloadType() any

    // Validate validates the payload before execution (optional)
    Validate(ctx context.Context, payload any) error

    // Execute processes the event
    Execute(ctx context.Context, payload any) error
}
```

**Example Event Handler:**

```go
package events

import (
    "context"
    "fmt"

    "github.com/pitabwire/frame"
    "github.com/pitabwire/frame/queue"
)

const PatchGenerationEventName = "feature.patch.generation"

type PatchGenerationEvent struct {
    cfg         *config.FeatureConfig
    repoService repository.RepositoryService
    bamlClient  baml.Client
    queueMan    queue.Manager
}

func NewPatchGenerationEvent(
    cfg *config.FeatureConfig,
    repoService repository.RepositoryService,
    bamlClient baml.Client,
    queueMan queue.Manager,
) *PatchGenerationEvent {
    return &PatchGenerationEvent{
        cfg:         cfg,
        repoService: repoService,
        bamlClient:  bamlClient,
        queueMan:    queueMan,
    }
}

func (e *PatchGenerationEvent) Name() string {
    return PatchGenerationEventName
}

func (e *PatchGenerationEvent) PayloadType() any {
    return &PatchGenerationPayload{}
}

func (e *PatchGenerationEvent) Validate(ctx context.Context, payload any) error {
    p, ok := payload.(*PatchGenerationPayload)
    if !ok {
        return fmt.Errorf("invalid payload type")
    }
    if p.ExecutionID.IsZero() {
        return fmt.Errorf("execution_id is required")
    }
    return nil
}

func (e *PatchGenerationEvent) Execute(ctx context.Context, payload any) error {
    p := payload.(*PatchGenerationPayload)

    // 1. Process the patch generation
    result, err := e.generatePatch(ctx, p)
    if err != nil {
        // Emit failure event
        return e.emitFailure(ctx, p, err)
    }

    // 2. Emit success event
    return e.emitSuccess(ctx, p, result)
}
```

### 2. Queue Subscriber Pattern

For consuming from external queues (like Kafka/NATS):

```go
package queue

import (
    "context"
    "encoding/json"

    "github.com/pitabwire/frame/queue"
)

// FeatureRequestQueueHandler handles incoming feature requests
type FeatureRequestQueueHandler struct {
    featureService business.FeatureService
    eventsManager  events.Manager
}

// Ensure interface compliance
var _ queue.SubscribeWorker = (*FeatureRequestQueueHandler)(nil)

func NewFeatureRequestQueueHandler(
    featureService business.FeatureService,
    eventsManager events.Manager,
) *FeatureRequestQueueHandler {
    return &FeatureRequestQueueHandler{
        featureService: featureService,
        eventsManager:  eventsManager,
    }
}

// Handle processes incoming queue messages
func (h *FeatureRequestQueueHandler) Handle(
    ctx context.Context,
    headers map[string]string,
    payload []byte,
) error {
    var request FeatureRequest
    if err := json.Unmarshal(payload, &request); err != nil {
        return fmt.Errorf("unmarshal request: %w", err)
    }

    // Process the request
    execution, err := h.featureService.InitializeExecution(ctx, &request)
    if err != nil {
        return fmt.Errorf("initialize execution: %w", err)
    }

    // Emit event for next stage
    return h.eventsManager.Emit(ctx, events.RepositoryCheckoutEventName, &events.RepositoryCheckoutPayload{
        ExecutionID:   execution.ID,
        RepositoryURL: request.RepositoryURL,
        Branch:        request.Branch,
    })
}
```

### 3. Service Initialization

Register events, publishers, and subscribers during service startup:

```go
package main

import (
    "context"

    "github.com/pitabwire/frame"

    "github.com/antinvestor/builder/apps/default/config"
    "github.com/antinvestor/builder/apps/default/service/events"
    featurequeue "github.com/antinvestor/builder/apps/default/service/queue"
)

func main() {
    ctx := context.Background()

    // 1. Load configuration
    cfg, err := config.Load[config.FeatureConfig](ctx)
    if err != nil {
        panic(err)
    }

    // 2. Create service with Frame
    ctx, svc := frame.NewServiceWithContext(
        ctx,
        frame.WithConfig(&cfg),
        frame.WithRegisterServerOauth2Client(),
        frame.WithDatastore(),
    )
    defer svc.Stop(ctx)

    // 3. Get managers
    evtsMan := svc.EventsManager()
    qMan := svc.QueueManager()
    workMan := svc.WorkManager()
    dbMan := svc.DatastoreManager()

    // 4. Handle migrations
    if handleDatabaseMigration(ctx, dbMan, cfg) {
        return
    }

    // 5. Setup services
    repoService := repository.NewRepositoryService(cfg, dbMan)
    bamlClient := baml.NewClient(cfg)
    featureService := business.NewFeatureService(cfg, dbMan, repoService)

    // 6. Register publishers for outgoing events
    patchResultPublisher := frame.WithRegisterPublisher(
        cfg.QueuePatchResultName,
        cfg.QueuePatchResultURI,
    )

    // 7. Register subscribers for incoming queues
    featureRequestSubscriber := frame.WithRegisterSubscriber(
        cfg.QueueFeatureRequestName,
        cfg.QueueFeatureRequestURI,
        featurequeue.NewFeatureRequestQueueHandler(featureService, evtsMan),
    )

    // 8. Register event handlers
    eventHandlers := frame.WithRegisterEvents(
        events.NewRepositoryCheckoutEvent(cfg, repoService, evtsMan),
        events.NewSpecificationNormalizationEvent(cfg, bamlClient, evtsMan),
        events.NewImpactAnalysisEvent(cfg, bamlClient, repoService, evtsMan),
        events.NewPlanGenerationEvent(cfg, bamlClient, evtsMan),
        events.NewPatchGenerationEvent(cfg, repoService, bamlClient, qMan),
        events.NewPatchValidationEvent(cfg, repoService, evtsMan),
        events.NewTestGenerationEvent(cfg, bamlClient, evtsMan),
        events.NewReviewEvent(cfg, bamlClient, evtsMan),
        events.NewIterationEvent(cfg, bamlClient, evtsMan),
        events.NewCompletionEvent(cfg, qMan),
    )

    // 9. Setup Connect server
    connectHandler := setupConnectServer(ctx, svc, featureService)

    // 10. Initialize and run
    svc.Init(ctx,
        frame.WithHTTPHandler(connectHandler),
        patchResultPublisher,
        featureRequestSubscriber,
        eventHandlers,
    )

    if err := svc.Run(ctx, ""); err != nil {
        panic(err)
    }
}
```

### 4. Emitting Events

From business logic, emit events for async processing:

```go
package business

import (
    "context"

    "github.com/pitabwire/frame/events"

    featureevents "github.com/antinvestor/builder/apps/default/service/events"
)

type FeatureService struct {
    eventsMan events.Manager
    // ... other dependencies
}

func (s *FeatureService) StartFeatureExecution(ctx context.Context, spec *FeatureSpecification) error {
    // 1. Create execution record
    execution := &models.FeatureExecution{
        ID:            NewExecutionID(),
        Specification: spec,
        Status:        StatusInitializing,
    }

    if err := s.repo.Create(ctx, execution); err != nil {
        return fmt.Errorf("create execution: %w", err)
    }

    // 2. Emit event for async processing
    payload := &featureevents.RepositoryCheckoutPayload{
        ExecutionID:   execution.ID,
        RepositoryURL: spec.RepositoryURL,
        Branch:        spec.Branch,
    }

    return s.eventsMan.Emit(ctx, featureevents.RepositoryCheckoutEventName, payload)
}
```

### 5. Publishing to Queues

For publishing to external queues:

```go
package events

import (
    "context"

    "github.com/pitabwire/frame/queue"
    "github.com/pitabwire/util/data"
)

type CompletionEvent struct {
    cfg      *config.FeatureConfig
    queueMan queue.Manager
}

func (e *CompletionEvent) Execute(ctx context.Context, payload any) error {
    p := payload.(*CompletionPayload)

    // Publish result to external queue
    if e.cfg.QueueFeatureResultName != "" {
        result := data.JSONMap{
            "execution_id": p.ExecutionID.String(),
            "status":       p.Status,
            "result":       p.Result,
        }

        headers := map[string]string{
            "execution_id": p.ExecutionID.String(),
            "event_type":   "feature.completed",
        }

        if err := e.queueMan.Publish(ctx, e.cfg.QueueFeatureResultName, result, headers); err != nil {
            return fmt.Errorf("publish result: %w", err)
        }
    }

    return nil
}
```

## Configuration

Queue and event configuration via environment variables:

```go
package config

import "github.com/pitabwire/frame/config"

type FeatureConfig struct {
    config.ConfigurationDefault

    // Queue URIs (mem:// for in-memory, nats:// for NATS, kafka:// for Kafka)
    QueueFeatureRequestURI  string `envDefault:"mem://feature.requests"`
    QueueFeatureRequestName string `envDefault:"feature.requests"`

    QueueFeatureResultURI  string `envDefault:"mem://feature.results"`
    QueueFeatureResultName string `envDefault:"feature.results"`

    QueuePatchResultURI  string `envDefault:"mem://feature.patches"`
    QueuePatchResultName string `envDefault:"feature.patches"`

    // Retry configuration
    QueueRetryLevel1URI  string `envDefault:"mem://feature.retry.1"`
    QueueRetryLevel1Name string `envDefault:"feature.retry.1"`

    QueueRetryLevel2URI  string `envDefault:"mem://feature.retry.2"`
    QueueRetryLevel2Name string `envDefault:"feature.retry.2"`

    QueueDLQURI  string `envDefault:"mem://feature.dlq"`
    QueueDLQName string `envDefault:"feature.dlq"`

    // BAML configuration
    AnthropicAPIKey string `env:"ANTHROPIC_API_KEY"`
    OpenAIAPIKey    string `env:"OPENAI_API_KEY"`
    GoogleAPIKey    string `env:"GOOGLE_API_KEY"`

    // Repository configuration
    WorkspaceBasePath   string `envDefault:"/var/lib/feature-service/workspaces"`
    MaxConcurrentClones int    `envDefault:"10"`
    CloneTimeoutSeconds int    `envDefault:"300"`

    // Git authentication
    GitSSHKeyPath     string `env:"GIT_SSH_KEY_PATH"`
    GitHTTPSUsername  string `env:"GIT_HTTPS_USERNAME"`
    GitHTTPSPassword  string `env:"GIT_HTTPS_PASSWORD"`
}
```

## Agent Implementation Guide

### Step 1: Define Event Payload

```go
// internal/events/payloads.go
package events

type PatchGenerationPayload struct {
    ExecutionID    ExecutionID `json:"execution_id"`
    StepNumber     int         `json:"step_number"`
    PlanStepID     StepID      `json:"plan_step_id"`
    TargetFiles    []string    `json:"target_files"`
    Context        string      `json:"context"`
    PreviousResult *PatchResult `json:"previous_result,omitempty"`
}

type PatchGenerationResultPayload struct {
    ExecutionID ExecutionID `json:"execution_id"`
    StepNumber  int         `json:"step_number"`
    Success     bool        `json:"success"`
    Patches     []Patch     `json:"patches,omitempty"`
    Error       *ErrorInfo  `json:"error,omitempty"`
}
```

### Step 2: Implement Event Handler

```go
// internal/events/patch_generation.go
package events

const PatchGenerationEventName = "feature.patch.generation"

type PatchGenerationEvent struct {
    cfg         *config.FeatureConfig
    repoService repository.RepositoryService
    bamlClient  baml.Client
    eventsMan   events.Manager
}

func (e *PatchGenerationEvent) Name() string {
    return PatchGenerationEventName
}

func (e *PatchGenerationEvent) PayloadType() any {
    return &PatchGenerationPayload{}
}

func (e *PatchGenerationEvent) Execute(ctx context.Context, payload any) error {
    p := payload.(*PatchGenerationPayload)

    // 1. Get workspace
    workspace, err := e.repoService.GetWorkspace(ctx, p.ExecutionID)
    if err != nil {
        return e.handleError(ctx, p, err, "workspace_not_found")
    }

    // 2. Read target files
    fileContents, err := workspace.ReadFiles(ctx, p.TargetFiles)
    if err != nil {
        return e.handleError(ctx, p, err, "file_read_error")
    }

    // 3. Invoke BAML for patch generation
    patches, err := e.bamlClient.GeneratePatch(ctx, &baml.GeneratePatchRequest{
        FileContents: fileContents,
        Context:      p.Context,
    })
    if err != nil {
        return e.handleError(ctx, p, err, "llm_error")
    }

    // 4. Validate patches
    if err := e.validatePatches(ctx, patches); err != nil {
        return e.handleError(ctx, p, err, "validation_error")
    }

    // 5. Emit success event
    return e.eventsMan.Emit(ctx, PatchValidationEventName, &PatchValidationPayload{
        ExecutionID: p.ExecutionID,
        StepNumber:  p.StepNumber,
        Patches:     patches,
    })
}

func (e *PatchGenerationEvent) handleError(
    ctx context.Context,
    p *PatchGenerationPayload,
    err error,
    errorCode string,
) error {
    // Emit failure event for retry/DLQ handling
    return e.eventsMan.Emit(ctx, PatchGenerationFailedEventName, &PatchGenerationFailedPayload{
        ExecutionID:  p.ExecutionID,
        StepNumber:   p.StepNumber,
        ErrorCode:    errorCode,
        ErrorMessage: err.Error(),
    })
}
```

### Step 3: Register in Main

```go
// cmd/main.go
eventHandlers := frame.WithRegisterEvents(
    // ... other handlers
    events.NewPatchGenerationEvent(cfg, repoService, bamlClient, evtsMan),
)
```

## Event Flow

The feature building pipeline follows this event chain:

```
FeatureRequest (Queue)
    ↓
RepositoryCheckoutEvent
    ↓
SpecificationNormalizationEvent
    ↓
ImpactAnalysisEvent
    ↓
PlanGenerationEvent
    ↓
┌─────────────────────────────────────┐
│ For each plan step:                 │
│   PatchGenerationEvent              │
│       ↓                             │
│   PatchValidationEvent              │
│       ↓                             │
│   PatchApplicationEvent             │
│       ↓ (on failure)                │
│   IterationEvent → retry step       │
└─────────────────────────────────────┘
    ↓
TestGenerationEvent
    ↓
ReviewEvent
    ↓
CompletionEvent
    ↓
FeatureResult (Queue)
```

## Error Handling

### Retry Pattern

```go
type RetryableEventHandler struct {
    wrapped   EventHandler
    eventsMan events.Manager
    maxRetry  int
}

func (h *RetryableEventHandler) Execute(ctx context.Context, payload any) error {
    err := h.wrapped.Execute(ctx, payload)
    if err == nil {
        return nil
    }

    // Check if retryable
    if !isRetryable(err) {
        return h.emitDLQ(ctx, payload, err)
    }

    attempt := getAttempt(ctx)
    if attempt >= h.maxRetry {
        return h.emitDLQ(ctx, payload, err)
    }

    // Schedule retry with backoff
    return h.emitRetry(ctx, payload, attempt+1)
}
```

### Dead Letter Queue

```go
const DLQEventName = "feature.dlq"

type DLQPayload struct {
    OriginalEvent    string    `json:"original_event"`
    OriginalPayload  any       `json:"original_payload"`
    ErrorCode        string    `json:"error_code"`
    ErrorMessage     string    `json:"error_message"`
    Attempts         int       `json:"attempts"`
    FailedAt         time.Time `json:"failed_at"`
    ExecutionID      ExecutionID `json:"execution_id"`
}
```

## Testing

### Unit Testing Events

```go
func TestPatchGenerationEvent_Execute(t *testing.T) {
    ctx := context.Background()

    // Setup mocks
    mockRepo := mocks.NewMockRepositoryService(t)
    mockBAML := mocks.NewMockBAMLClient(t)
    mockEvents := mocks.NewMockEventsManager(t)

    // Setup expectations
    mockRepo.EXPECT().GetWorkspace(ctx, mock.Anything).Return(workspace, nil)
    mockBAML.EXPECT().GeneratePatch(ctx, mock.Anything).Return(patches, nil)
    mockEvents.EXPECT().Emit(ctx, events.PatchValidationEventName, mock.Anything).Return(nil)

    // Create handler
    handler := events.NewPatchGenerationEvent(cfg, mockRepo, mockBAML, mockEvents)

    // Execute
    payload := &events.PatchGenerationPayload{
        ExecutionID: events.NewExecutionID(),
        StepNumber:  1,
        TargetFiles: []string{"main.go"},
    }

    err := handler.Execute(ctx, payload)
    assert.NoError(t, err)
}
```

### Integration Testing with Frame

```go
func TestFeaturePipeline_Integration(t *testing.T) {
    ctx := context.Background()

    // Setup test service with in-memory queues
    cfg := &config.FeatureConfig{
        QueueFeatureRequestURI: "mem://test.requests",
        QueueFeatureResultURI:  "mem://test.results",
    }

    ctx, svc := frame.NewServiceWithContext(ctx,
        frame.WithConfig(cfg),
        frame.WithDatastore(), // Uses SQLite for tests
    )
    defer svc.Stop(ctx)

    evtsMan := svc.EventsManager()
    qMan := svc.QueueManager()

    // Register handlers
    svc.Init(ctx,
        frame.WithRegisterEvents(
            events.NewRepositoryCheckoutEvent(cfg, repoService, evtsMan),
            // ... other handlers
        ),
    )

    // Emit initial event
    err := evtsMan.Emit(ctx, events.RepositoryCheckoutEventName, &events.RepositoryCheckoutPayload{
        ExecutionID:   events.NewExecutionID(),
        RepositoryURL: "https://github.com/test/repo.git",
        Branch:        "main",
    })
    assert.NoError(t, err)

    // Wait for pipeline completion
    // ... assertions
}
```

## Best Practices

1. **Single Responsibility**: Each event handler should do one thing well
2. **Idempotency**: Handlers must be idempotent - processing the same event twice should have the same effect
3. **Error Classification**: Distinguish between retryable and non-retryable errors
4. **Observability**: Log structured data with execution_id for tracing
5. **Timeout Handling**: Set appropriate timeouts for BAML calls and git operations
6. **Resource Cleanup**: Always clean up workspaces and temporary files
7. **Transaction Boundaries**: Keep database transactions short and focused

## Managers Reference

Frame provides these managers accessed via `svc.<Manager>()`:

| Manager | Purpose | Access |
|---------|---------|--------|
| `EventsManager()` | Emit and handle internal events | `Emit(ctx, name, payload)` |
| `QueueManager()` | Publish to external queues | `Publish(ctx, name, payload, headers)` |
| `DatastoreManager()` | Database operations | `DB()`, `Transaction()` |
| `WorkManager()` | Async job processing | `Submit()`, `Pool()` |
| `SecurityManager()` | Auth and security | `GetToken()`, `ValidateToken()` |
| `CacheManager()` | Distributed caching | `Get()`, `Set()`, `Delete()` |

---

## Test Generation & Execution Agent

The Test Generation & Execution Agent is responsible for generating tests using BAML, executing them in isolated sandboxes, classifying failures, and coordinating iteration or rollback based on results.

### Agent Responsibilities

1. **Test Generation**: Generate tests for implementation files using BAML
2. **Test Execution**: Execute tests in isolated sandboxes with resource limits
3. **Failure Classification**: Analyze and classify test failures
4. **Baseline Management**: Capture pre-feature test state for comparison
5. **Rollback Signaling**: Request rollback when tests fail after feature implementation
6. **Iteration Coordination**: Request code iteration when tests need fixes

### Test Lifecycle

The test agent implements a two-phase verification model:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Test Lifecycle Flow                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. Pre-Feature Phase (Tests SHOULD FAIL)                               │
│     ┌──────────────────┐     ┌──────────────────┐                       │
│     │ Generate Tests   │────▶│ Execute Tests    │                       │
│     │ (via BAML)       │     │ (Pre-Feature)    │                       │
│     └──────────────────┘     └────────┬─────────┘                       │
│                                       │                                  │
│                                       ▼                                  │
│                              ┌──────────────────┐                       │
│                              │ Capture Baseline │                       │
│                              │ (Failed Tests)   │                       │
│                              └────────┬─────────┘                       │
│                                       │                                  │
│  ────────────────────────────────────┼────────────────────────────────  │
│                                       │                                  │
│  2. Feature Implementation           │                                  │
│     ┌──────────────────┐             │                                  │
│     │ Patch Generation │◀────────────┘                                  │
│     │ & Application    │                                                │
│     └────────┬─────────┘                                                │
│              │                                                           │
│  ────────────┼──────────────────────────────────────────────────────────│
│              │                                                           │
│  3. Post-Feature Phase (Tests SHOULD PASS)                              │
│              ▼                                                           │
│     ┌──────────────────┐     ┌──────────────────┐                       │
│     │ Execute Tests    │────▶│ Compare Results  │                       │
│     │ (Post-Feature)   │     │ vs Baseline      │                       │
│     └──────────────────┘     └────────┬─────────┘                       │
│                                       │                                  │
│                                       ▼                                  │
│                         ┌─────────────────────────┐                     │
│                         │ Determine Action        │                     │
│                         │ ┌─────┬────────┬──────┐│                     │
│                         │ │Pass │Iterate │Rollbk││                     │
│                         │ └──┬──┴───┬────┴──┬───┘│                     │
│                         └────┼──────┼───────┼────┘                     │
│                              │      │       │                           │
│                              ▼      ▼       ▼                           │
│                         Continue  Fix     Revert                        │
│                         Pipeline  Code    Changes                       │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Event Handlers

#### TestGenerationEvent

Generates tests for implementation files using BAML:

```go
const TestGenerationEventName = "feature.test.generation"

type TestGenerationEvent struct {
    cfg             *config.FeatureConfig
    bamlClient      TestGenerationBAMLClient
    workspaceReader WorkspaceReader
    fileWriter      TestFileWriter
    eventsMan       EventsEmitter
}

func (e *TestGenerationEvent) Execute(ctx context.Context, payload any) error {
    p := payload.(*baseevents.TestGenerationRequestedPayload)

    // 1. Read target implementation files
    fileContents, err := e.workspaceReader.ReadFiles(ctx, p.ExecutionID, targetPaths)

    // 2. Read existing tests for reference
    existingTestContents, _ := e.workspaceReader.ReadFiles(ctx, p.ExecutionID, p.ExistingTests)

    // 3. Get project structure for context
    projectStructure, err := e.workspaceReader.GetProjectStructure(ctx, p.ExecutionID)

    // 4. Invoke BAML for test generation
    bamlResp, err := e.bamlClient.GenerateTests(ctx, bamlReq)

    // 5. Write test files to workspace
    for _, test := range bamlResp.Tests {
        e.fileWriter.WriteFile(ctx, p.ExecutionID, test.FilePath, []byte(test.Content))
    }

    // 6. Emit success event
    return e.eventsMan.Emit(ctx, TestGenerationCompletedEventName, &TestGenerationCompletedPayload{...})
}
```

#### TestExecutionEvent

Executes tests in isolated sandboxes:

```go
const TestExecutionEventName = "feature.test.execution"

type TestExecutionEvent struct {
    cfg            *config.FeatureConfig
    sandboxExec    SandboxExecutor
    testRunner     TestRunner
    baselineStore  BaselineStore
    classifier     *FailureClassifier
    eventsMan      EventsEmitter
}

func (e *TestExecutionEvent) Execute(ctx context.Context, payload any) error {
    p := payload.(*baseevents.TestExecutionRequestedPayload)

    // 1. Execute tests in sandbox
    execReq := &SandboxExecutionRequest{
        ExecutionID: p.ExecutionID,
        Command:     runnerCmd,
        Args:        runnerArgs,
        Config:      p.SandboxConfig,
    }
    sandboxResult, err := e.sandboxExec.Execute(ctx, execReq)

    // 2. Parse test results
    result, err := e.testRunner.ParseResults(sandboxResult.Output, sandboxResult.ExitCode)

    // 3. Handle based on test phase
    return e.handleTestResult(ctx, p, result)
}
```

### Sandbox Execution Design

The sandbox executor provides isolated test execution with multiple backends:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                       Sandbox Execution Architecture                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌────────────────────────────────────────────────────────────────┐    │
│   │                     SandboxExecutor                             │    │
│   │  ┌─────────────────────────────────────────────────────────┐   │    │
│   │  │ execSem chan struct{} (concurrency limiter)             │   │    │
│   │  │ active  map[string]*sandboxInstance (tracking)          │   │    │
│   │  └─────────────────────────────────────────────────────────┘   │    │
│   └────────────────────────────────────────────────────────────────┘    │
│                              │                                           │
│           ┌──────────────────┼──────────────────┐                       │
│           ▼                  ▼                  ▼                        │
│   ┌───────────────┐  ┌───────────────┐  ┌───────────────┐              │
│   │    Docker     │  │   gVisor      │  │  Firecracker  │              │
│   │   Executor    │  │   Executor    │  │   Executor    │              │
│   │               │  │               │  │               │              │
│   │ • --rm        │  │ • --runtime   │  │ • microVM     │              │
│   │ • --memory    │  │   =runsc      │  │ • vsock       │              │
│   │ • --cpus      │  │ • User-space  │  │ • rootfs      │              │
│   │ • --network   │  │   kernel      │  │ • Jailer      │              │
│   │ • --read-only │  │               │  │               │              │
│   └───────────────┘  └───────────────┘  └───────────────┘              │
│                                                                          │
│   ┌───────────────┐                                                     │
│   │    Direct     │  (No sandbox - for trusted environments)           │
│   │   Executor    │                                                     │
│   │               │                                                     │
│   │ • Process     │                                                     │
│   │   groups      │                                                     │
│   │ • Resource    │                                                     │
│   │   monitoring  │                                                     │
│   └───────────────┘                                                     │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

#### Docker Execution Options

```go
func (e *SandboxExecutor) buildDockerArgs(req *SandboxExecutionRequest) []string {
    args := []string{"run", "--rm"}

    // Resource limits
    args = append(args, fmt.Sprintf("--memory=%dm", req.Config.MemoryLimitMB))
    args = append(args, fmt.Sprintf("--cpus=%f", req.Config.CPULimit))

    // Security options
    args = append(args,
        "--security-opt=no-new-privileges",
        "--cap-drop=ALL",
        "--read-only",
    )

    // Network isolation
    if !req.Config.NetworkEnabled {
        args = append(args, "--network=none")
    }

    // Workspace mount
    args = append(args, fmt.Sprintf("-v=%s:/workspace:rw", workspacePath))

    return args
}
```

### Test Runner Abstraction

The test runner abstraction supports multiple languages with framework detection:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                       Test Runner Architecture                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌────────────────────────────────────────────────────────────────┐    │
│   │                        MultiRunner                              │    │
│   │  ┌─────────────────────────────────────────────────────────┐   │    │
│   │  │ languageRunners map[string]LanguageRunner               │   │    │
│   │  │   "go"         → GoRunner                               │   │    │
│   │  │   "python"     → PythonRunner                           │   │    │
│   │  │   "javascript" → JavaScriptRunner                       │   │    │
│   │  │   "typescript" → TypeScriptRunner                       │   │    │
│   │  │   "java"       → JavaRunner                             │   │    │
│   │  │   "rust"       → RustRunner                             │   │    │
│   │  └─────────────────────────────────────────────────────────┘   │    │
│   └────────────────────────────────────────────────────────────────┘    │
│                              │                                           │
│                              ▼                                           │
│   ┌────────────────────────────────────────────────────────────────┐    │
│   │                    LanguageRunner Interface                     │    │
│   │                                                                 │    │
│   │  Detect(ctx, workspacePath) (*TestFrameworkInfo, bool)         │    │
│   │  BuildCommand(framework, files, filter, coverage) (cmd, args)  │    │
│   │  ParseResults(output, exitCode) (*TestResult, error)           │    │
│   │  ParseCoverage(output) (*CoverageResult, error)                │    │
│   └────────────────────────────────────────────────────────────────┘    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

#### Language-Specific Runners

| Language | Frameworks | Output Format |
|----------|------------|---------------|
| Go | `go test` | JSON (`-json` flag) |
| Python | pytest, unittest | JUnit XML, pytest JSON |
| JavaScript | Jest, Vitest, Mocha | Jest JSON, JUnit XML |
| TypeScript | Jest, Vitest | Jest JSON |
| Java | JUnit, TestNG | JUnit XML, Maven Surefire |
| Rust | `cargo test` | Cargo test output |

```go
type GoRunner struct {
    cfg *config.FeatureConfig
}

func (r *GoRunner) Detect(ctx context.Context, workspacePath string) (*TestFrameworkInfo, bool) {
    // Check for go.mod
    goModPath := filepath.Join(workspacePath, "go.mod")
    if _, err := os.Stat(goModPath); err == nil {
        return &TestFrameworkInfo{
            Name:       "go-test",
            Language:   "go",
            Version:    r.detectGoVersion(ctx, workspacePath),
            ConfigFile: "go.mod",
        }, true
    }
    return nil, false
}

func (r *GoRunner) BuildCommand(framework *TestFrameworkInfo, files []string, filter string, coverage bool) (string, []string) {
    args := []string{"test", "-json"}
    if coverage {
        args = append(args, "-coverprofile=coverage.out", "-covermode=atomic")
    }
    if filter != "" {
        args = append(args, "-run", filter)
    }
    args = append(args, files...)
    return "go", args
}
```

### Failure Classification

The failure classifier analyzes test output to determine appropriate actions:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      Failure Classification System                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Category              │ Retryable │ Action         │ Examples           │
│  ──────────────────────┼───────────┼────────────────┼──────────────────  │
│  compilation           │ No        │ Fix Code       │ Syntax errors      │
│  assertion             │ No        │ Fix Code       │ Test assertions    │
│  runtime               │ Sometimes │ Investigate    │ Panics, exceptions │
│  timeout               │ Yes       │ Retry/Extend   │ Slow tests         │
│  resource              │ Yes       │ Retry/Increase │ OOM, disk full     │
│  environment           │ Yes       │ Fix Setup      │ Missing deps       │
│  dependency            │ Sometimes │ Fix Imports    │ Missing packages   │
│  flaky                 │ Yes       │ Retry (3x)     │ Race conditions    │
│  regression            │ No        │ Rollback       │ Previously passing │
│  infrastructure        │ Yes       │ Retry          │ Network, DB issues │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

#### Classification Patterns

```go
type FailureClassifier struct {
    patterns []classificationPattern
}

type classificationPattern struct {
    category    baseevents.FailureCategory
    patterns    []*regexp.Regexp
    severity    baseevents.FailureSeverity
    retryable   bool
    maxRetries  int
    action      baseevents.FailureAction
}

// Example patterns for Go compilation errors
var compilationPatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)cannot find package`),
    regexp.MustCompile(`(?i)undefined:`),
    regexp.MustCompile(`(?i)syntax error`),
    regexp.MustCompile(`(?i)cannot use .* as .* in`),
    regexp.MustCompile(`(?i)missing return`),
    regexp.MustCompile(`(?i)declared but not used`),
}

func (c *FailureClassifier) Classify(result *baseevents.TestResult) *baseevents.TestFailureClassification {
    combinedOutput := c.combineTestOutput(result)

    // Try pattern matching
    for _, p := range c.patterns {
        for _, re := range p.patterns {
            if re.MatchString(combinedOutput) {
                return &baseevents.TestFailureClassification{
                    Category:          p.category,
                    Severity:          p.severity,
                    Retryable:         p.retryable,
                    SuggestedAction:   p.action,
                    RootCauseAnalysis: c.generateRootCauseAnalysis(result, p.category),
                }
            }
        }
    }

    // Default to assertion failure
    return &baseevents.TestFailureClassification{
        Category:        baseevents.FailureCategoryAssertion,
        Severity:        baseevents.FailureSeverityMedium,
        SuggestedAction: baseevents.FailureActionFix,
    }
}
```

### Rollback Signaling

When post-feature tests fail unexpectedly, the agent signals rollback:

```go
// Emitted when tests that passed pre-feature now fail post-feature
type TestRollbackRequestedPayload struct {
    ExecutionID    ExecutionID                 `json:"execution_id"`
    StepID         StepID                      `json:"step_id"`
    Reason         RollbackReason              `json:"reason"`
    FailedTests    []TestCaseResult            `json:"failed_tests,omitempty"`
    Classification *TestFailureClassification  `json:"classification,omitempty"`
    RollbackTo     *RollbackTarget             `json:"rollback_to,omitempty"`
    RequestedAt    time.Time                   `json:"requested_at"`
}

type RollbackReason string

const (
    RollbackReasonRegression       RollbackReason = "regression"        // Previously passing tests now fail
    RollbackReasonCriticalFailure  RollbackReason = "critical_failure"  // Critical test failures
    RollbackReasonMaxIterations    RollbackReason = "max_iterations"    // Exceeded retry limit
    RollbackReasonResourceExhausted RollbackReason = "resource_exhausted" // Out of resources
)
```

### Iteration Signaling

When tests need code fixes, the agent requests iteration:

```go
type TestIterationRequestedPayload struct {
    ExecutionID      ExecutionID                 `json:"execution_id"`
    StepID           StepID                      `json:"step_id"`
    FailedTests      []TestCaseResult            `json:"failed_tests"`
    Classification   *TestFailureClassification  `json:"classification"`
    IterationNumber  int                         `json:"iteration_number"`
    PreviousAttempts []IterationAttempt          `json:"previous_attempts,omitempty"`
    SuggestedFixes   []string                    `json:"suggested_fixes,omitempty"`
    RequestedAt      time.Time                   `json:"requested_at"`
}

type IterationAttempt struct {
    AttemptNumber int       `json:"attempt_number"`
    FailedTests   []string  `json:"failed_tests"`
    ErrorSummary  string    `json:"error_summary"`
    FixApplied    string    `json:"fix_applied,omitempty"`
    AttemptedAt   time.Time `json:"attempted_at"`
}
```

### Event Flow

```
TestGenerationRequested
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                       TestGenerationEvent                                │
│   • Read implementation files                                            │
│   • Invoke BAML for test generation                                      │
│   • Write tests to workspace                                             │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
                                ▼
                    TestGenerationCompleted
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│               TestExecutionEvent (Pre-Feature Phase)                     │
│   • Execute tests in sandbox                                             │
│   • Tests SHOULD FAIL (feature not implemented)                          │
│   • Capture baseline of failed tests                                     │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │ If all pass          │ If some fail          │ If error
        ▼                      ▼                       ▼
  TestAnalysis            TestBaseline           TestExecutionFailed
  Completed               Captured               (infrastructure)
  (unexpected)                │
        │                     ▼
        │           [Feature Implementation]
        │                     │
        │                     ▼
        │    ┌────────────────────────────────────────────────────────────┐
        │    │           TestExecutionEvent (Post-Feature Phase)          │
        │    │   • Execute tests in sandbox                               │
        │    │   • Tests SHOULD PASS (feature implemented)                │
        │    │   • Compare against baseline                               │
        │    └───────────────────────────┬────────────────────────────────┘
        │                                │
        │        ┌───────────────────────┼───────────────────────┐
        │        │ All pass             │ Some fail             │ Regression
        │        ▼                      ▼                       ▼
        │  TestAnalysis           TestIteration          TestRollback
        │  Completed              Requested              Requested
        │  (success)                   │                       │
        │        │                     ▼                       ▼
        │        │              [Code Iteration]         [Revert Changes]
        │        │                     │
        │        │                     ▼
        │        │              Re-execute Tests
        │        │                     │
        │        │         ┌───────────┴───────────┐
        │        │         │ Pass                  │ Fail (max retries)
        │        │         ▼                       ▼
        │        │   TestAnalysis            TestRollback
        │        │   Completed               Requested
        │        │        │
        └────────┴────────┴───────────────────────────────────────────────▶
                                                                      Next
                                                                      Stage
```

### Configuration

```go
type FeatureConfig struct {
    // ... existing fields ...

    // Sandbox configuration
    SandboxEnabled        bool   `envDefault:"true"`
    SandboxImage          string `envDefault:"golang:1.22-alpine"`
    MaxConcurrentExecutions int  `envDefault:"10"`

    // Test execution limits
    TestTimeoutSeconds    int    `envDefault:"300"`
    TestMemoryLimitMB     int    `envDefault:"1024"`
    TestCPULimit          float64 `envDefault:"2.0"`
    TestNetworkEnabled    bool   `envDefault:"false"`

    // Iteration limits
    MaxTestIterations     int    `envDefault:"3"`
    MaxRetryAttempts      int    `envDefault:"2"`

    // Coverage
    CoverageEnabled       bool   `envDefault:"true"`
    CoverageThreshold     float64 `envDefault:"70.0"`
}
```

### Service Registration

```go
// In main.go
func main() {
    // ... service setup ...

    // Create sandbox executor
    sandboxExec := sandbox.NewSandboxExecutor(cfg)

    // Create test runner
    testRunner := testrunner.NewMultiRunner(cfg)

    // Create failure classifier
    classifier := events.NewFailureClassifier()

    // Create baseline store
    baselineStore := store.NewBaselineStore(dbMan)

    // Register test event handlers
    eventHandlers := frame.WithRegisterEvents(
        // ... other handlers ...
        events.NewTestGenerationEvent(cfg, bamlClient, workspaceReader, fileWriter, evtsMan),
        events.NewTestExecutionEvent(cfg, sandboxExec, testRunner, baselineStore, classifier, evtsMan),
    )

    svc.Init(ctx, eventHandlers)
}
```

### Best Practices for Test Agent

1. **Isolation**: Always run tests in sandboxes for untrusted code
2. **Resource Limits**: Set appropriate memory/CPU/time limits
3. **Baseline Capture**: Always capture pre-feature test state
4. **Classification**: Classify failures before taking action
5. **Iteration Limits**: Set maximum iteration attempts to prevent infinite loops
6. **Observability**: Log test output and classifications for debugging
7. **Cleanup**: Clean up sandbox resources after execution

---

## Review & Control Agent

The Review & Control Agent is the **gatekeeper** at the end of the feature building pipeline. It is responsible for reviewing patches and tests, assessing risk, and making critical control decisions: iterate, abort, or mark complete.

### Agent Responsibilities

1. **Patch & Test Review**: Comprehensive review of all generated code
2. **Security Assessment**: Detect vulnerabilities, secrets, and security regressions
3. **Architecture Assessment**: Detect breaking changes, dependency violations, layering issues
4. **Risk Scoring**: Calculate composite risk scores across multiple dimensions
5. **Decision Making**: Make control decisions based on thresholds and rules
6. **Kill Switch Management**: Emergency stop mechanisms for critical situations

### Core Principle: Conservative by Default

The Review & Control Agent is intentionally conservative:

- **Zero tolerance** for critical security issues
- **Zero tolerance** for secrets in code
- **Zero tolerance** for breaking changes (unless explicitly allowed)
- **Low thresholds** for high-severity issues
- **Mandatory iteration** before any issues pass through

### Decision Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      Review & Control Decision Flow                      │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌──────────────┐                                                      │
│   │ Review       │                                                      │
│   │ Requested    │                                                      │
│   └──────┬───────┘                                                      │
│          │                                                               │
│          ▼                                                               │
│   ┌──────────────┐     ┌──────────────────┐                            │
│   │ Check Kill   │────▶│ ABORT            │  (Kill switch active)      │
│   │ Switch       │     │                  │                            │
│   └──────┬───────┘     └──────────────────┘                            │
│          │ (not active)                                                  │
│          ▼                                                               │
│   ┌──────────────────────────────────────────────────────────────────┐  │
│   │                    SECURITY ANALYSIS                              │  │
│   │  • SQL Injection       • Command Injection    • XSS               │  │
│   │  • Path Traversal      • SSRF                 • Open Redirect     │  │
│   │  • Hardcoded Creds     • Weak Crypto          • Insecure TLS      │  │
│   │  • Secrets Detection   • Security Regressions                     │  │
│   └──────────────────────────────────────────────────────────────────┘  │
│          │                                                               │
│          ▼                                                               │
│   ┌──────────────────────────────────────────────────────────────────┐  │
│   │                  ARCHITECTURE ANALYSIS                            │  │
│   │  • Breaking Changes       • Removed APIs      • Changed Sigs      │  │
│   │  • Dependency Violations  • Layering Issues   • Circular Deps     │  │
│   │  • Interface Changes      • Pattern Violations                    │  │
│   └──────────────────────────────────────────────────────────────────┘  │
│          │                                                               │
│          ▼                                                               │
│   ┌──────────────────────────────────────────────────────────────────┐  │
│   │                    RISK SCORING                                   │  │
│   │                                                                   │  │
│   │   Security Risk ────┐                                             │  │
│   │   Architecture Risk ─┼────▶ Overall Risk Score (0-100)            │  │
│   │   Quality Risk ──────┤                                            │  │
│   │   Test Risk ─────────┤                                            │  │
│   │   Regression Risk ───┘                                            │  │
│   └──────────────────────────────────────────────────────────────────┘  │
│          │                                                               │
│          ▼                                                               │
│   ┌──────────────────────────────────────────────────────────────────┐  │
│   │                  DECISION ENGINE                                  │  │
│   │                                                                   │  │
│   │   Rule 1: Security BLOCKED? ─────────────▶ ABORT                  │  │
│   │   Rule 2: Architecture BLOCKED? ─────────▶ ABORT                  │  │
│   │   Rule 3: Max iterations exceeded? ──────▶ ABORT                  │  │
│   │   Rule 4: Critical issues? ──────────────▶ ITERATE                │  │
│   │   Rule 5: High severity issues? ─────────▶ ITERATE                │  │
│   │   Rule 6: Breaking changes? ─────────────▶ MANUAL REVIEW          │  │
│   │   Rule 7: Security review required? ─────▶ MANUAL REVIEW          │  │
│   │   Rule 8: Tests failing? ────────────────▶ ITERATE                │  │
│   │   Rule 9: Risk > threshold? ─────────────▶ ITERATE                │  │
│   │   Rule 10: Final phase, all pass? ───────▶ MARK COMPLETE          │  │
│   │   Rule 11: No blockers, some warnings? ──▶ APPROVE WITH WARNINGS  │  │
│   │   Rule 12: All clear? ───────────────────▶ APPROVE                │  │
│   └──────────────────────────────────────────────────────────────────┘  │
│          │                                                               │
│          ▼                                                               │
│   ┌──────────────┬──────────────┬──────────────┬──────────────┐        │
│   │   APPROVE    │   ITERATE    │    ABORT     │   COMPLETE   │        │
│   │              │              │              │              │        │
│   │  Continue    │  Fix issues  │  Stop exec   │  Finalize    │        │
│   │  pipeline    │  retry       │  rollback    │  deliver     │        │
│   └──────────────┴──────────────┴──────────────┴──────────────┘        │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Security Analyzer

Detects security issues using pattern matching:

```go
type PatternSecurityAnalyzer struct {
    cfg      *config.FeatureConfig
    patterns []securityPattern
}

func (a *PatternSecurityAnalyzer) Analyze(ctx context.Context, req *SecurityAnalysisRequest) (*events.SecurityAssessment, error) {
    // 1. Analyze each file for insecure patterns
    // 2. Detect secrets (API keys, passwords, tokens)
    // 3. Check for security regressions
    // 4. Calculate security score
}
```

#### Security Patterns Detected

| Pattern Type | CWE | OWASP | Severity |
|-------------|-----|-------|----------|
| SQL Injection | CWE-89 | A03:2021 | Critical |
| Command Injection | CWE-78 | A03:2021 | Critical |
| XSS | CWE-79 | A03:2021 | High |
| Path Traversal | CWE-22 | A01:2021 | High |
| SSRF | CWE-918 | A10:2021 | High |
| Hardcoded Credentials | CWE-798 | A07:2021 | High |
| Weak Crypto | CWE-327 | A02:2021 | Medium |
| Insecure Random | CWE-330 | A02:2021 | Medium |
| Insecure Deserialization | CWE-502 | A08:2021 | Critical |
| Open Redirect | CWE-601 | A01:2021 | Medium |
| Insecure TLS | CWE-295 | A07:2021 | High |

#### Secrets Detection

```go
var secretPatterns = []struct {
    name    string
    pattern *regexp.Regexp
}{
    {"aws_key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
    {"github_token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{36,}`)},
    {"generic_api_key", regexp.MustCompile(`api[_-]?key.*['"][A-Za-z0-9\-_]{20,}['"]`)},
    {"private_key", regexp.MustCompile(`-----BEGIN.*PRIVATE KEY-----`)},
    {"jwt_token", regexp.MustCompile(`eyJ[A-Za-z0-9-_=]+\.eyJ[A-Za-z0-9-_=]+`)},
    // ... more patterns
}
```

### Architecture Analyzer

Detects architectural issues:

```go
type PatternArchitectureAnalyzer struct {
    cfg              *config.FeatureConfig
    defaultStructure *ProjectStructure
}

func (a *PatternArchitectureAnalyzer) Analyze(ctx context.Context, req *ArchitectureAnalysisRequest) (*events.ArchitectureAssessment, error) {
    // 1. Detect breaking changes (removed APIs, changed signatures)
    // 2. Check dependency violations
    // 3. Check layering violations
    // 4. Detect circular dependencies
    // 5. Detect interface changes
    // 6. Check for anti-patterns
}
```

#### Breaking Change Detection

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    Breaking Change Types                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Type                    │ Example                   │ Severity          │
│  ────────────────────────┼───────────────────────────┼─────────────────  │
│  removed_api             │ Deleted public function   │ Critical          │
│  changed_signature       │ Added required parameter  │ High              │
│  changed_behavior        │ Different return value    │ High              │
│  removed_field           │ Deleted struct field      │ High              │
│  changed_type            │ int → string              │ High              │
│  renamed_symbol          │ OldName → NewName         │ Medium            │
│  changed_default         │ Default value changed     │ Medium            │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Risk Scoring Model

The risk scoring model combines multiple dimensions:

```go
type RiskAssessment struct {
    OverallRiskScore      int  // 0-100 (weighted composite)
    SecurityRiskScore     int  // 0-100
    ArchitectureRiskScore int  // 0-100
    QualityRiskScore      int  // 0-100
    TestRiskScore         int  // 0-100
    RegressionRiskScore   int  // 0-100
}

// Weights for overall score calculation
weights := map[string]float64{
    "security":     0.35,  // Security has highest weight
    "architecture": 0.25,
    "quality":      0.15,
    "test":         0.15,
    "regression":   0.10,
}
```

### Review Thresholds

```go
type ReviewThresholds struct {
    MaxRiskScore             int     // Maximum acceptable risk (default: 50)
    MaxSecurityRiskScore     int     // Max security risk (default: 30)
    MaxArchitectureRiskScore int     // Max architecture risk (default: 40)
    MinTestCoverage          float64 // Minimum coverage (default: 70%)
    MaxCriticalIssues        int     // Critical issues allowed (default: 0)
    MaxHighIssues            int     // High issues allowed (default: 2)
    MaxBreakingChanges       int     // Breaking changes allowed (default: 0)
    MaxIterations            int     // Max iterations (default: 3)
}

// Conservative defaults
func DefaultReviewThresholds() ReviewThresholds {
    return ReviewThresholds{
        MaxRiskScore:             50,
        MaxSecurityRiskScore:     30,
        MaxArchitectureRiskScore: 40,
        MinTestCoverage:          70,
        MaxCriticalIssues:        0,   // Zero tolerance
        MaxHighIssues:            2,
        MaxBreakingChanges:       0,   // Zero tolerance
        MaxIterations:            3,
    }
}
```

### Kill Switch System

Emergency stop mechanisms for critical situations:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      Kill Switch Architecture                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌────────────────────────────────────────────────────────────────┐    │
│   │                    Kill Switch Service                          │    │
│   │                                                                 │    │
│   │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │    │
│   │  │   Global    │  │  Feature    │  │ Repository  │            │    │
│   │  │   Switch    │  │   Switch    │  │   Switch    │            │    │
│   │  │             │  │             │  │             │            │    │
│   │  │ Affects ALL │  │ Affects ONE │  │ Affects ALL │            │    │
│   │  │ executions  │  │ execution   │  │ in repo     │            │    │
│   │  └─────────────┘  └─────────────┘  └─────────────┘            │    │
│   └────────────────────────────────────────────────────────────────┘    │
│                              │                                           │
│                              ▼                                           │
│   ┌────────────────────────────────────────────────────────────────┐    │
│   │                    Auto-Trigger Evaluation                      │    │
│   │                                                                 │    │
│   │  • Error rate > 50%           → Activate global                │    │
│   │  • 5 consecutive failures     → Activate global                │    │
│   │  • Resource usage > 90%       → Activate global                │    │
│   │  • Security breach detected   → Activate feature               │    │
│   │  • Rate limit exceeded        → Activate global                │    │
│   └────────────────────────────────────────────────────────────────┘    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

#### Kill Switch Reasons

```go
const (
    KillSwitchReasonManual            = "manual"             // Human activated
    KillSwitchReasonSecurityBreach    = "security_breach"    // Security issue detected
    KillSwitchReasonResourceExhausted = "resource_exhausted" // OOM, disk full
    KillSwitchReasonAnomalyDetected   = "anomaly_detected"   // Unusual behavior
    KillSwitchReasonRateLimitExceeded = "rate_limit_exceeded"// Too many requests
    KillSwitchReasonSystemOverload    = "system_overload"    // System under stress
    KillSwitchReasonPolicyViolation   = "policy_violation"   // Policy rules violated
)
```

### Control Events

The Review Agent emits explicit control events:

```go
// Iteration requested - issues need fixing
type FeatureIterationRequestedPayload struct {
    ExecutionID            ExecutionID        `json:"execution_id"`
    ReviewID               string             `json:"review_id"`
    IterationNumber        int                `json:"iteration_number"`
    Issues                 []ReviewIssue      `json:"issues"`
    IterationGuidance      *IterationGuidance `json:"iteration_guidance"`
    MaxRemainingIterations int                `json:"max_remaining_iterations"`
}

// Abort requested - cannot continue
type FeatureAbortRequestedPayload struct {
    ExecutionID      ExecutionID   `json:"execution_id"`
    ReviewID         string        `json:"review_id"`
    AbortReason      AbortReason   `json:"abort_reason"`
    AbortDetails     string        `json:"abort_details"`
    BlockingIssues   []ReviewIssue `json:"blocking_issues,omitempty"`
    RollbackRequired bool          `json:"rollback_required"`
}

// Complete requested - all checks passed
type FeatureCompleteRequestedPayload struct {
    ExecutionID       ExecutionID     `json:"execution_id"`
    ReviewID          string          `json:"review_id"`
    FinalAssessment   *RiskAssessment `json:"final_assessment"`
    CompletionSummary string          `json:"completion_summary"`
    Warnings          []string        `json:"warnings,omitempty"`
}
```

### Event Flow

```
ComprehensiveReviewRequested
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    ComprehensiveReviewEvent                              │
│   • Check kill switch                                                    │
│   • Read file contents                                                   │
│   • Run security analysis                                                │
│   • Run architecture analysis                                            │
│   • Calculate risk scores                                                │
│   • Apply decision rules                                                 │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
                                ▼
                    ComprehensiveReviewCompleted
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
        ▼                       ▼                       ▼
  FeatureIteration        FeatureAbort          FeatureComplete
  Requested               Requested             Requested
        │                       │                       │
        ▼                       ▼                       ▼
  [Patch Generation]      [Rollback]            [Delivery]
  [Re-review]             [Cleanup]             [Push Branch]
```

### Service Registration

```go
// In main.go
func main() {
    // ... service setup ...

    // Create analyzers
    securityAnalyzer := review.NewPatternSecurityAnalyzer(cfg)
    architectureAnalyzer := review.NewPatternArchitectureAnalyzer(cfg)

    // Create kill switch service
    killSwitchService := review.NewDefaultKillSwitchService(cfg, evtsMan)

    // Create decision engine
    decisionEngine := review.NewConservativeDecisionEngine(cfg)

    // Create safety guard for other services to use
    safetyGuard := review.NewSafetyGuard(killSwitchService)

    // Register review event handler
    eventHandlers := frame.WithRegisterEvents(
        // ... other handlers ...
        review.NewComprehensiveReviewEvent(
            cfg,
            securityAnalyzer,
            architectureAnalyzer,
            decisionEngine,
            killSwitchService,
            workspaceReader,
            evtsMan,
        ),
    )

    svc.Init(ctx, eventHandlers)
}
```

### Configuration

```go
type FeatureConfig struct {
    // ... existing fields ...

    // Review thresholds
    ReviewThresholds ReviewThresholds `json:"review_thresholds"`

    // Kill switch configuration
    KillSwitchEnabled       bool    `envDefault:"true"`
    ErrorRateThreshold      float64 `envDefault:"0.5"`   // 50%
    MaxConsecutiveFailures  int     `envDefault:"5"`
    ResourceUsageThreshold  float64 `envDefault:"0.9"`   // 90%

    // Security configuration
    RequireSecurityApproval bool `envDefault:"true"`
    BlockOnSecrets          bool `envDefault:"true"`

    // Architecture configuration
    RequireArchitectApproval bool `envDefault:"false"`
    AllowBreakingChanges     bool `envDefault:"false"`
}
```

### Best Practices for Review Agent

1. **Be Conservative**: When in doubt, request iteration or manual review
2. **Zero Tolerance for Secrets**: Never allow secrets through
3. **Breaking Changes Need Approval**: Auto-block breaking changes
4. **Security First**: Security score has highest weight
5. **Clear Rationale**: Always explain decisions
6. **Iteration Limits**: Prevent infinite loops with max iterations
7. **Kill Switch Ready**: Always check kill switch before operations
8. **Audit Trail**: Log all decisions for compliance

### Integration with Other Agents

The Review & Control Agent sits at the end of the pipeline:

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│   Planning   │───▶│    Patch     │───▶│    Test      │───▶│   Review &   │
│    Agent     │    │    Agent     │    │    Agent     │    │   Control    │
└──────────────┘    └──────────────┘    └──────────────┘    └──────┬───────┘
                                                                    │
                           ┌────────────────────────────────────────┼────┐
                           │                                        ▼    │
                           │                              ┌──────────────┐│
                           │                              │   Decision   ││
                           │                              └──────┬───────┘│
                           │     ┌──────────────────────────────┼────────┘
                           │     │                              │
                           ▼     ▼                              ▼
                    ┌──────────────┐                    ┌──────────────┐
                    │   Iterate    │                    │   Complete   │
                    │ (back to     │                    │   (deliver)  │
                    │  Patch)      │                    │              │
                    └──────────────┘                    └──────────────┘
```
