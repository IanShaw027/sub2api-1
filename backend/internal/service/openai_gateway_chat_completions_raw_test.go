package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestConvertResponsesInputRawToChatMessages_PreservesInputTextWhitespace(t *testing.T) {
	raw := `[{"type":"input_text","text":"  hello  "}]`

	got, err := convertResponsesInputRawToChatMessages(raw)
	require.NoError(t, err)

	var messages []apicompat.ChatMessage
	require.NoError(t, json.Unmarshal(got, &messages))
	require.Len(t, messages, 1)
	require.Equal(t, "user", messages[0].Role)
	require.Equal(t, `"  hello  "`, string(messages[0].Content))
}

func TestConvertResponsesInputRawToChatMessages_ConvertsInputImageItems(t *testing.T) {
	raw := `[{"type":"input_image","image_url":"https://example.com/a.png"}]`

	got, err := convertResponsesInputRawToChatMessages(raw)
	require.NoError(t, err)

	var messages []apicompat.ChatMessage
	require.NoError(t, json.Unmarshal(got, &messages))
	require.Len(t, messages, 1)
	require.Equal(t, "user", messages[0].Role)
	require.Contains(t, string(messages[0].Content), `"image_url"`)
	require.Contains(t, string(messages[0].Content), "https://example.com/a.png")
}

func TestNormalizeRawChatCompletionsCompatBody_StripsResponsesOnlyFields(t *testing.T) {
	body := []byte(`{"model":"gpt-5.1","messages":[{"role":"user","content":"hello"}],"store":false,"parallel_tool_calls":true,"include":[{"type":"reasoning"}],"previous_response_id":"resp_1","reasoning":{"effort":"low"}}`)

	got, err := normalizeRawChatCompletionsCompatBody(body)
	require.NoError(t, err)

	require.False(t, gjson.GetBytes(got, "store").Exists())
	require.False(t, gjson.GetBytes(got, "parallel_tool_calls").Exists())
	require.False(t, gjson.GetBytes(got, "include").Exists())
	require.False(t, gjson.GetBytes(got, "previous_response_id").Exists())
	require.False(t, gjson.GetBytes(got, "reasoning").Exists())
}
