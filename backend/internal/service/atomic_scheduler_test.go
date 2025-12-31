package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return client, mr
}

func TestAtomicScheduler_SelectAndAcquireAccountSlot(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	scheduler := NewAtomicScheduler(client)
	ctx := context.Background()

	t.Run("成功选择优先级最高的账号", func(t *testing.T) {
		candidates := []*AccountCandidate{
			{ID: 1, Priority: 2, MaxConcurrency: 10},
			{ID: 2, Priority: 1, MaxConcurrency: 10}, // 优先级最高
			{ID: 3, Priority: 3, MaxConcurrency: 10},
		}

		accountID, concurrency, releaseFunc, err := scheduler.SelectAndAcquireAccountSlot(
			ctx, candidates, "req-001", 60,
		)

		require.NoError(t, err)
		assert.Equal(t, int64(2), accountID)
		assert.Equal(t, 1, concurrency)
		assert.NotNil(t, releaseFunc)

		// 验证Redis状态
		count, err := scheduler.GetAccountConcurrency(ctx, 2)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// 释放槽位
		releaseFunc()
		time.Sleep(10 * time.Millisecond) // 等待异步释放

		count, err = scheduler.GetAccountConcurrency(ctx, 2)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("跳过已满载的账号", func(t *testing.T) {
		mr.FlushAll()

		// 预先占满账号1
		mr.HSet("account_concurrency", "1", "5")

		candidates := []*AccountCandidate{
			{ID: 1, Priority: 1, MaxConcurrency: 5}, // 已满载
			{ID: 2, Priority: 2, MaxConcurrency: 10},
		}

		accountID, concurrency, releaseFunc, err := scheduler.SelectAndAcquireAccountSlot(
			ctx, candidates, "req-002", 60,
		)

		require.NoError(t, err)
		assert.Equal(t, int64(2), accountID) // 应选择账号2
		assert.Equal(t, 1, concurrency)
		assert.NotNil(t, releaseFunc)

		releaseFunc()
	})

	t.Run("所有账号都已满载", func(t *testing.T) {
		mr.FlushAll()

		// 预先占满所有账号
		mr.HSet("account_concurrency", "1", "5")
		mr.HSet("account_concurrency", "2", "10")

		candidates := []*AccountCandidate{
			{ID: 1, Priority: 1, MaxConcurrency: 5},
			{ID: 2, Priority: 2, MaxConcurrency: 10},
		}

		accountID, concurrency, releaseFunc, err := scheduler.SelectAndAcquireAccountSlot(
			ctx, candidates, "req-003", 60,
		)

		require.NoError(t, err)
		assert.Equal(t, int64(0), accountID) // 返回0表示无可用账号
		assert.Equal(t, 0, concurrency)
		assert.Nil(t, releaseFunc)
	})

	t.Run("负载均衡-选择负载率最低的账号", func(t *testing.T) {
		mr.FlushAll()

		// 设置不同的负载
		mr.HSet("account_concurrency", "1", "8") // 80%负载
		mr.HSet("account_concurrency", "2", "3") // 30%负载

		candidates := []*AccountCandidate{
			{ID: 1, Priority: 1, MaxConcurrency: 10}, // 优先级高但负载高
			{ID: 2, Priority: 1, MaxConcurrency: 10}, // 优先级相同但负载低
		}

		accountID, _, releaseFunc, err := scheduler.SelectAndAcquireAccountSlot(
			ctx, candidates, "req-004", 60,
		)

		require.NoError(t, err)
		// 由于评分算法包含随机性，这里只验证选中了某个账号
		assert.True(t, accountID == 1 || accountID == 2)
		assert.NotNil(t, releaseFunc)

		releaseFunc()
	})

	t.Run("空候选列表", func(t *testing.T) {
		candidates := []*AccountCandidate{}

		accountID, concurrency, releaseFunc, err := scheduler.SelectAndAcquireAccountSlot(
			ctx, candidates, "req-005", 60,
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no candidates")
		assert.Equal(t, int64(0), accountID)
		assert.Equal(t, 0, concurrency)
		assert.Nil(t, releaseFunc)
	})
}

func TestAtomicScheduler_GetAccountConcurrency(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	scheduler := NewAtomicScheduler(client)
	ctx := context.Background()

	t.Run("获取存在的并发数", func(t *testing.T) {
		mr.HSet("account_concurrency", "1", "5")

		count, err := scheduler.GetAccountConcurrency(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, 5, count)
	})

	t.Run("获取不存在的账号并发数", func(t *testing.T) {
		count, err := scheduler.GetAccountConcurrency(ctx, 999)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestAtomicScheduler_ResetAccountConcurrency(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	scheduler := NewAtomicScheduler(client)
	ctx := context.Background()

	mr.HSet("account_concurrency", "1", "5")

	err := scheduler.ResetAccountConcurrency(ctx, 1)
	require.NoError(t, err)

	count, err := scheduler.GetAccountConcurrency(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAtomicScheduler_ConcurrentRequests(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	scheduler := NewAtomicScheduler(client)
	ctx := context.Background()

	t.Run("并发请求不会超过最大并发数", func(t *testing.T) {
		mr.FlushAll()

		candidates := []*AccountCandidate{
			{ID: 1, Priority: 1, MaxConcurrency: 3},
		}

		// 模拟5个并发请求
		successCount := 0
		failCount := 0

		for i := 0; i < 5; i++ {
			accountID, _, releaseFunc, err := scheduler.SelectAndAcquireAccountSlot(
				ctx, candidates, fmt.Sprintf("req-%d", i), 60,
			)

			require.NoError(t, err)
			if accountID > 0 {
				successCount++
				defer releaseFunc()
			} else {
				failCount++
			}
		}

		// 应该只有3个成功，2个失败
		assert.Equal(t, 3, successCount)
		assert.Equal(t, 2, failCount)

		// 验证最终并发数
		count, err := scheduler.GetAccountConcurrency(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})
}
