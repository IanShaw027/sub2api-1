//go:build unit

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestCreationVideoQueueRestartRecoveryAndLeaseFencing(t *testing.T) {
	mr := miniredis.RunT(t)
	now := time.Now().UTC()
	mr.SetTime(now)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	q := NewCreationVideoObservationQueue(rdb)
	ctx := context.Background()
	job := &service.CreationVideoObservation{ID: "job-1", UserID: 7, GroupID: 3, APIKeyID: 9, SubscriptionID: 42}
	require.NoError(t, q.Reserve(ctx, job, 1, 2*time.Minute))
	require.ErrorIs(t, q.Reserve(ctx, &service.CreationVideoObservation{ID: "job-2"}, 1, 2*time.Minute), service.ErrCreationVideoObservationFull)
	require.NoError(t, q.Bind(ctx, job.ID, "upstream-1", 30*time.Minute))
	first, err := q.Claim(ctx, "worker-before-restart", time.Minute)
	require.NoError(t, err)
	require.Equal(t, "upstream-1", first.TaskID)
	qAfterRestart := NewCreationVideoObservationQueue(rdb)
	blocked, err := qAfterRestart.Claim(ctx, "worker-after-restart", time.Minute)
	require.NoError(t, err)
	require.Nil(t, blocked)
	mr.FastForward(61 * time.Second)
	mr.SetTime(now.Add(61 * time.Second))
	recovered, err := qAfterRestart.Claim(ctx, "worker-after-restart", time.Minute)
	require.NoError(t, err)
	require.Equal(t, first.ID, recovered.ID)
	require.Equal(t, int64(42), recovered.SubscriptionID)
	require.ErrorIs(t, q.Finish(ctx, first.ID, first.LeaseToken, true, 0), service.ErrCreationVideoObservationMissing)
	require.NoError(t, qAfterRestart.Finish(ctx, recovered.ID, recovered.LeaseToken, true, 0))
	require.EqualValues(t, 0, rdb.HLen(ctx, creationVideoQueueKeys[0]).Val())
	require.EqualValues(t, 0, rdb.ZCard(ctx, creationVideoQueueKeys[1]).Val())
	require.EqualValues(t, 0, rdb.ZCard(ctx, creationVideoQueueKeys[2]).Val())
}

func TestCreationVideoQueueReservationsExpireAndBoundJobsCannotBeCancelled(t *testing.T) {
	mr := miniredis.RunT(t)
	now := time.Now().UTC()
	mr.SetTime(now)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	q := NewCreationVideoObservationQueue(rdb)
	ctx := context.Background()
	require.NoError(t, q.Reserve(ctx, &service.CreationVideoObservation{ID: "unknown-acceptance"}, 1, time.Minute))
	mr.FastForward(61 * time.Second)
	mr.SetTime(now.Add(61 * time.Second))
	require.NoError(t, q.Reserve(ctx, &service.CreationVideoObservation{ID: "accepted", UserID: 7, GroupID: 3, APIKeyID: 9}, 1, time.Minute))
	require.NoError(t, q.Bind(ctx, "accepted", "task", 2*time.Minute))
	require.NoError(t, q.CancelReservation(ctx, "accepted"))
	job, err := q.Claim(ctx, "worker", time.Minute)
	require.NoError(t, err)
	require.NotNil(t, job)
	serialized := rdb.HGet(ctx, creationVideoQueueKeys[0], "accepted").Val()
	for _, forbidden := range []string{"prompt", "authorization", "cookie", "image", "jwt"} {
		require.NotContains(t, strings.ToLower(serialized), forbidden)
	}
	mr.FastForward(3 * time.Minute)
	mr.SetTime(now.Add(4*time.Minute + time.Second))
	expired, err := q.Claim(ctx, "worker", time.Minute)
	require.NoError(t, err)
	require.Nil(t, expired)
	require.EqualValues(t, 0, rdb.HLen(ctx, creationVideoQueueKeys[0]).Val())
}
