//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadOutboundDeviceProfileRequiresService(t *testing.T) {
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(nil)
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	p, err := LoadOutboundDeviceProfile(context.Background(), &Account{ID: 1, Platform: PlatformAnthropic})
	require.Error(t, err)
	require.Nil(t, p)
	require.ErrorContains(t, err, "identity_reject")
}

func TestLoadOutboundDeviceProfile_UsesContextCache(t *testing.T) {
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(nil)
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	account := &Account{ID: 7, Platform: PlatformOpenAI}
	profile := leftoverValidProfile(7, PlatformOpenAI, DefaultClientFamily(PlatformOpenAI), "codex-cli/1.2.3", "mid-cache")
	require.NoError(t, ValidateOutboundBundle(profile))

	got, err := LoadOutboundDeviceProfile(withOutboundDeviceProfile(context.Background(), profile), account)
	require.NoError(t, err)
	require.Equal(t, profile.InstallationID, got.InstallationID)
	require.Equal(t, profile.AccountID, got.AccountID)
}
