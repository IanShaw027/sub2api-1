package service

import (
	"context"
	"time"
)

const (
	OpsAlertStatusFiring   = "firing"
	OpsAlertStatusResolved = "resolved"
)

const (
	OpsMetricSuccessRate        = "success_rate"
	OpsMetricErrorRate          = "error_rate"
	OpsMetricP95LatencyMs       = "p95_latency_ms"
	OpsMetricP99LatencyMs       = "p99_latency_ms"
	OpsMetricCPUUsagePercent    = "cpu_usage_percent"
	OpsMetricMemoryUsagePercent = "memory_usage_percent"
	OpsMetricQueueDepth         = "concurrency_queue_depth"
)

type OpsAlertRule struct {
	ID                    int64          `json:"id"`
	Name                  string         `json:"name"`
	Description           string         `json:"description"`
	Enabled               bool           `json:"enabled"`
	MetricType            string         `json:"metric_type"`
	Operator              string         `json:"operator"`
	Threshold             float64        `json:"threshold"`
	WindowMinutes         int            `json:"window_minutes"`
	SustainedMinutes      int            `json:"sustained_minutes"`
	Severity              string         `json:"severity"`
	NotifyEmail           bool           `json:"notify_email"`
	CooldownMinutes       int            `json:"cooldown_minutes"`
	DimensionFilters      map[string]any `json:"dimension_filters,omitempty"`
	NotifyChannels        []string       `json:"notify_channels,omitempty"`
	NotifyConfig          map[string]any `json:"notify_config,omitempty"`
	AlertCategory         string         `json:"alert_category,omitempty"`
	FilterConditions      map[string]any `json:"filter_conditions,omitempty"`
	AggregationDimensions []string       `json:"aggregation_dimensions,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

type OpsAlertEvent struct {
	ID             int64      `json:"id"`
	RuleID         int64      `json:"rule_id"`
	Severity       string     `json:"severity"`
	Status         string     `json:"status"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	MetricValue    float64    `json:"metric_value"`
	ThresholdValue float64    `json:"threshold_value"`
	FiredAt        time.Time  `json:"fired_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	EmailSent      bool       `json:"email_sent"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (s *OpsQueryService) ListAlertRules(ctx context.Context) ([]OpsAlertRule, error) {
	return s.repo.ListAlertRules(ctx)
}

func (s *OpsIngestService) CreateAlertRule(ctx context.Context, rule *OpsAlertRule) error {
	return s.repo.CreateAlertRule(ctx, rule)
}

func (s *OpsIngestService) UpdateAlertRule(ctx context.Context, rule *OpsAlertRule) error {
	return s.repo.UpdateAlertRule(ctx, rule)
}

func (s *OpsIngestService) DeleteAlertRule(ctx context.Context, id int64) error {
	return s.repo.DeleteAlertRule(ctx, id)
}

func (s *OpsQueryService) ListAlertEvents(ctx context.Context, limit int) ([]OpsAlertEvent, error) {
	return s.repo.ListAlertEvents(ctx, limit)
}

func (s *OpsQueryService) GetActiveAlertEvent(ctx context.Context, ruleID int64) (*OpsAlertEvent, error) {
	return s.repo.GetActiveAlertEvent(ctx, ruleID)
}

func (s *OpsQueryService) GetLatestAlertEvent(ctx context.Context, ruleID int64) (*OpsAlertEvent, error) {
	return s.repo.GetLatestAlertEvent(ctx, ruleID)
}

func (s *OpsIngestService) CreateAlertEvent(ctx context.Context, event *OpsAlertEvent) error {
	return s.repo.CreateAlertEvent(ctx, event)
}

func (s *OpsIngestService) UpdateAlertEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error {
	return s.repo.UpdateAlertEventStatus(ctx, eventID, status, resolvedAt)
}

func (s *OpsIngestService) UpdateAlertEventNotifications(ctx context.Context, eventID int64, emailSent bool) error {
	return s.repo.UpdateAlertEventNotifications(ctx, eventID, emailSent)
}

func (s *OpsQueryService) ListRecentSystemMetrics(ctx context.Context, windowMinutes, limit int) ([]OpsMetrics, error) {
	return s.repo.ListRecentSystemMetrics(ctx, windowMinutes, limit)
}

func (s *OpsQueryService) CountActiveAlerts(ctx context.Context) (int, error) {
	return s.repo.CountActiveAlerts(ctx)
}

// OpsGroupAvailabilityConfig 分组可用性监控配置
type OpsGroupAvailabilityConfig struct {
	ID      int64 `json:"id"`
	GroupID int64 `json:"group_id"`

	Enabled              bool `json:"enabled"`
	MinAvailableAccounts int  `json:"min_available_accounts"`
	// ThresholdMode controls how availability is evaluated:
	// - count: only `MinAvailableAccounts` is enforced
	// - percentage: only `MinAvailablePercentage` is enforced
	// - both: both thresholds must be satisfied
	ThresholdMode          string  `json:"threshold_mode"`
	MinAvailablePercentage float64 `json:"min_available_percentage"`

	NotifyEmail bool `json:"notify_email"`

	Severity        string `json:"severity"`
	CooldownMinutes int    `json:"cooldown_minutes"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Group *Group `json:"group,omitempty"`
}

// OpsGroupAvailabilityEvent 分组可用性告警事件
type OpsGroupAvailabilityEvent struct {
	ID       int64 `json:"id"`
	ConfigID int64 `json:"config_id"`
	GroupID  int64 `json:"group_id"`

	Status   string `json:"status"`
	Severity string `json:"severity"`

	Title       string `json:"title"`
	Description string `json:"description"`

	AvailableAccounts int `json:"available_accounts"`
	ThresholdAccounts int `json:"threshold_accounts"`
	TotalAccounts     int `json:"total_accounts"`

	EmailSent bool `json:"email_sent"`

	FiredAt    time.Time  `json:"fired_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
	CreatedAt  time.Time  `json:"created_at"`

	Group *Group `json:"group,omitempty"`
}

// OpsGroupAvailabilityStatus 分组可用性实时状态
type OpsGroupAvailabilityStatus struct {
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
	Platform  string `json:"platform"`

	TotalAccounts     int `json:"total_accounts"`
	AvailableAccounts int `json:"available_accounts"`
	DisabledAccounts  int `json:"disabled_accounts"`
	ErrorAccounts     int `json:"error_accounts"`
	OverloadAccounts  int `json:"overload_accounts"`

	MonitoringEnabled      bool    `json:"monitoring_enabled"`
	MinAvailableAccounts   int     `json:"min_available_accounts"`
	ThresholdMode          string  `json:"threshold_mode"`
	MinAvailablePercentage float64 `json:"min_available_percentage"`

	IsHealthy   bool       `json:"is_healthy"`
	AlertStatus string     `json:"alert_status"`
	LastAlertAt *time.Time `json:"last_alert_at"`
}

func (s *OpsQueryService) ListGroupAvailabilityConfigs(ctx context.Context, enabledOnly bool) ([]OpsGroupAvailabilityConfig, error) {
	return s.repo.ListGroupAvailabilityConfigs(ctx, enabledOnly)
}

func (s *OpsQueryService) GetGroupAvailabilityConfig(ctx context.Context, groupID int64) (*OpsGroupAvailabilityConfig, error) {
	return s.repo.GetGroupAvailabilityConfig(ctx, groupID)
}

func (s *OpsIngestService) CreateGroupAvailabilityConfig(ctx context.Context, config *OpsGroupAvailabilityConfig) error {
	return s.repo.CreateGroupAvailabilityConfig(ctx, config)
}

func (s *OpsIngestService) UpdateGroupAvailabilityConfig(ctx context.Context, config *OpsGroupAvailabilityConfig) error {
	return s.repo.UpdateGroupAvailabilityConfig(ctx, config)
}

func (s *OpsIngestService) DeleteGroupAvailabilityConfig(ctx context.Context, groupID int64) error {
	return s.repo.DeleteGroupAvailabilityConfig(ctx, groupID)
}

func (s *OpsQueryService) GetActiveGroupAvailabilityEvent(ctx context.Context, configID int64) (*OpsGroupAvailabilityEvent, error) {
	return s.repo.GetActiveGroupAvailabilityEvent(ctx, configID)
}

func (s *OpsQueryService) GetLatestGroupAvailabilityEvent(ctx context.Context, configID int64) (*OpsGroupAvailabilityEvent, error) {
	return s.repo.GetLatestGroupAvailabilityEvent(ctx, configID)
}

func (s *OpsIngestService) CreateGroupAvailabilityEvent(ctx context.Context, event *OpsGroupAvailabilityEvent) error {
	return s.repo.CreateGroupAvailabilityEvent(ctx, event)
}

func (s *OpsIngestService) UpdateGroupAvailabilityEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error {
	return s.repo.UpdateGroupAvailabilityEventStatus(ctx, eventID, status, resolvedAt)
}

func (s *OpsIngestService) UpdateGroupAvailabilityEventNotifications(ctx context.Context, eventID int64, emailSent bool) error {
	return s.repo.UpdateGroupAvailabilityEventNotifications(ctx, eventID, emailSent)
}

func (s *OpsQueryService) ListGroupAvailabilityEvents(ctx context.Context, limit int, status string) ([]OpsGroupAvailabilityEvent, error) {
	return s.repo.ListGroupAvailabilityEvents(ctx, limit, status)
}

func (s *OpsQueryService) CountAvailableAccountsByGroup(ctx context.Context, groupID int64) (available, total int, err error) {
	return s.repo.CountAvailableAccountsByGroup(ctx, groupID)
}
