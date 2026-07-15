package repository

import (
	"context"
	"database/sql"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type auditRetentionRepository struct {
	sql sqlExecutor
}

func NewAuditRetentionRepository(_ *dbent.Client, sqlDB *sql.DB) service.AuditRetentionRepository {
	return &auditRetentionRepository{sql: sqlDB}
}

// deleteOlderThan 按 created_at 批量删除单张审计表中超过保留期的旧行。
// ent 不支持 LIMIT 删除，这里用 CTE victims 原生 SQL 先选出待删主键再删除。
func (r *auditRetentionRepository) deleteOlderThan(ctx context.Context, table string, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	query := `
		WITH victims AS (
			SELECT id
			FROM ` + table + `
			WHERE created_at <= $1
			ORDER BY created_at ASC
			LIMIT $2
		)
		DELETE FROM ` + table + `
		WHERE id IN (SELECT id FROM victims)
	`
	res, err := r.sql.ExecContext(ctx, query, cutoff, limit)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *auditRetentionRepository) DeleteAIAuditLogsOlderThan(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return r.deleteOlderThan(ctx, "ai_audit_logs", cutoff, limit)
}

func (r *auditRetentionRepository) DeleteCodexInviteResetHistoryOlderThan(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return r.deleteOlderThan(ctx, "codex_invite_reset_history", cutoff, limit)
}

func (r *auditRetentionRepository) DeleteDeletedAPIKeyAuditsOlderThan(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return r.deleteOlderThan(ctx, "deleted_api_key_audits", cutoff, limit)
}
