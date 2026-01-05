-- +goose Up
-- 添加客户端 IP 和创建时间复合索引以优化查询性能
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_client_ip_time
ON ops_error_logs(client_ip, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_ops_error_logs_client_ip_time;
