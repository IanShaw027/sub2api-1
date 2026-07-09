-- Non-transactional migration: CREATE INDEX CONCURRENTLY cannot run in a transaction.
--
-- 审计表自动保留(TTL purge)按 created_at 批量删除旧行（AuditRetentionService）。删除 SQL 形如
-- WHERE created_at <= $cutoff ORDER BY created_at ASC LIMIT $batch，需要各表在 created_at 上有
-- 可走的单列索引，否则大表下每轮清理退化为全表扫描。
--
-- 现状：ai_audit_logs 已有独立 (created_at) 索引（144_create_ai_center_core.sql）。
-- 以下 4 张表原本只有以 created_at 为“非前导列”的复合索引或完全没有 created_at 索引，
-- 无法服务纯 created_at 范围扫描，这里补齐独立单列索引。CONCURRENTLY 避免建索引期间阻塞审计写入。

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ai_skill_runs_created_at
    ON ai_skill_runs (created_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ai_skill_settlements_created_at
    ON ai_skill_settlements (created_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_codex_invite_reset_history_created_at
    ON codex_invite_reset_history (created_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS deletedapikeyaudit_created_at
    ON deleted_api_key_audits (created_at);
