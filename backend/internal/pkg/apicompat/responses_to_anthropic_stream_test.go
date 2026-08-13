package apicompat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesEventToAnthropicEvents_ToolSearchCallOpensToolUse(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_ts", Model: "gpt-5.4"},
	}, state)

	start := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item: &ResponsesOutput{
			Type:   "tool_search_call",
			ID:     "ts_1",
			CallID: "call_search",
			Name:   "ignored_name",
		},
	}, state)
	require.Len(t, start, 1)
	assert.Equal(t, "content_block_start", start[0].Type)
	require.NotNil(t, start[0].ContentBlock)
	assert.Equal(t, "tool_use", start[0].ContentBlock.Type)
	assert.Equal(t, "call_search", start[0].ContentBlock.ID)
	assert.Equal(t, toolSearchProxyName, start[0].ContentBlock.Name)
	assert.JSONEq(t, `{}`, string(start[0].ContentBlock.Input))
	assert.True(t, state.HasToolCall)
	assert.True(t, state.ContentBlockOpen)

	// Arguments arrive on output_item.done (no function_call_arguments.* events).
	done := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.output_item.done",
		OutputIndex: 0,
		Item: &ResponsesOutput{
			Type:      "tool_search_call",
			ID:        "ts_1",
			CallID:    "call_search",
			Arguments: `{"query":"docs","limit":3}`,
			Status:    "completed",
		},
	}, state)
	require.Len(t, done, 2)
	assert.Equal(t, "content_block_delta", done[0].Type)
	require.NotNil(t, done[0].Delta)
	assert.Equal(t, "input_json_delta", done[0].Delta.Type)
	assert.JSONEq(t, `{"query":"docs","limit":3}`, done[0].Delta.PartialJSON)
	assert.Equal(t, "content_block_stop", done[1].Type)
	assert.False(t, state.ContentBlockOpen)

	// Finalize should report tool_use stop_reason via HasToolCall.
	final := FinalizeResponsesAnthropicStream(state)
	require.NotEmpty(t, final)
	var stopReason string
	for _, evt := range final {
		if evt.Type == "message_delta" && evt.Delta != nil {
			stopReason = evt.Delta.StopReason
		}
	}
	assert.Equal(t, "tool_use", stopReason)
}
