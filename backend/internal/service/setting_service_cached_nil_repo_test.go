//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func resetKiroRuntimeSettingsTestCache(t *testing.T) {
	t.Helper()

	kiroRuntimeSettingsCache.Store((*cachedKiroRuntimeSettings)(nil))
	kiroRuntimeSettingsSF.Forget("kiro_runtime")
	t.Cleanup(func() {
		kiroRuntimeSettingsCache.Store((*cachedKiroRuntimeSettings)(nil))
		kiroRuntimeSettingsSF.Forget("kiro_runtime")
	})
}

func resetAccountSchedulingThresholdsTestCache(t *testing.T) {
	t.Helper()

	accountSchedulingThresholdsCache.Store((*cachedAccountSchedulingThresholds)(nil))
	accountSchedulingThresholdsSF.Forget(SettingKeyAccountSchedulingThresholds)
	t.Cleanup(func() {
		accountSchedulingThresholdsCache.Store((*cachedAccountSchedulingThresholds)(nil))
		accountSchedulingThresholdsSF.Forget(SettingKeyAccountSchedulingThresholds)
	})
}

func resetVersionBoundsTestCache(t *testing.T) {
	t.Helper()

	versionBoundsCache.Store((*cachedVersionBounds)(nil))
	versionBoundsSF.Forget("version_bounds")
	t.Cleanup(func() {
		versionBoundsCache.Store((*cachedVersionBounds)(nil))
		versionBoundsSF.Forget("version_bounds")
	})
}

func TestGetKiroRuntimeSettings_NilRepoReturnsDefaults(t *testing.T) {
	resetKiroRuntimeSettingsTestCache(t)
	svc := NewSettingService(nil, &config.Config{})

	got := svc.GetKiroRuntimeSettings(context.Background())

	require.Equal(t, DefaultKiroRuntimeSettings(), got)
}

func TestGetAccountSchedulingThresholds_NilRepoReturnsDefaults(t *testing.T) {
	resetAccountSchedulingThresholdsTestCache(t)
	svc := NewSettingService(nil, &config.Config{})

	got := svc.GetAccountSchedulingThresholds(context.Background())

	require.Equal(t, defaultAccountSchedulingThresholds(), got)
}

func TestGetClaudeCodeVersionBounds_NilRepoReturnsOpenBounds(t *testing.T) {
	resetVersionBoundsTestCache(t)
	svc := NewSettingService(nil, &config.Config{})

	min, max := svc.GetClaudeCodeVersionBounds(context.Background())

	require.Empty(t, min)
	require.Empty(t, max)
}
