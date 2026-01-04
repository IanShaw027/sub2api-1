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
	OpsMetricHTTP2Errors        = "http2_errors"
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

func (s *OpsService) ListAlertRules(ctx context.Context) ([]OpsAlertRule, error) {
	return s.repo.ListAlertRules(ctx)
}

func (s *OpsService) CreateAlertRule(ctx context.Context, rule *OpsAlertRule) error {
	return s.repo.CreateAlertRule(ctx, rule)
}

func (s *OpsService) UpdateAlertRule(ctx context.Context, rule *OpsAlertRule) error {
	return s.repo.UpdateAlertRule(ctx, rule)
}

func (s *OpsService) DeleteAlertRule(ctx context.Context, id int64) error {
	return s.repo.DeleteAlertRule(ctx, id)
}

func (s *OpsService) ListAlertEvents(ctx context.Context, limit int) ([]OpsAlertEvent, error) {
	return s.repo.ListAlertEvents(ctx, limit)
}

func (s *OpsService) GetActiveAlertEvent(ctx context.Context, ruleID int64) (*OpsAlertEvent, error) {
	return s.repo.GetActiveAlertEvent(ctx, ruleID)
}

func (s *OpsService) GetLatestAlertEvent(ctx context.Context, ruleID int64) (*OpsAlertEvent, error) {
	return s.repo.GetLatestAlertEvent(ctx, ruleID)
}

func (s *OpsService) CreateAlertEvent(ctx context.Context, event *OpsAlertEvent) error {
	return s.repo.CreateAlertEvent(ctx, event)
}

func (s *OpsService) UpdateAlertEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error {
	return s.repo.UpdateAlertEventStatus(ctx, eventID, status, resolvedAt)
}

func (s *OpsService) UpdateAlertEventNotifications(ctx context.Context, eventID int64, emailSent bool) error {
	return s.repo.UpdateAlertEventNotifications(ctx, eventID, emailSent)
}

func (s *OpsService) ListRecentSystemMetrics(ctx context.Context, windowMinutes, limit int) ([]OpsMetrics, error) {
	return s.repo.ListRecentSystemMetrics(ctx, windowMinutes, limit)
}

func (s *OpsService) CountActiveAlerts(ctx context.Context) (int, error) {
	return s.repo.CountActiveAlerts(ctx)
}

// OpsGroupAvailabilityConfig 分组可用性监控配置
type OpsGroupAvailabilityConfig struct {
	ID       int64 `json:"id"`
	GroupID  int64 `json:"group_id"`

	Enabled              bool `json:"enabled"`
	MinAvailableAccounts int  `json:"min_available_accounts"`

	NotifyEmail   bool   `json:"notify_email"`

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

	EmailSent   bool `json:"email_sent"`

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

	MonitoringEnabled    bool `json:"monitoring_enabled"`
	MinAvailableAccounts int  `json:"min_available_accounts"`

	IsHealthy   bool       `json:"is_healthy"`
	AlertStatus string     `json:"alert_status"`
	LastAlertAt *time.Time `json:"last_alert_at"`
}

func (s *OpsService) ListGroupAvailabilityConfigs(ctx context.Context, enabledOnly bool) ([]OpsGroupAvailabilityConfig, error) {
	return s.repo.ListGroupAvailabilityConfigs(ctx, enabledOnly)
}

func (s *OpsService) GetGroupAvailabilityConfig(ctx context.Context, groupID int64) (*OpsGroupAvailabilityConfig, error) {
	return s.repo.GetGroupAvailabilityConfig(ctx, groupID)
}

func (s *OpsService) CreateGroupAvailabilityConfig(ctx context.Context, config *OpsGroupAvailabilityConfig) error {
	return s.repo.CreateGroupAvailabilityConfig(ctx, config)
}

func (s *OpsService) UpdateGroupAvailabilityConfig(ctx context.Context, config *OpsGroupAvailabilityConfig) error {
	return s.repo.UpdateGroupAvailabilityConfig(ctx, config)
}

func (s *OpsService) DeleteGroupAvailabilityConfig(ctx context.Context, groupID int64) error {
	return s.repo.DeleteGroupAvailabilityConfig(ctx, groupID)
}

func (s *OpsService) GetActiveGroupAvailabilityEvent(ctx context.Context, configID int64) (*OpsGroupAvailabilityEvent, error) {
	return s.repo.GetActiveGroupAvailabilityEvent(ctx, configID)
}

func (s *OpsService) GetLatestGroupAvailabilityEvent(ctx context.Context, configID int64) (*OpsGroupAvailabilityEvent, error) {
	return s.repo.GetLatestGroupAvailabilityEvent(ctx, configID)
}

func (s *OpsService) CreateGroupAvailabilityEvent(ctx context.Context, event *OpsGroupAvailabilityEvent) error {
	return s.repo.CreateGroupAvailabilityEvent(ctx, event)
}

func (s *OpsService) UpdateGroupAvailabilityEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error {
	return s.repo.UpdateGroupAvailabilityEventStatus(ctx, eventID, status, resolvedAt)
}

func (s *OpsService) UpdateGroupAvailabilityEventNotifications(ctx context.Context, eventID int64, emailSent bool) error {
	return s.repo.UpdateGroupAvailabilityEventNotifications(ctx, eventID, emailSent)
}

func (s *OpsService) ListGroupAvailabilityEvents(ctx context.Context, limit int, status string) ([]OpsGroupAvailabilityEvent, error) {
	return s.repo.ListGroupAvailabilityEvents(ctx, limit, status)
}

func (s *OpsService) CountAvailableAccountsByGroup(ctx context.Context, groupID int64) (available, total int, err error) {
	return s.repo.CountAvailableAccountsByGroup(ctx, groupID)
}
