package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPrepareOpenAIWSUnsafeToolContinuationFullReplayPreflightDropsPreviousWhenToolContextComplete(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","store":false,"previous_response_id":"resp_prev","input":[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"ok"},{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]}`)
	current := map[string]any{}
	require.NoError(t, jsonUnmarshalOpenAIWSTest(body, &current))

	prepared, changed, err := prepareOpenAIWSUnsafeToolContinuationFullReplayPreflight(current, body)
	require.NoError(t, err)
	require.True(t, changed)

	rebuilt := payloadAsJSONBytes(prepared)
	require.False(t, gjson.GetBytes(rebuilt, "previous_response_id").Exists())
	require.False(t, gjson.GetBytes(rebuilt, "store").Bool())
	require.True(t, HasFunctionCallOutput(prepared))
	require.Equal(t, "function_call", gjson.GetBytes(rebuilt, "input.0.type").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(rebuilt, "input.1.type").String())
}

func TestPrepareOpenAIWSUnsafeToolContinuationFullReplayPreflightKeepsUnsafePayloadWhenToolContextMissing(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","store":false,"previous_response_id":"resp_prev","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"},{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]}`)
	current := map[string]any{}
	require.NoError(t, jsonUnmarshalOpenAIWSTest(body, &current))

	prepared, changed, err := prepareOpenAIWSUnsafeToolContinuationFullReplayPreflight(current, body)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, current, prepared)
}

func jsonUnmarshalOpenAIWSTest(body []byte, out any) error {
	return json.Unmarshal(body, out)
}
