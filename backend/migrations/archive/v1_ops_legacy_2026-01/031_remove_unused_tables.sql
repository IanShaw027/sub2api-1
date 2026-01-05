-- 删除未使用的数据库表
-- 创建时间: 2026-01-04
-- 说明: 清理运维监控系统中已被验证为完全未使用的表，减少数据库复杂度
--
-- 被删除的表（已代码审查确认无引用）:
-- 1. ops_upstream_stats (027 migration): 上游三级统计表 - 未被后端代码使用
-- 2. ops_dimension_stats (025 migration): 多维度统计表 - 未被后端代码使用
-- 3. ops_data_retention_config (025 migration): 数据清理配置表 - 未被后端代码使用
-- 4. ops_alert_notifications (026 migration): 告警通知发送历史 - 未被后端代码使用
-- 5. ops_retry_logs (027 migration): 请求重试日志 - 未被后端代码使用
--
-- 依赖关系分析:
-- - ops_upstream_stats 被视图 ops_upstream_health_dashboard 引用
--   但该视图本身也未被代码使用,已在 027_ops_deep_monitoring_down.sql 中删除
-- - ops_retry_logs 外键引用 ops_error_logs,但已无代码调用
-- - ops_alert_notifications 外键引用 ops_alert_events 和 ops_alert_rules,但未被使用
-- - 其他表无外键依赖
--
-- 删除顺序（遵循外键约束）:
-- 1. 先删除有外键约束的表: ops_retry_logs, ops_alert_notifications
-- 2. 再删除独立表: ops_upstream_stats, ops_dimension_stats, ops_data_retention_config

-- ============================================
-- 1. 删除 ops_retry_logs 表
-- ============================================
-- 该表有外键引用 ops_error_logs, 需要先删除

DROP TABLE IF EXISTS ops_retry_logs CASCADE;

-- ============================================
-- 2. 删除 ops_alert_notifications 表
-- ============================================
-- 该表有外键引用 ops_alert_events 和 ops_alert_rules, 需要先删除

DROP TABLE IF EXISTS ops_alert_notifications CASCADE;

-- ============================================
-- 3. 删除 ops_upstream_stats 表
-- ============================================
-- 该表被视图 ops_upstream_health_dashboard 引用, 但该视图已在 027_ops_deep_monitoring_down.sql 中删除

DROP TABLE IF EXISTS ops_upstream_stats CASCADE;

-- ============================================
-- 4. 删除 ops_dimension_stats 表
-- ============================================

DROP TABLE IF EXISTS ops_dimension_stats CASCADE;

-- ============================================
-- 5. 删除或清理 ops_data_retention_config 配置
-- ============================================
-- 删除与这些表相关的配置记录

DELETE FROM ops_data_retention_config
WHERE table_name IN (
    'ops_upstream_stats',
    'ops_dimension_stats',
    'ops_retry_logs',
    'ops_alert_notifications'
);

-- ============================================
-- 完成
-- ============================================

DO $$
BEGIN
    RAISE NOTICE '========================================';
    RAISE NOTICE '未使用的表已成功删除';
    RAISE NOTICE '========================================';
    RAISE NOTICE '1. ops_retry_logs 表已删除';
    RAISE NOTICE '2. ops_alert_notifications 表已删除';
    RAISE NOTICE '3. ops_upstream_stats 表已删除';
    RAISE NOTICE '4. ops_dimension_stats 表已删除';
    RAISE NOTICE '5. ops_data_retention_config 配置已清理';
    RAISE NOTICE '';
    RAISE NOTICE '删除原因: 代码审查确认这些表未被后端代码调用';
    RAISE NOTICE '- 未来如需恢复,请参考对应的创建迁移文件';
    RAISE NOTICE '  * ops_upstream_stats, ops_retry_logs -> 027_ops_deep_monitoring.sql';
    RAISE NOTICE '  * ops_dimension_stats, ops_data_retention_config -> 025_enhance_ops_monitoring.sql';
    RAISE NOTICE '  * ops_alert_notifications -> 026_ops_error_classification.sql';
    RAISE NOTICE '========================================';
END $$;
