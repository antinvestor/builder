package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/pitabwire/frame/datastore/pool"
	"gorm.io/gorm"
)

// ErrDatabaseUnavailable is returned when the database connection is not available.
var ErrDatabaseUnavailable = errors.New("database connection is not available")

// ExecutionStatus represents the status of an execution.
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusAborted   ExecutionStatus = "aborted"
)

// Execution represents a feature execution record.
type Execution struct {
	ID             string          `json:"id"                      gorm:"primaryKey"`
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
func (r *PGExecutionRepository) UpdateStatus(
	ctx context.Context,
	id string,
	status ExecutionStatus,
	errorMsg string,
) error {
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
	if status == ExecutionStatusCompleted || status == ExecutionStatusFailed ||
		status == ExecutionStatusAborted {
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

// WorkspaceStatus represents the status of a workspace.
type WorkspaceStatus string

const (
	WorkspaceStatusActive         WorkspaceStatus = "active"
	WorkspaceStatusCleanupPending WorkspaceStatus = "cleanup_pending"
	WorkspaceStatusCleaned        WorkspaceStatus = "cleaned"
)

// WorkspaceRepository handles workspace persistence.
type WorkspaceRepository interface {
	Create(ctx context.Context, workspace *Workspace) error
	GetByExecutionID(ctx context.Context, executionID string) (*Workspace, error)
	Delete(ctx context.Context, executionID string) error
	UpdateStatus(ctx context.Context, executionID string, status WorkspaceStatus) error
	UpdateLastAccessed(ctx context.Context, executionID string) error
	ListByStatus(ctx context.Context, status WorkspaceStatus) ([]*Workspace, error)
	ListOrphaned(ctx context.Context, olderThan time.Duration) ([]*Workspace, error)
	ListAll(ctx context.Context) ([]*Workspace, error)
}

// Workspace represents a repository workspace.
type Workspace struct {
	ExecutionID   string          `json:"execution_id"   gorm:"primaryKey"`
	LocalPath     string          `json:"local_path"`
	RepositoryURL string          `json:"repository_url"`
	Branch        string          `json:"branch"`
	CommitSHA     string          `json:"commit_sha"`
	Status        WorkspaceStatus `json:"status"         gorm:"default:active"`
	CreatedAt     time.Time       `json:"created_at"`
	LastAccessed  time.Time       `json:"last_accessed"`
}

// TableName returns the table name for the Workspace model.
func (Workspace) TableName() string {
	return "workspaces"
}

// PGWorkspaceRepository is the PostgreSQL implementation of WorkspaceRepository.
type PGWorkspaceRepository struct {
	pool pool.Pool
}

// NewWorkspaceRepository creates a new workspace repository.
// If a database pool is provided, it uses PostgreSQL for persistence.
// Otherwise, it falls back to in-memory storage.
func NewWorkspaceRepository(_ context.Context, p pool.Pool) WorkspaceRepository {
	if p != nil {
		return &PGWorkspaceRepository{pool: p}
	}
	return &MemoryWorkspaceRepository{
		workspaces: make(map[string]*Workspace),
	}
}

func (r *PGWorkspaceRepository) db(ctx context.Context, readOnly bool) *gorm.DB {
	if r.pool == nil {
		return nil
	}
	return r.pool.DB(ctx, readOnly)
}

// Create creates a workspace record.
func (r *PGWorkspaceRepository) Create(ctx context.Context, workspace *Workspace) error {
	db := r.db(ctx, false)
	if db == nil {
		return ErrDatabaseUnavailable
	}

	workspace.Status = WorkspaceStatusActive
	workspace.CreatedAt = time.Now()
	workspace.LastAccessed = time.Now()
	return db.Create(workspace).Error
}

// GetByExecutionID retrieves a workspace by execution ID.
func (r *PGWorkspaceRepository) GetByExecutionID(
	ctx context.Context,
	executionID string,
) (*Workspace, error) {
	db := r.db(ctx, true)
	if db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var ws Workspace
	if err := db.First(&ws, "execution_id = ?", executionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("workspace not found: %s", executionID)
		}
		return nil, err
	}
	return &ws, nil
}

// Delete deletes a workspace record.
func (r *PGWorkspaceRepository) Delete(ctx context.Context, executionID string) error {
	db := r.db(ctx, false)
	if db == nil {
		return ErrDatabaseUnavailable
	}

	return db.Delete(&Workspace{}, "execution_id = ?", executionID).Error
}

// UpdateStatus updates the workspace status.
func (r *PGWorkspaceRepository) UpdateStatus(
	ctx context.Context,
	executionID string,
	status WorkspaceStatus,
) error {
	db := r.db(ctx, false)
	if db == nil {
		return ErrDatabaseUnavailable
	}

	return db.Model(&Workspace{}).
		Where("execution_id = ?", executionID).
		Update("status", status).
		Error
}

// UpdateLastAccessed updates the last accessed timestamp.
func (r *PGWorkspaceRepository) UpdateLastAccessed(ctx context.Context, executionID string) error {
	db := r.db(ctx, false)
	if db == nil {
		return ErrDatabaseUnavailable
	}

	return db.Model(&Workspace{}).
		Where("execution_id = ?", executionID).
		Update("last_accessed", time.Now()).
		Error
}

// ListByStatus lists workspaces with a specific status.
func (r *PGWorkspaceRepository) ListByStatus(
	ctx context.Context,
	status WorkspaceStatus,
) ([]*Workspace, error) {
	db := r.db(ctx, true)
	if db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var workspaces []*Workspace
	if err := db.Where("status = ?", status).Find(&workspaces).Error; err != nil {
		return nil, err
	}
	return workspaces, nil
}

// ListOrphaned lists workspaces that haven't been accessed recently.
func (r *PGWorkspaceRepository) ListOrphaned(
	ctx context.Context,
	olderThan time.Duration,
) ([]*Workspace, error) {
	db := r.db(ctx, true)
	if db == nil {
		return nil, ErrDatabaseUnavailable
	}

	cutoff := time.Now().Add(-olderThan)
	var workspaces []*Workspace
	err := db.Where("status = ? AND last_accessed < ?", WorkspaceStatusActive, cutoff).
		Find(&workspaces).Error
	if err != nil {
		return nil, err
	}
	return workspaces, nil
}

// ListAll lists all workspaces.
func (r *PGWorkspaceRepository) ListAll(ctx context.Context) ([]*Workspace, error) {
	db := r.db(ctx, true)
	if db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var workspaces []*Workspace
	if err := db.Find(&workspaces).Error; err != nil {
		return nil, err
	}
	return workspaces, nil
}

// MemoryWorkspaceRepository is an in-memory workspace repository for testing.
type MemoryWorkspaceRepository struct {
	mu         sync.RWMutex
	workspaces map[string]*Workspace
}

// Create creates a workspace record.
func (r *MemoryWorkspaceRepository) Create(ctx context.Context, workspace *Workspace) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	workspace.Status = WorkspaceStatusActive
	workspace.LastAccessed = time.Now()
	r.workspaces[workspace.ExecutionID] = workspace
	return nil
}

// GetByExecutionID retrieves a workspace by execution ID.
func (r *MemoryWorkspaceRepository) GetByExecutionID(
	ctx context.Context,
	executionID string,
) (*Workspace, error) {
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

// UpdateStatus updates the workspace status.
func (r *MemoryWorkspaceRepository) UpdateStatus(
	_ context.Context,
	executionID string,
	status WorkspaceStatus,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ws, ok := r.workspaces[executionID]; ok {
		ws.Status = status
	}
	return nil
}

// UpdateLastAccessed updates the last accessed timestamp.
func (r *MemoryWorkspaceRepository) UpdateLastAccessed(
	_ context.Context,
	executionID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ws, ok := r.workspaces[executionID]; ok {
		ws.LastAccessed = time.Now()
	}
	return nil
}

// ListByStatus lists workspaces with a specific status.
func (r *MemoryWorkspaceRepository) ListByStatus(
	_ context.Context,
	status WorkspaceStatus,
) ([]*Workspace, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*Workspace
	for _, ws := range r.workspaces {
		if ws.Status == status {
			result = append(result, ws)
		}
	}
	return result, nil
}

// ListOrphaned lists workspaces that haven't been accessed recently.
func (r *MemoryWorkspaceRepository) ListOrphaned(
	_ context.Context,
	olderThan time.Duration,
) ([]*Workspace, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cutoff := time.Now().Add(-olderThan)
	var result []*Workspace
	for _, ws := range r.workspaces {
		if ws.Status == WorkspaceStatusActive && ws.LastAccessed.Before(cutoff) {
			result = append(result, ws)
		}
	}
	return result, nil
}

// ListAll lists all workspaces.
func (r *MemoryWorkspaceRepository) ListAll(_ context.Context) ([]*Workspace, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Workspace, 0, len(r.workspaces))
	for _, ws := range r.workspaces {
		result = append(result, ws)
	}
	return result, nil
}
