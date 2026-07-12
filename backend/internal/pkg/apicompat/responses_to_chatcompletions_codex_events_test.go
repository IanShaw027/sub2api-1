package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// custom_tool_call（custom/freeform 工具，如新版 apply_patch）应像 function_call 一样
// 注册为工具调用；freeform 输入在 delta 阶段只累加，done 时以 {"input":...} 信封发出，
// 与非流式 ResponsesToChatCompletions 一致。
func TestResponsesEventToChatChunks_CustomToolCallInputDelta(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-5-codex"
	state.SentRole = true

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 1,
		Item: &ResponsesOutput{
			Type:   "custom_tool_call",
			CallID: "call_patch",
			Name:   "apply_patch",
		},
	}, state)
	require.Len(t, chunks, 1)
	require.Len(t, chunks[0].Choices[0].Delta.ToolCalls, 1)
	tc := chunks[0].Choices[0].Delta.ToolCalls[0]
	assert.Equal(t, "call_patch", tc.ID)
	assert.Equal(t, "apply_patch", tc.Function.Name)
	assert.True(t, state.SawToolCall)

	// Freeform deltas are accumulated, not forwarded raw (would disagree with non-stream envelope).
	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.custom_tool_call_input.delta",
		OutputIndex: 1,
		Delta:       "*** Begin ",
	}, state)
	assert.Empty(t, chunks)
	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.custom_tool_call_input.delta",
		OutputIndex: 1,
		Delta:       "Patch",
	}, state)
	assert.Empty(t, chunks)

	// Done synthesizes the Chat {"input":...} envelope from the full input
	// (done payload preferred over accumulated deltas when present).
	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.custom_tool_call_input.done",
		OutputIndex: 1,
		Input:       "*** Begin Patch",
	}, state)
	require.Len(t, chunks, 1)
	tc = chunks[0].Choices[0].Delta.ToolCalls[0]
	require.NotNil(t, tc.Index)
	assert.Equal(t, 0, *tc.Index)
	assert.JSONEq(t, `{"input":"*** Begin Patch"}`, tc.Function.Arguments)
}

func TestResponsesEventToChatChunks_CustomToolCallInputDoneOnly(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-5-codex"
	state.SentRole = true

	_ = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 1,
		Item: &ResponsesOutput{
			Type:   "custom_tool_call",
			CallID: "call_patch",
			Name:   "apply_patch",
		},
	}, state)

	var evt ResponsesStreamEvent
	require.NoError(t, json.Unmarshal([]byte(`{"type":"response.custom_tool_call_input.done","output_index":1,"input":"*** Begin Patch"}`), &evt))
	chunks := ResponsesEventToChatChunks(&evt, state)
	require.Len(t, chunks, 1)
	tc := chunks[0].Choices[0].Delta.ToolCalls[0]
	require.NotNil(t, tc.Index)
	assert.Equal(t, 0, *tc.Index)
	assert.JSONEq(t, `{"input":"*** Begin Patch"}`, tc.Function.Arguments)
}

// tool_search_call 流式 output_item.added 必须注册工具索引，名称固定为 toolSearchProxyName，
// 并置 SawToolCall，以便后续 arguments delta/done 与 finish_reason=tool_calls 生效。
func TestResponsesEventToChatChunks_ToolSearchCallAdded(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-5-codex"
	state.SentRole = true

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 2,
		Item: &ResponsesOutput{
			Type:   "tool_search_call",
			CallID: "call_search",
			// Name may be empty on the wire; Chat side always uses the proxy name.
		},
	}, state)
	require.Len(t, chunks, 1)
	require.Len(t, chunks[0].Choices[0].Delta.ToolCalls, 1)
	tc := chunks[0].Choices[0].Delta.ToolCalls[0]
	require.NotNil(t, tc.Index)
	assert.Equal(t, 0, *tc.Index)
	assert.Equal(t, "call_search", tc.ID)
	assert.Equal(t, toolSearchProxyName, tc.Function.Name)
	assert.True(t, state.SawToolCall)

	// Arguments stream as function_call_arguments.* and must map to the registered index.
	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.delta",
		OutputIndex: 2,
		Delta:       `{"query":`,
	}, state)
	require.Len(t, chunks, 1)
	tc = chunks[0].Choices[0].Delta.ToolCalls[0]
	require.NotNil(t, tc.Index)
	assert.Equal(t, 0, *tc.Index)
	assert.Equal(t, `{"query":`, tc.Function.Arguments)

	// finish_reason should be tool_calls once completed.
	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
		},
	}, state)
	require.NotEmpty(t, chunks)
	require.NotNil(t, chunks[0].Choices[0].FinishReason)
	assert.Equal(t, "tool_calls", *chunks[0].Choices[0].FinishReason)
}

// When custom_tool_call_input.done omits input, fall back to accumulated freeform deltas
// and still emit the Chat {"input":...} envelope.
func TestResponsesEventToChatChunks_CustomToolCallDeltaAccumThenEmptyDone(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-5-codex"
	state.SentRole = true

	_ = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item: &ResponsesOutput{
			Type:   "custom_tool_call",
			CallID: "call_exec",
			Name:   "exec",
		},
	}, state)
	_ = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.custom_tool_call_input.delta",
		OutputIndex: 0,
		Delta:       "dir",
	}, state)

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.custom_tool_call_input.done",
		OutputIndex: 0,
	}, state)
	require.Len(t, chunks, 1)
	assert.JSONEq(t, `{"input":"dir"}`, chunks[0].Choices[0].Delta.ToolCalls[0].Function.Arguments)
}

// 原始推理文本增量 reasoning_text.delta 应像 reasoning_summary_text.delta 一样
// 映射为 reasoning_content。
func TestResponsesEventToChatChunks_ReasoningTextDelta(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-5-codex"
	state.SentRole = true

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:  "response.reasoning_text.delta",
		Delta: "thinking step",
	}, state)
	require.Len(t, chunks, 1)
	require.NotNil(t, chunks[0].Choices[0].Delta.ReasoningContent)
	assert.Equal(t, "thinking step", *chunks[0].Choices[0].Delta.ReasoningContent)
}

// 缓冲（非流式）累加器同样需识别两类新事件。
func TestBufferedResponseAccumulator_CodexEvents(t *testing.T) {
	acc := NewBufferedResponseAccumulator()
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item:        &ResponsesOutput{Type: "custom_tool_call", CallID: "c1", Name: "apply_patch"},
	})
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.custom_tool_call_input.delta",
		OutputIndex: 0,
		Delta:       "patch-body",
	})
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:  "response.reasoning_text.delta",
		Delta: "raw-reasoning",
	})
	require.True(t, acc.HasContent())
}

func TestBufferedResponseAccumulator_CustomToolCallInputDoneOnly(t *testing.T) {
	acc := NewBufferedResponseAccumulator()
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item:        &ResponsesOutput{Type: "custom_tool_call", CallID: "c1", Name: "apply_patch"},
	})
	var evt ResponsesStreamEvent
	require.NoError(t, json.Unmarshal([]byte(`{"type":"response.custom_tool_call_input.done","output_index":0,"input":"patch-body"}`), &evt))
	acc.ProcessEvent(&evt)

	output := acc.BuildOutput()
	require.Len(t, output, 1)
	assert.Equal(t, "patch-body", output[0].Arguments)
}
