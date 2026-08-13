-- Generic multi-platform account capacity forecasting.
--
-- Creates platform-agnostic tables used by CapacityForecastService:
--   account_quota_snapshots  — latest rolling-window snapshot per (platform, account, window_kind)
--   account_quota_periods    — archived (closed) windows for capacity-inference fallback
--   capacity_hourly          — sealed hourly spend/available/capacity facts per (platform, group_id)
--
-- Idempotent: CREATE TABLE / INDEX IF NOT EXISTS.
-- No backfill from openai_oauth_* tables (those do not exist on this branch).

-- account_quota_snapshots holds the *latest* known rolling-window snapshot per
-- (platform, account, window_kind) — not a full history. History lives in
-- account_quota_periods (closed windows) and capacity_hourly (sealed hourly facts).
CREATE TABLE IF NOT EXISTS account_quota_snapshots (
    id             BIGSERIAL PRIMARY KEY,
    platform       TEXT NOT NULL,
    account_id     BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    window_kind    TEXT NOT NULL,
    used_ratio     DOUBLE PRECISION NOT NULL DEFAULT 0,
    reset_at       TIMESTAMPTZ,
    window_minutes INT,
    observed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (platform, account_id, window_kind)
);

CREATE INDEX IF NOT EXISTS idx_account_quota_snapshots_platform_account
    ON account_quota_snapshots (platform, account_id);

-- account_quota_periods archives a rolling window once it closes (reset_at crossed),
-- freezing the final used_ratio/spend so capacity can still be inferred even when a
-- probe finds used_ratio too low (<5%) to infer capacity for the *current* window.
CREATE TABLE IF NOT EXISTS account_quota_periods (
    id                    BIGSERIAL PRIMARY KEY,
    platform              TEXT NOT NULL,
    account_id            BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    window_kind           TEXT NOT NULL,
    period_start          TIMESTAMPTZ NOT NULL,
    period_end            TIMESTAMPTZ NOT NULL,
    final_used_ratio      DOUBLE PRECISION,
    spend_usd             NUMERIC(20, 10) NOT NULL DEFAULT 0,
    inferred_capacity_usd NUMERIC(20, 10),
    sample_count          INT NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (platform, account_id, window_kind, period_start)
);

CREATE INDEX IF NOT EXISTS idx_account_quota_periods_platform_window
    ON account_quota_periods (platform, window_kind, period_end DESC);

-- capacity_hourly stores sealed hourly (platform, group) facts: actual spend plus the
-- best-effort available/capacity figures computed at seal time. group_id = NULL means
-- "platform total" (all groups); a real group is scoped by its numeric id.
CREATE TABLE IF NOT EXISTS capacity_hourly (
    id            BIGSERIAL PRIMARY KEY,
    platform      TEXT NOT NULL,
    group_id      BIGINT,
    bucket_start  TIMESTAMPTZ NOT NULL,
    spend_usd     NUMERIC(20, 10) NOT NULL DEFAULT 0,
    available_usd NUMERIC(20, 10),
    capacity_usd  NUMERIC(20, 10),
    sealed        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_capacity_hourly_platform_group_bucket
    ON capacity_hourly (platform, COALESCE(group_id, 0), bucket_start);

COMMENT ON TABLE account_quota_snapshots IS
    'Latest known rolling-window quota snapshot per (platform, account, window_kind).';
COMMENT ON TABLE account_quota_periods IS
    'Archived (closed) rolling quota windows, used as a capacity-inference fallback.';
COMMENT ON TABLE capacity_hourly IS
    'Sealed hourly spend/available/capacity facts per (platform, group_id). group_id NULL = platform total.';
