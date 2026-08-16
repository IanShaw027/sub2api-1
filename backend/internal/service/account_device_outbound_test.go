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
