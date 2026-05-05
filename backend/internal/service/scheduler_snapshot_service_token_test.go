//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type schedulerRebuildTokenCache struct {
	SchedulerCache
	tryLockToken     string
	setSnapshotToken string
	unlockToken      string
}

func (c *schedulerRebuildTokenCache) TryLockBucket(ctx context.Context, bucket SchedulerBucket, ttl time.Duration) (bool, error) {
	SetSchedulerBucketLockToken(ctx, "lock-token")
	c.tryLockToken = SchedulerBucketLockToken(ctx)
	return true, nil
}

func (c *schedulerRebuildTokenCache) SetSnapshot(ctx context.Context, bucket SchedulerBucket, accounts []Account) error {
	c.setSnapshotToken = SchedulerBucketLockToken(ctx)
	return nil
}

func (c *schedulerRebuildTokenCache) UnlockBucket(ctx context.Context, bucket SchedulerBucket) error {
	c.unlockToken = SchedulerBucketLockToken(ctx)
	return nil
}

type schedulerRebuildTokenAccountRepo struct {
	AccountRepository
	loadToken string
}

func (r *schedulerRebuildTokenAccountRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	r.loadToken = SchedulerBucketLockToken(ctx)
	return []Account{
		{
			ID:          88,
			Platform:    platform,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
		},
	}, nil
}

func TestSchedulerSnapshotServiceRebuildBucketPropagatesLockToken(t *testing.T) {
	cache := &schedulerRebuildTokenCache{}
	accountRepo := &schedulerRebuildTokenAccountRepo{}
	svc := &SchedulerSnapshotService{
		cache:       cache,
		accountRepo: accountRepo,
	}

	bucket := SchedulerBucket{GroupID: 5, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}
	require.NoError(t, svc.rebuildBucket(context.Background(), bucket, "test"))
	require.Equal(t, "lock-token", cache.tryLockToken)
	require.Equal(t, "lock-token", accountRepo.loadToken)
	require.Equal(t, "lock-token", cache.setSnapshotToken)
	require.Equal(t, "lock-token", cache.unlockToken)
}
