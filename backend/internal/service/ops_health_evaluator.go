package service

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

type HealthCheck struct {
	Enabled  bool                 `json:"enabled"`
	Status   HealthStatus          `json:"status"`
	Healthy  bool                  `json:"healthy"`
	Warnings []string              `json:"warnings"`
	Checks   map[string]CheckItem  `json:"checks"`
}

type CheckItem struct {
	Name   string      `json:"name"`
	Status string      `json:"status"`
	Value  interface{} `json:"value"`
}

type HealthEvaluator struct{}

func NewHealthEvaluator() *HealthEvaluator {
	return &HealthEvaluator{}
}

func (e *HealthEvaluator) Evaluate(metrics *OpsMetrics, concurrency map[string]*PlatformConcurrencyInfo) *HealthCheck {
	hc := &HealthCheck{
		Enabled:  true,
		Status:   HealthStatusHealthy,
		Healthy:  true,
		Warnings: []string{},
		Checks:   map[string]CheckItem{},
	}

	if metrics != nil {
		// 规则1: P95延迟 > 60s → degraded
		if metrics.LatencyP95 > 60000 {
			hc.Status = HealthStatusDegraded
			hc.Warnings = append(hc.Warnings, "P95响应时间超过60秒")
		}

		// 规则2: 错误率 > 5% → unhealthy, > 1% → degraded
		if metrics.ErrorRate > 5 {
			hc.Status = HealthStatusUnhealthy
			hc.Warnings = append(hc.Warnings, "错误率超过5%")
		} else if metrics.ErrorRate > 1 && hc.Status != HealthStatusUnhealthy {
			hc.Status = HealthStatusDegraded
			hc.Warnings = append(hc.Warnings, "错误率超过1%")
		}

		// 规则3: 并发队列深度 > 1000 → degraded
		if metrics.ConcurrencyQueueDepth > 1000 && hc.Status != HealthStatusUnhealthy {
			hc.Status = HealthStatusDegraded
			hc.Warnings = append(hc.Warnings, "等待队列超过1000")
		}
	}

	// 规则4: 任一平台负载 > 90% → degraded
	if concurrency != nil {
		for platform, info := range concurrency {
			if info.LoadPercentage > 90 && hc.Status != HealthStatusUnhealthy {
				hc.Status = HealthStatusDegraded
				hc.Warnings = append(hc.Warnings, platform+"平台负载超过90%")
			}
		}
	}

	hc.Healthy = hc.Status == HealthStatusHealthy
	return hc
}
