package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
)

type OpsDistributedLockSettings struct {
	Enabled    bool   `json:"enabled"`
	Key        string `json:"key"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type OpsAlertWebhookSettings struct {
	Enabled         bool   `json:"enabled"`
	URL             string `json:"url"`
	Secret          string `json:"secret"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	MaxRetries      int    `json:"max_retries"`
	IncludeResolved bool   `json:"include_resolved"`
	MinSeverity     string `json:"min_severity"`
}

type OpsAlertSilenceEntry struct {
	// Optional matchers (empty means "match all").
	RuleID     *int64   `json:"rule_id,omitempty"`
	Severities []string `json:"severities,omitempty"`

	// UntilRFC3339 is the RFC3339 time when the silence expires.
	UntilRFC3339 string `json:"until_rfc3339"`
	Reason       string `json:"reason"`
}

type OpsAlertSilencingSettings struct {
	Enabled bool `json:"enabled"`

	// GlobalUntilRFC3339 silences all alert notifications until this time (RFC3339).
	GlobalUntilRFC3339 string `json:"global_until_rfc3339"`
	GlobalReason       string `json:"global_reason"`

	Entries []OpsAlertSilenceEntry `json:"entries,omitempty"`
}

type OpsAlertRuntimeSettings struct {
	// EvaluationIntervalSeconds controls how often the alert evaluator runs.
	// 0 means "use default".
	EvaluationIntervalSeconds int `json:"evaluation_interval_seconds"`

	DistributedLock OpsDistributedLockSettings `json:"distributed_lock"`

	Webhook   OpsAlertWebhookSettings   `json:"webhook"`
	Silencing OpsAlertSilencingSettings `json:"silencing"`
}

type OpsGroupAvailabilityRuntimeSettings struct {
	// EvaluationIntervalSeconds controls how often group availability monitoring runs.
	// 0 means "use default".
	EvaluationIntervalSeconds int `json:"evaluation_interval_seconds"`

	DistributedLock OpsDistributedLockSettings `json:"distributed_lock"`
}

func defaultOpsAlertRuntimeSettings() *OpsAlertRuntimeSettings {
	defaultInterval := 0
	switch {
	case opsAlertEvalInterval <= 0:
		// Keep a sensible production default even if opsAlertEvalInterval is misconfigured.
		defaultInterval = 60
	case opsAlertEvalInterval < time.Second:
		// Allow tests to override opsAlertEvalInterval to sub-second values without being forced
		// into the DB-backed integer seconds setting.
		defaultInterval = 0
	default:
		defaultInterval = int(opsAlertEvalInterval.Seconds())
	}
	return &OpsAlertRuntimeSettings{
		EvaluationIntervalSeconds: defaultInterval,
		DistributedLock: OpsDistributedLockSettings{
			Enabled:    true,
			Key:        opsAlertLeaderLockKeyDefault,
			TTLSeconds: int(opsAlertLeaderLockTTLDefault.Seconds()),
		},
		Webhook: OpsAlertWebhookSettings{
			Enabled:         false,
			URL:             "",
			Secret:          "",
			TimeoutSeconds:  5,
			MaxRetries:      2,
			IncludeResolved: false,
			MinSeverity:     "",
		},
		Silencing: OpsAlertSilencingSettings{
			Enabled:            false,
			GlobalUntilRFC3339: "",
			GlobalReason:       "",
			Entries:            []OpsAlertSilenceEntry{},
		},
	}
}

func defaultOpsGroupAvailabilityRuntimeSettings() *OpsGroupAvailabilityRuntimeSettings {
	defaultInterval := 0
	switch {
	case opsGroupAvailabilityMonitorInterval <= 0:
		// Keep a sensible production default even if opsGroupAvailabilityMonitorInterval is misconfigured.
		defaultInterval = 300
	case opsGroupAvailabilityMonitorInterval < time.Second:
		defaultInterval = 0
	default:
		defaultInterval = int(opsGroupAvailabilityMonitorInterval.Seconds())
	}
	return &OpsGroupAvailabilityRuntimeSettings{
		EvaluationIntervalSeconds: defaultInterval,
		DistributedLock: OpsDistributedLockSettings{
			Enabled:    true,
			Key:        opsGroupAvailabilityLeaderLockKeyDefault,
			TTLSeconds: int(opsGroupAvailabilityLeaderLockTTLDefault.Seconds()),
		},
	}
}

func normalizeOpsDistributedLockSettings(s *OpsDistributedLockSettings, defaultKey string, defaultTTLSeconds int) {
	if s == nil {
		return
	}
	s.Key = strings.TrimSpace(s.Key)
	if s.Key == "" {
		s.Key = defaultKey
	}
	if s.TTLSeconds <= 0 {
		s.TTLSeconds = defaultTTLSeconds
	}
}

func normalizeOpsAlertWebhookSettings(s *OpsAlertWebhookSettings, defaults OpsAlertWebhookSettings) {
	if s == nil {
		return
	}
	s.URL = strings.TrimSpace(s.URL)
	if s.TimeoutSeconds <= 0 {
		s.TimeoutSeconds = defaults.TimeoutSeconds
	}
	if s.MaxRetries < 0 {
		s.MaxRetries = defaults.MaxRetries
	}
	s.MinSeverity = strings.TrimSpace(s.MinSeverity)
}

func normalizeOpsAlertSilencingSettings(s *OpsAlertSilencingSettings) {
	if s == nil {
		return
	}
	s.GlobalUntilRFC3339 = strings.TrimSpace(s.GlobalUntilRFC3339)
	s.GlobalReason = strings.TrimSpace(s.GlobalReason)
	if s.Entries == nil {
		s.Entries = []OpsAlertSilenceEntry{}
	}
	for i := range s.Entries {
		s.Entries[i].UntilRFC3339 = strings.TrimSpace(s.Entries[i].UntilRFC3339)
		s.Entries[i].Reason = strings.TrimSpace(s.Entries[i].Reason)
	}
}

func validateOpsDistributedLockSettings(s OpsDistributedLockSettings) error {
	if strings.TrimSpace(s.Key) == "" {
		return errors.New("distributed_lock.key is required")
	}
	if s.TTLSeconds <= 0 || s.TTLSeconds > int((24*time.Hour).Seconds()) {
		return errors.New("distributed_lock.ttl_seconds must be between 1 and 86400")
	}
	return nil
}

func validateOpsAlertSilencingSettings(s OpsAlertSilencingSettings) error {
	parse := func(raw string) error {
		if strings.TrimSpace(raw) == "" {
			return nil
		}
		if _, err := time.Parse(time.RFC3339, raw); err != nil {
			return errors.New("silencing time must be RFC3339")
		}
		return nil
	}

	if err := parse(s.GlobalUntilRFC3339); err != nil {
		return err
	}
	for _, entry := range s.Entries {
		if strings.TrimSpace(entry.UntilRFC3339) == "" {
			return errors.New("silencing.entries.until_rfc3339 is required")
		}
		if _, err := time.Parse(time.RFC3339, entry.UntilRFC3339); err != nil {
			return errors.New("silencing.entries.until_rfc3339 must be RFC3339")
		}
	}
	return nil
}

func (s *OpsService) GetOpsAlertRuntimeSettings(ctx context.Context) (*OpsAlertRuntimeSettings, error) {
	defaultCfg := defaultOpsAlertRuntimeSettings()
	if s == nil || s.settingRepo == nil {
		return defaultCfg, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyOpsAlertRuntimeSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			if b, mErr := json.Marshal(defaultCfg); mErr == nil {
				_ = s.settingRepo.Set(ctx, SettingKeyOpsAlertRuntimeSettings, string(b))
			}
			return defaultCfg, nil
		}
		return nil, err
	}

	cfg := &OpsAlertRuntimeSettings{}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return defaultCfg, nil
	}

	if cfg.EvaluationIntervalSeconds <= 0 {
		cfg.EvaluationIntervalSeconds = defaultCfg.EvaluationIntervalSeconds
	}
	normalizeOpsDistributedLockSettings(&cfg.DistributedLock, opsAlertLeaderLockKeyDefault, defaultCfg.DistributedLock.TTLSeconds)
	normalizeOpsAlertWebhookSettings(&cfg.Webhook, defaultCfg.Webhook)
	normalizeOpsAlertSilencingSettings(&cfg.Silencing)

	return cfg, nil
}

func (s *OpsService) UpdateOpsAlertRuntimeSettings(ctx context.Context, cfg *OpsAlertRuntimeSettings) (*OpsAlertRuntimeSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("setting repository not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil {
		return nil, errors.New("invalid config")
	}

	defaultCfg := defaultOpsAlertRuntimeSettings()
	if cfg.EvaluationIntervalSeconds <= 0 {
		cfg.EvaluationIntervalSeconds = defaultCfg.EvaluationIntervalSeconds
	}
	normalizeOpsDistributedLockSettings(&cfg.DistributedLock, opsAlertLeaderLockKeyDefault, defaultCfg.DistributedLock.TTLSeconds)
	normalizeOpsAlertWebhookSettings(&cfg.Webhook, defaultCfg.Webhook)
	normalizeOpsAlertSilencingSettings(&cfg.Silencing)

	if cfg.EvaluationIntervalSeconds <= 0 || cfg.EvaluationIntervalSeconds > int((24*time.Hour).Seconds()) {
		return nil, errors.New("evaluation_interval_seconds must be between 1 and 86400")
	}
	if err := validateOpsDistributedLockSettings(cfg.DistributedLock); err != nil {
		return nil, err
	}
	if cfg.Webhook.Enabled && cfg.Webhook.URL == "" {
		return nil, errors.New("webhook.url is required when webhook is enabled")
	}
	if cfg.Webhook.Enabled {
		parsed, err := url.Parse(strings.TrimSpace(cfg.Webhook.URL))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, errors.New("webhook.url is invalid")
		}
		if parsed.Scheme != "https" && parsed.Scheme != "http" {
			return nil, errors.New("webhook.url must start with http:// or https://")
		}
	}
	if cfg.Webhook.TimeoutSeconds <= 0 || cfg.Webhook.TimeoutSeconds > int((5*time.Minute).Seconds()) {
		return nil, errors.New("webhook.timeout_seconds must be between 1 and 300")
	}
	if cfg.Webhook.MaxRetries < 0 || cfg.Webhook.MaxRetries > 10 {
		return nil, errors.New("webhook.max_retries must be between 0 and 10")
	}
	if err := validateOpsAlertSilencingSettings(cfg.Silencing); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyOpsAlertRuntimeSettings, string(raw)); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *OpsService) GetOpsGroupAvailabilityRuntimeSettings(ctx context.Context) (*OpsGroupAvailabilityRuntimeSettings, error) {
	defaultCfg := defaultOpsGroupAvailabilityRuntimeSettings()
	if s == nil || s.settingRepo == nil {
		return defaultCfg, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyOpsGroupAvailabilityRuntimeSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			if b, mErr := json.Marshal(defaultCfg); mErr == nil {
				_ = s.settingRepo.Set(ctx, SettingKeyOpsGroupAvailabilityRuntimeSettings, string(b))
			}
			return defaultCfg, nil
		}
		return nil, err
	}

	cfg := &OpsGroupAvailabilityRuntimeSettings{}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return defaultCfg, nil
	}

	if cfg.EvaluationIntervalSeconds <= 0 {
		cfg.EvaluationIntervalSeconds = defaultCfg.EvaluationIntervalSeconds
	}
	normalizeOpsDistributedLockSettings(&cfg.DistributedLock, opsGroupAvailabilityLeaderLockKeyDefault, defaultCfg.DistributedLock.TTLSeconds)
	return cfg, nil
}

func (s *OpsService) UpdateOpsGroupAvailabilityRuntimeSettings(ctx context.Context, cfg *OpsGroupAvailabilityRuntimeSettings) (*OpsGroupAvailabilityRuntimeSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("setting repository not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil {
		return nil, errors.New("invalid config")
	}

	defaultCfg := defaultOpsGroupAvailabilityRuntimeSettings()
	if cfg.EvaluationIntervalSeconds <= 0 {
		cfg.EvaluationIntervalSeconds = defaultCfg.EvaluationIntervalSeconds
	}
	normalizeOpsDistributedLockSettings(&cfg.DistributedLock, opsGroupAvailabilityLeaderLockKeyDefault, defaultCfg.DistributedLock.TTLSeconds)

	if cfg.EvaluationIntervalSeconds <= 0 || cfg.EvaluationIntervalSeconds > int((24*time.Hour).Seconds()) {
		return nil, errors.New("evaluation_interval_seconds must be between 1 and 86400")
	}
	if err := validateOpsDistributedLockSettings(cfg.DistributedLock); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyOpsGroupAvailabilityRuntimeSettings, string(raw)); err != nil {
		return nil, err
	}
	return cfg, nil
}
