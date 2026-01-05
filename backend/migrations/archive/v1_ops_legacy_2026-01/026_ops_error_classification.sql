-- 错误分类和账号状态追踪系统
-- 创建时间: 2026-01-03
-- 说明: 实现完整的错误分类体系和账号状态监控

-- 设置超时以避免启动无限等待
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- ============================================
-- 1. 扩展 ops_error_logs 表
-- ============================================

-- 添加错误分类字段
ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS error_source VARCHAR(50),
    ADD COLUMN IF NOT EXISTS error_owner VARCHAR(50),
    ADD COLUMN IF NOT EXISTS account_status VARCHAR(50),
    ADD COLUMN IF NOT EXISTS upstream_status_code INT,
    ADD COLUMN IF NOT EXISTS upstream_error_message TEXT,
    ADD COLUMN IF NOT EXISTS upstream_error_detail TEXT,
    ADD COLUMN IF NOT EXISTS network_error_type VARCHAR(50),
    ADD COLUMN IF NOT EXISTS retry_after_seconds INT;

-- 添加注释
COMMENT ON COLUMN ops_error_logs.error_source IS '错误来源: downstream_business, downstream_system, upstream_business, upstream_system, infrastructure, internal';
COMMENT ON COLUMN ops_error_logs.error_owner IS '错误责任方: client, platform, provider, infrastructure';
COMMENT ON COLUMN ops_error_logs.account_status IS '账号状态: normal, auth_failed, permission_denied, rate_limited, quota_exceeded, disabled, error';
COMMENT ON COLUMN ops_error_logs.upstream_status_code IS '上游实际返回的 HTTP 状态码';
COMMENT ON COLUMN ops_error_logs.upstream_error_message IS '上游错误消息（原始）';
COMMENT ON COLUMN ops_error_logs.upstream_error_detail IS '上游错误详情（网络/超时错误的详细信息）';
COMMENT ON COLUMN ops_error_logs.network_error_type IS '网络错误类型: timeout, connection_refused, dns_error, etc';
COMMENT ON COLUMN ops_error_logs.retry_after_seconds IS '上游建议的重试等待时间（秒）';

-- ============================================
-- 2. 创建账号状态追踪表
-- ============================================

CREATE TABLE IF NOT EXISTS ops_account_status (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL,
    platform VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'normal',

    -- 最近错误信息
    last_error_type VARCHAR(100),
    last_error_message TEXT,
    last_error_time TIMESTAMPTZ,

    -- 统计指标（1小时窗口）
    error_count_1h INT NOT NULL DEFAULT 0 CHECK (error_count_1h >= 0),
    success_count_1h INT NOT NULL DEFAULT 0 CHECK (success_count_1h >= 0),
    timeout_count_1h INT NOT NULL DEFAULT 0 CHECK (timeout_count_1h >= 0),
    rate_limit_count_1h INT NOT NULL DEFAULT 0 CHECK (rate_limit_count_1h >= 0),

    -- 统计指标（24小时窗口）
    error_count_24h INT NOT NULL DEFAULT 0 CHECK (error_count_24h >= 0),
    success_count_24h INT NOT NULL DEFAULT 0 CHECK (success_count_24h >= 0),
    timeout_count_24h INT NOT NULL DEFAULT 0 CHECK (timeout_count_24h >= 0),
    rate_limit_count_24h INT NOT NULL DEFAULT 0 CHECK (rate_limit_count_24h >= 0),

    -- 最近成功时间
    last_success_time TIMESTAMPTZ,

    -- 状态变更时间
    status_changed_at TIMESTAMPTZ,

    -- 时间戳
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(account_id, platform)
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_ops_account_status_platform ON ops_account_status(platform);
CREATE INDEX IF NOT EXISTS idx_ops_account_status_status ON ops_account_status(status);
CREATE INDEX IF NOT EXISTS idx_ops_account_status_updated ON ops_account_status(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_ops_account_status_error_count_1h ON ops_account_status(error_count_1h DESC);

-- 添加注释
COMMENT ON TABLE ops_account_status IS '账号状态追踪表，实时监控每个上游账号的健康状态';
COMMENT ON COLUMN ops_account_status.status IS '账号状态: normal, auth_failed, permission_denied, rate_limited, quota_exceeded, disabled, error';
COMMENT ON COLUMN ops_account_status.error_count_1h IS '过去1小时错误次数';
COMMENT ON COLUMN ops_account_status.error_count_24h IS '过去24小时错误次数';

-- ============================================
-- 3. 扩展 ops_alert_rules 表
-- ============================================

ALTER TABLE ops_alert_rules
    ADD COLUMN IF NOT EXISTS alert_category VARCHAR(50),
    ADD COLUMN IF NOT EXISTS filter_conditions JSONB,
    ADD COLUMN IF NOT EXISTS aggregation_dimensions TEXT[],
    ADD COLUMN IF NOT EXISTS notification_channels JSONB,
    ADD COLUMN IF NOT EXISTS notification_frequency VARCHAR(50) DEFAULT 'immediate',
    ADD COLUMN IF NOT EXISTS notification_template TEXT;

-- 添加注释
COMMENT ON COLUMN ops_alert_rules.alert_category IS '告警类别: error_rate, error_count, account_status, latency, availability, cost, scheduled_report';
COMMENT ON COLUMN ops_alert_rules.filter_conditions IS '过滤条件 JSON: {"error_source": ["upstream_business"], "platform": ["openai"]}';
COMMENT ON COLUMN ops_alert_rules.aggregation_dimensions IS '聚合维度: platform, error_type, error_source, account_id, user_id';
COMMENT ON COLUMN ops_alert_rules.notification_channels IS '通知渠道配置 JSON: {"email": {...}, "webhook": {...}}';
COMMENT ON COLUMN ops_alert_rules.notification_frequency IS '通知频率: immediate, throttled_5m, throttled_1h, daily_digest, hourly_digest';
COMMENT ON COLUMN ops_alert_rules.notification_template IS '通知模板（支持变量替换）';

-- ============================================
-- 4. 创建错误统计视图
-- ============================================

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
-- 5. 创建账号状态统计视图
-- ============================================

CREATE OR REPLACE VIEW ops_account_status_summary AS
SELECT
    platform,
    status,
    COUNT(*) AS account_count,
    SUM(error_count_1h) AS total_errors_1h,
    SUM(success_count_1h) AS total_success_1h,
    AVG(CASE
        WHEN (error_count_1h + success_count_1h) > 0
        THEN error_count_1h::DECIMAL / (error_count_1h + success_count_1h) * 100
        ELSE 0
    END) AS avg_error_rate_1h
FROM ops_account_status
GROUP BY platform, status
ORDER BY platform, status;

COMMENT ON VIEW ops_account_status_summary IS '账号状态汇总统计';

-- ============================================
-- 6. 创建告警通知历史表
-- ============================================

CREATE TABLE IF NOT EXISTS ops_alert_notifications (
    id BIGSERIAL PRIMARY KEY,
    alert_event_id BIGINT NOT NULL,
    rule_id BIGINT NOT NULL,
    channel_type VARCHAR(50) NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    sent_at TIMESTAMPTZ DEFAULT NOW(),
    error_message TEXT,
    retry_count INT NOT NULL DEFAULT 0 CHECK (retry_count >= 0),

    FOREIGN KEY (alert_event_id) REFERENCES ops_alert_events(id) ON DELETE CASCADE,
    FOREIGN KEY (rule_id) REFERENCES ops_alert_rules(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ops_alert_notifications_event ON ops_alert_notifications(alert_event_id);
CREATE INDEX IF NOT EXISTS idx_ops_alert_notifications_rule ON ops_alert_notifications(rule_id);
CREATE INDEX IF NOT EXISTS idx_ops_alert_notifications_sent ON ops_alert_notifications(sent_at DESC);

COMMENT ON TABLE ops_alert_notifications IS '告警通知发送历史记录';
COMMENT ON COLUMN ops_alert_notifications.channel_type IS '通知渠道: email, webhook, sms, slack';
COMMENT ON COLUMN ops_alert_notifications.status IS '发送状态: pending, sent, failed';

-- ============================================
-- 7. 创建定时报告配置表
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

CREATE INDEX IF NOT EXISTS idx_ops_scheduled_reports_next_run ON ops_scheduled_reports(next_run_at) WHERE enabled = true;

COMMENT ON TABLE ops_scheduled_reports IS '定时报告配置表';
COMMENT ON COLUMN ops_scheduled_reports.schedule_cron IS 'Cron 表达式: 0 * * * * (每小时), 0 0 * * * (每天)';
COMMENT ON COLUMN ops_scheduled_reports.report_type IS '报告类型: hourly_summary, daily_summary, account_health';
COMMENT ON COLUMN ops_scheduled_reports.report_config IS '报告配置 JSON: {"sections": [...], "filters": {...}}';

-- ============================================
-- 8. 数据迁移：填充现有数据的分类字段
-- ============================================

-- 根据现有的 error_type 和 status_code 推断错误分类
UPDATE ops_error_logs
SET
    error_source = CASE
        WHEN error_type IN ('authentication_error', 'billing_error', 'subscription_error')
            AND error_phase = 'auth' THEN 'downstream_business'
        WHEN error_type = 'rate_limit_error'
            AND error_phase = 'concurrency' THEN 'downstream_system'
        WHEN error_type IN ('authentication_error', 'permission_error', 'rate_limit_error')
            AND error_phase = 'upstream' THEN 'upstream_business'
        WHEN error_type IN ('upstream_error', 'overloaded_error', 'timeout_error') THEN 'upstream_system'
        WHEN error_type IN ('database_error', 'redis_error', 'internal_error') THEN 'infrastructure'
        ELSE 'internal'
    END,
    error_owner = CASE
        WHEN error_type IN ('authentication_error', 'billing_error', 'invalid_request_error')
            AND error_phase IN ('auth', 'billing', 'response') THEN 'client'
        WHEN error_type IN ('authentication_error', 'permission_error', 'rate_limit_error', 'upstream_error')
            AND error_phase = 'upstream' THEN 'provider'
        WHEN error_type IN ('database_error', 'redis_error', 'timeout_error') THEN 'infrastructure'
        ELSE 'platform'
    END,
    account_status = CASE
        WHEN error_type = 'authentication_error' AND error_phase = 'upstream' THEN 'auth_failed'
        WHEN error_type = 'permission_error' AND error_phase = 'upstream' THEN 'permission_denied'
        WHEN error_type = 'rate_limit_error' AND error_phase = 'upstream' THEN 'rate_limited'
        WHEN error_type IN ('upstream_error', 'timeout_error') THEN 'error'
        ELSE NULL
    END
WHERE error_source IS NULL;

-- 创建索引以加速查询（在数据回填后创建，避免更新时维护索引）
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_error_source_time ON ops_error_logs(error_source, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_error_owner_time ON ops_error_logs(error_owner, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_account_status_time ON ops_error_logs(account_status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_account_id_time ON ops_error_logs(account_id, created_at DESC) WHERE account_id IS NOT NULL;

-- ============================================
-- 9. 插入默认告警规则
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
-- 10. 插入默认定时报告
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
