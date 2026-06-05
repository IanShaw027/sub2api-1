//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func newSettingServiceForPlatformThresholdTest(seed map[string]string) *SettingService {
	accountSchedulingThresholdsSF.Forget(SettingKeyAccountSchedulingThresholds)
	accountSchedulingThresholdsCache.Store(&cachedAccountSchedulingThresholds{})
	repo := newMockSettingRepo()
	for k, v := range seed {
		repo.data[k] = v
	}
	return NewSettingService(repo, &config.Config{})
}

func TestPlatformSchedulingThresholds_RoundTrip_DefaultsAndStoredValues(t *testing.T) {
	svc := newSettingServiceForPlatformThresholdTest(nil)

	got := svc.parseSettings(map[string]string{})
	require.Equal(t, map[string]int{
		PlatformOpenAI:      100,
		PlatformAnthropic:   100,
		PlatformGemini:      100,
		PlatformKiro:        100,
		PlatformAntigravity: 100,
	}, got.AccountSchedulingThresholds)

	got = svc.parseSettings(map[string]string{
		SettingKeyAccountSchedulingThresholds: `{"openai":91,"gemini":85,"kiro":99}`,
	})
	require.Equal(t, 91, got.AccountSchedulingThresholds[PlatformOpenAI])
	require.Equal(t, 85, got.AccountSchedulingThresholds[PlatformGemini])
	require.Equal(t, 99, got.AccountSchedulingThresholds[PlatformKiro])
	require.Equal(t, 100, got.AccountSchedulingThresholds[PlatformAnthropic])
}

func TestBuildSystemSettingsUpdates_PersistsAccountSchedulingThresholds(t *testing.T) {
	svc := newSettingServiceForPlatformThresholdTest(nil)

	updates, err := svc.buildSystemSettingsUpdates(context.Background(), &SystemSettings{
		AccountSchedulingThresholds: map[string]int{
			PlatformOpenAI:      91,
			PlatformAnthropic:   88,
			PlatformGemini:      87,
			PlatformKiro:        95,
			PlatformAntigravity: 92,
		},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"openai":91,"anthropic":88,"gemini":87,"kiro":95,"antigravity":92}`, updates[SettingKeyAccountSchedulingThresholds])
}

func TestValidateAndNormalizeAccountSchedulingThresholds_FillsMissingPlatforms(t *testing.T) {
	normalized, err := validateAndNormalizeAccountSchedulingThresholds(map[string]int{
		PlatformOpenAI: 91,
		PlatformGemini: 85,
	})
	require.NoError(t, err)
	require.Equal(t, 91, normalized[PlatformOpenAI])
	require.Equal(t, 85, normalized[PlatformGemini])
	require.Equal(t, 100, normalized[PlatformAnthropic])
	require.Equal(t, 100, normalized[PlatformKiro])
	require.Equal(t, 100, normalized[PlatformAntigravity])
}

func TestUpdateSettings_StoresAccountSchedulingThresholds(t *testing.T) {
	svc := newSettingServiceForPlatformThresholdTest(nil)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		AccountSchedulingThresholds: map[string]int{
			PlatformOpenAI:      92,
			PlatformAnthropic:   89,
			PlatformGemini:      87,
			PlatformKiro:        96,
			PlatformAntigravity: 93,
		},
	})
	require.NoError(t, err)

	got := svc.parseSettings(map[string]string{
		SettingKeyAccountSchedulingThresholds: svc.settingRepo.(*mockSettingRepo).data[SettingKeyAccountSchedulingThresholds],
	})
	require.Equal(t, 92, got.AccountSchedulingThresholds[PlatformOpenAI])
	require.Equal(t, 96, got.AccountSchedulingThresholds[PlatformKiro])
}

func TestGetAccountSchedulingThresholds_ReadsStoredValue(t *testing.T) {
	svc := newSettingServiceForPlatformThresholdTest(map[string]string{
		SettingKeyAccountSchedulingThresholds: `{"openai":93,"kiro":88}`,
	})

	got := svc.GetAccountSchedulingThresholds(context.Background())

	require.Equal(t, 93, got[PlatformOpenAI])
	require.Equal(t, 88, got[PlatformKiro])
	require.Equal(t, 100, got[PlatformAnthropic])
}
