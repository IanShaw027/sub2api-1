package apicompat

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesToAnthropicRequest_ConvertsStringInput(t *testing.T) {
	req := &ResponsesRequest{
		Model:           "gpt-5.4",
		Input:           json.RawMessage(`"Hello, Anthropic"`),
		MaxOutputTokens: intPtr(256),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)

	assert.Equal(t, "gpt-5.4", out.Model)
	assert.Equal(t, 256, out.MaxTokens)
	require.Len(t, out.Messages, 1)
	assert.Equal(t, "user", out.Messages[0].Role)
	assert.JSONEq(t, `"Hello, Anthropic"`, string(out.Messages[0].Content))
}

func TestResponsesToAnthropicRequest_MapsWebFetchToolDefinitionToServerTool(t *testing.T) {
	req := &ResponsesRequest{
		Model:           "gpt-5.4",
		Input:           json.RawMessage(`"Fetch this page"`),
		MaxOutputTokens: intPtr(256),
		Tools: []ResponsesTool{
			{
				Type:        "web_fetch_20250910",
				Name:        "web_fetch",
				Description: "Fetch a specific URL",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"}},"required":["url"]}`),
			},
		},
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Tools, 1)
	assert.Equal(t, "web_fetch_20250910", out.Tools[0].Type)
	assert.Equal(t, "web_fetch", out.Tools[0].Name)
	assert.Empty(t, out.Tools[0].Description)
	assert.Empty(t, out.Tools[0].InputSchema)
}

func TestResponsesToAnthropicRequest_MapsServerToolChoice(t *testing.T) {
	tests := []struct {
		name       string
		toolType   string
		choice     string
		wantName   string
		wantChoice string
	}{
		{
			name:       "web_search",
			toolType:   "web_search",
			choice:     `{"type":"web_search"}`,
			wantName:   "web_search",
			wantChoice: `{"type":"tool","name":"web_search"}`,
		},
		{
			name:       "google_search",
			toolType:   "google_search",
			choice:     `{"type":"google_search"}`,
			wantName:   "web_search",
			wantChoice: `{"type":"tool","name":"web_search"}`,
		},
		{
			name:       "web_fetch",
			toolType:   "web_fetch",
			choice:     `{"type":"web_fetch"}`,
			wantName:   "web_fetch",
			wantChoice: `{"type":"tool","name":"web_fetch"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ResponsesRequest{
				Model:           "gpt-5.4",
				Input:           json.RawMessage(`"Use the server tool"`),
				MaxOutputTokens: intPtr(256),
				Tools:           []ResponsesTool{{Type: tt.toolType}},
				ToolChoice:      json.RawMessage(tt.choice),
			}

			out, err := ResponsesToAnthropicRequest(req)
			require.NoError(t, err)
			require.Len(t, out.Tools, 1)
			assert.Equal(t, tt.wantName, out.Tools[0].Name)
			require.JSONEq(t, tt.wantChoice, string(out.ToolChoice))
		})
	}
}

func TestResponsesToAnthropicRequest_DefaultsMaxOutputTokensWhenUnsetOrZero(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		maxOutputTokens *int
		wantMaxTokens   int
	}{
		{name: "unset", wantMaxTokens: 8192},
		{name: "zero", maxOutputTokens: intPtr(0), wantMaxTokens: 8192},
		{name: "negative", maxOutputTokens: intPtr(-1), wantMaxTokens: 8192},
		{name: "positive", maxOutputTokens: intPtr(256), wantMaxTokens: 256},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			req := &ResponsesRequest{
				Model:           "gpt-5.4",
				Input:           json.RawMessage(`"Hello"`),
				MaxOutputTokens: tt.maxOutputTokens,
			}

			out, err := ResponsesToAnthropicRequest(req)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMaxTokens, out.MaxTokens)
		})
	}
}

func TestResponsesToAnthropicRequest_ReturnsErrorForMalformedInput(t *testing.T) {
	t.Parallel()

	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Input: json.RawMessage(`not json`),
	}

	_, err := ResponsesToAnthropicRequest(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse responses input")
}

func TestResponsesToAnthropicRequest_ConvertsLegacyMessages(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Input: json.RawMessage(`[
			{"role":"system","content":[{"type":"input_text","text":"You are helpful."}]},
			{"role":"user","content":"Hello"},
			{"role":"assistant","content":"Sure."},
			{"role":"user","content":[
				{"type":"input_text","text":"Please help"},
				{"type":"input_image","image_url":"data:image/png;base64,abc123"}
			]}
		]`),
		MaxOutputTokens: intPtr(256),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)

	assert.JSONEq(t, `"You are helpful."`, string(out.System))
	require.Len(t, out.Messages, 3)

	assert.Equal(t, "user", out.Messages[0].Role)
	assert.JSONEq(t, `"Hello"`, string(out.Messages[0].Content))

	assert.Equal(t, "assistant", out.Messages[1].Role)
	var assistantBlocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[1].Content, &assistantBlocks))
	require.Len(t, assistantBlocks, 1)
	assert.Equal(t, "text", assistantBlocks[0].Type)
	assert.Equal(t, "Sure.", assistantBlocks[0].Text)

	assert.Equal(t, "user", out.Messages[2].Role)
	var userBlocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[2].Content, &userBlocks))
	require.Len(t, userBlocks, 2)
	assert.Equal(t, "text", userBlocks[0].Type)
	assert.Equal(t, "Please help", userBlocks[0].Text)
	require.NotNil(t, userBlocks[1].Source)
	assert.Equal(t, "image", userBlocks[1].Type)
	assert.Equal(t, "base64", userBlocks[1].Source.Type)
	assert.Equal(t, "image/png", userBlocks[1].Source.MediaType)
	assert.Equal(t, "abc123", userBlocks[1].Source.Data)
}

func TestResponsesToAnthropicRequest_MergesConsecutiveSameRoleMessages(t *testing.T) {
	t.Parallel()

	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Input: json.RawMessage(`[
			{"role":"user","content":"First"},
			{"role":"user","content":"Second"}
		]`),
		MaxOutputTokens: intPtr(256),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 1)
	assert.Equal(t, "user", out.Messages[0].Role)

	var blocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[0].Content, &blocks))
	require.Len(t, blocks, 2)
	assert.Equal(t, "text", blocks[0].Type)
	assert.Equal(t, "First", blocks[0].Text)
	assert.Equal(t, "text", blocks[1].Type)
	assert.Equal(t, "Second", blocks[1].Text)
}

func TestResponsesToAnthropicRequest_ConvertsFunctionCallOutput(t *testing.T) {
	req := &ResponsesRequest{
		Model:           "gpt-5.4",
		Input:           json.RawMessage(`[{"type":"function_call","call_id":"call_123","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"call_123","output":"done"}]`),
		MaxOutputTokens: intPtr(256),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	assertAnthropicPairing(t, out.Messages)

	require.Len(t, out.Messages, 2)
	assert.Equal(t, "user", out.Messages[1].Role)

	var blocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[1].Content, &blocks))
	require.Len(t, blocks, 1)
	assert.Equal(t, "tool_result", blocks[0].Type)
	assert.Equal(t, "call_123", blocks[0].ToolUseID)
	assert.JSONEq(t, `"done"`, string(blocks[0].Content))
}

func TestResponsesToAnthropicRequest_RestoresToolResultErrorState(t *testing.T) {
	t.Run("literal prefix stays plain text", func(t *testing.T) {
		req := &ResponsesRequest{
			Model:           "gpt-5.4",
			Input:           json.RawMessage(`[{"type":"function_call","call_id":"fc_toolu_123","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"fc_toolu_123","output":"[tool_error] tool failed"}]`),
			MaxOutputTokens: intPtr(256),
		}

		out, err := ResponsesToAnthropicRequest(req)
		require.NoError(t, err)
		assertAnthropicPairing(t, out.Messages)

		require.Len(t, out.Messages, 2)
		assert.Equal(t, "user", out.Messages[1].Role)

		var blocks []AnthropicContentBlock
		require.NoError(t, json.Unmarshal(out.Messages[1].Content, &blocks))
		require.Len(t, blocks, 1)
		assert.Equal(t, "tool_result", blocks[0].Type)
		assert.Equal(t, "toolu_123", blocks[0].ToolUseID)
		assert.False(t, blocks[0].IsError)
		assert.JSONEq(t, `"[tool_error] tool failed"`, string(blocks[0].Content))
	})

	t.Run("structured envelope", func(t *testing.T) {
		output, err := json.Marshal(anthropicToolResultEnvelope{
			Format: anthropicToolResultEnvelopeFormat,
			Text:   []string{"tool failed"},
			ToolReferences: []anthropicToolReferenceEnvelope{
				{ToolName: "WebFetch"},
			},
			IsError: true,
		})
		require.NoError(t, err)

		req := &ResponsesRequest{
			Model:           "gpt-5.4",
			Input:           json.RawMessage(`[{"type":"function_call","call_id":"fc_toolu_456","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"fc_toolu_456","output":` + strconv.Quote(string(output)) + `}]`),
			MaxOutputTokens: intPtr(256),
		}

		out, err := ResponsesToAnthropicRequest(req)
		require.NoError(t, err)
		assertAnthropicPairing(t, out.Messages)

		require.Len(t, out.Messages, 2)
		assert.Equal(t, "user", out.Messages[1].Role)

		var blocks []AnthropicContentBlock
		require.NoError(t, json.Unmarshal(out.Messages[1].Content, &blocks))
		require.Len(t, blocks, 1)
		assert.Equal(t, "tool_result", blocks[0].Type)
		assert.Equal(t, "toolu_456", blocks[0].ToolUseID)
		assert.True(t, blocks[0].IsError)

		var contentBlocks []AnthropicContentBlock
		require.NoError(t, json.Unmarshal(blocks[0].Content, &contentBlocks))
		require.Len(t, contentBlocks, 2)
		assert.Equal(t, "text", contentBlocks[0].Type)
		assert.Equal(t, "tool failed", contentBlocks[0].Text)
		assert.Equal(t, "tool_reference", contentBlocks[1].Type)
		assert.Equal(t, "WebFetch", contentBlocks[1].ToolName)
	})

}

func TestResponsesToAnthropicRequest_PreservesErrorToolResultEnvelopeWithoutTextOrReferences(t *testing.T) {
	output, err := json.Marshal(anthropicToolResultEnvelope{
		Format:  anthropicToolResultEnvelopeFormat,
		IsError: true,
	})
	require.NoError(t, err)

	req := &ResponsesRequest{
		Model:           "gpt-5.4",
		Input:           json.RawMessage(`[{"type":"function_call","call_id":"fc_toolu_789","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"fc_toolu_789","output":` + strconv.Quote(string(output)) + `}]`),
		MaxOutputTokens: intPtr(256),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	assertAnthropicPairing(t, out.Messages)

	require.Len(t, out.Messages, 2)
	assert.Equal(t, "user", out.Messages[1].Role)

	var blocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[1].Content, &blocks))
	require.Len(t, blocks, 1)
	assert.Equal(t, "tool_result", blocks[0].Type)
	assert.True(t, blocks[0].IsError)
	assert.JSONEq(t, `[]`, string(blocks[0].Content))
}

func TestResponsesToAnthropicRequest_ConvertsWebSearchCallInputToServerToolHistory(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Input: json.RawMessage(`[
			{
				"type":"web_search_call",
				"id":"search_1",
				"status":"completed",
				"action":{
					"type":"search",
					"query":"golang",
					"sources":[
						{"type":"url","url":"https://go.dev","title":"The Go Programming Language"}
					]
				}
			}
		]`),
		MaxOutputTokens: intPtr(256),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 2)

	assert.Equal(t, "assistant", out.Messages[0].Role)
	var assistantBlocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[0].Content, &assistantBlocks))
	require.Len(t, assistantBlocks, 1)
	assert.Equal(t, "server_tool_use", assistantBlocks[0].Type)
	assert.Equal(t, "srvtoolu_search_1", assistantBlocks[0].ID)
	assert.Equal(t, "web_search", assistantBlocks[0].Name)
	assert.JSONEq(t, `{"query":"golang"}`, string(assistantBlocks[0].Input))

	assert.Equal(t, "user", out.Messages[1].Role)
	var userBlocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[1].Content, &userBlocks))
	require.Len(t, userBlocks, 1)
	assert.Equal(t, "web_search_tool_result", userBlocks[0].Type)
	assert.Equal(t, "srvtoolu_search_1", userBlocks[0].ToolUseID)
	assert.JSONEq(t, `[{"type":"web_search_result","url":"https://go.dev","title":"The Go Programming Language"}]`, string(userBlocks[0].Content))
}

func TestResponsesToAnthropicRequest_WebSearchCallWithoutSourcesDoesNotInventToolResult(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Input: json.RawMessage(`[
			{
				"type":"web_search_call",
				"id":"search_native_1",
				"status":"completed",
				"action":{
					"type":"search",
					"query":"kiro native"
				}
			}
		]`),
		MaxOutputTokens: intPtr(256),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 1)
	assert.Equal(t, "assistant", out.Messages[0].Role)

	var assistantBlocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[0].Content, &assistantBlocks))
	require.Len(t, assistantBlocks, 1)
	assert.Equal(t, "server_tool_use", assistantBlocks[0].Type)
	assert.Equal(t, "srvtoolu_search_native_1", assistantBlocks[0].ID)
	assert.Equal(t, "web_search", assistantBlocks[0].Name)
	assert.JSONEq(t, `{"query":"kiro native"}`, string(assistantBlocks[0].Input))
}

func TestResponsesToAnthropicRequest_RestoresWebFetchServerToolHistoryFromFunctionCallEnvelope(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Input: json.RawMessage(`[
			{
				"type":"function_call",
				"call_id":"srvtoolu_fetch_1",
				"name":"webfetch",
				"arguments":"{\"url\":\"https://example.com\",\"_sub2api_server_tool_result\":{\"type\":\"web_fetch_result\",\"url\":\"https://example.com\",\"title\":\"Example\",\"text\":\"Hello from fetch\"}}"
			}
		]`),
		MaxOutputTokens: intPtr(256),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 2)

	assert.Equal(t, "assistant", out.Messages[0].Role)
	var assistantBlocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[0].Content, &assistantBlocks))
	require.Len(t, assistantBlocks, 1)
	assert.Equal(t, "server_tool_use", assistantBlocks[0].Type)
	assert.Equal(t, "srvtoolu_fetch_1", assistantBlocks[0].ID)
	assert.Equal(t, "web_fetch", assistantBlocks[0].Name)
	assert.JSONEq(t, `{"url":"https://example.com"}`, string(assistantBlocks[0].Input))

	assert.Equal(t, "user", out.Messages[1].Role)
	var userBlocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[1].Content, &userBlocks))
	require.Len(t, userBlocks, 1)
	assert.Equal(t, "web_fetch_tool_result", userBlocks[0].Type)
	assert.Equal(t, "srvtoolu_fetch_1", userBlocks[0].ToolUseID)
	assert.JSONEq(t, `{"type":"web_fetch_result","url":"https://example.com","title":"Example","text":"Hello from fetch"}`, string(userBlocks[0].Content))
}

func TestResponsesToAnthropicRequest_WebFetchFunctionCallWithoutResultStaysServerToolUse(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Input: json.RawMessage(`[
			{
				"type":"function_call",
				"call_id":"srvtoolu_fetch_native_1",
				"name":"webfetch",
				"arguments":"{\"url\":\"https://example.com\"}"
			}
		]`),
		MaxOutputTokens: intPtr(256),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 1)
	assert.Equal(t, "assistant", out.Messages[0].Role)

	var assistantBlocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[0].Content, &assistantBlocks))
	require.Len(t, assistantBlocks, 1)
	assert.Equal(t, "server_tool_use", assistantBlocks[0].Type)
	assert.Equal(t, "srvtoolu_fetch_native_1", assistantBlocks[0].ID)
	assert.Equal(t, "web_fetch", assistantBlocks[0].Name)
	assert.JSONEq(t, `{"url":"https://example.com"}`, string(assistantBlocks[0].Input))
}

func TestResponsesToAnthropicRequest_CustomFunctionNamedWebfetchStaysRegularToolUse(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Input: json.RawMessage(`[
			{
				"type":"function_call",
				"call_id":"call_custom_webfetch_1",
				"name":"webfetch",
				"arguments":"{\"url\":\"https://example.com\",\"mode\":\"summary\"}"
			},
			{
				"type":"function_call_output",
				"call_id":"call_custom_webfetch_1",
				"output":"fetched"
			}
		]`),
		MaxOutputTokens: intPtr(256),
		Tools: []ResponsesTool{
			{
				Type:       "function",
				Name:       "webfetch",
				Parameters: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"},"mode":{"type":"string"}}}`),
			},
		},
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	assertAnthropicPairing(t, out.Messages)
	require.Len(t, out.Messages, 2)
	assert.Equal(t, "assistant", out.Messages[0].Role)

	var assistantBlocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[0].Content, &assistantBlocks))
	require.Len(t, assistantBlocks, 1)
	assert.Equal(t, "tool_use", assistantBlocks[0].Type)
	assert.Equal(t, "call_custom_webfetch_1", assistantBlocks[0].ID)
	assert.Equal(t, "webfetch", assistantBlocks[0].Name)
	assert.JSONEq(t, `{"url":"https://example.com","mode":"summary"}`, string(assistantBlocks[0].Input))

	assert.Equal(t, "user", out.Messages[1].Role)
	var userBlocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[1].Content, &userBlocks))
	require.Len(t, userBlocks, 1)
	assert.Equal(t, "tool_result", userBlocks[0].Type)
	assert.Equal(t, "call_custom_webfetch_1", userBlocks[0].ToolUseID)
	assert.JSONEq(t, `"fetched"`, string(userBlocks[0].Content))
}

func intPtr(v int) *int {
	return &v
}
