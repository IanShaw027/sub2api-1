package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *usageLogRepository) RecordOpenAIStickyScheduleEvent(ctx context.Context, input *service.OpenAIStickyScheduleEventInput) error {
	if r == nil || r.sql == nil {
		return fmt.Errorf("nil usage log repository")
	}
	if input == nil || input.StickyAccountID <= 0 {
		return nil
	}

	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err := r.sql.ExecContext(ctx, `
		INSERT INTO ops_sticky_schedule_events (
			created_at,
			platform,
			group_id,
			api_key_id,
			sticky_account_id,
			selected_account_id,
			sticky_original_unavailable,
			reason,
			sticky_source
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`,
		createdAt.UTC(),
		strings.ToLower(strings.TrimSpace(input.Platform)),
		nullInt64(input.GroupID),
		input.APIKeyID,
		input.StickyAccountID,
		nullInt64(input.SelectedAccountID),
		input.StickyOriginalUnavailable,
		strings.TrimSpace(input.Reason),
		strings.TrimSpace(input.StickySource),
	)
	return err
}
