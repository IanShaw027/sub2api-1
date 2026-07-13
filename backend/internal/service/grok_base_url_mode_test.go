package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestNormalizeGrokDefaultBaseURLMode(t *testing.T) {
	t.Parallel()
	require.Equal(t, GrokDefaultBaseURLModeAPI, normalizeGrokDefaultBaseURLMode(""))
	require.Equal(t, GrokDefaultBaseURLModeAPI, normalizeGrokDefaultBaseURLMode("api"))
	require.Equal(t, GrokDefaultBaseURLModeAPI, normalizeGrokDefaultBaseURLMode("API"))
	require.Equal(t, GrokDefaultBaseURLModeCLI, normalizeGrokDefaultBaseURLMode("cli"))
	require.Equal(t, GrokDefaultBaseURLModeCLI, normalizeGrokDefaultBaseURLMode(" CLI "))
	require.Equal(t, GrokDefaultBaseURLModeAPI, normalizeGrokDefaultBaseURLMode("unknown"))
}

func TestGrokBaseURLForMode(t *testing.T) {
	t.Parallel()
	require.Equal(t, xai.DefaultBaseURL, GrokBaseURLForMode("api"))
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
	require.Equal(t, xai.DefaultBaseURL, acc.GetGrokBaseURLOr(""))

	acc.Credentials["base_url"] = "https://api.x.ai/v1"
	require.Equal(t, "https://api.x.ai/v1", acc.GetGrokBaseURLOr(xai.DefaultCLIBaseURL))
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
	require.Equal(t, xai.DefaultBaseURL, svc.ResolveGrokBaseURL(context.Background(), acc))
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
