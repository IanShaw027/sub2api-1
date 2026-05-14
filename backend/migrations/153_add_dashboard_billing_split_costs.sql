-- Add balance/subscription actual-cost split columns to dashboard aggregation tables
-- so admin dashboard totals remain consistent after usage_logs retention cleanup.

ALTER TABLE usage_dashboard_hourly
    ADD COLUMN IF NOT EXISTS balance_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS subscription_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0;

ALTER TABLE usage_dashboard_daily
    ADD COLUMN IF NOT EXISTS balance_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS subscription_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0;
