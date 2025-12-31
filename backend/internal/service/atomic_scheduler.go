package service

import (
	_ "embed"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed atomic_select.lua
var atomicSelectLua string

// AccountCandidate 候选账号信息
type AccountCandidate struct {
	ID             int64 `json:"id"`
	Priority       int   `json:"priority"`
	MaxConcurrency int   `json:"max_concurrency"`
}

// AtomicScheduler 原子化账号调度器
type AtomicScheduler struct {
	redis     *redis.Client
	luaScript *redis.Script
}

// NewAtomicScheduler 创建原子化调度器
func NewAtomicScheduler(redisClient *redis.Client) *AtomicScheduler {
	return &AtomicScheduler{
		redis:     redisClient,
		luaScript: redis.NewScript(atomicSelectLua),
	}
}

// SelectAndAcquireAccountSlot 原子化选择账号并占用槽位
// 返回: 选中的账号ID, 当前并发数, 释放函数, 错误
func (s *AtomicScheduler) SelectAndAcquireAccountSlot(
	ctx context.Context,
	candidates []*AccountCandidate,
	requestID string,
	timeout int,
) (int64, int, func(), error) {
	if len(candidates) == 0 {
		return 0, 0, nil, fmt.Errorf("no candidates provided")
	}

	// 构建Lua脚本参数
	// ARGV[1]: 候选账号数量
	// ARGV[2+]: 每个账号的 (id, priority, max_concurrency) 三个一组
	// ARGV[last-1]: requestID
	// ARGV[last]: timeout
	args := make([]interface{}, 0, 2+len(candidates)*3+2)
	args = append(args, len(candidates))

	for _, c := range candidates {
		args = append(args, c.ID, c.Priority, c.MaxConcurrency)
	}

	args = append(args, requestID, timeout)

	// 执行Lua脚本
	result, err := s.luaScript.Run(ctx, s.redis, nil, args...).Result()
	if err != nil {
		return 0, 0, nil, fmt.Errorf("lua script execution failed: %w", err)
	}

	// 解析返回值
	resultSlice, ok := result.([]interface{})
	if !ok || len(resultSlice) != 2 {
		return 0, 0, nil, fmt.Errorf("unexpected lua script result format")
	}

	accountID, ok1 := resultSlice[0].(int64)
	currentConcurrency, ok2 := resultSlice[1].(int64)
	if !ok1 || !ok2 {
		return 0, 0, nil, fmt.Errorf("unexpected lua script result types")
	}

	// 如果返回0表示所有账号都已满载
	if accountID == 0 {
		return 0, 0, nil, nil
	}

	// 创建释放函数
	releaseFunc := func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 递减并发计数
		if err := s.redis.HIncrBy(bgCtx, "account_concurrency", fmt.Sprintf("%d", accountID), -1).Err(); err != nil {
			// 日志记录但不返回错误
			fmt.Printf("Warning: failed to decrement concurrency for account %d: %v\n", accountID, err)
		}

		// 删除槽位标记
		slotKey := fmt.Sprintf("slot:%d:%s", accountID, requestID)
		if err := s.redis.Del(bgCtx, slotKey).Err(); err != nil {
			fmt.Printf("Warning: failed to delete slot key %s: %v\n", slotKey, err)
		}
	}

	return accountID, int(currentConcurrency), releaseFunc, nil
}

// GetAccountConcurrency 获取账号当前并发数
func (s *AtomicScheduler) GetAccountConcurrency(ctx context.Context, accountID int64) (int, error) {
	val, err := s.redis.HGet(ctx, "account_concurrency", fmt.Sprintf("%d", accountID)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	var count int
	if _, err := fmt.Sscanf(val, "%d", &count); err != nil {
		return 0, fmt.Errorf("parse concurrency count failed: %w", err)
	}

	return count, nil
}

// ResetAccountConcurrency 重置账号并发计数（用于维护）
func (s *AtomicScheduler) ResetAccountConcurrency(ctx context.Context, accountID int64) error {
	return s.redis.HDel(ctx, "account_concurrency", fmt.Sprintf("%d", accountID)).Err()
}
