-- Persist sealed hourly OpenAI OAuth capacity facts so trend charts avoid re-scanning usage_logs.

CREATE TABLE IF NOT EXISTS openai_oauth_capacity_hourly (
    bucket_start     TIMESTAMPTZ NOT NULL,
    -- -1 = all groups; 0 remains the real "ungrouped" request-time bucket.
    group_id         BIGINT NOT NULL DEFAULT -1,
    -- Reserved for future plan-scoped series; currently always 'all'.
    plan_type        TEXT NOT NULL DEFAULT 'all',
    spent_usd        NUMERIC(20, 10) NOT NULL DEFAULT 0,
    -- Optional window metrics frozen at seal time. NULL means unknown — UI must omit.
    capacity_5h_usd  NUMERIC(20, 10),
    available_5h_usd NUMERIC(20, 10),
    used_percent_5h  NUMERIC(8, 4),
    capacity_7d_usd  NUMERIC(20, 10),
    available_7d_usd NUMERIC(20, 10),
    used_percent_7d  NUMERIC(8, 4),
    -- true once the hour has fully ended and values are frozen (no re-query).
    sealed           BOOLEAN NOT NULL DEFAULT FALSE,
    source           TEXT NOT NULL DEFAULT 'usage_logs',
    computed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bucket_start, group_id, plan_type)
);

CREATE INDEX IF NOT EXISTS idx_openai_oauth_capacity_hourly_group_bucket
    ON openai_oauth_capacity_hourly (group_id, bucket_start DESC);

CREATE INDEX IF NOT EXISTS idx_openai_oauth_capacity_hourly_sealed
    ON openai_oauth_capacity_hourly (sealed, bucket_start DESC)
    WHERE sealed = TRUE;

COMMENT ON TABLE openai_oauth_capacity_hourly IS
    'Sealed hourly OpenAI OAuth account-cost spend (and optional available/capacity) for capacity trend charts.';
COMMENT ON COLUMN openai_oauth_capacity_hourly.group_id IS
    '-1 = global total; 0 = ungrouped; positive values are usage_logs.group_id at request time.';
COMMENT ON COLUMN openai_oauth_capacity_hourly.sealed IS
    'Completed hours are sealed and served from this table without re-aggregating usage_logs.';
COMMENT ON COLUMN openai_oauth_capacity_hourly.available_5h_usd IS
    'NULL when capacity could not be inferred at seal time; clients must not invent a value.';
