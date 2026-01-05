package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/infraerror"
	"github.com/redis/go-redis/v9"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

const (
	opsMetricsInterval       = 1 * time.Minute
	opsMetricsCollectTimeout = 10 * time.Second

	opsMetricsWindowShortMinutes = 1
	opsMetricsWindowLongMinutes  = 5
	opsMetricsWindowHourlyMinutes = 60

	opsMetricsCollectorCacheKeyPrefix = "ops:metrics:collector:window:"

	bytesPerMB             = 1024 * 1024
	cpuUsageSampleInterval = 0 * time.Second

	percentScale = 100
)

type OpsMetricsCollector struct {
	opsService         *OpsService
	concurrencyService *ConcurrencyService
	redisClient        *redis.Client
	cacheEnabled       bool
	cacheTTL           time.Duration
	cacheHits          uint64
	cacheMisses        uint64
	lastCacheStatsMu   sync.Mutex
	lastCacheHits      uint64
	lastCacheMisses    uint64
	interval           time.Duration
	lastGCPauseTotal   uint64
	lastGCPauseMu      sync.Mutex
	stopCh             chan struct{}
	startOnce          sync.Once
	stopOnce           sync.Once

	distributedLockOn    bool
	distributedLockKey   string
	distributedLockTTL   time.Duration
	distributedLockWarn  sync.Once
	distributedSkipLogMu sync.Mutex
	distributedSkipLogAt time.Time
}

func NewOpsMetricsCollector(
	opsService *OpsService,
	concurrencyService *ConcurrencyService,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsMetricsCollector {
	cacheEnabled := false
	cacheTTL := 60 * time.Second
	if cfg != nil {
		cacheEnabled = cfg.Ops.MetricsCollectorCache.Enabled
		if cfg.Ops.MetricsCollectorCache.TTL > 0 {
			cacheTTL = cfg.Ops.MetricsCollectorCache.TTL
		}
	}
	return &OpsMetricsCollector{
		opsService:         opsService,
		concurrencyService: concurrencyService,
		redisClient:        redisClient,
		cacheEnabled:       cacheEnabled,
		cacheTTL:           cacheTTL,
		interval:           opsMetricsInterval,

		distributedLockOn:  true,
		distributedLockKey: opsMetricsCollectorLeaderLockKeyDefault,
		distributedLockTTL: opsMetricsCollectorLeaderLockTTLDefault,
	}
}

func (c *OpsMetricsCollector) Start() {
	if c == nil {
		return
	}
	c.startOnce.Do(func() {
		if c.stopCh == nil {
			c.stopCh = make(chan struct{})
		}
		go c.run()
	})
}

func (c *OpsMetricsCollector) Stop() {
	if c == nil {
		return
	}
	c.stopOnce.Do(func() {
		if c.stopCh != nil {
			close(c.stopCh)
		}
	})
}

func (c *OpsMetricsCollector) run() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.collectOnce()
	for {
		select {
		case <-ticker.C:
			c.collectOnce()
		case <-c.stopCh:
			return
		}
	}
}

func (c *OpsMetricsCollector) collectOnce() {
	if c.opsService == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), opsMetricsCollectTimeout)
	defer cancel()

	releaseLeaderLock, ok := c.tryAcquireLeaderLock(ctx)
	if !ok {
		return
	}
	if releaseLeaderLock != nil {
		defer releaseLeaderLock()
	}

	now := time.Now()
	// Use stable minute boundaries to maximize Redis cache hits across replicas.
	// This also prevents small clock skews from producing unique cache keys.
	windowEnd := now.Truncate(time.Minute)
	systemStats := c.collectSystemStats(ctx)
	queueDepth := c.collectQueueDepth(ctx)
	activeAlerts := c.collectActiveAlerts(ctx)

	for _, window := range []int{opsMetricsWindowShortMinutes, opsMetricsWindowLongMinutes, opsMetricsWindowHourlyMinutes} {
		startTime := windowEnd.Add(-time.Duration(window) * time.Minute)
		windowStats, err := c.getWindowStatsCached(ctx, window, startTime, windowEnd)
		if err != nil {
			log.Printf("[OpsMetrics] failed to get window stats (%dm): %v", window, err)
			continue
		}

		successRate, errorRate := computeRates(windowStats.SuccessCount, windowStats.ErrorCount)
		requestCount := windowStats.SuccessCount + windowStats.ErrorCount
		windowSeconds := float64(window * 60)
		metric := &OpsMetrics{
			WindowMinutes:         window,
			RequestCount:          requestCount,
			QPS:                   float64(requestCount) / windowSeconds,
			SuccessCount:          windowStats.SuccessCount,
			ErrorCount:            windowStats.ErrorCount,
			SuccessRate:           successRate,
			ErrorRate:             errorRate,
			TokenConsumed:         windowStats.TokenConsumed,
			TPS:                   float64(windowStats.TokenConsumed) / windowSeconds,
			TokenRate:             float64(windowStats.TokenConsumed) / windowSeconds,
			LatencyP95:            float64(windowStats.P95LatencyMs),
			LatencyP99:            float64(windowStats.P99LatencyMs),
			ActiveAlerts:          activeAlerts,
			CPUUsagePercent:       systemStats.cpuUsage,
			MemoryUsedMB:          systemStats.memoryUsedMB,
			MemoryTotalMB:         systemStats.memoryTotalMB,
			MemoryUsagePercent:    systemStats.memoryUsagePercent,
			ConcurrencyQueueDepth: queueDepth,
			UpdatedAt:             now,
		}

		if err := c.opsService.RecordMetrics(ctx, metric); err != nil {
			log.Printf("[OpsMetrics] failed to record metrics (%dm): %v", window, err)
		}
	}

	if c.cacheEnabled && c.redisClient != nil {
		hits := atomic.LoadUint64(&c.cacheHits)
		misses := atomic.LoadUint64(&c.cacheMisses)

		c.lastCacheStatsMu.Lock()
		deltaHits := hits - c.lastCacheHits
		deltaMisses := misses - c.lastCacheMisses
		c.lastCacheHits = hits
		c.lastCacheMisses = misses
		c.lastCacheStatsMu.Unlock()

		total := deltaHits + deltaMisses
		hitRate := 0.0
		if total > 0 {
			hitRate = float64(deltaHits) / float64(total) * percentScale
		}
		log.Printf("[OpsMetrics] window-stats cache hits=%d misses=%d hit_rate=%.1f%% ttl=%s", deltaHits, deltaMisses, hitRate, c.cacheTTL)
	}
}

const (
	opsMetricsCollectorLeaderLockKeyDefault = "ops:metrics:collector:leader"
	// TTL must outlive a single collection (opsMetricsCollectTimeout) and should cover
	// occasional GC pauses; collection is expected to finish within ~10s.
	opsMetricsCollectorLeaderLockTTLDefault = 90 * time.Second

	opsMetricsCollectorLeaderLockSkipLogMinInterval = 1 * time.Minute
)

var opsMetricsCollectorLeaderUnlockScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0
`)

var opsMetricsCollectorLeaderRenewScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0
`)

func (c *OpsMetricsCollector) tryAcquireLeaderLock(ctx context.Context) (func(), bool) {
	if c == nil || !c.distributedLockOn {
		return nil, true
	}
	if ctx == nil {
		ctx = context.Background()
	}

	key := strings.TrimSpace(c.distributedLockKey)
	if key == "" {
		key = opsMetricsCollectorLeaderLockKeyDefault
	}
	ttl := c.distributedLockTTL
	if ttl <= 0 {
		ttl = opsMetricsCollectorLeaderLockTTLDefault
	}

	if c.redisClient == nil {
		c.distributedLockWarn.Do(func() {
			log.Printf("[OpsMetrics] distributed lock enabled but redis client is nil; proceeding without leader lock (key=%q)", key)
		})
		return nil, true
	}

	token := opsAlertLeaderToken()
	ok, err := c.redisClient.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		log.Printf("[OpsMetrics] failed to acquire leader lock (key=%q): %v", key, err)
		return nil, false
	}
	if !ok {
		c.logLeaderLockSkipped(key)
		return nil, false
	}

	renewCancel, renewDone := c.startLeaderLockRenewal(key, token, ttl)

	release := func() {
		if renewCancel != nil {
			renewCancel()
		}
		if renewDone != nil {
			select {
			case <-renewDone:
			case <-time.After(2 * time.Second):
				log.Printf("[OpsMetrics] leader lock renewal goroutine did not stop in time (key=%q)", key)
			}
		}

		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if _, err := opsMetricsCollectorLeaderUnlockScript.Run(releaseCtx, c.redisClient, []string{key}, token).Int(); err != nil {
			log.Printf("[OpsMetrics] failed to release leader lock (key=%q token=%s): %v", key, shortenLockToken(token), err)
		}
	}

	return release, true
}

func (c *OpsMetricsCollector) startLeaderLockRenewal(key string, token string, ttl time.Duration) (context.CancelFunc, <-chan struct{}) {
	if c == nil || c.redisClient == nil {
		return nil, nil
	}
	if strings.TrimSpace(key) == "" || token == "" || ttl <= 0 {
		return nil, nil
	}

	refreshEvery := ttl / 2
	if refreshEvery < 5*time.Second {
		refreshEvery = 5 * time.Second
	}
	ttlMillis := ttl.Milliseconds()
	if ttlMillis <= 0 {
		ttlMillis = 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)

		ticker := time.NewTicker(refreshEvery)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				res, err := opsMetricsCollectorLeaderRenewScript.Run(context.Background(), c.redisClient, []string{key}, token, ttlMillis).Int()
				if err != nil {
					log.Printf("[OpsMetrics] leader lock renewal failed (key=%q token=%s): %v", key, shortenLockToken(token), err)
					continue
				}
				if res == 0 {
					log.Printf("[OpsMetrics] leader lock no longer owned; stop renewing (key=%q token=%s)", key, shortenLockToken(token))
					return
				}
			}
		}
	}()

	return cancel, done
}

func (c *OpsMetricsCollector) logLeaderLockSkipped(key string) {
	if c == nil {
		return
	}
	now := time.Now()

	c.distributedSkipLogMu.Lock()
	defer c.distributedSkipLogMu.Unlock()

	if !c.distributedSkipLogAt.IsZero() && now.Sub(c.distributedSkipLogAt) < opsMetricsCollectorLeaderLockSkipLogMinInterval {
		return
	}
	c.distributedSkipLogAt = now
	log.Printf("[OpsMetrics] skipped collection; leader lock held by another instance (key=%q)", key)
}

func (c *OpsMetricsCollector) getWindowStatsCached(
	ctx context.Context,
	windowMinutes int,
	startTime time.Time,
	endTime time.Time,
) (*OpsWindowStats, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil || c.opsService == nil {
		return nil, nil
	}

	// Cache is optional; always fall back to DB on any cache issue.
	if c.cacheEnabled && c.redisClient != nil {
		key := fmt.Sprintf("%s%d:end:%d", opsMetricsCollectorCacheKeyPrefix, windowMinutes, endTime.UTC().Unix())
		data, err := c.redisClient.Get(ctx, key).Bytes()
		if err == nil {
			var stats OpsWindowStats
			unmarshalErr := json.Unmarshal(data, &stats)
			if unmarshalErr == nil {
				atomic.AddUint64(&c.cacheHits, 1)
				return &stats, nil
			}
			// Corrupt payload shouldn't break collection; treat as miss and recompute.
			atomic.AddUint64(&c.cacheMisses, 1)
			log.Printf("[OpsMetrics] failed to unmarshal cached window stats (%dm): %v", windowMinutes, unmarshalErr)
			_ = c.redisClient.Del(ctx, key).Err()
		} else if errors.Is(err, redis.Nil) {
			atomic.AddUint64(&c.cacheMisses, 1)
		} else {
			atomic.AddUint64(&c.cacheMisses, 1)
			infraerror.RecordInfrastructureError(ctx, "redis", "OpsMetricsCollector.getWindowStatsCached.get", err)
		}
	}

	stats, err := c.opsService.GetWindowStats(ctx, startTime, endTime)
	if err != nil {
		return nil, err
	}

	if c.cacheEnabled && c.redisClient != nil {
		key := fmt.Sprintf("%s%d:end:%d", opsMetricsCollectorCacheKeyPrefix, windowMinutes, endTime.UTC().Unix())
		payload, marshalErr := json.Marshal(stats)
		if marshalErr != nil {
			log.Printf("[OpsMetrics] failed to marshal window stats for cache (%dm): %v", windowMinutes, marshalErr)
			return stats, nil
		}
		if setErr := c.redisClient.Set(ctx, key, payload, c.cacheTTL).Err(); setErr != nil {
			infraerror.RecordInfrastructureError(ctx, "redis", "OpsMetricsCollector.getWindowStatsCached.set", setErr)
		}
	}

	return stats, nil
}

func computeRates(successCount, errorCount int64) (float64, float64) {
	total := successCount + errorCount
	if total == 0 {
		// No traffic => no data. Rates are kept at 0 and request_count will be 0.
		// The UI should render this as N/A instead of "100% success".
		return 0, 0
	}
	successRate := float64(successCount) / float64(total) * percentScale
	errorRate := float64(errorCount) / float64(total) * percentScale
	return successRate, errorRate
}

type opsSystemStats struct {
	cpuUsage           float64
	memoryUsedMB       int64
	memoryTotalMB      int64
	memoryUsagePercent float64
}

func (c *OpsMetricsCollector) collectSystemStats(ctx context.Context) opsSystemStats {
	stats := opsSystemStats{}

	if percents, err := cpu.PercentWithContext(ctx, cpuUsageSampleInterval, false); err == nil && len(percents) > 0 {
		stats.cpuUsage = percents[0]
	}

	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		stats.memoryUsedMB = int64(vm.Used / bytesPerMB)
		stats.memoryTotalMB = int64(vm.Total / bytesPerMB)
		stats.memoryUsagePercent = vm.UsedPercent
	}

	return stats
}

func (c *OpsMetricsCollector) collectQueueDepth(ctx context.Context) int {
	if c.concurrencyService == nil {
		return 0
	}
	depth, err := c.concurrencyService.GetTotalWaitCount(ctx)
	if err != nil {
		log.Printf("[OpsMetrics] failed to get queue depth: %v", err)
		return 0
	}
	return depth
}

func (c *OpsMetricsCollector) collectActiveAlerts(ctx context.Context) int {
	if c.opsService == nil {
		return 0
	}
	count, err := c.opsService.CountActiveAlerts(ctx)
	if err != nil {
		return 0
	}
	return count
}
