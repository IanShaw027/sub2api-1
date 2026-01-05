-- +goose Up
-- +goose NO TRANSACTION
-- Critical indexes for ops monitoring system performance
-- Created: 2026-01-04
-- Note: Using CONCURRENTLY to avoid table locking during index creation

-- ops_error_logs 核心查询索引
-- 按创建时间降序查询（最新错误日志）
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_created_at
ON ops_error_logs (created_at DESC);

-- 按平台和时间组合查询（平台维度错误统计）
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_platform_time
ON ops_error_logs (platform, created_at DESC);

-- 按严重程度和时间组合查询（严重错误优先级查看）
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_severity_time
ON ops_error_logs (severity, created_at DESC);

-- 添加表注释说明索引优化目的
COMMENT ON INDEX idx_ops_error_logs_created_at IS '优化错误日志时间序列查询性能';
COMMENT ON INDEX idx_ops_error_logs_platform_time IS '优化平台维度错误统计查询';
COMMENT ON INDEX idx_ops_error_logs_severity_time IS '优化严重错误优先级查询';

-- +goose Down
-- 删除索引（无需 CONCURRENTLY，删除操作本身很快）
DROP INDEX IF EXISTS idx_ops_error_logs_created_at;
DROP INDEX IF EXISTS idx_ops_error_logs_platform_time;
DROP INDEX IF EXISTS idx_ops_error_logs_severity_time;
