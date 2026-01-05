-- +goose Up
-- 添加 GIN 全文索引用于错误消息搜索
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_msg_gin
ON ops_error_logs USING gin (to_tsvector('english', error_message));

-- 添加复合索引用于常见查询模式
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_composite
ON ops_error_logs (created_at DESC, platform, status_code);

-- +goose Down
DROP INDEX IF EXISTS idx_ops_error_logs_msg_gin;
DROP INDEX IF EXISTS idx_ops_error_logs_composite;
