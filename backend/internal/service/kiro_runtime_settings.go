package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"golang.org/x/sync/singleflight"
)

const KiroCacheMinBlockTokensMax = 1 << 20

type KiroRuntimeSettings struct {
	KiroVersion                 string           `json:"kiro_version"`
	KiroCommit                  string           `json:"kiro_commit"`
	SystemVersion               string           `json:"system_version"`
	NodeVersion                 string           `json:"node_version"`
	CacheHitRateScale           int              `json:"cache_hit_rate_scale"`
	CacheMinBlockTokens         int              `json:"cache_min_block_tokens"`
	CacheIndependentTTLSecs     int              `json:"cache_independent_ttl_seconds"`
	CachePrefixTTLSecs          int              `json:"cache_prefix_ttl_seconds"`
	ThinkingMode                KiroThinkingMode `json:"-"`
	ThinkingEffortThreshold     string           `json:"-"`
	ThinkingSimulationTemplate  string           `json:"-"`
	CodeExecutionSandboxCommand string           `json:"kiro_code_execution_sandbox_command"`
}

type KiroThinkingMode string

const (
	KiroThinkingModeDisabled         KiroThinkingMode = ""
	KiroThinkingModeSimulate         KiroThinkingMode = "simulate"
	KiroThinkingModeModelAndSimulate KiroThinkingMode = "model_and_simulate"
)

const (
	defaultKiroVersion             = "0.10.0"
	defaultKiroSystemVersion       = "darwin#24.6.0"
	defaultKiroNodeVersion         = "22.21.1"
	defaultKiroCacheHitRateScale   = 85
	defaultKiroCacheMinBlockTokens = 1024
	defaultKiroCacheIndependentTTL = 3600
	defaultKiroCachePrefixTTL      = 3600
)

type cachedKiroRuntimeSettings struct {
	settings  *KiroRuntimeSettings
	expiresAt int64
}

var kiroRuntimeSettingsCache atomic.Value
var kiroRuntimeSettingsSF singleflight.Group

const kiroRuntimeSettingsCacheTTL = 60 * time.Second
const kiroRuntimeSettingsErrorTTL = 5 * time.Second
const kiroRuntimeSettingsDBTimeout = 5 * time.Second

func DefaultKiroRuntimeSettings() *KiroRuntimeSettings {
	return &KiroRuntimeSettings{
		KiroVersion:                 defaultKiroVersion,
		KiroCommit:                  "",
		SystemVersion:               defaultKiroSystemVersion,
		NodeVersion:                 defaultKiroNodeVersion,
		CacheHitRateScale:           defaultKiroCacheHitRateScale,
		CacheMinBlockTokens:         defaultKiroCacheMinBlockTokens,
		CacheIndependentTTLSecs:     defaultKiroCacheIndependentTTL,
		CachePrefixTTLSecs:          defaultKiroCachePrefixTTL,
		CodeExecutionSandboxCommand: "",
	}
}

func parseKiroRuntimeSettingsMap(settings map[string]string) *KiroRuntimeSettings {
	result := DefaultKiroRuntimeSettings()
	if settings == nil {
		return result
	}
	result.KiroVersion = firstNonEmpty(strings.TrimSpace(settings[SettingKeyKiroDefaultVersion]), result.KiroVersion)
	result.KiroCommit = strings.TrimSpace(settings[SettingKeyKiroDefaultCommit])
	result.SystemVersion = firstNonEmpty(strings.TrimSpace(settings[SettingKeyKiroDefaultSystemVersion]), result.SystemVersion)
	result.NodeVersion = firstNonEmpty(strings.TrimSpace(settings[SettingKeyKiroDefaultNodeVersion]), result.NodeVersion)
	if v, err := strconv.Atoi(strings.TrimSpace(settings[SettingKeyKiroCacheHitRateScale])); err == nil {
		result.CacheHitRateScale = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(settings[SettingKeyKiroCacheMinBlockTokens])); err == nil {
		result.CacheMinBlockTokens = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(settings[SettingKeyKiroCacheIndependentTTLSeconds])); err == nil {
		result.CacheIndependentTTLSecs = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(settings[SettingKeyKiroCachePrefixTTLSeconds])); err == nil {
		result.CachePrefixTTLSecs = v
	}
	result.CodeExecutionSandboxCommand = strings.TrimSpace(settings[SettingKeyKiroCodeExecutionSandboxCommand])
	return normalizeKiroRuntimeSettings(result)
}

func normalizeKiroRuntimeSettings(settings *KiroRuntimeSettings) *KiroRuntimeSettings {
	if settings == nil {
		return DefaultKiroRuntimeSettings()
	}
	normalized := *settings
	settings = &normalized
	settings.KiroVersion = normalizeKiroHeaderValue(settings.KiroVersion, defaultKiroVersion)
	settings.KiroCommit = normalizeKiroOptionalHeaderValue(settings.KiroCommit)
	settings.SystemVersion = normalizeKiroHeaderValue(settings.SystemVersion, defaultKiroSystemVersion)
	settings.NodeVersion = normalizeKiroHeaderValue(settings.NodeVersion, defaultKiroNodeVersion)
	settings.CacheHitRateScale = clampInt(settings.CacheHitRateScale, 0, 100, defaultKiroCacheHitRateScale)
	settings.CacheMinBlockTokens = boundedIntOrDefault(settings.CacheMinBlockTokens, 0, KiroCacheMinBlockTokensMax, defaultKiroCacheMinBlockTokens)
	settings.CacheIndependentTTLSecs = clampInt(settings.CacheIndependentTTLSecs, 60, 86400, defaultKiroCacheIndependentTTL)
	settings.CachePrefixTTLSecs = clampInt(settings.CachePrefixTTLSecs, 60, 3600, defaultKiroCachePrefixTTL)
	if settings.CachePrefixTTLSecs > settings.CacheIndependentTTLSecs {
		settings.CachePrefixTTLSecs = settings.CacheIndependentTTLSecs
	}
	settings.CodeExecutionSandboxCommand = strings.TrimSpace(settings.CodeExecutionSandboxCommand)
	return settings
}

func normalizeKiroHeaderValue(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || !isSafeKiroHeaderValue(trimmed) {
		return fallback
	}
	return trimmed
}

func normalizeKiroOptionalHeaderValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || !isSafeKiroHeaderValue(trimmed) {
		return ""
	}
	return trimmed
}

func isSafeKiroHeaderValue(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] == 0x7f {
			return false
		}
	}
	return true
}

func clampInt(value, minValue, maxValue, defaultValue int) int {
	if value < minValue || value > maxValue {
		return defaultValue
	}
	return value
}

func (s *SettingService) GetKiroRuntimeSettings(ctx context.Context) *KiroRuntimeSettings {
	if s == nil || s.settingRepo == nil {
		return DefaultKiroRuntimeSettings()
	}
	if cached, ok := kiroRuntimeSettingsCache.Load().(*cachedKiroRuntimeSettings); ok {
		if cached != nil && cached.settings != nil && time.Now().UnixNano() < cached.expiresAt {
			cloned := *cached.settings
			return &cloned
		}
	}
	result, err, _ := kiroRuntimeSettingsSF.Do("kiro_runtime", func() (any, error) {
		if cached, ok := kiroRuntimeSettingsCache.Load().(*cachedKiroRuntimeSettings); ok {
			if cached != nil && cached.settings != nil && time.Now().UnixNano() < cached.expiresAt {
				cloned := *cached.settings
				return &cloned, nil
			}
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), kiroRuntimeSettingsDBTimeout)
		defer cancel()
		values, err := s.settingRepo.GetMultiple(dbCtx, []string{
			SettingKeyKiroDefaultVersion,
			SettingKeyKiroDefaultCommit,
			SettingKeyKiroDefaultSystemVersion,
			SettingKeyKiroDefaultNodeVersion,
			SettingKeyKiroCacheHitRateScale,
			SettingKeyKiroCacheMinBlockTokens,
			SettingKeyKiroCacheIndependentTTLSeconds,
			SettingKeyKiroCachePrefixTTLSeconds,
			SettingKeyKiroCodeExecutionSandboxCommand,
		})
		if err != nil {
			slog.Warn("failed to get kiro runtime settings, falling back to defaults", "error", err)
			settings := DefaultKiroRuntimeSettings()
			kiroRuntimeSettingsCache.Store(&cachedKiroRuntimeSettings{
				settings:  settings,
				expiresAt: time.Now().Add(kiroRuntimeSettingsErrorTTL).UnixNano(),
			})
			cloned := *settings
			return &cloned, nil
		}
		settings := parseKiroRuntimeSettingsMap(values)
		kiroRuntimeSettingsCache.Store(&cachedKiroRuntimeSettings{
			settings:  settings,
			expiresAt: time.Now().Add(kiroRuntimeSettingsCacheTTL).UnixNano(),
		})
		cloned := *settings
		return &cloned, nil
	})
	if err != nil {
		return DefaultKiroRuntimeSettings()
	}
	if settings, ok := result.(*KiroRuntimeSettings); ok && settings != nil {
		return settings
	}
	return DefaultKiroRuntimeSettings()
}

func validateKiroRuntimeSettingsForUpdate(settings *SystemSettings) error {
	if settings == nil {
		return nil
	}
	settings.KiroDefaultVersion = strings.TrimSpace(settings.KiroDefaultVersion)
	settings.KiroDefaultCommit = strings.TrimSpace(settings.KiroDefaultCommit)
	settings.KiroDefaultSystemVersion = strings.TrimSpace(settings.KiroDefaultSystemVersion)
	settings.KiroDefaultNodeVersion = strings.TrimSpace(settings.KiroDefaultNodeVersion)
	settings.KiroCodeExecutionSandboxCommand = strings.TrimSpace(settings.KiroCodeExecutionSandboxCommand)
	if !isSafeKiroHeaderValue(settings.KiroDefaultVersion) {
		return infraerrors.BadRequest("INVALID_KIRO_RUNTIME_SETTINGS", "Kiro version contains invalid header characters")
	}
	if !isSafeKiroHeaderValue(settings.KiroDefaultCommit) {
		return infraerrors.BadRequest("INVALID_KIRO_RUNTIME_SETTINGS", "Kiro commit contains invalid header characters")
	}
	if !isSafeKiroHeaderValue(settings.KiroDefaultSystemVersion) {
		return infraerrors.BadRequest("INVALID_KIRO_RUNTIME_SETTINGS", "Kiro system version contains invalid header characters")
	}
	if !isSafeKiroHeaderValue(settings.KiroDefaultNodeVersion) {
		return infraerrors.BadRequest("INVALID_KIRO_RUNTIME_SETTINGS", "Kiro node version contains invalid header characters")
	}
	if settings.KiroCacheHitRateScale < 0 || settings.KiroCacheHitRateScale > 100 {
		return infraerrors.BadRequest("INVALID_KIRO_RUNTIME_SETTINGS", "Kiro cache hit rate scale must be between 0 and 100")
	}
	if settings.KiroCacheMinBlockTokens < 0 || settings.KiroCacheMinBlockTokens > KiroCacheMinBlockTokensMax {
		return infraerrors.BadRequest(
			"INVALID_KIRO_RUNTIME_SETTINGS",
			fmt.Sprintf("Kiro cache min block tokens must be between 0 and %d", KiroCacheMinBlockTokensMax),
		)
	}
	if settings.KiroCacheIndependentTTLSeconds != 0 &&
		(settings.KiroCacheIndependentTTLSeconds < 60 || settings.KiroCacheIndependentTTLSeconds > 86400) {
		return infraerrors.BadRequest("INVALID_KIRO_RUNTIME_SETTINGS", "Kiro independent TTL must be between 60 and 86400 seconds")
	}
	if settings.KiroCachePrefixTTLSeconds != 0 &&
		(settings.KiroCachePrefixTTLSeconds < 60 || settings.KiroCachePrefixTTLSeconds > 3600) {
		return infraerrors.BadRequest("INVALID_KIRO_RUNTIME_SETTINGS", "Kiro prefix TTL must be between 60 and 3600 seconds")
	}
	if settings.KiroCacheIndependentTTLSeconds != 0 &&
		settings.KiroCachePrefixTTLSeconds != 0 &&
		settings.KiroCachePrefixTTLSeconds > settings.KiroCacheIndependentTTLSeconds {
		return infraerrors.BadRequest("INVALID_KIRO_RUNTIME_SETTINGS", "Kiro prefix TTL cannot exceed independent TTL")
	}
	return nil
}
