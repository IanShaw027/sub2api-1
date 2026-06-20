package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnthropicUserBlocksToChatPreservesToolResultOrder(t *testing.T) {
	msgs, err := anthropicUserBlocksToChat([]AnthropicContentBlock{
		{Type: "tool_result", ToolUseID: "toolu_1", Content: json.RawMessage(`"ok"`)},
		{Type: "text", Text: "next user message"},
	})

	require.NoError(t, err)
	require.Len(t, msgs, 2)
	require.Equal(t, "tool", msgs[0].Role)
	require.Equal(t, "toolu_1", msgs[0].ToolCallID)
	require.Equal(t, "user", msgs[1].Role)
	require.JSONEq(t, `[{"type":"text","text":"next user message"}]`, string(msgs[1].Content))
}

func TestAnthropicUserBlocksToChatKeepsToolResultBeforeEarlierUserContent(t *testing.T) {
	msgs, err := anthropicUserBlocksToChat([]AnthropicContentBlock{
		{Type: "text", Text: "queued user text"},
		{Type: "tool_result", ToolUseID: "toolu_1", Content: json.RawMessage(`"ok"`)},
	})

	require.NoError(t, err)
	require.Len(t, msgs, 2)
	require.Equal(t, "tool", msgs[0].Role)
	require.Equal(t, "toolu_1", msgs[0].ToolCallID)
	require.JSONEq(t, `"ok"`, string(msgs[0].Content))
	require.Equal(t, "user", msgs[1].Role)
	require.JSONEq(t, `[{"type":"text","text":"queued user text"}]`, string(msgs[1].Content))
}

func TestAnthropicRequestToChatCompletions_MapsServerToolsToNativeTools(t *testing.T) {
	req := &AnthropicRequest{
		Model: "claude-sonnet-4-5",
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"hello"`)},
		},
		Tools: []AnthropicTool{
			{Type: "web_search_20250305", Name: "web_search"},
			{Type: "web_fetch_20250910", Name: "web_fetch"},
			{Name: "get_weather", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
		ToolChoice: json.RawMessage(`{"type":"tool","name":"web_search"}`),
	}

	out, err := AnthropicRequestToChatCompletions(req)
	require.NoError(t, err)
	require.Len(t, out.Tools, 3)
	require.Equal(t, "web_search", out.Tools[0].Type)
	require.Nil(t, out.Tools[0].Function)
	require.Equal(t, "web_fetch", out.Tools[1].Type)
	require.Nil(t, out.Tools[1].Function)
	require.Equal(t, "function", out.Tools[2].Type)
	require.Equal(t, "get_weather", out.Tools[2].Function.Name)
	require.JSONEq(t, `{"type":"web_search"}`, string(out.ToolChoice))
}

func TestAnthropicRequestToChatCompletions_RejectsUnsupportedServerTools(t *testing.T) {
	req := &AnthropicRequest{
		Model: "claude-sonnet-4-5",
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"hello"`)},
		},
		Tools: []AnthropicTool{
			{Type: "code_execution_20250522", Name: "code_execution"},
		},
	}

	_, err := AnthropicRequestToChatCompletions(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported anthropic server tool for chat completions")
}
