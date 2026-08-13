package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

// UsageUserDailyCostRepository 定义每用户·每业务日用量成本预聚合表的幂等重算与保留清理能力。
type UsageUserDailyCostRepository interface {
	RecomputeDay(ctx context.Context, bucketDate string, dayStartUTC, dayEndUTC time.Time) error
	DeleteOlderThan(ctx context.Context, cutoffDate string) (int64, error)
}

// UsageUserDailyCostAggregator 周期性按业务日全量重算 usage_user_daily_cost，供管理后台用量排序读取。
type UsageUserDailyCostAggregator struct {
	repo          UsageUserDailyCostRepository
	enabled       bool
	interval      time.Duration
	backfillDays  int
	retentionDays int

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
}

func NewUsageUserDailyCostAggregator(repo UsageUserDailyCostRepository, cfg *config.Config) *UsageUserDailyCostAggregator {
	enabled := true
	interval := 300 * time.Second
	backfillDays := 31
	retentionDays := 35
	if cfg != nil {
		if cfg.UsageUserDailyCost.IntervalSeconds > 0 {
			interval = time.Duration(cfg.UsageUserDailyCost.IntervalSeconds) * time.Second
		}
		if cfg.UsageUserDailyCost.BackfillDays > 0 {
			backfillDays = cfg.UsageUserDailyCost.BackfillDays
		}
		if cfg.UsageUserDailyCost.RetentionDays > 0 {
			retentionDays = cfg.UsageUserDailyCost.RetentionDays
		}
		if cfg.UsageUserDailyCost.Disabled {
			enabled = false
		}
	}
	return &UsageUserDailyCostAggregator{
		repo:          repo,
		enabled:       enabled,
		interval:      interval,
		backfillDays:  backfillDays,
		retentionDays: retentionDays,
		stopCh:        make(chan struct{}),
	}
}

func (s *UsageUserDailyCostAggregator) Start() {
	if s == nil || s.repo == nil {
		return
	}
	if !s.enabled {
		SetUsageUserDailyCostRollupReady(false)
		logger.LegacyPrintf("service.usage_user_daily_cost", "[UsageUserDailyCost] disabled by config, skip start")
		return
	}
	s.startOnce.Do(func() {
		SetUsageUserDailyCostRollupReady(false)
		logger.LegacyPrintf("service.usage_user_daily_cost", "[UsageUserDailyCost] started interval=%s backfillDays=%d retentionDays=%d", s.interval, s.backfillDays, s.retentionDays)
		go s.runLoop()
	})
}

func (s *UsageUserDailyCostAggregator) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
		logger.LegacyPrintf("service.usage_user_daily_cost", "[UsageUserDailyCost] stopped")
	})
}

func (s *UsageUserDailyCostAggregator) runLoop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.refreshReadiness()

	for {
		select {
		case <-ticker.C:
			if !IsUsageUserDailyCostRollupReady() {
				s.refreshReadiness()
				continue
			}
			_ = s.recomputeDay(0)
			_ = s.recomputeDay(1)
			s.cleanup()
		case <-s.stopCh:
			return
		}
	}
}

func (s *UsageUserDailyCostAggregator) refreshReadiness() {
	if s.backfill() {
		SetUsageUserDailyCostRollupReady(true)
		return
	}
	SetUsageUserDailyCostRollupReady(false)
}

func (s *UsageUserDailyCostAggregator) backfill() bool {
	ok := true
	for i := 0; i < s.backfillDays; i++ {
		if err := s.recomputeDay(i); err != nil {
			ok = false
		}
	}
	return ok
}

func (s *UsageUserDailyCostAggregator) recomputeDay(daysAgo int) error {
	todayStart := timezone.Today()
	dayStart := todayStart.AddDate(0, 0, -daysAgo)
	dayEnd := dayStart.AddDate(0, 0, 1)
	bucketDate := dayStart.Format("2006-01-02")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := s.repo.RecomputeDay(ctx, bucketDate, dayStart.UTC(), dayEnd.UTC()); err != nil {
		logger.LegacyPrintf("service.usage_user_daily_cost", "[UsageUserDailyCost] recompute day failed bucketDate=%s err=%v", bucketDate, err)
		return err
	}
	return nil
}

func (s *UsageUserDailyCostAggregator) cleanup() {
	cutoffDate := timezone.Today().AddDate(0, 0, -s.retentionDays).Format("2006-01-02")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	deleted, err := s.repo.DeleteOlderThan(ctx, cutoffDate)
	if err != nil {
		logger.LegacyPrintf("service.usage_user_daily_cost", "[UsageUserDailyCost] cleanup failed cutoff=%s err=%v", cutoffDate, err)
		return
	}
	if deleted > 0 {
		logger.LegacyPrintf("service.usage_user_daily_cost", "[UsageUserDailyCost] cleaned old rows cutoff=%s count=%d", cutoffDate, deleted)
	}
}
