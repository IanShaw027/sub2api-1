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
