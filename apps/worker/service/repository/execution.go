package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pitabwire/frame/datastore/pool"
	"gorm.io/gorm"
)

// ExecutionStatus represents the status of an execution.
type ExecutionStatus string

const (
	ExecutionStatusPending    ExecutionStatus = "pending"
	ExecutionStatusRunning    ExecutionStatus = "running"
	ExecutionStatusCompleted  ExecutionStatus = "completed"
	ExecutionStatusFailed     ExecutionStatus = "failed"
	ExecutionStatusAborted    ExecutionStatus = "aborted"
)

// Execution represents a feature execution record.
type Execution struct {
	ID             string          `json:"id" gorm:"primaryKey"`
	RepositoryURL  string          `json:"repository_url"`
	Branch         string          `json:"branch"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	Status         ExecutionStatus `json:"status"`
	RequestedBy    string          `json:"requested_by"`
	RequestedAt    time.Time       `json:"requested_at"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	IterationCount int             `json:"iteration_count"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// TableName returns the table name for the Execution model.
func (Execution) TableName() string {
	return "executions"
}

// ExecutionRepository defines the interface for execution persistence.
type ExecutionRepository interface {
	Create(ctx context.Context, execution *Execution) error
	GetByID(ctx context.Context, id string) (*Execution, error)
	UpdateStatus(ctx context.Context, id string, status ExecutionStatus, errorMsg string) error
	IncrementIteration(ctx context.Context, id string) error
}

// PGExecutionRepository is the PostgreSQL implementation of ExecutionRepository.
type PGExecutionRepository struct {
	pool pool.Pool
}

// NewExecutionRepository creates a new execution repository.
func NewExecutionRepository(ctx context.Context, pool pool.Pool) ExecutionRepository {
	return &PGExecutionRepository{
		pool: pool,
	}
}

func (r *PGExecutionRepository) db(ctx context.Context, readOnly bool) *gorm.DB {
	if r.pool == nil {
		return nil
	}
	return r.pool.DB(ctx, readOnly)
}

// Create creates a new execution record.
func (r *PGExecutionRepository) Create(ctx context.Context, execution *Execution) error {
	db := r.db(ctx, false)
	if db == nil {
		return nil // No database, stub mode
	}

	execution.CreatedAt = time.Now()
	execution.UpdatedAt = time.Now()
	return db.Create(execution).Error
}

// GetByID retrieves an execution by ID.
func (r *PGExecutionRepository) GetByID(ctx context.Context, id string) (*Execution, error) {
	db := r.db(ctx, true)
	if db == nil {
		return nil, fmt.Errorf("execution not found: %s", id)
	}

	var exec Execution
	if err := db.First(&exec, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &exec, nil
}

// UpdateStatus updates the execution status.
func (r *PGExecutionRepository) UpdateStatus(ctx context.Context, id string, status ExecutionStatus, errorMsg string) error {
	db := r.db(ctx, false)
	if db == nil {
		return nil
	}

	updates := map[string]interface{}{
		"status":        status,
		"error_message": errorMsg,
		"updated_at":    time.Now(),
	}

	now := time.Now()
	if status == ExecutionStatusRunning {
		updates["started_at"] = &now
	}
	if status == ExecutionStatusCompleted || status == ExecutionStatusFailed || status == ExecutionStatusAborted {
		updates["completed_at"] = &now
	}

	return db.Model(&Execution{}).Where("id = ?", id).Updates(updates).Error
}

// IncrementIteration increments the iteration count.
func (r *PGExecutionRepository) IncrementIteration(ctx context.Context, id string) error {
	db := r.db(ctx, false)
	if db == nil {
		return nil
	}

	return db.Model(&Execution{}).Where("id = ?", id).
		UpdateColumn("iteration_count", gorm.Expr("iteration_count + 1")).
		UpdateColumn("updated_at", time.Now()).Error
}

// Migrate runs database migrations.
func Migrate(ctx context.Context, dbManager interface{}, migrationPath string) error {
	// Stub: In production, use proper migration tooling
	return nil
}

// WorkspaceRepository handles workspace persistence.
type WorkspaceRepository interface {
	Create(ctx context.Context, workspace *Workspace) error
	GetByExecutionID(ctx context.Context, executionID string) (*Workspace, error)
	Delete(ctx context.Context, executionID string) error
}

// Workspace represents a repository workspace.
type Workspace struct {
	ExecutionID   string    `json:"execution_id" gorm:"primaryKey"`
	LocalPath     string    `json:"local_path"`
	RepositoryURL string    `json:"repository_url"`
	Branch        string    `json:"branch"`
	CommitSHA     string    `json:"commit_sha"`
	CreatedAt     time.Time `json:"created_at"`
}

// MemoryWorkspaceRepository is an in-memory workspace repository.
type MemoryWorkspaceRepository struct {
	mu         sync.RWMutex
	workspaces map[string]*Workspace
}

// NewWorkspaceRepository creates a new workspace repository.
func NewWorkspaceRepository(ctx context.Context, pool pool.Pool) WorkspaceRepository {
	return &MemoryWorkspaceRepository{
		workspaces: make(map[string]*Workspace),
	}
}

// Create creates a workspace record.
func (r *MemoryWorkspaceRepository) Create(ctx context.Context, workspace *Workspace) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workspaces[workspace.ExecutionID] = workspace
	return nil
}

// GetByExecutionID retrieves a workspace by execution ID.
func (r *MemoryWorkspaceRepository) GetByExecutionID(ctx context.Context, executionID string) (*Workspace, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ws, ok := r.workspaces[executionID]
	if !ok {
		return nil, fmt.Errorf("workspace not found: %s", executionID)
	}
	return ws, nil
}

// Delete deletes a workspace record.
func (r *MemoryWorkspaceRepository) Delete(ctx context.Context, executionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.workspaces, executionID)
	return nil
}
