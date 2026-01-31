# builder Domain Models

## Overview

This document defines the domain models used throughout builder. These models follow the patterns established in service-profile and service-notification, using GORM for ORM with PostgreSQL.

---

## Entity Relationship Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           DOMAIN MODEL RELATIONSHIPS                             │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌─────────────────┐                                                            │
│  │    Tenant       │                                                            │
│  │    (external)   │                                                            │
│  └────────┬────────┘                                                            │
│           │ 1                                                                    │
│           │                                                                      │
│           │ *                                                                    │
│  ┌────────▼────────┐         ┌─────────────────┐                               │
│  │                 │    *    │                 │                               │
│  │   Repository    │◄────────│ FeatureExecution│                               │
│  │                 │    1    │                 │                               │
│  └────────┬────────┘         └────────┬────────┘                               │
│           │                           │                                          │
│           │ 1                         │ 1                                        │
│           │                           │                                          │
│           │ 0..1                      │ *                                        │
│  ┌────────▼────────┐         ┌────────▼────────┐                               │
│  │                 │         │                 │                               │
│  │   Credential    │         │  ExecutionStep  │                               │
│  │   (encrypted)   │         │                 │                               │
│  └─────────────────┘         └────────┬────────┘                               │
│                                       │                                          │
│                                       │ 1                                        │
│                                       │                                          │
│                                       │ *                                        │
│                              ┌────────▼────────┐                               │
│                              │                 │                               │
│                              │  FileChange     │                               │
│                              │                 │                               │
│                              └─────────────────┘                               │
│                                                                                  │
│  ┌─────────────────┐         ┌─────────────────┐                               │
│  │                 │    *    │                 │                               │
│  │FeatureExecution │────────▶│ ExecutionEvent  │                               │
│  │                 │    1    │                 │                               │
│  └─────────────────┘         └─────────────────┘                               │
│                                                                                  │
│  ┌─────────────────┐         ┌─────────────────┐                               │
│  │                 │    1    │                 │                               │
│  │FeatureExecution │────────▶│   Artifact      │                               │
│  │                 │    *    │                 │                               │
│  └─────────────────┘         └─────────────────┘                               │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Core Models

### Repository

Represents a registered git repository.

```go
// models/repository.go
package models

import (
    "time"

    "gorm.io/gorm"
)

// Repository represents a registered git repository
type Repository struct {
    ID              string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    TenantID        string         `gorm:"type:uuid;not null;index:idx_repo_tenant" json:"tenant_id"`
    Name            string         `gorm:"type:varchar(255);not null;index:idx_repo_tenant_name,unique" json:"name"`
    URL             string         `gorm:"type:text;not null" json:"url"`
    DefaultBranch   string         `gorm:"type:varchar(255);not null;default:'main'" json:"default_branch"`
    Provider        GitProvider    `gorm:"type:smallint;not null;default:0" json:"provider"`
    CredentialID    *string        `gorm:"type:uuid" json:"credential_id,omitempty"`
    State           State          `gorm:"type:smallint;not null;default:1" json:"state"`
    Properties      Properties     `gorm:"type:jsonb;default:'{}'" json:"properties"`
    LastAccessedAt  *time.Time     `json:"last_accessed_at,omitempty"`
    CreatedAt       time.Time      `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt       time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
    DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

    // Relations
    Credential      *Credential       `gorm:"foreignKey:CredentialID" json:"-"`
    Features        []FeatureExecution `gorm:"foreignKey:RepositoryID" json:"-"`
}

// TableName returns the table name
func (Repository) TableName() string {
    return "repositories"
}

// GitProvider identifies the git hosting provider
type GitProvider int

const (
    GitProviderUnspecified GitProvider = iota
    GitProviderGitHub
    GitProviderGitLab
    GitProviderBitbucket
    GitProviderAzureDevOps
    GitProviderGitea
    GitProviderGeneric
)

// State represents entity lifecycle state
type State int

const (
    StateUnspecified State = iota
    StateActive
    StateInactive
    StateDeleted
)

// Properties holds arbitrary key-value metadata
type Properties map[string]interface{}
```

### Credential

Stores encrypted repository credentials.

```go
// models/credential.go
package models

import (
    "time"

    "gorm.io/gorm"
)

// Credential stores encrypted repository credentials
type Credential struct {
    ID                string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    TenantID          string         `gorm:"type:uuid;not null;index" json:"tenant_id"`
    RepositoryID      string         `gorm:"type:uuid;not null;uniqueIndex" json:"repository_id"`
    Type              CredentialType `gorm:"type:smallint;not null" json:"type"`
    EncryptedData     string         `gorm:"type:text;not null" json:"-"` // Encrypted credential data
    EncryptionKeyID   string         `gorm:"type:varchar(64);not null" json:"-"`
    LastValidatedAt   *time.Time     `json:"last_validated_at,omitempty"`
    CreatedAt         time.Time      `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt         time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
    DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`

    // Relations
    Repository        *Repository    `gorm:"foreignKey:RepositoryID" json:"-"`
}

// TableName returns the table name
func (Credential) TableName() string {
    return "credentials"
}

// CredentialType identifies the credential mechanism
type CredentialType int

const (
    CredentialTypeUnspecified CredentialType = iota
    CredentialTypeSSHKey
    CredentialTypeToken
    CredentialTypeOAuth
    CredentialTypeBasic
)

// CredentialData is the unencrypted credential structure
type CredentialData struct {
    SSHPrivateKey     string `json:"ssh_private_key,omitempty"`
    Token             string `json:"token,omitempty"`
    Username          string `json:"username,omitempty"`
    Password          string `json:"password,omitempty"`
    OAuthToken        string `json:"oauth_token,omitempty"`
    OAuthRefreshToken string `json:"oauth_refresh_token,omitempty"`
}
```

### FeatureExecution

Represents a feature execution instance.

```go
// models/feature_execution.go
package models

import (
    "time"

    "gorm.io/gorm"
)

// FeatureExecution represents a feature execution instance
type FeatureExecution struct {
    ID              string            `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    TenantID        string            `gorm:"type:uuid;not null;index:idx_feature_tenant" json:"tenant_id"`
    RepositoryID    string            `gorm:"type:uuid;not null;index:idx_feature_repo" json:"repository_id"`
    CorrelationID   string            `gorm:"type:varchar(255);index:idx_feature_correlation" json:"correlation_id,omitempty"`
    IdempotencyKey  string            `gorm:"type:varchar(255);uniqueIndex" json:"idempotency_key,omitempty"`

    // Specification
    Title           string            `gorm:"type:varchar(255);not null" json:"title"`
    Description     string            `gorm:"type:text;not null" json:"description"`
    TargetBranch    string            `gorm:"type:varchar(255);not null" json:"target_branch"`
    FeatureBranch   string            `gorm:"type:varchar(255)" json:"feature_branch,omitempty"`
    Constraints     FeatureConstraints `gorm:"type:jsonb;default:'{}'" json:"constraints"`

    // State
    State           FeatureState      `gorm:"type:smallint;not null;default:1;index:idx_feature_state" json:"state"`
    SequenceCursor  uint64            `gorm:"not null;default:0" json:"sequence_cursor"`

    // Execution data (populated during execution)
    Plan            *ExecutionPlan    `gorm:"type:jsonb" json:"plan,omitempty"`
    Progress        *ExecutionProgress `gorm:"type:jsonb" json:"progress,omitempty"`
    Error           *ExecutionError   `gorm:"type:jsonb" json:"error,omitempty"`
    Result          *DeliveryResult   `gorm:"type:jsonb" json:"result,omitempty"`

    // Workspace
    WorkspacePath   string            `gorm:"type:text" json:"-"`
    BaseCommit      string            `gorm:"type:varchar(64)" json:"base_commit,omitempty"`

    // Metadata
    Properties      Properties        `gorm:"type:jsonb;default:'{}'" json:"properties,omitempty"`
    RequestedBy     string            `gorm:"type:varchar(255)" json:"requested_by,omitempty"`

    // Timestamps
    CreatedAt       time.Time         `gorm:"autoCreateTime;index:idx_feature_created" json:"created_at"`
    UpdatedAt       time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
    CompletedAt     *time.Time        `json:"completed_at,omitempty"`
    DeletedAt       gorm.DeletedAt    `gorm:"index" json:"-"`

    // Relations
    Repository      *Repository       `gorm:"foreignKey:RepositoryID" json:"-"`
    Steps           []ExecutionStep   `gorm:"foreignKey:FeatureExecutionID" json:"-"`
    Artifacts       []Artifact        `gorm:"foreignKey:FeatureExecutionID" json:"-"`
}

// TableName returns the table name
func (FeatureExecution) TableName() string {
    return "feature_executions"
}

// FeatureState represents the execution state
type FeatureState int

const (
    FeatureStateUnspecified FeatureState = iota
    FeatureStatePending
    FeatureStateAnalyzing
    FeatureStatePlanning
    FeatureStateExecuting
    FeatureStateVerifying
    FeatureStateCompleted
    FeatureStateFailed
    FeatureStateCancelled
)

// String returns the string representation
func (s FeatureState) String() string {
    return [...]string{
        "unspecified",
        "pending",
        "analyzing",
        "planning",
        "executing",
        "verifying",
        "completed",
        "failed",
        "cancelled",
    }[s]
}

// FeatureConstraints define execution boundaries
type FeatureConstraints struct {
    MaxSteps        int      `json:"max_steps,omitempty"`
    TimeoutMinutes  int      `json:"timeout_minutes,omitempty"`
    AllowedPaths    []string `json:"allowed_paths,omitempty"`
    ForbiddenPaths  []string `json:"forbidden_paths,omitempty"`
    RequireTests    bool     `json:"require_tests,omitempty"`
    RequireBuild    bool     `json:"require_build,omitempty"`
}

// ExecutionPlan describes the implementation plan
type ExecutionPlan struct {
    Steps             []PlanStep `json:"steps"`
    EstimatedDuration int64      `json:"estimated_duration_seconds,omitempty"`
    AffectedFiles     []string   `json:"affected_files,omitempty"`
    Summary           string     `json:"summary,omitempty"`
}

// PlanStep describes a single implementation step
type PlanStep struct {
    Index        int      `json:"index"`
    Description  string   `json:"description"`
    Type         StepType `json:"type"`
    TargetFiles  []string `json:"target_files,omitempty"`
    Dependencies []int    `json:"dependencies,omitempty"`
}

// StepType categorizes implementation steps
type StepType string

const (
    StepTypeCreateFile    StepType = "create_file"
    StepTypeModifyFile    StepType = "modify_file"
    StepTypeDeleteFile    StepType = "delete_file"
    StepTypeRenameFile    StepType = "rename_file"
    StepTypeAddDependency StepType = "add_dependency"
    StepTypeRefactor      StepType = "refactor"
    StepTypeTest          StepType = "test"
    StepTypeDocumentation StepType = "documentation"
)

// ExecutionProgress tracks execution progress
type ExecutionProgress struct {
    TotalSteps              int     `json:"total_steps"`
    CompletedSteps          int     `json:"completed_steps"`
    CurrentStep             int     `json:"current_step"`
    CurrentStepDescription  string  `json:"current_step_description,omitempty"`
    PercentComplete         float64 `json:"percent_complete"`
}

// ExecutionError describes a failure
type ExecutionError struct {
    Code       ErrorCode              `json:"code"`
    Message    string                 `json:"message"`
    Details    map[string]interface{} `json:"details,omitempty"`
    Retryable  bool                   `json:"retryable"`
    FailedStep *int                   `json:"failed_step,omitempty"`
}

// ErrorCode categorizes errors
type ErrorCode string

const (
    ErrorCodeInvalidInput     ErrorCode = "invalid_input"
    ErrorCodeRepositoryAccess ErrorCode = "repository_access"
    ErrorCodeAnalysisFailed   ErrorCode = "analysis_failed"
    ErrorCodePlanningFailed   ErrorCode = "planning_failed"
    ErrorCodeGenerationFailed ErrorCode = "generation_failed"
    ErrorCodeBuildFailed      ErrorCode = "build_failed"
    ErrorCodeTestFailed       ErrorCode = "test_failed"
    ErrorCodePushFailed       ErrorCode = "push_failed"
    ErrorCodeTimeout          ErrorCode = "timeout"
    ErrorCodeCancelled        ErrorCode = "cancelled"
    ErrorCodeInternal         ErrorCode = "internal"
)

// DeliveryResult describes successful completion
type DeliveryResult struct {
    BranchName   string `json:"branch_name"`
    CommitSHA    string `json:"commit_sha"`
    CommitCount  int    `json:"commit_count"`
    FilesChanged int    `json:"files_changed"`
    LinesAdded   int    `json:"lines_added"`
    LinesRemoved int    `json:"lines_removed"`
    PatchURL     string `json:"patch_url,omitempty"`
    Summary      string `json:"summary,omitempty"`
}
```

### ExecutionStep

Represents a single step in the execution plan.

```go
// models/execution_step.go
package models

import (
    "time"

    "gorm.io/gorm"
)

// ExecutionStep represents a single step in the execution plan
type ExecutionStep struct {
    ID                 string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    FeatureExecutionID string         `gorm:"type:uuid;not null;index:idx_step_feature" json:"feature_execution_id"`
    StepIndex          int            `gorm:"not null;index:idx_step_feature_index" json:"step_index"`

    // Step definition
    Description        string         `gorm:"type:text;not null" json:"description"`
    StepType           StepType       `gorm:"type:varchar(32);not null" json:"step_type"`
    TargetFiles        []string       `gorm:"type:jsonb;default:'[]'" json:"target_files,omitempty"`

    // Execution state
    State              StepState      `gorm:"type:smallint;not null;default:0" json:"state"`
    AttemptCount       int            `gorm:"not null;default:0" json:"attempt_count"`

    // Results
    CommitSHA          string         `gorm:"type:varchar(64)" json:"commit_sha,omitempty"`
    LinesAdded         int            `gorm:"default:0" json:"lines_added"`
    LinesRemoved       int            `gorm:"default:0" json:"lines_removed"`

    // Error details
    Error              *StepError     `gorm:"type:jsonb" json:"error,omitempty"`

    // Timestamps
    StartedAt          *time.Time     `json:"started_at,omitempty"`
    CompletedAt        *time.Time     `json:"completed_at,omitempty"`
    CreatedAt          time.Time      `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt          time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
    DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`

    // Relations
    FeatureExecution   *FeatureExecution `gorm:"foreignKey:FeatureExecutionID" json:"-"`
    FileChanges        []FileChange      `gorm:"foreignKey:ExecutionStepID" json:"-"`
}

// TableName returns the table name
func (ExecutionStep) TableName() string {
    return "execution_steps"
}

// StepState represents step execution state
type StepState int

const (
    StepStatePending StepState = iota
    StepStateRunning
    StepStateCompleted
    StepStateFailed
    StepStateSkipped
)

// StepError contains step failure details
type StepError struct {
    Code      string                 `json:"code"`
    Message   string                 `json:"message"`
    Phase     string                 `json:"phase"` // generate, apply, validate
    Details   map[string]interface{} `json:"details,omitempty"`
    Retryable bool                   `json:"retryable"`
}
```

### FileChange

Tracks individual file modifications.

```go
// models/file_change.go
package models

import (
    "time"

    "gorm.io/gorm"
)

// FileChange tracks individual file modifications
type FileChange struct {
    ID               string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    FeatureExecutionID string       `gorm:"type:uuid;not null;index:idx_filechange_feature" json:"feature_execution_id"`
    ExecutionStepID  string         `gorm:"type:uuid;not null;index:idx_filechange_step" json:"execution_step_id"`

    // File details
    FilePath         string         `gorm:"type:text;not null" json:"file_path"`
    ChangeType       ChangeType     `gorm:"type:varchar(16);not null" json:"change_type"`

    // Change metrics
    PreviousHash     string         `gorm:"type:varchar(64)" json:"previous_hash,omitempty"`
    NewHash          string         `gorm:"type:varchar(64)" json:"new_hash,omitempty"`
    LinesAdded       int            `gorm:"default:0" json:"lines_added"`
    LinesRemoved     int            `gorm:"default:0" json:"lines_removed"`

    // Patch content (stored in artifact store for large patches)
    PatchContent     string         `gorm:"type:text" json:"-"`
    PatchArtifactID  string         `gorm:"type:uuid" json:"patch_artifact_id,omitempty"`

    // Timestamps
    CreatedAt        time.Time      `gorm:"autoCreateTime" json:"created_at"`
    DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

    // Relations
    FeatureExecution *FeatureExecution `gorm:"foreignKey:FeatureExecutionID" json:"-"`
    ExecutionStep    *ExecutionStep    `gorm:"foreignKey:ExecutionStepID" json:"-"`
}

// TableName returns the table name
func (FileChange) TableName() string {
    return "file_changes"
}

// ChangeType categorizes file changes
type ChangeType string

const (
    ChangeTypeCreated  ChangeType = "created"
    ChangeTypeModified ChangeType = "modified"
    ChangeTypeDeleted  ChangeType = "deleted"
    ChangeTypeRenamed  ChangeType = "renamed"
)
```

### Artifact

Stores references to execution artifacts.

```go
// models/artifact.go
package models

import (
    "time"

    "gorm.io/gorm"
)

// Artifact stores references to execution artifacts
type Artifact struct {
    ID                 string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    FeatureExecutionID string         `gorm:"type:uuid;not null;index:idx_artifact_feature" json:"feature_execution_id"`

    // Artifact details
    Name               string         `gorm:"type:varchar(255);not null" json:"name"`
    Type               ArtifactType   `gorm:"type:varchar(32);not null" json:"type"`
    ContentType        string         `gorm:"type:varchar(128);not null" json:"content_type"`

    // Storage
    StorageKey         string         `gorm:"type:text;not null" json:"-"` // S3 key
    Size               int64          `gorm:"not null;default:0" json:"size"`
    Checksum           string         `gorm:"type:varchar(64)" json:"checksum,omitempty"` // SHA256

    // Metadata
    Metadata           Properties     `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`

    // Timestamps
    CreatedAt          time.Time      `gorm:"autoCreateTime" json:"created_at"`
    ExpiresAt          *time.Time     `json:"expires_at,omitempty"`
    DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`

    // Relations
    FeatureExecution   *FeatureExecution `gorm:"foreignKey:FeatureExecutionID" json:"-"`
}

// TableName returns the table name
func (Artifact) TableName() string {
    return "artifacts"
}

// ArtifactType categorizes artifacts
type ArtifactType string

const (
    ArtifactTypePatch          ArtifactType = "patch"
    ArtifactTypeBuildLog       ArtifactType = "build_log"
    ArtifactTypeTestReport     ArtifactType = "test_report"
    ArtifactTypeCoverageReport ArtifactType = "coverage_report"
    ArtifactTypeAnalysisReport ArtifactType = "analysis_report"
    ArtifactTypePlan           ArtifactType = "plan"
    ArtifactTypeDependencyGraph ArtifactType = "dependency_graph"
)
```

### ExecutionEvent

Stores feature execution events for audit and replay.

```go
// models/execution_event.go
package models

import (
    "time"

    "gorm.io/gorm"
)

// ExecutionEvent stores feature execution events
type ExecutionEvent struct {
    ID                 string         `gorm:"type:uuid;primaryKey" json:"id"` // UUID v7
    FeatureExecutionID string         `gorm:"type:uuid;not null;index:idx_event_feature_seq" json:"feature_execution_id"`
    EventType          string         `gorm:"type:varchar(64);not null;index:idx_event_type" json:"event_type"`

    // Ordering
    SequenceNumber     uint64         `gorm:"not null;index:idx_event_feature_seq" json:"sequence_number"`
    Timestamp          time.Time      `gorm:"not null;index:idx_event_timestamp" json:"timestamp"`

    // Causality
    CausationID        string         `gorm:"type:uuid" json:"causation_id,omitempty"`
    CorrelationID      string         `gorm:"type:uuid;index:idx_event_correlation" json:"correlation_id,omitempty"`

    // Payload
    Payload            []byte         `gorm:"type:bytea;not null" json:"-"`
    PayloadSchema      string         `gorm:"type:varchar(32);not null" json:"payload_schema"`

    // Metadata
    ProducerID         string         `gorm:"type:varchar(255);not null" json:"producer_id"`
    Tags               Properties     `gorm:"type:jsonb;default:'{}'" json:"tags,omitempty"`

    // Timestamps
    CreatedAt          time.Time      `gorm:"autoCreateTime" json:"created_at"`

    // Relations
    FeatureExecution   *FeatureExecution `gorm:"foreignKey:FeatureExecutionID" json:"-"`
}

// TableName returns the table name
func (ExecutionEvent) TableName() string {
    return "execution_events"
}

// Composite index for efficient event queries
// CREATE INDEX idx_event_feature_seq ON execution_events (feature_execution_id, sequence_number);
```

### DistributedLock

Manages distributed locks for feature execution.

```go
// models/distributed_lock.go
package models

import (
    "time"
)

// DistributedLock manages distributed locks
type DistributedLock struct {
    LockKey    string    `gorm:"type:varchar(255);primaryKey" json:"lock_key"`
    OwnerID    string    `gorm:"type:varchar(255);not null" json:"owner_id"`
    FeatureID  string    `gorm:"type:uuid;not null;index" json:"feature_id"`
    AcquiredAt time.Time `gorm:"not null;default:now()" json:"acquired_at"`
    ExpiresAt  time.Time `gorm:"not null;index:idx_lock_expires" json:"expires_at"`
}

// TableName returns the table name
func (DistributedLock) TableName() string {
    return "distributed_locks"
}
```

---

## Database Migrations

### Migration Files

```sql
-- migrations/000001_create_repositories.up.sql
CREATE TABLE IF NOT EXISTS repositories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    default_branch VARCHAR(255) NOT NULL DEFAULT 'main',
    provider SMALLINT NOT NULL DEFAULT 0,
    credential_id UUID,
    state SMALLINT NOT NULL DEFAULT 1,
    properties JSONB DEFAULT '{}',
    last_accessed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_repo_tenant ON repositories(tenant_id);
CREATE UNIQUE INDEX idx_repo_tenant_name ON repositories(tenant_id, name) WHERE deleted_at IS NULL;
CREATE INDEX idx_repo_deleted ON repositories(deleted_at);

-- migrations/000002_create_credentials.up.sql
CREATE TABLE IF NOT EXISTS credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    repository_id UUID NOT NULL,
    type SMALLINT NOT NULL,
    encrypted_data TEXT NOT NULL,
    encryption_key_id VARCHAR(64) NOT NULL,
    last_validated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_credential_repo FOREIGN KEY (repository_id) REFERENCES repositories(id)
);

CREATE INDEX idx_cred_tenant ON credentials(tenant_id);
CREATE UNIQUE INDEX idx_cred_repo ON credentials(repository_id) WHERE deleted_at IS NULL;

-- migrations/000003_create_feature_executions.up.sql
CREATE TABLE IF NOT EXISTS feature_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    repository_id UUID NOT NULL,
    correlation_id VARCHAR(255),
    idempotency_key VARCHAR(255),
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    target_branch VARCHAR(255) NOT NULL,
    feature_branch VARCHAR(255),
    constraints JSONB DEFAULT '{}',
    state SMALLINT NOT NULL DEFAULT 1,
    sequence_cursor BIGINT NOT NULL DEFAULT 0,
    plan JSONB,
    progress JSONB,
    error JSONB,
    result JSONB,
    workspace_path TEXT,
    base_commit VARCHAR(64),
    properties JSONB DEFAULT '{}',
    requested_by VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_feature_repo FOREIGN KEY (repository_id) REFERENCES repositories(id)
);

CREATE INDEX idx_feature_tenant ON feature_executions(tenant_id);
CREATE INDEX idx_feature_repo ON feature_executions(repository_id);
CREATE INDEX idx_feature_state ON feature_executions(state);
CREATE INDEX idx_feature_correlation ON feature_executions(correlation_id);
CREATE INDEX idx_feature_created ON feature_executions(created_at DESC);
CREATE UNIQUE INDEX idx_feature_idempotency ON feature_executions(idempotency_key) WHERE idempotency_key IS NOT NULL AND deleted_at IS NULL;

-- migrations/000004_create_execution_steps.up.sql
CREATE TABLE IF NOT EXISTS execution_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    feature_execution_id UUID NOT NULL,
    step_index INT NOT NULL,
    description TEXT NOT NULL,
    step_type VARCHAR(32) NOT NULL,
    target_files JSONB DEFAULT '[]',
    state SMALLINT NOT NULL DEFAULT 0,
    attempt_count INT NOT NULL DEFAULT 0,
    commit_sha VARCHAR(64),
    lines_added INT DEFAULT 0,
    lines_removed INT DEFAULT 0,
    error JSONB,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_step_feature FOREIGN KEY (feature_execution_id) REFERENCES feature_executions(id) ON DELETE CASCADE
);

CREATE INDEX idx_step_feature ON execution_steps(feature_execution_id);
CREATE UNIQUE INDEX idx_step_feature_index ON execution_steps(feature_execution_id, step_index) WHERE deleted_at IS NULL;

-- migrations/000005_create_file_changes.up.sql
CREATE TABLE IF NOT EXISTS file_changes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    feature_execution_id UUID NOT NULL,
    execution_step_id UUID NOT NULL,
    file_path TEXT NOT NULL,
    change_type VARCHAR(16) NOT NULL,
    previous_hash VARCHAR(64),
    new_hash VARCHAR(64),
    lines_added INT DEFAULT 0,
    lines_removed INT DEFAULT 0,
    patch_content TEXT,
    patch_artifact_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_change_feature FOREIGN KEY (feature_execution_id) REFERENCES feature_executions(id) ON DELETE CASCADE,
    CONSTRAINT fk_change_step FOREIGN KEY (execution_step_id) REFERENCES execution_steps(id) ON DELETE CASCADE
);

CREATE INDEX idx_filechange_feature ON file_changes(feature_execution_id);
CREATE INDEX idx_filechange_step ON file_changes(execution_step_id);

-- migrations/000006_create_artifacts.up.sql
CREATE TABLE IF NOT EXISTS artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    feature_execution_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(32) NOT NULL,
    content_type VARCHAR(128) NOT NULL,
    storage_key TEXT NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    checksum VARCHAR(64),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_artifact_feature FOREIGN KEY (feature_execution_id) REFERENCES feature_executions(id) ON DELETE CASCADE
);

CREATE INDEX idx_artifact_feature ON artifacts(feature_execution_id);
CREATE INDEX idx_artifact_type ON artifacts(type);

-- migrations/000007_create_execution_events.up.sql
CREATE TABLE IF NOT EXISTS execution_events (
    id UUID PRIMARY KEY,
    feature_execution_id UUID NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    sequence_number BIGINT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    causation_id UUID,
    correlation_id UUID,
    payload BYTEA NOT NULL,
    payload_schema VARCHAR(32) NOT NULL,
    producer_id VARCHAR(255) NOT NULL,
    tags JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_event_feature FOREIGN KEY (feature_execution_id) REFERENCES feature_executions(id) ON DELETE CASCADE
);

CREATE INDEX idx_event_feature_seq ON execution_events(feature_execution_id, sequence_number);
CREATE INDEX idx_event_type ON execution_events(event_type);
CREATE INDEX idx_event_timestamp ON execution_events(timestamp DESC);
CREATE INDEX idx_event_correlation ON execution_events(correlation_id);

-- migrations/000008_create_distributed_locks.up.sql
CREATE TABLE IF NOT EXISTS distributed_locks (
    lock_key VARCHAR(255) PRIMARY KEY,
    owner_id VARCHAR(255) NOT NULL,
    feature_id UUID NOT NULL,
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT fk_lock_feature FOREIGN KEY (feature_id) REFERENCES feature_executions(id) ON DELETE CASCADE
);

CREATE INDEX idx_lock_expires ON distributed_locks(expires_at);
CREATE INDEX idx_lock_feature ON distributed_locks(feature_id);
```

---

## Repository Interfaces

Following the service-profile pattern:

```go
// repository/interfaces.go
package repository

import (
    "context"

    "github.com/antinvestor/github.com/antinvestor/builder/apps/default/service/models"
    "github.com/pitabwire/frame/datastore"
)

// RepositoryRepository manages repository entities
type RepositoryRepository interface {
    datastore.BaseRepository[*models.Repository]
    GetByName(ctx context.Context, tenantID, name string) (*models.Repository, error)
    ListByTenant(ctx context.Context, tenantID string) ([]*models.Repository, error)
}

// CredentialRepository manages credential entities
type CredentialRepository interface {
    datastore.BaseRepository[*models.Credential]
    GetByRepositoryID(ctx context.Context, repoID string) (*models.Credential, error)
}

// FeatureExecutionRepository manages feature execution entities
type FeatureExecutionRepository interface {
    datastore.BaseRepository[*models.FeatureExecution]
    GetByCorrelationID(ctx context.Context, tenantID, correlationID string) (*models.FeatureExecution, error)
    GetByIdempotencyKey(ctx context.Context, key string) (*models.FeatureExecution, error)
    Search(ctx context.Context, query *FeatureSearchQuery) ([]*models.FeatureExecution, error)
    UpdateState(ctx context.Context, id string, state models.FeatureState, sequenceCursor uint64) error
}

// ExecutionStepRepository manages execution step entities
type ExecutionStepRepository interface {
    datastore.BaseRepository[*models.ExecutionStep]
    GetByFeatureID(ctx context.Context, featureID string) ([]*models.ExecutionStep, error)
    UpdateState(ctx context.Context, id string, state models.StepState) error
}

// ArtifactRepository manages artifact entities
type ArtifactRepository interface {
    datastore.BaseRepository[*models.Artifact]
    GetByFeatureID(ctx context.Context, featureID string) ([]*models.Artifact, error)
}

// ExecutionEventRepository manages execution event entities
type ExecutionEventRepository interface {
    Store(ctx context.Context, event *models.ExecutionEvent) error
    GetByFeatureID(ctx context.Context, featureID string, afterSeq uint64) ([]*models.ExecutionEvent, error)
    GetLatestSequence(ctx context.Context, featureID string) (uint64, error)
}

// LockRepository manages distributed locks
type LockRepository interface {
    Acquire(ctx context.Context, lockKey, ownerID, featureID string, ttl time.Duration) error
    Release(ctx context.Context, lockKey, ownerID string) error
    Refresh(ctx context.Context, lockKey, ownerID string, ttl time.Duration) error
    IsHeld(ctx context.Context, lockKey, ownerID string) (bool, error)
}

// FeatureSearchQuery defines search parameters
type FeatureSearchQuery struct {
    TenantID     string
    RepositoryID string
    States       []models.FeatureState
    CreatedAfter *time.Time
    CreatedBefore *time.Time
    Query        string
    Limit        int
    Offset       int
}
```

---

## Model Validation

```go
// models/validation.go
package models

import (
    "errors"
    "net/url"
    "strings"
)

// Validate validates a Repository
func (r *Repository) Validate() error {
    if r.TenantID == "" {
        return errors.New("tenant_id is required")
    }
    if r.Name == "" {
        return errors.New("name is required")
    }
    if len(r.Name) > 255 {
        return errors.New("name must be 255 characters or less")
    }
    if r.URL == "" {
        return errors.New("url is required")
    }
    if _, err := url.Parse(r.URL); err != nil {
        return errors.New("url must be valid")
    }
    return nil
}

// Validate validates a FeatureExecution
func (f *FeatureExecution) Validate() error {
    if f.TenantID == "" {
        return errors.New("tenant_id is required")
    }
    if f.RepositoryID == "" {
        return errors.New("repository_id is required")
    }
    if f.Title == "" {
        return errors.New("title is required")
    }
    if len(f.Title) > 255 {
        return errors.New("title must be 255 characters or less")
    }
    if f.Description == "" {
        return errors.New("description is required")
    }
    if len(f.Description) < 10 {
        return errors.New("description must be at least 10 characters")
    }
    if f.TargetBranch == "" {
        return errors.New("target_branch is required")
    }
    return nil
}

// Validate validates FeatureConstraints
func (c *FeatureConstraints) Validate() error {
    if c.MaxSteps < 0 || c.MaxSteps > 50 {
        return errors.New("max_steps must be between 0 and 50")
    }
    if c.TimeoutMinutes < 0 || c.TimeoutMinutes > 120 {
        return errors.New("timeout_minutes must be between 0 and 120")
    }
    return nil
}
```

This completes the domain models documentation. The models follow the same patterns as service-profile and service-notification, using GORM with PostgreSQL and JSONB for flexible schemas.
