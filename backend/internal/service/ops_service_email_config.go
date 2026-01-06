package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// GetEmailNotificationConfig 获取邮件通知配置
func (s *OpsSettingsService) GetEmailNotificationConfig(ctx context.Context) (*OpsEmailNotificationConfig, error) {
	defaultCfg := defaultOpsEmailNotificationConfig()
	if s == nil || s.settingRepo == nil {
		return defaultCfg, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyOpsEmailNotificationConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			// Initialize defaults on first read.
			if b, mErr := json.Marshal(defaultCfg); mErr == nil {
				_ = s.settingRepo.Set(ctx, SettingKeyOpsEmailNotificationConfig, string(b))
			}
			return defaultCfg, nil
		}
		return nil, err
	}

	cfg := &OpsEmailNotificationConfig{}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		// If stored JSON is corrupted, fall back to defaults (best-effort) to avoid breaking ops UI.
		return defaultCfg, nil
	}

	normalizeOpsEmailNotificationConfig(cfg)
	return cfg, nil
}

// UpdateEmailNotificationConfig 更新邮件通知配置
func (s *OpsSettingsService) UpdateEmailNotificationConfig(ctx context.Context, req *OpsEmailNotificationConfigUpdateRequest) error {
	if s == nil || s.settingRepo == nil {
		return errors.New("setting repository not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if req == nil {
		return errors.New("invalid request")
	}

	cfg, err := s.GetEmailNotificationConfig(ctx)
	if err != nil {
		return err
	}

	if req.Alert != nil {
		cfg.Alert.Enabled = req.Alert.Enabled
		if req.Alert.Recipients != nil {
			cfg.Alert.Recipients = req.Alert.Recipients
		}
		cfg.Alert.MinSeverity = strings.TrimSpace(req.Alert.MinSeverity)
		cfg.Alert.RateLimitPerHour = req.Alert.RateLimitPerHour
		cfg.Alert.BatchingWindowSeconds = req.Alert.BatchingWindowSeconds
		cfg.Alert.IncludeResolvedAlerts = req.Alert.IncludeResolvedAlerts
	}

	if req.Report != nil {
		cfg.Report.Enabled = req.Report.Enabled
		if req.Report.Recipients != nil {
			cfg.Report.Recipients = req.Report.Recipients
		}
		cfg.Report.DailySummaryEnabled = req.Report.DailySummaryEnabled
		cfg.Report.DailySummarySchedule = strings.TrimSpace(req.Report.DailySummarySchedule)
		cfg.Report.WeeklySummaryEnabled = req.Report.WeeklySummaryEnabled
		cfg.Report.WeeklySummarySchedule = strings.TrimSpace(req.Report.WeeklySummarySchedule)
		cfg.Report.ErrorDigestEnabled = req.Report.ErrorDigestEnabled
		cfg.Report.ErrorDigestSchedule = strings.TrimSpace(req.Report.ErrorDigestSchedule)
		cfg.Report.ErrorDigestMinCount = req.Report.ErrorDigestMinCount
		cfg.Report.AccountHealthEnabled = req.Report.AccountHealthEnabled
		cfg.Report.AccountHealthSchedule = strings.TrimSpace(req.Report.AccountHealthSchedule)
		cfg.Report.AccountHealthErrorRateThreshold = req.Report.AccountHealthErrorRateThreshold
	}

	if err := validateOpsEmailNotificationConfig(cfg); err != nil {
		return err
	}

	normalizeOpsEmailNotificationConfig(cfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.settingRepo.Set(ctx, SettingKeyOpsEmailNotificationConfig, string(raw))
}

func defaultOpsEmailNotificationConfig() *OpsEmailNotificationConfig {
	return &OpsEmailNotificationConfig{
		Alert: OpsEmailAlertConfig{
			Enabled:               true,
			Recipients:            []string{},
			MinSeverity:           "",
			RateLimitPerHour:      0,
			BatchingWindowSeconds: 0,
			IncludeResolvedAlerts: false,
		},
		Report: OpsEmailReportConfig{
			Enabled:                         false,
			Recipients:                      []string{},
			DailySummaryEnabled:             false,
			DailySummarySchedule:            "0 9 * * *",
			WeeklySummaryEnabled:            false,
			WeeklySummarySchedule:           "0 9 * * 1",
			ErrorDigestEnabled:              false,
			ErrorDigestSchedule:             "0 9 * * *",
			ErrorDigestMinCount:             10,
			AccountHealthEnabled:            false,
			AccountHealthSchedule:           "0 9 * * *",
			AccountHealthErrorRateThreshold: 10.0,
		},
	}
}

func normalizeOpsEmailNotificationConfig(cfg *OpsEmailNotificationConfig) {
	if cfg == nil {
		return
	}
	if cfg.Alert.Recipients == nil {
		cfg.Alert.Recipients = []string{}
	}
	if cfg.Report.Recipients == nil {
		cfg.Report.Recipients = []string{}
	}

	cfg.Alert.MinSeverity = strings.TrimSpace(cfg.Alert.MinSeverity)
	cfg.Report.DailySummarySchedule = strings.TrimSpace(cfg.Report.DailySummarySchedule)
	cfg.Report.WeeklySummarySchedule = strings.TrimSpace(cfg.Report.WeeklySummarySchedule)
	cfg.Report.ErrorDigestSchedule = strings.TrimSpace(cfg.Report.ErrorDigestSchedule)
	cfg.Report.AccountHealthSchedule = strings.TrimSpace(cfg.Report.AccountHealthSchedule)

	// Fill missing schedules with defaults to avoid breaking cron logic if clients send empty strings.
	if cfg.Report.DailySummarySchedule == "" {
		cfg.Report.DailySummarySchedule = "0 9 * * *"
	}
	if cfg.Report.WeeklySummarySchedule == "" {
		cfg.Report.WeeklySummarySchedule = "0 9 * * 1"
	}
	if cfg.Report.ErrorDigestSchedule == "" {
		cfg.Report.ErrorDigestSchedule = "0 9 * * *"
	}
	if cfg.Report.AccountHealthSchedule == "" {
		cfg.Report.AccountHealthSchedule = "0 9 * * *"
	}
}

func validateOpsEmailNotificationConfig(cfg *OpsEmailNotificationConfig) error {
	if cfg == nil {
		return errors.New("invalid config")
	}

	if cfg.Alert.RateLimitPerHour < 0 {
		return errors.New("alert.rate_limit_per_hour must be >= 0")
	}
	if cfg.Alert.BatchingWindowSeconds < 0 {
		return errors.New("alert.batching_window_seconds must be >= 0")
	}
	switch strings.TrimSpace(cfg.Alert.MinSeverity) {
	case "", "critical", "warning", "info":
	default:
		return errors.New("alert.min_severity must be one of: critical, warning, info, or empty")
	}

	if cfg.Report.ErrorDigestMinCount < 0 {
		return errors.New("report.error_digest_min_count must be >= 0")
	}
	if cfg.Report.AccountHealthErrorRateThreshold < 0 || cfg.Report.AccountHealthErrorRateThreshold > 100 {
		return errors.New("report.account_health_error_rate_threshold must be between 0 and 100")
	}
	return nil
}
