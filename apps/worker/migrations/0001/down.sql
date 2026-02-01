-- Rollback migration: Drop workspaces and executions tables

DROP INDEX IF EXISTS idx_executions_repository;
DROP INDEX IF EXISTS idx_executions_status;
DROP TABLE IF EXISTS executions;

DROP INDEX IF EXISTS idx_workspaces_status_last_accessed;
DROP INDEX IF EXISTS idx_workspaces_status;
DROP TABLE IF EXISTS workspaces;
