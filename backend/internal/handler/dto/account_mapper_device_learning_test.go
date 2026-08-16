package dto

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestAccountFromServiceShallow_MapsDeviceLearningEnabledTrue(t *testing.T) {
	t.Parallel()

	got := AccountFromServiceShallow(&service.Account{
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeOAuth,
		Extra: map[string]any{
			"device_learning_enabled": true,
		},
	})

	require.NotNil(t, got)
	require.NotNil(t, got.DeviceLearningEnabled)
	require.True(t, *got.DeviceLearningEnabled)
}

func TestAccountFromServiceShallow_OmitsAbsentDeviceLearningEnabled(t *testing.T) {
	t.Parallel()

	got := AccountFromServiceShallow(&service.Account{
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeOAuth,
		Extra:    map[string]any{},
	})

	require.NotNil(t, got)
	require.Nil(t, got.DeviceLearningEnabled)
}

func TestAccountFromServiceShallow_OmitsExplicitFalseDeviceLearningEnabled(t *testing.T) {
	t.Parallel()

	got := AccountFromServiceShallow(&service.Account{
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeOAuth,
		Extra: map[string]any{
			"device_learning_enabled": false,
		},
	})

	require.NotNil(t, got)
	require.Nil(t, got.DeviceLearningEnabled)
}
