CREATE TABLE IF NOT EXISTS usage_user_daily_cost (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bucket_date DATE NOT NULL,
    actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,
    balance_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,
    subscription_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, bucket_date)
);

CREATE INDEX IF NOT EXISTS idx_usage_user_daily_cost_bucket_date
    ON usage_user_daily_cost (bucket_date);

COMMENT ON TABLE usage_user_daily_cost IS '用户日用量汇总，供管理端近 30 天用量排序';
