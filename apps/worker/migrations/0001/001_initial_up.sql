-- =============================================================================
-- Worker Service - Initial Database Migration
-- =============================================================================
-- Creates the core tables for the worker service.
-- =============================================================================

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- =============================================================================
-- Executions Table
-- =============================================================================
-- Tracks feature execution requests and their status
CREATE TABLE IF NOT EXISTS executions (
    id VARCHAR(20) PRIMARY KEY,  -- XID format

    -- Request information
    repository_url TEXT NOT NULL,
    branch VARCHAR(255) NOT NULL DEFAULT 'main',
    specification JSONB NOT NULL DEFAULT '{}',

    -- Status tracking
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    phase VARCHAR(50) NOT NULL DEFAULT 'initialization',

    -- Progress tracking
    current_step INTEGER DEFAULT 0,
    total_steps INTEGER DEFAULT 0,
    iteration_count INTEGER DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    -- Result information
    result JSONB,
    error_message TEXT,

    -- Metadata
    metadata JSONB DEFAULT '{}',

    -- Indexes for common queries
    CONSTRAINT valid_status CHECK (status IN (
        'pending', 'initializing', 'running', 'paused',
        'completed', 'failed', 'aborted', 'rolled_back'
    ))
);

CREATE INDEX IF NOT EXISTS idx_executions_status ON executions(status);
CREATE INDEX IF NOT EXISTS idx_executions_created_at ON executions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_executions_repository ON executions(repository_url);

-- =============================================================================
-- Execution Steps Table
-- =============================================================================
-- Tracks individual steps within an execution
CREATE TABLE IF NOT EXISTS execution_steps (
    id VARCHAR(20) PRIMARY KEY,  -- XID format
    execution_id VARCHAR(20) NOT NULL REFERENCES executions(id) ON DELETE CASCADE,

    -- Step information
    step_number INTEGER NOT NULL,
    step_type VARCHAR(50) NOT NULL,
    description TEXT,

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'pending',

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    -- Result
    input JSONB,
    output JSONB,
    error_message TEXT,

    -- Retry tracking
    attempt_number INTEGER DEFAULT 1,
    max_attempts INTEGER DEFAULT 5,

    UNIQUE(execution_id, step_number)
);

CREATE INDEX IF NOT EXISTS idx_execution_steps_execution ON execution_steps(execution_id);
CREATE INDEX IF NOT EXISTS idx_execution_steps_status ON execution_steps(status);

-- =============================================================================
-- Patches Table
-- =============================================================================
-- Stores generated patches for review and application
CREATE TABLE IF NOT EXISTS patches (
    id VARCHAR(20) PRIMARY KEY,  -- XID format
    execution_id VARCHAR(20) NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    step_id VARCHAR(20) REFERENCES execution_steps(id) ON DELETE SET NULL,

    -- Patch information
    file_path TEXT NOT NULL,
    patch_type VARCHAR(50) NOT NULL DEFAULT 'modify',
    content TEXT NOT NULL,

    -- Review status
    review_status VARCHAR(50) DEFAULT 'pending',
    review_notes TEXT,

    -- Application status
    applied BOOLEAN DEFAULT FALSE,
    applied_at TIMESTAMPTZ,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT valid_patch_type CHECK (patch_type IN ('create', 'modify', 'delete', 'rename'))
);

CREATE INDEX IF NOT EXISTS idx_patches_execution ON patches(execution_id);
CREATE INDEX IF NOT EXISTS idx_patches_review ON patches(review_status);

-- =============================================================================
-- Test Results Table
-- =============================================================================
-- Stores test execution results
CREATE TABLE IF NOT EXISTS test_results (
    id VARCHAR(20) PRIMARY KEY,  -- XID format
    execution_id VARCHAR(20) NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    step_id VARCHAR(20) REFERENCES execution_steps(id) ON DELETE SET NULL,

    -- Test information
    test_phase VARCHAR(50) NOT NULL,  -- pre_feature, post_feature
    language VARCHAR(50),
    framework VARCHAR(100),

    -- Results
    total_tests INTEGER DEFAULT 0,
    passed INTEGER DEFAULT 0,
    failed INTEGER DEFAULT 0,
    skipped INTEGER DEFAULT 0,

    -- Coverage
    coverage_percentage DECIMAL(5, 2),
    coverage_report JSONB,

    -- Output
    output TEXT,
    error_output TEXT,

    -- Timing
    duration_ms INTEGER,

    -- Classification
    failure_classification JSONB,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT valid_test_phase CHECK (test_phase IN ('pre_feature', 'post_feature', 'regression'))
);

CREATE INDEX IF NOT EXISTS idx_test_results_execution ON test_results(execution_id);
CREATE INDEX IF NOT EXISTS idx_test_results_phase ON test_results(test_phase);

-- =============================================================================
-- Reviews Table
-- =============================================================================
-- Stores review decisions and assessments
CREATE TABLE IF NOT EXISTS reviews (
    id VARCHAR(20) PRIMARY KEY,  -- XID format
    execution_id VARCHAR(20) NOT NULL REFERENCES executions(id) ON DELETE CASCADE,

    -- Review type
    review_type VARCHAR(50) NOT NULL,
    review_phase VARCHAR(50) NOT NULL,

    -- Decision
    decision VARCHAR(50) NOT NULL,
    rationale TEXT,

    -- Assessments
    security_assessment JSONB,
    architecture_assessment JSONB,
    risk_assessment JSONB,

    -- Issues found
    blocking_issues JSONB DEFAULT '[]',
    warnings JSONB DEFAULT '[]',

    -- Next actions
    next_actions JSONB DEFAULT '[]',

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT valid_decision CHECK (decision IN (
        'approve', 'approve_with_warnings', 'iterate',
        'abort', 'rollback', 'manual_review', 'mark_complete'
    ))
);

CREATE INDEX IF NOT EXISTS idx_reviews_execution ON reviews(execution_id);
CREATE INDEX IF NOT EXISTS idx_reviews_decision ON reviews(decision);

-- =============================================================================
-- Kill Switch State Table
-- =============================================================================
-- Tracks kill switch activations
CREATE TABLE IF NOT EXISTS kill_switch_state (
    id SERIAL PRIMARY KEY,

    -- Scope
    scope VARCHAR(50) NOT NULL,  -- global, feature, repository
    scope_id VARCHAR(255),  -- execution_id or repository_url

    -- Status
    active BOOLEAN NOT NULL DEFAULT TRUE,
    reason VARCHAR(100) NOT NULL,
    details TEXT,

    -- Activation
    activated_by VARCHAR(255) NOT NULL,
    activated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Deactivation
    deactivated_by VARCHAR(255),
    deactivated_at TIMESTAMPTZ,
    deactivation_reason TEXT,

    CONSTRAINT valid_scope CHECK (scope IN ('global', 'feature', 'repository'))
);

CREATE INDEX IF NOT EXISTS idx_kill_switch_active ON kill_switch_state(active, scope);
CREATE INDEX IF NOT EXISTS idx_kill_switch_scope_id ON kill_switch_state(scope_id) WHERE scope_id IS NOT NULL;

-- =============================================================================
-- Event Log Table
-- =============================================================================
-- Audit log for all events (for debugging and compliance)
CREATE TABLE IF NOT EXISTS event_log (
    id BIGSERIAL PRIMARY KEY,

    -- Event identification
    event_id VARCHAR(20) NOT NULL,
    event_name VARCHAR(255) NOT NULL,
    execution_id VARCHAR(20),

    -- Payload
    payload JSONB NOT NULL,

    -- Processing
    processed BOOLEAN DEFAULT FALSE,
    processed_at TIMESTAMPTZ,
    error_message TEXT,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_event_log_execution ON event_log(execution_id);
CREATE INDEX IF NOT EXISTS idx_event_log_name ON event_log(event_name);
CREATE INDEX IF NOT EXISTS idx_event_log_created ON event_log(created_at DESC);

-- =============================================================================
-- Functions
-- =============================================================================

-- Update timestamp trigger
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply trigger to executions
DROP TRIGGER IF EXISTS executions_updated_at ON executions;
CREATE TRIGGER executions_updated_at
    BEFORE UPDATE ON executions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();
