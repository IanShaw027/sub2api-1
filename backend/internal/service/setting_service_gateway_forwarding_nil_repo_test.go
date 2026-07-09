//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSettingService_GetGatewayForwardingSettings_NilRepoUsesSafeDefaults(t *testing.T) {
	resetGatewayForwardingSettingsCacheForTest(t)
	svc := NewSettingService(nil, &config.Config{})

	fp, mp := svc.GetGatewayForwardingSettings(context.Background())

	require.True(t, fp)
	require.False(t, mp)
}

func TestSettingService_IsAnthropicCacheTTL1hInjectionEnabled_NilRepoDefaultsFalse(t *testing.T) {
	resetGatewayForwardingSettingsCacheForTest(t)
	svc := NewSettingService(nil, &config.Config{})

	require.False(t, svc.IsAnthropicCacheTTL1hInjectionEnabled(context.Background()))
}
