package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesEventToAnthropicEvents_MessageStartSSEEmitsPingAndCleanEnvelope(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.created",
		Response: &ResponsesResponse{
			ID:    "resp_ping_1",
			Model: "gpt-5.4",
		},
	}, state)

	require.Len(t, events, 1)
	assert.Equal(t, "message_start", events[0].Type)

	payload, err := json.Marshal(events[0])
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"stop_reason":null`)
	assert.Contains(t, string(payload), `"stop_sequence":null`)
	assert.NotContains(t, string(payload), "stop_details")

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))

	message, ok := decoded["message"].(map[string]any)
	require.True(t, ok)
	require.NotNil(t, events[0].Message)
	assert.JSONEq(t, "null", string(events[0].Message.Container))
	assert.JSONEq(t, "null", string(events[0].Message.ContextManagement))
	stopReason, ok := message["stop_reason"]
	require.True(t, ok)
	assert.Nil(t, stopReason)
	stopSequence, ok := message["stop_sequence"]
	require.True(t, ok)
	assert.Nil(t, stopSequence)
	usage, ok := message["usage"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(0), usage["input_tokens"])
	assert.Equal(t, float64(0), usage["output_tokens"])
	assert.Equal(t, float64(0), usage["cache_creation_input_tokens"])
	assert.Equal(t, float64(0), usage["cache_read_input_tokens"])

	sse, err := ResponsesAnthropicEventToSSE(events[0])
	require.NoError(t, err)
	assert.Contains(t, sse, "event: message_start\n")
	assert.Contains(t, sse, `"stop_reason":null`)
	assert.Contains(t, sse, `"stop_sequence":null`)
	assert.Contains(t, sse, "event: ping\n")
	assert.Contains(t, sse, `{"type":"ping"}`)
}

func TestResponsesEventToAnthropicEvents_RefusalDeltaIncludesStopDetails(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.created",
		Response: &ResponsesResponse{
			ID:    "resp_refusal_1",
			Model: "gpt-5.4",
		},
	}, state)

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
			Output: []ResponsesOutput{
				{
					Type: "message",
					Content: []ResponsesContentPart{
						{Type: "refusal", Refusal: "I can't help with that."},
					},
				},
			},
			Usage: &ResponsesUsage{
				InputTokens:  12,
				OutputTokens: 3,
			},
		},
	}, state)

	require.Len(t, events, 2)
	assert.Equal(t, "message_delta", events[0].Type)
	require.NotNil(t, events[0].Delta)
	assert.Equal(t, "refusal", events[0].Delta.StopReason)
	require.NotNil(t, events[0].Delta.StopDetails)
	assert.Equal(t, "refusal", events[0].Delta.StopDetails.Type)
	assert.Equal(t, "message_stop", events[1].Type)
}

func TestResponsesEventToAnthropicEvents_MessageItemAddedClosesOpenToolBlock(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.created",
		Response: &ResponsesResponse{
			ID:    "resp_autoclose_1",
			Model: "gpt-5.4",
		},
	}, state)

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item: &ResponsesOutput{
			Type:   "function_call",
			CallID: "call_1",
			Name:   "ping",
		},
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "content_block_start", events[0].Type)
	assert.Equal(t, "tool_use", events[0].ContentBlock.Type)

	events = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 1,
		Item: &ResponsesOutput{
			Type: "message",
		},
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "content_block_stop", events[0].Type)
}

func TestResponsesEventToAnthropicEvents_CompletedDeltaEmitsAnthropicEnvelopeDefaults(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.created",
		Response: &ResponsesResponse{
			ID:    "resp_completed_1",
			Model: "gpt-5.4",
		},
	}, state)

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
		},
	}, state)

	require.Len(t, events, 2)
	assert.Equal(t, "message_delta", events[0].Type)
	require.NotNil(t, events[0].Delta)
	assert.Equal(t, "end_turn", events[0].Delta.StopReason)
	assert.JSONEq(t, "null", string(events[0].Delta.Container))
	assert.JSONEq(t, "null", string(events[0].ContextManagement))
	require.NotNil(t, events[0].Usage)
	assert.Equal(t, 0, events[0].Usage.InputTokens)
	assert.Equal(t, 0, events[0].Usage.OutputTokens)
	assert.Equal(t, 0, events[0].Usage.CacheCreationInputTokens)
	assert.Equal(t, 0, events[0].Usage.CacheReadInputTokens)
	require.NotNil(t, events[0].Usage.CacheCreation)
	assert.Equal(t, 0, events[0].Usage.CacheCreation.Ephemeral5mInputTokens)
	assert.Equal(t, 0, events[0].Usage.CacheCreation.Ephemeral1hInputTokens)
	assert.Equal(t, "standard", events[0].Usage.ServiceTier)
	assert.Equal(t, "", events[0].Usage.InferenceGeo)
	assert.Empty(t, events[0].Usage.Iterations)
	assert.Equal(t, "standard", events[0].Usage.Speed)
	assert.Equal(t, "message_stop", events[1].Type)

	payload, err := json.Marshal(events[0])
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"context_management":null`)
	assert.Contains(t, string(payload), `"container":null`)
	assert.Contains(t, string(payload), `"input_tokens":0`)
	assert.Contains(t, string(payload), `"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":0}`)
}

func TestResponsesEventToAnthropicEvents_FunctionCallArgumentsDoneSynthesizesInputJSONDelta(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.created",
		Response: &ResponsesResponse{
			ID:    "resp_tool_1",
			Model: "gpt-5.4",
		},
	}, state)

	startEvents := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item: &ResponsesOutput{
			Type:   "function_call",
			CallID: "call_1",
			Name:   "lookup_weather",
		},
	}, state)
	require.Len(t, startEvents, 1)

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.done",
		OutputIndex: 0,
		Arguments:   `{"city":"NYC"}`,
	}, state)

	require.Len(t, events, 2)
	assert.Equal(t, "content_block_delta", events[0].Type)
	require.NotNil(t, events[0].Index)
	assert.Equal(t, 0, *events[0].Index)
	require.NotNil(t, events[0].Delta)
	assert.Equal(t, "input_json_delta", events[0].Delta.Type)
	assert.Equal(t, `{"city":"NYC"}`, events[0].Delta.PartialJSON)
	assert.Equal(t, "content_block_stop", events[1].Type)
	require.NotNil(t, events[1].Index)
	assert.Equal(t, 0, *events[1].Index)
	assert.True(t, state.HasReceivedArgumentsDelta)
	assert.False(t, state.ContentBlockOpen)
	assert.Equal(t, 1, state.ContentBlockIndex)
}

func TestResponsesEventToAnthropicEvents_CustomToolInputDoneSynthesizesInputDelta(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_custom", Model: "gpt-5.4"},
	}, state)
	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item:        &ResponsesOutput{Type: "custom_tool_call", CallID: "call_custom", Name: "apply_patch"},
	}, state)

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.custom_tool_call_input.done",
		OutputIndex: 0,
		Input:       "*** Begin Patch",
	}, state)

	require.Len(t, events, 2)
	assert.Equal(t, "content_block_delta", events[0].Type)
	require.NotNil(t, events[0].Delta)
	assert.Equal(t, "input_json_delta", events[0].Delta.Type)
	// Freeform text is JSON-encoded to match non-stream anthropicToolUseInputFromArguments.
	assert.Equal(t, `"*** Begin Patch"`, events[0].Delta.PartialJSON)
	assert.JSONEq(t, `"*** Begin Patch"`, events[0].Delta.PartialJSON)
	assert.Equal(t, "content_block_stop", events[1].Type)
	assert.False(t, state.ContentBlockOpen)
	assert.True(t, state.SawToolUse)
}

func TestResponsesEventToAnthropicEvents_CustomToolInputDeltaBufferedAndJSONEncodedOnDone(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_custom_delta", Model: "gpt-5.4"},
	}, state)
	start := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item:        &ResponsesOutput{Type: "custom_tool_call", CallID: "call_patch", Name: "apply_patch"},
	}, state)
	require.Len(t, start, 1)
	assert.Equal(t, "content_block_start", start[0].Type)
	assert.Equal(t, "tool_use", start[0].ContentBlock.Type)
	assert.Equal(t, "apply_patch", start[0].ContentBlock.Name)

	// Freeform deltas are buffered (not emitted raw as PartialJSON).
	mid := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.custom_tool_call_input.delta",
		OutputIndex: 0,
		Delta:       "*** Begin ",
	}, state)
	assert.Empty(t, mid)

	mid = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.custom_tool_call_input.delta",
		OutputIndex: 0,
		Delta:       "Patch",
	}, state)
	assert.Empty(t, mid)

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.custom_tool_call_input.done",
		OutputIndex: 0,
		// Empty Input: fall back to buffered deltas.
	}, state)

	require.Len(t, events, 2)
	assert.Equal(t, "content_block_delta", events[0].Type)
	require.NotNil(t, events[0].Delta)
	assert.Equal(t, "input_json_delta", events[0].Delta.Type)
	assert.JSONEq(t, `"*** Begin Patch"`, events[0].Delta.PartialJSON)
	assert.Equal(t, "content_block_stop", events[1].Type)
}

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
	assert.True(t, state.SawToolUse)
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

	// Finalize should report tool_use stop_reason via SawToolUse.
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

func TestResponsesEventToAnthropicEvents_CompletedStopReasonUsesOutputToolCalls(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_tools", Model: "gpt-5.2"},
	}, state)
	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item:        &ResponsesOutput{Type: "function_call", CallID: "call_1", Name: "get_weather"},
	}, state)
	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.done",
		OutputIndex: 0,
		Arguments:   `{"city":"NYC"}`,
	}, state)
	// Simulate residual CurrentBlockType after a later text block would have been wrong;
	// completed must still prefer Response.Output tool calls.
	state.CurrentBlockType = "text"

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			ID:     "resp_tools",
			Status: "completed",
			Output: []ResponsesOutput{{
				Type:      "function_call",
				CallID:    "call_1",
				Name:      "get_weather",
				Arguments: `{"city":"NYC"}`,
			}},
		},
	}, state)

	var stopReason string
	for _, evt := range events {
		if evt.Type == "message_delta" && evt.Delta != nil {
			stopReason = evt.Delta.StopReason
		}
	}
	assert.Equal(t, "tool_use", stopReason)
}
