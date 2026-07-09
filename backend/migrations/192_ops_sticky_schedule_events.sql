-- 192_ops_sticky_schedule_events.sql
-- Structured telemetry for sticky-session scheduling outcomes used by Ops trends.

CREATE TABLE IF NOT EXISTS ops_sticky_schedule_events (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  platform TEXT NOT NULL DEFAULT '',
  group_id BIGINT,
  api_key_id BIGINT NOT NULL DEFAULT 0,
  sticky_account_id BIGINT NOT NULL,
  selected_account_id BIGINT,
  sticky_original_unavailable BOOLEAN NOT NULL DEFAULT FALSE,
  reason TEXT NOT NULL DEFAULT '',
  sticky_source TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_ops_sticky_schedule_events_created_at
  ON ops_sticky_schedule_events (created_at);

CREATE INDEX IF NOT EXISTS idx_ops_sticky_schedule_events_platform_group_created_at
  ON ops_sticky_schedule_events (platform, group_id, created_at);

CREATE INDEX IF NOT EXISTS idx_ops_sticky_schedule_events_sticky_account_created_at
  ON ops_sticky_schedule_events (sticky_account_id, created_at);
