package repository

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/antinvestor/builder/apps/worker/config"
	"github.com/antinvestor/builder/internal/events"
)

const (
	// Directory permissions for workspaces.
	dirPermissions = 0o750
	// File permissions for created files.
	filePermissions = 0o600
)

// Service handles git repository operations.
type Service struct {
	cfg           *appconfig.WorkerConfig
	workspaceRepo WorkspaceRepository

	// Semaphore for limiting concurrent clones
	cloneSem chan struct{}
}

// NewService creates a new repository service.
func NewService(
	cfg *appconfig.WorkerConfig,
	workspaceRepo WorkspaceRepository,
) *Service {
	return &Service{
		cfg:           cfg,
		workspaceRepo: workspaceRepo,
		cloneSem:      make(chan struct{}, cfg.MaxConcurrentClones),
	}
}

// RepositoryService is an alias for Service for backwards compatibility.
//
//nolint:revive // intentional stutter for backwards compatibility
type RepositoryService = Service

// NewRepositoryService is an alias for NewService for backwards compatibility.
func NewRepositoryService(cfg *appconfig.WorkerConfig, workspaceRepo WorkspaceRepository) *Service {
	return NewService(cfg, workspaceRepo)
}

// CheckoutRequest contains data for repository checkout.
type CheckoutRequest struct {
	ExecutionID   events.ExecutionID
	RepositoryURL string
	Branch        string
	CommitSHA     string
}

// CheckoutResult contains the result of a checkout operation.
type CheckoutResult struct {
	WorkspacePath  string
	CommitSHA      string
	Branch         string
	CheckoutTimeMS int64
}

// Checkout clones or fetches a repository to the workspace.
func (s *Service) Checkout(
	ctx context.Context,
	req *CheckoutRequest,
) (*CheckoutResult, error) {
	startTime := time.Now()

	// Acquire semaphore
	select {
	case s.cloneSem <- struct{}{}:
		defer func() { <-s.cloneSem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Create workspace directory
	workspacePath := filepath.Join(s.cfg.WorkspaceBasePath, req.ExecutionID.String())
	if err := os.MkdirAll(workspacePath, dirPermissions); err != nil {
		return nil, fmt.Errorf("create workspace directory: %w", err)
	}

	// Clone the repository
	cloneCtx, cancel := context.WithTimeout(
		ctx,
		time.Duration(s.cfg.CloneTimeoutSeconds)*time.Second,
	)
	defer cancel()

	args := []string{
		"clone",
		"--branch",
		req.Branch,
		"--single-branch",
		"--depth",
		"100",
		req.RepositoryURL,
		workspacePath,
	}

	cmd := exec.CommandContext(cloneCtx, "git", args...)
	cmd.Env = s.buildGitEnv()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git clone failed: %w: %s", err, string(output))
	}

	// Get the current commit SHA
	commitSHA := req.CommitSHA
	if commitSHA == "" {
		shaCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
		shaCmd.Dir = workspacePath
		shaOutput, shaErr := shaCmd.Output()
		if shaErr != nil {
			return nil, fmt.Errorf("get commit SHA: %w", shaErr)
		}
		commitSHA = strings.TrimSpace(string(shaOutput))
	}

	// Record workspace
	workspace := &Workspace{
		ExecutionID:   req.ExecutionID.String(),
		LocalPath:     workspacePath,
		RepositoryURL: req.RepositoryURL,
		Branch:        req.Branch,
		CommitSHA:     commitSHA,
		CreatedAt:     time.Now(),
	}

	if createErr := s.workspaceRepo.Create(ctx, workspace); createErr != nil {
		return nil, fmt.Errorf("record workspace: %w", createErr)
	}

	return &CheckoutResult{
		WorkspacePath:  workspacePath,
		CommitSHA:      commitSHA,
		Branch:         req.Branch,
		CheckoutTimeMS: time.Since(startTime).Milliseconds(),
	}, nil
}

// GetWorkspace retrieves a workspace by execution ID.
func (s *Service) GetWorkspace(
	ctx context.Context,
	executionID events.ExecutionID,
) (*Workspace, error) {
	return s.workspaceRepo.GetByExecutionID(ctx, executionID.String())
}

// GetWorkspacePath returns the workspace path for an execution.
func (s *Service) GetWorkspacePath(executionID events.ExecutionID) string {
	return filepath.Join(s.cfg.WorkspaceBasePath, executionID.String())
}

// ReadFiles reads the content of multiple files from a workspace.
func (s *Service) ReadFiles(
	_ context.Context,
	executionID events.ExecutionID,
	paths []string,
) (map[string]string, error) {
	workspacePath := s.GetWorkspacePath(executionID)

	contents := make(map[string]string, len(paths))
	for _, path := range paths {
		fullPath := filepath.Join(workspacePath, path)

		// Security check: ensure path is within workspace
		if !isSubPath(workspacePath, fullPath) {
			return nil, fmt.Errorf("path escapes workspace: %s", path)
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read file %s: %w", path, err)
		}

		contents[path] = string(content)
	}

	return contents, nil
}

// GetProjectStructure returns the project structure as a string.
func (s *Service) GetProjectStructure(
	ctx context.Context,
	executionID events.ExecutionID,
) (string, error) {
	workspacePath := s.GetWorkspacePath(executionID)

	// Use tree command if available, otherwise fall back to find
	cmd := exec.CommandContext(
		ctx,
		"tree",
		"-L",
		"4",
		"--noreport",
		"-I",
		"node_modules|.git|vendor|__pycache__|.venv",
	)
	cmd.Dir = workspacePath

	output, err := cmd.Output()
	if err != nil {
		// Fallback to find
		cmd = exec.CommandContext(
			ctx,
			"find",
			".",
			"-type",
			"f",
			"-not",
			"-path",
			"./.git/*",
			"-not",
			"-path",
			"./node_modules/*",
		)
		cmd.Dir = workspacePath
		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("get project structure: %w", err)
		}
	}

	return string(output), nil
}

// CleanupWorkspace removes a workspace.
func (s *Service) CleanupWorkspace(
	ctx context.Context,
	executionID events.ExecutionID,
) error {
	workspace, err := s.workspaceRepo.GetByExecutionID(ctx, executionID.String())
	if err != nil {
		return err
	}

	// Remove directory
	if rmErr := os.RemoveAll(workspace.LocalPath); rmErr != nil {
		return fmt.Errorf("remove workspace directory: %w", rmErr)
	}

	// Remove record
	return s.workspaceRepo.Delete(ctx, executionID.String())
}

// ApplyPatch applies a patch to a file in the workspace.
func (s *Service) ApplyPatch(
	ctx context.Context,
	executionID events.ExecutionID,
	patch *events.Patch,
) error {
	workspace, err := s.workspaceRepo.GetByExecutionID(ctx, executionID.String())
	if err != nil {
		return err
	}

	filePath := filepath.Join(workspace.LocalPath, patch.FilePath)

	switch patch.Action {
	case events.FileActionCreate, events.FileActionModify:
		// Ensure parent directory exists
		if mkdirErr := os.MkdirAll(filepath.Dir(filePath), dirPermissions); mkdirErr != nil {
			return fmt.Errorf("create parent directory: %w", mkdirErr)
		}
		// Write new content
		if writeErr := os.WriteFile(filePath, []byte(patch.NewContent), filePermissions); writeErr != nil {
			return fmt.Errorf("write file: %w", writeErr)
		}

	case events.FileActionDelete:
		if rmErr := os.Remove(filePath); rmErr != nil && !os.IsNotExist(rmErr) {
			return fmt.Errorf("delete file: %w", rmErr)
		}

	case events.FileActionRename:
		oldPath := filepath.Join(workspace.LocalPath, patch.OldPath)
		if renameErr := os.Rename(oldPath, filePath); renameErr != nil {
			return fmt.Errorf("rename file: %w", renameErr)
		}
	}

	return nil
}

// CreateCommit creates a git commit in the workspace.
func (s *Service) CreateCommit(
	ctx context.Context,
	executionID events.ExecutionID,
	message string,
) (*events.CommitInfo, error) {
	workspacePath := s.GetWorkspacePath(executionID)

	// Add all changes
	addCmd := exec.CommandContext(ctx, "git", "add", "-A")
	addCmd.Dir = workspacePath
	if output, err := addCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git add failed: %w: %s", err, string(output))
	}

	// Create commit
	commitCmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	commitCmd.Dir = workspacePath
	commitCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Feature Service",
		"GIT_AUTHOR_EMAIL=feature-service@example.com",
		"GIT_COMMITTER_NAME=Feature Service",
		"GIT_COMMITTER_EMAIL=feature-service@example.com",
	)
	if output, err := commitCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git commit failed: %w: %s", err, string(output))
	}

	// Get commit SHA
	shaCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	shaCmd.Dir = workspacePath
	shaOutput, err := shaCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get commit SHA: %w", err)
	}

	commitSHA := strings.TrimSpace(string(shaOutput))

	return &events.CommitInfo{
		SHA:       commitSHA,
		Message:   message,
		Timestamp: time.Now(),
		Author: events.GitIdentity{
			Name:  "Feature Service",
			Email: "feature-service@example.com",
		},
		Committer: events.GitIdentity{
			Name:  "Feature Service",
			Email: "feature-service@example.com",
		},
	}, nil
}

// PushBranch pushes the feature branch to the remote.
func (s *Service) PushBranch(
	ctx context.Context,
	executionID events.ExecutionID,
	branchName string,
) error {
	workspacePath := s.GetWorkspacePath(executionID)

	pushCmd := exec.CommandContext(ctx, "git", "push", "origin", branchName)
	pushCmd.Dir = workspacePath
	pushCmd.Env = s.buildGitEnv()

	if output, err := pushCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push failed: %w: %s", err, string(output))
	}

	return nil
}

// buildGitEnv builds environment variables for git commands.
func (s *Service) buildGitEnv() []string {
	env := os.Environ()

	// Add SSH configuration if available
	if s.cfg.GitSSHKeyPath != "" {
		env = append(
			env,
			fmt.Sprintf(
				"GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=accept-new",
				s.cfg.GitSSHKeyPath,
			),
		)
	}

	// Add HTTPS credentials if available
	if s.cfg.GitHTTPSUsername != "" && s.cfg.GitHTTPSPassword != "" {
		env = append(env, "GIT_TERMINAL_PROMPT=0")
	}

	return env
}

// isSubPath checks if child is a subpath of parent.
func isSubPath(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)

	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}

	return !filepath.IsAbs(rel) && rel != ".." && !hasPrefix(rel, ".."+string(filepath.Separator))
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
