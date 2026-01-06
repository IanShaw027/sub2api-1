-- 清理旧版 Ops 相关对象（表/视图/函数）
--
-- 背景：
-- - master 分支已存在 ops 相关迁移（例如 026_ops_metrics_aggregation_tables.sql）
-- - 本分支将 Ops 需要的所有表统一整理到 031_ops_schema.sql
-- - 为避免历史表结构与新 schema 冲突，这里先删除旧对象（数据会丢失，但 Ops 数据可重建）

DROP VIEW IF EXISTS ops_latest_metrics CASCADE;

-- 函数签名保持与 031_ops_schema.sql 一致，避免 `cannot change name of input parameter` 类问题
DROP FUNCTION IF EXISTS calculate_latency_breakdown(BIGINT, BIGINT, BIGINT, BIGINT) CASCADE;
DROP FUNCTION IF EXISTS calculate_health_score(NUMERIC, NUMERIC, NUMERIC, NUMERIC) CASCADE;

-- Ops tables (drop in reverse dependency order)
DROP TABLE IF EXISTS ops_group_availability_events CASCADE;
DROP TABLE IF EXISTS ops_group_availability_configs CASCADE;

DROP TABLE IF EXISTS ops_metrics_daily CASCADE;
DROP TABLE IF EXISTS ops_metrics_hourly CASCADE;

DROP TABLE IF EXISTS ops_scheduled_reports CASCADE;
DROP TABLE IF EXISTS ops_alert_events CASCADE;
DROP TABLE IF EXISTS ops_alert_rules CASCADE;

DROP TABLE IF EXISTS ops_system_metrics CASCADE;
DROP TABLE IF EXISTS ops_error_logs CASCADE;

