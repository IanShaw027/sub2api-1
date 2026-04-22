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

func TestResolveOpenAIModelCapabilities_GPT53AliasSharesCapabilityProfile(t *testing.T) {
	caps := ResolveOpenAIModelCapabilities("gpt-5.3")

	require.Equal(t, "gpt-5.3-codex", caps.UpstreamModel)
	require.True(t, caps.SupportsVerbosity)
	require.True(t, caps.SupportsCompatPromptCacheKey)
}

func TestResolveOpenAIModelCapabilities_CompatPromptCacheKeyCoverage(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		upstream  string
		supported bool
	}{
		{name: "gpt-5.1", model: "gpt-5.1", upstream: "gpt-5.1", supported: true},
		{name: "gpt-5.1-codex", model: "gpt-5.1-codex", upstream: "gpt-5.1-codex", supported: true},
		{name: "gpt-5.1-codex-mini", model: "gpt-5.1-codex-mini", upstream: "gpt-5.1-codex-mini", supported: true},
		{name: "gpt-5.2-codex", model: "gpt-5.2-codex", upstream: "gpt-5.2-codex", supported: true},
		{name: "gpt-5.3-codex-spark", model: "gpt-5.3-codex-spark", upstream: "gpt-5.3-codex", supported: true},
		{name: "gpt-4o", model: "gpt-4o", supported: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := ResolveOpenAIModelCapabilities(tt.model)
			if tt.upstream != "" {
				require.Equal(t, tt.upstream, caps.UpstreamModel)
			}
			require.Equal(t, tt.supported, caps.SupportsCompatPromptCacheKey)
		})
	}
}
