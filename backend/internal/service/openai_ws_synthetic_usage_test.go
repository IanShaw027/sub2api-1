//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildOpenAIWSSyntheticReadFailureEventIncludesCacheWrite(t *testing.T) {
	payload := buildOpenAIWSSyntheticReadFailureEvent("resp_1", "gpt-5.6", "eof", &OpenAIUsage{
		InputTokens:              100,
		OutputTokens:             10,
		CacheReadInputTokens:     40,
		CacheCreationInputTokens: 20,
	})

	require.True(t, json.Valid(payload))
	require.Equal(t, int64(20), gjson.GetBytes(payload, "response.usage.cache_write_tokens").Int())
	require.Equal(t, int64(20), gjson.GetBytes(payload, "response.usage.cache_creation_input_tokens").Int())
	require.Equal(t, int64(20), gjson.GetBytes(payload, "response.usage.input_tokens_details.cache_write_tokens").Int())
	require.Equal(t, int64(40), gjson.GetBytes(payload, "response.usage.input_tokens_details.cached_tokens").Int())
}
