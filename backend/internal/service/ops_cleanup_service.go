package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

const (
	opsCleanupLeaderLockKeyDefault = "ops:cleanup:leader"
	opsCleanupLeaderLockTTLDefault = 30 * time.Minute
)

var opsCleanupCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// OpsCleanupService periodically deletes old ops data to prevent unbounded DB growth.
//
// It is scheduled via a 5-field cron spec (minute hour dom month dow).
// In multi-instance deployments, it uses a Redis leader lock (best-effort) so only one instance runs cleanup.
type OpsCleanupService struct {
	repo       OpsIngestRepository
	redisClient *redis.Client
	cfg        *config.Config

	enabled  bool
	schedule string
	location *time.Location

	cron     *cron.Cron
	entryID  cron.EntryID
	startOnce sync.Once
	stopOnce  sync.Once

	leaderLockOn   bool
	leaderLockKey  string
	leaderLockTTL  time.Duration
	leaderLockWarn sync.Once
}

func NewOpsCleanupService(repo OpsIngestRepository, redisClient *redis.Client, cfg *config.Config) *OpsCleanupService {
	svc := &OpsCleanupService{
		repo:        repo,
		redisClient: redisClient,
		cfg:         cfg,
		enabled:     true,
		schedule:    "0 2 * * *",
		location:    time.Local,

		leaderLockOn:  true,
		leaderLockKey: opsCleanupLeaderLockKeyDefault,
		leaderLockTTL: opsCleanupLeaderLockTTLDefault,
	}

	if cfg != nil {
		svc.enabled = cfg.Ops.Cleanup.Enabled
		if strings.TrimSpace(cfg.Ops.Cleanup.Schedule) != "" {
			svc.schedule = strings.TrimSpace(cfg.Ops.Cleanup.Schedule)
		}
		if strings.TrimSpace(cfg.Timezone) != "" {
			if loc, err := time.LoadLocation(strings.TrimSpace(cfg.Timezone)); err == nil && loc != nil {
				svc.location = loc
			}
		}
		if cfg.RunMode == config.RunModeSimple {
			svc.leaderLockOn = false
		}
	}

	return svc
}

func (s *OpsCleanupService) Start() {
	if s == nil {
		return
	}
	if !s.enabled {
		log.Printf("[OpsCleanup] not started (disabled)")
		return
	}
	if s.repo == nil {
		log.Printf("[OpsCleanup] not started (missing repository)")
		return
	}

	s.startOnce.Do(func() {
		if s.location == nil {
			s.location = time.Local
		}

		c := cron.New(cron.WithParser(opsCleanupCronParser), cron.WithLocation(s.location))
		entryID, err := c.AddFunc(s.schedule, func() {
			s.runScheduled()
		})
		if err != nil {
			log.Printf("[OpsCleanup] not started (invalid schedule=%q): %v", s.schedule, err)
			return
		}

		s.cron = c
		s.entryID = entryID
		s.cron.Start()
		log.Printf("[OpsCleanup] started (schedule=%q tz=%s leader_lock=%v)", s.schedule, s.location.String(), s.leaderLockOn)
	})
}

func (s *OpsCleanupService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				log.Printf("[OpsCleanup] cron stop timed out")
			}
		}
	})
}

func (s *OpsCleanupService) runScheduled() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	release, ok := s.tryAcquireLeaderLock(ctx)
	if !ok {
		return
	}
	if release != nil {
		defer release()
	}

	deleted, err := s.runCleanupOnce(ctx)
	if err != nil {
		log.Printf("[OpsCleanup] cleanup run failed: %v", err)
		return
	}
	log.Printf("[OpsCleanup] cleanup complete: %s", deleted)
}

type opsCleanupDeletedCounts struct {
	errorLogs int64
	metrics   map[int]int64
}

func (c opsCleanupDeletedCounts) String() string {
	if len(c.metrics) == 0 {
		return fmt.Sprintf("deleted_error_logs=%d", c.errorLogs)
	}

	parts := make([]string, 0, len(c.metrics)+1)
	parts = append(parts, fmt.Sprintf("deleted_error_logs=%d", c.errorLogs))
	for _, window := range []int{1, 5, 60, 1440} {
		if n, ok := c.metrics[window]; ok {
			parts = append(parts, fmt.Sprintf("deleted_metrics_window_%d=%d", window, n))
		}
	}
	return strings.Join(parts, " ")
}

func (s *OpsCleanupService) runCleanupOnce(ctx context.Context) (opsCleanupDeletedCounts, error) {
	if s == nil || s.repo == nil || s.cfg == nil {
		return opsCleanupDeletedCounts{}, nil
	}

	out := opsCleanupDeletedCounts{metrics: map[int]int64{}}

	// Error logs cleanup
	if days := s.cfg.Ops.Cleanup.ErrorLogRetentionDays; days > 0 {
		n, err := s.repo.DeleteOldErrorLogs(ctx, days)
		if err != nil {
			return out, err
		}
		out.errorLogs = n
	}

	// Minute-level metrics are stored in ops_system_metrics (window_minutes=1/5).
	if days := s.cfg.Ops.Cleanup.MinuteMetricsRetentionDays; days > 0 {
		for _, window := range []int{1, 5} {
			n, err := s.repo.DeleteOldMetrics(ctx, window, days)
			if err != nil && !errorsIsUndefinedTable(err) {
				return out, err
			}
			out.metrics[window] = n
		}
	}

	// Hourly-level metrics:
	// - ops_system_metrics (window_minutes=60) for backward compatibility
	// - ops_metrics_hourly for pre-aggregation
	// Daily pre-aggregation (ops_metrics_daily) is kept in sync with the same retention window.
	if days := s.cfg.Ops.Cleanup.HourlyMetricsRetentionDays; days > 0 {
		for _, window := range []int{60, 1440} {
			n, err := s.repo.DeleteOldMetrics(ctx, window, days)
			if err != nil && !errorsIsUndefinedTable(err) {
				return out, err
			}
			out.metrics[window] = n
		}
	}

	return out, nil
}

func (s *OpsCleanupService) tryAcquireLeaderLock(ctx context.Context) (func(), bool) {
	if s == nil || !s.leaderLockOn {
		return nil, true
	}
	key := strings.TrimSpace(s.leaderLockKey)
	if key == "" {
		key = opsCleanupLeaderLockKeyDefault
	}
	ttl := s.leaderLockTTL
	if ttl <= 0 {
		ttl = opsCleanupLeaderLockTTLDefault
	}

	opts := RedisLeaderLockOptions{
		Enabled:         true,
		Redis:           s.redisClient,
		Key:             key,
		TTL:             ttl,
		LogPrefix:       "[OpsCleanup]",
		WarnNoRedisOnce: &s.leaderLockWarn,
		OnSkip:          nil,
		LogAcquired:     true,
		LogReleased:     true,
	}
	return TryAcquireRedisLeaderLock(ctx, opts)
}

// errorsIsUndefinedTable guards cleanup in environments where pre-aggregation tables may not exist yet.
// We only use this for optional cleanup targets; core ops tables should always exist once ops is enabled.
func errorsIsUndefinedTable(err error) bool {
	if err == nil {
		return false
	}
	// Best-effort string match to avoid hard dependency on pg driver error types.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") && strings.Contains(msg, "relation")
}
