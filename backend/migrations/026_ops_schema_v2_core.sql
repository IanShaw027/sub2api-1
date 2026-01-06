-- ============================================
-- 运维监控系统 v2 - 核心表结构整合
-- ============================================
-- 整合来源:
--   - 原 026_ops_error_classification.sql (错误分类系统)
--   - 原 027_ops_deep_monitoring.sql (深度监控)
--   - 原 031_remove_unused_tables.sql (清理废弃表)
--   - 原 032_remove_ops_account_status.sql (删除未使用表)
--   - 原 033_simplify_ops_metrics.sql (简化指标)
--   - 原 036_remove_webhook_notification_channels.sql (移除webhook)
--
-- 说明:
--   - 一次性扩展核心表结构(ops_error_logs, usage_logs, ops_alert_rules等)
--   - 删除已验证未使用的废弃表(避免创建后再删除)
--   - 确保幂等性和向下兼容(IF NOT EXISTS/IF EXISTS)
-- ============================================

-- 设置超时以避免启动无限等待
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

-- ============================================
-- 1. 扩展 ops_error_logs 表
-- ============================================

-- 添加错误分类字段 (来自 026)
ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS error_source VARCHAR(50),
    ADD COLUMN IF NOT EXISTS error_owner VARCHAR(50),
    ADD COLUMN IF NOT EXISTS account_status VARCHAR(50),
    ADD COLUMN IF NOT EXISTS upstream_status_code INT,
    ADD COLUMN IF NOT EXISTS upstream_error_message TEXT,
    ADD COLUMN IF NOT EXISTS upstream_error_detail TEXT,
    ADD COLUMN IF NOT EXISTS network_error_type VARCHAR(50),
    ADD COLUMN IF NOT EXISTS retry_after_seconds INT;

-- 添加深度监控字段 (来自 027)
ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS time_to_first_token_ms BIGINT,
    ADD COLUMN IF NOT EXISTS auth_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS routing_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS upstream_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS response_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS request_body JSONB,
    ADD COLUMN IF NOT EXISTS user_agent TEXT;

-- 添加注释
COMMENT ON COLUMN ops_error_logs.error_source IS '错误来源: downstream_business, downstream_system, upstream_business, upstream_system, infrastructure, internal';
COMMENT ON COLUMN ops_error_logs.error_owner IS '错误责任方: client, platform, provider, infrastructure';
COMMENT ON COLUMN ops_error_logs.account_status IS '账号状态: normal, auth_failed, permission_denied, rate_limited, quota_exceeded, disabled, error';
COMMENT ON COLUMN ops_error_logs.upstream_status_code IS '上游实际返回的 HTTP 状态码';
COMMENT ON COLUMN ops_error_logs.upstream_error_message IS '上游错误消息（原始）';
COMMENT ON COLUMN ops_error_logs.upstream_error_detail IS '上游错误详情（网络/超时错误的详细信息）';
COMMENT ON COLUMN ops_error_logs.network_error_type IS '网络错误类型: timeout, connection_refused, dns_error, etc';
COMMENT ON COLUMN ops_error_logs.retry_after_seconds IS '上游建议的重试等待时间（秒）';
COMMENT ON COLUMN ops_error_logs.time_to_first_token_ms IS '首Token延迟(ms) - 流式响应场景下从发送请求到收到第一个Token的时间';
COMMENT ON COLUMN ops_error_logs.auth_latency_ms IS '认证延迟(ms) - 验证API Key和查询用户信息的耗时';
COMMENT ON COLUMN ops_error_logs.routing_latency_ms IS '路由决策延迟(ms) - 账号选择、负载均衡、健康检查的耗时';
COMMENT ON COLUMN ops_error_logs.upstream_latency_ms IS '上游请求延迟(ms) - 发送请求到上游并等待响应的耗时';
COMMENT ON COLUMN ops_error_logs.response_latency_ms IS '响应处理延迟(ms) - 流式响应处理、数据转换、写入数据库的耗时';
COMMENT ON COLUMN ops_error_logs.request_body IS '请求体(脱敏后) - 存储失败请求的请求体用于问题排查';
COMMENT ON COLUMN ops_error_logs.user_agent IS '用户代理 - 识别客户端类型(SDK/浏览器/爬虫)';

-- ============================================
-- 2. 扩展 usage_logs 表
-- ============================================

-- 添加延迟细化字段 (来自 027)
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS time_to_first_token_ms BIGINT,
    ADD COLUMN IF NOT EXISTS auth_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS routing_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS upstream_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS response_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS provider VARCHAR(50);

-- 添加注释
COMMENT ON COLUMN usage_logs.time_to_first_token_ms IS '首Token延迟(ms) - 用于评估用户体验质量';
COMMENT ON COLUMN usage_logs.auth_latency_ms IS '认证延迟(ms)';
COMMENT ON COLUMN usage_logs.routing_latency_ms IS '路由决策延迟(ms)';
COMMENT ON COLUMN usage_logs.upstream_latency_ms IS '上游请求延迟(ms)';
COMMENT ON COLUMN usage_logs.response_latency_ms IS '响应处理延迟(ms)';
COMMENT ON COLUMN usage_logs.provider IS '上游供应商(openai/anthropic/gemini) - 用于按平台统计';

-- ============================================
-- 3. 扩展和清理 ops_alert_rules 表
-- ============================================

-- 添加新字段 (来自 026)
ALTER TABLE ops_alert_rules
    ADD COLUMN IF NOT EXISTS alert_category VARCHAR(50),
    ADD COLUMN IF NOT EXISTS filter_conditions JSONB,
    ADD COLUMN IF NOT EXISTS aggregation_dimensions TEXT[],
    ADD COLUMN IF NOT EXISTS notification_channels JSONB,
    ADD COLUMN IF NOT EXISTS notification_frequency VARCHAR(50) DEFAULT 'immediate',
    ADD COLUMN IF NOT EXISTS notification_template TEXT;

-- 删除 webhook 相关字段 (来自 036)
ALTER TABLE ops_alert_rules
    DROP COLUMN IF EXISTS notify_webhook,
    DROP COLUMN IF EXISTS webhook_url;

-- 添加注释
COMMENT ON COLUMN ops_alert_rules.alert_category IS '告警类别: error_rate, error_count, account_status, latency, availability, cost, scheduled_report';
COMMENT ON COLUMN ops_alert_rules.filter_conditions IS '过滤条件 JSON: {"error_source": ["upstream_business"], "platform": ["openai"]}';
COMMENT ON COLUMN ops_alert_rules.aggregation_dimensions IS '聚合维度: platform, error_type, error_source, account_id, user_id';
COMMENT ON COLUMN ops_alert_rules.notification_channels IS '通知渠道配置 JSON: {"email": {...}}';
COMMENT ON COLUMN ops_alert_rules.notification_frequency IS '通知频率: immediate, throttled_5m, throttled_1h, daily_digest, hourly_digest';
COMMENT ON COLUMN ops_alert_rules.notification_template IS '通知模板（支持变量替换）';

-- ============================================
-- 4. 清理 ops_alert_events 表
-- ============================================

-- 删除 webhook 相关字段 (来自 036)
ALTER TABLE ops_alert_events
    DROP COLUMN IF EXISTS webhook_sent;

-- ============================================
-- 5. 简化 ops_system_metrics 表
-- ============================================

-- 旧视图使用了 `SELECT m.*`，会对所有列产生依赖；先删除以解除依赖关系，
-- 否则 DROP COLUMN 会因为 view 依赖而失败（尤其是 disk_used 等字段）。
DROP VIEW IF EXISTS ops_latest_metrics CASCADE;

-- 旧 GIN 索引依赖 tags 字段（由 025_enhance_ops_monitoring.sql 创建）。
DROP INDEX IF EXISTS idx_ops_metrics_tags;

-- 删除过度监控字段 (来自 033)
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

-- 确保保留正确类型的延迟字段
ALTER TABLE ops_system_metrics
    ADD COLUMN IF NOT EXISTS latency_p95 DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS latency_p99 DOUBLE PRECISION;

COMMENT ON TABLE ops_system_metrics IS '运维系统指标表（已简化）- 从63个字段精简到30个核心字段，删除磁盘/网络/GC等过度监控指标';

-- 重建视图: 最新指标快照（避免使用 SELECT m.* 导致列依赖过强）
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
        m.cpu_usage_percent::DECIMAL
    ) AS health_score
FROM ops_system_metrics m
WHERE m.window_minutes = 1
  AND m.created_at = (SELECT MAX(created_at) FROM ops_system_metrics WHERE window_minutes = 1)
LIMIT 1;

COMMENT ON VIEW ops_latest_metrics IS '最新的系统指标快照,包含健康度评分';

-- ============================================
-- 6. 删除废弃表 (来自 031, 032)
-- ============================================

-- 这些表在早期迁移中被创建，但经代码审查确认未被使用
DROP TABLE IF EXISTS ops_retry_logs CASCADE;
DROP TABLE IF EXISTS ops_alert_notifications CASCADE;
DROP TABLE IF EXISTS ops_upstream_stats CASCADE;
DROP TABLE IF EXISTS ops_dimension_stats CASCADE;
DROP TABLE IF EXISTS ops_account_status CASCADE;
DROP TABLE IF EXISTS ops_data_retention_config CASCADE;

-- 删除相关视图
DROP VIEW IF EXISTS ops_account_status_summary CASCADE;
DROP VIEW IF EXISTS ops_upstream_health_dashboard CASCADE;

-- ============================================
-- 完成
-- ============================================

DO $$
BEGIN
    RAISE NOTICE '========================================';
    RAISE NOTICE '运维监控系统 v2 核心表结构整合完成';
    RAISE NOTICE '========================================';
    RAISE NOTICE '1. ops_error_logs 表扩展: ✓';
    RAISE NOTICE '   - 错误分类字段 (8个)';
    RAISE NOTICE '   - 深度监控字段 (7个)';
    RAISE NOTICE '';
    RAISE NOTICE '2. usage_logs 表扩展: ✓';
    RAISE NOTICE '   - 延迟分析字段 (6个)';
    RAISE NOTICE '';
    RAISE NOTICE '3. ops_alert_rules 表更新: ✓';
    RAISE NOTICE '   - 新增字段 (6个)';
    RAISE NOTICE '   - 删除 webhook 字段 (2个)';
    RAISE NOTICE '';
    RAISE NOTICE '4. ops_system_metrics 表简化: ✓';
    RAISE NOTICE '   - 删除过度监控字段 (11个)';
    RAISE NOTICE '';
    RAISE NOTICE '5. 废弃表清理: ✓';
    RAISE NOTICE '   - 删除6个未使用的表和2个视图';
    RAISE NOTICE '========================================';
END $$;
