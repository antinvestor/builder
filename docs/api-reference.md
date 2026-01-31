# builder API Reference

## Overview

builder exposes its API via Connect-RPC (gRPC-compatible with HTTP/JSON support). All endpoints require authentication via JWT Bearer tokens.

**Base URL:** `https://feature.api.example.com`

**Authentication:** OAuth2 Bearer Token
```
Authorization: Bearer <token>
```

---

## Protocol Buffer Definitions

### Common Types

```protobuf
// feature/common/v1/common.proto

syntax = "proto3";

package feature.common.v1;

import "google/protobuf/timestamp.proto";
import "buf/validate/validate.proto";

// Pagination for list operations
message Pagination {
    int32 page_size = 1 [(buf.validate.field).int32 = {gte: 1, lte: 100}];
    string page_token = 2;
}

// PaginationResult contains pagination metadata
message PaginationResult {
    string next_page_token = 1;
    int32 total_count = 2;
}

// ObjectReference references an object by type and ID
message ObjectReference {
    string object_type = 1 [(buf.validate.field).string.min_len = 1];
    string object_id = 2 [(buf.validate.field).string.uuid = true];
}

// State represents entity lifecycle state
enum State {
    STATE_UNSPECIFIED = 0;
    STATE_ACTIVE = 1;
    STATE_INACTIVE = 2;
    STATE_DELETED = 3;
}

// Properties holds arbitrary key-value metadata
message Properties {
    map<string, string> values = 1;
}
```

---

## Feature Service

### Service Definition

```protobuf
// feature/feature/v1/feature.proto

syntax = "proto3";

package feature.feature.v1;

import "google/protobuf/timestamp.proto";
import "buf/validate/validate.proto";
import "feature/common/v1/common.proto";

option go_package = "github.com/antinvestor/apis/go/feature/feature/v1;featurev1";

// FeatureService manages feature execution lifecycle
service FeatureService {
    // Create submits a new feature for execution
    rpc Create(CreateRequest) returns (CreateResponse) {
        option idempotency_level = IDEMPOTENT;
    }

    // Get retrieves a feature execution by ID
    rpc Get(GetRequest) returns (GetResponse) {
        option idempotency_level = NO_SIDE_EFFECTS;
    }

    // GetByCorrelation retrieves feature by correlation ID
    rpc GetByCorrelation(GetByCorrelationRequest) returns (GetByCorrelationResponse) {
        option idempotency_level = NO_SIDE_EFFECTS;
    }

    // Search searches feature executions with filters
    rpc Search(SearchRequest) returns (stream SearchResponse) {
        option idempotency_level = NO_SIDE_EFFECTS;
    }

    // Cancel cancels an in-progress feature execution
    rpc Cancel(CancelRequest) returns (CancelResponse) {
        option idempotency_level = IDEMPOTENT;
    }

    // Retry retries a failed feature execution
    rpc Retry(RetryRequest) returns (RetryResponse) {
        option idempotency_level = IDEMPOTENT;
    }

    // ListEvents lists events for a feature execution
    rpc ListEvents(ListEventsRequest) returns (stream ListEventsResponse) {
        option idempotency_level = NO_SIDE_EFFECTS;
    }

    // GetArtifacts retrieves artifacts for a feature execution
    rpc GetArtifacts(GetArtifactsRequest) returns (GetArtifactsResponse) {
        option idempotency_level = NO_SIDE_EFFECTS;
    }
}
```

### Messages

```protobuf
// FeatureSpec defines the feature to be built
message FeatureSpec {
    // Title is a short description of the feature
    string title = 1 [(buf.validate.field).string = {min_len: 1, max_len: 255}];

    // Description is the detailed feature specification
    string description = 2 [(buf.validate.field).string = {min_len: 10, max_len: 10000}];

    // RepositoryID is the target repository
    string repository_id = 3 [(buf.validate.field).string.uuid = true];

    // TargetBranch is the branch to build upon (default: repository default branch)
    string target_branch = 4;

    // FeatureBranch is the name for the feature branch (auto-generated if empty)
    string feature_branch = 5;

    // Constraints define execution constraints
    FeatureConstraints constraints = 6;

    // Properties holds additional metadata
    feature.common.v1.Properties properties = 7;
}

// FeatureConstraints define execution boundaries
message FeatureConstraints {
    // MaxSteps limits the number of implementation steps
    int32 max_steps = 1 [(buf.validate.field).int32 = {gte: 1, lte: 50}];

    // TimeoutMinutes limits total execution time
    int32 timeout_minutes = 2 [(buf.validate.field).int32 = {gte: 5, lte: 120}];

    // AllowedPaths restricts modifications to specific paths
    repeated string allowed_paths = 3;

    // ForbiddenPaths prevents modifications to specific paths
    repeated string forbidden_paths = 4;

    // RequireTests requires test execution to pass
    bool require_tests = 5;

    // RequireBuild requires build to succeed
    bool require_build = 6;
}

// FeatureExecution represents a feature execution instance
message FeatureExecution {
    // ID is the unique execution identifier
    string id = 1;

    // CorrelationID is the client-provided correlation ID
    string correlation_id = 2;

    // RepositoryID is the target repository
    string repository_id = 3;

    // Spec is the original feature specification
    FeatureSpec spec = 4;

    // State is the current execution state
    FeatureState state = 5;

    // Plan is the generated implementation plan (populated after PLANNING)
    ExecutionPlan plan = 6;

    // Progress shows execution progress
    ExecutionProgress progress = 7;

    // Error contains error details if failed
    ExecutionError error = 8;

    // Result contains delivery details if completed
    DeliveryResult result = 9;

    // CreatedAt is when the feature was submitted
    google.protobuf.Timestamp created_at = 10;

    // UpdatedAt is when the feature was last updated
    google.protobuf.Timestamp updated_at = 11;

    // CompletedAt is when the feature completed (success or failure)
    google.protobuf.Timestamp completed_at = 12;
}

// FeatureState represents the execution state
enum FeatureState {
    FEATURE_STATE_UNSPECIFIED = 0;
    FEATURE_STATE_PENDING = 1;
    FEATURE_STATE_ANALYZING = 2;
    FEATURE_STATE_PLANNING = 3;
    FEATURE_STATE_EXECUTING = 4;
    FEATURE_STATE_VERIFYING = 5;
    FEATURE_STATE_COMPLETED = 6;
    FEATURE_STATE_FAILED = 7;
    FEATURE_STATE_CANCELLED = 8;
}

// ExecutionPlan describes the implementation plan
message ExecutionPlan {
    // Steps are the ordered implementation steps
    repeated PlanStep steps = 1;

    // EstimatedDuration is the estimated execution time
    google.protobuf.Duration estimated_duration = 2;

    // AffectedFiles lists files that will be modified
    repeated string affected_files = 3;

    // Summary is a human-readable plan summary
    string summary = 4;
}

// PlanStep describes a single implementation step
message PlanStep {
    // Index is the step order (0-based)
    int32 index = 1;

    // Description describes what this step does
    string description = 2;

    // Type categorizes the step
    StepType type = 3;

    // TargetFiles lists files this step will modify
    repeated string target_files = 4;

    // Dependencies lists step indices this step depends on
    repeated int32 dependencies = 5;
}

// StepType categorizes implementation steps
enum StepType {
    STEP_TYPE_UNSPECIFIED = 0;
    STEP_TYPE_CREATE_FILE = 1;
    STEP_TYPE_MODIFY_FILE = 2;
    STEP_TYPE_DELETE_FILE = 3;
    STEP_TYPE_RENAME_FILE = 4;
    STEP_TYPE_ADD_DEPENDENCY = 5;
    STEP_TYPE_REFACTOR = 6;
    STEP_TYPE_TEST = 7;
    STEP_TYPE_DOCUMENTATION = 8;
}

// ExecutionProgress tracks execution progress
message ExecutionProgress {
    // TotalSteps is the total number of steps
    int32 total_steps = 1;

    // CompletedSteps is the number of completed steps
    int32 completed_steps = 2;

    // CurrentStep is the currently executing step index
    int32 current_step = 3;

    // CurrentStepDescription describes the current activity
    string current_step_description = 4;

    // PercentComplete is the overall progress percentage
    float percent_complete = 5;
}

// ExecutionError describes a failure
message ExecutionError {
    // Code is the error category
    ErrorCode code = 1;

    // Message is a human-readable error message
    string message = 2;

    // Details contains structured error details
    google.protobuf.Struct details = 3;

    // Retryable indicates if the error is retryable
    bool retryable = 4;

    // FailedStep is the step that failed (if applicable)
    int32 failed_step = 5;
}

// ErrorCode categorizes errors
enum ErrorCode {
    ERROR_CODE_UNSPECIFIED = 0;
    ERROR_CODE_INVALID_INPUT = 1;
    ERROR_CODE_REPOSITORY_ACCESS = 2;
    ERROR_CODE_ANALYSIS_FAILED = 3;
    ERROR_CODE_PLANNING_FAILED = 4;
    ERROR_CODE_GENERATION_FAILED = 5;
    ERROR_CODE_BUILD_FAILED = 6;
    ERROR_CODE_TEST_FAILED = 7;
    ERROR_CODE_PUSH_FAILED = 8;
    ERROR_CODE_TIMEOUT = 9;
    ERROR_CODE_CANCELLED = 10;
    ERROR_CODE_INTERNAL = 11;
}

// DeliveryResult describes successful completion
message DeliveryResult {
    // BranchName is the created feature branch
    string branch_name = 1;

    // CommitSHA is the final commit SHA
    string commit_sha = 2;

    // CommitCount is the number of commits created
    int32 commit_count = 3;

    // FilesChanged is the number of files modified
    int32 files_changed = 4;

    // LinesAdded is the number of lines added
    int32 lines_added = 5;

    // LinesRemoved is the number of lines removed
    int32 lines_removed = 6;

    // PatchURL is URL to download the patch
    string patch_url = 7;

    // Summary is a human-readable completion summary
    string summary = 8;
}

// CreateRequest creates a new feature execution
message CreateRequest {
    // Spec is the feature specification
    FeatureSpec spec = 1 [(buf.validate.field).required = true];

    // CorrelationID is an optional client-provided correlation ID
    string correlation_id = 2;

    // IdempotencyKey prevents duplicate submissions
    string idempotency_key = 3;
}

// CreateResponse returns the created feature execution
message CreateResponse {
    // Execution is the created feature execution
    FeatureExecution execution = 1;
}

// GetRequest retrieves a feature by ID
message GetRequest {
    // ID is the feature execution ID
    string id = 1 [(buf.validate.field).string.uuid = true];
}

// GetResponse returns the feature execution
message GetResponse {
    // Execution is the feature execution
    FeatureExecution execution = 1;
}

// GetByCorrelationRequest retrieves a feature by correlation ID
message GetByCorrelationRequest {
    // CorrelationID is the client-provided correlation ID
    string correlation_id = 1 [(buf.validate.field).string.min_len = 1];
}

// GetByCorrelationResponse returns the feature execution
message GetByCorrelationResponse {
    // Execution is the feature execution
    FeatureExecution execution = 1;
}

// SearchRequest searches feature executions
message SearchRequest {
    // RepositoryID filters by repository
    string repository_id = 1;

    // States filters by execution states
    repeated FeatureState states = 2;

    // CreatedAfter filters by creation time
    google.protobuf.Timestamp created_after = 3;

    // CreatedBefore filters by creation time
    google.protobuf.Timestamp created_before = 4;

    // Query is a text search query
    string query = 5;

    // Pagination controls result pagination
    feature.common.v1.Pagination pagination = 6;
}

// SearchResponse streams feature executions
message SearchResponse {
    // Execution is a matching feature execution
    FeatureExecution execution = 1;
}

// CancelRequest cancels a feature execution
message CancelRequest {
    // ID is the feature execution ID
    string id = 1 [(buf.validate.field).string.uuid = true];

    // Reason is the cancellation reason
    string reason = 2;
}

// CancelResponse confirms cancellation
message CancelResponse {
    // Execution is the cancelled feature execution
    FeatureExecution execution = 1;
}

// RetryRequest retries a failed feature execution
message RetryRequest {
    // ID is the feature execution ID to retry
    string id = 1 [(buf.validate.field).string.uuid = true];

    // FromStep optionally restarts from a specific step
    int32 from_step = 2;
}

// RetryResponse returns the retried execution
message RetryResponse {
    // Execution is the new feature execution
    FeatureExecution execution = 1;
}

// ListEventsRequest lists events for a feature
message ListEventsRequest {
    // FeatureExecutionID is the feature execution ID
    string feature_execution_id = 1 [(buf.validate.field).string.uuid = true];

    // AfterSequence filters events after this sequence number
    uint64 after_sequence = 2;

    // EventTypes filters by event types
    repeated string event_types = 3;
}

// ListEventsResponse streams feature events
message ListEventsResponse {
    // Event is a feature event
    FeatureEvent event = 1;
}

// FeatureEvent represents a feature execution event
message FeatureEvent {
    // ID is the event ID
    string id = 1;

    // Type is the event type
    string type = 2;

    // SequenceNumber is the event sequence
    uint64 sequence_number = 3;

    // Timestamp is when the event occurred
    google.protobuf.Timestamp timestamp = 4;

    // Summary is a human-readable event summary
    string summary = 5;

    // Payload is the event payload (JSON)
    google.protobuf.Struct payload = 6;
}

// GetArtifactsRequest retrieves feature artifacts
message GetArtifactsRequest {
    // FeatureExecutionID is the feature execution ID
    string feature_execution_id = 1 [(buf.validate.field).string.uuid = true];
}

// GetArtifactsResponse returns feature artifacts
message GetArtifactsResponse {
    // Artifacts lists available artifacts
    repeated Artifact artifacts = 1;
}

// Artifact represents a feature execution artifact
message Artifact {
    // Name is the artifact name
    string name = 1;

    // Type is the artifact type
    ArtifactType type = 2;

    // URL is the download URL (signed, time-limited)
    string url = 3;

    // Size is the artifact size in bytes
    int64 size = 4;

    // ContentType is the MIME type
    string content_type = 5;

    // CreatedAt is when the artifact was created
    google.protobuf.Timestamp created_at = 6;
}

// ArtifactType categorizes artifacts
enum ArtifactType {
    ARTIFACT_TYPE_UNSPECIFIED = 0;
    ARTIFACT_TYPE_PATCH = 1;
    ARTIFACT_TYPE_BUILD_LOG = 2;
    ARTIFACT_TYPE_TEST_REPORT = 3;
    ARTIFACT_TYPE_COVERAGE_REPORT = 4;
    ARTIFACT_TYPE_ANALYSIS_REPORT = 5;
}
```

---

## Repository Service

### Service Definition

```protobuf
// feature/repository/v1/repository.proto

syntax = "proto3";

package feature.repository.v1;

import "google/protobuf/timestamp.proto";
import "buf/validate/validate.proto";
import "feature/common/v1/common.proto";

option go_package = "github.com/antinvestor/apis/go/feature/repository/v1;repositoryv1";

// RepositoryService manages repository registration
service RepositoryService {
    // Register registers a new repository
    rpc Register(RegisterRequest) returns (RegisterResponse) {
        option idempotency_level = IDEMPOTENT;
    }

    // Get retrieves a repository by ID
    rpc Get(GetRequest) returns (GetResponse) {
        option idempotency_level = NO_SIDE_EFFECTS;
    }

    // GetByName retrieves a repository by name
    rpc GetByName(GetByNameRequest) returns (GetByNameResponse) {
        option idempotency_level = NO_SIDE_EFFECTS;
    }

    // List lists repositories
    rpc List(ListRequest) returns (stream ListResponse) {
        option idempotency_level = NO_SIDE_EFFECTS;
    }

    // Update updates repository configuration
    rpc Update(UpdateRequest) returns (UpdateResponse) {
        option idempotency_level = IDEMPOTENT;
    }

    // UpdateCredentials updates repository credentials
    rpc UpdateCredentials(UpdateCredentialsRequest) returns (UpdateCredentialsResponse) {
        option idempotency_level = IDEMPOTENT;
    }

    // ValidateAccess validates repository accessibility
    rpc ValidateAccess(ValidateAccessRequest) returns (ValidateAccessResponse) {
        option idempotency_level = NO_SIDE_EFFECTS;
    }

    // Delete removes a repository
    rpc Delete(DeleteRequest) returns (DeleteResponse) {
        option idempotency_level = IDEMPOTENT;
    }
}
```

### Messages

```protobuf
// Repository represents a registered repository
message Repository {
    // ID is the unique repository identifier
    string id = 1;

    // Name is the repository display name
    string name = 2;

    // URL is the repository clone URL
    string url = 3;

    // DefaultBranch is the default branch name
    string default_branch = 4;

    // Provider is the detected git provider
    GitProvider provider = 5;

    // HasCredentials indicates if credentials are configured
    bool has_credentials = 6;

    // CredentialType is the type of configured credentials
    CredentialType credential_type = 7;

    // Properties holds additional metadata
    feature.common.v1.Properties properties = 8;

    // State is the repository lifecycle state
    feature.common.v1.State state = 9;

    // LastAccessedAt is when the repository was last accessed
    google.protobuf.Timestamp last_accessed_at = 10;

    // CreatedAt is when the repository was registered
    google.protobuf.Timestamp created_at = 11;

    // UpdatedAt is when the repository was last updated
    google.protobuf.Timestamp updated_at = 12;
}

// GitProvider identifies the git hosting provider
enum GitProvider {
    GIT_PROVIDER_UNSPECIFIED = 0;
    GIT_PROVIDER_GITHUB = 1;
    GIT_PROVIDER_GITLAB = 2;
    GIT_PROVIDER_BITBUCKET = 3;
    GIT_PROVIDER_AZURE_DEVOPS = 4;
    GIT_PROVIDER_GITEA = 5;
    GIT_PROVIDER_GENERIC = 6;
}

// CredentialType identifies the credential mechanism
enum CredentialType {
    CREDENTIAL_TYPE_UNSPECIFIED = 0;
    CREDENTIAL_TYPE_SSH_KEY = 1;
    CREDENTIAL_TYPE_TOKEN = 2;
    CREDENTIAL_TYPE_OAUTH = 3;
    CREDENTIAL_TYPE_BASIC = 4;
}

// Credential contains repository credentials
message Credential {
    // Type is the credential type
    CredentialType type = 1;

    // SSHPrivateKey is the SSH private key (for SSH_KEY type)
    string ssh_private_key = 2;

    // Token is the access token (for TOKEN type)
    string token = 3;

    // Username is the username (for BASIC type)
    string username = 4;

    // Password is the password (for BASIC type)
    string password = 5;

    // OAuthToken is the OAuth token (for OAUTH type)
    string oauth_token = 6;

    // OAuthRefreshToken is the OAuth refresh token
    string oauth_refresh_token = 7;
}

// RegisterRequest registers a new repository
message RegisterRequest {
    // Name is the repository display name
    string name = 1 [(buf.validate.field).string = {min_len: 1, max_len: 255}];

    // URL is the repository clone URL
    string url = 2 [(buf.validate.field).string.uri = true];

    // DefaultBranch overrides the default branch (auto-detected if empty)
    string default_branch = 3;

    // Credential provides repository credentials
    Credential credential = 4;

    // Properties holds additional metadata
    feature.common.v1.Properties properties = 5;
}

// RegisterResponse returns the registered repository
message RegisterResponse {
    // Repository is the registered repository
    Repository repository = 1;
}

// GetRequest retrieves a repository by ID
message GetRequest {
    // ID is the repository ID
    string id = 1 [(buf.validate.field).string.uuid = true];
}

// GetResponse returns the repository
message GetResponse {
    // Repository is the repository
    Repository repository = 1;
}

// GetByNameRequest retrieves a repository by name
message GetByNameRequest {
    // Name is the repository name
    string name = 1 [(buf.validate.field).string.min_len = 1];
}

// GetByNameResponse returns the repository
message GetByNameResponse {
    // Repository is the repository
    Repository repository = 1;
}

// ListRequest lists repositories
message ListRequest {
    // Query is a text search query
    string query = 1;

    // States filters by repository states
    repeated feature.common.v1.State states = 2;

    // Pagination controls result pagination
    feature.common.v1.Pagination pagination = 3;
}

// ListResponse streams repositories
message ListResponse {
    // Repository is a matching repository
    Repository repository = 1;
}

// UpdateRequest updates repository configuration
message UpdateRequest {
    // ID is the repository ID
    string id = 1 [(buf.validate.field).string.uuid = true];

    // Name updates the repository name
    optional string name = 2;

    // DefaultBranch updates the default branch
    optional string default_branch = 3;

    // Properties updates additional metadata
    feature.common.v1.Properties properties = 4;
}

// UpdateResponse returns the updated repository
message UpdateResponse {
    // Repository is the updated repository
    Repository repository = 1;
}

// UpdateCredentialsRequest updates repository credentials
message UpdateCredentialsRequest {
    // ID is the repository ID
    string id = 1 [(buf.validate.field).string.uuid = true];

    // Credential is the new credential
    Credential credential = 2 [(buf.validate.field).required = true];
}

// UpdateCredentialsResponse confirms credential update
message UpdateCredentialsResponse {
    // Repository is the updated repository
    Repository repository = 1;
}

// ValidateAccessRequest validates repository access
message ValidateAccessRequest {
    // ID is the repository ID
    string id = 1 [(buf.validate.field).string.uuid = true];
}

// ValidateAccessResponse returns access validation result
message ValidateAccessResponse {
    // Valid indicates if access is valid
    bool valid = 1;

    // Error describes the access error (if invalid)
    string error = 2;

    // Permissions lists available permissions
    repeated Permission permissions = 3;
}

// Permission describes a repository permission
enum Permission {
    PERMISSION_UNSPECIFIED = 0;
    PERMISSION_READ = 1;
    PERMISSION_WRITE = 2;
    PERMISSION_ADMIN = 3;
}

// DeleteRequest deletes a repository
message DeleteRequest {
    // ID is the repository ID
    string id = 1 [(buf.validate.field).string.uuid = true];
}

// DeleteResponse confirms deletion
message DeleteResponse {
    // Deleted indicates if the repository was deleted
    bool deleted = 1;
}
```

---

## HTTP API Mapping

Connect-RPC automatically provides HTTP/JSON endpoints. The mapping follows Connect protocol conventions:

### Feature Service Endpoints

| Method | HTTP Path | Description |
|--------|-----------|-------------|
| `Create` | `POST /feature.feature.v1.FeatureService/Create` | Create feature |
| `Get` | `POST /feature.feature.v1.FeatureService/Get` | Get feature by ID |
| `GetByCorrelation` | `POST /feature.feature.v1.FeatureService/GetByCorrelation` | Get by correlation ID |
| `Search` | `POST /feature.feature.v1.FeatureService/Search` | Search features (streaming) |
| `Cancel` | `POST /feature.feature.v1.FeatureService/Cancel` | Cancel feature |
| `Retry` | `POST /feature.feature.v1.FeatureService/Retry` | Retry failed feature |
| `ListEvents` | `POST /feature.feature.v1.FeatureService/ListEvents` | List events (streaming) |
| `GetArtifacts` | `POST /feature.feature.v1.FeatureService/GetArtifacts` | Get artifacts |

### Repository Service Endpoints

| Method | HTTP Path | Description |
|--------|-----------|-------------|
| `Register` | `POST /feature.repository.v1.RepositoryService/Register` | Register repository |
| `Get` | `POST /feature.repository.v1.RepositoryService/Get` | Get repository |
| `GetByName` | `POST /feature.repository.v1.RepositoryService/GetByName` | Get by name |
| `List` | `POST /feature.repository.v1.RepositoryService/List` | List repositories |
| `Update` | `POST /feature.repository.v1.RepositoryService/Update` | Update repository |
| `UpdateCredentials` | `POST /feature.repository.v1.RepositoryService/UpdateCredentials` | Update credentials |
| `ValidateAccess` | `POST /feature.repository.v1.RepositoryService/ValidateAccess` | Validate access |
| `Delete` | `POST /feature.repository.v1.RepositoryService/Delete` | Delete repository |

---

## Usage Examples

### Create a Feature

**Request:**
```bash
curl -X POST https://feature.api.example.com/feature.feature.v1.FeatureService/Create \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "spec": {
      "title": "Add user authentication",
      "description": "Implement JWT-based user authentication with login and logout endpoints. Include password hashing with bcrypt and token refresh functionality.",
      "repository_id": "550e8400-e29b-41d4-a716-446655440000",
      "target_branch": "main",
      "constraints": {
        "max_steps": 10,
        "timeout_minutes": 30,
        "require_tests": true,
        "require_build": true
      }
    },
    "correlation_id": "client-request-12345"
  }'
```

**Response:**
```json
{
  "execution": {
    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "correlation_id": "client-request-12345",
    "repository_id": "550e8400-e29b-41d4-a716-446655440000",
    "spec": {
      "title": "Add user authentication",
      "description": "Implement JWT-based user authentication...",
      "repository_id": "550e8400-e29b-41d4-a716-446655440000",
      "target_branch": "main",
      "constraints": {
        "max_steps": 10,
        "timeout_minutes": 30,
        "require_tests": true,
        "require_build": true
      }
    },
    "state": "FEATURE_STATE_PENDING",
    "progress": {
      "total_steps": 0,
      "completed_steps": 0,
      "percent_complete": 0
    },
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
}
```

### Get Feature Status

**Request:**
```bash
curl -X POST https://feature.api.example.com/feature.feature.v1.FeatureService/Get \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"id": "7c9e6679-7425-40de-944b-e07fc1f90ae7"}'
```

**Response (In Progress):**
```json
{
  "execution": {
    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "state": "FEATURE_STATE_EXECUTING",
    "plan": {
      "steps": [
        {"index": 0, "description": "Create auth middleware", "type": "STEP_TYPE_CREATE_FILE"},
        {"index": 1, "description": "Implement login endpoint", "type": "STEP_TYPE_CREATE_FILE"},
        {"index": 2, "description": "Implement logout endpoint", "type": "STEP_TYPE_CREATE_FILE"},
        {"index": 3, "description": "Add password hashing utility", "type": "STEP_TYPE_CREATE_FILE"},
        {"index": 4, "description": "Add token refresh endpoint", "type": "STEP_TYPE_CREATE_FILE"},
        {"index": 5, "description": "Write unit tests", "type": "STEP_TYPE_TEST"}
      ],
      "summary": "Implementing JWT authentication with 6 steps"
    },
    "progress": {
      "total_steps": 6,
      "completed_steps": 3,
      "current_step": 3,
      "current_step_description": "Adding password hashing utility",
      "percent_complete": 50.0
    }
  }
}
```

**Response (Completed):**
```json
{
  "execution": {
    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "state": "FEATURE_STATE_COMPLETED",
    "result": {
      "branch_name": "feature/7c9e6679",
      "commit_sha": "abc123def456789",
      "commit_count": 6,
      "files_changed": 8,
      "lines_added": 450,
      "lines_removed": 12,
      "patch_url": "https://artifacts.example.com/7c9e6679/final.patch",
      "summary": "Successfully implemented JWT authentication with login, logout, and token refresh"
    },
    "completed_at": "2024-01-15T10:45:00Z"
  }
}
```

### Register a Repository

**Request:**
```bash
curl -X POST https://feature.api.example.com/feature.repository.v1.RepositoryService/Register \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-app",
    "url": "git@github.com:myorg/my-app.git",
    "default_branch": "main",
    "credential": {
      "type": "CREDENTIAL_TYPE_SSH_KEY",
      "ssh_private_key": "-----BEGIN OPENSSH PRIVATE KEY-----\n..."
    }
  }'
```

**Response:**
```json
{
  "repository": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "my-app",
    "url": "git@github.com:myorg/my-app.git",
    "default_branch": "main",
    "provider": "GIT_PROVIDER_GITHUB",
    "has_credentials": true,
    "credential_type": "CREDENTIAL_TYPE_SSH_KEY",
    "state": "STATE_ACTIVE",
    "created_at": "2024-01-15T10:00:00Z",
    "updated_at": "2024-01-15T10:00:00Z"
  }
}
```

---

## Error Responses

All errors follow Connect-RPC error format:

```json
{
  "code": "invalid_argument",
  "message": "repository_id is required",
  "details": [
    {
      "@type": "type.googleapis.com/buf.validate.Violations",
      "violations": [
        {
          "field_path": "spec.repository_id",
          "constraint_id": "string.uuid",
          "message": "value must be a valid UUID"
        }
      ]
    }
  ]
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `cancelled` | 499 | Request cancelled by client |
| `unknown` | 500 | Unknown error |
| `invalid_argument` | 400 | Invalid request parameters |
| `deadline_exceeded` | 504 | Request timeout |
| `not_found` | 404 | Resource not found |
| `already_exists` | 409 | Resource already exists |
| `permission_denied` | 403 | Permission denied |
| `resource_exhausted` | 429 | Rate limit exceeded |
| `failed_precondition` | 400 | Precondition failed |
| `aborted` | 409 | Operation aborted |
| `out_of_range` | 400 | Value out of range |
| `unimplemented` | 501 | Not implemented |
| `internal` | 500 | Internal error |
| `unavailable` | 503 | Service unavailable |
| `data_loss` | 500 | Data loss |
| `unauthenticated` | 401 | Not authenticated |

---

## Rate Limits

| Endpoint | Limit | Window |
|----------|-------|--------|
| `Feature.Create` | 10 | per minute |
| `Feature.Get` | 100 | per minute |
| `Feature.Search` | 20 | per minute |
| `Repository.Register` | 10 | per minute |
| `Repository.*` | 100 | per minute |

Rate limit headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1705315260
```

---

## Webhooks (Optional)

While the platform is internally event-driven, optional webhooks can be configured for external notifications:

### Webhook Events

| Event | Trigger |
|-------|---------|
| `feature.created` | Feature execution created |
| `feature.state_changed` | Feature state transition |
| `feature.completed` | Feature successfully completed |
| `feature.failed` | Feature execution failed |

### Webhook Payload

```json
{
  "event": "feature.completed",
  "timestamp": "2024-01-15T10:45:00Z",
  "data": {
    "feature_execution_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "repository_id": "550e8400-e29b-41d4-a716-446655440000",
    "state": "FEATURE_STATE_COMPLETED",
    "result": {
      "branch_name": "feature/7c9e6679",
      "commit_sha": "abc123def456789"
    }
  }
}
```
