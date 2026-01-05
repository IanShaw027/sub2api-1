-- +goose Up
-- 简化 OpsMetrics 结构体 - 删除对 API 转发系统不必要的过度监控字段
-- 从 63 个字段精简到 30 个核心字段

-- 1. 删除磁盘监控字段（3个）
ALTER TABLE ops_system_metrics
    DROP COLUMN IF EXISTS disk_used,
    DROP COLUMN IF EXISTS disk_total,
    DROP COLUMN IF EXISTS disk_iops;

-- 2. 删除网络监控字段（2个）
ALTER TABLE ops_system_metrics
    DROP COLUMN IF EXISTS network_in_bytes,
    DROP COLUMN IF EXISTS network_out_bytes;

-- 3. 删除 GC 监控字段（2个）
ALTER TABLE ops_system_metrics
    DROP COLUMN IF EXISTS heap_alloc_mb,
    DROP COLUMN IF EXISTS gc_pause_ms;

-- 4. 删除过细的延迟指标（latency_p999保留P99即可）
ALTER TABLE ops_system_metrics
    DROP COLUMN IF EXISTS latency_p999;

-- 5. 删除 Tags 字段（未使用）
ALTER TABLE ops_system_metrics
    DROP COLUMN IF EXISTS tags;

-- 6. 删除 HTTP2 错误统计（未定义）
ALTER TABLE ops_system_metrics
    DROP COLUMN IF EXISTS http2_errors;

-- 7. 删除重复的延迟字段（p95_latency_ms, p99_latency_ms 与 latency_p95, latency_p99 重复）
-- 保留浮点型的 latency_p95, latency_p99
ALTER TABLE ops_system_metrics
    DROP COLUMN IF EXISTS p95_latency_ms,
    DROP COLUMN IF EXISTS p99_latency_ms;

-- 8. 添加缺失的 latency_p95 和 latency_p99 列（如果不存在）
-- 这些是实际需要保留的延迟字段
ALTER TABLE ops_system_metrics
    ADD COLUMN IF NOT EXISTS latency_p95 DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS latency_p99 DOUBLE PRECISION;

-- 添加注释说明简化原因
COMMENT ON TABLE ops_system_metrics IS '运维系统指标表（已简化）- 从63个字段精简到30个核心字段，删除磁盘/网络/GC等过度监控指标';

-- +goose Down
-- 回滚时恢复删除的字段

ALTER TABLE ops_system_metrics
    ADD COLUMN IF NOT EXISTS disk_used BIGINT,
    ADD COLUMN IF NOT EXISTS disk_total BIGINT,
    ADD COLUMN IF NOT EXISTS disk_iops BIGINT,
    ADD COLUMN IF NOT EXISTS network_in_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS network_out_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS heap_alloc_mb BIGINT,
    ADD COLUMN IF NOT EXISTS gc_pause_ms DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS latency_p999 DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS tags JSONB,
    ADD COLUMN IF NOT EXISTS http2_errors INT,
    ADD COLUMN IF NOT EXISTS p95_latency_ms INT,
    ADD COLUMN IF NOT EXISTS p99_latency_ms INT;

ALTER TABLE ops_system_metrics
    DROP COLUMN IF EXISTS latency_p95,
    DROP COLUMN IF EXISTS latency_p99;
