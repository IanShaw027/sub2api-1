//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIWSLiveRelayResponseSetEvictsOldest(t *testing.T) {
	ids := newOpenAIWSLiveRelayResponseSet(2)

	ids.Remember("resp_1")
	ids.Remember("resp_2")
	require.True(t, ids.Contains("resp_1"))
	require.True(t, ids.Contains("resp_2"))

	ids.Remember("resp_3")
	require.False(t, ids.Contains("resp_1"))
	require.True(t, ids.Contains("resp_2"))
	require.True(t, ids.Contains("resp_3"))

	ids.Remember("resp_3")
	ids.Remember("resp_4")
	require.False(t, ids.Contains("resp_2"))
	require.True(t, ids.Contains("resp_3"))
	require.True(t, ids.Contains("resp_4"))
}

func TestSubtractOpenAIUsageReturnsCurrentTurnDelta(t *testing.T) {
	total := OpenAIUsage{
		InputTokens:              30,
		OutputTokens:             18,
		CacheCreationInputTokens: 7,
		CacheReadInputTokens:     11,
		ImageOutputTokens:        4,
	}
	completed := OpenAIUsage{
		InputTokens:              20,
		OutputTokens:             12,
		CacheCreationInputTokens: 2,
		CacheReadInputTokens:     9,
		ImageOutputTokens:        1,
	}

	require.Equal(t, OpenAIUsage{
		InputTokens:              10,
		OutputTokens:             6,
		CacheCreationInputTokens: 5,
		CacheReadInputTokens:     2,
		ImageOutputTokens:        3,
	}, subtractOpenAIUsage(total, completed))
}

func TestSubtractOpenAIUsageClampsNegativeDeltas(t *testing.T) {
	require.Equal(t, OpenAIUsage{}, subtractOpenAIUsage(
		OpenAIUsage{InputTokens: 1, OutputTokens: 2},
		OpenAIUsage{InputTokens: 3, OutputTokens: 4},
	))
}
