package service

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const userPlatformQuotaDBBatchInterval = 250 * time.Millisecond

var defaultUserPlatformQuotaDBAggregator = newUserPlatformQuotaDBAggregator(userPlatformQuotaDBBatchInterval)

type userPlatformQuotaDBAggregator struct {
	mu       sync.Mutex
	interval time.Duration
	timer    *time.Timer
	pending  map[userPlatformQuotaDBIncrementKey]userPlatformQuotaDBIncrement
}

type userPlatformQuotaDBIncrementKey struct {
	repoID   uintptr
	userID   int64
	platform string
}

type userPlatformQuotaDBIncrement struct {
	repo     UserPlatformQuotaRepository
	userID   int64
	platform string
	cost     float64
	count    int64
}

func newUserPlatformQuotaDBAggregator(interval time.Duration) *userPlatformQuotaDBAggregator {
	if interval <= 0 {
		interval = userPlatformQuotaDBBatchInterval
	}
	return &userPlatformQuotaDBAggregator{
		interval: interval,
		pending:  make(map[userPlatformQuotaDBIncrementKey]userPlatformQuotaDBIncrement),
	}
}

func enqueueUserPlatformQuotaDBIncrement(repo UserPlatformQuotaRepository, userID int64, platform string, cost float64) {
	defaultUserPlatformQuotaDBAggregator.enqueue(repo, userID, platform, cost)
}

func (a *userPlatformQuotaDBAggregator) enqueue(repo UserPlatformQuotaRepository, userID int64, platform string, cost float64) {
	if a == nil || repo == nil || userID == 0 || platform == "" || cost <= 0 {
		return
	}

	key := userPlatformQuotaDBIncrementKey{
		repoID:   userPlatformQuotaDBRepoID(repo),
		userID:   userID,
		platform: platform,
	}

	a.mu.Lock()
	incr := a.pending[key]
	if incr.repo == nil {
		incr.repo = repo
		incr.userID = userID
		incr.platform = platform
	}
	incr.cost += cost
	incr.count++
	a.pending[key] = incr
	if a.timer == nil {
		a.timer = time.AfterFunc(a.interval, a.flushScheduled)
	}
	a.mu.Unlock()
}

func (a *userPlatformQuotaDBAggregator) flushScheduled() {
	defer func() {
		if r := recover(); r != nil {
			logger.LegacyPrintf("service.gateway", "ALERT: panic in user platform quota DB batch flush: %v", r)
		}
	}()
	a.flushPending()
}

func (a *userPlatformQuotaDBAggregator) flushPending() {
	if a == nil {
		return
	}

	pending := a.drainPending()
	for _, incr := range pending {
		a.flushIncrement(incr)
	}
}

func (a *userPlatformQuotaDBAggregator) drainPending() []userPlatformQuotaDBIncrement {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}
	if len(a.pending) == 0 {
		return nil
	}

	pending := make([]userPlatformQuotaDBIncrement, 0, len(a.pending))
	for _, incr := range a.pending {
		pending = append(pending, incr)
	}
	a.pending = make(map[userPlatformQuotaDBIncrementKey]userPlatformQuotaDBIncrement)
	return pending
}

func (a *userPlatformQuotaDBAggregator) flushIncrement(incr userPlatformQuotaDBIncrement) {
	if incr.repo == nil || incr.userID == 0 || incr.platform == "" || incr.cost <= 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), postUsageBillingTimeout)
	defer cancel()

	if err := incr.repo.IncrementUsageWithReset(ctx, incr.userID, incr.platform, incr.cost, time.Now().UTC()); err != nil {
		failedCount := incr.count
		if failedCount <= 0 {
			failedCount = 1
		}
		userPlatformQuotaDBIncrErrorTotal.Add(failedCount)
		logger.LegacyPrintf(
			"service.gateway",
			"ALERT: incr user platform quota DB failed user=%d platform=%s cost=%f count=%d: %v",
			incr.userID,
			incr.platform,
			incr.cost,
			failedCount,
			err,
		)
	}
}

func userPlatformQuotaDBRepoID(repo UserPlatformQuotaRepository) uintptr {
	value := reflect.ValueOf(repo)
	if !value.IsValid() {
		return 0
	}
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer:
		if value.IsNil() {
			return 0
		}
		return value.Pointer()
	default:
		return 0
	}
}
