-- ============================================
-- Ops / 运维监控模块：核心表结构（整合迁移）
-- ============================================
-- 目标：
--   - 将 ops 相关“需要的表”集中在一个迁移脚本中，便于维护与审计
--   - 保持幂等（CREATE/ALTER ... IF [NOT] EXISTS）
--   - 与当前代码（Ent schema + repository SQL）对齐
--
-- 注意：
--   - 本仓库通过 application-level schema_migrations (filename + checksum) 记录迁移，
--     因此请避免修改已发布的迁移文件；如需变更请新增迁移文件。
-- ============================================

-- 防止在启动阶段因锁等待导致无限阻塞
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

-- ============================================
-- 1) ops_error_logs：错误日志（高吞吐、无外键约束）
-- ============================================

CREATE TABLE IF NOT EXISTS ops_error_logs (
    id BIGSERIAL PRIMARY KEY,

    -- Correlation / identities
    request_id VARCHAR(64),
    user_id BIGINT,
    api_key_id BIGINT,
    account_id BIGINT,
    group_id BIGINT,
    client_ip inet,

    -- Core error classification
    error_phase VARCHAR(32) NOT NULL,
    error_type VARCHAR(64) NOT NULL,
    severity VARCHAR(4) NOT NULL,
    status_code INT,
    platform VARCHAR(32),
    model VARCHAR(100),
    request_path VARCHAR(256),
    stream BOOLEAN NOT NULL DEFAULT false,

    -- Payload
    error_message TEXT,
    error_body TEXT,
    provider_error_code VARCHAR(64),
    provider_error_type VARCHAR(64),
    is_retryable BOOLEAN NOT NULL DEFAULT false,
    is_user_actionable BOOLEAN NOT NULL DEFAULT false,
    retry_count INT NOT NULL DEFAULT 0,
    completion_status VARCHAR(16),
    duration_ms INT,

    -- v2: error classification fields
    error_source VARCHAR(50),
    error_owner VARCHAR(50),
    account_status VARCHAR(50),
    upstream_status_code INT,
    upstream_error_message TEXT,
    upstream_error_detail TEXT,
    network_error_type VARCHAR(50),
    retry_after_seconds INT,

    -- v2: deep monitoring timings (ms)
    time_to_first_token_ms BIGINT,
    auth_latency_ms BIGINT,
    routing_latency_ms BIGINT,
    upstream_latency_ms BIGINT,
    response_latency_ms BIGINT,

    -- Context
    request_body JSONB,
    user_agent TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 如果旧表存在但缺列：补齐（幂等）
ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS id BIGSERIAL,
    ADD COLUMN IF NOT EXISTS request_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS user_id BIGINT,
    ADD COLUMN IF NOT EXISTS api_key_id BIGINT,
    ADD COLUMN IF NOT EXISTS account_id BIGINT,
    ADD COLUMN IF NOT EXISTS group_id BIGINT,
    ADD COLUMN IF NOT EXISTS client_ip inet,
    ADD COLUMN IF NOT EXISTS error_phase VARCHAR(32),
    ADD COLUMN IF NOT EXISTS error_type VARCHAR(64),
    ADD COLUMN IF NOT EXISTS severity VARCHAR(4),
    ADD COLUMN IF NOT EXISTS status_code INT,
    ADD COLUMN IF NOT EXISTS platform VARCHAR(32),
    ADD COLUMN IF NOT EXISTS model VARCHAR(100),
    ADD COLUMN IF NOT EXISTS request_path VARCHAR(256),
    ADD COLUMN IF NOT EXISTS stream BOOLEAN,
    ADD COLUMN IF NOT EXISTS error_message TEXT,
    ADD COLUMN IF NOT EXISTS error_body TEXT,
    ADD COLUMN IF NOT EXISTS provider_error_code VARCHAR(64),
    ADD COLUMN IF NOT EXISTS provider_error_type VARCHAR(64),
    ADD COLUMN IF NOT EXISTS is_retryable BOOLEAN,
    ADD COLUMN IF NOT EXISTS is_user_actionable BOOLEAN,
    ADD COLUMN IF NOT EXISTS retry_count INT,
    ADD COLUMN IF NOT EXISTS completion_status VARCHAR(16),
    ADD COLUMN IF NOT EXISTS duration_ms INT,
    ADD COLUMN IF NOT EXISTS error_source VARCHAR(50),
    ADD COLUMN IF NOT EXISTS error_owner VARCHAR(50),
    ADD COLUMN IF NOT EXISTS account_status VARCHAR(50),
    ADD COLUMN IF NOT EXISTS upstream_status_code INT,
    ADD COLUMN IF NOT EXISTS upstream_error_message TEXT,
    ADD COLUMN IF NOT EXISTS upstream_error_detail TEXT,
    ADD COLUMN IF NOT EXISTS network_error_type VARCHAR(50),
    ADD COLUMN IF NOT EXISTS retry_after_seconds INT,
    ADD COLUMN IF NOT EXISTS time_to_first_token_ms BIGINT,
    ADD COLUMN IF NOT EXISTS auth_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS routing_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS upstream_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS response_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS request_body JSONB,
    ADD COLUMN IF NOT EXISTS user_agent TEXT,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ;

COMMENT ON TABLE ops_error_logs IS 'Ops error logs for monitoring/alerting. High-write table; keep schema stable and indexes targeted.';

-- 常用查询索引（按代码查询/聚合模式整理；均为幂等）
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_created_at
    ON ops_error_logs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_platform_time
    ON ops_error_logs (platform, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_severity_time
    ON ops_error_logs (severity, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_phase_time
    ON ops_error_logs (error_phase, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_status_code_time
    ON ops_error_logs (status_code, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_account_id_time
    ON ops_error_logs (account_id, created_at DESC)
    WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_group_id_time
    ON ops_error_logs (group_id, created_at DESC)
    WHERE group_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_client_ip_time
    ON ops_error_logs (client_ip, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_composite
    ON ops_error_logs (created_at DESC, platform, status_code);

-- 全文检索索引（用于错误消息搜索；可按需启用）
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_msg_gin
    ON ops_error_logs USING gin (to_tsvector('english', COALESCE(error_message, '')));

-- ============================================
-- 2) usage_logs：为 Ops 统计补齐字段（不改变现有主流程）
-- ============================================

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS time_to_first_token_ms BIGINT,
    ADD COLUMN IF NOT EXISTS auth_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS routing_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS upstream_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS response_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS provider VARCHAR(50);

CREATE INDEX IF NOT EXISTS idx_usage_logs_ttft
    ON usage_logs(time_to_first_token_ms)
    WHERE time_to_first_token_ms IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_usage_logs_provider
    ON usage_logs(provider)
    WHERE provider IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_usage_logs_provider_created
    ON usage_logs(provider, created_at)
    WHERE provider IS NOT NULL;

-- ============================================
-- 3) ops_system_metrics：系统分钟级指标快照
-- ============================================

CREATE TABLE IF NOT EXISTS ops_system_metrics (
    id BIGSERIAL PRIMARY KEY,
    window_minutes INT NOT NULL DEFAULT 1,

    request_count BIGINT NOT NULL DEFAULT 0,
    success_count BIGINT NOT NULL DEFAULT 0,
    error_count BIGINT NOT NULL DEFAULT 0,
    qps DOUBLE PRECISION,
    tps DOUBLE PRECISION,

    error_4xx_count BIGINT NOT NULL DEFAULT 0,
    error_5xx_count BIGINT NOT NULL DEFAULT 0,
    error_timeout_count BIGINT NOT NULL DEFAULT 0,

    latency_p50 DOUBLE PRECISION,
    latency_p95 DOUBLE PRECISION,
    latency_p99 DOUBLE PRECISION,
    latency_avg DOUBLE PRECISION,
    latency_max DOUBLE PRECISION,
    upstream_latency_avg DOUBLE PRECISION,

    success_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
    error_rate DOUBLE PRECISION NOT NULL DEFAULT 0,

    cpu_usage_percent DOUBLE PRECISION,
    memory_used_mb BIGINT,
    memory_total_mb BIGINT,
    memory_usage_percent DOUBLE PRECISION,

    db_conn_active INT,
    db_conn_idle INT,
    db_conn_waiting INT,
    goroutine_count INT,

    token_consumed BIGINT NOT NULL DEFAULT 0,
    token_rate DOUBLE PRECISION,
    active_subscriptions INT,

    active_alerts INT NOT NULL DEFAULT 0,
    concurrency_queue_depth INT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 如果旧表存在但缺列：补齐（幂等）
ALTER TABLE ops_system_metrics
    ADD COLUMN IF NOT EXISTS id BIGSERIAL,
    ADD COLUMN IF NOT EXISTS window_minutes INT,
    ADD COLUMN IF NOT EXISTS request_count BIGINT,
    ADD COLUMN IF NOT EXISTS success_count BIGINT,
    ADD COLUMN IF NOT EXISTS error_count BIGINT,
    ADD COLUMN IF NOT EXISTS qps DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS tps DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS error_4xx_count BIGINT,
    ADD COLUMN IF NOT EXISTS error_5xx_count BIGINT,
    ADD COLUMN IF NOT EXISTS error_timeout_count BIGINT,
    ADD COLUMN IF NOT EXISTS latency_p50 DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS latency_p95 DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS latency_p99 DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS latency_avg DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS latency_max DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS upstream_latency_avg DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS success_rate DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS error_rate DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS cpu_usage_percent DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS memory_used_mb BIGINT,
    ADD COLUMN IF NOT EXISTS memory_total_mb BIGINT,
    ADD COLUMN IF NOT EXISTS memory_usage_percent DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS db_conn_active INT,
    ADD COLUMN IF NOT EXISTS db_conn_idle INT,
    ADD COLUMN IF NOT EXISTS db_conn_waiting INT,
    ADD COLUMN IF NOT EXISTS goroutine_count INT,
    ADD COLUMN IF NOT EXISTS token_consumed BIGINT,
    ADD COLUMN IF NOT EXISTS token_rate DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS active_subscriptions INT,
    ADD COLUMN IF NOT EXISTS active_alerts INT,
    ADD COLUMN IF NOT EXISTS concurrency_queue_depth INT,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_ops_system_metrics_created_at
    ON ops_system_metrics (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_system_metrics_window_time
    ON ops_system_metrics (window_minutes, created_at DESC);

-- 清理旧字段（如果来自历史版本）
DROP VIEW IF EXISTS ops_latest_metrics CASCADE;
DROP INDEX IF EXISTS idx_ops_metrics_tags;

ALTER TABLE ops_system_metrics
    DROP COLUMN IF EXISTS disk_used,
    DROP COLUMN IF EXISTS disk_total,
    DROP COLUMN IF EXISTS disk_iops,
    DROP COLUMN IF EXISTS network_in_bytes,
    DROP COLUMN IF EXISTS network_out_bytes,
    DROP COLUMN IF EXISTS heap_alloc_mb,
    DROP COLUMN IF EXISTS gc_pause_ms,
    DROP COLUMN IF EXISTS latency_p999,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS http2_errors,
    DROP COLUMN IF EXISTS p95_latency_ms,
    DROP COLUMN IF EXISTS p99_latency_ms;

-- ============================================
-- 4) ops_alert_rules / ops_alert_events：告警规则与事件
-- ============================================

CREATE TABLE IF NOT EXISTS ops_alert_rules (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,

    metric_type VARCHAR(64) NOT NULL,
    operator VARCHAR(8) NOT NULL,
    threshold DOUBLE PRECISION NOT NULL,
    window_minutes INT NOT NULL DEFAULT 1,
    sustained_minutes INT NOT NULL DEFAULT 1,

    severity VARCHAR(20) NOT NULL DEFAULT 'P1',
    notify_email BOOLEAN NOT NULL DEFAULT false,
    cooldown_minutes INT NOT NULL DEFAULT 10,

    -- v2+ extensions
    dimension_filters JSONB,
    notify_channels JSONB,
    notify_config JSONB,
    created_by VARCHAR(100),
    last_triggered_at TIMESTAMPTZ,

    alert_category VARCHAR(50),
    filter_conditions JSONB,
    aggregation_dimensions TEXT[],
    notification_channels JSONB,
    notification_frequency VARCHAR(50) DEFAULT 'immediate',
    notification_template TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 如果旧表存在但缺列：补齐（幂等）
ALTER TABLE ops_alert_rules
    ADD COLUMN IF NOT EXISTS id BIGSERIAL,
    ADD COLUMN IF NOT EXISTS name VARCHAR(128),
    ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS enabled BOOLEAN,
    ADD COLUMN IF NOT EXISTS metric_type VARCHAR(64),
    ADD COLUMN IF NOT EXISTS operator VARCHAR(8),
    ADD COLUMN IF NOT EXISTS threshold DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS window_minutes INT,
    ADD COLUMN IF NOT EXISTS sustained_minutes INT,
    ADD COLUMN IF NOT EXISTS severity VARCHAR(20),
    ADD COLUMN IF NOT EXISTS notify_email BOOLEAN,
    ADD COLUMN IF NOT EXISTS cooldown_minutes INT,
    ADD COLUMN IF NOT EXISTS dimension_filters JSONB,
    ADD COLUMN IF NOT EXISTS notify_channels JSONB,
    ADD COLUMN IF NOT EXISTS notify_config JSONB,
    ADD COLUMN IF NOT EXISTS created_by VARCHAR(100),
    ADD COLUMN IF NOT EXISTS last_triggered_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS alert_category VARCHAR(50),
    ADD COLUMN IF NOT EXISTS filter_conditions JSONB,
    ADD COLUMN IF NOT EXISTS aggregation_dimensions TEXT[],
    ADD COLUMN IF NOT EXISTS notification_channels JSONB,
    ADD COLUMN IF NOT EXISTS notification_frequency VARCHAR(50),
    ADD COLUMN IF NOT EXISTS notification_template TEXT,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ;

-- 兼容：移除旧 webhook 字段（如果存在）
ALTER TABLE ops_alert_rules
    DROP COLUMN IF EXISTS notify_webhook,
    DROP COLUMN IF EXISTS webhook_url;

CREATE INDEX IF NOT EXISTS idx_ops_alert_rules_enabled
    ON ops_alert_rules (enabled);

CREATE INDEX IF NOT EXISTS idx_ops_alert_rules_metric_window
    ON ops_alert_rules (metric_type, window_minutes);

CREATE INDEX IF NOT EXISTS idx_ops_alert_rules_created_at
    ON ops_alert_rules (created_at DESC);

CREATE TABLE IF NOT EXISTS ops_alert_events (
    id BIGSERIAL PRIMARY KEY,
    rule_id BIGINT NOT NULL,

    severity VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'firing',
    title VARCHAR(200),
    description TEXT,
    metric_value DOUBLE PRECISION,
    threshold_value DOUBLE PRECISION,

    fired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    email_sent BOOLEAN NOT NULL DEFAULT false,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 如果旧表存在但缺列：补齐（幂等）
ALTER TABLE ops_alert_events
    ADD COLUMN IF NOT EXISTS id BIGSERIAL,
    ADD COLUMN IF NOT EXISTS rule_id BIGINT,
    ADD COLUMN IF NOT EXISTS severity VARCHAR(20),
    ADD COLUMN IF NOT EXISTS status VARCHAR(20),
    ADD COLUMN IF NOT EXISTS title VARCHAR(200),
    ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS metric_value DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS threshold_value DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS fired_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS email_sent BOOLEAN,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ;

-- 兼容：移除旧 webhook 字段（如果存在）
ALTER TABLE ops_alert_events
    DROP COLUMN IF EXISTS webhook_sent;

CREATE INDEX IF NOT EXISTS idx_ops_alert_events_rule_status
    ON ops_alert_events (rule_id, status);

CREATE INDEX IF NOT EXISTS idx_ops_alert_events_fired_at
    ON ops_alert_events (fired_at DESC);

-- ============================================
-- 5) ops_scheduled_reports：定时报告配置
-- ============================================

CREATE TABLE IF NOT EXISTS ops_scheduled_reports (
    id BIGSERIAL PRIMARY KEY,
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

-- 如果旧表存在但缺列：补齐（幂等）
ALTER TABLE ops_scheduled_reports
    ADD COLUMN IF NOT EXISTS id BIGSERIAL,
    ADD COLUMN IF NOT EXISTS name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS schedule_cron VARCHAR(100),
    ADD COLUMN IF NOT EXISTS report_type VARCHAR(50),
    ADD COLUMN IF NOT EXISTS report_config JSONB,
    ADD COLUMN IF NOT EXISTS notification_channels JSONB,
    ADD COLUMN IF NOT EXISTS enabled BOOLEAN,
    ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_ops_scheduled_reports_next_run
    ON ops_scheduled_reports(next_run_at) WHERE enabled = true;

-- ============================================
-- 6) ops_metrics_hourly / ops_metrics_daily：预聚合表（用于仪表盘查询）
-- ============================================

CREATE TABLE IF NOT EXISTS ops_metrics_hourly (
    id BIGSERIAL PRIMARY KEY,
    bucket_start TIMESTAMPTZ NOT NULL,
    platform VARCHAR(50) NOT NULL DEFAULT '',

    request_count BIGINT DEFAULT 0,
    success_count BIGINT DEFAULT 0,
    error_count BIGINT DEFAULT 0,
    error_4xx_count BIGINT DEFAULT 0,
    error_5xx_count BIGINT DEFAULT 0,
    timeout_count BIGINT DEFAULT 0,

    avg_latency_ms DECIMAL(10,2),
    p99_latency_ms DECIMAL(10,2),
    error_rate DECIMAL(5,2),

    computed_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 如果旧表存在但缺列：补齐（幂等）
ALTER TABLE ops_metrics_hourly
    ADD COLUMN IF NOT EXISTS id BIGSERIAL,
    ADD COLUMN IF NOT EXISTS bucket_start TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS platform VARCHAR(50),
    ADD COLUMN IF NOT EXISTS request_count BIGINT,
    ADD COLUMN IF NOT EXISTS success_count BIGINT,
    ADD COLUMN IF NOT EXISTS error_count BIGINT,
    ADD COLUMN IF NOT EXISTS error_4xx_count BIGINT,
    ADD COLUMN IF NOT EXISTS error_5xx_count BIGINT,
    ADD COLUMN IF NOT EXISTS timeout_count BIGINT,
    ADD COLUMN IF NOT EXISTS avg_latency_ms DECIMAL(10,2),
    ADD COLUMN IF NOT EXISTS p99_latency_ms DECIMAL(10,2),
    ADD COLUMN IF NOT EXISTS error_rate DECIMAL(5,2),
    ADD COLUMN IF NOT EXISTS computed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ops_metrics_hourly_unique
    ON ops_metrics_hourly(bucket_start, platform);

CREATE INDEX IF NOT EXISTS idx_ops_metrics_hourly_bucket
    ON ops_metrics_hourly(bucket_start DESC);

CREATE INDEX IF NOT EXISTS idx_ops_metrics_hourly_platform
    ON ops_metrics_hourly(platform, bucket_start DESC);

CREATE TABLE IF NOT EXISTS ops_metrics_daily (
    id BIGSERIAL PRIMARY KEY,
    bucket_date DATE NOT NULL,
    platform VARCHAR(50) NOT NULL DEFAULT '',

    request_count BIGINT DEFAULT 0,
    success_count BIGINT DEFAULT 0,
    error_count BIGINT DEFAULT 0,
    error_4xx_count BIGINT DEFAULT 0,
    error_5xx_count BIGINT DEFAULT 0,
    timeout_count BIGINT DEFAULT 0,

    avg_latency_ms DECIMAL(10,2),
    p99_latency_ms DECIMAL(10,2),
    error_rate DECIMAL(5,2),

    computed_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 如果旧表存在但缺列：补齐（幂等）
ALTER TABLE ops_metrics_daily
    ADD COLUMN IF NOT EXISTS id BIGSERIAL,
    ADD COLUMN IF NOT EXISTS bucket_date DATE,
    ADD COLUMN IF NOT EXISTS platform VARCHAR(50),
    ADD COLUMN IF NOT EXISTS request_count BIGINT,
    ADD COLUMN IF NOT EXISTS success_count BIGINT,
    ADD COLUMN IF NOT EXISTS error_count BIGINT,
    ADD COLUMN IF NOT EXISTS error_4xx_count BIGINT,
    ADD COLUMN IF NOT EXISTS error_5xx_count BIGINT,
    ADD COLUMN IF NOT EXISTS timeout_count BIGINT,
    ADD COLUMN IF NOT EXISTS avg_latency_ms DECIMAL(10,2),
    ADD COLUMN IF NOT EXISTS p99_latency_ms DECIMAL(10,2),
    ADD COLUMN IF NOT EXISTS error_rate DECIMAL(5,2),
    ADD COLUMN IF NOT EXISTS computed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ops_metrics_daily_unique
    ON ops_metrics_daily(bucket_date, platform);

CREATE INDEX IF NOT EXISTS idx_ops_metrics_daily_date
    ON ops_metrics_daily(bucket_date DESC);

CREATE INDEX IF NOT EXISTS idx_ops_metrics_daily_platform
    ON ops_metrics_daily(platform, bucket_date DESC);

-- ============================================
-- 7) ops_group_availability_*：分组可用性监控
-- ============================================

CREATE TABLE IF NOT EXISTS ops_group_availability_configs (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,

    enabled BOOLEAN NOT NULL DEFAULT false,
    min_available_accounts INT NOT NULL DEFAULT 1,

    threshold_mode VARCHAR(20) NOT NULL DEFAULT 'count',
    min_available_percentage DOUBLE PRECISION NOT NULL DEFAULT 0,

    notify_email BOOLEAN NOT NULL DEFAULT true,
    severity VARCHAR(20) NOT NULL DEFAULT 'warning',
    cooldown_minutes INT NOT NULL DEFAULT 30,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(group_id),

    CONSTRAINT chk_ops_group_availability_configs_threshold_mode
        CHECK (threshold_mode IN ('count', 'percentage', 'both')),
    CONSTRAINT chk_ops_group_availability_configs_min_available_percentage
        CHECK (min_available_percentage >= 0 AND min_available_percentage <= 100)
);

-- 如果旧表存在但缺列：补齐（幂等）
ALTER TABLE ops_group_availability_configs
    ADD COLUMN IF NOT EXISTS id BIGSERIAL,
    ADD COLUMN IF NOT EXISTS group_id BIGINT,
    ADD COLUMN IF NOT EXISTS enabled BOOLEAN,
    ADD COLUMN IF NOT EXISTS min_available_accounts INT,
    ADD COLUMN IF NOT EXISTS threshold_mode VARCHAR(20),
    ADD COLUMN IF NOT EXISTS min_available_percentage DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS notify_email BOOLEAN,
    ADD COLUMN IF NOT EXISTS severity VARCHAR(20),
    ADD COLUMN IF NOT EXISTS cooldown_minutes INT,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_ops_group_availability_configs_group_id
    ON ops_group_availability_configs(group_id);
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_configs_enabled
    ON ops_group_availability_configs(enabled);

CREATE TABLE IF NOT EXISTS ops_group_availability_events (
    id BIGSERIAL PRIMARY KEY,
    config_id BIGINT NOT NULL REFERENCES ops_group_availability_configs(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,

    status VARCHAR(20) NOT NULL DEFAULT 'firing',
    severity VARCHAR(20) NOT NULL,

    title TEXT NOT NULL,
    description TEXT NOT NULL,

    available_accounts INT NOT NULL,
    threshold_accounts INT NOT NULL,
    total_accounts INT NOT NULL,

    email_sent BOOLEAN NOT NULL DEFAULT false,

    fired_at TIMESTAMP NOT NULL,
    resolved_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_ops_group_availability_events_status
        CHECK (status IN ('firing', 'resolved'))
);

-- 如果旧表存在但缺列：补齐（幂等）
ALTER TABLE ops_group_availability_events
    ADD COLUMN IF NOT EXISTS id BIGSERIAL,
    ADD COLUMN IF NOT EXISTS config_id BIGINT,
    ADD COLUMN IF NOT EXISTS group_id BIGINT,
    ADD COLUMN IF NOT EXISTS status VARCHAR(20),
    ADD COLUMN IF NOT EXISTS severity VARCHAR(20),
    ADD COLUMN IF NOT EXISTS title TEXT,
    ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS available_accounts INT,
    ADD COLUMN IF NOT EXISTS threshold_accounts INT,
    ADD COLUMN IF NOT EXISTS total_accounts INT,
    ADD COLUMN IF NOT EXISTS email_sent BOOLEAN,
    ADD COLUMN IF NOT EXISTS fired_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_ops_group_availability_events_config_id
    ON ops_group_availability_events(config_id);
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_events_group_id
    ON ops_group_availability_events(group_id);
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_events_status
    ON ops_group_availability_events(status);
CREATE INDEX IF NOT EXISTS idx_ops_group_availability_events_fired_at
    ON ops_group_availability_events(fired_at DESC);

-- ============================================
-- 8) 辅助函数与视图（仪表盘/排障）
-- ============================================

-- 说明：
-- PostgreSQL 的 `CREATE OR REPLACE FUNCTION` 不允许在“同签名函数已存在”的情况下变更入参名称，
-- 否则会报错：cannot change name of input parameter ...
-- 为确保幂等 + 兼容旧环境，这里先按签名 DROP，再重新创建。
DROP FUNCTION IF EXISTS calculate_latency_breakdown(BIGINT, BIGINT, BIGINT, BIGINT) CASCADE;
DROP FUNCTION IF EXISTS calculate_health_score(NUMERIC, NUMERIC, NUMERIC, NUMERIC) CASCADE;

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
    total_latency := COALESCE(p_auth_latency, 0) +
                     COALESCE(p_routing_latency, 0) +
                     COALESCE(p_upstream_latency, 0) +
                     COALESCE(p_response_latency, 0);

    IF total_latency = 0 THEN
        RETURN '{"auth_pct": 0, "routing_pct": 0, "upstream_pct": 0, "response_pct": 0, "total_ms": 0}'::JSONB;
    END IF;

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

-- DB 侧健康分（用于 view/调试；业务侧 health_score 仍以 Go 计算为准）
CREATE OR REPLACE FUNCTION calculate_health_score(
    p_success_rate DECIMAL,
    p_error_rate DECIMAL,
    p_latency_p99 DECIMAL,
    p_cpu_usage_percent DECIMAL
) RETURNS INT AS $$
DECLARE
    success_rate DECIMAL := COALESCE(p_success_rate, 0);
    error_rate DECIMAL := COALESCE(p_error_rate, 0);
    latency_p99 DECIMAL := COALESCE(p_latency_p99, 0);
    cpu_usage DECIMAL := COALESCE(p_cpu_usage_percent, 0);
    score DECIMAL := 100;
BEGIN
    -- SLA impact (max -45)
    IF success_rate < 99.9 THEN
        score := score - LEAST(45, (99.9 - success_rate) * 12);
    END IF;

    -- Latency impact (max -35)
    IF latency_p99 > 1000 THEN
        score := score - LEAST(35, (latency_p99 - 1000) / 80);
    END IF;

    -- Error rate impact (max -20) (percent, e.g. 0.1 = 0.1%)
    IF error_rate > 0.1 THEN
        score := score - LEAST(20, (error_rate - 0.1) * 60);
    END IF;

    -- CPU impact (max -15)
    IF cpu_usage > 90 THEN
        score := score - LEAST(15, (cpu_usage - 90) / 2);
    END IF;

    IF score < 0 THEN
        score := 0;
    END IF;
    IF score > 100 THEN
        score := 100;
    END IF;

    RETURN ROUND(score)::INT;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

CREATE OR REPLACE VIEW ops_latest_metrics AS
SELECT
    m.created_at,
    m.window_minutes,
    m.request_count,
    m.success_count,
    m.error_count,
    m.success_rate,
    m.error_rate,
    m.qps,
    m.tps,
    m.latency_p95,
    m.latency_p99,
    m.cpu_usage_percent,
    m.memory_usage_percent,
    m.concurrency_queue_depth,
    m.active_alerts,
    calculate_health_score(
        m.success_rate::DECIMAL,
        m.error_rate::DECIMAL,
        COALESCE(m.latency_p99, 0)::DECIMAL,
        COALESCE(m.cpu_usage_percent, 0)::DECIMAL
    ) AS health_score
FROM ops_system_metrics m
WHERE m.window_minutes = 1
  AND m.created_at = (SELECT MAX(created_at) FROM ops_system_metrics WHERE window_minutes = 1)
LIMIT 1;

CREATE OR REPLACE VIEW ops_error_detail_view AS
SELECT
    e.id,
    e.request_id,
    e.created_at,
    e.error_phase,
    e.error_type,
    e.severity,
    e.status_code,
    e.error_message,
    e.duration_ms,

    e.request_path,
    e.stream,
    e.time_to_first_token_ms,
    e.auth_latency_ms,
    e.routing_latency_ms,
    e.upstream_latency_ms,
    e.response_latency_ms,
    e.request_body,

    e.platform,
    e.model,
    e.account_id,
    a.name AS account_name,
    a.type AS auth_type,
    a.status AS account_status_runtime,

    e.user_id,
    u.email AS user_email,
    e.client_ip,
    e.user_agent,
    e.api_key_id,
    k.name AS api_key_name,
    e.group_id,
    g.name AS group_name,

    e.is_retryable,
    e.retry_count
FROM ops_error_logs e
LEFT JOIN accounts a ON e.account_id = a.id
LEFT JOIN users u ON e.user_id = u.id
LEFT JOIN api_keys k ON e.api_key_id = k.id
LEFT JOIN groups g ON e.group_id = g.id;

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

-- ============================================
-- 9) 清理历史遗留的未使用对象（若曾存在）
-- ============================================

DROP TABLE IF EXISTS ops_retry_logs CASCADE;
DROP TABLE IF EXISTS ops_alert_notifications CASCADE;
DROP TABLE IF EXISTS ops_upstream_stats CASCADE;
DROP TABLE IF EXISTS ops_dimension_stats CASCADE;
DROP TABLE IF EXISTS ops_account_status CASCADE;
DROP TABLE IF EXISTS ops_data_retention_config CASCADE;

DROP VIEW IF EXISTS ops_account_status_summary CASCADE;
DROP VIEW IF EXISTS ops_upstream_health_dashboard CASCADE;
