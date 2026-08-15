//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyKiroProfileRuntimeOverrides_PrefersProfileClientVersion(t *testing.T) {
	settings := &KiroRuntimeSettings{KiroVersion: "0.10.0"}
	profile := validKiroOutboundProfile(1, kiroPinnedOutboundMachineID)
	profile.ClientVersion = "0.11.0"

	got := applyKiroProfileRuntimeOverrides(settings, profile)
	require.Equal(t, "0.11.0", got.KiroVersion)
	require.Equal(t, "0.10.0", settings.KiroVersion)
}

func TestApplyKiroProfileRuntimeOverrides_EmptyProfileVersionKeepsSettings(t *testing.T) {
	settings := &KiroRuntimeSettings{KiroVersion: "0.10.0"}
	profile := validKiroOutboundProfile(1, kiroPinnedOutboundMachineID)
	profile.ClientVersion = "  "

	got := applyKiroProfileRuntimeOverrides(settings, profile)
	require.Equal(t, "0.10.0", got.KiroVersion)
}

func TestApplyKiroProfileRuntimeOverrides_NilProfileKeepsNormalizedSettings(t *testing.T) {
	settings := &KiroRuntimeSettings{KiroVersion: "0.10.0"}

	got := applyKiroProfileRuntimeOverrides(settings, nil)
	require.Equal(t, "0.10.0", got.KiroVersion)
	require.Equal(t, defaultKiroSystemVersion, got.SystemVersion)
	require.Equal(t, defaultKiroNodeVersion, got.NodeVersion)
}

func TestBuildKiroGenerateAssistantRequest_UsesProfileClientVersionNotSettings(t *testing.T) {
	profile := validKiroOutboundProfile(15, kiroPinnedOutboundMachineID)
	profile.ClientVersion = "0.11.0"
	installKiroOutboundDeviceProfile(t, profile, nil)

	req, err := buildKiroGenerateAssistantRequest(context.Background(), &Account{
		ID:       15,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, []byte(`{}`), "access", &KiroRuntimeSettings{
		KiroVersion:   "0.10.0",
		SystemVersion: defaultKiroSystemVersion,
		NodeVersion:   defaultKiroNodeVersion,
	})
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Contains(t, req.Header.Get("User-Agent"), "KiroIDE-0.11.0-")
	require.Contains(t, req.Header.Get("x-amz-user-agent"), "KiroIDE-0.11.0-")
	require.NotContains(t, req.Header.Get("User-Agent"), "KiroIDE-0.10.0-")
	require.NotContains(t, req.Header.Get("x-amz-user-agent"), "KiroIDE-0.10.0-")
	require.Contains(t, req.Header.Get("User-Agent"), kiroPinnedOutboundMachineID)
	require.Contains(t, req.Header.Get("x-amz-user-agent"), kiroPinnedOutboundMachineID)
}
