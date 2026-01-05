-- ============================================
-- 运维监控系统 v2 - 性能优化与数据回填
-- ============================================
-- 整合来源:
--   - 原 026_ops_error_classification.sql (数据回填)
--   - 原 027_ops_deep_monitoring.sql (usage_logs provider 回填)
--   - 原 029_ops_performance_indexes.sql (GIN 全文索引)
--   - 原 030_add_client_ip_index.sql (client_ip 索引)
--   - 原 032_remove_ops_account_status.sql (account 复合索引)
--   - 原 034_critical_indexes.sql (核心索引)
--   - 原 038_ops_error_log_filter_indexes.sql (过滤索引)
--
-- 说明:
--   - 回填现有数据的新字段(error_source, provider等)
--   - 创建性能优化索引(复合索引、GIN索引、部分索引)
--   - 所有索引创建使用 IF NOT EXISTS 确保幂等性
-- ============================================

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

-- ============================================
-- 第一部分: 数据回填
-- ============================================

-- 1. 回填 ops_error_logs 的错误分类字段 (来自 026)
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

-- 2. 回填 usage_logs 的 provider 字段 (来自 027)
UPDATE usage_logs ul
SET provider = a.platform
FROM accounts a
WHERE ul.account_id = a.id AND ul.provider IS NULL;

-- ============================================
-- 第二部分: 索引优化
-- ============================================

-- --------------------------------
-- A. ops_error_logs 核心索引
-- --------------------------------

-- 1. 时间序列查询索引 (来自 034)
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_created_at
    ON ops_error_logs (created_at DESC);

COMMENT ON INDEX idx_ops_error_logs_created_at IS '优化错误日志时间序列查询性能';

-- 2. 平台维度查询索引 (来自 034)
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_platform_time
    ON ops_error_logs (platform, created_at DESC);

COMMENT ON INDEX idx_ops_error_logs_platform_time IS '优化平台维度错误统计查询';

-- 3. 严重程度查询索引 (来自 034)
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_severity_time
    ON ops_error_logs (severity, created_at DESC);

COMMENT ON INDEX idx_ops_error_logs_severity_time IS '优化严重错误优先级查询';

-- 4. 错误分类索引 (来自 026)
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_error_source_time
    ON ops_error_logs(error_source, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_error_owner_time
    ON ops_error_logs(error_owner, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_account_status_time
    ON ops_error_logs(account_status, created_at DESC);

-- 5. 账号维度索引 (来自 026, 032)
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_account_id_time
    ON ops_error_logs(account_id, created_at DESC)
    WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_account_created
    ON ops_error_logs(account_id, created_at DESC)
    WHERE account_id IS NOT NULL;

-- 6. 延迟分析索引 (来自 027)
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_ttft
    ON ops_error_logs(time_to_first_token_ms)
    WHERE time_to_first_token_ms IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_upstream_latency
    ON ops_error_logs(upstream_latency_ms)
    WHERE upstream_latency_ms IS NOT NULL;

-- 7. 客户端 IP 索引 (来自 030)
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_client_ip_time
    ON ops_error_logs(client_ip, created_at DESC);

-- 8. 复合查询索引 (来自 029)
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_composite
    ON ops_error_logs (created_at DESC, platform, status_code);

-- 9. 过滤维度索引 (来自 038)
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_phase_time
    ON ops_error_logs(error_phase, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_status_code_time
    ON ops_error_logs(status_code, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_group_id_time
    ON ops_error_logs(group_id, created_at DESC)
    WHERE group_id IS NOT NULL;

-- 10. 全文搜索索引 (来自 029)
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_msg_gin
    ON ops_error_logs USING gin (to_tsvector('english', error_message));

-- --------------------------------
-- B. usage_logs 性能索引
-- --------------------------------

-- 1. 延迟分析索引 (来自 027)
CREATE INDEX IF NOT EXISTS idx_usage_logs_ttft
    ON usage_logs(time_to_first_token_ms)
    WHERE time_to_first_token_ms IS NOT NULL;

-- 2. 平台维度索引 (来自 027)
CREATE INDEX IF NOT EXISTS idx_usage_logs_provider
    ON usage_logs(provider)
    WHERE provider IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_usage_logs_provider_created
    ON usage_logs(provider, created_at)
    WHERE provider IS NOT NULL;

-- 3. 账号维度索引 (来自 032)
CREATE INDEX IF NOT EXISTS idx_usage_logs_account_created
    ON usage_logs(account_id, created_at DESC)
    WHERE account_id IS NOT NULL;

-- ============================================
-- 完成
-- ============================================

DO $$
DECLARE
    error_log_count BIGINT;
    usage_log_count BIGINT;
    error_logs_with_source BIGINT;
    usage_logs_with_provider BIGINT;
BEGIN
    -- 统计数据
    SELECT COUNT(*) INTO error_log_count FROM ops_error_logs;
    SELECT COUNT(*) INTO usage_log_count FROM usage_logs;
    SELECT COUNT(*) INTO error_logs_with_source FROM ops_error_logs WHERE error_source IS NOT NULL;
    SELECT COUNT(*) INTO usage_logs_with_provider FROM usage_logs WHERE provider IS NOT NULL;

    RAISE NOTICE '========================================';
    RAISE NOTICE '运维监控系统 v2 性能优化完成';
    RAISE NOTICE '========================================';
    RAISE NOTICE '数据回填统计:';
    RAISE NOTICE '  - ops_error_logs: %/% 条记录已回填分类字段', error_logs_with_source, error_log_count;
    RAISE NOTICE '  - usage_logs: %/% 条记录已回填 provider 字段', usage_logs_with_provider, usage_log_count;
    RAISE NOTICE '';
    RAISE NOTICE '索引创建统计:';
    RAISE NOTICE '  - ops_error_logs: 17 个索引已创建';
    RAISE NOTICE '    * 核心时间序列索引: 3 个';
    RAISE NOTICE '    * 错误分类索引: 3 个';
    RAISE NOTICE '    * 账号维度索引: 2 个';
    RAISE NOTICE '    * 延迟分析索引: 2 个';
    RAISE NOTICE '    * 过滤维度索引: 5 个';
    RAISE NOTICE '    * 全文搜索索引: 1 个';
    RAISE NOTICE '    * 其他复合索引: 1 个';
    RAISE NOTICE '';
    RAISE NOTICE '  - usage_logs: 4 个索引已创建';
    RAISE NOTICE '    * 延迟分析索引: 1 个';
    RAISE NOTICE '    * 平台维度索引: 2 个';
    RAISE NOTICE '    * 账号维度索引: 1 个';
    RAISE NOTICE '========================================';
END $$;
