-- Migration: Create workspaces table for persistent workspace tracking
-- Issue: #32

CREATE TABLE IF NOT EXISTS workspaces (
    execution_id VARCHAR(255) PRIMARY KEY,
    local_path TEXT NOT NULL,
    repository_url TEXT NOT NULL,
    branch VARCHAR(255) NOT NULL,
    commit_sha VARCHAR(40),
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_accessed TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for querying by status (cleanup queries)
CREATE INDEX IF NOT EXISTS idx_workspaces_status ON workspaces(status);

-- Index for finding orphaned workspaces
CREATE INDEX IF NOT EXISTS idx_workspaces_status_last_accessed ON workspaces(status, last_accessed);

-- Executions table (if not exists)
CREATE TABLE IF NOT EXISTS executions (
    id VARCHAR(255) PRIMARY KEY,
    repository_url TEXT NOT NULL,
    branch VARCHAR(255) NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    requested_by VARCHAR(255),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error_message TEXT,
    iteration_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for querying executions by status
CREATE INDEX IF NOT EXISTS idx_executions_status ON executions(status);

-- Index for querying executions by repository
CREATE INDEX IF NOT EXISTS idx_executions_repository ON executions(repository_url, branch);
