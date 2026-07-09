//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestIsRegistrationEnabled_NilRepoReturnsFalse(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{})

	require.False(t, svc.IsRegistrationEnabled(context.Background()))
}

func TestGetChannelMonitorRuntime_NilRepoUsesFailOpenDefaults(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{})

	runtime := svc.GetChannelMonitorRuntime(context.Background())

	require.True(t, runtime.Enabled)
	require.Equal(t, channelMonitorIntervalFallback, runtime.DefaultIntervalSeconds)
}

func TestGetAvailableChannelsRuntime_NilRepoReturnsDisabled(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{})

	runtime := svc.GetAvailableChannelsRuntime(context.Background())

	require.False(t, runtime.Enabled)
}

func TestIsUserErrorViewAllowed_NilRepoReturnsFalse(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{})

	require.False(t, svc.IsUserErrorViewAllowed(context.Background()))
}
