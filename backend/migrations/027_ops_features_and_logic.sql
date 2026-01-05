-- ============================================
-- 运维监控系统 v2 - 功能与逻辑层
-- ============================================
-- 整合来源:
--   - 原 026_ops_error_classification.sql (定时报告、告警规则)
--   - 原 027_ops_deep_monitoring.sql (辅助函数、视图)
--   - 原 028_ops_preaggregation_tables.sql (预聚合表)
--   - 原 035_add_group_availability_monitoring.sql (分组可用性监控)
--   - 原 037_group_availability_percentage_threshold.sql (百分比阈值)
--
-- 说明:
--   - 创建新功能表(定时报告、预聚合、分组可用性监控)
--   - 创建辅助函数和视图
--   - 插入默认配置数据
-- ============================================

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

-- ============================================
-- 1. 创建定时报告配置表 (来自 026)
-- ============================================

CREATE TABLE IF NOT EXISTS ops_scheduled_reports (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    schedule_cron VARCHAR(100) NOT NULL,
    report_type VARCHAR(50) NOT NULL,
    report_config JSONB,
    notification_channels JSONB,
    enabled BOOLEAN DEFAULT true,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ops_scheduled_reports_next_run
    ON ops_scheduled_reports(next_run_at) WHERE enabled = true;

COMMENT ON TABLE ops_scheduled_reports IS '定时报告配置表';
COMMENT ON COLUMN ops_scheduled_reports.schedule_cron IS 'Cron 表达式: 0 * * * * (每小时), 0 0 * * * (每天)';
COMMENT ON COLUMN ops_scheduled_reports.report_type IS '报告类型: hourly_summary, daily_summary, account_health';
COMMENT ON COLUMN ops_scheduled_reports.report_config IS '报告配置 JSON: {"sections": [...], "filters": {...}}';

-- ============================================
-- 2. 创建预聚合表 (来自 028)
-- ============================================

-- 小时级预聚合表
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

CREATE UNIQUE INDEX IF NOT EXISTS idx_ops_metrics_hourly_unique
    ON ops_metrics_hourly(bucket_start, platform);

CREATE INDEX IF NOT EXISTS idx_ops_metrics_hourly_bucket
    ON ops_metrics_hourly(bucket_start DESC);

CREATE INDEX IF NOT EXISTS idx_ops_metrics_hourly_platform
    ON ops_metrics_hourly(platform, bucket_start DESC);

COMMENT ON TABLE ops_metrics_hourly IS '小时级预聚合表 - 从 usage_logs 和 ops_error_logs 聚合生成';
COMMENT ON COLUMN ops_metrics_hourly.bucket_start IS '小时桶起始时间（UTC，整点）';
COMMENT ON COLUMN ops_metrics_hourly.platform IS '上游平台（openai/anthropic/gemini），空字符串表示全平台汇总';
COMMENT ON COLUMN ops_metrics_hourly.computed_at IS '聚合计算时间';

-- 天级预聚合表
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

CREATE UNIQUE INDEX IF NOT EXISTS idx_ops_metrics_daily_unique
    ON ops_metrics_daily(bucket_date, platform);

CREATE INDEX IF NOT EXISTS idx_ops_metrics_daily_date
    ON ops_metrics_daily(bucket_date DESC);

CREATE INDEX IF NOT EXISTS idx_ops_metrics_daily_platform
    ON ops_metrics_daily(platform, bucket_date DESC);

COMMENT ON TABLE ops_metrics_daily IS '天级预聚合表 - 从 ops_metrics_hourly 聚合生成';
COMMENT ON COLUMN ops_metrics_daily.bucket_date IS '日期（UTC）';
COMMENT ON COLUMN ops_metrics_daily.platform IS '上游平台（openai/anthropic/gemini），空字符串表示全平台汇总';
COMMENT ON COLUMN ops_metrics_daily.computed_at IS '聚合计算时间';

-- ============================================
-- 3. 创建分组可用性监控表 (来自 035, 037)
-- ============================================

-- 分组可用性监控配置表
CREATE TABLE IF NOT EXISTS ops_group_availability_configs (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,

    -- 监控配置
    enabled BOOLEAN NOT NULL DEFAULT false,
    min_available_accounts INT NOT NULL DEFAULT 1,

    -- 阈值模式与百分比阈值 (来自 037)
    threshold_mode VARCHAR(20) NOT NULL DEFAULT 'count',
    min_available_percentage DOUBLE PRECISION NOT NULL DEFAULT 0,

    -- 告警配置
    notify_email BOOLEAN NOT NULL DEFAULT true,

    -- 告警级别 (critical/warning/info)
    severity VARCHAR(20) NOT NULL DEFAULT 'warning',

    -- 冷却期（分钟）- 避免重复告警
    cooldown_minutes INT NOT NULL DEFAULT 30,

    -- 时间戳
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- 唯一约束：每个分组只能有一个配置
    UNIQUE(group_id),

    -- 约束检查 (来自 037)
    CONSTRAINT chk_ops_group_availability_configs_threshold_mode
        CHECK (threshold_mode IN ('count', 'percentage', 'both')),
    CONSTRAINT chk_ops_group_availability_configs_min_available_percentage
        CHECK (min_available_percentage >= 0 AND min_available_percentage <= 100)
);

CREATE INDEX IF NOT EXISTS idx_ops_group_availability_configs_group_id
    ON ops_group_availability_configs(group_id);
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_configs_enabled
    ON ops_group_availability_configs(enabled);

COMMENT ON TABLE ops_group_availability_configs IS '分组可用性监控配置';
COMMENT ON COLUMN ops_group_availability_configs.enabled IS '是否启用监控';
COMMENT ON COLUMN ops_group_availability_configs.min_available_accounts IS '最低可用账号数阈值';
COMMENT ON COLUMN ops_group_availability_configs.threshold_mode IS '阈值模式: count/percentage/both';
COMMENT ON COLUMN ops_group_availability_configs.min_available_percentage IS '最低可用账号占比阈值(0-100)，0 表示未启用该阈值';
COMMENT ON COLUMN ops_group_availability_configs.severity IS '告警级别: critical/warning/info';
COMMENT ON COLUMN ops_group_availability_configs.cooldown_minutes IS '冷却期（分钟），避免重复告警';

-- 分组可用性告警事件表
CREATE TABLE IF NOT EXISTS ops_group_availability_events (
    id BIGSERIAL PRIMARY KEY,
    config_id BIGINT NOT NULL REFERENCES ops_group_availability_configs(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,

    -- 告警状态 (firing/resolved)
    status VARCHAR(20) NOT NULL DEFAULT 'firing',
    severity VARCHAR(20) NOT NULL,

    -- 告警详情
    title TEXT NOT NULL,
    description TEXT NOT NULL,

    -- 指标值
    available_accounts INT NOT NULL,
    threshold_accounts INT NOT NULL,
    total_accounts INT NOT NULL,

    -- 通知状态
    email_sent BOOLEAN NOT NULL DEFAULT false,

    -- 时间戳
    fired_at TIMESTAMP NOT NULL,
    resolved_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_ops_group_availability_events_status
        CHECK (status IN ('firing', 'resolved'))
);

CREATE INDEX IF NOT EXISTS idx_ops_group_availability_events_config_id
    ON ops_group_availability_events(config_id);
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_events_group_id
    ON ops_group_availability_events(group_id);
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_events_status
    ON ops_group_availability_events(status);
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_events_fired_at
    ON ops_group_availability_events(fired_at DESC);

COMMENT ON TABLE ops_group_availability_events IS '分组可用性告警事件';
COMMENT ON COLUMN ops_group_availability_events.status IS '告警状态: firing/resolved';
COMMENT ON COLUMN ops_group_availability_events.available_accounts IS '当前可用账号数';
COMMENT ON COLUMN ops_group_availability_events.threshold_accounts IS '阈值账号数';
COMMENT ON COLUMN ops_group_availability_events.total_accounts IS '总账号数';

-- ============================================
-- 4. 创建辅助函数 (来自 027)
-- ============================================

-- 计算延迟各阶段占比
CREATE OR REPLACE FUNCTION calculate_latency_breakdown(
    p_auth_latency BIGINT,
    p_routing_latency BIGINT,
    p_upstream_latency BIGINT,
    p_response_latency BIGINT
) RETURNS JSONB AS $$
DECLARE
    total_latency BIGINT;
    result JSONB;
BEGIN
    -- 计算总延迟
    total_latency := COALESCE(p_auth_latency, 0) +
                     COALESCE(p_routing_latency, 0) +
                     COALESCE(p_upstream_latency, 0) +
                     COALESCE(p_response_latency, 0);

    -- 避免除零
    IF total_latency = 0 THEN
        RETURN '{"auth_pct": 0, "routing_pct": 0, "upstream_pct": 0, "response_pct": 0, "total_ms": 0}'::JSONB;
    END IF;

    -- 计算各阶段占比
    result := jsonb_build_object(
        'auth_ms', COALESCE(p_auth_latency, 0),
        'routing_ms', COALESCE(p_routing_latency, 0),
        'upstream_ms', COALESCE(p_upstream_latency, 0),
        'response_ms', COALESCE(p_response_latency, 0),
        'total_ms', total_latency,
        'auth_pct', ROUND((COALESCE(p_auth_latency, 0)::DECIMAL / total_latency) * 100, 2),
        'routing_pct', ROUND((COALESCE(p_routing_latency, 0)::DECIMAL / total_latency) * 100, 2),
        'upstream_pct', ROUND((COALESCE(p_upstream_latency, 0)::DECIMAL / total_latency) * 100, 2),
        'response_pct', ROUND((COALESCE(p_response_latency, 0)::DECIMAL / total_latency) * 100, 2)
    );

    RETURN result;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

COMMENT ON FUNCTION calculate_latency_breakdown IS '计算延迟各阶段占比 - 用于生成延迟瀑布图数据';

-- ============================================
-- 5. 创建视图 (来自 026, 027)
-- ============================================

-- 错误详情下钻视图（四层监控）
CREATE OR REPLACE VIEW ops_error_detail_view AS
SELECT
    -- 基本信息
    e.id,
    e.request_id,
    e.created_at,
    e.error_phase,
    e.error_type,
    e.severity,
    e.status_code,
    e.error_message,
    e.duration_ms,

    -- L2层: API请求信息
    e.request_path,
    e.stream,
    e.time_to_first_token_ms,
    e.auth_latency_ms,
    e.routing_latency_ms,
    e.upstream_latency_ms,
    e.response_latency_ms,
    e.request_body,

    -- L3层: 上游信息
    e.platform,
    e.model,
    e.account_id,
    a.name AS account_name,
    a.type AS auth_type,
    a.status AS account_status,

    -- L4层: 客户信息
    e.user_id,
    u.email AS user_email,
    e.client_ip,
    e.user_agent,
    e.api_key_id,
    k.name AS api_key_name,
    e.group_id,
    g.name AS group_name,

    -- 重试信息
    e.is_retryable,
    e.retry_count
FROM ops_error_logs e
LEFT JOIN accounts a ON e.account_id = a.id
LEFT JOIN users u ON e.user_id = u.id
LEFT JOIN api_keys k ON e.api_key_id = k.id
LEFT JOIN groups g ON e.group_id = g.id;

COMMENT ON VIEW ops_error_detail_view IS '错误详情下钻视图 - 整合L1~L4层监控信息,用于快速定位问题根因';

-- 按错误来源统计视图
CREATE OR REPLACE VIEW ops_error_stats_by_source AS
SELECT
    DATE_TRUNC('hour', created_at) AS hour,
    error_source,
    error_owner,
    platform,
    COUNT(*) AS error_count,
    COUNT(DISTINCT user_id) AS affected_users,
    COUNT(DISTINCT account_id) AS affected_accounts,
    AVG(duration_ms) AS avg_duration_ms
FROM ops_error_logs
WHERE created_at >= NOW() - INTERVAL '24 hours'
GROUP BY DATE_TRUNC('hour', created_at), error_source, error_owner, platform
ORDER BY hour DESC, error_count DESC;

COMMENT ON VIEW ops_error_stats_by_source IS '按错误来源和责任方统计的错误趋势（24小时）';

-- ============================================
-- 6. 插入默认告警规则 (来自 026)
-- ============================================

INSERT INTO ops_alert_rules (
    name, alert_category, metric_type, operator, severity, filter_conditions, aggregation_dimensions,
    threshold, window_minutes, sustained_minutes, cooldown_minutes,
    notification_frequency, notification_channels, enabled
) VALUES
(
    '上游账号认证失败告警',
    'account_status',
    'error_count',
    '>',
    'P1',
    '{"error_source": ["upstream_business"], "error_type": ["authentication_error"], "account_status": ["auth_failed"]}'::jsonb,
    ARRAY['platform', 'account_id'],
    3,
    5,
    1,
    30,
    'throttled_5m',
    '{"email": {"enabled": true, "recipients": ["ops@example.com"]}}'::jsonb,
    false
),
(
    '上游账号限流告警',
    'account_status',
    'error_rate',
    '>',
    'P1',
    '{"error_source": ["upstream_business"], "account_status": ["rate_limited"]}'::jsonb,
    ARRAY['account_id'],
    50,
    10,
    5,
    60,
    'throttled_1h',
    '{"email": {"enabled": true, "recipients": ["ops@example.com"]}}'::jsonb,
    false
),
(
    '基础设施错误告警',
    'error_count',
    'error_count',
    '>',
    'P0',
    '{"error_source": ["infrastructure"]}'::jsonb,
    ARRAY['error_type'],
    10,
    5,
    2,
    15,
    'immediate',
    '{"email": {"enabled": true, "recipients": ["ops@example.com"]}}'::jsonb,
    false
)
ON CONFLICT DO NOTHING;

-- ============================================
-- 7. 插入默认定时报告 (来自 026)
-- ============================================

INSERT INTO ops_scheduled_reports (
    name, description, schedule_cron, report_type, report_config, notification_channels, enabled
) VALUES
(
    '每小时监控摘要',
    '每小时发送系统监控摘要，包括错误统计、账号状态、性能指标',
    '0 * * * *',
    'hourly_summary',
    '{"sections": ["error_stats", "account_status", "performance"]}'::jsonb,
    '{"email": {"enabled": true, "recipients": ["ops@example.com"]}}'::jsonb,
    false
),
(
    '每日监控报告',
    '每天早上8点发送详细的监控报告',
    '0 8 * * *',
    'daily_summary',
    '{"sections": ["error_stats", "account_status", "performance", "cost_analysis"]}'::jsonb,
    '{"email": {"enabled": true, "recipients": ["ops@example.com"]}}'::jsonb,
    false
)
ON CONFLICT DO NOTHING;

-- ============================================
-- 完成
-- ============================================

DO $$
BEGIN
    RAISE NOTICE '========================================';
    RAISE NOTICE '运维监控系统 v2 功能层创建完成';
    RAISE NOTICE '========================================';
    RAISE NOTICE '1. ops_scheduled_reports 表创建: ✓';
    RAISE NOTICE '2. ops_metrics_hourly/daily 预聚合表创建: ✓';
    RAISE NOTICE '3. ops_group_availability_* 表创建: ✓';
    RAISE NOTICE '4. calculate_latency_breakdown 函数创建: ✓';
    RAISE NOTICE '5. ops_error_detail_view 等视图创建: ✓';
    RAISE NOTICE '6. 默认告警规则插入: ✓ (3条)';
    RAISE NOTICE '7. 默认定时报告插入: ✓ (2条)';
    RAISE NOTICE '========================================';
END $$;
