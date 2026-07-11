package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesInputToChatMessages_DeveloperRoleMapsToSystem(t *testing.T) {
	messages, err := responsesInputToChatMessages("", json.RawMessage(`[{"role":"developer","content":"follow project instructions"}]`))
	require.NoError(t, err)
	require.Len(t, messages, 1)

	assert.Equal(t, "system", messages[0].Role)
	assert.JSONEq(t, `"follow project instructions"`, string(messages[0].Content))
}

func TestResponsesInputToChatMessages_KeepsChatCompletionRoles(t *testing.T) {
	input := json.RawMessage(`[
		{"role":"system","content":"system message"},
		{"role":"user","content":"user message"},
		{"role":"assistant","content":"assistant message"},
		{"role":"tool","content":"tool message"}
	]`)

	messages, err := responsesInputToChatMessages("", input)
	require.NoError(t, err)
	require.Len(t, messages, 4)

	assert.Equal(t, []string{"system", "user", "assistant", "tool"}, chatMessageRoles(messages))
}

func TestResponsesInputToChatMessages_EmptyRoleFallsBackToUser(t *testing.T) {
	messages, err := responsesInputToChatMessages("", json.RawMessage(`[{"role":"","content":"hello"}]`))
	require.NoError(t, err)
	require.Len(t, messages, 1)

	assert.Equal(t, "user", messages[0].Role)
}

func TestResponsesInputToChatMessages_DeveloperRoleTrimAndCaseInsensitive(t *testing.T) {
	input := json.RawMessage(`[
		{"role":" Developer ","content":"one"},
		{"role":"\tDEVELOPER\n","content":"two"}
	]`)

	messages, err := responsesInputToChatMessages("", input)
	require.NoError(t, err)
	require.Len(t, messages, 2)

	assert.Equal(t, []string{"system", "system"}, chatMessageRoles(messages))
}

func TestResponsesToChatCompletionsRequest_InstructionsAndInputDeveloperRole(t *testing.T) {
	req := &ResponsesRequest{
		Model:        "gpt-4o",
		Instructions: "Use concise answers.",
		Input: json.RawMessage(`[
			{"role":"developer","content":[{"type":"input_text","text":"Prefer JSON."}]},
			{"role":"user","content":"Hello"}
		]`),
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 3)

	assert.Equal(t, []string{"system", "system", "user"}, chatMessageRoles(out.Messages))
	assert.JSONEq(t, `"Use concise answers."`, string(out.Messages[0].Content))
	assert.JSONEq(t, `"Prefer JSON."`, string(out.Messages[1].Content))
	assert.JSONEq(t, `"Hello"`, string(out.Messages[2].Content))
}

func TestResponsesToChatCompletionsRequest_RejectsNilRequest(t *testing.T) {
	out, err := ResponsesToChatCompletionsRequest(nil)

	require.Nil(t, out)
	require.EqualError(t, err, "responses request is required")
}

func TestResponsesToChatCompletionsRequest_ParallelToolCalls(t *testing.T) {
	parallel := false
	req := &ResponsesRequest{
		Model: "gpt-4o",
		Input: json.RawMessage(`[
			{"role":"user","content":"Use tools"}
		]`),
		ParallelToolCalls: &parallel,
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out.ParallelToolCalls)
	assert.False(t, *out.ParallelToolCalls)

	payload, err := json.Marshal(out)
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"parallel_tool_calls":false`)
}

func TestResponsesToChatCompletionsRequest_NamespacedHistoryUsesFlattenedNameOnce(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-4o",
		Input: json.RawMessage(`[
			{"type":"function_call","call_id":"call_1","namespace":"gmail","name":"send","arguments":"{\"to\":\"a@example.com\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"ok"}
		]`),
		Tools: []ResponsesTool{{
			Type:  "namespace",
			Name:  "gmail",
			Tools: []ResponsesTool{{Type: "function", Name: "send"}},
		}},
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Tools, 1)
	require.Len(t, out.Messages, 2)
	require.Len(t, out.Messages[0].ToolCalls, 1)

	assert.Equal(t, "gmail__send", out.Tools[0].Function.Name)
	assert.Equal(t, "gmail__send", out.Messages[0].ToolCalls[0].Function.Name)
}

func TestResponsesUsageCacheAliasesRoundTripThroughChatUsage(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    int
	}{
		{name: "canonical", payload: `{"input_tokens":20,"output_tokens":5,"cache_creation_input_tokens":6}`, want: 6},
		{name: "top level write", payload: `{"prompt_tokens":20,"completion_tokens":5,"cache_write_input_tokens":7}`, want: 7},
		{name: "nested creation", payload: `{"input_tokens":20,"output_tokens":5,"input_tokens_details":{"cached_tokens":4,"cache_creation_tokens":8}}`, want: 8},
		{name: "nested write", payload: `{"prompt_tokens":20,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":4,"cache_write_tokens":9}}`, want: 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var responsesUsage ResponsesUsage
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &responsesUsage))
			assert.Equal(t, tt.want, responsesUsage.CacheCreationInputTokens)

			chatUsage := chatUsageFromResponsesUsage(&responsesUsage)
			require.NotNil(t, chatUsage)
			require.NotNil(t, chatUsage.PromptTokensDetails)

			roundTrip := ChatUsageToResponsesUsage(chatUsage)
			require.NotNil(t, roundTrip)
			assert.Equal(t, tt.want, roundTrip.CacheCreationInputTokens)
			assert.Equal(t, 20, roundTrip.InputTokens)
			assert.Equal(t, 5, roundTrip.OutputTokens)
		})
	}
}

func TestChatBridgePreservesCacheUsageInNonStreamAndStreamResponses(t *testing.T) {
	usage := &ChatUsage{
		PromptTokens:     20,
		CompletionTokens: 5,
		TotalTokens:      25,
		PromptTokensDetails: &ChatTokenDetails{
			CachedTokens:     4,
			CacheWriteTokens: 6,
		},
	}

	nonStream := ChatCompletionsResponseToResponses(&ChatCompletionsResponse{Usage: usage}, "gpt-4o")
	streamState := NewChatCompletionsToResponsesStreamState("gpt-4o")
	streamEvents := ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{Usage: usage}, streamState)
	streamEvents = append(streamEvents, FinalizeChatCompletionsResponsesStream(streamState)...)
	require.NotEmpty(t, streamEvents)
	require.NotNil(t, streamEvents[len(streamEvents)-1].Response)

	for _, got := range []*ResponsesUsage{nonStream.Usage, streamEvents[len(streamEvents)-1].Response.Usage} {
		require.NotNil(t, got)
		assert.Equal(t, 20, got.InputTokens)
		assert.Equal(t, 5, got.OutputTokens)
		assert.Equal(t, 25, got.TotalTokens)
		assert.Equal(t, 6, got.CacheCreationInputTokens)
		require.NotNil(t, got.InputTokensDetails)
		assert.Equal(t, 4, got.InputTokensDetails.CachedTokens)
		assert.Equal(t, 6, got.InputTokensDetails.CacheWriteTokens)
	}
}

func chatMessageRoles(messages []ChatMessage) []string {
	roles := make([]string, 0, len(messages))
	for _, message := range messages {
		roles = append(roles, message.Role)
	}
	return roles
}
