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
