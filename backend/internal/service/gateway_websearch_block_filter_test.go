//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const emulatedWebSearchBody = `{"model":"claude-sonnet-4-6","messages":[` +
	`{"role":"user","content":[{"type":"text","text":"search"}]},` +
	`{"role":"assistant","content":[` +
	`{"type":"server_tool_use","id":"srvtoolu_ws_0123456789abcdef","name":"web_search","input":{"query":"weather"}},` +
	`{"type":"web_search_tool_result","tool_use_id":"srvtoolu_ws_0123456789abcdef","content":[]},` +
	`{"type":"text","text":"search summary"}]}]}`

const genuineWebSearchBody = `{"model":"claude-sonnet-4-6","messages":[` +
	`{"role":"assistant","content":[` +
	`{"type":"server_tool_use","id":"srvtoolu_01ABCDEF","name":"web_search","input":{"query":"weather"}},` +
	`{"type":"web_search_tool_result","tool_use_id":"srvtoolu_01ABCDEF","content":[]},` +
	`{"type":"text","text":"genuine summary"}]}]}`

func webSearchContentTypes(body []byte) []string {
	var types []string
	for _, msg := range gjson.GetBytes(body, "messages").Array() {
		for _, block := range msg.Get("content").Array() {
			types = append(types, block.Get("type").String())
		}
	}
	return types
}

func TestFilterWebSearchHistoryBlocks_StripsEmulatedBlocks(t *testing.T) {
	out := FilterWebSearchHistoryBlocks([]byte(emulatedWebSearchBody), "claude-sonnet-4-6")
	require.Equal(t, []string{"text", "text"}, webSearchContentTypes(out))
	require.Contains(t, string(out), "search summary")
	require.NotContains(t, string(out), "srvtoolu_ws_")
}

func TestFilterWebSearchHistoryBlocks_KeepsGenuineAnthropicBlocks(t *testing.T) {
	body := []byte(genuineWebSearchBody)
	require.Equal(t, body, FilterWebSearchHistoryBlocks(body, "claude-sonnet-4-6"))
}

func TestFilterWebSearchHistoryBlocks_StripsAllForPassbackModel(t *testing.T) {
	out := FilterWebSearchHistoryBlocks([]byte(genuineWebSearchBody), "glm-4.7")
	require.Equal(t, []string{"text"}, webSearchContentTypes(out))
	require.Contains(t, string(out), "genuine summary")
}

func TestFilterWebSearchHistoryBlocks_AddsPlaceholderForEmptyMessage(t *testing.T) {
	body := []byte(`{"messages":[{"role":"assistant","content":[{"type":"server_tool_use","id":"srvtoolu_ws_x"}]}]}`)
	out := FilterWebSearchHistoryBlocks(body, "claude-sonnet-4-6")
	require.Equal(t, "text", gjson.GetBytes(out, "messages.0.content.0.type").String())
	require.Equal(t, "(assistant content removed)", gjson.GetBytes(out, "messages.0.content.0.text").String())
}

func TestFilterWebSearchHistoryBlocks_FastPathsUnchanged(t *testing.T) {
	plain := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	require.Equal(t, plain, FilterWebSearchHistoryBlocks(plain, "glm-4.7"))
	stringMention := []byte(`{"messages":[{"role":"user","content":"server_tool_use"}]}`)
	require.Equal(t, stringMention, FilterWebSearchHistoryBlocks(stringMention, "glm-4.7"))
}

func TestFilterWebSearchHistoryBlocks_PreservesOtherToolBlocks(t *testing.T) {
	body := []byte(`{"messages":[{"role":"assistant","content":[` +
		`{"type":"tool_use","id":"toolu_1"},` +
		`{"type":"server_tool_use","id":"srvtoolu_ws_x"},` +
		`{"type":"text","text":"result"}]}]}`)
	out := FilterWebSearchHistoryBlocks(body, "glm-4.7")
	require.Equal(t, []string{"tool_use", "text"}, webSearchContentTypes(out))
}

func TestFilterWebSearchHistoryBlocks_PassbackPreservesNonWebSearchServerToolUse(t *testing.T) {
	body := []byte(`{"messages":[{"role":"assistant","content":[` +
		`{"type":"server_tool_use","id":"srvtoolu_code_1","name":"code_execution","input":{}},` +
		`{"type":"tool_result","tool_use_id":"srvtoolu_code_1","content":"ok"},` +
		`{"type":"server_tool_use","id":"srvtoolu_01ABCDEF","name":"web_search","input":{"query":"weather"}},` +
		`{"type":"web_search_tool_result","tool_use_id":"srvtoolu_01ABCDEF","content":[]},` +
		`{"type":"text","text":"summary"}]}]}`)
	out := FilterWebSearchHistoryBlocks(body, "glm-4.7")
	require.Equal(t, []string{"server_tool_use", "tool_result", "text"}, webSearchContentTypes(out))
	require.Contains(t, string(out), "srvtoolu_code_1")
	require.NotContains(t, string(out), "srvtoolu_01ABCDEF")
}
