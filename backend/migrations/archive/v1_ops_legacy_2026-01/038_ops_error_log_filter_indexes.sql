-- +goose Up
-- Additional indexes to speed up common ops_error_logs filters used by the Ops dashboard.

-- error_phase filter (auth/network/upstream/etc) + time ordering
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_phase_time
ON ops_error_logs(error_phase, created_at DESC);

-- status_code filter + time ordering (e.g. 5xx / 429)
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_status_code_time
ON ops_error_logs(status_code, created_at DESC);

-- group_id filter + time ordering (only when group_id is present)
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_group_id_time
ON ops_error_logs(group_id, created_at DESC)
WHERE group_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_ops_error_logs_phase_time;
DROP INDEX IF EXISTS idx_ops_error_logs_status_code_time;
DROP INDEX IF EXISTS idx_ops_error_logs_group_id_time;

