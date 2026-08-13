package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesToAnthropicRequest_ConvertsCustomToolInput(t *testing.T) {
	req := &ResponsesRequest{
		Model: "claude-opus-4-6",
		Input: json.RawMessage(`[
			{"type":"custom_tool_call","call_id":"call_patch","name":"apply_patch","input":"*** Begin Patch"},
			{"type":"custom_tool_call_output","call_id":"call_patch","output":"Done"}
		]`),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 2)

	var toolUse []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[0].Content, &toolUse))
	require.Len(t, toolUse, 1)
	require.Equal(t, "apply_patch", toolUse[0].Name)
	require.JSONEq(t, `"*** Begin Patch"`, string(toolUse[0].Input))
}

func TestResponsesToAnthropicRequest_ConvertsToolSearchObjectArgumentsAndOutput(t *testing.T) {
	req := &ResponsesRequest{
		Model: "claude-opus-4-6",
		Input: json.RawMessage(`[
			{"type":"tool_search_call","call_id":"call_search","arguments":{"query":"gmail","limit":2}},
			{"type":"tool_search_output","call_id":"call_search","output":{"tools":[{"name":"gmail_search"}]}}
		]`),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 2)

	var toolUse []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[0].Content, &toolUse))
	require.Len(t, toolUse, 1)
	require.Equal(t, "tool_search", toolUse[0].Name)
	require.JSONEq(t, `{"query":"gmail","limit":2}`, string(toolUse[0].Input))

	var toolResult []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[1].Content, &toolResult))
	require.Len(t, toolResult, 1)
	require.JSONEq(t, `{"tools":[{"name":"gmail_search"}]}`, string(toolResult[0].Content))
}
