-- 运维深度监控系统 - 数据模型扩展回滚脚本
-- 创建时间: 2026-01-03
-- 说明: 回滚 027_ops_deep_monitoring.sql 的所有变更

-- ============================================
-- 1. 删除辅助函数
-- ============================================

DROP FUNCTION IF EXISTS calculate_latency_breakdown(BIGINT, BIGINT, BIGINT, BIGINT);

-- ============================================
-- 2. 删除视图
-- ============================================

DROP VIEW IF EXISTS ops_error_detail_view;
DROP VIEW IF EXISTS ops_upstream_health_dashboard;

-- ============================================
-- 3. 删除 ops_retry_logs 表
-- ============================================

DROP TABLE IF EXISTS ops_retry_logs CASCADE;

-- ============================================
-- 4. 删除 ops_upstream_stats 表
-- ============================================

DROP TABLE IF EXISTS ops_upstream_stats CASCADE;

-- ============================================
-- 5. 移除 ops_data_retention_config 配置
-- ============================================

DELETE FROM ops_data_retention_config
WHERE table_name IN ('ops_upstream_stats', 'ops_retry_logs');

-- ============================================
-- 6. 回滚 usage_logs 表扩展
-- ============================================

-- 删除索引
DROP INDEX IF EXISTS idx_usage_logs_provider_created;
DROP INDEX IF EXISTS idx_usage_logs_provider;
DROP INDEX IF EXISTS idx_usage_logs_ttft;

-- 删除字段
ALTER TABLE usage_logs
    DROP COLUMN IF EXISTS provider,
    DROP COLUMN IF EXISTS response_latency_ms,
    DROP COLUMN IF EXISTS upstream_latency_ms,
    DROP COLUMN IF EXISTS routing_latency_ms,
    DROP COLUMN IF EXISTS auth_latency_ms,
    DROP COLUMN IF EXISTS time_to_first_token_ms;

-- ============================================
-- 7. 回滚 ops_error_logs 表扩展
-- ============================================

-- 删除索引
DROP INDEX IF EXISTS idx_ops_error_logs_upstream_latency;
DROP INDEX IF EXISTS idx_ops_error_logs_ttft;

-- 删除字段
ALTER TABLE ops_error_logs
    DROP COLUMN IF EXISTS user_agent,
    DROP COLUMN IF EXISTS request_body,
    DROP COLUMN IF EXISTS response_latency_ms,
    DROP COLUMN IF EXISTS upstream_latency_ms,
    DROP COLUMN IF EXISTS routing_latency_ms,
    DROP COLUMN IF EXISTS auth_latency_ms,
    DROP COLUMN IF EXISTS time_to_first_token_ms;

-- ============================================
-- 完成
-- ============================================

DO $$
BEGIN
    RAISE NOTICE '========================================';
    RAISE NOTICE '运维深度监控系统数据模型扩展已回滚';
    RAISE NOTICE '========================================';
    RAISE NOTICE '1. ops_error_logs 表字段已移除';
    RAISE NOTICE '2. usage_logs 表字段已移除';
    RAISE NOTICE '3. ops_upstream_stats 表已删除';
    RAISE NOTICE '4. ops_retry_logs 表已删除';
    RAISE NOTICE '5. 辅助视图和函数已删除';
    RAISE NOTICE '========================================';
END $$;
