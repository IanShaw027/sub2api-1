-- 每用户·每业务日 的用量成本预聚合表（选项 B：与 dashboard 增量聚合管线解耦的独立幂等 rollup）。
-- 用途：管理后台按用量排序用户列表时，避免每次翻页对 usage_logs 近 30 天全表按 user 实时聚合。
-- 维护：UserDailyCostAggregator 按业务日「全量重算」（同一业务日 DELETE 后 INSERT，幂等、无双计费），
--       bucket_date 为业务时区下的日期；成本口径与 usage_logs.actual_cost 一致（DECIMAL(20,10)）。
-- 保留：随排序仅用近 30 天，旧行由聚合器按保留窗口清理（见 aggregator cleanup）。

CREATE TABLE IF NOT EXISTS usage_user_daily_cost (
    user_id                  BIGINT      NOT NULL,
    bucket_date              DATE        NOT NULL,                       -- 业务时区下的日期
    actual_cost              DECIMAL(20, 10) NOT NULL DEFAULT 0,         -- 当日该用户 actual_cost 合计
    balance_actual_cost      DECIMAL(20, 10) NOT NULL DEFAULT 0,         -- 其中 billing_type=余额(0)
    subscription_actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,         -- 其中 billing_type=订阅(1)
    computed_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, bucket_date)
);

-- 排序 CTE 对近 30 天全部用户按 user 汇总（WHERE bucket_date >= cutoff），此索引服务该范围扫描与按日清理。
CREATE INDEX IF NOT EXISTS idx_usage_user_daily_cost_bucket_date
    ON usage_user_daily_cost (bucket_date);

COMMENT ON TABLE usage_user_daily_cost IS 'Per-user per-business-day usage cost rollup for admin user-list usage sorting (idempotent recompute).';
