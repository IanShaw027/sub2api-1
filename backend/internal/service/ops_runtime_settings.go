package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type OpsDistributedLockSettings struct {
	Enabled    bool   `json:"enabled"`
	Key        string `json:"key"`
	TTLSeconds int    `json:"ttl_seconds"`
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
	Silencing       OpsAlertSilencingSettings  `json:"silencing"`
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

// EffectiveOpsAlertRuntimeSettings computes the effective runtime settings for OpsAlertService,
// applying sane fallbacks and enforcing single-node overrides (when applicable).
func (s *OpsSettingsService) EffectiveOpsAlertRuntimeSettings(ctx context.Context, singleNodeMode bool) (time.Duration, OpsDistributedLockSettings, OpsAlertSilencingSettings) {
	defaultCfg := defaultOpsAlertRuntimeSettings()
	cfg := defaultCfg

	if s != nil {
		if loaded, err := s.GetOpsAlertRuntimeSettings(ctx); err == nil && loaded != nil {
			cfg = loaded
		}
	}

	interval := time.Duration(cfg.EvaluationIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = opsAlertEvalInterval
	}

	lock := cfg.DistributedLock
	if singleNodeMode {
		lock.Enabled = false
	}
	normalizeOpsDistributedLockSettings(&lock, opsAlertLeaderLockKeyDefault, defaultCfg.DistributedLock.TTLSeconds)

	silencing := cfg.Silencing
	normalizeOpsAlertSilencingSettings(&silencing)

	return interval, lock, silencing
}

// EffectiveOpsGroupAvailabilityRuntimeSettings computes the effective runtime settings for OpsGroupAvailabilityMonitor,
// applying fallbacks and enforcing single-node overrides (when applicable).
func (s *OpsSettingsService) EffectiveOpsGroupAvailabilityRuntimeSettings(ctx context.Context, singleNodeMode bool) (time.Duration, OpsDistributedLockSettings) {
	defaultCfg := defaultOpsGroupAvailabilityRuntimeSettings()
	cfg := defaultCfg

	if s != nil {
		if loaded, err := s.GetOpsGroupAvailabilityRuntimeSettings(ctx); err == nil && loaded != nil {
			cfg = loaded
		}
	}

	interval := time.Duration(cfg.EvaluationIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = opsGroupAvailabilityMonitorInterval
	}

	lock := cfg.DistributedLock
	if singleNodeMode {
		lock.Enabled = false
	}
	normalizeOpsDistributedLockSettings(&lock, opsGroupAvailabilityLeaderLockKeyDefault, defaultCfg.DistributedLock.TTLSeconds)

	return interval, lock
}

func (s *OpsSettingsService) GetOpsAlertRuntimeSettings(ctx context.Context) (*OpsAlertRuntimeSettings, error) {
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
	normalizeOpsAlertSilencingSettings(&cfg.Silencing)

	return cfg, nil
}

func (s *OpsSettingsService) UpdateOpsAlertRuntimeSettings(ctx context.Context, cfg *OpsAlertRuntimeSettings) (*OpsAlertRuntimeSettings, error) {
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
	normalizeOpsAlertSilencingSettings(&cfg.Silencing)

	if cfg.EvaluationIntervalSeconds <= 0 || cfg.EvaluationIntervalSeconds > int((24*time.Hour).Seconds()) {
		return nil, errors.New("evaluation_interval_seconds must be between 1 and 86400")
	}
	if err := validateOpsDistributedLockSettings(cfg.DistributedLock); err != nil {
		return nil, err
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

func (s *OpsSettingsService) GetOpsGroupAvailabilityRuntimeSettings(ctx context.Context) (*OpsGroupAvailabilityRuntimeSettings, error) {
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

func (s *OpsSettingsService) UpdateOpsGroupAvailabilityRuntimeSettings(ctx context.Context, cfg *OpsGroupAvailabilityRuntimeSettings) (*OpsGroupAvailabilityRuntimeSettings, error) {
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
