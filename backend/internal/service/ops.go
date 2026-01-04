package service

import (
	"context"
	"time"
)

// ErrorLog represents an ops error log item for list queries.
//
// Field naming matches docs/API-运维监控中心2.0.md (L3 根因追踪 - 错误日志列表).
type ErrorLog struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	Level        string `json:"level,omitempty"`
	RequestID    string `json:"request_id,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
	APIPath      string `json:"api_path,omitempty"`
	Provider     string `json:"provider,omitempty"`
	Model        string `json:"model,omitempty"`
	HTTPCode     int    `json:"http_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`

	DurationMs *int `json:"duration_ms,omitempty"`
	RetryCount *int `json:"retry_count,omitempty"`
	Stream     bool `json:"stream,omitempty"`
}

// ErrorLogFilter describes optional filters and pagination for listing ops error logs.
type ErrorLogFilter struct {
	StartTime *time.Time
	EndTime   *time.Time

	ErrorCode   *int
	Provider    string   // 保留用于向后兼容
	Platforms   []string // 新增：多平台过滤
	StatusCodes []int    // 新增：多状态码过滤
	ClientIP    string   // 新增：客户端 IP 过滤
	AccountID   *int64

	Page     int
	PageSize int
}

func (f *ErrorLogFilter) normalize() (page, pageSize int) {
	page = 1
	pageSize = 20
	if f == nil {
		return page, pageSize
	}

	if f.Page > 0 {
		page = f.Page
	}
	if f.PageSize > 0 {
		pageSize = f.PageSize
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

type ErrorLogListResponse struct {
	Errors   []*ErrorLog `json:"errors"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

func (s *OpsService) GetErrorLogs(ctx context.Context, filter *ErrorLogFilter) (*ErrorLogListResponse, error) {
	if s == nil || s.repo == nil {
		return &ErrorLogListResponse{
			Errors:   []*ErrorLog{},
			Total:    0,
			Page:     1,
			PageSize: 20,
		}, nil
	}

	page, pageSize := filter.normalize()
	filterCopy := &ErrorLogFilter{}
	if filter != nil {
		*filterCopy = *filter
	}
	filterCopy.Page = page
	filterCopy.PageSize = pageSize

	items, total, err := s.repo.ListErrorLogs(ctx, filterCopy)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []*ErrorLog{}
	}

	return &ErrorLogListResponse{
		Errors:   items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// OpsEmailNotificationConfig 邮件通知配置响应
type OpsEmailNotificationConfig struct {
	Alert  OpsEmailAlertConfig  `json:"alert"`
	Report OpsEmailReportConfig `json:"report"`
}

// OpsEmailAlertConfig 告警邮件通知配置
type OpsEmailAlertConfig struct {
	Enabled                   bool   `json:"enabled"`
	Recipients                string `json:"recipients"`
	MinSeverity               string `json:"min_severity"`
	RateLimitPerHour          int    `json:"rate_limit_per_hour"`
	BatchingWindowSeconds     int    `json:"batching_window_seconds"`
	IncludeResolvedAlerts     bool   `json:"include_resolved_alerts"`
}

// OpsEmailReportConfig 定时报告邮件通知配置
type OpsEmailReportConfig struct {
	Enabled                         bool    `json:"enabled"`
	Recipients                      string  `json:"recipients"`
	DailySummaryEnabled             bool    `json:"daily_summary_enabled"`
	DailySummarySchedule            string  `json:"daily_summary_schedule"`
	WeeklySummaryEnabled            bool    `json:"weekly_summary_enabled"`
	WeeklySummarySchedule           string  `json:"weekly_summary_schedule"`
	ErrorDigestEnabled              bool    `json:"error_digest_enabled"`
	ErrorDigestSchedule             string  `json:"error_digest_schedule"`
	ErrorDigestMinCount             int     `json:"error_digest_min_count"`
	AccountHealthEnabled            bool    `json:"account_health_enabled"`
	AccountHealthSchedule           string  `json:"account_health_schedule"`
	AccountHealthErrorRateThreshold float64 `json:"account_health_error_rate_threshold"`
}

// OpsEmailNotificationConfigUpdateRequest 更新邮件通知配置请求
type OpsEmailNotificationConfigUpdateRequest struct {
	Alert  *OpsEmailAlertConfig  `json:"alert"`
	Report *OpsEmailReportConfig `json:"report"`
}
