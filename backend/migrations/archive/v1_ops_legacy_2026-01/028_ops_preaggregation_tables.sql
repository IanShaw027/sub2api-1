-- 运维监控预聚合表
-- 创建时间: 2026-01-03
-- 说明: 创建小时级和天级预聚合表，提升大时间范围查询性能

-- ============================================
-- 1. 创建小时级预聚合表
-- ============================================

CREATE TABLE IF NOT EXISTS ops_metrics_hourly (
    id BIGSERIAL PRIMARY KEY,
    bucket_start TIMESTAMPTZ NOT NULL,
    platform VARCHAR(50) NOT NULL DEFAULT '',

    -- 请求统计
    request_count BIGINT DEFAULT 0,
    success_count BIGINT DEFAULT 0,
    error_count BIGINT DEFAULT 0,
    error_4xx_count BIGINT DEFAULT 0,
    error_5xx_count BIGINT DEFAULT 0,
    timeout_count BIGINT DEFAULT 0,

    -- 延迟统计
    avg_latency_ms DECIMAL(10,2),
    p99_latency_ms DECIMAL(10,2),

    -- 错误率
    error_rate DECIMAL(5,2),

    -- 元数据
    computed_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 创建唯一索引（确保每个小时+平台组合只有一条记录）
CREATE UNIQUE INDEX IF NOT EXISTS idx_ops_metrics_hourly_unique
    ON ops_metrics_hourly(bucket_start, platform);

-- 创建查询索引
CREATE INDEX IF NOT EXISTS idx_ops_metrics_hourly_bucket
    ON ops_metrics_hourly(bucket_start DESC);

CREATE INDEX IF NOT EXISTS idx_ops_metrics_hourly_platform
    ON ops_metrics_hourly(platform, bucket_start DESC);

-- 添加注释
COMMENT ON TABLE ops_metrics_hourly IS '小时级预聚合表 - 从 usage_logs 和 ops_error_logs 聚合生成';
COMMENT ON COLUMN ops_metrics_hourly.bucket_start IS '小时桶起始时间（UTC，整点）';
COMMENT ON COLUMN ops_metrics_hourly.platform IS '上游平台（openai/anthropic/gemini），空字符串表示全平台汇总';
COMMENT ON COLUMN ops_metrics_hourly.computed_at IS '聚合计算时间';

-- ============================================
-- 2. 创建天级预聚合表
-- ============================================

CREATE TABLE IF NOT EXISTS ops_metrics_daily (
    id BIGSERIAL PRIMARY KEY,
    bucket_date DATE NOT NULL,
    platform VARCHAR(50) NOT NULL DEFAULT '',

    -- 请求统计
    request_count BIGINT DEFAULT 0,
    success_count BIGINT DEFAULT 0,
    error_count BIGINT DEFAULT 0,
    error_4xx_count BIGINT DEFAULT 0,
    error_5xx_count BIGINT DEFAULT 0,
    timeout_count BIGINT DEFAULT 0,

    -- 延迟统计
    avg_latency_ms DECIMAL(10,2),
    p99_latency_ms DECIMAL(10,2),

    -- 错误率
    error_rate DECIMAL(5,2),

    -- 元数据
    computed_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 创建唯一索引（确保每天+平台组合只有一条记录）
CREATE UNIQUE INDEX IF NOT EXISTS idx_ops_metrics_daily_unique
    ON ops_metrics_daily(bucket_date, platform);

-- 创建查询索引
CREATE INDEX IF NOT EXISTS idx_ops_metrics_daily_date
    ON ops_metrics_daily(bucket_date DESC);

CREATE INDEX IF NOT EXISTS idx_ops_metrics_daily_platform
    ON ops_metrics_daily(platform, bucket_date DESC);

-- 添加注释
COMMENT ON TABLE ops_metrics_daily IS '天级预聚合表 - 从 ops_metrics_hourly 聚合生成';
COMMENT ON COLUMN ops_metrics_daily.bucket_date IS '日期（UTC）';
COMMENT ON COLUMN ops_metrics_daily.platform IS '上游平台（openai/anthropic/gemini），空字符串表示全平台汇总';
COMMENT ON COLUMN ops_metrics_daily.computed_at IS '聚合计算时间';

-- ============================================
-- 3. 更新数据保留策略配置
-- ============================================

INSERT INTO ops_data_retention_config (table_name, retention_days, enabled) VALUES
    ('ops_metrics_hourly', 30, true),  -- 小时级数据保留30天
    ('ops_metrics_daily', 365, true)   -- 天级数据保留1年
ON CONFLICT (table_name) DO NOTHING;

-- ============================================
-- 完成
-- ============================================

DO $$
BEGIN
    RAISE NOTICE '========================================';
    RAISE NOTICE '预聚合表创建完成';
    RAISE NOTICE '========================================';
    RAISE NOTICE '1. ops_metrics_hourly 表创建: ✓';
    RAISE NOTICE '   - 小时级聚合，支持按平台维度统计';
    RAISE NOTICE '   - 保留30天';
    RAISE NOTICE '';
    RAISE NOTICE '2. ops_metrics_daily 表创建: ✓';
    RAISE NOTICE '   - 天级聚合，从小时级数据汇总';
    RAISE NOTICE '   - 保留1年';
    RAISE NOTICE '';
    RAISE NOTICE '3. 数据保留策略配置: ✓';
    RAISE NOTICE '========================================';
END $$;
