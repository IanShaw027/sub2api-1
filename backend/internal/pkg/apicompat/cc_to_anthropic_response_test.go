package apicompat

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCcUsageToAnthropicSeparatesCachedPromptTokens(t *testing.T) {
	got := CcUsageToAnthropic(&ChatUsage{
		PromptTokens:     20,
		CompletionTokens: 5,
		PromptTokensDetails: &ChatTokenDetails{
			CachedTokens: 7,
		},
	})

	require.NotNil(t, got)
	require.Equal(t, 13, got.InputTokens)
	require.Equal(t, 7, got.CacheReadInputTokens)
	require.Equal(t, 5, got.OutputTokens)
}

func TestCcUsageToAnthropicFloorsInputTokensWhenCachedExceedsPrompt(t *testing.T) {
	got := CcUsageToAnthropic(&ChatUsage{
		PromptTokens:     3,
		CompletionTokens: 5,
		PromptTokensDetails: &ChatTokenDetails{
			CachedTokens: 7,
		},
	})

	require.NotNil(t, got)
	require.Equal(t, 0, got.InputTokens)
	require.Equal(t, 7, got.CacheReadInputTokens)
	require.Equal(t, 5, got.OutputTokens)
}
