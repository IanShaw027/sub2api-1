package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// AnthropicToResponses tests
// ---------------------------------------------------------------------------

func TestAnthropicToResponses_BasicText(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Stream:    true,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	assert.Equal(t, "gpt-5.2", resp.Model)
	assert.True(t, resp.Stream)
	assert.Equal(t, 1024, *resp.MaxOutputTokens)
	assert.False(t, *resp.Store)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)
	assert.Equal(t, "user", items[0].Role)
}

func TestAnthropicToResponses_ReportsDroppedCacheControlWithoutSerializingSidecar(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 128,
		System: json.RawMessage(`[
			{"type":"text","text":"system","cache_control":{"type":"ephemeral","ttl":"1h"}}
		]`),
		Messages: []AnthropicMessage{{
			Role: "user",
			Content: json.RawMessage(`[
				{"type":"tool_result","tool_use_id":"toolu_1","content":[
					{"type":"text","text":"nested","cache_control":{"type":"ephemeral"}}
				]}
			]`),
		}},
		Tools: []AnthropicTool{{
			Name:         "lookup",
			InputSchema:  json.RawMessage(`{"type":"object"}`),
			CacheControl: &AnthropicCacheControl{Type: "ephemeral"},
		}},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	require.Equal(t, []string{"cache_control"}, resp.DroppedCompatibilityFields)

	wire, err := json.Marshal(resp)
	require.NoError(t, err)
	require.NotContains(t, string(wire), "DroppedCompatibilityFields")
	require.NotContains(t, string(wire), "cache_control")
	require.NotContains(t, string(wire), "prompt_cache_breakpoint")
}

func TestAnthropicToResponsesForModel_MapsSupportedCacheControlBreakpoints(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 128,
		System: json.RawMessage(`[
			{"type":"text","text":"stable system","cache_control":{"type":"ephemeral"}},
			{"type":"text","text":"dynamic system"}
		]`),
		Messages: []AnthropicMessage{{
			Role: "user",
			Content: json.RawMessage(`[
				{"type":"text","text":"stable user","cache_control":{"type":"ephemeral"}},
				{"type":"image","source":{"type":"base64","media_type":"image/png","data":"aGVsbG8="},"cache_control":{"type":"ephemeral"}}
			]`),
		}},
	}

	resp, err := AnthropicToResponsesForModel(req, "gpt-5.6-sol")
	require.NoError(t, err)
	require.Equal(t, "gpt-5.6-sol", resp.Model)
	require.Empty(t, resp.DroppedCompatibilityFields)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 2)

	var systemParts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[0].Content, &systemParts))
	require.Len(t, systemParts, 2)
	require.Equal(t, "explicit", systemParts[0].PromptCacheBreakpoint.Mode)
	require.Nil(t, systemParts[1].PromptCacheBreakpoint)

	var userParts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[1].Content, &userParts))
	require.Len(t, userParts, 2)
	require.Equal(t, "explicit", userParts[0].PromptCacheBreakpoint.Mode)
	require.Equal(t, "explicit", userParts[1].PromptCacheBreakpoint.Mode)

	wire, err := json.Marshal(resp)
	require.NoError(t, err)
	require.Contains(t, string(wire), `"prompt_cache_breakpoint":{"mode":"explicit"}`)
	require.NotContains(t, string(wire), "cache_control")
}

func TestAnthropicToResponsesForModel_ReportsUnmappedCacheControlSemantics(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 128,
		Messages: []AnthropicMessage{{
			Role: "user",
			Content: json.RawMessage(`[
				{"type":"text","text":"ttl differs","cache_control":{"type":"ephemeral","ttl":"1h"}},
				{"type":"tool_result","tool_use_id":"toolu_1","content":"ok","cache_control":{"type":"ephemeral"}}
			]`),
		}},
		Tools: []AnthropicTool{{
			Name:         "lookup",
			InputSchema:  json.RawMessage(`{"type":"object"}`),
			CacheControl: &AnthropicCacheControl{Type: "ephemeral"},
		}},
	}

	resp, err := AnthropicToResponsesForModel(req, "openai/gpt-5.6")
	require.NoError(t, err)
	require.Equal(t, []string{"cache_control"}, resp.DroppedCompatibilityFields)

	wire, err := json.Marshal(resp)
	require.NoError(t, err)
	require.Contains(t, string(wire), `"prompt_cache_breakpoint":{"mode":"explicit"}`)
	require.NotContains(t, string(wire), "cache_control")
}

func TestSupportsResponsesPromptCacheBreakpoints(t *testing.T) {
	tests := map[string]bool{
		"gpt-5.5":           false,
		"gpt-5.6":           true,
		"gpt-5.6-sol":       true,
		"openai/gpt-5.6":    true,
		"gpt-6":             true,
		"gpt-6.1-latest":    true,
		"gpt-oss-120b":      false,
		"claude-sonnet-4-6": false,
	}
	for model, want := range tests {
		t.Run(model, func(t *testing.T) {
			require.Equal(t, want, supportsResponsesPromptCacheBreakpoints(model))
		})
	}
}

func TestAnthropicToResponses_UserPlainTextCanonicalizesAcrossAnthropicShapes(t *testing.T) {
	stringReq := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 128,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"好的，继续"`)},
		},
	}
	blockReq := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 128,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`[{"type":"text","text":"好的，继续"}]`)},
		},
	}

	stringResp, err := AnthropicToResponses(stringReq)
	require.NoError(t, err)
	blockResp, err := AnthropicToResponses(blockReq)
	require.NoError(t, err)

	var stringItems []ResponsesInputItem
	require.NoError(t, json.Unmarshal(stringResp.Input, &stringItems))
	require.Len(t, stringItems, 1)

	var blockItems []ResponsesInputItem
	require.NoError(t, json.Unmarshal(blockResp.Input, &blockItems))
	require.Len(t, blockItems, 1)

	require.Equal(t, stringItems[0].Role, blockItems[0].Role)
	require.Equal(t, stringItems[0].Content, blockItems[0].Content)
}

func TestAnthropicToResponses_SystemPrompt(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		req := &AnthropicRequest{
			Model:     "gpt-5.2",
			MaxTokens: 100,
			System:    json.RawMessage(`"You are helpful."`),
			Messages:  []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hi"`)}},
		}
		resp, err := AnthropicToResponses(req)
		require.NoError(t, err)

		var items []ResponsesInputItem
		require.NoError(t, json.Unmarshal(resp.Input, &items))
		require.Len(t, items, 2)
		assert.Equal(t, "system", items[0].Role)
	})

	t.Run("array", func(t *testing.T) {
		req := &AnthropicRequest{
			Model:     "gpt-5.2",
			MaxTokens: 100,
			System:    json.RawMessage(`[{"type":"text","text":"Part 1"},{"type":"text","text":"Part 2"}]`),
			Messages:  []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hi"`)}},
		}
		resp, err := AnthropicToResponses(req)
		require.NoError(t, err)

		var items []ResponsesInputItem
		require.NoError(t, json.Unmarshal(resp.Input, &items))
		require.Len(t, items, 2)
		assert.Equal(t, "system", items[0].Role)
		// System text should be joined with double newline.
		var text string
		require.NoError(t, json.Unmarshal(items[0].Content, &text))
		assert.Equal(t, "Part 1\n\nPart 2", text)
	})
}

func TestAnthropicToResponses_ToolUse(t *testing.T) {
	strict := true
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"What is the weather?"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"text","text":"Let me check."},{"type":"tool_use","id":"call_1","name":"get_weather","input":{"city":"NYC"}}]`)},
			{Role: "user", Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"call_1","content":"Sunny, 72°F"}]`)},
		},
		Tools: []AnthropicTool{
			{Name: "get_weather", Description: "Get weather", InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`), Strict: &strict},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	// Check tools
	require.Len(t, resp.Tools, 1)
	assert.Equal(t, "function", resp.Tools[0].Type)
	assert.Equal(t, "get_weather", resp.Tools[0].Name)
	require.NotNil(t, resp.Tools[0].Strict)
	assert.True(t, *resp.Tools[0].Strict)

	// Check input items
	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	// user + assistant + function_call + function_call_output = 4
	require.Len(t, items, 4)

	assert.Equal(t, "user", items[0].Role)
	assert.Equal(t, "assistant", items[1].Role)
	assert.Equal(t, "function_call", items[2].Type)
	assert.Equal(t, "call_1", items[2].CallID)
	assert.Empty(t, items[2].ID)
	assert.Equal(t, "function_call_output", items[3].Type)
	assert.Equal(t, "call_1", items[3].CallID)
	assert.Equal(t, "Sunny, 72°F", items[3].Output)
}

func TestAnthropicToResponses_ToolUseJSONStringInputUnwrapsBackToOriginalArguments(t *testing.T) {
	content, err := json.Marshal([]AnthropicContentBlock{{
		Type:  "tool_use",
		ID:    "call_1",
		Name:  "get_weather",
		Input: anthropicToolUseInputFromArguments(`{"city":`),
	}})
	require.NoError(t, err)

	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "assistant", Content: content},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)
	assert.Equal(t, "function_call", items[0].Type)
	assert.Equal(t, `{"city":`, items[0].Arguments)
}

func TestAnthropicToResponses_MapsWebFetchToolDefinition(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.4",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"Fetch https://example.com"`)},
		},
		Tools: []AnthropicTool{
			{Type: "web_fetch_20250910", Name: "web_fetch"},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	require.Len(t, resp.Tools, 1)
	assert.Equal(t, "web_fetch", resp.Tools[0].Type)
	assert.Empty(t, resp.Tools[0].Name)
	assert.Empty(t, resp.Tools[0].Parameters)
}

func TestAnthropicToResponses_PreservesToolReferenceInToolResult(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"toolu_123","content":[{"type":"text","text":"candidate tools"},{"type":"tool_reference","tool_name":"WebFetch"},{"type":"tool_reference","tool_name":"AskUserQuestion"}]}]`),
			},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)
	require.Equal(t, "function_call_output", items[0].Type)
	require.Contains(t, items[0].Output, "tool_references")
	require.Contains(t, items[0].Output, "WebFetch")
	require.Contains(t, items[0].Output, "AskUserQuestion")
}

func TestAnthropicToResponses_ToolResultWithoutToolReferenceStaysPlain(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"toolu_123","content":[{"type":"text","text":"plain output"}]}]`),
			},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)
	require.Equal(t, "function_call_output", items[0].Type)
	require.Equal(t, "plain output", items[0].Output)
}

func TestAnthropicToResponses_ToolResultErrorUsesEnvelope(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"toolu_123","is_error":true,"content":"[tool_error] tool failed"}]`),
			},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)
	require.Equal(t, "function_call_output", items[0].Type)

	var env anthropicToolResultEnvelope
	require.NoError(t, json.Unmarshal([]byte(items[0].Output), &env))
	require.Equal(t, anthropicToolResultEnvelopeFormat, env.Format)
	require.True(t, env.IsError)
	require.Equal(t, []string{"[tool_error] tool failed"}, env.Text)
}

func TestAnthropicToResponses_PreservesToolResultErrorEnvelope(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"toolu_123","is_error":true,"content":[{"type":"text","text":"tool failed"},{"type":"tool_reference","tool_name":"WebFetch"}]}]`),
			},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)
	require.Equal(t, "function_call_output", items[0].Type)

	var env anthropicToolResultEnvelope
	require.NoError(t, json.Unmarshal([]byte(items[0].Output), &env))
	require.Equal(t, anthropicToolResultEnvelopeFormat, env.Format)
	require.True(t, env.IsError)
	require.Equal(t, []string{"tool failed"}, env.Text)
	require.Len(t, env.ToolReferences, 1)
	require.Equal(t, "WebFetch", env.ToolReferences[0].ToolName)
}

func TestAnthropicToResponses_PreservesToolResultErrorEnvelopeWithOnlyImages(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"toolu_123","is_error":true,"content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"aGVsbG8="}}]}]`),
			},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 2)
	require.Equal(t, "function_call_output", items[0].Type)

	var env anthropicToolResultEnvelope
	require.NoError(t, json.Unmarshal([]byte(items[0].Output), &env))
	require.Equal(t, anthropicToolResultEnvelopeFormat, env.Format)
	require.True(t, env.IsError)
	require.Empty(t, env.Text)
	require.Empty(t, env.ToolReferences)
}

func TestAnthropicToResponses_ToolResultWithToolReferenceStillEmitsImages(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"toolu_123","content":[{"type":"text","text":"candidate tools"},{"type":"tool_reference","tool_name":"WebFetch"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"aGVsbG8="}}]}]`),
			},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 2)
	require.Equal(t, "function_call_output", items[0].Type)
	require.Equal(t, "user", items[1].Role)
	require.Contains(t, string(items[1].Content), "input_image")
}

func TestAnthropicToResponses_ThinkingIgnored(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"thinking","thinking":"deep thought"},{"type":"text","text":"Hi!"}]`)},
			{Role: "user", Content: json.RawMessage(`"More"`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	// user + assistant(text only, thinking ignored) + user = 3
	require.Len(t, items, 3)
	assert.Equal(t, "assistant", items[1].Role)
	// Assistant content should only have text, not thinking.
	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[1].Content, &parts))
	require.Len(t, parts, 1)
	assert.Equal(t, "output_text", parts[0].Type)
	assert.Equal(t, "Hi!", parts[0].Text)
}

func TestAnthropicToResponses_MaxTokensFloor(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 10, // below minMaxOutputTokens (128)
		Messages:  []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hi"`)}},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	assert.Equal(t, 128, *resp.MaxOutputTokens)
}

// ---------------------------------------------------------------------------
// ResponsesToAnthropic (non-streaming) tests
// ---------------------------------------------------------------------------

func TestResponsesToAnthropic_TextOnly(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_123",
		Model:  "gpt-5.2",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type: "message",
				Content: []ResponsesContentPart{
					{Type: "output_text", Text: "Hello there!"},
				},
			},
		},
		Usage: &ResponsesUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")
	assert.Equal(t, "resp_123", anth.ID)
	assert.Equal(t, "claude-opus-4-6", anth.Model)
	assert.Equal(t, "end_turn", anth.StopReason)
	require.Len(t, anth.Content, 1)
	assert.Equal(t, "text", anth.Content[0].Type)
	assert.Equal(t, "Hello there!", anth.Content[0].Text)
	assert.Equal(t, 10, anth.Usage.InputTokens)
	assert.Equal(t, 5, anth.Usage.OutputTokens)
	assert.JSONEq(t, `null`, string(anth.Container))
	assert.JSONEq(t, `null`, string(anth.ContextManagement))
	assert.Equal(t, "standard", anth.Usage.ServiceTier)
	assert.Equal(t, "standard", anth.Usage.Speed)
	require.NotNil(t, anth.Usage.CacheCreation)
	assert.Equal(t, 0, anth.Usage.CacheCreation.Ephemeral5mInputTokens)
	assert.Equal(t, 0, anth.Usage.CacheCreation.Ephemeral1hInputTokens)
	assert.Empty(t, anth.Usage.Iterations)
}

func TestResponsesToAnthropic_ToolUse(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_456",
		Model:  "gpt-5.2",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type: "message",
				Content: []ResponsesContentPart{
					{Type: "output_text", Text: "Let me check."},
				},
			},
			{
				Type:      "function_call",
				CallID:    "call_1",
				Name:      "get_weather",
				Arguments: `{"city":"NYC"}`,
			},
		},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6", nil)
	assert.Equal(t, "tool_use", anth.StopReason)
	require.Len(t, anth.Content, 2)
	assert.Equal(t, "text", anth.Content[0].Type)
	assert.Equal(t, "tool_use", anth.Content[1].Type)
	assert.Equal(t, "call_1", anth.Content[1].ID)
	assert.Equal(t, "get_weather", anth.Content[1].Name)
}

func TestResponsesToAnthropic_ToolUseStopReasonPersistsWhenTrailingTextFollows(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_tool_use_then_text",
		Model:  "gpt-5.2",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type:      "function_call",
				CallID:    "call_1",
				Name:      "get_weather",
				Arguments: `{"city":"NYC"}`,
			},
			{
				Type: "message",
				Content: []ResponsesContentPart{
					{Type: "output_text", Text: "Let me know once the tool returns."},
				},
			},
		},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6", nil)
	assert.Equal(t, "tool_use", anth.StopReason)
	require.Len(t, anth.Content, 2)
	assert.Equal(t, "tool_use", anth.Content[0].Type)
	assert.Equal(t, "text", anth.Content[1].Type)
}

func TestResponsesToAnthropic_CustomToolCallBecomesToolUse(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_custom_tool",
		Model:  "gpt-5.2",
		Status: "completed",
		Output: []ResponsesOutput{{
			Type:   "custom_tool_call",
			ID:     "ct_1",
			CallID: "call_custom",
			Name:   "apply_patch",
			Input:  "*** Begin Patch",
		}},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")

	require.Equal(t, "tool_use", anth.StopReason)
	require.Len(t, anth.Content, 1)
	require.Equal(t, "tool_use", anth.Content[0].Type)
	require.Equal(t, "call_custom", anth.Content[0].ID)
	require.Equal(t, "apply_patch", anth.Content[0].Name)
	require.JSONEq(t, `"*** Begin Patch"`, string(anth.Content[0].Input))
}

func TestResponsesToAnthropic_ToolSearchCallBecomesToolUse(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_tool_search",
		Model:  "gpt-5.2",
		Status: "completed",
		Output: []ResponsesOutput{{
			Type:      "tool_search_call",
			ID:        "ts_1",
			CallID:    "call_search",
			Arguments: `{"query":"docs","limit":3}`,
		}},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")

	require.Equal(t, "tool_use", anth.StopReason)
	require.Len(t, anth.Content, 1)
	require.Equal(t, "tool_use", anth.Content[0].Type)
	require.Equal(t, "call_search", anth.Content[0].ID)
	require.Equal(t, "tool_search", anth.Content[0].Name)
	require.JSONEq(t, `{"query":"docs","limit":3}`, string(anth.Content[0].Input))
}

func TestResponsesToAnthropic_ToolUseNormalizesEmptyArgumentsToEmptyObject(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_456",
		Model:  "gpt-5.2",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type:      "function_call",
				CallID:    "call_1",
				Name:      "get_weather",
				Arguments: "",
			},
		},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")

	require.Len(t, anth.Content, 1)
	require.Equal(t, "tool_use", anth.Content[0].Type)
	require.True(t, json.Valid(anth.Content[0].Input))
	require.JSONEq(t, `{}`, string(anth.Content[0].Input))
	_, err := json.Marshal(anth)
	require.NoError(t, err)
}

func TestResponsesToAnthropic_ToolUsePreservesInvalidArgumentsAsJSONString(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_456",
		Model:  "gpt-5.2",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type:      "function_call",
				CallID:    "call_1",
				Name:      "get_weather",
				Arguments: `{"city":`,
			},
		},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")

	require.Len(t, anth.Content, 1)
	require.Equal(t, "tool_use", anth.Content[0].Type)
	require.True(t, json.Valid(anth.Content[0].Input))
	require.Equal(t, `"{\"city\":"`, string(anth.Content[0].Input))
	_, err := json.Marshal(anth)
	require.NoError(t, err)
}

func TestClaudeToolNameMapFromTools_PreservesOriginalClaudeToolName(t *testing.T) {
	nameMap := ClaudeToolNameMapFromTools([]ResponsesTool{
		{Type: "function", Name: "__ReadFile"},
	})

	assert.Equal(t, "__ReadFile", MapClaudeToolName("readfile", nameMap))
}

func TestClaudeToolNameMapFromTools_CanonicalCollisionKeepsFirstSeenName(t *testing.T) {
	nameMap := ClaudeToolNameMapFromTools([]ResponsesTool{
		{Type: "function", Name: "__ReadFile"},
		{Type: "function", Name: "_readfile"},
		{Type: "function", Name: "readfile"},
	})

	assert.Equal(t, "__ReadFile", MapClaudeToolName("readfile", nameMap))
	assert.Equal(t, "__ReadFile", MapClaudeToolName("__ReadFile", nameMap))
}

func TestResponsesToAnthropic_ToolUse_RestoresOriginalClaudeToolName(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_456",
		Model:  "gpt-5.2",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type: "message",
				Content: []ResponsesContentPart{
					{Type: "output_text", Text: "Let me check."},
				},
			},
			{
				Type:      "function_call",
				CallID:    "call_1",
				Name:      "readfile",
				Arguments: `{"path":"/tmp/demo.txt"}`,
			},
		},
	}

	nameMap := ClaudeToolNameMapFromTools([]ResponsesTool{
		{Type: "function", Name: "__ReadFile"},
	})

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6", nameMap)
	require.Len(t, anth.Content, 2)
	assert.Equal(t, "__ReadFile", anth.Content[1].Name)
}

func TestResponsesToAnthropic_WebSearchUsesMessageAnnotations(t *testing.T) {
	var resp ResponsesResponse
	require.NoError(t, json.Unmarshal([]byte(`{
		"id": "resp_ws_annotations",
		"model": "gpt-5.4",
		"status": "completed",
		"output": [
			{
				"type": "web_search_call",
				"id": "ws_1",
				"status": "completed",
				"action": {
					"type": "search",
					"query": "latest news"
				}
			},
			{
				"type": "message",
				"id": "msg_1",
				"status": "completed",
				"role": "assistant",
				"content": [
					{
						"type": "output_text",
						"text": "Result with citation",
						"annotations": [
							{
								"type": "url_citation",
								"url": "https://example.com/a",
								"title": "Example A",
								"start_index": 0,
								"end_index": 6
							}
						]
					}
				]
			}
		]
	}`), &resp))

	anth := ResponsesToAnthropic(&resp, "claude-opus-4-6")
	require.Len(t, anth.Content, 3)
	assert.Equal(t, "server_tool_use", anth.Content[0].Type)
	assert.Equal(t, "web_search_tool_result", anth.Content[1].Type)

	var results []map[string]any
	require.NoError(t, json.Unmarshal(anth.Content[1].Content, &results))
	require.NotEmpty(t, results)
	assert.Equal(t, "web_search_result", results[0]["type"])
	assert.Equal(t, "https://example.com/a", results[0]["url"])
	assert.Equal(t, "Example A", results[0]["title"])
}

func TestResponsesToAnthropic_WebSearchUsesActionSources(t *testing.T) {
	var resp ResponsesResponse
	require.NoError(t, json.Unmarshal([]byte(`{
		"id": "resp_ws_sources",
		"model": "gpt-5.4",
		"status": "completed",
		"output": [
			{
				"type": "web_search_call",
				"id": "ws_1",
				"status": "completed",
				"action": {
					"type": "search",
					"query": "latest news",
					"sources": [
						{
							"type": "url",
							"url": "https://example.com/source",
							"title": "Example Source"
						}
					]
				}
			}
		]
	}`), &resp))

	anth := ResponsesToAnthropic(&resp, "claude-opus-4-6")
	require.Len(t, anth.Content, 2)
	assert.Equal(t, "web_search_tool_result", anth.Content[1].Type)

	var results []map[string]any
	require.NoError(t, json.Unmarshal(anth.Content[1].Content, &results))
	require.NotEmpty(t, results)
	assert.Equal(t, "https://example.com/source", results[0]["url"])
	assert.Equal(t, "Example Source", results[0]["title"])
}

func TestResponsesToAnthropic_WebSearchUsesEmptyArrayWhenNoResultsAvailable(t *testing.T) {
	var resp ResponsesResponse
	require.NoError(t, json.Unmarshal([]byte(`{
		"id": "resp_ws_empty",
		"model": "gpt-5.4",
		"status": "completed",
		"output": [
			{
				"type": "web_search_call",
				"id": "ws_1",
				"status": "completed",
				"action": {
					"type": "search",
					"query": "latest news"
				}
			}
		]
	}`), &resp))

	anth := ResponsesToAnthropic(&resp, "claude-opus-4-6")
	require.Len(t, anth.Content, 2)
	assert.Equal(t, "web_search_tool_result", anth.Content[1].Type)
	assert.JSONEq(t, `[]`, string(anth.Content[1].Content))
}

func TestResponsesToAnthropic_Reasoning(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_789",
		Model:  "gpt-5.2",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type: "reasoning",
				Summary: []ResponsesSummary{
					{Type: "summary_text", Text: "Thinking about the answer..."},
				},
			},
			{
				Type: "message",
				Content: []ResponsesContentPart{
					{Type: "output_text", Text: "42"},
				},
			},
		},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")
	require.Len(t, anth.Content, 2)
	assert.Equal(t, "thinking", anth.Content[0].Type)
	assert.Equal(t, "Thinking about the answer...", anth.Content[0].Thinking)
	assert.Equal(t, "text", anth.Content[1].Type)
	assert.Equal(t, "42", anth.Content[1].Text)
}

func TestResponsesToAnthropic_Incomplete(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_inc",
		Model:  "gpt-5.2",
		Status: "incomplete",
		IncompleteDetails: &ResponsesIncompleteDetails{
			Reason: "max_output_tokens",
		},
		Output: []ResponsesOutput{
			{
				Type:    "message",
				Content: []ResponsesContentPart{{Type: "output_text", Text: "Partial..."}},
			},
		},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")
	assert.Equal(t, "max_tokens", anth.StopReason)
}

func TestResponsesToAnthropic_EmptyOutput(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_empty",
		Model:  "gpt-5.2",
		Status: "completed",
		Output: []ResponsesOutput{},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")
	require.Len(t, anth.Content, 1)
	assert.Equal(t, "text", anth.Content[0].Type)
	assert.Equal(t, "", anth.Content[0].Text)
}

func TestResponsesToAnthropic_RefusalContentMapsStopReason(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_refusal",
		Model:  "gpt-5.4",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type: "message",
				Content: []ResponsesContentPart{
					{Type: "refusal", Refusal: "I’m sorry, I can’t help with that."},
				},
			},
		},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")
	assert.Equal(t, "refusal", anth.StopReason)
	require.NotNil(t, anth.StopDetails)
	assert.Equal(t, "refusal", anth.StopDetails.Type)
	require.NotNil(t, anth.StopDetails.Explanation)
	assert.Equal(t, "I’m sorry, I can’t help with that.", *anth.StopDetails.Explanation)
	require.Len(t, anth.Content, 1)
	assert.Equal(t, "text", anth.Content[0].Type)
	assert.Equal(t, "I’m sorry, I can’t help with that.", anth.Content[0].Text)
}

func TestResponsesToAnthropic_ContentFilterIncompleteMapsToRefusal(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_content_filter",
		Model:  "gpt-5.4",
		Status: "incomplete",
		Output: []ResponsesOutput{},
		IncompleteDetails: &ResponsesIncompleteDetails{
			Reason: "content_filter",
		},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")
	assert.Equal(t, "refusal", anth.StopReason)
	require.NotNil(t, anth.StopDetails)
	assert.Equal(t, "content blocked by upstream policy", *anth.StopDetails.Explanation)
}

func TestResponsesToAnthropic_FailedContentFilterMapsToRefusal(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_failed_filter",
		Model:  "gpt-5.4",
		Status: "failed",
		Error: &ResponsesError{
			Code:    "content_filter",
			Message: "Request blocked by content policy",
		},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")
	assert.Equal(t, "refusal", anth.StopReason)
	require.NotNil(t, anth.StopDetails)
	require.NotNil(t, anth.StopDetails.Explanation)
	assert.Equal(t, "Request blocked by content policy", *anth.StopDetails.Explanation)
}

func TestResponsesToAnthropic_ModelContextWindowExceededMapsStopReason(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_ctx_limit",
		Model:  "gpt-5.4",
		Status: "incomplete",
		Output: []ResponsesOutput{
			{
				Type: "message",
				Content: []ResponsesContentPart{
					{Type: "output_text", Text: "Partial answer"},
				},
			},
		},
		IncompleteDetails: &ResponsesIncompleteDetails{
			Reason: "model_context_window_exceeded",
		},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")
	assert.Equal(t, "model_context_window_exceeded", anth.StopReason)
}

func TestResponsesToAnthropicRequest_RestoresToolReferenceEnvelope(t *testing.T) {
	input, err := json.Marshal([]ResponsesInputItem{
		{
			Type:      "function_call",
			CallID:    "call_123",
			Name:      "lookup",
			Arguments: "{}",
		},
		{
			Type:   "function_call_output",
			CallID: "call_123",
			Output: `{"sub2api_format":"anthropic_tool_result_v1","text":["candidate tools"],"tool_references":[{"tool_name":"WebFetch"},{"tool_name":"AskUserQuestion"}]}`,
		},
	})
	require.NoError(t, err)

	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Input: input,
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	assertAnthropicPairing(t, out.Messages)
	require.Len(t, out.Messages, 2)

	var blocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[1].Content, &blocks))
	require.Len(t, blocks, 1)
	require.Equal(t, "tool_result", blocks[0].Type)

	var inner []map[string]any
	require.NoError(t, json.Unmarshal(blocks[0].Content, &inner))
	require.Len(t, inner, 3)
	require.Equal(t, "text", inner[0]["type"])
	require.Equal(t, "candidate tools", inner[0]["text"])
	require.Equal(t, "tool_reference", inner[1]["type"])
	require.Equal(t, "WebFetch", inner[1]["tool_name"])
	require.Equal(t, "tool_reference", inner[2]["type"])
	require.Equal(t, "AskUserQuestion", inner[2]["tool_name"])
}

func TestResponsesToAnthropicRequest_RestoresDocumentBlocks(t *testing.T) {
	input, err := json.Marshal([]ResponsesInputItem{
		{
			Role: "user",
			Content: json.RawMessage(`[
				{"type":"input_file","file_data":"JVBERi0x","filename":"report.pdf"},
				{"type":"input_file","file_url":"https://example.com/report.pdf","filename":"remote.pdf"},
				{"type":"input_file","file_id":"file_abc123","filename":"stored.pdf"},
				{"type":"input_text","text":"Summarize these files"}
			]`),
		},
	})
	require.NoError(t, err)

	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Input: input,
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 1)

	var blocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(out.Messages[0].Content, &blocks))
	require.Len(t, blocks, 4)
	require.Equal(t, "document", blocks[0].Type)
	require.Equal(t, "report.pdf", blocks[0].Title)
	require.NotNil(t, blocks[0].Source)
	require.Equal(t, "base64", blocks[0].Source.Type)
	require.Equal(t, "JVBERi0x", blocks[0].Source.Data)
	require.Equal(t, "document", blocks[1].Type)
	require.Equal(t, "url", blocks[1].Source.Type)
	require.Equal(t, "https://example.com/report.pdf", blocks[1].Source.URL)
	require.Equal(t, "document", blocks[2].Type)
	require.Equal(t, "file", blocks[2].Source.Type)
	require.Equal(t, "file_abc123", blocks[2].Source.FileID)
	require.Equal(t, "text", blocks[3].Type)
	require.Equal(t, "Summarize these files", blocks[3].Text)
}

func TestResponsesToAnthropicRequest_RestoresDisableParallelToolUse(t *testing.T) {
	req := &ResponsesRequest{
		Model:      "gpt-5.4",
		Input:      json.RawMessage(`[]`),
		ToolChoice: json.RawMessage(`"required"`),
		ParallelToolCalls: func() *bool {
			v := false
			return &v
		}(),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)

	var toolChoice map[string]any
	require.NoError(t, json.Unmarshal(out.ToolChoice, &toolChoice))
	require.Equal(t, "any", toolChoice["type"])
	require.Equal(t, true, toolChoice["disable_parallel_tool_use"])
}

// ---------------------------------------------------------------------------
// Streaming: ResponsesEventToAnthropicEvents tests
// ---------------------------------------------------------------------------

func TestStreamingTextOnly(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	// 1. response.created
	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.created",
		Response: &ResponsesResponse{
			ID:    "resp_1",
			Model: "gpt-5.2",
		},
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "message_start", events[0].Type)

	// 2. output_item.added (message)
	events = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item:        &ResponsesOutput{Type: "message"},
	}, state)
	assert.Len(t, events, 0) // message item doesn't emit events

	// 3. text delta
	events = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:  "response.output_text.delta",
		Delta: "Hello",
	}, state)
	require.Len(t, events, 2) // content_block_start + content_block_delta
	assert.Equal(t, "content_block_start", events[0].Type)
	assert.Equal(t, "text", events[0].ContentBlock.Type)
	assert.Equal(t, "content_block_delta", events[1].Type)
	assert.Equal(t, "text_delta", events[1].Delta.Type)
	assert.Equal(t, "Hello", events[1].Delta.Text)

	// 4. more text
	events = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:  "response.output_text.delta",
		Delta: " world",
	}, state)
	require.Len(t, events, 1) // only delta, no new block start
	assert.Equal(t, "content_block_delta", events[0].Type)

	// 5. text done
	events = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.output_text.done",
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "content_block_stop", events[0].Type)

	// 6. completed
	events = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
			Usage:  &ResponsesUsage{InputTokens: 10, OutputTokens: 5},
		},
	}, state)
	require.Len(t, events, 2) // message_delta + message_stop
	assert.Equal(t, "message_delta", events[0].Type)
	assert.Equal(t, "end_turn", events[0].Delta.StopReason)
	assert.Equal(t, 10, events[0].Usage.InputTokens)
	assert.Equal(t, 5, events[0].Usage.OutputTokens)
	assert.Equal(t, "message_stop", events[1].Type)
}

func TestResponsesEventToAnthropicEvents_ResponseDone(t *testing.T) {
	state := NewResponsesEventToAnthropicState()
	state.Model = "gpt-4o"

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.done",
		Response: &ResponsesResponse{
			Status: "completed",
			Usage:  &ResponsesUsage{InputTokens: 12, OutputTokens: 4},
		},
	}, state)
	require.Len(t, events, 2)
	assert.Equal(t, "message_delta", events[0].Type)
	assert.Equal(t, "end_turn", events[0].Delta.StopReason)
	assert.Equal(t, 12, events[0].Usage.InputTokens)
	assert.Equal(t, 4, events[0].Usage.OutputTokens)
	assert.Equal(t, "message_stop", events[1].Type)
	assert.Nil(t, FinalizeResponsesAnthropicStream(state))
}

func TestResponsesEventToAnthropicEvents_TopLevelTerminalUsage(t *testing.T) {
	state := NewResponsesEventToAnthropicState()
	state.Model = "gpt-4o"

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
		},
		Usage: &ResponsesUsage{
			InputTokens:  20,
			OutputTokens: 6,
			InputTokensDetails: &ResponsesInputTokensDetails{
				CachedTokens: 5,
			},
		},
	}, state)

	require.Len(t, events, 2)
	assert.Equal(t, "message_delta", events[0].Type)
	require.NotNil(t, events[0].Usage)
	assert.Equal(t, 15, events[0].Usage.InputTokens)
	assert.Equal(t, 5, events[0].Usage.CacheReadInputTokens)
	assert.Equal(t, 6, events[0].Usage.OutputTokens)
	assert.Equal(t, "message_stop", events[1].Type)
}

func TestResponsesEventToAnthropicEvents_ResponseDoneIncomplete(t *testing.T) {
	state := NewResponsesEventToAnthropicState()
	state.Model = "gpt-4o"

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.done",
		Response: &ResponsesResponse{
			Status:            "incomplete",
			IncompleteDetails: &ResponsesIncompleteDetails{Reason: "max_output_tokens"},
			Usage:             &ResponsesUsage{InputTokens: 12, OutputTokens: 4},
		},
	}, state)
	require.Len(t, events, 2)
	assert.Equal(t, "message_delta", events[0].Type)
	assert.Equal(t, "max_tokens", events[0].Delta.StopReason)
	assert.Equal(t, "message_stop", events[1].Type)
	assert.Nil(t, FinalizeResponsesAnthropicStream(state))
}

func TestStreamingCachedTokensUseAnthropicInputSemantics(t *testing.T) {
	state := NewResponsesEventToAnthropicState()
	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_cached_stream", Model: "gpt-5.2"},
	}, state)

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
			Usage: &ResponsesUsage{
				InputTokens:  54006,
				OutputTokens: 123,
				TotalTokens:  54129,
				InputTokensDetails: &ResponsesInputTokensDetails{
					CachedTokens: 50688,
				},
			},
		},
	}, state)

	require.Len(t, events, 2)
	assert.Equal(t, "message_delta", events[0].Type)
	assert.Equal(t, 3318, events[0].Usage.InputTokens)
	assert.Equal(t, 50688, events[0].Usage.CacheReadInputTokens)
	assert.Equal(t, 123, events[0].Usage.OutputTokens)
	assert.Equal(t, "message_stop", events[1].Type)
}
func TestStreamingToolCall(t *testing.T) {
	state := NewResponsesEventToAnthropicState()
	state.ToolNameMap = ClaudeToolNameMapFromTools([]ResponsesTool{
		{Type: "function", Name: "__ReadFile"},
	})

	// 1. response.created
	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_2", Model: "gpt-5.2"},
	}, state)

	// 2. function_call added
	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item:        &ResponsesOutput{Type: "function_call", CallID: "call_1", Name: "readfile"},
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "content_block_start", events[0].Type)
	assert.Equal(t, "tool_use", events[0].ContentBlock.Type)
	assert.Equal(t, "call_1", events[0].ContentBlock.ID)
	assert.Equal(t, "__ReadFile", events[0].ContentBlock.Name)

	// 3. arguments delta
	events = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.delta",
		OutputIndex: 0,
		Delta:       `{"city":`,
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "content_block_delta", events[0].Type)
	assert.Equal(t, "input_json_delta", events[0].Delta.Type)
	assert.Equal(t, `{"city":`, events[0].Delta.PartialJSON)

	// 4. arguments done
	events = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.function_call_arguments.done",
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "content_block_stop", events[0].Type)

	// 5. completed with tool_calls
	events = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
			Usage:  &ResponsesUsage{InputTokens: 20, OutputTokens: 10},
		},
	}, state)
	require.Len(t, events, 2)
	assert.Equal(t, "tool_use", events[0].Delta.StopReason)
}

func TestBuildToolUseBlocks_PreservesInvalidArgumentsAsJSONString(t *testing.T) {
	state := NewChatChunkToAnthropicState("gpt-5.2")
	AccumulateToolCall(state, &ChatToolCall{
		ID:   "call_1",
		Type: "function",
		Function: ChatFunctionCall{
			Name:      "get_weather",
			Arguments: `{"city":`,
		},
	})

	blocks := BuildToolUseBlocks(state)
	require.Len(t, blocks, 1)
	require.Equal(t, "tool_use", blocks[0].Type)
	require.True(t, json.Valid(blocks[0].Input))
	require.Equal(t, `"{\"city\":"`, string(blocks[0].Input))

	resp := AnthropicResponse{
		ID:      "msg_1",
		Type:    "message",
		Role:    "assistant",
		Model:   "claude-opus-4-6",
		Content: blocks,
	}
	_, err := json.Marshal(resp)
	require.NoError(t, err)
}

func TestBuildToolUseBlocks_PreservesSparseToolCallIndex(t *testing.T) {
	state := NewChatChunkToAnthropicState("gpt-5.2")
	AccumulateToolCall(state, &ChatToolCall{
		Index: ccIntPtr(1),
		ID:    "call_sparse",
		Type:  "function",
		Function: ChatFunctionCall{
			Name:      "get_weather",
			Arguments: `{"city":"NYC"}`,
		},
	})

	blocks := BuildToolUseBlocks(state)
	require.Len(t, blocks, 1)
	require.Equal(t, "call_sparse", blocks[0].ID)
	require.Equal(t, "get_weather", blocks[0].Name)
	require.JSONEq(t, `{"city":"NYC"}`, string(blocks[0].Input))
}

func TestStreamingWebSearchUsesActionSources(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_ws_stream", Model: "gpt-5.4"},
	}, state)

	var evt ResponsesStreamEvent
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "response.output_item.done",
		"output_index": 0,
		"item": {
			"type": "web_search_call",
			"id": "ws_1",
			"status": "completed",
			"action": {
				"type": "search",
				"query": "latest news",
				"sources": [
					{
						"type": "url",
						"url": "https://example.com/source",
						"title": "Example Source"
					}
				]
			}
		}
	}`), &evt))

	events := ResponsesEventToAnthropicEvents(&evt, state)
	require.Len(t, events, 4)
	assert.Equal(t, "server_tool_use", events[0].ContentBlock.Type)
	assert.Equal(t, "web_search_tool_result", events[2].ContentBlock.Type)

	var results []map[string]any
	require.NoError(t, json.Unmarshal(events[2].ContentBlock.Content, &results))
	require.NotEmpty(t, results)
	assert.Equal(t, "https://example.com/source", results[0]["url"])
	assert.Equal(t, "Example Source", results[0]["title"])
}

func TestStreamingWebSearchUsesCompletionAnnotationsWhenSourcesMissing(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_ws_stream_annotations", Model: "gpt-5.4"},
	}, state)

	var doneEvt ResponsesStreamEvent
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "response.output_item.done",
		"output_index": 0,
		"item": {
			"type": "web_search_call",
			"id": "ws_1",
			"status": "completed",
			"action": {
				"type": "search",
				"query": "latest news"
			}
		}
	}`), &doneEvt))

	events := ResponsesEventToAnthropicEvents(&doneEvt, state)
	require.Len(t, events, 2)
	assert.Equal(t, "server_tool_use", events[0].ContentBlock.Type)

	var completedEvt ResponsesStreamEvent
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "response.completed",
		"response": {
			"id": "resp_ws_stream_annotations",
			"model": "gpt-5.4",
			"status": "completed",
			"output": [
				{
					"type": "web_search_call",
					"id": "ws_1",
					"status": "completed",
					"action": {
						"type": "search",
						"query": "latest news"
					}
				},
				{
					"type": "message",
					"id": "msg_1",
					"status": "completed",
					"role": "assistant",
					"content": [
						{
							"type": "output_text",
							"text": "Result with citation",
							"annotations": [
								{
									"type": "url_citation",
									"url": "https://example.com/a",
									"title": "Example A",
									"start_index": 0,
									"end_index": 6
								}
							]
						}
					]
				}
			]
		}
	}`), &completedEvt))

	events = ResponsesEventToAnthropicEvents(&completedEvt, state)
	require.Len(t, events, 4)
	assert.Equal(t, "web_search_tool_result", events[0].ContentBlock.Type)

	var results []map[string]any
	require.NoError(t, json.Unmarshal(events[0].ContentBlock.Content, &results))
	require.NotEmpty(t, results)
	assert.Equal(t, "https://example.com/a", results[0]["url"])
	assert.Equal(t, "Example A", results[0]["title"])
}

func TestStreamingWebSearchUsesEmptyArrayWhenNoResultsAvailable(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_ws_stream_empty", Model: "gpt-5.4"},
	}, state)

	var doneEvt ResponsesStreamEvent
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "response.output_item.done",
		"output_index": 0,
		"item": {
			"type": "web_search_call",
			"id": "ws_1",
			"status": "completed",
			"action": {
				"type": "search",
				"query": "latest news"
			}
		}
	}`), &doneEvt))

	events := ResponsesEventToAnthropicEvents(&doneEvt, state)
	require.Len(t, events, 2)

	var completedEvt ResponsesStreamEvent
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "response.completed",
		"response": {
			"id": "resp_ws_stream_empty",
			"model": "gpt-5.4",
			"status": "completed",
			"output": [
				{
					"type": "web_search_call",
					"id": "ws_1",
					"status": "completed",
					"action": {
						"type": "search",
						"query": "latest news"
					}
				}
			]
		}
	}`), &completedEvt))

	events = ResponsesEventToAnthropicEvents(&completedEvt, state)
	require.Len(t, events, 4)
	assert.Equal(t, "web_search_tool_result", events[0].ContentBlock.Type)
	assert.JSONEq(t, `[]`, string(events[0].ContentBlock.Content))
}

func TestStreamingReasoning(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_3", Model: "gpt-5.2"},
	}, state)

	// reasoning item added
	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item:        &ResponsesOutput{Type: "reasoning"},
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "content_block_start", events[0].Type)
	assert.Equal(t, "thinking", events[0].ContentBlock.Type)

	sse, err := ResponsesAnthropicEventToSSE(events[0])
	require.NoError(t, err)
	assert.Contains(t, sse, `"content_block":{"thinking":"","type":"thinking"}`)

	// reasoning text delta
	events = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.reasoning_summary_text.delta",
		OutputIndex: 0,
		Delta:       "Let me think...",
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "content_block_delta", events[0].Type)
	assert.Equal(t, "thinking_delta", events[0].Delta.Type)
	assert.Equal(t, "Let me think...", events[0].Delta.Thinking)

	// reasoning done
	events = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.reasoning_summary_text.done",
	}, state)
	require.Len(t, events, 1)
	assert.Equal(t, "content_block_stop", events[0].Type)
}

func TestStreamingIncomplete(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_4", Model: "gpt-5.2"},
	}, state)

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:  "response.output_text.delta",
		Delta: "Partial output...",
	}, state)

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.incomplete",
		Response: &ResponsesResponse{
			Status:            "incomplete",
			IncompleteDetails: &ResponsesIncompleteDetails{Reason: "max_output_tokens"},
			Usage:             &ResponsesUsage{InputTokens: 100, OutputTokens: 4096},
		},
	}, state)

	// Should close the text block + message_delta + message_stop
	require.Len(t, events, 3)
	assert.Equal(t, "content_block_stop", events[0].Type)
	assert.Equal(t, "message_delta", events[1].Type)
	assert.Equal(t, "max_tokens", events[1].Delta.StopReason)
	assert.Equal(t, "message_stop", events[2].Type)
}

func TestFinalizeStream_NeverStarted(t *testing.T) {
	state := NewResponsesEventToAnthropicState()
	events := FinalizeResponsesAnthropicStream(state)
	assert.Nil(t, events)
}

func TestFinalizeStream_AlreadyCompleted(t *testing.T) {
	state := NewResponsesEventToAnthropicState()
	state.MessageStartSent = true
	state.MessageStopSent = true
	events := FinalizeResponsesAnthropicStream(state)
	assert.Nil(t, events)
}

func TestFinalizeStream_AbnormalTermination(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	// Simulate a stream that started but never completed
	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_5", Model: "gpt-5.2"},
	}, state)

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:  "response.output_text.delta",
		Delta: "Interrupted...",
	}, state)

	// Stream ends without response.completed
	events := FinalizeResponsesAnthropicStream(state)
	require.Len(t, events, 3) // content_block_stop + message_delta + message_stop
	assert.Equal(t, "content_block_stop", events[0].Type)
	assert.Equal(t, "message_delta", events[1].Type)
	assert.Equal(t, "end_turn", events[1].Delta.StopReason)
	assert.Equal(t, "message_stop", events[2].Type)
}

func TestFinalizeStream_AbnormalTerminationAfterToolUsePreservesToolUseStopReason(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_tool_1", Model: "gpt-5.2"},
	}, state)

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item: &ResponsesOutput{
			Type:   "function_call",
			CallID: "call_1",
			Name:   "readfile",
		},
	}, state)

	events := FinalizeResponsesAnthropicStream(state)
	require.Len(t, events, 3)
	assert.Equal(t, "content_block_stop", events[0].Type)
	assert.Equal(t, "message_delta", events[1].Type)
	assert.Equal(t, "tool_use", events[1].Delta.StopReason)
	assert.Equal(t, "message_stop", events[2].Type)
}

func TestStreamingEmptyResponse(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_6", Model: "gpt-5.2"},
	}, state)

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
			Usage:  &ResponsesUsage{InputTokens: 5, OutputTokens: 0},
		},
	}, state)

	require.Len(t, events, 2) // message_delta + message_stop
	assert.Equal(t, "message_delta", events[0].Type)
	assert.Equal(t, "end_turn", events[0].Delta.StopReason)
}

func TestResponsesAnthropicEventToSSE(t *testing.T) {
	evt := AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:   "resp_1",
			Type: "message",
			Role: "assistant",
		},
	}
	sse, err := ResponsesAnthropicEventToSSE(evt)
	require.NoError(t, err)
	assert.Contains(t, sse, "event: message_start\n")
	assert.Contains(t, sse, "data: ")
	assert.Contains(t, sse, `"resp_1"`)
}

// ---------------------------------------------------------------------------
// response.failed tests
// ---------------------------------------------------------------------------

func TestStreamingFailed(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	// 1. response.created
	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_fail_1", Model: "gpt-5.2"},
	}, state)

	// 2. Some text output before failure
	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:  "response.output_text.delta",
		Delta: "Partial output before failure",
	}, state)

	// 3. response.failed
	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.failed",
		Response: &ResponsesResponse{
			Status: "failed",
			Error:  &ResponsesError{Code: "server_error", Message: "Internal error"},
			Usage:  &ResponsesUsage{InputTokens: 50, OutputTokens: 10},
		},
	}, state)

	// Should close text block + message_delta + message_stop
	require.Len(t, events, 3)
	assert.Equal(t, "content_block_stop", events[0].Type)
	assert.Equal(t, "message_delta", events[1].Type)
	assert.Equal(t, "end_turn", events[1].Delta.StopReason)
	assert.Equal(t, 50, events[1].Usage.InputTokens)
	assert.Equal(t, 10, events[1].Usage.OutputTokens)
	assert.Equal(t, "message_stop", events[2].Type)
}

func TestStreamingFailedNoOutput(t *testing.T) {
	state := NewResponsesEventToAnthropicState()

	// 1. response.created
	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_fail_2", Model: "gpt-5.2"},
	}, state)

	// 2. response.failed with no prior output
	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.failed",
		Response: &ResponsesResponse{
			Status: "failed",
			Error:  &ResponsesError{Code: "rate_limit_error", Message: "Too many requests"},
			Usage:  &ResponsesUsage{InputTokens: 20, OutputTokens: 0},
		},
	}, state)

	// Should emit message_delta + message_stop (no block to close)
	require.Len(t, events, 2)
	assert.Equal(t, "message_delta", events[0].Type)
	assert.Equal(t, "end_turn", events[0].Delta.StopReason)
	assert.Equal(t, "message_stop", events[1].Type)
}

func TestResponsesToAnthropic_Failed(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_fail_3",
		Model:  "gpt-5.2",
		Status: "failed",
		Error:  &ResponsesError{Code: "server_error", Message: "Something went wrong"},
		Output: []ResponsesOutput{},
		Usage:  &ResponsesUsage{InputTokens: 30, OutputTokens: 0},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")
	// Failed status defaults to "end_turn" stop reason
	assert.Equal(t, "end_turn", anth.StopReason)
	// Should have at least an empty text block
	require.Len(t, anth.Content, 1)
	assert.Equal(t, "text", anth.Content[0].Type)
}

// ---------------------------------------------------------------------------
// thinking → reasoning conversion tests
// ---------------------------------------------------------------------------

func TestAnthropicToResponses_ThinkingEnabled(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages:  []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
		Thinking:  &AnthropicThinking{Type: "enabled", BudgetTokens: 10000},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Reasoning)
	// thinking.type is ignored for effort; default high applies.
	assert.Equal(t, "high", resp.Reasoning.Effort)
	assert.Equal(t, "auto", resp.Reasoning.Summary)
	assert.Contains(t, resp.Include, "reasoning.encrypted_content")
	assert.NotContains(t, resp.Include, "reasoning.summary")
}

func TestAnthropicToResponses_ThinkingAdaptive(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages:  []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
		Thinking:  &AnthropicThinking{Type: "adaptive", BudgetTokens: 5000},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Reasoning)
	// thinking.type is ignored for effort; default high applies.
	assert.Equal(t, "high", resp.Reasoning.Effort)
	assert.Equal(t, "auto", resp.Reasoning.Summary)
	assert.NotContains(t, resp.Include, "reasoning.summary")
}

func TestAnthropicToResponses_ThinkingDisabled(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages:  []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
		Thinking:  &AnthropicThinking{Type: "disabled"},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	// Default effort applies (high → high) even when thinking is disabled.
	require.NotNil(t, resp.Reasoning)
	assert.Equal(t, "high", resp.Reasoning.Effort)
}

func TestAnthropicToResponses_NoThinking(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages:  []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	// Default effort applies (high → high) when no thinking/output_config is set.
	require.NotNil(t, resp.Reasoning)
	assert.Equal(t, "high", resp.Reasoning.Effort)
}

// ---------------------------------------------------------------------------
// output_config.effort override tests
// ---------------------------------------------------------------------------

func TestAnthropicToResponses_OutputConfigOverridesDefault(t *testing.T) {
	// Default is high, but output_config.effort="low" overrides. low→low after mapping.
	req := &AnthropicRequest{
		Model:        "gpt-5.2",
		MaxTokens:    1024,
		Messages:     []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
		Thinking:     &AnthropicThinking{Type: "enabled", BudgetTokens: 10000},
		OutputConfig: &AnthropicOutputConfig{Effort: "low"},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Reasoning)
	assert.Equal(t, "low", resp.Reasoning.Effort)
	assert.Equal(t, "auto", resp.Reasoning.Summary)
}

func TestAnthropicToResponses_OutputConfigWithoutThinking(t *testing.T) {
	// No thinking field, but output_config.effort="medium" → creates reasoning.
	// medium→medium after 1:1 mapping.
	req := &AnthropicRequest{
		Model:        "gpt-5.2",
		MaxTokens:    1024,
		Messages:     []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
		OutputConfig: &AnthropicOutputConfig{Effort: "medium"},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Reasoning)
	assert.Equal(t, "medium", resp.Reasoning.Effort)
	assert.Equal(t, "auto", resp.Reasoning.Summary)
}

func TestAnthropicToResponses_OutputConfigHigh(t *testing.T) {
	// output_config.effort="high" → mapped to "high" (1:1, both sides' default).
	req := &AnthropicRequest{
		Model:        "gpt-5.2",
		MaxTokens:    1024,
		Messages:     []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
		OutputConfig: &AnthropicOutputConfig{Effort: "high"},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Reasoning)
	assert.Equal(t, "high", resp.Reasoning.Effort)
	assert.Equal(t, "auto", resp.Reasoning.Summary)
}

func TestAnthropicToResponses_OutputConfigMax(t *testing.T) {
	// output_config.effort="max" → mapped to OpenAI's highest supported level "xhigh".
	req := &AnthropicRequest{
		Model:        "gpt-5.2",
		MaxTokens:    1024,
		Messages:     []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
		OutputConfig: &AnthropicOutputConfig{Effort: "max"},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Reasoning)
	assert.Equal(t, "xhigh", resp.Reasoning.Effort)
	assert.Equal(t, "auto", resp.Reasoning.Summary)
}

func TestAnthropicToResponses_NoOutputConfig(t *testing.T) {
	// No output_config → default high regardless of thinking.type.
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages:  []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
		Thinking:  &AnthropicThinking{Type: "enabled", BudgetTokens: 10000},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Reasoning)
	assert.Equal(t, "high", resp.Reasoning.Effort)
}

func TestAnthropicToResponses_OutputConfigWithoutEffort(t *testing.T) {
	// output_config present but effort empty (e.g. only format set) → default high.
	req := &AnthropicRequest{
		Model:        "gpt-5.2",
		MaxTokens:    1024,
		Messages:     []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
		OutputConfig: &AnthropicOutputConfig{},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Reasoning)
	assert.Equal(t, "high", resp.Reasoning.Effort)
}

// ---------------------------------------------------------------------------
// tool_choice conversion tests
// ---------------------------------------------------------------------------

func TestAnthropicToResponses_ToolChoiceAuto(t *testing.T) {
	req := &AnthropicRequest{
		Model:      "gpt-5.2",
		MaxTokens:  1024,
		Messages:   []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
		ToolChoice: json.RawMessage(`{"type":"auto"}`),
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var tc string
	require.NoError(t, json.Unmarshal(resp.ToolChoice, &tc))
	assert.Equal(t, "auto", tc)
}

func TestAnthropicToResponses_ToolChoiceAny(t *testing.T) {
	req := &AnthropicRequest{
		Model:      "gpt-5.2",
		MaxTokens:  1024,
		Messages:   []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
		ToolChoice: json.RawMessage(`{"type":"any"}`),
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var tc string
	require.NoError(t, json.Unmarshal(resp.ToolChoice, &tc))
	assert.Equal(t, "required", tc)
}

func TestAnthropicToResponses_ToolChoiceSpecific(t *testing.T) {
	req := &AnthropicRequest{
		Model:      "gpt-5.2",
		MaxTokens:  1024,
		Messages:   []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
		ToolChoice: json.RawMessage(`{"type":"tool","name":"get_weather"}`),
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var tc map[string]any
	require.NoError(t, json.Unmarshal(resp.ToolChoice, &tc))
	assert.Equal(t, "function", tc["type"])
	assert.Equal(t, "get_weather", tc["name"])
	assert.NotContains(t, tc, "function")
}

func TestAnthropicToResponses_NormalizesLegacyPrefixedToolUseID(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		Messages: []AnthropicMessage{
			{
				Role:    "assistant",
				Content: json.RawMessage(`[{"type":"tool_use","id":"fc_toolu_123","name":"lookup","input":{"q":"x"}}]`),
			},
			{
				Role:    "user",
				Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"fc_toolu_123","content":"ok"}]`),
			},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 2)
	require.Equal(t, "function_call", items[0].Type)
	require.Equal(t, "toolu_123", items[0].CallID)
	require.Equal(t, "function_call_output", items[1].Type)
	require.Equal(t, "toolu_123", items[1].CallID)
}

func TestResponsesToAnthropicRequest_ToolChoiceSpecificLegacyShape(t *testing.T) {
	req := &ResponsesRequest{
		Model:      "gpt-5.2",
		Input:      json.RawMessage(`[{"role":"user","content":"Hello"}]`),
		ToolChoice: json.RawMessage(`{"type":"function","function":{"name":"get_weather"}}`),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)

	var tc map[string]any
	require.NoError(t, json.Unmarshal(out.ToolChoice, &tc))
	assert.Equal(t, "tool", tc["type"])
	assert.Equal(t, "get_weather", tc["name"])
}

func TestResponsesToAnthropicRequest_ToolChoiceSpecificNewShape(t *testing.T) {
	req := &ResponsesRequest{
		Model:      "gpt-5.2",
		Input:      json.RawMessage(`[{"role":"user","content":"Hello"}]`),
		ToolChoice: json.RawMessage(`{"type":"function","name":"get_weather"}`),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)

	var tc map[string]any
	require.NoError(t, json.Unmarshal(out.ToolChoice, &tc))
	assert.Equal(t, "tool", tc["type"])
	assert.Equal(t, "get_weather", tc["name"])
}

func TestResponsesToAnthropicRequest_ToolChoiceAutoObjectPreservesDisableParallelToolUse(t *testing.T) {
	req := &ResponsesRequest{
		Model:      "gpt-5.2",
		Input:      json.RawMessage(`[{"role":"user","content":"Hello"}]`),
		ToolChoice: json.RawMessage(`{"type":"auto"}`),
		ParallelToolCalls: func() *bool {
			v := false
			return &v
		}(),
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)

	var tc map[string]any
	require.NoError(t, json.Unmarshal(out.ToolChoice, &tc))
	assert.Equal(t, "auto", tc["type"])
	assert.Equal(t, true, tc["disable_parallel_tool_use"])
}

// ---------------------------------------------------------------------------
// Image content block conversion tests
// ---------------------------------------------------------------------------

func TestAnthropicToResponses_UserImageBlock(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`[
				{"type":"text","text":"What is in this image?"},
				{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBOR"}}
			]`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)
	assert.Equal(t, "user", items[0].Role)

	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[0].Content, &parts))
	require.Len(t, parts, 2)
	assert.Equal(t, "input_text", parts[0].Type)
	assert.Equal(t, "What is in this image?", parts[0].Text)
	assert.Equal(t, "input_image", parts[1].Type)
	assert.Equal(t, "data:image/png;base64,iVBOR", parts[1].ImageURL)
}

func TestAnthropicToResponses_UserDocumentBlockBase64PDF(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`[
				{"type":"document","title":"report.pdf","source":{"type":"base64","media_type":"application/pdf","data":"JVBERi0x"}},
				{"type":"text","text":"Summarize this PDF"}
			]`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)

	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[0].Content, &parts))
	require.Len(t, parts, 2)
	assert.Equal(t, "input_file", parts[0].Type)
	assert.Equal(t, "JVBERi0x", parts[0].FileData)
	assert.Equal(t, "report.pdf", parts[0].Filename)
	assert.Equal(t, "input_text", parts[1].Type)
}

func TestAnthropicToResponses_UserDocumentBlockURLPDF(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`[
				{"type":"document","title":"report.pdf","source":{"type":"url","url":"https://example.com/report.pdf"}},
				{"type":"text","text":"Summarize this PDF"}
			]`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)

	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[0].Content, &parts))
	require.Len(t, parts, 2)
	assert.Equal(t, "input_file", parts[0].Type)
	assert.Equal(t, "https://example.com/report.pdf", parts[0].FileURL)
	assert.Equal(t, "report.pdf", parts[0].Filename)
}

func TestAnthropicToResponses_UserDocumentBlockFileIDPDF(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`[
				{"type":"document","title":"report.pdf","source":{"type":"file","file_id":"file_abc123"}},
				{"type":"text","text":"Summarize this PDF"}
			]`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)

	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[0].Content, &parts))
	require.Len(t, parts, 2)
	assert.Equal(t, "input_file", parts[0].Type)
	assert.Equal(t, "file_abc123", parts[0].FileID)
	assert.Equal(t, "report.pdf", parts[0].Filename)
}

func TestAnthropicToResponses_ImageOnlyUserMessage(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`[
				{"type":"image","source":{"type":"base64","media_type":"image/jpeg","data":"/9j/4AAQ"}}
			]`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)

	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[0].Content, &parts))
	require.Len(t, parts, 1)
	assert.Equal(t, "input_image", parts[0].Type)
	assert.Equal(t, "data:image/jpeg;base64,/9j/4AAQ", parts[0].ImageURL)
}

func TestAnthropicToResponses_ToolResultWithImage(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"Read the screenshot"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"/tmp/screen.png"}}]`)},
			{Role: "user", Content: json.RawMessage(`[
				{"type":"tool_result","tool_use_id":"toolu_1","content":[
					{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBOR"}}
				]}
			]`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	// user + function_call + function_call_output + user(image) = 4
	require.Len(t, items, 4)

	// function_call_output should have text-only output (no image).
	assert.Equal(t, "function_call_output", items[2].Type)
	assert.Equal(t, "toolu_1", items[2].CallID)
	assert.Equal(t, "(empty)", items[2].Output)

	// Image should be in a separate user message.
	assert.Equal(t, "user", items[3].Role)
	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[3].Content, &parts))
	require.Len(t, parts, 1)
	assert.Equal(t, "input_image", parts[0].Type)
	assert.Equal(t, "data:image/png;base64,iVBOR", parts[0].ImageURL)
}

func TestAnthropicToResponses_ToolResultImageOnlyPreservesErrorFlag(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"Read the screenshot"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"/tmp/screen.png"}}]`)},
			{Role: "user", Content: json.RawMessage(`[
				{"type":"tool_result","tool_use_id":"toolu_1","is_error":true,"content":[
					{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBOR"}}
				]}
			]`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	// user + function_call + function_call_output + user(image) = 4
	require.Len(t, items, 4)

	var env anthropicToolResultEnvelope
	require.NoError(t, json.Unmarshal([]byte(items[2].Output), &env))
	assert.True(t, env.IsError)
	assert.Empty(t, env.Text)

	back, err := ResponsesToAnthropicRequest(resp)
	require.NoError(t, err)

	var found *AnthropicContentBlock
	for _, msg := range back.Messages {
		if msg.Role != "user" {
			continue
		}
		var blocks []AnthropicContentBlock
		if err := json.Unmarshal(msg.Content, &blocks); err != nil {
			continue
		}
		for i := range blocks {
			if blocks[i].Type == "tool_result" {
				found = &blocks[i]
				break
			}
		}
		if found != nil {
			break
		}
	}

	require.NotNil(t, found)
	assert.True(t, found.IsError)
	assert.JSONEq(t, `[]`, string(found.Content))
}

func TestAnthropicToResponses_ToolResultMixed(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"Describe the file"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"tool_use","id":"toolu_2","name":"Read","input":{"file_path":"/tmp/photo.png"}}]`)},
			{Role: "user", Content: json.RawMessage(`[
				{"type":"tool_result","tool_use_id":"toolu_2","content":[
					{"type":"text","text":"File metadata: 800x600 PNG"},
					{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAAA"}}
				]}
			]`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	// user + function_call + function_call_output + user(image) = 4
	require.Len(t, items, 4)

	// function_call_output should have text-only output.
	assert.Equal(t, "function_call_output", items[2].Type)
	assert.Equal(t, "File metadata: 800x600 PNG", items[2].Output)

	// Image should be in a separate user message.
	assert.Equal(t, "user", items[3].Role)
	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[3].Content, &parts))
	require.Len(t, parts, 1)
	assert.Equal(t, "input_image", parts[0].Type)
	assert.Equal(t, "data:image/png;base64,AAAA", parts[0].ImageURL)
}

func TestAnthropicToResponses_TextOnlyToolResultBackwardCompat(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"Check weather"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"tool_use","id":"call_1","name":"get_weather","input":{"city":"NYC"}}]`)},
			{Role: "user", Content: json.RawMessage(`[
				{"type":"tool_result","tool_use_id":"call_1","content":[
					{"type":"text","text":"Sunny, 72°F"}
				]}
			]`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	// user + function_call + function_call_output = 3
	require.Len(t, items, 3)

	// Text-only tool_result should produce a plain string.
	assert.Equal(t, "Sunny, 72°F", items[2].Output)
}

func TestAnthropicToResponses_ImageEmptyMediaType(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`[
				{"type":"image","source":{"type":"base64","media_type":"","data":"iVBOR"}}
			]`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)

	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[0].Content, &parts))
	require.Len(t, parts, 1)
	assert.Equal(t, "input_image", parts[0].Type)
	// Should default to image/png when media_type is empty.
	assert.Equal(t, "data:image/png;base64,iVBOR", parts[0].ImageURL)
}

// ---------------------------------------------------------------------------
// normalizeToolParameters tests
// ---------------------------------------------------------------------------

func TestNormalizeToolParameters(t *testing.T) {
	tests := []struct {
		name     string
		input    json.RawMessage
		expected string
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: `{"type":"object","properties":{}}`,
		},
		{
			name:     "empty input",
			input:    json.RawMessage(``),
			expected: `{"type":"object","properties":{}}`,
		},
		{
			name:     "null input",
			input:    json.RawMessage(`null`),
			expected: `{"type":"object","properties":{}}`,
		},
		{
			name:     "object without properties",
			input:    json.RawMessage(`{"type":"object"}`),
			expected: `{"type":"object","properties":{}}`,
		},
		{
			name:     "object with properties",
			input:    json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
			expected: `{"type":"object","properties":{"city":{"type":"string"}}}`,
		},
		{
			name:     "non-object type",
			input:    json.RawMessage(`{"type":"string"}`),
			expected: `{"type":"string"}`,
		},
		{
			name:     "object with additional fields preserved",
			input:    json.RawMessage(`{"type":"object","required":["name"]}`),
			expected: `{"type":"object","required":["name"],"properties":{}}`,
		},
		{
			name:     "invalid JSON passthrough",
			input:    json.RawMessage(`not json`),
			expected: `not json`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeToolParameters(tt.input)
			if tt.name == "invalid JSON passthrough" {
				assert.Equal(t, tt.expected, string(result))
			} else {
				assert.JSONEq(t, tt.expected, string(result))
			}
		})
	}
}

func TestAnthropicToResponses_ToolWithoutProperties(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
		Tools: []AnthropicTool{
			{Name: "mcp__pencil__get_style_guide_tags", Description: "Get style tags", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	require.Len(t, resp.Tools, 1)
	assert.Equal(t, "function", resp.Tools[0].Type)
	assert.Equal(t, "mcp__pencil__get_style_guide_tags", resp.Tools[0].Name)

	// Parameters must have "properties" field after normalization.
	var params map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp.Tools[0].Parameters, &params))
	assert.Contains(t, params, "properties")
}

func TestAnthropicToResponses_DropsDeferredToolSearchMetaTools(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
		Tools: []AnthropicTool{
			{Type: "tool_search", Name: "tool_search", Description: "Claude local tool search"},
			{Type: "tool_search_tool_regex_20251119", Name: "tool_search_tool_regex_20251119", Description: "Claude local regex search"},
			{Name: "simple_tool", Description: "A tool", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	require.Len(t, resp.Tools, 1)
	assert.Equal(t, "function", resp.Tools[0].Type)
	assert.Equal(t, "simple_tool", resp.Tools[0].Name)
}

func TestAnthropicToResponses_ToolWithNilSchema(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "gpt-5.2",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
		Tools: []AnthropicTool{
			{Name: "simple_tool", Description: "A tool"},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	require.Len(t, resp.Tools, 1)
	var params map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp.Tools[0].Parameters, &params))
	assert.JSONEq(t, `"object"`, string(params["type"]))
	assert.JSONEq(t, `{}`, string(params["properties"]))
}

// ---------------------------------------------------------------------------
// isReasoningModel / temperature-stripping tests
// ---------------------------------------------------------------------------

func TestAnthropicToResponses_TemperatureStrippedForReasoningModel(t *testing.T) {
	temp := 0.7
	req := &AnthropicRequest{
		Model:       "gpt-5.2",
		MaxTokens:   1024,
		Messages:    []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
		Temperature: &temp,
		TopP:        &temp,
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)
	assert.Nil(t, resp.Temperature, "reasoning model: temperature must be stripped")
	assert.Nil(t, resp.TopP, "reasoning model: top_p must be stripped")

	// Verify the fields are absent from the serialised JSON.
	b, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotContains(t, string(b), `"temperature"`)
	assert.NotContains(t, string(b), `"top_p"`)
}

func TestAnthropicToResponses_TemperatureStrippedForAllGpt5Variants(t *testing.T) {
	temp := 1.0
	models := []string{"gpt-5.2", "gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex", "gpt-5.5"}
	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			req := &AnthropicRequest{
				Model:       model,
				MaxTokens:   1024,
				Messages:    []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"Hello"`)}},
				Temperature: &temp,
				TopP:        &temp,
			}
			resp, err := AnthropicToResponses(req)
			require.NoError(t, err)
			assert.Nil(t, resp.Temperature, "model %s: temperature must be stripped", model)
			assert.Nil(t, resp.TopP, "model %s: top_p must be stripped", model)
		})
	}
}

// ---------------------------------------------------------------------------
// AnthropicToResponsesResponse: Anthropic input_tokens excludes cached tokens
// while OpenAI Responses input_tokens is the total including cached tokens.
// ---------------------------------------------------------------------------

func TestAnthropicToResponsesResponse_CacheTokensUseOpenAIInputSemantics(t *testing.T) {
	resp := &AnthropicResponse{
		ID:    "msg_cache",
		Model: "claude-sonnet-4-5-20250929",
		Content: []AnthropicContentBlock{
			{Type: "text", Text: "ok"},
		},
		StopReason: "end_turn",
		Usage: AnthropicUsage{
			InputTokens:              3318,
			OutputTokens:             123,
			CacheReadInputTokens:     50688,
			CacheCreationInputTokens: 200,
		},
	}

	out := AnthropicToResponsesResponse(resp)
	require.NotNil(t, out.Usage)
	// 3318 (uncached) + 50688 (read) + 200 (creation) = 54206
	assert.Equal(t, 54206, out.Usage.InputTokens)
	assert.Equal(t, 123, out.Usage.OutputTokens)
	assert.Equal(t, 54329, out.Usage.TotalTokens)
	require.NotNil(t, out.Usage.InputTokensDetails)
	assert.Equal(t, 50688, out.Usage.InputTokensDetails.CachedTokens)
}

func TestAnthropicToResponsesResponse_NoCacheTokens(t *testing.T) {
	resp := &AnthropicResponse{
		ID:    "msg_nocache",
		Model: "claude-sonnet-4-5-20250929",
		Content: []AnthropicContentBlock{
			{Type: "text", Text: "ok"},
		},
		StopReason: "end_turn",
		Usage: AnthropicUsage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	out := AnthropicToResponsesResponse(resp)
	require.NotNil(t, out.Usage)
	assert.Equal(t, 100, out.Usage.InputTokens)
	assert.Equal(t, 50, out.Usage.OutputTokens)
	assert.Equal(t, 150, out.Usage.TotalTokens)
	assert.Nil(t, out.Usage.InputTokensDetails)
}

func TestAnthropicEventToResponses_CacheTokensRoundTripFromMessageStart(t *testing.T) {
	state := NewAnthropicEventToResponsesState()

	// message_start carries cache fields on the initial Usage object.
	AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_stream_cache",
			Model: "claude-sonnet-4-5-20250929",
			Usage: AnthropicUsage{
				InputTokens:              12,
				CacheReadInputTokens:     9,
				CacheCreationInputTokens: 3,
			},
		},
	}, state)

	AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_delta",
		Usage: &AnthropicUsage{
			OutputTokens: 7,
		},
	}, state)

	events := AnthropicEventToResponsesEvents(&AnthropicStreamEvent{Type: "message_stop"}, state)

	// The terminal response.completed event must include OpenAI-semantic usage.
	var completed *ResponsesStreamEvent
	for i := range events {
		if events[i].Type == "response.completed" {
			completed = &events[i]
		}
	}
	require.NotNil(t, completed, "response.completed event must be emitted")
	require.NotNil(t, completed.Response)
	require.NotNil(t, completed.Response.Usage)
	// 12 (uncached) + 9 (read) + 3 (creation) = 24
	assert.Equal(t, 24, completed.Response.Usage.InputTokens)
	assert.Equal(t, 7, completed.Response.Usage.OutputTokens)
	assert.Equal(t, 31, completed.Response.Usage.TotalTokens)
	require.NotNil(t, completed.Response.Usage.InputTokensDetails)
	assert.Equal(t, 9, completed.Response.Usage.InputTokensDetails.CachedTokens)
}

func TestAnthropicEventToResponses_CacheTokensFromMessageDelta(t *testing.T) {
	state := NewAnthropicEventToResponsesState()

	AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_delta_cache",
			Model: "claude-sonnet-4-5-20250929",
			Usage: AnthropicUsage{InputTokens: 20},
		},
	}, state)

	// Some upstreams only emit cache fields on the final message_delta.
	AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_delta",
		Usage: &AnthropicUsage{
			OutputTokens:             8,
			CacheReadInputTokens:     11,
			CacheCreationInputTokens: 4,
		},
	}, state)

	events := AnthropicEventToResponsesEvents(&AnthropicStreamEvent{Type: "message_stop"}, state)

	var completed *ResponsesStreamEvent
	for i := range events {
		if events[i].Type == "response.completed" {
			completed = &events[i]
		}
	}
	require.NotNil(t, completed)
	require.NotNil(t, completed.Response.Usage)
	// 20 (uncached) + 11 (read) + 4 (creation) = 35
	assert.Equal(t, 35, completed.Response.Usage.InputTokens)
	assert.Equal(t, 8, completed.Response.Usage.OutputTokens)
	require.NotNil(t, completed.Response.Usage.InputTokensDetails)
	assert.Equal(t, 11, completed.Response.Usage.InputTokensDetails.CachedTokens)
}
