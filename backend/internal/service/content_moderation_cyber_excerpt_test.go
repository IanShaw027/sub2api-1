package service

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestBuildCyberPolicyInputExcerpt_OpenAIChatPreservesToolLoop(t *testing.T) {
	body := []byte(`{
		"messages":[
			{"role":"system","content":"follow policy"},
			{"role":"user","content":"look up order 42"},
			{"role":"assistant","tool_calls":[{"type":"function","function":{"name":"orders","arguments":"{\"id\":42}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"order 42 is pending"}
		]
	}`)

	excerpt := BuildCyberPolicyInputExcerpt(ContentModerationProtocolOpenAIChat, body)
	require.Contains(t, excerpt, "[system]\nfollow policy")
	require.Contains(t, excerpt, "[user]\nlook up order 42")
	require.Contains(t, excerpt, "[assistant tool_call orders]\n{\"id\":42}")
	require.Contains(t, excerpt, "[tool]\norder 42 is pending")
	require.Less(t, strings.Index(excerpt, "look up order 42"), strings.Index(excerpt, "order 42 is pending"))
}

func TestBuildCyberPolicyInputExcerpt_ResponsesToolResultWithoutUser(t *testing.T) {
	body := []byte(`{
		"input":[
			{"type":"function_call","call_id":"call_1","name":"run_tests","arguments":"{\"scope\":\"service\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"all tests passed"}
		]
	}`)

	excerpt := BuildCyberPolicyInputExcerpt(ContentModerationProtocolOpenAIResponses, body)
	require.Contains(t, excerpt, "[assistant tool_call run_tests]")
	require.Contains(t, excerpt, "{\"scope\":\"service\"}")
	require.Contains(t, excerpt, "[tool result]\nall tests passed")
}

func TestBuildCyberPolicyInputExcerpt_AnthropicLabelsToolBlocks(t *testing.T) {
	body := []byte(`{
		"system":"be concise",
		"messages":[
			{"role":"user","content":"check weather"},
			{"role":"assistant","content":[{"type":"tool_use","id":"tool_1","name":"weather","input":{"city":"LA"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool_1","content":"sunny"}]}
		]
	}`)

	excerpt := BuildCyberPolicyInputExcerpt(ContentModerationProtocolAnthropicMessages, body)
	require.Contains(t, excerpt, "[assistant tool_call weather]\n{\"city\":\"LA\"}")
	require.Contains(t, excerpt, "[tool result]\nsunny")
}

func TestBuildCyberPolicyInputExcerpt_BoundsLargeToolOutput(t *testing.T) {
	large := strings.Repeat("result-line-", 1000)
	body := []byte(`{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"run it"}]},{"type":"function_call_output","output":` + mustJSONText(t, large) + `}]}`)

	excerpt := BuildCyberPolicyInputExcerpt(ContentModerationProtocolOpenAIResponses, body)
	require.Contains(t, excerpt, "[user]\nrun it")
	require.Contains(t, excerpt, "[tool result]")
	require.Contains(t, excerpt, "…")
	require.LessOrEqual(t, utf8.RuneCountInString(excerpt), maxCyberPolicyInputExcerptRunes)
}

func mustJSONText(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}
