package service

import (
	"context"
	"time"
)

type OpsIngestRepository interface {
	CreateErrorLog(ctx context.Context, log *OpsErrorLog) error
	CreateSystemMetric(ctx context.Context, metric *OpsMetrics) error

	// Alert/group availability write paths.
	CreateAlertRule(ctx context.Context, rule *OpsAlertRule) error
	UpdateAlertRule(ctx context.Context, rule *OpsAlertRule) error
	DeleteAlertRule(ctx context.Context, id int64) error
	CreateAlertEvent(ctx context.Context, event *OpsAlertEvent) error
	UpdateAlertEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error
	UpdateAlertEventNotifications(ctx context.Context, eventID int64, emailSent bool) error

	CreateGroupAvailabilityConfig(ctx context.Context, config *OpsGroupAvailabilityConfig) error
	UpdateGroupAvailabilityConfig(ctx context.Context, config *OpsGroupAvailabilityConfig) error
	DeleteGroupAvailabilityConfig(ctx context.Context, groupID int64) error
	CreateGroupAvailabilityEvent(ctx context.Context, event *OpsGroupAvailabilityEvent) error
	UpdateGroupAvailabilityEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error
	UpdateGroupAvailabilityEventNotifications(ctx context.Context, eventID int64, emailSent bool) error

	// Pre-aggregation write paths.
	UpsertHourlyMetrics(ctx context.Context, startTime, endTime time.Time) error
	UpsertDailyMetrics(ctx context.Context, startTime, endTime time.Time) error
	GetLatestHourlyBucketStart(ctx context.Context) (time.Time, bool, error)
	GetLatestDailyBucketDate(ctx context.Context) (time.Time, bool, error)

	// Cache updates (latest metrics snapshot).
	SetCachedLatestSystemMetric(ctx context.Context, metric *OpsMetrics) error

	// Data cleanup methods.
	DeleteOldErrorLogs(ctx context.Context, retentionDays int) (int64, error)
	DeleteOldMetrics(ctx context.Context, windowMinutes int, retentionDays int) (int64, error)
}

type OpsQueryRepository interface {
	// ListErrorLogs provides a paginated error-log query API (with total count).
	// NOTE: list payload uses `OpsErrorLog` so it matches `/admin/ops/errors/:id`.
	ListErrorLogs(ctx context.Context, filter *ErrorLogFilter) ([]*OpsErrorLog, int64, error)
	// GetErrorLogByID retrieves a single error log by its ID with all details.
	GetErrorLogByID(ctx context.Context, id int64) (*OpsErrorLog, error)

	// ListRequestDetails returns request-level rows (success + error) with request_id for metric drill-down.
	ListRequestDetails(ctx context.Context, filter *OpsRequestDetailFilter) ([]*OpsRequestDetail, int64, error)

	GetLatestSystemMetric(ctx context.Context) (*OpsMetrics, error)
	GetWindowStats(ctx context.Context, startTime, endTime time.Time) (*OpsWindowStats, error)
	GetWindowStatsGrouped(ctx context.Context, startTime, endTime time.Time, groupBy string) ([]*OpsWindowStatsGroupedItem, error)
	GetProviderStats(ctx context.Context, startTime, endTime time.Time) ([]*ProviderStats, error)
	GetLatencyHistogram(ctx context.Context, startTime, endTime time.Time) ([]*LatencyHistogramItem, error)
	GetErrorDistribution(ctx context.Context, startTime, endTime time.Time) ([]*ErrorDistributionItem, error)
	ListRecentSystemMetrics(ctx context.Context, windowMinutes, limit int) ([]OpsMetrics, error)
	ListSystemMetricsRange(ctx context.Context, windowMinutes int, startTime, endTime time.Time, limit int) ([]OpsMetrics, error)
	GetTokenTPS(ctx context.Context, startTime, endTime time.Time) (current, peak, avg float64, err error)

	ListAlertRules(ctx context.Context) ([]OpsAlertRule, error)
	ListAlertEvents(ctx context.Context, limit int) ([]OpsAlertEvent, error)
	GetActiveAlertEvent(ctx context.Context, ruleID int64) (*OpsAlertEvent, error)
	GetLatestAlertEvent(ctx context.Context, ruleID int64) (*OpsAlertEvent, error)
	CountActiveAlerts(ctx context.Context) (int, error)
	GetOverviewStats(ctx context.Context, startTime, endTime time.Time) (*OverviewStats, error)

	// Group availability monitoring methods
	ListGroupAvailabilityConfigs(ctx context.Context, enabledOnly bool) ([]OpsGroupAvailabilityConfig, error)
	GetGroupAvailabilityConfig(ctx context.Context, groupID int64) (*OpsGroupAvailabilityConfig, error)
	GetActiveGroupAvailabilityEvent(ctx context.Context, configID int64) (*OpsGroupAvailabilityEvent, error)
	GetLatestGroupAvailabilityEvent(ctx context.Context, configID int64) (*OpsGroupAvailabilityEvent, error)
	ListGroupAvailabilityEvents(ctx context.Context, limit int, status string) ([]OpsGroupAvailabilityEvent, error)
	CountAvailableAccountsByGroup(ctx context.Context, groupID int64) (available, total int, err error)

	// Redis-backed cache/health (best-effort; implementation lives in repository layer).
	GetCachedLatestSystemMetric(ctx context.Context) (*OpsMetrics, error)
	SetCachedLatestSystemMetric(ctx context.Context, metric *OpsMetrics) error
	GetCachedDashboardOverview(ctx context.Context, timeRange string) (*DashboardOverviewData, error)
	SetCachedDashboardOverview(ctx context.Context, timeRange string, data *DashboardOverviewData, ttl time.Duration) error
	GetCachedPlatformConcurrency(ctx context.Context) (map[string]*PlatformConcurrencyInfo, error)
	GetCachedGroupConcurrency(ctx context.Context) (map[int64]*GroupConcurrencyInfo, error)
	// GetCachedConcurrencyCollectedAt returns the collection timestamp for concurrency cache, if available.
	GetCachedConcurrencyCollectedAt(ctx context.Context) (time.Time, bool, error)
	PingRedis(ctx context.Context) error

	// Account status monitoring methods
	// GetAllActiveAccountStatus returns account stats for all active accounts.
	// "Active" is defined by repository implementation (currently: accounts seen in ops_error_logs within 24h).
	GetAllActiveAccountStatus(ctx context.Context, platform string, groupID int64) ([]AccountStatusSummary, error)

	// IP statistics methods
	GetErrorStatsByIP(ctx context.Context, startTime, endTime time.Time, limit int, sortBy, sortOrder string) ([]IPErrorStats, error)
	GetErrorsByIP(ctx context.Context, ip string, startTime, endTime time.Time, page, pageSize int) ([]OpsErrorLog, int64, error)

	GetRetentionConfig(ctx context.Context) (map[string]int, error)
}

type OpsRepository interface {
	OpsIngestRepository
	OpsQueryRepository
}
