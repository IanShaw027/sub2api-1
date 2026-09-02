CREATE TABLE IF NOT EXISTS account_seven_day_forecast_snapshots (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    window_start TIMESTAMPTZ NOT NULL,
    bucket SMALLINT NOT NULL CHECK (bucket BETWEEN 10 AND 100 AND bucket % 10 = 0),
    utilization DOUBLE PRECISION NOT NULL CHECK (utilization > 0),
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, window_start, bucket)
);

CREATE INDEX IF NOT EXISTS idx_account_7d_forecast_latest
    ON account_seven_day_forecast_snapshots (account_id, observed_at DESC);

COMMENT ON TABLE account_seven_day_forecast_snapshots IS
    'First observed upstream 7d utilization sample for each 10 percent bucket and quota window';
