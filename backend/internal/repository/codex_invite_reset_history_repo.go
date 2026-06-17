package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type codexInviteResetHistoryRepository struct {
	sql sqlExecutor
}

// NewCodexInviteResetHistoryRepository 创建 Codex 邀请/重置历史流水仓储。
func NewCodexInviteResetHistoryRepository(_ *dbent.Client, sqlDB *sql.DB) service.CodexInviteResetHistoryRepository {
	return &codexInviteResetHistoryRepository{sql: sqlDB}
}

func (r *codexInviteResetHistoryRepository) Create(ctx context.Context, entry *service.CodexInviteResetHistoryEntry) error {
	if entry == nil {
		return nil
	}
	emailsJSON, err := json.Marshal(codexHistoryStringSlice(entry.Emails))
	if err != nil {
		return err
	}
	failedJSON, err := json.Marshal(codexHistoryStringSlice(entry.FailedEmails))
	if err != nil {
		return err
	}
	const query = `
		INSERT INTO codex_invite_reset_history (
			account_id, action_type, operator_user_id, emails, failed_emails,
			credit_id, success, result_code, message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = r.sql.ExecContext(ctx, query,
		entry.AccountID,
		entry.ActionType,
		entry.OperatorUserID,
		emailsJSON,
		failedJSON,
		entry.CreditID,
		entry.Success,
		entry.ResultCode,
		entry.Message,
	)
	return err
}

func (r *codexInviteResetHistoryRepository) ListByAccount(ctx context.Context, accountID int64, params pagination.PaginationParams) ([]service.CodexInviteResetHistoryEntry, *pagination.PaginationResult, error) {
	var total int64
	countRows, err := r.sql.QueryContext(ctx, `SELECT COUNT(*) FROM codex_invite_reset_history WHERE account_id = $1`, accountID)
	if err != nil {
		return nil, nil, err
	}
	if countRows.Next() {
		if err := countRows.Scan(&total); err != nil {
			_ = countRows.Close()
			return nil, nil, err
		}
	}
	if err := countRows.Err(); err != nil {
		_ = countRows.Close()
		return nil, nil, err
	}
	_ = countRows.Close()

	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, account_id, action_type, operator_user_id, emails, failed_emails,
			credit_id, success, result_code, message, created_at
		FROM codex_invite_reset_history
		WHERE account_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, accountID, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.CodexInviteResetHistoryEntry, 0, params.Limit())
	for rows.Next() {
		var (
			entry      service.CodexInviteResetHistoryEntry
			emailsRaw  []byte
			failedRaw  []byte
			operatorID sql.NullInt64
		)
		if err := rows.Scan(
			&entry.ID,
			&entry.AccountID,
			&entry.ActionType,
			&operatorID,
			&emailsRaw,
			&failedRaw,
			&entry.CreditID,
			&entry.Success,
			&entry.ResultCode,
			&entry.Message,
			&entry.CreatedAt,
		); err != nil {
			return nil, nil, err
		}
		if operatorID.Valid {
			id := operatorID.Int64
			entry.OperatorUserID = &id
		}
		entry.Emails = codexHistoryUnmarshalStrings(emailsRaw)
		entry.FailedEmails = codexHistoryUnmarshalStrings(failedRaw)
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

// codexHistoryStringSlice 保证写入的 JSON 是 [] 而非 null。
func codexHistoryStringSlice(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func codexHistoryUnmarshalStrings(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
