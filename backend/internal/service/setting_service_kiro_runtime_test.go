package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type kiroRuntimeSettingRepoStub struct {
	values        map[string]string
	updates       map[string]string
	errs          map[string]error
	getValueCalls map[string]int
}

func (s *kiroRuntimeSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *kiroRuntimeSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s.getValueCalls == nil {
		s.getValueCalls = map[string]int{}
	}
	s.getValueCalls[key]++
	if s.values == nil {
		return "", ErrSettingNotFound
	}
	if err := s.errs[key]; err != nil {
		return "", err
	}
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (s *kiroRuntimeSettingRepoStub) Set(ctx context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *kiroRuntimeSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if s.values != nil {
			result[key] = s.values[key]
		}
	}
	return result, nil
}

func (s *kiroRuntimeSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	s.updates = make(map[string]string, len(settings))
	if s.values == nil {
		s.values = map[string]string{}
	}
	for key, value := range settings {
		s.updates[key] = value
		s.values[key] = value
	}
	return nil
}

func (s *kiroRuntimeSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	if s.values == nil {
		return map[string]string{}, nil
	}
	result := make(map[string]string, len(s.values))
	for key, value := range s.values {
		result[key] = value
	}
	return result, nil
}

func (s *kiroRuntimeSettingRepoStub) Delete(ctx context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func TestSettingService_UpdateSettings_WritesKiroRuntimeDefaults(t *testing.T) {
	kiroRuntimeSettingsCache.Store((*cachedKiroRuntimeSettings)(nil))
	kiroRuntimeSettingsSF.Forget("kiro_runtime")
	repo := &kiroRuntimeSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		KiroDefaultVersion:              "0.11.0",
		KiroDefaultCommit:               "commit-123",
		KiroDefaultSystemVersion:        "linux#6.8.0",
		KiroDefaultNodeVersion:          "22.22.0",
		KiroCacheHitRateScale:           88,
		KiroCacheMinBlockTokens:         2048,
		KiroCacheIndependentTTLSeconds:  7200,
		KiroCachePrefixTTLSeconds:       600,
		KiroCodeExecutionSandboxCommand: "  sandbox-run --kiro  ",
	})
	require.NoError(t, err)
	require.Equal(t, "0.11.0", repo.updates[SettingKeyKiroDefaultVersion])
	require.Equal(t, "commit-123", repo.updates[SettingKeyKiroDefaultCommit])
	require.Equal(t, "linux#6.8.0", repo.updates[SettingKeyKiroDefaultSystemVersion])
	require.Equal(t, "22.22.0", repo.updates[SettingKeyKiroDefaultNodeVersion])
	require.Equal(t, "88", repo.updates[SettingKeyKiroCacheHitRateScale])
	require.Equal(t, "2048", repo.updates[SettingKeyKiroCacheMinBlockTokens])
	require.Equal(t, "7200", repo.updates[SettingKeyKiroCacheIndependentTTLSeconds])
	require.Equal(t, "600", repo.updates[SettingKeyKiroCachePrefixTTLSeconds])
	require.Equal(t, "sandbox-run --kiro", repo.updates[SettingKeyKiroCodeExecutionSandboxCommand])

	got := svc.GetKiroRuntimeSettings(context.Background())
	require.Equal(t, "0.11.0", got.KiroVersion)
	require.Equal(t, "commit-123", got.KiroCommit)
	require.Equal(t, "linux#6.8.0", got.SystemVersion)
	require.Equal(t, "22.22.0", got.NodeVersion)
	require.Equal(t, 88, got.CacheHitRateScale)
	require.Equal(t, 2048, got.CacheMinBlockTokens)
	require.Equal(t, 7200, got.CacheIndependentTTLSecs)
	require.Equal(t, 600, got.CachePrefixTTLSecs)
	require.Equal(t, "sandbox-run --kiro", got.CodeExecutionSandboxCommand)
}

func TestSettingService_GetAllSettings_ReturnsKiroCodeExecutionSandboxCommand(t *testing.T) {
	repo := &kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyKiroCodeExecutionSandboxCommand: "  sandbox-from-db  ",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	got, err := svc.GetAllSettings(context.Background())

	require.NoError(t, err)
	require.Equal(t, "sandbox-from-db", got.KiroCodeExecutionSandboxCommand)
}

func TestSettingService_GetAllSettings_NilRepoReturnsDefaults(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{})

	got, err := svc.GetAllSettings(context.Background())

	require.NoError(t, err)
	require.Equal(t, "Sub2API", got.SiteName)
	require.True(t, got.PromoCodeEnabled)
}

func TestSettingService_GetAllSettingsAndUpdateSettings_PreserveGatewayDebugTimelineBodySettings(t *testing.T) {
	repo := &kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyGatewayDebugTimelineEnabled:       "true",
			SettingKeyGatewayDebugTimelineDirectory:     "logs/gateway-debug",
			SettingKeyGatewayDebugTimelineRetentionDays: "7",
			SettingKeyGatewayDebugTimelineMaxSizeMB:     "1024",
			SettingKeyGatewayDebugTimelineIncludeBody:   "true",
			SettingKeyGatewayDebugTimelineBodyMaxKB:     "256",
			SettingKeySiteName:                          "before",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.GatewayDebugTimelineIncludeBody)
	require.Equal(t, 256, settings.GatewayDebugTimelineBodyMaxKB)

	settings.SiteName = "after"
	err = svc.UpdateSettings(context.Background(), settings)
	require.NoError(t, err)
	require.Equal(t, "true", repo.updates[SettingKeyGatewayDebugTimelineIncludeBody])
	require.Equal(t, "256", repo.updates[SettingKeyGatewayDebugTimelineBodyMaxKB])
}

func TestDefaultKiroRuntimeSettings_UsesOneHourPrefixTTL(t *testing.T) {
	got := DefaultKiroRuntimeSettings()
	require.Equal(t, 3600, got.CachePrefixTTLSecs)
}

func TestSettingService_UpdateSettings_RejectsKiroCacheMinBlockTokensAboveMax(t *testing.T) {
	repo := &kiroRuntimeSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		KiroCacheMinBlockTokens: KiroCacheMinBlockTokensMax + 1,
	})
	require.Error(t, err)
	require.Nil(t, repo.updates)
}

func TestSettingService_UpdateSettings_RejectsInvalidKiroRuntimeRanges(t *testing.T) {
	repo := &kiroRuntimeSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		KiroCacheHitRateScale:          101,
		KiroCacheIndependentTTLSeconds: 30,
		KiroCachePrefixTTLSeconds:      3601,
	})
	require.Error(t, err)
	require.Nil(t, repo.updates)
}

func TestSettingService_UpdateSettings_RejectsUnsafeKiroHeaderValues(t *testing.T) {
	repo := &kiroRuntimeSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		KiroDefaultVersion: "0.12.0\r\nbad",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid header characters")
	require.Nil(t, repo.updates)
}

func TestSettingService_GetKiroRuntimeSettings_NormalizesInvalidValues(t *testing.T) {
	repo := &kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyKiroDefaultVersion:             "bad\r\nversion",
			SettingKeyKiroDefaultCommit:              "  abcdef\u007f  ",
			SettingKeyKiroDefaultSystemVersion:       "bad\nsystem",
			SettingKeyKiroDefaultNodeVersion:         "bad\tnode",
			SettingKeyKiroCacheHitRateScale:          "999",
			SettingKeyKiroCacheMinBlockTokens:        "1048577",
			SettingKeyKiroCacheIndependentTTLSeconds: "30",
			SettingKeyKiroCachePrefixTTLSeconds:      "9999",
		},
	}
	kiroRuntimeSettingsCache.Store((*cachedKiroRuntimeSettings)(nil))
	kiroRuntimeSettingsSF.Forget("kiro_runtime")
	svc := NewSettingService(repo, &config.Config{})

	got := svc.GetKiroRuntimeSettings(context.Background())
	require.Equal(t, defaultKiroVersion, got.KiroVersion)
	require.Equal(t, "", got.KiroCommit)
	require.Equal(t, defaultKiroSystemVersion, got.SystemVersion)
	require.Equal(t, defaultKiroNodeVersion, got.NodeVersion)
	require.Equal(t, defaultKiroCacheHitRateScale, got.CacheHitRateScale)
	require.Equal(t, defaultKiroCacheMinBlockTokens, got.CacheMinBlockTokens)
	require.Equal(t, defaultKiroCacheIndependentTTL, got.CacheIndependentTTLSecs)
	require.Equal(t, defaultKiroCachePrefixTTL, got.CachePrefixTTLSecs)
}

func TestSettingService_GetKiroRuntimeSettings_PreservesZeroHitRateScale(t *testing.T) {
	repo := &kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyKiroCacheHitRateScale: "0",
		},
	}
	kiroRuntimeSettingsCache.Store((*cachedKiroRuntimeSettings)(nil))
	kiroRuntimeSettingsSF.Forget("kiro_runtime")
	svc := NewSettingService(repo, &config.Config{})

	got := svc.GetKiroRuntimeSettings(context.Background())
	require.Equal(t, 0, got.CacheHitRateScale)
}

func TestSettingService_GetKiroRuntimeSettings_DoesNotExposeLegacyThinkingFields(t *testing.T) {
	repo := &kiroRuntimeSettingRepoStub{
		values: map[string]string{
			"kiro_thinking_mode":                "model_and_simulate",
			"kiro_thinking_effort_threshold":    "max",
			"kiro_thinking_simulation_template": "legacy template",
			"kiro_thinking_free_prompt":         "legacy prompt",
		},
	}
	kiroRuntimeSettingsCache.Store((*cachedKiroRuntimeSettings)(nil))
	kiroRuntimeSettingsSF.Forget("kiro_runtime")
	svc := NewSettingService(repo, &config.Config{})

	got := svc.GetKiroRuntimeSettings(context.Background())
	raw, err := json.Marshal(got)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.NotContains(t, payload, "thinking_mode")
	require.NotContains(t, payload, "thinking_effort_threshold")
	require.NotContains(t, payload, "thinking_simulation_template")
	require.NotContains(t, payload, "thinking_free_prompt")
}
