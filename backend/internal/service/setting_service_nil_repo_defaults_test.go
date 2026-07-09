//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGetOverloadCooldownSettings_NilRepoReturnsDefaults(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{})

	settings, err := svc.GetOverloadCooldownSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultOverloadCooldownSettings(), settings)
}

func TestGetRateLimit429CooldownSettings_NilRepoReturnsDefaults(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{})

	settings, err := svc.GetRateLimit429CooldownSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultRateLimit429CooldownSettings(), settings)
}

func TestGetStreamTimeoutSettings_NilRepoReturnsDefaults(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{})

	settings, err := svc.GetStreamTimeoutSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultStreamTimeoutSettings(), settings)
}

func TestGetBetaPolicySettings_NilRepoReturnsDefaults(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{})

	settings, err := svc.GetBetaPolicySettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultBetaPolicySettings(), settings)
}

func TestGetOpenAIFastPolicySettings_NilRepoReturnsDefaults(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{})

	settings, err := svc.GetOpenAIFastPolicySettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultOpenAIFastPolicySettings(), settings)
}

func TestGetTempUnschedThresholdSettings_NilRepoReturnsDefaults(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{})

	settings, err := svc.GetTempUnschedThresholdSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultTempUnschedThresholdSettings(), settings)
}
