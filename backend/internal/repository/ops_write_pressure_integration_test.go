//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsRepositoryBatchInsertErrorLogs(t *testing.T) {
	ctx := context.Background()
	_, _ = integrationDB.ExecContext(ctx, "TRUNCATE ops_error_logs RESTART IDENTITY")

	repo := NewOpsRepository(integrationDB).(*opsRepository)
	now := time.Now().UTC()
	inserted, err := repo.BatchInsertErrorLogs(ctx, []*service.OpsInsertErrorLogInput{
		{
			RequestID:    "batch-ops-1",
			ErrorPhase:   "upstream",
			ErrorType:    "upstream_error",
			Severity:     "error",
			StatusCode:   429,
			ErrorMessage: "rate limited",
			CreatedAt:    now,
		},
		{
			RequestID:    "batch-ops-2",
			ErrorPhase:   "internal",
			ErrorType:    "api_error",
			Severity:     "error",
			StatusCode:   500,
			ErrorMessage: "internal error",
			CreatedAt:    now.Add(time.Millisecond),
		},
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, inserted)

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM ops_error_logs WHERE request_id IN ('batch-ops-1', 'batch-ops-2')").Scan(&count))
	require.Equal(t, 2, count)
}

func TestEnqueueSchedulerOutbox_DeduplicatesIdempotentEvents(t *testing.T) {
	ctx := context.Background()
	_, _ = integrationDB.ExecContext(ctx, "TRUNCATE scheduler_outbox RESTART IDENTITY")

	accountID := int64(12345)
	require.NoError(t, enqueueSchedulerOutbox(ctx, integrationDB, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil))
	require.NoError(t, enqueueSchedulerOutbox(ctx, integrationDB, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil))

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE event_type = $1", service.SchedulerOutboxEventAccountChanged).Scan(&count))
	require.Equal(t, 1, count)

	var firstID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id FROM scheduler_outbox WHERE event_type = $1", service.SchedulerOutboxEventAccountChanged).Scan(&firstID))
	event, err := NewSchedulerOutboxRepository(integrationDB).ClaimPending(ctx, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, event)
	require.Equal(t, firstID, event.ID)

	require.NoError(t, enqueueSchedulerOutbox(ctx, integrationDB, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE event_type = $1", service.SchedulerOutboxEventAccountChanged).Scan(&count))
	require.Equal(t, 2, count)
}

func TestSchedulerOutbox_ClaimPendingAllowsSameKeyWhileEventInFlight(t *testing.T) {
	ctx := context.Background()
	_, _ = integrationDB.ExecContext(ctx, "TRUNCATE scheduler_outbox RESTART IDENTITY")

	accountID := int64(17345)
	require.NoError(t, enqueueSchedulerOutbox(ctx, integrationDB, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil))

	event, err := NewSchedulerOutboxRepository(integrationDB).ClaimPending(ctx, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, event)

	require.NoError(t, enqueueSchedulerOutbox(ctx, integrationDB, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil))

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE event_type = $1", service.SchedulerOutboxEventAccountChanged).Scan(&count))
	require.Equal(t, 2, count)

	var pendingKeys int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE dedup_key IS NOT NULL").Scan(&pendingKeys))
	require.Equal(t, 1, pendingKeys)
}

func TestSchedulerOutbox_ClaimPendingDoesNotSkipLateCommitWithLowerSequenceID(t *testing.T) {
	ctx := context.Background()
	_, _ = integrationDB.ExecContext(ctx, "TRUNCATE scheduler_outbox RESTART IDENTITY")

	slowTx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = slowTx.Rollback() })

	var slowID int64
	require.NoError(t, slowTx.QueryRowContext(ctx, `
		INSERT INTO scheduler_outbox (event_type) VALUES ($1) RETURNING id
	`, service.SchedulerOutboxEventAccountLastUsed).Scan(&slowID))

	var fastID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO scheduler_outbox (event_type) VALUES ($1) RETURNING id
	`, service.SchedulerOutboxEventAccountLastUsed).Scan(&fastID))
	require.Greater(t, fastID, slowID)

	repo := NewSchedulerOutboxRepository(integrationDB)
	fastEvent, err := repo.ClaimPending(ctx, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, fastEvent)
	require.Equal(t, fastID, fastEvent.ID)
	acked, err := repo.AckClaim(ctx, fastEvent.ID, fastEvent.ClaimToken)
	require.NoError(t, err)
	require.True(t, acked)

	require.NoError(t, slowTx.Commit())
	slowEvent, err := repo.ClaimPending(ctx, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, slowEvent, "late committed row must remain claimable after a higher ID is acknowledged")
	require.Equal(t, slowID, slowEvent.ID)
}

func TestSchedulerOutbox_ClaimPendingSkipsActiveClaimAndReclaimsExpiredLease(t *testing.T) {
	ctx := context.Background()
	_, _ = integrationDB.ExecContext(ctx, "TRUNCATE scheduler_outbox RESTART IDENTITY")

	for range 2 {
		require.NoError(t, enqueueSchedulerOutbox(
			ctx, integrationDB, service.SchedulerOutboxEventAccountLastUsed, nil, nil, nil,
		))
	}

	repo := NewSchedulerOutboxRepository(integrationDB)
	first, err := repo.ClaimPending(ctx, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, first)
	second, err := repo.ClaimPending(ctx, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.NotEqual(t, first.ID, second.ID, "active claims must be exclusive across workers")

	_, err = integrationDB.ExecContext(ctx, `
		UPDATE scheduler_outbox SET claimed_at = NOW() - INTERVAL '2 minutes' WHERE id = $1
	`, first.ID)
	require.NoError(t, err)
	reclaimed, err := repo.ClaimPending(ctx, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, reclaimed)
	require.Equal(t, first.ID, reclaimed.ID)
	require.NotEqual(t, first.ClaimToken, reclaimed.ClaimToken)

	acked, err := repo.AckClaim(ctx, first.ID, first.ClaimToken)
	require.NoError(t, err)
	require.False(t, acked, "an expired owner must not acknowledge a newer claim")
}

func TestEnqueueSchedulerOutbox_CoalescesAccountStateBurst(t *testing.T) {
	ctx := context.Background()
	_, _ = integrationDB.ExecContext(ctx, "TRUNCATE scheduler_outbox RESTART IDENTITY")

	accountID := int64(22345)
	for range 50 {
		require.NoError(t, enqueueSchedulerOutbox(ctx, integrationDB, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil))
	}

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE event_type = $1", service.SchedulerOutboxEventAccountChanged).Scan(&count))
	t.Logf("same-account account_changed burst: calls=50 inserted=%d", count)
	require.Equal(t, 1, count)
}

func TestEnqueueSchedulerOutbox_DoesNotDeduplicateDifferentPayload(t *testing.T) {
	ctx := context.Background()
	_, _ = integrationDB.ExecContext(ctx, "TRUNCATE scheduler_outbox RESTART IDENTITY")

	accountID := int64(32345)
	payload1 := map[string]any{"group_ids": []int64{1}}
	payload2 := map[string]any{"group_ids": []int64{2}}
	require.NoError(t, enqueueSchedulerOutbox(ctx, integrationDB, service.SchedulerOutboxEventAccountChanged, &accountID, nil, payload1))
	require.NoError(t, enqueueSchedulerOutbox(ctx, integrationDB, service.SchedulerOutboxEventAccountChanged, &accountID, nil, payload2))

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE event_type = $1", service.SchedulerOutboxEventAccountChanged).Scan(&count))
	require.Equal(t, 2, count)
}

func TestEnqueueSchedulerOutbox_DoesNotDeduplicateLastUsed(t *testing.T) {
	ctx := context.Background()
	_, _ = integrationDB.ExecContext(ctx, "TRUNCATE scheduler_outbox RESTART IDENTITY")

	accountID := int64(67890)
	payload1 := map[string]any{"last_used": map[string]int64{"67890": 100}}
	payload2 := map[string]any{"last_used": map[string]int64{"67890": 200}}
	require.NoError(t, enqueueSchedulerOutbox(ctx, integrationDB, service.SchedulerOutboxEventAccountLastUsed, &accountID, nil, payload1))
	require.NoError(t, enqueueSchedulerOutbox(ctx, integrationDB, service.SchedulerOutboxEventAccountLastUsed, &accountID, nil, payload2))

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE event_type = $1", service.SchedulerOutboxEventAccountLastUsed).Scan(&count))
	require.Equal(t, 2, count)
}
