package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshalOpenAIResponsesRequestBodyOrdered_PlacesStableFieldsBeforeInput(t *testing.T) {
	body := map[string]any{
		"input":            []any{"dynamic"},
		"prompt_cache_key": "session-1",
		"model":            "gpt-5.1-codex",
		"instructions":     "keep calm",
	}

	got, err := marshalOpenAIResponsesRequestBodyOrdered(body)
	require.NoError(t, err)
	require.Contains(t, string(got), `"model":"gpt-5.1-codex","instructions":"keep calm","prompt_cache_key":"session-1","input":["dynamic"]`)
}
