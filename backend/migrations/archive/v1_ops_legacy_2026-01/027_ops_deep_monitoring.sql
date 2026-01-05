-- 运维深度监控系统 - 数据模型扩展
-- 创建时间: 2026-01-03
-- 说明: 实现四层深度监控体系,支持TTFT统计、延迟细化拆分、请求体存储、上游三级统计
-- 参考文档: docs/ops-monitoring/deep-planning.md

-- ============================================
-- 1. 扩展 ops_error_logs 表
-- ============================================

-- 新增延迟细化字段
ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS time_to_first_token_ms BIGINT,
    ADD COLUMN IF NOT EXISTS auth_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS routing_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS upstream_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS response_latency_ms BIGINT;

-- 新增请求体存储字段(脱敏后,限制10KB)
ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS request_body JSONB;

-- 新增客户端信息字段(client_ip已存在,只需添加user_agent)
ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS user_agent TEXT;

-- 添加注释
COMMENT ON COLUMN ops_error_logs.time_to_first_token_ms IS '首Token延迟(ms) - 流式响应场景下从发送请求到收到第一个Token的时间';
COMMENT ON COLUMN ops_error_logs.auth_latency_ms IS '认证延迟(ms) - 验证API Key和查询用户信息的耗时';
COMMENT ON COLUMN ops_error_logs.routing_latency_ms IS '路由决策延迟(ms) - 账号选择、负载均衡、健康检查的耗时';
COMMENT ON COLUMN ops_error_logs.upstream_latency_ms IS '上游请求延迟(ms) - 发送请求到上游并等待响应的耗时';
COMMENT ON COLUMN ops_error_logs.response_latency_ms IS '响应处理延迟(ms) - 流式响应处理、数据转换、写入数据库的耗时';
COMMENT ON COLUMN ops_error_logs.request_body IS '请求体(脱敏后) - 存储失败请求的请求体用于问题排查';
COMMENT ON COLUMN ops_error_logs.user_agent IS '用户代理 - 识别客户端类型(SDK/浏览器/爬虫)';

-- 添加索引以优化查询
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_ttft ON ops_error_logs(time_to_first_token_ms)
    WHERE time_to_first_token_ms IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_upstream_latency ON ops_error_logs(upstream_latency_ms)
    WHERE upstream_latency_ms IS NOT NULL;

-- ============================================
-- 2. 扩展 usage_logs 表
-- ============================================

-- 新增延迟细化字段(成功请求也需要记录延迟以进行性能分析)
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS time_to_first_token_ms BIGINT,
    ADD COLUMN IF NOT EXISTS auth_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS routing_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS upstream_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS response_latency_ms BIGINT;

-- 新增provider字段用于与ops_error_logs统一(关联到accounts.platform)
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS provider VARCHAR(50);

-- 添加注释
COMMENT ON COLUMN usage_logs.time_to_first_token_ms IS '首Token延迟(ms) - 用于评估用户体验质量';
COMMENT ON COLUMN usage_logs.auth_latency_ms IS '认证延迟(ms)';
COMMENT ON COLUMN usage_logs.routing_latency_ms IS '路由决策延迟(ms)';
COMMENT ON COLUMN usage_logs.upstream_latency_ms IS '上游请求延迟(ms)';
COMMENT ON COLUMN usage_logs.response_latency_ms IS '响应处理延迟(ms)';
COMMENT ON COLUMN usage_logs.provider IS '上游供应商(openai/anthropic/gemini) - 用于按平台统计';

-- 添加索引
CREATE INDEX IF NOT EXISTS idx_usage_logs_ttft ON usage_logs(time_to_first_token_ms)
    WHERE time_to_first_token_ms IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_usage_logs_provider ON usage_logs(provider)
    WHERE provider IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_usage_logs_provider_created ON usage_logs(provider, created_at)
    WHERE provider IS NOT NULL;

-- 回填现有数据的provider字段
UPDATE usage_logs ul
SET provider = a.platform
FROM accounts a
WHERE ul.account_id = a.id AND ul.provider IS NULL;

-- ============================================
-- 3. 创建 ops_upstream_stats 表(上游三级统计)
-- ============================================

CREATE TABLE IF NOT EXISTS ops_upstream_stats (
    id BIGSERIAL PRIMARY KEY,

    -- 三级维度
    platform VARCHAR(50) NOT NULL,                  -- 平台: openai/anthropic/gemini
    auth_type VARCHAR(50),                          -- 认证类型: api_key/oauth/azure_ad
    account_id BIGINT REFERENCES accounts(id) ON DELETE CASCADE,  -- 账号ID

    -- 统计周期
    stat_time TIMESTAMPTZ NOT NULL,                 -- 统计时间点
    period_minutes INT NOT NULL,                    -- 统计周期(5/15/60分钟)

    -- 请求统计
    total_requests INT DEFAULT 0,
    success_count INT DEFAULT 0,
    error_count INT DEFAULT 0,
    success_rate DECIMAL(5,2),

    -- 延迟统计
    latency_avg INT,
    latency_p50 INT,
    latency_p95 INT,
    latency_p99 INT,
    ttft_median INT,                                -- 首Token延迟中位数

    -- 错误分布
    error_401_count INT DEFAULT 0,                  -- 认证失败
    error_403_count INT DEFAULT 0,                  -- 权限不足
    error_429_count INT DEFAULT 0,                  -- 限流
    error_500_count INT DEFAULT 0,                  -- 上游内部错误
    error_502_count INT DEFAULT 0,                  -- 网关错误
    error_504_count INT DEFAULT 0,                  -- 超时
    error_timeout_count INT DEFAULT 0,              -- 连接超时

    -- 账号状态
    last_error_time TIMESTAMPTZ,
    last_error_message TEXT,
    consecutive_errors INT DEFAULT 0,               -- 连续错误次数
    account_status VARCHAR(50),                     -- normal/auth_failed/rate_limited/error

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 创建复合唯一索引,确保每个维度组合在每个时间点只有一条记录
CREATE UNIQUE INDEX IF NOT EXISTS idx_ops_upstream_stats_unique
    ON ops_upstream_stats(
        platform,
        COALESCE(auth_type, ''),
        COALESCE(account_id, 0),
        stat_time,
        period_minutes
    );

-- 创建查询索引
CREATE INDEX IF NOT EXISTS idx_ops_upstream_stats_platform
    ON ops_upstream_stats(platform, stat_time DESC);

CREATE INDEX IF NOT EXISTS idx_ops_upstream_stats_account
    ON ops_upstream_stats(account_id, stat_time DESC)
    WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ops_upstream_stats_time
    ON ops_upstream_stats(stat_time DESC);

CREATE INDEX IF NOT EXISTS idx_ops_upstream_stats_status
    ON ops_upstream_stats(account_status)
    WHERE account_status IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ops_upstream_stats_period
    ON ops_upstream_stats(period_minutes, stat_time DESC);

-- 添加注释
COMMENT ON TABLE ops_upstream_stats IS '上游三级统计表 - 支持按platform/auth_type/account_id三级维度下钻分析';
COMMENT ON COLUMN ops_upstream_stats.platform IS '上游平台(Level 1)';
COMMENT ON COLUMN ops_upstream_stats.auth_type IS '认证类型(Level 2)';
COMMENT ON COLUMN ops_upstream_stats.account_id IS '账号ID(Level 3)';
COMMENT ON COLUMN ops_upstream_stats.stat_time IS '统计时间点(窗口结束时间)';
COMMENT ON COLUMN ops_upstream_stats.period_minutes IS '统计周期: 5分钟(原始)/15分钟/60分钟(1小时)/1440分钟(1天)';
COMMENT ON COLUMN ops_upstream_stats.ttft_median IS 'Time To First Token中位数 - 流式响应首屏等待时间';
COMMENT ON COLUMN ops_upstream_stats.consecutive_errors IS '连续错误次数 - 用于账号健康检测(连续失败3次标记为error)';
COMMENT ON COLUMN ops_upstream_stats.account_status IS '账号状态: normal(正常)/auth_failed(认证失败)/rate_limited(被限流)/error(错误)';

-- ============================================
-- 4. 创建 ops_retry_logs 表(请求重试日志)
-- ============================================

CREATE TABLE IF NOT EXISTS ops_retry_logs (
    id BIGSERIAL PRIMARY KEY,
    retry_id VARCHAR(100) UNIQUE NOT NULL,          -- 重试唯一ID(retry_abc123)
    original_error_id BIGINT NOT NULL REFERENCES ops_error_logs(id) ON DELETE CASCADE,
    original_request_id VARCHAR(100),               -- 原始请求ID

    -- 重试配置
    use_original_params BOOLEAN DEFAULT TRUE,       -- 是否使用原始请求参数
    override_account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,  -- 可选: 指定使用特定账号重试

    -- 重试结果
    status VARCHAR(50),                             -- success/failed
    http_code INT,
    latency_ms INT,
    time_to_first_token_ms INT,
    response_body TEXT,                             -- 重试响应体(成功或失败)
    error_message TEXT,                             -- 重试失败时的错误信息

    -- 操作信息
    initiated_by VARCHAR(100),                      -- 操作人员(用户ID或管理员邮箱)
    initiated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_ops_retry_logs_error
    ON ops_retry_logs(original_error_id);

CREATE INDEX IF NOT EXISTS idx_ops_retry_logs_time
    ON ops_retry_logs(initiated_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_retry_logs_status
    ON ops_retry_logs(status);

CREATE INDEX IF NOT EXISTS idx_ops_retry_logs_initiated_by
    ON ops_retry_logs(initiated_by);

-- 添加注释
COMMENT ON TABLE ops_retry_logs IS '请求重试日志 - 记录管理员手动重试失败请求的结果,用于验证问题是否已解决';
COMMENT ON COLUMN ops_retry_logs.retry_id IS '重试唯一标识,格式: retry_{timestamp}_{random}';
COMMENT ON COLUMN ops_retry_logs.use_original_params IS '是否使用原始请求的完整参数(model/messages等)';
COMMENT ON COLUMN ops_retry_logs.override_account_id IS '可选: 强制使用指定账号重试,用于测试特定账号是否恢复';
COMMENT ON COLUMN ops_retry_logs.initiated_by IS '发起重试的操作人员,用于审计';

-- ============================================
-- 5. 创建视图: 上游账号健康仪表盘
-- ============================================

CREATE OR REPLACE VIEW ops_upstream_health_dashboard AS
SELECT
    s.platform,
    s.auth_type,
    s.account_id,
    a.name AS account_name,
    a.status AS account_config_status,
    s.account_status,
    s.success_rate,
    s.latency_p99,
    s.ttft_median,
    s.consecutive_errors,
    s.last_error_time,
    s.last_error_message,
    s.total_requests,
    s.error_count,
    -- 错误分布
    s.error_401_count,
    s.error_403_count,
    s.error_429_count,
    s.error_500_count,
    s.error_502_count,
    s.error_504_count,
    s.error_timeout_count,
    s.stat_time,
    s.period_minutes
FROM ops_upstream_stats s
LEFT JOIN accounts a ON s.account_id = a.id
WHERE s.period_minutes = 5  -- 默认显示5分钟粒度的最新数据
  AND s.stat_time >= NOW() - INTERVAL '1 hour'
ORDER BY s.stat_time DESC, s.platform, s.account_id;

COMMENT ON VIEW ops_upstream_health_dashboard IS '上游账号健康仪表盘 - 展示最近1小时各账号的健康状况';

-- ============================================
-- 6. 创建视图: 错误详情下钻(四层监控)
-- ============================================

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
    e.retry_count,
    (SELECT COUNT(*) FROM ops_retry_logs r WHERE r.original_error_id = e.id) AS retry_attempts
FROM ops_error_logs e
LEFT JOIN accounts a ON e.account_id = a.id
LEFT JOIN users u ON e.user_id = u.id
LEFT JOIN api_keys k ON e.api_key_id = k.id
LEFT JOIN groups g ON e.group_id = g.id;

COMMENT ON VIEW ops_error_detail_view IS '错误详情下钻视图 - 整合L1~L4层监控信息,用于快速定位问题根因';

-- ============================================
-- 7. 数据保留策略配置
-- ============================================

-- 更新数据保留策略配置,添加新表
INSERT INTO ops_data_retention_config (table_name, retention_days, enabled) VALUES
    ('ops_upstream_stats', 7, true),    -- 5分钟粒度保留7天(会降采样到小时/天)
    ('ops_retry_logs', 30, true)        -- 重试日志保留30天
ON CONFLICT (table_name) DO NOTHING;

-- ============================================
-- 8. 创建辅助函数: 计算延迟各阶段占比
-- ============================================

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
-- 9. 验证数据完整性
-- ============================================

-- 验证ops_error_logs表结构
DO $$
BEGIN
    -- 检查必要字段是否存在
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ops_error_logs' AND column_name = 'time_to_first_token_ms'
    ) THEN
        RAISE EXCEPTION 'ops_error_logs.time_to_first_token_ms 字段创建失败';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ops_error_logs' AND column_name = 'request_body'
    ) THEN
        RAISE EXCEPTION 'ops_error_logs.request_body 字段创建失败';
    END IF;

    RAISE NOTICE 'ops_error_logs 表扩展成功';
END $$;

-- 验证usage_logs表结构
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'usage_logs' AND column_name = 'time_to_first_token_ms'
    ) THEN
        RAISE EXCEPTION 'usage_logs.time_to_first_token_ms 字段创建失败';
    END IF;

    RAISE NOTICE 'usage_logs 表扩展成功';
END $$;

-- 验证新表创建
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'ops_upstream_stats'
    ) THEN
        RAISE EXCEPTION 'ops_upstream_stats 表创建失败';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'ops_retry_logs'
    ) THEN
        RAISE EXCEPTION 'ops_retry_logs 表创建失败';
    END IF;

    RAISE NOTICE 'ops_upstream_stats 和 ops_retry_logs 表创建成功';
END $$;

-- ============================================
-- 完成
-- ============================================

-- 显示迁移摘要
DO $$
DECLARE
    error_log_count BIGINT;
    usage_log_count BIGINT;
BEGIN
    SELECT COUNT(*) INTO error_log_count FROM ops_error_logs;
    SELECT COUNT(*) INTO usage_log_count FROM usage_logs;

    RAISE NOTICE '========================================';
    RAISE NOTICE '运维深度监控系统数据模型扩展完成';
    RAISE NOTICE '========================================';
    RAISE NOTICE '1. ops_error_logs 表扩展: ✓';
    RAISE NOTICE '   - 新增延迟细化字段 (5个)';
    RAISE NOTICE '   - 新增请求体存储字段 (1个)';
    RAISE NOTICE '   - 新增客户端信息字段 (1个)';
    RAISE NOTICE '   - 现有记录数: %', error_log_count;
    RAISE NOTICE '';
    RAISE NOTICE '2. usage_logs 表扩展: ✓';
    RAISE NOTICE '   - 新增延迟细化字段 (5个)';
    RAISE NOTICE '   - 新增provider字段 (1个)';
    RAISE NOTICE '   - 现有记录数: %', usage_log_count;
    RAISE NOTICE '';
    RAISE NOTICE '3. ops_upstream_stats 表创建: ✓';
    RAISE NOTICE '   - 支持三级统计 (platform/auth_type/account_id)';
    RAISE NOTICE '   - 支持多时间粒度 (5min/15min/1h/1d)';
    RAISE NOTICE '';
    RAISE NOTICE '4. ops_retry_logs 表创建: ✓';
    RAISE NOTICE '   - 支持请求重试功能';
    RAISE NOTICE '';
    RAISE NOTICE '5. 辅助视图和函数创建: ✓';
    RAISE NOTICE '   - ops_upstream_health_dashboard 视图';
    RAISE NOTICE '   - ops_error_detail_view 视图';
    RAISE NOTICE '   - calculate_latency_breakdown 函数';
    RAISE NOTICE '========================================';
END $$;
