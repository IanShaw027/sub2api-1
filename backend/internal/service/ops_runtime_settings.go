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

type OpsAlertRuntimeSettings struct {
	// EvaluationIntervalSeconds controls how often the alert evaluator runs.
	// 0 means "use default".
	EvaluationIntervalSeconds int `json:"evaluation_interval_seconds"`

	DistributedLock OpsDistributedLockSettings `json:"distributed_lock"`
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

func validateOpsDistributedLockSettings(s OpsDistributedLockSettings) error {
	if strings.TrimSpace(s.Key) == "" {
		return errors.New("distributed_lock.key is required")
	}
	if s.TTLSeconds <= 0 || s.TTLSeconds > int((24*time.Hour).Seconds()) {
		return errors.New("distributed_lock.ttl_seconds must be between 1 and 86400")
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
