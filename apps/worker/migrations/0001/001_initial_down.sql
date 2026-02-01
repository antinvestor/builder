-- =============================================================================
-- Worker Service - Initial Database Migration Rollback
-- =============================================================================
-- Drops all tables created by 001_initial_up.sql
-- =============================================================================

-- Drop triggers first
DROP TRIGGER IF EXISTS executions_updated_at ON executions;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at();

-- Drop indexes (will be dropped automatically with tables, but explicit for clarity)
DROP INDEX IF EXISTS idx_event_log_created;
DROP INDEX IF EXISTS idx_event_log_name;
DROP INDEX IF EXISTS idx_event_log_execution;
DROP INDEX IF EXISTS idx_kill_switch_scope_id;
DROP INDEX IF EXISTS idx_kill_switch_active;
DROP INDEX IF EXISTS idx_reviews_decision;
DROP INDEX IF EXISTS idx_reviews_execution;
DROP INDEX IF EXISTS idx_test_results_phase;
DROP INDEX IF EXISTS idx_test_results_execution;
DROP INDEX IF EXISTS idx_patches_review;
DROP INDEX IF EXISTS idx_patches_execution;
DROP INDEX IF EXISTS idx_execution_steps_status;
DROP INDEX IF EXISTS idx_execution_steps_execution;
DROP INDEX IF EXISTS idx_executions_repository;
DROP INDEX IF EXISTS idx_executions_created_at;
DROP INDEX IF EXISTS idx_executions_status;

-- Drop tables in reverse order of dependencies
DROP TABLE IF EXISTS event_log;
DROP TABLE IF EXISTS kill_switch_state;
DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS test_results;
DROP TABLE IF EXISTS patches;
DROP TABLE IF EXISTS execution_steps;
DROP TABLE IF EXISTS executions;
