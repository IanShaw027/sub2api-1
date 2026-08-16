package dto

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestAccountFromServiceShallow_OmitsComputedRPMStickyBuffer(t *testing.T) {
	t.Parallel()

	src := &service.Account{
		Platform:    service.PlatformAnthropic,
		Type:        service.AccountTypeOAuth,
		Concurrency: 3,
		Extra: map[string]any{
			"base_rpm":     15,
			"max_sessions": 10,
		},
	}

	got := AccountFromServiceShallow(src)
	require.NotNil(t, got)
	require.NotNil(t, got.BaseRPM)
	require.Equal(t, 15, *got.BaseRPM)
	require.Nil(t, got.RPMStickyBuffer, "computed buffer must not be persisted as a manual override")
}

func TestAccountFromServiceShallow_ExposesManualRPMStickyBuffer(t *testing.T) {
	t.Parallel()

	src := &service.Account{
		Platform:    service.PlatformAnthropic,
		Type:        service.AccountTypeOAuth,
		Concurrency: 3,
		Extra: map[string]any{
			"base_rpm":          15,
			"rpm_sticky_buffer": 5,
		},
	}

	got := AccountFromServiceShallow(src)
	require.NotNil(t, got)
	require.NotNil(t, got.RPMStickyBuffer)
	require.Equal(t, 5, *got.RPMStickyBuffer)
}

func TestAccountFromServiceShallow_OmitsOutOfRangeManualRPMStickyBuffer(t *testing.T) {
	t.Parallel()

	src := &service.Account{
		Platform:    service.PlatformAnthropic,
		Type:        service.AccountTypeOAuth,
		Concurrency: 3,
		Extra: map[string]any{
			"base_rpm":          15,
			"rpm_sticky_buffer": 10001,
		},
	}

	got := AccountFromServiceShallow(src)
	require.NotNil(t, got)
	require.Nil(t, got.RPMStickyBuffer)
}
