package service

import "time"

type OpsMetrics struct {
	// 窗口和时间
	WindowMinutes int       `json:"window_minutes"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`

	// 请求统计
	RequestCount int64   `json:"request_count"`
	SuccessCount int64   `json:"success_count"`
	ErrorCount   int64   `json:"error_count"`
	QPS          float64 `json:"qps"`
	TPS          float64 `json:"tps"`

	// 错误分类
	Error4xxCount     int64 `json:"error_4xx_count"`
	Error5xxCount     int64 `json:"error_5xx_count"`
	ErrorTimeoutCount int64 `json:"error_timeout_count"`

	// 延迟指标
	LatencyP50         float64 `json:"latency_p50"`
	LatencyP95         float64 `json:"latency_p95"`
	LatencyP99         float64 `json:"latency_p99"`
	LatencyAvg         float64 `json:"latency_avg"`
	LatencyMax         float64 `json:"latency_max"`
	UpstreamLatencyAvg float64 `json:"upstream_latency_avg"`

	// 成功/错误率
	SuccessRate float64 `json:"success_rate"`
	ErrorRate   float64 `json:"error_rate"`

	// 系统资源（基础）
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	MemoryUsedMB       int64   `json:"memory_used_mb"`
	MemoryTotalMB      int64   `json:"memory_total_mb"`

	// 数据库连接
	DBConnActive  int `json:"db_conn_active"`
	DBConnIdle    int `json:"db_conn_idle"`
	DBConnWaiting int `json:"db_conn_waiting"`

	// Goroutine
	GoroutineCount int `json:"goroutine_count"`

	// 业务指标
	TokenConsumed       int64   `json:"token_consumed"`
	TokenRate           float64 `json:"token_rate"`
	ActiveSubscriptions int     `json:"active_subscriptions"`

	// 告警
	ActiveAlerts int `json:"active_alerts"`

	// 队列
	ConcurrencyQueueDepth int `json:"concurrency_queue_depth"`
}

type OpsErrorLog struct {
	ID          int64     `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	Phase       string    `json:"phase"`
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	StatusCode  int       `json:"status_code"`
	Platform    string    `json:"platform"`
	Model       string    `json:"model"`
	RequestPath string    `json:"request_path,omitempty"`
	LatencyMs   *int      `json:"latency_ms"`
	DurationMs  *int      `json:"duration_ms,omitempty"`
	RequestID   string    `json:"request_id"`
	Message     string    `json:"message"`
	ErrorBody   string    `json:"error_body,omitempty"`

	ProviderErrorCode string `json:"provider_error_code,omitempty"`
	ProviderErrorType string `json:"provider_error_type,omitempty"`

	// 延迟细化字段
	TimeToFirstTokenMs *int `json:"time_to_first_token_ms,omitempty"`
	AuthLatencyMs      *int `json:"auth_latency_ms,omitempty"`
	RoutingLatencyMs   *int `json:"routing_latency_ms,omitempty"`
	UpstreamLatencyMs  *int `json:"upstream_latency_ms,omitempty"`
	ResponseLatencyMs  *int `json:"response_latency_ms,omitempty"`

	// 请求体和客户端信息
	RequestBody string `json:"request_body,omitempty"`
	UserAgent   string `json:"user_agent,omitempty"`

	// 错误分类字段
	ErrorSource          string  `json:"error_source,omitempty"`
	ErrorOwner           string  `json:"error_owner,omitempty"`
	AccountStatus        string  `json:"account_status,omitempty"`
	UpstreamStatusCode   *int    `json:"upstream_status_code,omitempty"`
	UpstreamErrorMessage string  `json:"upstream_error_message,omitempty"`
	UpstreamErrorDetail  *string `json:"upstream_error_detail,omitempty"`
	NetworkErrorType     string  `json:"network_error_type,omitempty"`
	RetryAfterSeconds    *int    `json:"retry_after_seconds,omitempty"`

	IsRetryable      bool   `json:"is_retryable"`
	IsUserActionable bool   `json:"is_user_actionable"`
	RetryCount       int    `json:"retry_count"`
	CompletionStatus string `json:"completion_status,omitempty"`

	UserID    *int64 `json:"user_id,omitempty"`
	APIKeyID  *int64 `json:"api_key_id,omitempty"`
	AccountID *int64 `json:"account_id,omitempty"`
	GroupID   *int64 `json:"group_id,omitempty"`
	ClientIP  string `json:"client_ip,omitempty"`
	Stream    bool   `json:"stream"`
}

type OpsWindowStats struct {
	SuccessCount  int64
	ErrorCount    int64
	Error4xxCount int64
	Error5xxCount int64
	TimeoutCount  int64
	P50LatencyMs  int
	P95LatencyMs  int
	P99LatencyMs  int
	AvgLatencyMs  int
	MaxLatencyMs  int
	TokenConsumed int64
}

type OpsWindowStatsGroupedItem struct {
	Group         string `json:"group"`
	ErrorCount    int64  `json:"error_count"`
	Error4xxCount int64  `json:"error_4xx_count"`
	Error5xxCount int64  `json:"error_5xx_count"`
	TimeoutCount  int64  `json:"timeout_count"`
}

type ProviderStats struct {
	Platform string

	RequestCount int64
	SuccessCount int64
	ErrorCount   int64

	AvgLatencyMs int
	P99LatencyMs int

	Error4xxCount int64
	Error5xxCount int64
	TimeoutCount  int64
}

type ProviderHealthErrorsByType struct {
	HTTP4xx int64 `json:"4xx"`
	HTTP5xx int64 `json:"5xx"`
	Timeout int64 `json:"timeout"`
}

type ProviderHealthData struct {
	Name         string                     `json:"name"`
	RequestCount int64                      `json:"request_count"`
	SuccessRate  float64                    `json:"success_rate"`
	ErrorRate    float64                    `json:"error_rate"`
	LatencyAvg   int                        `json:"latency_avg"`
	LatencyP99   int                        `json:"latency_p99"`
	Status       string                     `json:"status"`
	ErrorsByType ProviderHealthErrorsByType `json:"errors_by_type"`
}

type LatencyHistogramItem struct {
	Range      string  `json:"range"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

type ErrorDistributionItem struct {
	Code       string  `json:"code"`
	Message    string  `json:"message"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

type IPErrorStats struct {
	ClientIP       string           `json:"client_ip"`
	ErrorCount     int64            `json:"error_count"`
	FirstErrorTime time.Time        `json:"first_error_time"`
	LastErrorTime  time.Time        `json:"last_error_time"`
	ErrorTypes     map[string]int64 `json:"error_types"`
}

// DashboardOverviewData represents aggregated metrics for the ops dashboard overview.
type DashboardOverviewData struct {
	Timestamp    time.Time        `json:"timestamp"`
	HealthScore  int              `json:"health_score"`
	SLA          SLAData          `json:"sla"`
	QPS          QPSData          `json:"qps"`
	TPS          TPSData          `json:"tps"`
	Latency      LatencyData      `json:"latency"`
	Errors       ErrorData        `json:"errors"`
	Resources    ResourceData     `json:"resources"`
	SystemStatus SystemStatusData `json:"system_status"`
}

type SLAData struct {
	Current   float64 `json:"current"`
	Threshold float64 `json:"threshold"`
	Status    string  `json:"status"`
	Trend     string  `json:"trend"`
	Change24h float64 `json:"change_24h"`
}

type QPSData struct {
	Current           float64 `json:"current"`
	Peak1h            float64 `json:"peak_1h"`
	Avg1h             float64 `json:"avg_1h"`
	ChangeVsYesterday float64 `json:"change_vs_yesterday"`
}

type TPSData struct {
	Current float64 `json:"current"`
	Peak1h  float64 `json:"peak_1h"`
	Avg1h   float64 `json:"avg_1h"`
}

type LatencyData struct {
	P50          int    `json:"p50"`
	P95          int    `json:"p95"`
	P99          int    `json:"p99"`
	Avg          int    `json:"avg"`
	Max          int    `json:"max"`
	ThresholdP99 int    `json:"threshold_p99"`
	Status       string `json:"status"`
}

type ErrorData struct {
	TotalCount   int64     `json:"total_count"`
	ErrorRate    float64   `json:"error_rate"`
	Count4xx     int64     `json:"4xx_count"`
	Count5xx     int64     `json:"5xx_count"`
	TimeoutCount int64     `json:"timeout_count"`
	TopError     *TopError `json:"top_error,omitempty"`
}

type TopError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Count   int64  `json:"count"`
}

type ResourceData struct {
	CPUUsage      float64           `json:"cpu_usage"`
	MemoryUsage   float64           `json:"memory_usage"`
	DiskUsage     float64           `json:"disk_usage"`
	Goroutines    int               `json:"goroutines"`
	DBConnections DBConnectionsData `json:"db_connections"`
}

type DBConnectionsData struct {
	Active  int `json:"active"`
	Idle    int `json:"idle"`
	Waiting int `json:"waiting"`
	Max     int `json:"max"`
}

type SystemStatusData struct {
	Redis          string `json:"redis"`
	Database       string `json:"database"`
	BackgroundJobs string `json:"background_jobs"`
}

type OverviewStats struct {
	RequestCount          int64
	SuccessCount          int64
	ErrorCount            int64
	Error4xxCount         int64
	Error5xxCount         int64
	TimeoutCount          int64
	LatencyP50            int
	LatencyP95            int
	LatencyP99            int
	LatencyAvg            int
	LatencyMax            int
	TopErrorCode          string
	TopErrorMsg           string
	TopErrorCount         int64
	CPUUsage              float64
	MemoryUsage           float64
	MemoryUsedMB          int64
	MemoryTotalMB         int64
	ConcurrencyQueueDepth int
}

// AccountStats 账号统计数据
type AccountStats struct {
	ErrorCount     int `json:"error_count"`
	SuccessCount   int `json:"success_count"`
	TimeoutCount   int `json:"timeout_count"`
	RateLimitCount int `json:"rate_limit_count"`
}

// AccountStatusSummary summarizes recent stats for an account.
type AccountStatusSummary struct {
	AccountID int64        `json:"account_id"`
	Stats1h   AccountStats `json:"stats_1h"`
	Stats24h  AccountStats `json:"stats_24h"`
}

// OpsAccountStatus 账号状态
type OpsAccountStatus struct {
	ID                int64
	AccountID         int64
	Platform          string
	Status            string
	LastErrorType     string
	LastErrorMessage  string
	LastErrorTime     time.Time
	ErrorCount1h      int
	SuccessCount1h    int
	TimeoutCount1h    int
	RateLimitCount1h  int
	ErrorCount24h     int
	SuccessCount24h   int
	TimeoutCount24h   int
	RateLimitCount24h int
	LastSuccessTime   *time.Time
	StatusChangedAt   *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
