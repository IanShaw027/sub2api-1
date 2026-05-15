package apicompat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicToResponsesResponse_RefusalExplanationWithoutTextCreatesRefusalPart(t *testing.T) {
	explanation := "Blocked by policy review"
	resp := &AnthropicResponse{
		ID:         "msg_refusal",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-opus-4-6",
		Content:    []AnthropicContentBlock{},
		StopReason: "refusal",
		StopDetails: &AnthropicStopDetails{
			Type:        "refusal",
			Explanation: &explanation,
		},
	}

	out := AnthropicToResponsesResponse(resp)
	require.Len(t, out.Output, 1)
	require.Len(t, out.Output[0].Content, 1)
	assert.Equal(t, "refusal", out.Output[0].Content[0].Type)
	assert.Equal(t, explanation, out.Output[0].Content[0].Refusal)
}

func TestAnthropicEventToResponsesEvents_ToolUseInputPreservedOnDone(t *testing.T) {
	state := NewAnthropicEventToResponsesState()

	events := AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_123",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-opus-4-6",
		},
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "response.created", events[0].Type)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_start",
		ContentBlock: &AnthropicContentBlock{
			Type:  "tool_use",
			ID:    "toolu_123",
			Name:  "lookup_weather",
			Input: []byte(`{"city":"NYC"}`),
		},
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "response.output_item.added", events[0].Type)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_stop",
	}, state)
	require.Len(t, events, 2)
	assert.Equal(t, "response.function_call_arguments.done", events[0].Type)
	assert.Equal(t, `{"city":"NYC"}`, events[0].Arguments)
	assert.Equal(t, "response.output_item.done", events[1].Type)
}

func TestAnthropicEventToResponsesEvents_ToolUseEmptyStarterDoesNotPrefixArguments(t *testing.T) {
	state := NewAnthropicEventToResponsesState()

	events := AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_123",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-opus-4-6",
		},
	}, state)
	require.Len(t, events, 1)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_start",
		ContentBlock: &AnthropicContentBlock{
			Type:  "tool_use",
			ID:    "toolu_123",
			Name:  "lookup_weather",
			Input: []byte(`{}`),
		},
	}, state)
	require.Len(t, events, 1)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_delta",
		Delta: &AnthropicDelta{
			Type:        "input_json_delta",
			PartialJSON: `{"city":"NYC"}`,
		},
	}, state)
	require.Len(t, events, 1)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_stop",
	}, state)
	require.Len(t, events, 2)
	assert.Equal(t, "response.function_call_arguments.done", events[0].Type)
	assert.Equal(t, `{"city":"NYC"}`, events[0].Arguments)
}

func TestAnthropicEventToResponsesEvents_ToolUseEmptyStarterFinishesWithEmptyObject(t *testing.T) {
	state := NewAnthropicEventToResponsesState()

	events := AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_123",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-opus-4-6",
		},
	}, state)
	require.Len(t, events, 1)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_start",
		ContentBlock: &AnthropicContentBlock{
			Type:  "tool_use",
			ID:    "toolu_123",
			Name:  "lookup_weather",
			Input: []byte(`{}`),
		},
	}, state)
	require.Len(t, events, 1)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_stop",
	}, state)
	require.Len(t, events, 2)
	assert.Equal(t, "response.function_call_arguments.done", events[0].Type)
	assert.Equal(t, `{}`, events[0].Arguments)
}

func TestAnthropicEventToResponsesEvents_ToolUseExplicitEmptyInputKeepsEmptyObjectArguments(t *testing.T) {
	state := NewAnthropicEventToResponsesState()

	events := AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_123",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-opus-4-6",
		},
	}, state)
	require.Len(t, events, 1)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_start",
		ContentBlock: &AnthropicContentBlock{
			Type:  "tool_use",
			ID:    "toolu_123",
			Name:  "lookup_weather",
			Input: []byte(`{}`),
		},
	}, state)
	require.Len(t, events, 1)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_stop",
	}, state)
	require.Len(t, events, 2)
	assert.Equal(t, "response.function_call_arguments.done", events[0].Type)
	assert.Equal(t, `{}`, events[0].Arguments)
	assert.Equal(t, "response.output_item.done", events[1].Type)
}
