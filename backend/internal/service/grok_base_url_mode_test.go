package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestNormalizeGrokDefaultBaseURLMode(t *testing.T) {
	t.Parallel()
	require.Equal(t, GrokDefaultBaseURLModeCLI, normalizeGrokDefaultBaseURLMode(""))
	require.Equal(t, GrokDefaultBaseURLModeAPI, normalizeGrokDefaultBaseURLMode("api"))
	require.Equal(t, GrokDefaultBaseURLModeAPI, normalizeGrokDefaultBaseURLMode("API"))
	require.Equal(t, GrokDefaultBaseURLModeUSEast1, normalizeGrokDefaultBaseURLMode("us-east-1"))
	require.Equal(t, GrokDefaultBaseURLModeUSWest2, normalizeGrokDefaultBaseURLMode("US-WEST-2"))
	require.Equal(t, GrokDefaultBaseURLModeEUWest1, normalizeGrokDefaultBaseURLMode(" eu-west-1 "))
	require.Equal(t, GrokDefaultBaseURLModeCLI, normalizeGrokDefaultBaseURLMode("cli"))
	require.Equal(t, GrokDefaultBaseURLModeCLI, normalizeGrokDefaultBaseURLMode(" CLI "))
	require.Equal(t, GrokDefaultBaseURLModeCLI, normalizeGrokDefaultBaseURLMode("unknown"))
}

func TestGrokBaseURLForMode(t *testing.T) {
	t.Parallel()
	require.Equal(t, xai.DefaultBaseURL, GrokBaseURLForMode("api"))
	require.Equal(t, xai.DefaultUSEast1BaseURL, GrokBaseURLForMode("us-east-1"))
	require.Equal(t, xai.DefaultUSWest2BaseURL, GrokBaseURLForMode("us-west-2"))
	require.Equal(t, xai.DefaultEUWest1BaseURL, GrokBaseURLForMode("eu-west-1"))
	require.Equal(t, xai.DefaultCLIBaseURL, GrokBaseURLForMode("cli"))
}

func TestAccountGetGrokBaseURLOr(t *testing.T) {
	t.Parallel()
	acc := &Account{
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{},
	}
	require.Equal(t, xai.DefaultCLIBaseURL, acc.GetGrokBaseURLOr(xai.DefaultCLIBaseURL))
	require.Equal(t, xai.DefaultCLIBaseURL, acc.GetGrokBaseURLOr(""))

	acc.Credentials["base_url"] = "https://api.x.ai/v1"
	require.Equal(t, xai.DefaultCLIBaseURL, acc.GetGrokBaseURLOr(xai.DefaultCLIBaseURL))
	require.Equal(t, xai.DefaultBaseURL, acc.GetGrokBaseURLOr(xai.DefaultBaseURL))
}

func TestSettingServiceResolveGrokBaseURL(t *testing.T) {
	t.Parallel()
	repo := &openAISettingRepoStub{values: map[string]string{
		SettingKeyGrokDefaultBaseURLMode: GrokDefaultBaseURLModeCLI,
	}}
	svc := NewSettingService(repo, nil)
	acc := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{}}
	require.Equal(t, xai.DefaultCLIBaseURL, svc.ResolveGrokBaseURL(context.Background(), acc))

	acc.Credentials["base_url"] = xai.DefaultBaseURL
	require.Equal(t, xai.DefaultCLIBaseURL, svc.ResolveGrokBaseURL(context.Background(), acc))
}

func TestSettingServiceResolveGrokRegionalBaseURLForOAuth(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		mode string
		want string
	}{
		{mode: GrokDefaultBaseURLModeUSEast1, want: xai.DefaultUSEast1BaseURL},
		{mode: GrokDefaultBaseURLModeUSWest2, want: xai.DefaultUSWest2BaseURL},
		{mode: GrokDefaultBaseURLModeEUWest1, want: xai.DefaultEUWest1BaseURL},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			repo := &openAISettingRepoStub{values: map[string]string{
				SettingKeyGrokDefaultBaseURLMode: tt.mode,
			}}
			svc := NewSettingService(repo, nil)
			acc := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{}}
			require.Equal(t, tt.want, svc.ResolveGrokBaseURL(context.Background(), acc))

			// Older OAuth records may have persisted api.x.ai as their generated default.
			acc.Credentials["base_url"] = xai.DefaultBaseURL
			require.Equal(t, tt.want, svc.ResolveGrokBaseURL(context.Background(), acc))
		})
	}
}

func TestAccountGetGrokBaseURLOrPreservesExplicitRegionalOAuthURL(t *testing.T) {
	t.Parallel()
	acc := &Account{
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"base_url": xai.DefaultUSWest2BaseURL},
	}
	require.Equal(t, xai.DefaultUSWest2BaseURL, acc.GetGrokBaseURLOr(xai.DefaultUSEast1BaseURL))
}

func TestSettingServiceResolveGrokMediaBaseURLIgnoresCLIMode(t *testing.T) {
	t.Parallel()
	repo := &openAISettingRepoStub{values: map[string]string{
		SettingKeyGrokDefaultBaseURLMode: GrokDefaultBaseURLModeCLI,
	}}
	svc := NewSettingService(repo, nil)
	acc := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{}}
	// Even when system mode is CLI, media stays on official api.x.ai.
	require.Equal(t, xai.DefaultBaseURL, svc.ResolveGrokMediaBaseURL(context.Background(), acc))

	// Explicit CLI pin on account is rewritten to official API for media.
	acc.Credentials["base_url"] = xai.DefaultCLIBaseURL
	require.Equal(t, xai.DefaultBaseURL, svc.ResolveGrokMediaBaseURL(context.Background(), acc))

	// Non-CLI custom base (e.g. enterprise reverse proxy of api.x.ai) is kept.
	acc.Credentials["base_url"] = "https://api.x.ai/v1"
	require.Equal(t, "https://api.x.ai/v1", svc.ResolveGrokMediaBaseURL(context.Background(), acc))
}
