package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeOpenAIResponsesReasoningModeMapsProToMax(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-sol","reasoning":{"mode":"pro"},"input":"hello"}`)

	normalized, changed, err := normalizeOpenAIResponsesReasoningMode(body)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "max", gjson.GetBytes(normalized, "reasoning.effort").String())
	require.False(t, gjson.GetBytes(normalized, "reasoning.mode").Exists())
}

func TestNormalizeOpenAIResponsesReasoningModePreservesExplicitEffort(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-sol","reasoning":{"mode":"pro","effort":"high"},"input":"hello"}`)

	normalized, changed, err := normalizeOpenAIResponsesReasoningMode(body)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "high", gjson.GetBytes(normalized, "reasoning.effort").String())
	require.False(t, gjson.GetBytes(normalized, "reasoning.mode").Exists())
}

func TestNormalizeOpenAIResponsesReasoningModeDropsUnsupportedMode(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-sol","reasoning":{"mode":"standard"},"input":"hello"}`)

	normalized, changed, err := normalizeOpenAIResponsesReasoningMode(body)

	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "reasoning").Exists())
}

func TestShouldNormalizeOpenAIResponsesReasoningMode(t *testing.T) {
	t.Parallel()

	require.True(t, shouldNormalizeOpenAIResponsesReasoningMode(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}))
	require.True(t, shouldNormalizeOpenAIResponsesReasoningMode(&Account{Platform: PlatformOpenAI, Type: AccountTypeSetupToken}))
	require.False(t, shouldNormalizeOpenAIResponsesReasoningMode(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}))
	require.False(t, shouldNormalizeOpenAIResponsesReasoningMode(&Account{Platform: PlatformGrok, Type: AccountTypeOAuth}))
	require.False(t, shouldNormalizeOpenAIResponsesReasoningMode(nil))
}

func TestNormalizeOpenAIResponsesTruncationDropsUnsupportedField(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-sol","truncation":"auto","input":"hello"}`)

	normalized, changed, err := normalizeOpenAIResponsesTruncation(body)

	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "truncation").Exists())
	require.Equal(t, "hello", gjson.GetBytes(normalized, "input").String())
}

func TestShouldNormalizeOpenAIResponsesTruncation(t *testing.T) {
	t.Parallel()

	require.True(t, shouldNormalizeOpenAIResponsesTruncation(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}))
	require.True(t, shouldNormalizeOpenAIResponsesTruncation(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeSetupToken,
	}))
	require.False(t, shouldNormalizeOpenAIResponsesTruncation(&Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeSetupToken,
	}))
	require.False(t, shouldNormalizeOpenAIResponsesTruncation(nil))
	require.False(t, shouldNormalizeOpenAIResponsesTruncation(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
	}))
}

func assertNormalizeOpenAIResponsesIngress_MergesLegacyMessagesWhenInputExists(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4.5","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}],"messages":[{"role":"developer","content":"ignore me"}],"previous_response_id":"resp_stale"}`)

	normalized, err := normalizeOpenAIResponsesIngress(body)
	require.NoError(t, err)
	require.Equal(t, string(body), string(normalized.PrimaryBody))
	require.Equal(t, "input+messages", normalized.FullReplaySource)
	require.False(t, gjson.GetBytes(normalized.FullReplayBody, "messages").Exists())
	require.False(t, gjson.GetBytes(normalized.FullReplayBody, "previous_response_id").Exists())
	require.NotEmpty(t, gjson.GetBytes(normalized.FullReplayBody, "input.0.role").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(normalized.FullReplayBody, "input.1.type").String())
	require.Equal(t, "call_1", gjson.GetBytes(normalized.FullReplayBody, "input.1.call_id").String())
}

func TestNormalizeOpenAIResponsesIngress_MergesLegacyMessagesWhenInputExists(t *testing.T) {
	t.Parallel()

	assertNormalizeOpenAIResponsesIngress_MergesLegacyMessagesWhenInputExists(t)
}

func TestNormalizeOpenAIResponsesIngress_PrefersExistingInputForUnsupportedMessages(t *testing.T) {
	t.Parallel()

	assertNormalizeOpenAIResponsesIngress_MergesLegacyMessagesWhenInputExists(t)
}

func TestNormalizeOpenAIResponsesIngress_MergesSupportedMessagesIntoFullReplay(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-sonnet-4.5","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}],"messages":[{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"hello\"}"}}]},{"role":"tool","tool_call_id":"call_1","content":"ok"}],"previous_response_id":"resp_stale"}`)

	normalized, err := normalizeOpenAIResponsesIngress(body)
	require.NoError(t, err)
	require.Equal(t, string(body), string(normalized.PrimaryBody))
	require.Equal(t, "input+messages", normalized.FullReplaySource)
	require.False(t, gjson.GetBytes(normalized.FullReplayBody, "messages").Exists())
	require.False(t, gjson.GetBytes(normalized.FullReplayBody, "previous_response_id").Exists())
	require.Equal(t, "function_call", gjson.GetBytes(normalized.FullReplayBody, "input.0.type").String())
	require.Equal(t, "call_1", gjson.GetBytes(normalized.FullReplayBody, "input.0.call_id").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(normalized.FullReplayBody, "input.1.type").String())
	require.Equal(t, "call_1", gjson.GetBytes(normalized.FullReplayBody, "input.1.call_id").String())
	require.False(t, gjson.GetBytes(normalized.FullReplayBody, "input.2").Exists())
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

func TestNormalizeOpenAIResponsesIngress_PreservesLegacyChatCompletionTopLevelFields(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-sonnet-4.5","messages":[{"role":"user","content":"weather?"}],"tools":[{"type":"function","function":{"name":"lookup","description":"Lookup weather","parameters":{"type":"object"}}}],"tool_choice":{"type":"function","function":{"name":"lookup"}},"max_tokens":64,"reasoning_effort":"high","service_tier":"flex","temperature":0.2,"top_p":0.7,"previous_response_id":"resp_stale"}`)

	normalized, err := normalizeOpenAIResponsesIngress(body)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(normalized.PrimaryBody, "messages").Exists())
	require.False(t, gjson.GetBytes(normalized.PrimaryBody, "previous_response_id").Exists())
	require.Equal(t, "user", gjson.GetBytes(normalized.PrimaryBody, "input.0.role").String())
	require.Equal(t, "function", gjson.GetBytes(normalized.PrimaryBody, "tools.0.type").String())
	require.Equal(t, "lookup", gjson.GetBytes(normalized.PrimaryBody, "tools.0.name").String())
	require.Equal(t, "Lookup weather", gjson.GetBytes(normalized.PrimaryBody, "tools.0.description").String())
	require.Equal(t, "object", gjson.GetBytes(normalized.PrimaryBody, "tools.0.parameters.type").String())
	require.Equal(t, int64(128), gjson.GetBytes(normalized.PrimaryBody, "max_output_tokens").Int())
	require.Equal(t, "high", gjson.GetBytes(normalized.PrimaryBody, "reasoning.effort").String())
	require.Equal(t, "auto", gjson.GetBytes(normalized.PrimaryBody, "reasoning.summary").String())
	require.Equal(t, "function", gjson.GetBytes(normalized.PrimaryBody, "tool_choice.type").String())
	require.Equal(t, "lookup", gjson.GetBytes(normalized.PrimaryBody, "tool_choice.function.name").String())
	require.Equal(t, "flex", gjson.GetBytes(normalized.PrimaryBody, "service_tier").String())
	require.InDelta(t, 0.2, gjson.GetBytes(normalized.PrimaryBody, "temperature").Float(), 0.001)
	require.InDelta(t, 0.7, gjson.GetBytes(normalized.PrimaryBody, "top_p").Float(), 0.001)
}

func TestNormalizeOpenAIResponsesIngress_PreservesResponsesNativeToolShapeWhenInputExists(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"gpt-5.4","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"compact me"}]}],"tools":[{"type":"function","function":{"name":"apply_patch"}}],"tool_choice":{"type":"function","function":{"name":"apply_patch"}},"previous_response_id":"resp_stale"}`)

	normalized, err := normalizeOpenAIResponsesIngress(body)
	require.NoError(t, err)
	require.Equal(t, "input", normalized.FullReplaySource)
	require.False(t, gjson.GetBytes(normalized.FullReplayBody, "previous_response_id").Exists())
	require.Equal(t, "apply_patch", gjson.GetBytes(normalized.FullReplayBody, "tools.0.function.name").String())
	require.False(t, gjson.GetBytes(normalized.FullReplayBody, "tools.0.name").Exists())
	require.Equal(t, "apply_patch", gjson.GetBytes(normalized.FullReplayBody, "tool_choice.function.name").String())
	require.False(t, gjson.GetBytes(normalized.FullReplayBody, "tool_choice.name").Exists())
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

func TestMergeResponsesReplayInputs_DedupesByID(t *testing.T) {
	t.Parallel()

	legacy := []byte(`[{"id":"msg_20","type":"message","role":"user","content":[{"type":"input_text","text":"prior"}]},{"id":"msg_21","type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}]`)
	current := []byte(`[{"id":"msg_21","type":"message","role":"assistant","content":[{"type":"output_text","text":"hello world"}]},{"id":"msg_22","type":"message","role":"user","content":[{"type":"input_text","text":"next"}]}]`)

	merged, changed, err := mergeResponsesReplayInputs(legacy, current)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, int64(3), gjson.GetBytes(merged, "#").Int())
	require.Equal(t, "msg_20", gjson.GetBytes(merged, "0.id").String())
	require.Equal(t, "msg_21", gjson.GetBytes(merged, "1.id").String())
	require.Equal(t, "hello world", gjson.GetBytes(merged, "1.content.0.text").String())
	require.Equal(t, "msg_22", gjson.GetBytes(merged, "2.id").String())
}

func TestMergeResponsesReplayInputs_DedupesByContentWhenIDMissing(t *testing.T) {
	t.Parallel()

	legacy := []byte(`[{"type":"message","role":"user","content":[{"type":"input_text","text":"earlier"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]`)
	current := []byte(`[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}]`)

	merged, changed, err := mergeResponsesReplayInputs(legacy, current)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, int64(3), gjson.GetBytes(merged, "#").Int())
	require.Equal(t, "earlier", gjson.GetBytes(merged, "0.content.0.text").String())
	require.Equal(t, "hi", gjson.GetBytes(merged, "1.content.0.text").String())
	require.Equal(t, "assistant", gjson.GetBytes(merged, "2.role").String())
}

func TestMergeResponsesReplayInputs_DedupesDuplicateIDsWithinCurrent(t *testing.T) {
	t.Parallel()

	current := []byte(`[{"id":"msg_21","type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]},{"id":"msg_21","type":"message","role":"assistant","content":[{"type":"output_text","text":"hello world"}]}]`)

	merged, changed, err := mergeResponsesReplayInputs(nil, current)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, int64(1), gjson.GetBytes(merged, "#").Int())
	require.Equal(t, "msg_21", gjson.GetBytes(merged, "0.id").String())
	require.Equal(t, "hello world", gjson.GetBytes(merged, "0.content.0.text").String())
}
