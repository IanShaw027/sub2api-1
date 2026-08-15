//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
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

func TestApplyKiroSidecarIdentity_PrefersProfileClientVersion(t *testing.T) {
	profile := validKiroOutboundProfile(31, kiroPinnedOutboundMachineID)
	profile.ClientVersion = "0.11.0"
	installKiroOutboundDeviceProfile(t, profile, nil)

	settings, machineID, err := applyKiroSidecarIdentity(context.Background(), &Account{
		ID:       31,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, &KiroRuntimeSettings{KiroVersion: "0.10.0"})
	require.NoError(t, err)
	require.Equal(t, "0.11.0", settings.KiroVersion)
	require.Equal(t, kiroPinnedOutboundMachineID, machineID)
}

func TestApplyKiroSidecarIdentity_StashMismatchLoadsRequestedAccount(t *testing.T) {
	const machineB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	profileA := validKiroOutboundProfile(41, kiroPinnedOutboundMachineID)
	profileA.ClientVersion = "0.11.0"
	profileB := validKiroOutboundProfile(42, machineB)
	profileB.ClientVersion = "0.12.0"
	installLeftoverOutboundProfile(t, profileA)
	installLeftoverOutboundProfile(t, profileB)

	stashed := applyKiroProfileRuntimeOverrides(&KiroRuntimeSettings{KiroVersion: "0.10.0"}, profileA)
	ctx := withKiroSidecarIdentity(context.Background(), 41, stashed, kiroPinnedOutboundMachineID)

	settings, machineID, err := applyKiroSidecarIdentity(ctx, &Account{
		ID:       42,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, &KiroRuntimeSettings{KiroVersion: "0.10.0"})
	require.NoError(t, err)
	require.Equal(t, "0.12.0", settings.KiroVersion)
	require.Equal(t, machineB, machineID)
	require.NotEqual(t, kiroPinnedOutboundMachineID, machineID)
	require.NotEqual(t, "0.11.0", settings.KiroVersion)
}

func TestApplyKiroSidecarIdentity_LoadFailureIsFailClosed(t *testing.T) {
	installKiroOutboundDeviceProfile(t, nil, errors.New("identity_reject: device profile unavailable"))

	settings, machineID, err := applyKiroSidecarIdentity(context.Background(), &Account{
		ID:       36,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, &KiroRuntimeSettings{KiroVersion: "0.10.0"})
	require.Error(t, err)
	require.Nil(t, settings)
	require.Empty(t, machineID)
	require.Contains(t, err.Error(), "identity_reject")
}

func TestKiroUsageSetOveragePreference_UsesProfileClientVersionNotSettings(t *testing.T) {
	profile := validKiroOutboundProfile(33, kiroPinnedOutboundMachineID)
	profile.ClientVersion = "0.11.0"
	installKiroOutboundDeviceProfile(t, profile, nil)

	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		},
	}
	svc := NewKiroUsageService().WithTransport(upstream, nil)
	err := svc.SetOveragePreference(context.Background(), &Account{
		ID:       33,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"region": "us-east-1",
		},
	}, "access", true)
	require.NoError(t, err)
	require.NotNil(t, upstream.req)
	require.Contains(t, upstream.req.Header.Get("User-Agent"), "KiroIDE-0.11.0-")
	require.Contains(t, upstream.req.Header.Get("x-amz-user-agent"), "KiroIDE-0.11.0-")
	require.NotContains(t, upstream.req.Header.Get("User-Agent"), "KiroIDE-0.10.0-")
	require.NotContains(t, upstream.req.Header.Get("x-amz-user-agent"), "KiroIDE-0.10.0-")
}

func TestKiroTokenRefresher_UsesProfileClientVersionNotSettings(t *testing.T) {
	profile := validKiroOutboundProfile(34, kiroPinnedOutboundMachineID)
	profile.ClientVersion = "0.11.0"
	installKiroOutboundDeviceProfile(t, profile, nil)

	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"accessToken":"new-access"}`)),
		},
	}
	refresher := NewKiroTokenRefresher().WithTransport(upstream, nil)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken", nil)
	require.NoError(t, err)
	var out kiroRefreshResponse
	err = refresher.doKiroRequest(req, &Account{
		ID:       34,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, "prod.us-east-1.auth.desktop.kiro.dev", &out, kiroPinnedOutboundMachineID)
	require.NoError(t, err)
	require.NotNil(t, upstream.req)
	require.Equal(t, "KiroIDE-0.11.0-"+kiroPinnedOutboundMachineID, upstream.req.Header.Get("User-Agent"))
	require.NotContains(t, upstream.req.Header.Get("User-Agent"), "KiroIDE-0.10.0-")
}

func TestKiroTokenRefresherOIDC_UsesProfileClientVersionNotSettings(t *testing.T) {
	profile := validKiroOutboundProfile(35, kiroPinnedOutboundMachineID)
	profile.ClientVersion = "0.11.0"
	installKiroOutboundDeviceProfile(t, profile, nil)

	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"accessToken":"new-access"}`)),
		},
	}
	refresher := NewKiroTokenRefresher().WithTransport(upstream, nil)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://oidc.us-east-1.amazonaws.com/token", nil)
	require.NoError(t, err)
	var out kiroRefreshResponse
	err = refresher.doKiroRequest(req, &Account{
		ID:       35,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, "oidc.us-east-1.amazonaws.com", &out, kiroPinnedOutboundMachineID)
	require.NoError(t, err)
	require.NotNil(t, upstream.req)
	require.Contains(t, upstream.req.Header.Get("User-Agent"), "KiroIDE-0.11.0-")
	require.Contains(t, upstream.req.Header.Get("x-amz-user-agent"), "KiroIDE-0.11.0-")
	require.NotContains(t, upstream.req.Header.Get("User-Agent"), "KiroIDE-0.10.0-")
	require.NotContains(t, upstream.req.Header.Get("x-amz-user-agent"), "KiroIDE-0.10.0-")
}
