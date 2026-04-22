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
