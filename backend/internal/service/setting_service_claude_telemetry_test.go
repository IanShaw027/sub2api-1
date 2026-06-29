package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSettingServiceClaudeTelemetryModeDefaultsAndNormalizes(t *testing.T) {
	repo := &kiroRuntimeSettingRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	require.Equal(t, ClaudeTelemetryModeDrop, svc.GetClaudeTelemetryMode(context.Background()))

	repo.values[SettingKeyClaudeTelemetryMode] = " forward "
	gatewayForwardingSF.Forget("gateway_forwarding")
	gatewayForwardingCache.Store((*cachedGatewayForwardingSettings)(nil))
	require.Equal(t, ClaudeTelemetryModeForward, svc.GetClaudeTelemetryMode(context.Background()))

	repo.values[SettingKeyClaudeTelemetryMode] = "invalid"
	gatewayForwardingSF.Forget("gateway_forwarding")
	gatewayForwardingCache.Store((*cachedGatewayForwardingSettings)(nil))
	require.Equal(t, ClaudeTelemetryModeDrop, svc.GetClaudeTelemetryMode(context.Background()))
}

func TestSettingServiceUpdateSettingsWritesClaudeTelemetryMode(t *testing.T) {
	repo := &kiroRuntimeSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{ClaudeTelemetryMode: " forward "})
	require.NoError(t, err)
	require.Equal(t, ClaudeTelemetryModeForward, repo.updates[SettingKeyClaudeTelemetryMode])
	require.Equal(t, ClaudeTelemetryModeForward, svc.GetClaudeTelemetryMode(context.Background()))

	err = svc.UpdateSettings(context.Background(), &SystemSettings{ClaudeTelemetryMode: "bad"})
	require.NoError(t, err)
	require.Equal(t, ClaudeTelemetryModeDrop, repo.updates[SettingKeyClaudeTelemetryMode])
}
