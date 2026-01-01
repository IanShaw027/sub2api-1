package service

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type OpsMetrics struct {
	WindowMinutes         int       `json:"window_minutes"`
	RequestCount          int64     `json:"request_count"`
	SuccessCount          int64     `json:"success_count"`
	ErrorCount            int64     `json:"error_count"`
	SuccessRate           float64   `json:"success_rate"`
	ErrorRate             float64   `json:"error_rate"`
	P95LatencyMs          int       `json:"p95_latency_ms"`
	P99LatencyMs          int       `json:"p99_latency_ms"`
	HTTP2Errors           int       `json:"http2_errors"`
	ActiveAlerts          int       `json:"active_alerts"`
	CPUUsagePercent       float64   `json:"cpu_usage_percent"`
	MemoryUsedMB          int64     `json:"memory_used_mb"`
	MemoryTotalMB         int64     `json:"memory_total_mb"`
	MemoryUsagePercent    float64   `json:"memory_usage_percent"`
	HeapAllocMB           int64     `json:"heap_alloc_mb"`
	GCPauseMs             float64   `json:"gc_pause_ms"`
	ConcurrencyQueueDepth int       `json:"concurrency_queue_depth"`
	UpdatedAt             time.Time `json:"updated_at,omitempty"`
}

type OpsErrorLog struct {
	ID         int64     `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Phase      string    `json:"phase"`
	Type       string    `json:"type"`
	Severity   string    `json:"severity"`
	StatusCode int       `json:"status_code"`
	Platform   string    `json:"platform"`
	Model      string    `json:"model"`
	LatencyMs  *int      `json:"latency_ms"`
	RequestID  string    `json:"request_id"`
	Message    string    `json:"message"`

	UserID      *int64 `json:"user_id,omitempty"`
	APIKeyID    *int64 `json:"api_key_id,omitempty"`
	AccountID   *int64 `json:"account_id,omitempty"`
	GroupID     *int64 `json:"group_id,omitempty"`
	ClientIP    string `json:"client_ip,omitempty"`
	RequestPath string `json:"request_path,omitempty"`
	Stream      bool   `json:"stream"`
}

type OpsErrorLogFilters struct {
	StartTime *time.Time
	EndTime   *time.Time
	Platform  string
	Phase     string
	Severity  string
	Query     string
	Limit     int
}

type OpsWindowStats struct {
	SuccessCount int64
	ErrorCount   int64
	P95LatencyMs int
	P99LatencyMs int
	HTTP2Errors  int
}

type OpsRepository interface {
	CreateErrorLog(ctx context.Context, log *OpsErrorLog) error
	ListErrorLogs(ctx context.Context, filters OpsErrorLogFilters) ([]OpsErrorLog, error)
	GetLatestSystemMetric(ctx context.Context) (*OpsMetrics, error)
	CreateSystemMetric(ctx context.Context, metric *OpsMetrics) error
	GetWindowStats(ctx context.Context, startTime, endTime time.Time) (*OpsWindowStats, error)
	ListRecentSystemMetrics(ctx context.Context, windowMinutes, limit int) ([]OpsMetrics, error)
	ListSystemMetricsRange(ctx context.Context, windowMinutes int, startTime, endTime time.Time, limit int) ([]OpsMetrics, error)
	ListAlertRules(ctx context.Context) ([]OpsAlertRule, error)
	GetActiveAlertEvent(ctx context.Context, ruleID int64) (*OpsAlertEvent, error)
	GetLatestAlertEvent(ctx context.Context, ruleID int64) (*OpsAlertEvent, error)
	CreateAlertEvent(ctx context.Context, event *OpsAlertEvent) error
	UpdateAlertEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error
	UpdateAlertEventNotifications(ctx context.Context, eventID int64, emailSent, webhookSent bool) error
	CountActiveAlerts(ctx context.Context) (int, error)
}

type OpsService struct {
	repo OpsRepository
}

func NewOpsService(repo OpsRepository) *OpsService {
	return &OpsService{repo: repo}
}

func (s *OpsService) RecordError(ctx context.Context, log *OpsErrorLog) error {
	if log == nil {
		return nil
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	if log.Severity == "" {
		log.Severity = "P2"
	}
	if log.Phase == "" {
		log.Phase = "internal"
	}
	if log.Type == "" {
		log.Type = "unknown_error"
	}
	if log.Message == "" {
		log.Message = "Unknown error"
	}
	return s.repo.CreateErrorLog(ctx, log)
}

func (s *OpsService) RecordMetrics(ctx context.Context, metric *OpsMetrics) error {
	if metric == nil {
		return nil
	}
	if metric.UpdatedAt.IsZero() {
		metric.UpdatedAt = time.Now()
	}
	return s.repo.CreateSystemMetric(ctx, metric)
}

func (s *OpsService) ListErrorLogs(ctx context.Context, filters OpsErrorLogFilters) ([]OpsErrorLog, int, error) {
	logs, err := s.repo.ListErrorLogs(ctx, filters)
	if err != nil {
		return nil, 0, err
	}
	return logs, len(logs), nil
}

func (s *OpsService) GetWindowStats(ctx context.Context, startTime, endTime time.Time) (*OpsWindowStats, error) {
	return s.repo.GetWindowStats(ctx, startTime, endTime)
}

func (s *OpsService) GetLatestMetrics(ctx context.Context) (*OpsMetrics, error) {
	metric, err := s.repo.GetLatestSystemMetric(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &OpsMetrics{WindowMinutes: 1}, nil
		}
		return nil, err
	}
	if metric == nil {
		return &OpsMetrics{WindowMinutes: 1}, nil
	}
	if metric.WindowMinutes == 0 {
		metric.WindowMinutes = 1
	}
	return metric, nil
}

func (s *OpsService) ListMetricsHistory(ctx context.Context, windowMinutes int, startTime, endTime time.Time, limit int) ([]OpsMetrics, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	if windowMinutes <= 0 {
		windowMinutes = 1
	}
	if limit <= 0 || limit > 5000 {
		limit = 300
	}
	if endTime.IsZero() {
		endTime = time.Now()
	}
	if startTime.IsZero() {
		startTime = endTime.Add(-time.Duration(limit) * opsMetricsInterval)
	}
	if startTime.After(endTime) {
		startTime, endTime = endTime, startTime
	}
	return s.repo.ListSystemMetricsRange(ctx, windowMinutes, startTime, endTime, limit)
}
