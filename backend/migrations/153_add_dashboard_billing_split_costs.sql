-- Add balance/subscription actual-cost split columns to dashboard aggregation tables
-- so admin dashboard totals remain consistent after usage_logs retention cleanup.

ALTER TABLE usage_dashboard_hourly
    ADD COLUMN IF NOT EXISTS balance_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS subscription_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0;

ALTER TABLE usage_dashboard_daily
    ADD COLUMN IF NOT EXISTS balance_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS subscription_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0;

WITH hourly_split AS (
    SELECT
        date_trunc('hour', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS bucket_start,
        COALESCE(SUM(actual_cost) FILTER (WHERE billing_type = 0), 0) AS balance_actual_cost,
        COALESCE(SUM(actual_cost) FILTER (WHERE billing_type = 1), 0) AS subscription_actual_cost
    FROM usage_logs
    GROUP BY 1
)
UPDATE usage_dashboard_hourly h
SET balance_actual_cost = s.balance_actual_cost,
    subscription_actual_cost = s.subscription_actual_cost,
    computed_at = NOW()
FROM hourly_split s
WHERE h.bucket_start = s.bucket_start;

WITH daily_split AS (
    SELECT
        (created_at AT TIME ZONE 'UTC')::date AS bucket_date,
        COALESCE(SUM(actual_cost) FILTER (WHERE billing_type = 0), 0) AS balance_actual_cost,
        COALESCE(SUM(actual_cost) FILTER (WHERE billing_type = 1), 0) AS subscription_actual_cost
    FROM usage_logs
    GROUP BY 1
)
UPDATE usage_dashboard_daily d
SET balance_actual_cost = s.balance_actual_cost,
    subscription_actual_cost = s.subscription_actual_cost,
    computed_at = NOW()
FROM daily_split s
WHERE d.bucket_date = s.bucket_date;
