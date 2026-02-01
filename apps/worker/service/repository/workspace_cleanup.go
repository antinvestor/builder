package repository

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/pitabwire/util"
)

// Byte size constants for calculating disk usage.
const bytesPerMB = 1024 * 1024

// WorkspaceCleanupService handles workspace cleanup operations.
type WorkspaceCleanupService struct {
	workspaceRepo     WorkspaceRepository
	workspaceBasePath string
	maxAge            time.Duration

	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// NewWorkspaceCleanupService creates a new workspace cleanup service.
func NewWorkspaceCleanupService(
	workspaceRepo WorkspaceRepository,
	workspaceBasePath string,
	maxAgeHours int,
) *WorkspaceCleanupService {
	return &WorkspaceCleanupService{
		workspaceRepo:     workspaceRepo,
		workspaceBasePath: workspaceBasePath,
		maxAge:            time.Duration(maxAgeHours) * time.Hour,
		stopCh:            make(chan struct{}),
		stoppedCh:         make(chan struct{}),
	}
}

// Start starts the periodic cleanup goroutine.
func (s *WorkspaceCleanupService) Start(ctx context.Context) {
	log := util.Log(ctx)

	// Run startup cleanup
	if err := s.CleanupOnStartup(ctx); err != nil {
		log.WithError(err).Error("startup workspace cleanup failed")
	}

	// Start periodic cleanup
	go s.periodicCleanup(ctx)
}

// Stop stops the cleanup service gracefully.
func (s *WorkspaceCleanupService) Stop() {
	close(s.stopCh)
	<-s.stoppedCh
}

// periodicCleanup runs cleanup periodically.
func (s *WorkspaceCleanupService) periodicCleanup(ctx context.Context) {
	defer close(s.stoppedCh)

	log := util.Log(ctx)
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if err := s.CleanupOrphaned(ctx); err != nil {
				log.WithError(err).Error("periodic workspace cleanup failed")
			}
		}
	}
}

// CleanupOnStartup reconciles database state with filesystem on startup.
// It identifies workspaces that exist in the database but not on disk (or vice versa).
func (s *WorkspaceCleanupService) CleanupOnStartup(ctx context.Context) error {
	log := util.Log(ctx)
	log.Info("starting workspace cleanup on startup")

	// Get all workspaces from database
	workspaces, err := s.workspaceRepo.ListAll(ctx)
	if err != nil {
		return err
	}

	// Check each workspace
	cleaned := 0

	for _, ws := range workspaces {
		// Check if directory exists
		if _, statErr := os.Stat(ws.LocalPath); os.IsNotExist(statErr) {
			// Directory doesn't exist, mark as cleaned
			updateErr := s.workspaceRepo.UpdateStatus(
				ctx, ws.ExecutionID, WorkspaceStatusCleaned,
			)
			if updateErr != nil {
				log.WithError(updateErr).Error("failed to update workspace status",
					"execution_id", ws.ExecutionID)
				continue
			}
			cleaned++
		}
	}

	// Find directories on disk that aren't in database
	orphanedDirs, detectErr := s.detectOrphanedDirectories(ctx, workspaces)
	if detectErr != nil {
		log.WithError(detectErr).Warn("failed to detect orphaned directories")
	}

	log.Info("workspace startup cleanup completed",
		"db_workspaces", len(workspaces),
		"marked_cleaned", cleaned,
		"orphaned_dirs", orphanedDirs)

	return nil
}

// detectOrphanedDirectories finds directories on disk that aren't tracked in the database.
// Returns the count of orphaned directories found.
func (s *WorkspaceCleanupService) detectOrphanedDirectories(
	ctx context.Context,
	trackedWorkspaces []*Workspace,
) (int, error) {
	log := util.Log(ctx)

	// Build set of tracked paths
	trackedPaths := make(map[string]bool, len(trackedWorkspaces))
	for _, ws := range trackedWorkspaces {
		trackedPaths[ws.LocalPath] = true
	}

	// Check if base path exists
	if _, err := os.Stat(s.workspaceBasePath); os.IsNotExist(err) {
		return 0, nil // No workspace directory yet
	}

	// List directories in workspace base path
	entries, err := os.ReadDir(s.workspaceBasePath)
	if err != nil {
		return 0, err
	}

	orphaned := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(s.workspaceBasePath, entry.Name())
		if trackedPaths[dirPath] {
			continue // Directory is tracked, skip
		}

		// This directory is not tracked - it's orphaned
		log.Warn("found orphaned workspace directory",
			"path", dirPath,
			"execution_id", entry.Name())
		orphaned++

		// Try to clean it up if it's old enough
		s.cleanupOrphanedDirectory(log, entry, dirPath)
	}

	if orphaned > 0 {
		log.Warn("detected orphaned workspace directories",
			"count", orphaned)
	}

	return orphaned, nil
}

// cleanupOrphanedDirectory removes an orphaned directory if it's old enough.
func (s *WorkspaceCleanupService) cleanupOrphanedDirectory(
	log *util.LogEntry,
	entry os.DirEntry,
	dirPath string,
) {
	info, infoErr := entry.Info()
	if infoErr != nil {
		return
	}

	if time.Since(info.ModTime()) <= s.maxAge {
		return
	}

	if removeErr := os.RemoveAll(dirPath); removeErr != nil {
		log.WithError(removeErr).Error("failed to remove orphaned directory",
			"path", dirPath)
		return
	}

	log.Info("removed orphaned workspace directory",
		"path", dirPath)
}

// CleanupOrphaned removes workspaces that haven't been accessed recently.
func (s *WorkspaceCleanupService) CleanupOrphaned(ctx context.Context) error {
	log := util.Log(ctx)

	// Find orphaned workspaces
	orphaned, err := s.workspaceRepo.ListOrphaned(ctx, s.maxAge)
	if err != nil {
		return err
	}

	if len(orphaned) == 0 {
		return nil
	}

	log.Info("cleaning up orphaned workspaces",
		"count", len(orphaned))

	cleaned := 0
	for _, ws := range orphaned {
		// Mark as cleanup pending
		updateErr := s.workspaceRepo.UpdateStatus(ctx, ws.ExecutionID, WorkspaceStatusCleanupPending)
		if updateErr != nil {
			log.WithError(updateErr).Error("failed to mark workspace for cleanup",
				"execution_id", ws.ExecutionID)
			continue
		}

		// Remove directory
		if removeErr := os.RemoveAll(ws.LocalPath); removeErr != nil {
			log.WithError(removeErr).Error("failed to remove workspace directory",
				"execution_id", ws.ExecutionID,
				"path", ws.LocalPath)
			continue
		}

		// Mark as cleaned
		updateErr = s.workspaceRepo.UpdateStatus(ctx, ws.ExecutionID, WorkspaceStatusCleaned)
		if updateErr != nil {
			log.WithError(updateErr).Error("failed to mark workspace as cleaned",
				"execution_id", ws.ExecutionID)
			continue
		}

		cleaned++
	}

	log.Info("workspace cleanup completed",
		"total_orphaned", len(orphaned),
		"cleaned", cleaned)

	return nil
}

// GetWorkspaceStats returns statistics about workspace usage.
func (s *WorkspaceCleanupService) GetWorkspaceStats(ctx context.Context) (*WorkspaceStats, error) {
	active, err := s.workspaceRepo.ListByStatus(ctx, WorkspaceStatusActive)
	if err != nil {
		return nil, err
	}

	pending, err := s.workspaceRepo.ListByStatus(ctx, WorkspaceStatusCleanupPending)
	if err != nil {
		return nil, err
	}

	cleaned, err := s.workspaceRepo.ListByStatus(ctx, WorkspaceStatusCleaned)
	if err != nil {
		return nil, err
	}

	// Calculate disk usage for active workspaces
	var totalDiskBytes int64
	for _, ws := range active {
		size, sizeErr := getDirSize(ws.LocalPath)
		if sizeErr == nil {
			totalDiskBytes += size
		}
	}

	return &WorkspaceStats{
		ActiveCount:       len(active),
		CleanupPending:    len(pending),
		CleanedCount:      len(cleaned),
		TotalDiskUsageMB:  totalDiskBytes / bytesPerMB,
		OldestActiveAge:   getOldestAge(active),
		WorkspaceBasePath: s.workspaceBasePath,
		MaxAgeHours:       int(s.maxAge.Hours()),
	}, nil
}

// WorkspaceStats contains workspace statistics.
type WorkspaceStats struct {
	ActiveCount       int           `json:"active_count"`
	CleanupPending    int           `json:"cleanup_pending"`
	CleanedCount      int           `json:"cleaned_count"`
	TotalDiskUsageMB  int64         `json:"total_disk_usage_mb"`
	OldestActiveAge   time.Duration `json:"oldest_active_age"`
	WorkspaceBasePath string        `json:"workspace_base_path"`
	MaxAgeHours       int           `json:"max_age_hours"`
}

// getDirSize calculates the total size of a directory.
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// getOldestAge returns the age of the oldest workspace.
func getOldestAge(workspaces []*Workspace) time.Duration {
	if len(workspaces) == 0 {
		return 0
	}

	oldest := workspaces[0].CreatedAt
	for _, ws := range workspaces[1:] {
		if ws.CreatedAt.Before(oldest) {
			oldest = ws.CreatedAt
		}
	}

	return time.Since(oldest)
}
