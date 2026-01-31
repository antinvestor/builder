package events

import "time"

// ===== REPOSITORY CHECKOUT =====

// RepositoryCheckoutStartedPayload is the payload for RepositoryCheckoutStarted.
type RepositoryCheckoutStartedPayload struct {
	RepositoryID  string           `json:"repository_id"`
	RemoteURL     string           `json:"remote_url"`
	TargetRef     string           `json:"target_ref"`
	WorkspacePath string           `json:"workspace_path"`
	Strategy      CheckoutStrategy `json:"strategy"`
	StartedAt     time.Time        `json:"started_at"`
}

// CheckoutStrategy defines how to obtain the repository.
type CheckoutStrategy string

const (
	CheckoutStrategyUnspecified CheckoutStrategy = ""
	CheckoutStrategyFullClone   CheckoutStrategy = "full_clone"    // git clone
	CheckoutStrategyShallowClone CheckoutStrategy = "shallow_clone" // git clone --depth=1
	CheckoutStrategyFetch       CheckoutStrategy = "fetch"         // git fetch (workspace exists)
	CheckoutStrategyCached      CheckoutStrategy = "cached"        // Use existing workspace
)

// RepositoryCheckoutCompletedPayload is the payload for RepositoryCheckoutCompleted.
type RepositoryCheckoutCompletedPayload struct {
	WorkspacePath     string            `json:"workspace_path"`
	HeadCommitSHA     string            `json:"head_commit_sha"`
	HeadCommitMessage string            `json:"head_commit_message"`
	BranchName        string            `json:"branch_name"`
	Metrics           RepositoryMetrics `json:"metrics"`
	DurationMS        int64             `json:"duration_ms"`
	CompletedAt       time.Time         `json:"completed_at"`
}

// RepositoryMetrics contains repository statistics.
type RepositoryMetrics struct {
	TotalSizeBytes int64 `json:"total_size_bytes"`
	FileCount      int   `json:"file_count"`
	DirectoryCount int   `json:"directory_count"`
	CommitCount    int   `json:"commit_count"`
}

// RepositoryCheckoutFailedPayload is the payload for RepositoryCheckoutFailed.
type RepositoryCheckoutFailedPayload struct {
	ErrorCode    CheckoutErrorCode `json:"error_code"`
	ErrorMessage string            `json:"error_message"`
	Retryable    bool              `json:"retryable"`
	ErrorContext map[string]string `json:"error_context,omitempty"`
	FailedAt     time.Time         `json:"failed_at"`
}

// CheckoutErrorCode categorizes checkout errors.
type CheckoutErrorCode string

const (
	CheckoutErrorUnspecified CheckoutErrorCode = ""
	CheckoutErrorNetwork     CheckoutErrorCode = "network"
	CheckoutErrorAuth        CheckoutErrorCode = "auth"
	CheckoutErrorNotFound    CheckoutErrorCode = "not_found"
	CheckoutErrorDiskSpace   CheckoutErrorCode = "disk_space"
	CheckoutErrorTimeout     CheckoutErrorCode = "timeout"
	CheckoutErrorCorruption  CheckoutErrorCode = "corruption"
)

// ===== REPOSITORY INDEXING =====

// RepositoryIndexingStartedPayload is the payload for RepositoryIndexingStarted.
type RepositoryIndexingStartedPayload struct {
	WorkspacePath string    `json:"workspace_path"`
	HeadCommitSHA string    `json:"head_commit_sha"`
	StartedAt     time.Time `json:"started_at"`
}

// RepositoryIndexingCompletedPayload is the payload for RepositoryIndexingCompleted.
type RepositoryIndexingCompletedPayload struct {
	Languages   LanguageBreakdown `json:"languages"`
	BuildSystem BuildSystemInfo   `json:"build_system"`
	Structure   ProjectStructure  `json:"structure"`
	Metrics     IndexingMetrics   `json:"metrics"`
	CompletedAt time.Time         `json:"completed_at"`
}

// LanguageBreakdown contains language detection results.
type LanguageBreakdown struct {
	PrimaryLanguage    string         `json:"primary_language"`
	LanguageFileCounts map[string]int `json:"language_file_counts"`
	LanguageLineCounts map[string]int `json:"language_line_counts"`
}

// BuildSystemInfo contains build system detection results.
type BuildSystemInfo struct {
	Type          BuildSystem `json:"type"`
	ConfigFile    string      `json:"config_file"`
	BuildCommands []string    `json:"build_commands"`
	TestCommands  []string    `json:"test_commands"`
	LintCommands  []string    `json:"lint_commands,omitempty"`
}

// BuildSystem identifies the build system.
type BuildSystem string

const (
	BuildSystemUnspecified BuildSystem = ""
	BuildSystemNPM         BuildSystem = "npm"
	BuildSystemYarn        BuildSystem = "yarn"
	BuildSystemPNPM        BuildSystem = "pnpm"
	BuildSystemBun         BuildSystem = "bun"
	BuildSystemGo          BuildSystem = "go"
	BuildSystemCargo       BuildSystem = "cargo"
	BuildSystemMaven       BuildSystem = "maven"
	BuildSystemGradle      BuildSystem = "gradle"
	BuildSystemMake        BuildSystem = "make"
	BuildSystemCMake       BuildSystem = "cmake"
	BuildSystemBazel       BuildSystem = "bazel"
	BuildSystemPoetry      BuildSystem = "poetry"
	BuildSystemPip         BuildSystem = "pip"
	BuildSystemPipenv      BuildSystem = "pipenv"
	BuildSystemDotNet      BuildSystem = "dotnet"
)

// ProjectStructure describes the project layout.
type ProjectStructure struct {
	SourceDirectories     []string `json:"source_directories"`
	TestDirectories       []string `json:"test_directories"`
	ConfigFiles           []string `json:"config_files"`
	EntryPoints           []string `json:"entry_points"`
	DocumentationFiles    []string `json:"documentation_files"`
	DependencyFiles       []string `json:"dependency_files"`
}

// IndexingMetrics contains indexing statistics.
type IndexingMetrics struct {
	FilesIndexed     int   `json:"files_indexed"`
	SymbolsExtracted int   `json:"symbols_extracted"`
	IndexSizeBytes   int64 `json:"index_size_bytes"`
	DurationMS       int64 `json:"duration_ms"`
}

// RepositoryIndexingFailedPayload is the payload for RepositoryIndexingFailed.
type RepositoryIndexingFailedPayload struct {
	ErrorCode    string            `json:"error_code"`
	ErrorMessage string            `json:"error_message"`
	Retryable    bool              `json:"retryable"`
	ErrorContext map[string]string `json:"error_context,omitempty"`
	FailedAt     time.Time         `json:"failed_at"`
}
