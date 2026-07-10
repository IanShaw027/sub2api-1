//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type batchImageUserDeletionGuard interface {
	EnsureUserCanDeleteBatchImageState(ctx context.Context, userID int64) error
}

func TestUserRepositoryEnsureUserCanDeleteBatchImageState(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newUserRepositoryWithSQL(client, integrationDB)
	guard, ok := any(repo).(batchImageUserDeletionGuard)
	require.True(t, ok, "user repository must expose the batch image deletion guard")

	createUser := func(t *testing.T) *service.User {
		t.Helper()
		return mustCreateUser(t, client, &service.User{
			Email:        fmt.Sprintf("batch-image-delete-%s@example.com", uuid.NewString()),
			PasswordHash: "hash",
			Balance:      1,
		})
	}
	createJob := func(t *testing.T, userID int64, status string) {
		t.Helper()
		_, err := integrationDB.ExecContext(ctx, `
			INSERT INTO batch_image_jobs (
				batch_id, user_id, provider, model, status, item_count
			) VALUES ($1, $2, $3, $4, $5, 1)
		`, "imgbatch_delete_"+uuid.NewString(), userID, service.BatchImageProviderGeminiAPI, "gemini-2.5-flash-image", status)
		require.NoError(t, err)
	}

	t.Run("active job", func(t *testing.T) {
		user := createUser(t)
		createJob(t, user.ID, service.BatchImageJobStatusRunning)

		err := guard.EnsureUserCanDeleteBatchImageState(ctx, user.ID)

		require.ErrorContains(t, err, "active batch image jobs")
	})

	t.Run("frozen balance", func(t *testing.T) {
		user := createUser(t)
		_, err := integrationDB.ExecContext(ctx,
			"UPDATE users SET frozen_balance = 0.1234567810 WHERE id = $1", user.ID,
		)
		require.NoError(t, err)

		err = guard.EnsureUserCanDeleteBatchImageState(ctx, user.ID)

		require.ErrorContains(t, err, "frozen batch image balance")
	})

	t.Run("terminal jobs and zero frozen balance", func(t *testing.T) {
		user := createUser(t)
		createJob(t, user.ID, service.BatchImageJobStatusCompleted)
		createJob(t, user.ID, service.BatchImageJobStatusFailed)
		createJob(t, user.ID, service.BatchImageJobStatusCancelled)
		createJob(t, user.ID, service.BatchImageJobStatusOutputDeleted)

		require.NoError(t, guard.EnsureUserCanDeleteBatchImageState(ctx, user.ID))
	})
}

func TestUserRepositoryEnsureUserCanDeleteBatchImageState_BlocksConcurrentJobInsertUntilDeleteCommits(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newUserRepositoryWithSQL(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("batch-image-delete-lock-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      1,
	})
	otherUser := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("batch-image-delete-other-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      1,
	})
	batchID := "imgbatch_delete_lock_" + uuid.NewString()
	otherBatchID := "imgbatch_delete_other_" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM batch_image_jobs WHERE batch_id IN ($1, $2)", batchID, otherBatchID)
	})

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	txCtx := dbent.NewTxContext(ctx, tx)
	require.NoError(t, repo.EnsureUserCanDeleteBatchImageState(txCtx, user.ID))
	require.NoError(t, repo.Delete(txCtx, user.ID))

	batchRepo := newBatchImageRepositoryWithSQL(integrationDB)
	_, err = batchRepo.CreateBatchImageJob(ctx, service.CreateBatchImageJobParams{
		BatchID:   otherBatchID,
		UserID:    otherUser.ID,
		Provider:  service.BatchImageProviderGeminiAPI,
		Model:     "gemini-2.5-flash-image",
		Status:    service.BatchImageJobStatusCreated,
		ItemCount: 1,
	})
	require.NoError(t, err, "different users must not share the deletion advisory lock")

	insertDone := make(chan error, 1)
	go func() {
		_, insertErr := batchRepo.CreateBatchImageJob(ctx, service.CreateBatchImageJobParams{
			BatchID:   batchID,
			UserID:    user.ID,
			Provider:  service.BatchImageProviderGeminiAPI,
			Model:     "gemini-2.5-flash-image",
			Status:    service.BatchImageJobStatusCreated,
			ItemCount: 1,
		})
		insertDone <- insertErr
	}()

	select {
	case insertErr := <-insertDone:
		require.NoError(t, insertErr)
		t.Fatal("batch image job insert completed before deletion transaction ended")
	case <-time.After(150 * time.Millisecond):
	}

	require.NoError(t, tx.Commit())
	select {
	case insertErr := <-insertDone:
		require.ErrorIs(t, insertErr, service.ErrUserNotFound)
	case <-time.After(2 * time.Second):
		t.Fatal("batch image job insert remained blocked after deletion transaction commit")
	}

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM batch_image_jobs WHERE batch_id = $1", batchID,
	).Scan(&count))
	require.Zero(t, count, "deleted user must not gain a batch image job after commit")
}
