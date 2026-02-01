package events

import "time"

// ===== PATCH GENERATION =====

// PatchGenerationStartedPayload is the payload for PatchGenerationStarted.
type PatchGenerationStartedPayload struct {
	PlanID     string    `json:"plan_id"`
	TotalSteps int       `json:"total_steps"`
	StartedAt  time.Time `json:"started_at"`
}

// PatchGenerationStepStartedPayload is the payload for PatchGenerationStepStarted.
type PatchGenerationStepStartedPayload struct {
	StepNumber   int       `json:"step_number"`
	StepID       StepID    `json:"step_id"`
	Action       string    `json:"action"`
	TargetFiles  []string  `json:"target_files"`
	StartedAt    time.Time `json:"started_at"`
}

// PatchGenerationStepCompletedPayload is the payload for PatchGenerationStepCompleted.
type PatchGenerationStepCompletedPayload struct {
	StepNumber     int               `json:"step_number"`
	StepID         StepID            `json:"step_id"`
	FileChanges    []FileChange      `json:"file_changes"`
	Commit         *CommitInfo       `json:"commit,omitempty"`
	LLMInfo        LLMProcessingInfo `json:"llm_info"`
	StepDurationMS int64             `json:"step_duration_ms"`
	CompletedAt    time.Time         `json:"completed_at"`
}

// FileChange describes a single file change.
type FileChange struct {
	// FilePath is the path to the file.
	FilePath string `json:"file_path"`

	// Action is the action taken.
	Action FileAction `json:"action"`

	// PreviousPath is set for renames.
	PreviousPath string `json:"previous_path,omitempty"`

	// LinesAdded is lines added in this change.
	LinesAdded int `json:"lines_added"`

	// LinesRemoved is lines removed in this change.
	LinesRemoved int `json:"lines_removed"`

	// ContentHash is the SHA-256 hash of the new content.
	ContentHash string `json:"content_hash"`

	// Patch is the unified diff patch (optional, can be large).
	Patch string `json:"patch,omitempty"`

	// Description describes what changed.
	Description string `json:"description"`
}

// CommitInfo describes a git commit.
type CommitInfo struct {
	// SHA is the commit SHA.
	SHA string `json:"sha"`

	// Message is the commit message.
	Message string `json:"message"`

	// Author is the commit author.
	Author GitIdentity `json:"author"`

	// Committer is the committer (may differ from author).
	Committer GitIdentity `json:"committer"`

	// Timestamp is when the commit was created.
	Timestamp time.Time `json:"timestamp"`

	// ParentSHAs are the parent commit SHAs.
	ParentSHAs []string `json:"parent_shas"`

	// FilesChanged is the number of files changed.
	FilesChanged int `json:"files_changed"`

	// Insertions is total lines added.
	Insertions int `json:"insertions"`

	// Deletions is total lines removed.
	Deletions int `json:"deletions"`
}

// GitIdentity represents a git author/committer.
type GitIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// PatchGenerationStepFailedPayload is the payload for PatchGenerationStepFailed.
type PatchGenerationStepFailedPayload struct {
	StepNumber     int               `json:"step_number"`
	StepID         StepID            `json:"step_id"`
	ErrorCode      string            `json:"error_code"`
	ErrorMessage   string            `json:"error_message"`
	ErrorCategory  StepErrorCategory `json:"error_category"`
	Retryable      bool              `json:"retryable"`
	ErrorContext   map[string]string `json:"error_context,omitempty"`
	PartialChanges []FileChange      `json:"partial_changes,omitempty"`
	FailedAt       time.Time         `json:"failed_at"`
}

// StepErrorCategory categorizes step errors.
type StepErrorCategory string

const (
	StepErrorCategoryLLM        StepErrorCategory = "llm"        // LLM invocation failed
	StepErrorCategoryParsing    StepErrorCategory = "parsing"    // Could not parse LLM output
	StepErrorCategorySyntax     StepErrorCategory = "syntax"     // Generated code has syntax errors
	StepErrorCategoryConflict   StepErrorCategory = "conflict"   // File conflict (concurrent modification)
	StepErrorCategoryValidation StepErrorCategory = "validation" // Output validation failed
	StepErrorCategoryTimeout    StepErrorCategory = "timeout"    // Step timed out
	StepErrorCategoryResource   StepErrorCategory = "resource"   // Resource exhaustion
)

// PatchGenerationCompletedPayload is the payload for PatchGenerationCompleted.
type PatchGenerationCompletedPayload struct {
	TotalSteps       int          `json:"total_steps"`
	StepsCompleted   int          `json:"steps_completed"`
	TotalFileChanges int          `json:"total_file_changes"`
	FilesCreated     int          `json:"files_created"`
	FilesModified    int          `json:"files_modified"`
	FilesDeleted     int          `json:"files_deleted"`
	TotalLinesAdded  int          `json:"total_lines_added"`
	TotalLinesRemoved int         `json:"total_lines_removed"`
	Commits          []CommitInfo `json:"commits"`
	FinalCommitSHA   string       `json:"final_commit_sha"`
	TotalDurationMS  int64        `json:"total_duration_ms"`
	TotalLLMTokens   int          `json:"total_llm_tokens"`
	CompletedAt      time.Time    `json:"completed_at"`
}

// PatchGenerationFailedPayload is the payload for PatchGenerationFailed.
type PatchGenerationFailedPayload struct {
	ErrorCode    string            `json:"error_code"`
	ErrorMessage string            `json:"error_message"`
	Retryable    bool              `json:"retryable"`
	ErrorContext map[string]string `json:"error_context,omitempty"`
	FailedAt     time.Time         `json:"failed_at"`
}

// ===== UNIFIED DIFF HELPERS =====

// DiffHunk represents a section of a unified diff.
type DiffHunk struct {
	OldStart int      `json:"old_start"`
	OldLines int      `json:"old_lines"`
	NewStart int      `json:"new_start"`
	NewLines int      `json:"new_lines"`
	Lines    []string `json:"lines"`
}

// FileDiff represents the complete diff for a single file.
type FileDiff struct {
	OldPath string     `json:"old_path"`
	NewPath string     `json:"new_path"`
	Mode    FileMode   `json:"mode"`
	Hunks   []DiffHunk `json:"hunks"`
}

// FileMode represents file permission mode.
type FileMode string

const (
	FileModeRegular    FileMode = "100644"
	FileModeExecutable FileMode = "100755"
	FileModeSymlink    FileMode = "120000"
)
