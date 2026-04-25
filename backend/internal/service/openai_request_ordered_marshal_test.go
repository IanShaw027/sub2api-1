package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func requireOrderedJSONKeys(t *testing.T, body []byte, keys ...string) {
	t.Helper()

	payload := string(body)
	prev := -1
	for _, key := range keys {
		idx := strings.Index(payload, `"`+key+`":`)
		require.NotEqualf(t, -1, idx, "missing key %q in body: %s", key, payload)
		if prev >= 0 {
			require.Greaterf(t, idx, prev, "key %q should appear after previous keys in body: %s", key, payload)
		}
		prev = idx
	}
}

func TestMarshalOpenAIResponsesRequestBodyOrdered_PlacesStableFieldsBeforeInput(t *testing.T) {
	body := map[string]any{
		"input":            []any{"dynamic"},
		"prompt_cache_key": "session-1",
		"model":            "gpt-5.1-codex",
		"instructions":     "keep calm",
	}

	got, err := marshalOpenAIResponsesRequestBodyOrdered(body)
	require.NoError(t, err)
	requireOrderedJSONKeys(t, got, "model", "instructions", "prompt_cache_key", "input")
}
