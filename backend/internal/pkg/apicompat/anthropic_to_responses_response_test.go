package apicompat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicToResponsesResponse_MapsWebSearchServerToolBlocksToWebSearchCall(t *testing.T) {
	resp := &AnthropicResponse{
		ID:         "msg_web_search",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-opus-4-6",
		StopReason: "pause_turn",
		Content: []AnthropicContentBlock{
			{
				Type:  "server_tool_use",
				ID:    "srvtoolu_search_1",
				Name:  "web_search",
				Input: []byte(`{"query":"golang"}`),
			},
			{
				Type:      "web_search_tool_result",
				ToolUseID: "srvtoolu_search_1",
				Content:   []byte(`[{"type":"url","url":"https://go.dev","title":"The Go Programming Language"}]`),
			},
		},
	}

	out := AnthropicToResponsesResponse(resp)
	require.Len(t, out.Output, 1)
	assert.Equal(t, "incomplete", out.Status)
	require.NotNil(t, out.IncompleteDetails)
	assert.Equal(t, "pause_turn", out.IncompleteDetails.Reason)
	assert.Equal(t, "web_search_call", out.Output[0].Type)
	require.NotNil(t, out.Output[0].Action)
	assert.Equal(t, "search", out.Output[0].Action.Type)
	assert.Equal(t, "golang", out.Output[0].Action.Query)
	require.Len(t, out.Output[0].Action.Sources, 1)
	assert.Equal(t, "url", out.Output[0].Action.Sources[0].Type)
	assert.Equal(t, "https://go.dev", out.Output[0].Action.Sources[0].URL)
	assert.Equal(t, "The Go Programming Language", out.Output[0].Action.Sources[0].Title)
}

func TestAnthropicToResponsesResponse_MapsWebSearchServerToolUseWithoutResult(t *testing.T) {
	resp := &AnthropicResponse{
		ID:         "msg_web_search_pause",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-opus-4-8",
		StopReason: "tool_use",
		Content: []AnthropicContentBlock{
			{
				Type:  "server_tool_use",
				ID:    "srvtoolu_search_2",
				Name:  "web_search",
				Input: []byte(`{"query":"kiro native web search"}`),
			},
		},
	}

	out := AnthropicToResponsesResponse(resp)
	require.Len(t, out.Output, 1)
	assert.Equal(t, "completed", out.Status)
	assert.Equal(t, "web_search_call", out.Output[0].Type)
	assert.Equal(t, "search_2", out.Output[0].ID)
	assert.Equal(t, "completed", out.Output[0].Status)
	require.NotNil(t, out.Output[0].Action)
	assert.Equal(t, "search", out.Output[0].Action.Type)
	assert.Equal(t, "kiro native web search", out.Output[0].Action.Query)
	assert.Empty(t, out.Output[0].Action.Sources)
}

func TestAnthropicToResponsesResponse_MapsWebFetchServerToolBlocksToFunctionCall(t *testing.T) {
	resp := &AnthropicResponse{
		ID:         "msg_web_fetch",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-opus-4-6",
		StopReason: "pause_turn",
		Content: []AnthropicContentBlock{
			{
				Type:  "server_tool_use",
				ID:    "srvtoolu_fetch_1",
				Name:  "web_fetch",
				Input: []byte(`{"url":"https://example.com"}`),
			},
			{
				Type:      "web_fetch_tool_result",
				ToolUseID: "srvtoolu_fetch_1",
				Content:   []byte(`{"url":"https://example.com"}`),
			},
		},
	}

	out := AnthropicToResponsesResponse(resp)
	require.Len(t, out.Output, 1)
	assert.Equal(t, "incomplete", out.Status)
	require.NotNil(t, out.IncompleteDetails)
	assert.Equal(t, "pause_turn", out.IncompleteDetails.Reason)
	assert.Equal(t, "function_call", out.Output[0].Type)
	assert.Equal(t, "webfetch", out.Output[0].Name)
	assert.JSONEq(t, `{
		"url":"https://example.com",
		"_sub2api_server_tool_result":{"url":"https://example.com"}
	}`, out.Output[0].Arguments)
	assert.Equal(t, "srvtoolu_fetch_1", out.Output[0].CallID)
	assert.Equal(t, "completed", out.Output[0].Status)
}

func TestAnthropicToResponsesResponse_UnwrapsJSONStringToolUseInputBackToOriginalArguments(t *testing.T) {
	resp := &AnthropicResponse{
		ID:         "msg_tool_use_invalid_args",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-opus-4-6",
		StopReason: "tool_use",
		Content: []AnthropicContentBlock{
			{
				Type:  "tool_use",
				ID:    "call_1",
				Name:  "get_weather",
				Input: []byte(`"{\"city\":"`),
			},
		},
	}

	out := AnthropicToResponsesResponse(resp)
	require.Len(t, out.Output, 1)
	assert.Equal(t, "function_call", out.Output[0].Type)
	assert.Equal(t, "get_weather", out.Output[0].Name)
	assert.Equal(t, `{"city":`, out.Output[0].Arguments)
}

func TestAnthropicEventToResponsesEvents_WebSearchServerToolUseWithoutResultStillCompletesOutputItem(t *testing.T) {
	state := NewAnthropicEventToResponsesState()

	events := AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_web_search_native",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-opus-4-8",
		},
	}, state)
	require.Len(t, events, 1)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_start",
		ContentBlock: &AnthropicContentBlock{
			Type:  "server_tool_use",
			ID:    "srvtoolu_search_3",
			Name:  "web_search",
			Input: []byte(`{}`),
		},
	}, state)
	require.Len(t, events, 1)
	require.NotNil(t, events[0].Item)
	assert.Equal(t, "web_search_call", events[0].Item.Type)
	assert.Equal(t, "in_progress", events[0].Item.Status)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_delta",
		Delta: &AnthropicDelta{
			Type:        "input_json_delta",
			PartialJSON: `{"query":"kiro native"}`,
		},
	}, state)
	require.Empty(t, events)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{Type: "content_block_stop"}, state)
	require.Empty(t, events)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_delta",
		Delta: &AnthropicDelta{
			StopReason: "tool_use",
		},
	}, state)
	require.Empty(t, events)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{Type: "message_stop"}, state)
	require.Len(t, events, 2)
	assert.Equal(t, "response.output_item.done", events[0].Type)
	require.NotNil(t, events[0].Item)
	assert.Equal(t, "web_search_call", events[0].Item.Type)
	assert.Equal(t, "completed", events[0].Item.Status)
	require.NotNil(t, events[0].Item.Action)
	assert.Equal(t, "search", events[0].Item.Action.Type)
	assert.Equal(t, "kiro native", events[0].Item.Action.Query)
	assert.Empty(t, events[0].Item.Action.Sources)
	assert.Equal(t, "response.completed", events[1].Type)
	require.NotNil(t, events[1].Response)
	require.Len(t, events[1].Response.Output, 1)
	assert.Equal(t, "web_search_call", events[1].Response.Output[0].Type)
}

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

func TestAnthropicEventToResponsesEvents_ToolUseJSONStringInputUnwrapsBackToOriginalArguments(t *testing.T) {
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
			Input: anthropicToolUseInputFromArguments(`{"city":`),
		},
	}, state)
	require.Len(t, events, 1)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_stop",
	}, state)
	require.Len(t, events, 2)
	assert.Equal(t, "response.function_call_arguments.done", events[0].Type)
	assert.Equal(t, `{"city":`, events[0].Arguments)
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

func TestAnthropicEventToResponsesEvents_WebSearchServerToolStreamCompletesWithPauseTurn(t *testing.T) {
	state := NewAnthropicEventToResponsesState()

	events := AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_web_search_stream",
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
			Type:  "server_tool_use",
			ID:    "srvtoolu_search_1",
			Name:  "web_search",
			Input: []byte(`{}`),
		},
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "response.output_item.added", events[0].Type)
	require.NotNil(t, events[0].Item)
	assert.Equal(t, "web_search_call", events[0].Item.Type)
	assert.Equal(t, "search_1", events[0].Item.ID)
	assert.Equal(t, "in_progress", events[0].Item.Status)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_delta",
		Delta: &AnthropicDelta{
			Type:        "input_json_delta",
			PartialJSON: `{"query":"golang"}`,
		},
	}, state)
	require.Empty(t, events)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_stop",
	}, state)
	require.Empty(t, events)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_start",
		ContentBlock: &AnthropicContentBlock{
			Type:      "web_search_tool_result",
			ToolUseID: "srvtoolu_search_1",
			Content:   []byte(`[{"type":"url","url":"https://go.dev","title":"The Go Programming Language"}]`),
		},
	}, state)
	require.Empty(t, events)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_stop",
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "response.output_item.done", events[0].Type)
	require.NotNil(t, events[0].Item)
	assert.Equal(t, "web_search_call", events[0].Item.Type)
	assert.Equal(t, "search_1", events[0].Item.ID)
	assert.Equal(t, "completed", events[0].Item.Status)
	require.NotNil(t, events[0].Item.Action)
	assert.Equal(t, "golang", events[0].Item.Action.Query)
	require.Len(t, events[0].Item.Action.Sources, 1)
	assert.Equal(t, "https://go.dev", events[0].Item.Action.Sources[0].URL)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_delta",
		Delta: &AnthropicDelta{
			StopReason: "pause_turn",
		},
		Usage: &AnthropicUsage{
			InputTokens:  10,
			OutputTokens: 3,
		},
	}, state)
	require.Empty(t, events)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_stop",
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "response.incomplete", events[0].Type)
	require.NotNil(t, events[0].Response)
	assert.Equal(t, "incomplete", events[0].Response.Status)
	require.NotNil(t, events[0].Response.IncompleteDetails)
	assert.Equal(t, "pause_turn", events[0].Response.IncompleteDetails.Reason)
	require.Len(t, events[0].Response.Output, 1)
	assert.Equal(t, "web_search_call", events[0].Response.Output[0].Type)
	assert.Equal(t, "golang", events[0].Response.Output[0].Action.Query)
}

func TestAnthropicEventToResponsesEvents_WebFetchServerToolStreamPreservesResultContentAndCallID(t *testing.T) {
	state := NewAnthropicEventToResponsesState()

	events := AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_web_fetch_stream",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-opus-4-6",
		},
	}, state)
	require.Len(t, events, 1)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_start",
		ContentBlock: &AnthropicContentBlock{
			Type:  "server_tool_use",
			ID:    "srvtoolu_fetch_1",
			Name:  "web_fetch",
			Input: []byte(`{}`),
		},
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "response.output_item.added", events[0].Type)
	require.NotNil(t, events[0].Item)
	assert.Equal(t, "function_call", events[0].Item.Type)
	assert.Equal(t, "srvtoolu_fetch_1", events[0].Item.CallID)
	assert.Equal(t, "webfetch", events[0].Item.Name)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_delta",
		Delta: &AnthropicDelta{
			Type:        "input_json_delta",
			PartialJSON: `{"url":"https://example.com"}`,
		},
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "response.function_call_arguments.delta", events[0].Type)
	assert.Equal(t, `{"url":"https://example.com"}`, events[0].Delta)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_stop",
	}, state)
	require.Empty(t, events)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_start",
		ContentBlock: &AnthropicContentBlock{
			Type:      "web_fetch_tool_result",
			ToolUseID: "srvtoolu_fetch_1",
			Content:   []byte(`{"type":"web_fetch_result","url":"https://example.com","title":"Example","text":"Hello from fetch"}`),
		},
	}, state)
	require.Empty(t, events)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "content_block_stop",
	}, state)
	require.Len(t, events, 2)
	assert.Equal(t, "response.function_call_arguments.done", events[0].Type)
	assert.Equal(t, "srvtoolu_fetch_1", events[0].CallID)
	assert.JSONEq(t, `{
		"url":"https://example.com",
		"_sub2api_server_tool_result":{
			"type":"web_fetch_result",
			"url":"https://example.com",
			"title":"Example",
			"text":"Hello from fetch"
		}
	}`, events[0].Arguments)
	assert.Equal(t, "response.output_item.done", events[1].Type)
	require.NotNil(t, events[1].Item)
	assert.Equal(t, "function_call", events[1].Item.Type)
	assert.Equal(t, "completed", events[1].Item.Status)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_delta",
		Delta: &AnthropicDelta{
			StopReason: "pause_turn",
		},
	}, state)
	require.Empty(t, events)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_stop",
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "response.incomplete", events[0].Type)
	require.NotNil(t, events[0].Response)
	assert.Equal(t, "incomplete", events[0].Response.Status)
	require.NotNil(t, events[0].Response.IncompleteDetails)
	assert.Equal(t, "pause_turn", events[0].Response.IncompleteDetails.Reason)
	require.Len(t, events[0].Response.Output, 1)
	assert.Equal(t, "function_call", events[0].Response.Output[0].Type)
	assert.Equal(t, "srvtoolu_fetch_1", events[0].Response.Output[0].CallID)
	assert.JSONEq(t, `{
		"url":"https://example.com",
		"_sub2api_server_tool_result":{
			"type":"web_fetch_result",
			"url":"https://example.com",
			"title":"Example",
			"text":"Hello from fetch"
		}
	}`, events[0].Response.Output[0].Arguments)
}

func TestAnthropicEventToResponsesEvents_MessageStopUsesStopReasonForIncompleteStatus(t *testing.T) {
	state := NewAnthropicEventToResponsesState()

	events := AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_incomplete",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-opus-4-6",
		},
	}, state)
	require.Len(t, events, 1)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_delta",
		Delta: &AnthropicDelta{
			StopReason: "model_context_window_exceeded",
		},
	}, state)
	require.Empty(t, events)

	events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_stop",
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "response.incomplete", events[0].Type)
	require.NotNil(t, events[0].Response)
	assert.Equal(t, "incomplete", events[0].Response.Status)
	require.NotNil(t, events[0].Response.IncompleteDetails)
	assert.Equal(t, "model_context_window_exceeded", events[0].Response.IncompleteDetails.Reason)
}

func TestFinalizeAnthropicResponsesStreamUsesParsedStopReason(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_truncated",
			Model: "claude-opus-4-6",
		},
	}, state)
	AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type:  "message_delta",
		Delta: &AnthropicDelta{StopReason: "max_tokens"},
	}, state)

	events := FinalizeAnthropicResponsesStream(state)
	require.Len(t, events, 1)
	assert.Equal(t, "response.incomplete", events[0].Type)
	require.NotNil(t, events[0].Response)
	assert.Equal(t, "incomplete", events[0].Response.Status)
	require.NotNil(t, events[0].Response.IncompleteDetails)
	assert.Equal(t, "max_output_tokens", events[0].Response.IncompleteDetails.Reason)
}
