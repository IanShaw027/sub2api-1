package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
)

func (s *OpsQueryService) GetDashboardOverview(ctx context.Context, timeRange string) (*DashboardOverviewData, error) {
	if s == nil {
		return nil, errors.New("ops service not initialized")
	}
	repo := s.repo
	if repo == nil {
		return nil, errors.New("ops repository not initialized")
	}
	if s.sqlDB == nil {
		return nil, errors.New("ops service not initialized")
	}
	if strings.TrimSpace(timeRange) == "" {
		timeRange = "1h"
	}

	duration, err := parseTimeRange(timeRange)
	if err != nil {
		return nil, err
	}

	if cached, ok := s.getDashboardOverviewFromLocalCache(timeRange); ok && cached != nil {
		return cached, nil
	}

	if cached, err := repo.GetCachedDashboardOverview(ctx, timeRange); err == nil && cached != nil {
		s.setDashboardOverviewLocalCache(timeRange, cached, opsDashboardLocalCacheTTL)
		return cached, nil
	}

	key := "dashboard_overview:" + strings.TrimSpace(timeRange)
	v, err, _ := s.dashboardSF.Do(key, func() (any, error) {
		// Double-check local/Redis caches to avoid recomputing after waiting on the singleflight.
		if cached, ok := s.getDashboardOverviewFromLocalCache(timeRange); ok && cached != nil {
			return cached, nil
		}
		if cached, err := repo.GetCachedDashboardOverview(ctx, timeRange); err == nil && cached != nil {
			s.setDashboardOverviewLocalCache(timeRange, cached, opsDashboardLocalCacheTTL)
			return cached, nil
		}

		// Use a background context so a single disconnected client doesn't cancel the shared refresh.
		// Per-query timeouts still ensure we don't overrun the DB in pathological cases.
		workCtx := context.Background()

		now := time.Now().UTC()
		startTime := now.Add(-duration)

		ctxStats, cancelStats := context.WithTimeout(workCtx, opsDBQueryTimeout)
		stats, err := repo.GetOverviewStats(ctxStats, startTime, now)
		cancelStats()
		if err != nil {
			return nil, fmt.Errorf("get overview stats: %w", err)
		}
		if stats == nil {
			return nil, errors.New("get overview stats returned nil")
		}

		var statsYesterday *OverviewStats
		{
			yesterdayEnd := now.Add(-24 * time.Hour)
			yesterdayStart := yesterdayEnd.Add(-duration)
			ctxYesterday, cancelYesterday := context.WithTimeout(workCtx, opsDBQueryTimeout)
			ys, err := repo.GetOverviewStats(ctxYesterday, yesterdayStart, yesterdayEnd)
			cancelYesterday()
			if err != nil {
				// Best-effort: overview should still work when historical comparison fails.
				log.Printf("[OpsOverview] get yesterday overview stats failed: %v", err)
			} else {
				statsYesterday = ys
			}
		}

		totalReqs := stats.SuccessCount + stats.ErrorCount
		successRate, errorRate := calculateRates(stats.SuccessCount, stats.ErrorCount, totalReqs)

		successRateYesterday := 0.0
		totalReqsYesterday := int64(0)
		if statsYesterday != nil {
			totalReqsYesterday = statsYesterday.SuccessCount + statsYesterday.ErrorCount
			successRateYesterday, _ = calculateRates(statsYesterday.SuccessCount, statsYesterday.ErrorCount, totalReqsYesterday)
		}

		slaThreshold := 99.9
		slaChange24h := roundTo2DP(successRate - successRateYesterday)
		slaTrend := classifyTrend(slaChange24h, 0.05)
		slaStatus := classifySLAStatus(successRate, slaThreshold)

		latencyThresholdP99 := 1000
		latencyStatus := classifyLatencyStatus(stats.LatencyP99, latencyThresholdP99)

		qpsCurrent := 0.0
		{
			ctxWindow, cancelWindow := context.WithTimeout(workCtx, opsDBQueryTimeout)
			windowStats, err := repo.GetWindowStats(ctxWindow, now.Add(-1*time.Minute), now)
			cancelWindow()
			if err == nil && windowStats != nil {
				qpsCurrent = roundTo1DP(float64(windowStats.SuccessCount+windowStats.ErrorCount) / 60)
			} else if err != nil {
				log.Printf("[OpsOverview] get realtime qps failed: %v", err)
			}
		}

		qpsAvg := roundTo1DP(safeDivide(float64(totalReqs), duration.Seconds()))
		qpsPeak := qpsAvg
		{
			limit := int(duration.Minutes()) + 5
			if limit < 10 {
				limit = 10
			}
			if limit > 5000 {
				limit = 5000
			}
			ctxMetrics, cancelMetrics := context.WithTimeout(workCtx, opsDBQueryTimeout)
			items, err := repo.ListSystemMetricsRange(ctxMetrics, 1, startTime, now, limit)
			cancelMetrics()
			if err != nil {
				log.Printf("[OpsOverview] get metrics range for peak qps failed: %v", err)
			} else {
				maxQPS := 0.0
				for _, item := range items {
					v := float64(item.RequestCount) / 60
					if v > maxQPS {
						maxQPS = v
					}
				}
				if maxQPS > 0 {
					qpsPeak = roundTo1DP(maxQPS)
				}
			}
		}

		qpsAvgYesterday := 0.0
		if duration.Seconds() > 0 && totalReqsYesterday > 0 {
			qpsAvgYesterday = float64(totalReqsYesterday) / duration.Seconds()
		}
		qpsChangeVsYesterday := roundTo1DP(percentChange(qpsAvgYesterday, float64(totalReqs)/duration.Seconds()))

		tpsCurrent, tpsPeak, tpsAvg := 0.0, 0.0, 0.0
		if current, peak, avg, err := s.getTokenTPS(workCtx, now, startTime, duration); err != nil {
			log.Printf("[OpsOverview] get token tps failed: %v", err)
		} else {
			tpsCurrent, tpsPeak, tpsAvg = roundTo1DP(current), roundTo1DP(peak), roundTo1DP(avg)
		}

		diskUsage := 0.0
		if v, err := getDiskUsagePercent(workCtx, "/"); err != nil {
			log.Printf("[OpsOverview] get disk usage failed: %v", err)
		} else {
			diskUsage = roundTo1DP(v)
		}

		redisStatus := s.checkRedisHealth(workCtx)
		dbStatus := s.checkDatabaseHealth(workCtx)
		healthScore := calculateHealthScore(successRate, stats.LatencyP99, errorRate, redisStatus, dbStatus)

		data := &DashboardOverviewData{
			Timestamp:   now,
			HealthScore: healthScore,
			SLA: SLAData{
				Current:   successRate,
				Threshold: slaThreshold,
				Status:    slaStatus,
				Trend:     slaTrend,
				Change24h: slaChange24h,
			},
			QPS: QPSData{
				Current:           qpsCurrent,
				Peak1h:            qpsPeak,
				Avg1h:             qpsAvg,
				ChangeVsYesterday: qpsChangeVsYesterday,
			},
			TPS: TPSData{
				Current: tpsCurrent,
				Peak1h:  tpsPeak,
				Avg1h:   tpsAvg,
			},
			Latency: LatencyData{
				P50:          stats.LatencyP50,
				P95:          stats.LatencyP95,
				P99:          stats.LatencyP99,
				Avg:          stats.LatencyAvg,
				Max:          stats.LatencyMax,
				ThresholdP99: latencyThresholdP99,
				Status:       latencyStatus,
			},
			Errors: ErrorData{
				TotalCount:   stats.ErrorCount,
				ErrorRate:    errorRate,
				Count4xx:     stats.Error4xxCount,
				Count5xx:     stats.Error5xxCount,
				TimeoutCount: stats.TimeoutCount,
			},
			Resources: ResourceData{
				CPUUsage:      roundTo1DP(stats.CPUUsage),
				MemoryUsage:   roundTo1DP(stats.MemoryUsage),
				DiskUsage:     diskUsage,
				Goroutines:    runtime.NumGoroutine(),
				DBConnections: s.getDBConnections(),
			},
			SystemStatus: SystemStatusData{
				Redis:          redisStatus,
				Database:       dbStatus,
				BackgroundJobs: "healthy",
			},
		}

		if stats.TopErrorCount > 0 {
			data.Errors.TopError = &TopError{
				Code:    stats.TopErrorCode,
				Message: stats.TopErrorMsg,
				Count:   stats.TopErrorCount,
			}
		}

		s.setDashboardOverviewLocalCache(timeRange, data, opsDashboardLocalCacheTTL)
		cacheCtx, cancelCache := context.WithTimeout(context.Background(), 2*time.Second)
		_ = repo.SetCachedDashboardOverview(cacheCtx, timeRange, data, 10*time.Second)
		cancelCache()

		return data, nil
	})
	if err != nil {
		return nil, err
	}
	out, ok := v.(*DashboardOverviewData)
	if !ok || out == nil {
		return nil, errors.New("dashboard overview: invalid cache payload")
	}
	return out, nil
}

func (s *OpsQueryService) GetProviderHealth(ctx context.Context, timeRange string) ([]*ProviderHealthData, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}

	if strings.TrimSpace(timeRange) == "" {
		timeRange = "1h"
	}

	if cached, ok := s.getProviderHealthFromLocalCache(timeRange); ok {
		return cached, nil
	}

	window, err := parseTimeRange(timeRange)
	if err != nil {
		return nil, err
	}

	key := "provider_health:" + strings.TrimSpace(timeRange)
	v, err, _ := s.dashboardSF.Do(key, func() (any, error) {
		if cached, ok := s.getProviderHealthFromLocalCache(timeRange); ok {
			return cached, nil
		}

		endTime := time.Now()
		startTime := endTime.Add(-window)

		ctxDB, cancel := context.WithTimeout(context.Background(), opsDBQueryTimeout)
		stats, err := s.repo.GetProviderStats(ctxDB, startTime, endTime)
		cancel()
		if err != nil {
			return nil, err
		}

		results := make([]*ProviderHealthData, 0, len(stats))
		for _, item := range stats {
			if item == nil {
				continue
			}

			successRate, errorRate := calculateRates(item.SuccessCount, item.ErrorCount, item.RequestCount)

			results = append(results, &ProviderHealthData{
				Name:         formatPlatformName(item.Platform),
				RequestCount: item.RequestCount,
				SuccessRate:  successRate,
				ErrorRate:    errorRate,
				LatencyAvg:   item.AvgLatencyMs,
				LatencyP99:   item.P99LatencyMs,
				Status:       classifyProviderStatus(successRate, item.P99LatencyMs, item.TimeoutCount, item.RequestCount),
				ErrorsByType: ProviderHealthErrorsByType{
					HTTP4xx: item.Error4xxCount,
					HTTP5xx: item.Error5xxCount,
					Timeout: item.TimeoutCount,
				},
			})
		}

		s.setProviderHealthLocalCache(timeRange, results, opsDashboardLocalCacheTTL)
		return results, nil
	})
	if err != nil {
		return nil, err
	}
	out, ok := v.([]*ProviderHealthData)
	if !ok {
		return nil, errors.New("provider health: invalid cache payload")
	}
	return out, nil
}

func (s *OpsQueryService) GetLatencyHistogram(ctx context.Context, timeRange string) ([]*LatencyHistogramItem, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}

	if cached, ok := s.getLatencyHistogramFromLocalCache(timeRange); ok {
		return cached, nil
	}

	duration, err := parseTimeRange(timeRange)
	if err != nil {
		return nil, err
	}

	key := "latency_histogram:" + strings.TrimSpace(timeRange)
	v, err, _ := s.dashboardSF.Do(key, func() (any, error) {
		if cached, ok := s.getLatencyHistogramFromLocalCache(timeRange); ok {
			return cached, nil
		}

		endTime := time.Now()
		startTime := endTime.Add(-duration)
		ctxDB, cancel := context.WithTimeout(context.Background(), opsDBQueryTimeout)
		items, err := s.repo.GetLatencyHistogram(ctxDB, startTime, endTime)
		cancel()
		if err != nil {
			return nil, err
		}
		s.setLatencyHistogramLocalCache(timeRange, items, opsDashboardLocalCacheTTL)
		return items, nil
	})
	if err != nil {
		return nil, err
	}
	out, ok := v.([]*LatencyHistogramItem)
	if !ok {
		return nil, errors.New("latency histogram: invalid cache payload")
	}
	return out, nil
}

func (s *OpsQueryService) GetErrorDistribution(ctx context.Context, timeRange string) ([]*ErrorDistributionItem, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}

	if cached, ok := s.getErrorDistributionFromLocalCache(timeRange); ok {
		return cached, nil
	}

	duration, err := parseTimeRange(timeRange)
	if err != nil {
		return nil, err
	}

	key := "error_distribution:" + strings.TrimSpace(timeRange)
	v, err, _ := s.dashboardSF.Do(key, func() (any, error) {
		if cached, ok := s.getErrorDistributionFromLocalCache(timeRange); ok {
			return cached, nil
		}

		endTime := time.Now()
		startTime := endTime.Add(-duration)
		ctxDB, cancel := context.WithTimeout(context.Background(), opsDBQueryTimeout)
		items, err := s.repo.GetErrorDistribution(ctxDB, startTime, endTime)
		cancel()
		if err != nil {
			return nil, err
		}
		s.setErrorDistributionLocalCache(timeRange, items, opsDashboardLocalCacheTTL)
		return items, nil
	})
	if err != nil {
		return nil, err
	}
	out, ok := v.([]*ErrorDistributionItem)
	if !ok {
		return nil, errors.New("error distribution: invalid cache payload")
	}
	return out, nil
}

func calculateHealthScore(successRate float64, p99Latency int, errorRate float64, redisStatus, dbStatus string) int {
	score := 100.0

	// SLA impact (max -45 points)
	if successRate < 99.9 {
		score -= math.Min(45, (99.9-successRate)*12)
	}

	// Latency impact (max -35 points)
	if p99Latency > 1000 {
		score -= math.Min(35, float64(p99Latency-1000)/80)
	}

	// Error rate impact (max -20 points)
	if errorRate > 0.1 {
		score -= math.Min(20, (errorRate-0.1)*60)
	}

	// Infra status impact
	if redisStatus != "healthy" {
		score -= 15
	}
	if dbStatus != "healthy" {
		score -= 20
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return int(math.Round(score))
}

func calculateRates(successCount, errorCount, requestCount int64) (successRate float64, errorRate float64) {
	if requestCount <= 0 {
		return 0, 0
	}
	successRate = (float64(successCount) / float64(requestCount)) * 100
	errorRate = (float64(errorCount) / float64(requestCount)) * 100
	return roundTo2DP(successRate), roundTo2DP(errorRate)
}

func roundTo2DP(v float64) float64 {
	return math.Round(v*100) / 100
}

func roundTo1DP(v float64) float64 {
	return math.Round(v*10) / 10
}

func safeDivide(numerator float64, denominator float64) float64 {
	if denominator <= 0 {
		return 0
	}
	return numerator / denominator
}

func percentChange(previous float64, current float64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100.0
		}
		return 0
	}
	return (current - previous) / previous * 100
}

func classifyTrend(delta float64, deadband float64) string {
	if delta > deadband {
		return "up"
	}
	if delta < -deadband {
		return "down"
	}
	return "stable"
}

func classifySLAStatus(successRate float64, threshold float64) string {
	if successRate >= threshold {
		return "healthy"
	}
	if successRate >= threshold-0.5 {
		return "warning"
	}
	return "critical"
}

func classifyLatencyStatus(p99LatencyMs int, thresholdP99 int) string {
	if thresholdP99 <= 0 {
		return "healthy"
	}
	if p99LatencyMs <= thresholdP99 {
		return "healthy"
	}
	if p99LatencyMs <= thresholdP99*2 {
		return "warning"
	}
	return "critical"
}

func getDiskUsagePercent(ctx context.Context, path string) (float64, error) {
	usage, err := disk.UsageWithContext(ctx, path)
	if err != nil {
		return 0, err
	}
	if usage == nil {
		return 0, nil
	}
	return usage.UsedPercent, nil
}

func (s *OpsQueryService) checkRedisHealth(ctx context.Context) string {
	if s == nil {
		log.Printf("[OpsOverview][WARN] ops service is nil; redis health check skipped")
		return "critical"
	}
	if s.repo == nil {
		s.redisNilWarnOnce.Do(func() {
			log.Printf("[OpsOverview][WARN] ops repository is nil; redis health check skipped")
		})
		return "critical"
	}

	ctxPing, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()

	if err := s.repo.PingRedis(ctxPing); err != nil {
		recordInfrastructureError(ctx, "redis", "OpsService.checkRedisHealth", err)
		log.Printf("[OpsOverview][WARN] redis ping failed: %v", err)
		return "critical"
	}
	return "healthy"
}

func (s *OpsQueryService) checkDatabaseHealth(ctx context.Context) string {
	if s == nil {
		log.Printf("[OpsOverview][WARN] ops service is nil; db health check skipped")
		return "critical"
	}
	if s.sqlDB == nil {
		s.dbNilWarnOnce.Do(func() {
			log.Printf("[OpsOverview][WARN] database is nil; db health check skipped")
		})
		return "critical"
	}

	ctxPing, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()

	if err := s.sqlDB.PingContext(ctxPing); err != nil {
		recordInfrastructureError(ctx, "db", "OpsService.checkDatabaseHealth", err)
		log.Printf("[OpsOverview][WARN] db ping failed: %v", err)
		return "critical"
	}
	return "healthy"
}

func (s *OpsQueryService) getDBConnections() DBConnectionsData {
	if s == nil || s.sqlDB == nil {
		return DBConnectionsData{}
	}

	stats := s.sqlDB.Stats()
	maxOpen := stats.MaxOpenConnections
	if maxOpen < 0 {
		maxOpen = 0
	}

	return DBConnectionsData{
		Active:  stats.InUse,
		Idle:    stats.Idle,
		Waiting: 0,
		Max:     maxOpen,
	}
}

func (s *OpsQueryService) getTokenTPS(ctx context.Context, endTime time.Time, startTime time.Time, duration time.Duration) (current float64, peak float64, avg float64, err error) {
	if s == nil || s.repo == nil {
		return 0, 0, 0, nil
	}

	if duration <= 0 {
		return 0, 0, 0, nil
	}

	ctxQuery, cancel := context.WithTimeout(ctx, opsDBQueryTimeout)
	defer cancel()

	return s.repo.GetTokenTPS(ctxQuery, startTime, endTime)
}

func formatPlatformName(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformOpenAI:
		return "OpenAI"
	case PlatformAnthropic:
		return "Anthropic"
	case PlatformGemini:
		return "Gemini"
	case PlatformAntigravity:
		return "Antigravity"
	default:
		if platform == "" {
			return "Unknown"
		}
		if len(platform) == 1 {
			return strings.ToUpper(platform)
		}
		return strings.ToUpper(platform[:1]) + platform[1:]
	}
}

func classifyProviderStatus(successRate float64, p99LatencyMs int, timeoutCount int64, requestCount int64) string {
	if requestCount <= 0 {
		return "healthy"
	}

	if successRate < 98 {
		return "critical"
	}
	if successRate < 99.5 {
		return "warning"
	}

	// Heavy timeout volume should be highlighted even if the overall success rate is okay.
	if timeoutCount >= 10 && requestCount >= 100 {
		return "warning"
	}

	if p99LatencyMs > 0 && p99LatencyMs >= 5000 {
		return "warning"
	}

	return "healthy"
}

func (s *OpsQueryService) GetConcurrencyStats(ctx context.Context) (map[string]*PlatformConcurrencyInfo, map[int64]*GroupConcurrencyInfo, *time.Time, error) {
	if s == nil || s.repo == nil {
		return make(map[string]*PlatformConcurrencyInfo), make(map[int64]*GroupConcurrencyInfo), nil, nil
	}
	platform, _ := s.repo.GetCachedPlatformConcurrency(ctx)
	group, _ := s.repo.GetCachedGroupConcurrency(ctx)
	var collectedAt *time.Time
	if t, ok, _ := s.repo.GetCachedConcurrencyCollectedAt(ctx); ok {
		collectedAt = &t
	}
	return platform, group, collectedAt, nil
}
