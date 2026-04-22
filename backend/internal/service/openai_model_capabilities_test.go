package service

import "testing"

import "github.com/stretchr/testify/require"

func TestResolveCodexRequestProfile_GPT51CodexDisablesUnsupportedControls(t *testing.T) {
	profile := ResolveCodexRequestProfile("gpt-5.1-codex")

	require.Equal(t, "gpt-5.1-codex", profile.UpstreamModel)
	require.False(t, profile.SupportsVerbosity)
	require.False(t, profile.SupportsTemperature)
	require.False(t, profile.SupportsTopP)
}

func TestResolveCodexRequestProfile_CodexMiniLatestMapsToStableUpstream(t *testing.T) {
	profile := ResolveCodexRequestProfile("codex-mini-latest")

	require.Equal(t, "gpt-5.1-codex-mini", profile.UpstreamModel)
}

func TestResolveOpenAIModelCapabilities_GPT54SupportsCompatPromptCacheKey(t *testing.T) {
	caps := ResolveOpenAIModelCapabilities("gpt-5.4")

	require.Equal(t, "gpt-5.4", caps.UpstreamModel)
	require.True(t, caps.SupportsVerbosity)
	require.True(t, caps.SupportsCompatPromptCacheKey)
}

func TestResolveOpenAIModelCapabilities_GPT51CodexDisablesCompatPromptCacheKey(t *testing.T) {
	caps := ResolveOpenAIModelCapabilities("gpt-5.1-codex")

	require.Equal(t, "gpt-5.1-codex", caps.UpstreamModel)
	require.False(t, caps.SupportsVerbosity)
	require.False(t, caps.SupportsCompatPromptCacheKey)
}

func TestResolveOpenAIModelCapabilities_GPT53AliasSharesCapabilityProfile(t *testing.T) {
	caps := ResolveOpenAIModelCapabilities("gpt-5.3")

	require.Equal(t, "gpt-5.3-codex", caps.UpstreamModel)
	require.True(t, caps.SupportsVerbosity)
	require.True(t, caps.SupportsCompatPromptCacheKey)
}
