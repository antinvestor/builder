package events

import "time"

// ===== GIT BRANCH EVENTS =====

// GitBranchCreatedPayload is the payload for GitBranchCreated.
type GitBranchCreatedPayload struct {
	// BranchName is the name of the created branch.
	BranchName string `json:"branch_name"`

	// BaseBranch is the branch it was created from.
	BaseBranch string `json:"base_branch"`

	// BaseCommitSHA is the commit SHA of the base.
	BaseCommitSHA string `json:"base_commit_sha"`

	// CreatedAt is when the branch was created.
	CreatedAt time.Time `json:"created_at"`
}

// ===== GIT COMMIT EVENTS =====

// GitCommitCreatedPayload is the payload for GitCommitCreated.
type GitCommitCreatedPayload struct {
	// Commit contains commit details.
	Commit CommitInfo `json:"commit"`

	// StepNumber is the step that created this commit (if applicable).
	StepNumber int `json:"step_number,omitempty"`

	// StepID is the step ID (if applicable).
	StepID *StepID `json:"step_id,omitempty"`

	// IsIterationCommit indicates if this is from an iteration.
	IsIterationCommit bool `json:"is_iteration_commit"`

	// IterationNumber is set if this is an iteration commit.
	IterationNumber int `json:"iteration_number,omitempty"`
}

// ===== GIT PUSH EVENTS =====

// GitPushStartedPayload is the payload for GitPushStarted.
type GitPushStartedPayload struct {
	// BranchName is the branch being pushed.
	BranchName string `json:"branch_name"`

	// RemoteName is the remote name (usually "origin").
	RemoteName string `json:"remote_name"`

	// RemoteURL is the remote URL.
	RemoteURL string `json:"remote_url"`

	// LocalCommitSHA is the local commit being pushed.
	LocalCommitSHA string `json:"local_commit_sha"`

	// CommitCount is the number of commits being pushed.
	CommitCount int `json:"commit_count"`

	// StartedAt is when push started.
	StartedAt time.Time `json:"started_at"`
}

// GitPushCompletedPayload is the payload for GitPushCompleted.
type GitPushCompletedPayload struct {
	// BranchName is the pushed branch.
	BranchName string `json:"branch_name"`

	// RemoteRef is the full remote reference.
	RemoteRef string `json:"remote_ref"`

	// RemoteCommitSHA is the commit SHA on remote.
	RemoteCommitSHA string `json:"remote_commit_sha"`

	// CommitsPushed is the number of commits pushed.
	CommitsPushed int `json:"commits_pushed"`

	// DurationMS is push duration.
	DurationMS int64 `json:"duration_ms"`

	// CompletedAt is when push completed.
	CompletedAt time.Time `json:"completed_at"`
}

// GitPushFailedPayload is the payload for GitPushFailed.
type GitPushFailedPayload struct {
	// BranchName is the branch that failed to push.
	BranchName string `json:"branch_name"`

	// ErrorCode categorizes the push error.
	ErrorCode GitPushErrorCode `json:"error_code"`

	// ErrorMessage is the error message.
	ErrorMessage string `json:"error_message"`

	// Retryable indicates if push can be retried.
	Retryable bool `json:"retryable"`

	// ErrorContext provides additional context.
	ErrorContext map[string]string `json:"error_context,omitempty"`

	// FailedAt is when push failed.
	FailedAt time.Time `json:"failed_at"`
}

// GitPushErrorCode categorizes push errors.
type GitPushErrorCode string

const (
	GitPushErrorNetwork     GitPushErrorCode = "network"
	GitPushErrorAuth        GitPushErrorCode = "auth"
	GitPushErrorRejected    GitPushErrorCode = "rejected"     // Push rejected (e.g., non-fast-forward)
	GitPushErrorHook        GitPushErrorCode = "hook"         // Pre-push or server hook failed
	GitPushErrorQuota       GitPushErrorCode = "quota"        // Storage quota exceeded
	GitPushErrorTimeout     GitPushErrorCode = "timeout"
	GitPushErrorProtected   GitPushErrorCode = "protected"    // Protected branch rules
)

// ===== GIT OPERATION HELPERS =====

// GitRef represents a git reference.
type GitRef struct {
	// Name is the ref name (e.g., "refs/heads/main").
	Name string `json:"name"`

	// SHA is the commit SHA.
	SHA string `json:"sha"`

	// RefType is the type of ref.
	RefType GitRefType `json:"ref_type"`
}

// GitRefType categorizes git refs.
type GitRefType string

const (
	GitRefTypeBranch GitRefType = "branch"
	GitRefTypeTag    GitRefType = "tag"
	GitRefTypeRemote GitRefType = "remote"
)

// GitDiff summarizes differences between two commits.
type GitDiff struct {
	// FromCommit is the starting commit.
	FromCommit string `json:"from_commit"`

	// ToCommit is the ending commit.
	ToCommit string `json:"to_commit"`

	// FilesChanged is the number of files changed.
	FilesChanged int `json:"files_changed"`

	// Insertions is lines added.
	Insertions int `json:"insertions"`

	// Deletions is lines removed.
	Deletions int `json:"deletions"`

	// Files are the changed files.
	Files []GitFileStat `json:"files,omitempty"`
}

// GitFileStat contains per-file diff statistics.
type GitFileStat struct {
	FilePath   string `json:"file_path"`
	Insertions int    `json:"insertions"`
	Deletions  int    `json:"deletions"`
	Binary     bool   `json:"binary"`
	Status     string `json:"status"` // A=added, M=modified, D=deleted, R=renamed
}
