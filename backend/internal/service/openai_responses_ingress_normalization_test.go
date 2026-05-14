package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeOpenAIResponsesIngress_PrefersExistingInputForFullReplay(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-sonnet-4.5","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}],"messages":[{"role":"developer","content":"ignore me"}],"previous_response_id":"resp_stale"}`)

	normalized, err := normalizeOpenAIResponsesIngress(body)
	require.NoError(t, err)
	require.Equal(t, string(body), string(normalized.PrimaryBody))
	require.Equal(t, "input", normalized.FullReplaySource)
	require.False(t, gjson.GetBytes(normalized.FullReplayBody, "messages").Exists())
	require.False(t, gjson.GetBytes(normalized.FullReplayBody, "previous_response_id").Exists())
	require.Equal(t, "function_call_output", gjson.GetBytes(normalized.FullReplayBody, "input.0.type").String())
	require.Equal(t, "call_1", gjson.GetBytes(normalized.FullReplayBody, "input.0.call_id").String())
}

func TestNormalizeOpenAIResponsesIngress_NormalizesLegacyMessagesWhenInputMissing(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-sonnet-4.5","messages":[{"role":"system","content":"repo policy"},{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"hello\"}"}}]},{"role":"tool","tool_call_id":"call_1","content":"ok"}],"previous_response_id":"resp_stale"}`)

	normalized, err := normalizeOpenAIResponsesIngress(body)
	require.NoError(t, err)
	require.Empty(t, normalized.FullReplayBody)
	require.Empty(t, normalized.FullReplaySource)
	require.False(t, gjson.GetBytes(normalized.PrimaryBody, "messages").Exists())
	require.False(t, gjson.GetBytes(normalized.PrimaryBody, "previous_response_id").Exists())
	require.Equal(t, "system", gjson.GetBytes(normalized.PrimaryBody, "input.0.role").String())
	require.Equal(t, "repo policy", gjson.GetBytes(normalized.PrimaryBody, "input.0.content").String())
	require.Equal(t, "function_call", gjson.GetBytes(normalized.PrimaryBody, "input.1.type").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(normalized.PrimaryBody, "input.2.type").String())
}

func TestSetOpenAIResponsesIngressInstructions_PreservesNonBlankValue(t *testing.T) {
	t.Parallel()

	body := []byte(`{"instructions":"keep me","input":[]}`)

	updated, err := setOpenAIResponsesIngressInstructions(body, "replace me")
	require.NoError(t, err)
	require.JSONEq(t, string(body), string(updated))
}

func TestSetOpenAIResponsesIngressInstructions_FillsBlankValue(t *testing.T) {
	t.Parallel()

	body := []byte(`{"instructions":"   ","input":[]}`)

	updated, err := setOpenAIResponsesIngressInstructions(body, "use me")
	require.NoError(t, err)
	require.Equal(t, "use me", gjson.GetBytes(updated, "instructions").String())
}

func TestDropOpenAIResponsesIngressPreviousResponseID_MalformedJSONKeepsSjsonResult(t *testing.T) {
	t.Parallel()

	body := []byte(`{"previous_response_id":"resp_broken"`)

	updated := dropOpenAIResponsesIngressPreviousResponseID(body)
	require.Equal(t, "{", string(updated))
}
