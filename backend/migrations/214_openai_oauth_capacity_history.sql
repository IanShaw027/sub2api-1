-- Persist OpenAI OAuth quota telemetry so capacity estimates survive rolling-window resets.

CREATE TABLE IF NOT EXISTS openai_oauth_quota_snapshots (
    id                         BIGSERIAL PRIMARY KEY,
    account_id                 BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    plan_type                  TEXT NOT NULL DEFAULT 'unknown',
    observed_at                TIMESTAMPTZ NOT NULL,
    five_hour_used_percent     NUMERIC(8, 4),
    five_hour_reset_at         TIMESTAMPTZ,
    five_hour_window_minutes   INT,
    seven_day_used_percent     NUMERIC(8, 4),
    seven_day_reset_at         TIMESTAMPTZ,
    seven_day_window_minutes   INT,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, observed_at)
);

CREATE INDEX IF NOT EXISTS idx_openai_oauth_quota_snapshots_observed
    ON openai_oauth_quota_snapshots (observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_openai_oauth_quota_snapshots_account_reset
    ON openai_oauth_quota_snapshots (account_id, five_hour_reset_at, seven_day_reset_at);

CREATE TABLE IF NOT EXISTS openai_oauth_quota_periods (
    id                         BIGSERIAL PRIMARY KEY,
    account_id                 BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    plan_type                  TEXT NOT NULL DEFAULT 'unknown',
    window_kind                TEXT NOT NULL CHECK (window_kind IN ('5h', '7d')),
    period_start               TIMESTAMPTZ NOT NULL,
    period_end                 TIMESTAMPTZ NOT NULL,
    final_used_percent         NUMERIC(8, 4),
    spend_usd                  NUMERIC(20, 10) NOT NULL DEFAULT 0,
    inferred_capacity_usd      NUMERIC(20, 10),
    sample_count               INT NOT NULL DEFAULT 0,
    closed_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, window_kind, period_end)
);

CREATE INDEX IF NOT EXISTS idx_openai_oauth_quota_periods_plan_window
    ON openai_oauth_quota_periods (plan_type, window_kind, period_end DESC);

-- Seed one snapshot from the latest account state. Future writes are coalesced into 15-minute buckets.
INSERT INTO openai_oauth_quota_snapshots (
    account_id,
    plan_type,
    observed_at,
    five_hour_used_percent,
    five_hour_reset_at,
    five_hour_window_minutes,
    seven_day_used_percent,
    seven_day_reset_at,
    seven_day_window_minutes
)
SELECT
    a.id,
    COALESCE(NULLIF(a.credentials->>'plan_type', ''), NULLIF(a.extra->>'subscription_type', ''), 'unknown'),
    date_bin(
        INTERVAL '15 minutes',
        COALESCE(NULLIF(a.extra->>'codex_usage_updated_at', '')::timestamptz, a.updated_at),
        TIMESTAMPTZ '2000-01-01 00:00:00+00'
    ),
    NULLIF(a.extra->>'codex_5h_used_percent', '')::numeric,
    NULLIF(a.extra->>'codex_5h_reset_at', '')::timestamptz,
    COALESCE(NULLIF(a.extra->>'codex_5h_window_minutes', '')::int, 300),
    NULLIF(a.extra->>'codex_7d_used_percent', '')::numeric,
    NULLIF(a.extra->>'codex_7d_reset_at', '')::timestamptz,
    COALESCE(NULLIF(a.extra->>'codex_7d_window_minutes', '')::int, 10080)
FROM accounts a
WHERE a.deleted_at IS NULL
  AND a.platform = 'openai'
  AND a.type IN ('oauth', 'setup-token')
  AND (a.extra ? 'codex_5h_used_percent' OR a.extra ? 'codex_7d_used_percent')
ON CONFLICT (account_id, observed_at) DO NOTHING;
