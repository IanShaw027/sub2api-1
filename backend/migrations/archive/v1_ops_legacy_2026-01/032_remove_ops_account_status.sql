-- Migration 032: Remove unused ops_account_status table
-- Purpose: Clean up the deprecated ops_account_status table that was never used
-- in production. Real-time account status is now calculated via GetAllActiveAccountStatus()
-- which queries ops_error_logs and usage_logs directly.

-- Drop the unused ops_account_status table
DROP TABLE IF EXISTS ops_account_status CASCADE;

-- Add composite indexes for real-time account status query optimization
-- These indexes support the GetAllActiveAccountStatus query that aggregates
-- error_logs and usage_logs for active accounts
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_account_created
ON ops_error_logs(account_id, created_at DESC) WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_usage_logs_account_created
ON usage_logs(account_id, created_at DESC) WHERE account_id IS NOT NULL;
