package repository

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSchedulerOutboxRepositoryClaimPendingLeasesOldestAvailableEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &schedulerOutboxRepository{db: db}
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	mock.ExpectQuery(`(?s)WITH selected AS MATERIALIZED.*dedup_key = NULL`).
		WithArgs(int64(150), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_type", "account_id", "group_id", "payload", "created_at", "claim_token",
		}).AddRow(int64(9), "account_changed", int64(42), nil, []byte(`{"group_ids":[7]}`), createdAt, "db-claim-token"))

	event, err := repo.ClaimPending(context.Background(), 150*time.Second)

	require.NoError(t, err)
	require.NotNil(t, event)
	require.EqualValues(t, 9, event.ID)
	require.Equal(t, "db-claim-token", event.ClaimToken)
	require.EqualValues(t, 42, *event.AccountID)
	require.Equal(t, []any{float64(7)}, event.Payload["group_ids"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSchedulerOutboxRepositoryClaimPendingReturnsNilWhenEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &schedulerOutboxRepository{db: db}
	mock.ExpectQuery("WITH selected AS MATERIALIZED").
		WithArgs(int64(150), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_type", "account_id", "group_id", "payload", "created_at", "claim_token",
		}))

	event, err := repo.ClaimPending(context.Background(), 150*time.Second)

	require.NoError(t, err)
	require.Nil(t, event)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSchedulerOutboxRepositoryClaimPendingRejectsInvalidLease(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &schedulerOutboxRepository{db: db}
	event, err := repo.ClaimPending(context.Background(), 0)

	require.Error(t, err)
	require.Nil(t, event)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSchedulerOutboxRepositoryAckClaimChecksOwnership(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &schedulerOutboxRepository{db: db}
	mock.ExpectExec(regexp.QuoteMeta(`
		DELETE FROM scheduler_outbox
		WHERE id = $1 AND claim_token = $2
	`)).WithArgs(int64(9), "owner-token").WillReturnResult(sqlmock.NewResult(0, 0))

	acked, err := repo.AckClaim(context.Background(), 9, "owner-token")

	require.NoError(t, err)
	require.False(t, acked)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSchedulerOutboxRepositoryReleaseClaimChecksOwnership(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &schedulerOutboxRepository{db: db}
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE scheduler_outbox
		SET claimed_at = NULL, claim_token = NULL
		WHERE id = $1 AND claim_token = $2
	`)).WithArgs(int64(9), "owner-token").WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.ReleaseClaim(context.Background(), 9, "owner-token"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSchedulerOutboxRepositoryOldestPendingCreatedAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &schedulerOutboxRepository{db: db}
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	mock.ExpectQuery("SELECT MIN\\(created_at\\) FROM scheduler_outbox").
		WillReturnRows(sqlmock.NewRows([]string{"min"}).AddRow(createdAt))

	got, ok, err := repo.OldestPendingCreatedAt(context.Background())

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, createdAt, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSchedulerOutboxRepositoryOldestPendingCreatedAtReturnsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &schedulerOutboxRepository{db: db}
	mock.ExpectQuery("SELECT MIN\\(created_at\\) FROM scheduler_outbox").
		WillReturnRows(sqlmock.NewRows([]string{"min"}).AddRow(nil))

	got, ok, err := repo.OldestPendingCreatedAt(context.Background())

	require.NoError(t, err)
	require.False(t, ok)
	require.True(t, got.IsZero())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSchedulerOutboxRepositoryPendingCountIgnoresInFlightClaims(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &schedulerOutboxRepository{db: db}
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM scheduler_outbox").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))

	got, err := repo.PendingCount(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 2, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

// buildSchedulerGroupPayload 在 groupIDs 为空时必须返回 untyped nil（any），
// 否则 enqueueSchedulerOutbox 的 "payload != nil" 接口判空会被 typed-nil 欺骗。
func TestEnqueueSchedulerOutbox_UngroupedAccountDedupesWithLiteralNilPayload(t *testing.T) {
	accountID := int64(42)
	keyLiteralNil := schedulerOutboxDedupKey("account_changed", &accountID, nil, nil)

	emptyGroupsPayload := buildSchedulerGroupPayload(nil)
	require.Nil(t, emptyGroupsPayload)

	var payloadJSON []byte
	if emptyGroupsPayload != nil {
		t.Fatal("typed-nil regression: empty groups payload interface should be nil")
	}
	keyEmptyGroups := schedulerOutboxDedupKey("account_changed", &accountID, nil, payloadJSON)
	require.Equal(t, keyLiteralNil, keyEmptyGroups)
}

func TestSchedulerOutboxAccountBulkChangedSupportsPendingDedup(t *testing.T) {
	require.True(t, schedulerOutboxEventSupportsDedup(service.SchedulerOutboxEventAccountBulkChanged))
	payload, err := json.Marshal(map[string]any{"account_ids": []int64{7, 11}})
	require.NoError(t, err)
	require.Equal(t,
		schedulerOutboxDedupKey(service.SchedulerOutboxEventAccountBulkChanged, nil, nil, payload),
		schedulerOutboxDedupKey(service.SchedulerOutboxEventAccountBulkChanged, nil, nil, payload),
	)
}
