-- Split dashboard aggregates by wallet vs subscription actual cost.
-- Rewritten from personal-dev 153 to match current usage_dashboard_* schema
-- (account_cost already exists via 107). Recharge/refund stay live queries
-- against payment_orders / payment_audit_logs rather than pre-aggregation.
--
-- Hourly backfill uses stored bucket_start windows (timezone-independent).
-- Daily is summed from those hourly rows using the same date already stored
-- on usage_dashboard_daily, avoiding a second usage_logs scan in a different TZ.

ALTER TABLE usage_dashboard_hourly
    ADD COLUMN IF NOT EXISTS balance_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS subscription_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0;

ALTER TABLE usage_dashboard_daily
    ADD COLUMN IF NOT EXISTS balance_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS subscription_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0;

COMMENT ON COLUMN usage_dashboard_hourly.balance_actual_cost IS 'SUM(actual_cost) where billing_type = 0 (wallet)';
COMMENT ON COLUMN usage_dashboard_hourly.subscription_actual_cost IS 'SUM(actual_cost) where billing_type = 1 (subscription)';
COMMENT ON COLUMN usage_dashboard_daily.balance_actual_cost IS 'SUM(actual_cost) where billing_type = 0 (wallet)';
COMMENT ON COLUMN usage_dashboard_daily.subscription_actual_cost IS 'SUM(actual_cost) where billing_type = 1 (subscription)';

WITH hourly_split AS (
    SELECT
        h.bucket_start,
        COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.billing_type = 0), 0) AS balance_actual_cost,
        COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.billing_type = 1), 0) AS subscription_actual_cost
    FROM usage_dashboard_hourly h
    LEFT JOIN usage_logs ul
        ON ul.created_at >= h.bucket_start
       AND ul.created_at < h.bucket_start + INTERVAL '1 hour'
    GROUP BY h.bucket_start
)
UPDATE usage_dashboard_hourly h
SET balance_actual_cost = s.balance_actual_cost,
    subscription_actual_cost = s.subscription_actual_cost,
    computed_at = NOW()
FROM hourly_split s
WHERE h.bucket_start = s.bucket_start;

-- Daily rows store app-local dates (default Asia/Shanghai, matching timezone.Init).
-- Interpret bucket_date as that zone rather than the migrator session TimeZone.
WITH daily_split AS (
    SELECT
        d.bucket_date,
        COALESCE(SUM(h.balance_actual_cost), 0) AS balance_actual_cost,
        COALESCE(SUM(h.subscription_actual_cost), 0) AS subscription_actual_cost
    FROM usage_dashboard_daily d
    JOIN usage_dashboard_hourly h
        ON h.bucket_start >= (d.bucket_date::timestamp AT TIME ZONE 'Asia/Shanghai')
       AND h.bucket_start < ((d.bucket_date + 1)::timestamp AT TIME ZONE 'Asia/Shanghai')
    GROUP BY d.bucket_date
)
UPDATE usage_dashboard_daily d
SET balance_actual_cost = s.balance_actual_cost,
    subscription_actual_cost = s.subscription_actual_cost,
    computed_at = NOW()
FROM daily_split s
WHERE d.bucket_date = s.bucket_date;
