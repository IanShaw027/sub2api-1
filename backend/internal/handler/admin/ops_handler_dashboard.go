package admin

import (
	"math"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GetDashboardOverview returns realtime ops dashboard overview.
// GET /api/v1/admin/ops/dashboard/overview
//
// Query params:
// - time_range: string (optional; default "1h") one of: 5m, 30m, 1h, 6h, 24h
func (h *OpsHandler) GetDashboardOverview(c *gin.Context) {
	timeRange, _, err := parseDashboardTimeRangeParam(c, "1h")
	if err != nil {
		return
	}

	data, err := h.opsService.GetDashboardOverview(c.Request.Context(), timeRange)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get dashboard overview")
		return
	}
	response.Success(c, data)
}

// GetProviderHealth returns upstream provider health comparison data.
// GET /api/v1/admin/ops/dashboard/providers
//
// Query params:
// - time_range: string (optional; default "1h") one of: 5m, 30m, 1h, 6h, 24h
func (h *OpsHandler) GetProviderHealth(c *gin.Context) {
	timeRange, _, err := parseDashboardTimeRangeParam(c, "1h")
	if err != nil {
		return
	}

	providers, err := h.opsService.GetProviderHealth(c.Request.Context(), timeRange)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get provider health")
		return
	}

	var totalRequests int64
	var weightedSuccess float64
	var bestProvider string
	var worstProvider string
	var bestRate float64
	var worstRate float64
	hasRate := false

	for _, p := range providers {
		if p == nil {
			continue
		}
		totalRequests += p.RequestCount
		weightedSuccess += (p.SuccessRate / 100) * float64(p.RequestCount)

		if p.RequestCount <= 0 {
			continue
		}
		if !hasRate {
			bestProvider = p.Name
			worstProvider = p.Name
			bestRate = p.SuccessRate
			worstRate = p.SuccessRate
			hasRate = true
			continue
		}

		if p.SuccessRate > bestRate {
			bestProvider = p.Name
			bestRate = p.SuccessRate
		}
		if p.SuccessRate < worstRate {
			worstProvider = p.Name
			worstRate = p.SuccessRate
		}
	}

	avgSuccessRate := 0.0
	if totalRequests > 0 {
		avgSuccessRate = (weightedSuccess / float64(totalRequests)) * 100
		avgSuccessRate = math.Round(avgSuccessRate*100) / 100
	}

	response.Success(c, gin.H{
		"providers": providers,
		"summary": gin.H{
			"total_requests":   totalRequests,
			"avg_success_rate": avgSuccessRate,
			"best_provider":    bestProvider,
			"worst_provider":   worstProvider,
		},
	})
}

// GetLatencyHistogram returns the latency distribution histogram.
// GET /api/v1/admin/ops/dashboard/latency-histogram
func (h *OpsHandler) GetLatencyHistogram(c *gin.Context) {
	timeRange, _, err := parseDashboardTimeRangeParam(c, "1h")
	if err != nil {
		return
	}

	buckets, err := h.opsService.GetLatencyHistogram(c.Request.Context(), timeRange)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get latency histogram")
		return
	}

	totalRequests := int64(0)
	for _, b := range buckets {
		totalRequests += b.Count
	}

	response.Success(c, gin.H{
		"buckets":                buckets,
		"total_requests":         totalRequests,
		"slow_request_threshold": 1000,
	})
}

// GetErrorDistribution returns the error distribution.
// GET /api/v1/admin/ops/dashboard/errors/distribution
func (h *OpsHandler) GetErrorDistribution(c *gin.Context) {
	timeRange, _, err := parseDashboardTimeRangeParam(c, "1h")
	if err != nil {
		return
	}

	items, err := h.opsService.GetErrorDistribution(c.Request.Context(), timeRange)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get error distribution")
		return
	}

	response.Success(c, gin.H{
		"items": items,
	})
}

// GetConcurrencyStats returns real-time concurrency statistics.
// GET /api/v1/admin/ops/concurrency
func (h *OpsHandler) GetConcurrencyStats(c *gin.Context) {
	ctx := c.Request.Context()
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not initialized")
		return
	}

	realtimeEnabled := h.opsService.IsRealtimeMonitoringEnabled(ctx)
	if !realtimeEnabled {
		response.Success(c, gin.H{
			"enabled":   false,
			"platform":  map[string]*service.PlatformConcurrencyInfo{},
			"group":     map[int64]*service.GroupConcurrencyInfo{},
			"timestamp": time.Now(),
		})
		return
	}

	platform, group, collectedAt, err := h.opsService.GetConcurrencyStats(ctx)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get concurrency stats")
		return
	}

	payload := gin.H{
		"enabled":  true,
		"platform": platform,
		"group":    group,
	}
	// Timestamp should reflect the cached sample time (collector tick) rather than request time.
	if collectedAt != nil {
		payload["timestamp"] = collectedAt.UTC()
	}
	response.Success(c, payload)
}

// GetSystemHealth returns system health status.
// GET /api/v1/admin/ops/health
func (h *OpsHandler) GetSystemHealth(c *gin.Context) {
	ctx := c.Request.Context()

	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not initialized")
		return
	}

	metrics, _ := h.opsService.GetLatestMetrics(ctx)
	realtimeEnabled := h.opsService.IsRealtimeMonitoringEnabled(ctx)
	concurrency := map[string]*service.PlatformConcurrencyInfo{}
	if realtimeEnabled {
		concurrency, _, _, _ = h.opsService.GetConcurrencyStats(ctx)
	}

	evaluator := service.NewHealthEvaluator()
	health := evaluator.Evaluate(metrics, concurrency)
	health.Enabled = realtimeEnabled

	response.Success(c, health)
}
