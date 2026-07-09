//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettingService_ParseSettings_NilConfigUsesZeroConfigDefaults(t *testing.T) {
	svc := NewSettingService(nil, nil)

	got := svc.parseSettings(map[string]string{})

	require.Equal(t, "Sub2API", got.SiteName)
	require.Equal(t, 0, got.DefaultConcurrency)
	require.InDelta(t, 0, got.DefaultBalance, 1e-12)
	require.False(t, got.LinuxDoConnectEnabled)
	require.False(t, got.DingTalkConnectEnabled)
	require.False(t, got.OIDCConnectEnabled)
}

func TestSettingService_GetAllSettings_NilConfigDoesNotPanic(t *testing.T) {
	repo := &kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeySiteName: "custom-site",
		},
	}
	svc := NewSettingService(repo, nil)

	got, err := svc.GetAllSettings(context.Background())

	require.NoError(t, err)
	require.Equal(t, "custom-site", got.SiteName)
	require.Equal(t, 0, got.DefaultConcurrency)
	require.InDelta(t, 0, got.DefaultBalance, 1e-12)
}
