package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureOpenAIResponsesImageGenerationToolChoiceAuto_NormalizesExplicitImageChoice(t *testing.T) {
	for _, tc := range []struct {
		name       string
		toolChoice any
	}{
		{name: "object", toolChoice: map[string]any{"type": "image_generation"}},
		{name: "string", toolChoice: "image_generation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := map[string]any{
				"model": "gpt-5.5",
				"tools": []any{
					map[string]any{"type": "image_generation", "output_format": "png"},
				},
				"tool_choice": tc.toolChoice,
			}

			modified := ensureOpenAIResponsesImageGenerationToolChoiceAuto(reqBody)

			require.True(t, modified)
			require.Equal(t, "auto", reqBody["tool_choice"])
		})
	}
}

func TestApplyCodexOAuthTransform_ToolContinuationPreservesInput(t *testing.T) {
	// 续链场景：保留 item_reference 与 id，但不再强制 store=true。

	reqBody := map[string]any{
		"model": "gpt-5.2",
		"input": []any{
			map[string]any{"type": "item_reference", "id": "ref1", "text": "x"},
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "ok", "id": "o1"},
		},
		"tool_choice": "auto",
	}

	applyCodexOAuthTransform(reqBody, false, false)

	// 未显式设置 store=true，默认为 false。
	store, ok := reqBody["store"].(bool)
	require.True(t, ok)
	require.False(t, store)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)

	// 校验 input[0] 为 map，避免断言失败导致测试中断。
	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "item_reference", first["type"])
	require.Equal(t, "ref1", first["id"])

	// 校验 input[1] 为 map，确保后续字段断言安全。
	second, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "o1", second["id"])
	require.Equal(t, "fc_1", second["call_id"])
}

func TestApplyCodexOAuthTransform_MessagesBridgePromptCacheKeyIsHeaderOnly(t *testing.T) {
	reqBody := map[string]any{
		"model":            "gpt-5.5",
		"prompt_cache_key": "anthropic-metadata-session-1",
		"input": []any{
			map[string]any{
				"type": "message",
				"role": "developer",
				"content": []any{
					map[string]any{
						"type": "input_text",
						"text": openAICompatClaudeCodeTodoGuardMarker,
					},
				},
			},
			map[string]any{
				"type":    "message",
				"role":    "user",
				"content": "hello",
			},
		},
	}

	result := applyCodexOAuthTransformWithOptions(reqBody, codexOAuthTransformOptions{
		SkipDefaultInstructions: true,
		PreserveToolCallIDs:     true,
	})

	require.Equal(t, "anthropic-metadata-session-1", result.PromptCacheKey)
	require.True(t, result.Modified)
	require.NotContains(t, reqBody, "prompt_cache_key")
}

func TestApplyCodexOAuthTransform_PreservesPromptCacheKeyAndReasoningInclude(t *testing.T) {
	reqBody := map[string]any{
		"model":            "gpt-5.1",
		"store":            false,
		"prompt_cache_key": "pcache_1",
		"reasoning":        map[string]any{"effort": "medium"},
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "hello"},
		},
	}

	result := applyCodexOAuthTransform(reqBody, false, false)

	require.Equal(t, "pcache_1", reqBody["prompt_cache_key"])
	include, ok := reqBody["include"].([]any)
	require.True(t, ok)
	require.Contains(t, include, "reasoning.encrypted_content")
	require.True(t, result.Modified)
}

func TestApplyCodexOAuthTransform_ToolContinuationPreservesNativeMessageAndReasoningIDs(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.2",
		"input": []any{
			map[string]any{"type": "message", "id": "msg_0", "role": "user", "content": "hi"},
			map[string]any{"type": "item_reference", "id": "rs_123"},
		},
		"tool_choice": "auto",
	}

	applyCodexOAuthTransform(reqBody, false, false)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)

	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "msg_0", first["id"])

	second, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "rs_123", second["id"])
}

func TestApplyCodexOAuthTransform_ToolContinuationNormalizesToolReferenceIDsOnly(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.2",
		"input": []any{
			map[string]any{"type": "item_reference", "id": "call_1"},
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "ok"},
		},
		"tool_choice": "auto",
	}

	applyCodexOAuthTransform(reqBody, false, false)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)

	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "fc_1", first["id"])

	second, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "fc_1", second["call_id"])
}

func TestApplyCodexOAuthTransform_ToolContinuationRewritesToolCallItemIDPrefix(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.5",
		"input": []any{
			map[string]any{
				"type":    "function_call",
				"id":      "item_24b0657c20b5397d08f83d12",
				"call_id": "call_tool",
				"name":    "run",
			},
			map[string]any{
				"type":    "function_call_output",
				"call_id": "call_tool",
				"output":  "ok",
			},
		},
	}

	result := applyCodexOAuthTransform(reqBody, false, false)

	require.True(t, result.Modified)
	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	call, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "fc_tool", call["id"])
	require.Equal(t, "fc_tool", call["call_id"])
	output, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "fc_tool", output["call_id"])
}

func TestApplyCodexOAuthTransform_RemovesInvalidMessageItemID(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.5",
		"input": []any{
			map[string]any{
				"type":    "message",
				"id":      "item_8f2fe8e6d109059a090dfc2d",
				"role":    "user",
				"content": "hi",
			},
			map[string]any{
				"type":    "message",
				"id":      "msg_keep",
				"role":    "assistant",
				"content": "ok",
			},
			map[string]any{"type": "item_reference", "id": "fc_keep_context"},
		},
	}

	result := applyCodexOAuthTransform(reqBody, false, false)

	require.True(t, result.Modified)
	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.NotContains(t, first, "id")
	second, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "msg_keep", second["id"])
}

func TestApplyCodexOAuthTransform_ToolSearchOutputPreservesCallID(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.2",
		"input": []any{
			map[string]any{"type": "tool_search_output", "call_id": "call_1", "output": "ok"},
		},
	}

	applyCodexOAuthTransform(reqBody, false, false)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)

	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_search_output", first["type"])
	require.Equal(t, "fc_1", first["call_id"])
}

func TestApplyCodexOAuthTransform_CustomAndMCPToolOutputsPreserveCallID(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.2",
		"input": []any{
			map[string]any{"type": "custom_tool_call_output", "call_id": "call_custom", "output": "ok"},
			map[string]any{"type": "mcp_tool_call_output", "call_id": "call_mcp", "output": "ok"},
		},
	}

	applyCodexOAuthTransform(reqBody, false, false)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)

	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "fc_custom", first["call_id"])

	second, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "fc_mcp", second["call_id"])
}

func TestApplyCodexOAuthTransform_NormalizesStructuredToolCallArgumentsString(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.5",
		"input": []any{
			map[string]any{
				"type":      "tool_search_call",
				"call_id":   "call_search",
				"arguments": `{"query":"golang","limit":5}`,
			},
			map[string]any{
				"type":      "function_call",
				"call_id":   "call_fn",
				"name":      "shell",
				"arguments": `{"cmd":"pwd"}`,
			},
		},
	}

	result := applyCodexOAuthTransform(reqBody, false, false)

	require.True(t, result.Modified)
	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)

	searchCall, ok := input[0].(map[string]any)
	require.True(t, ok)
	arguments, ok := searchCall["arguments"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "golang", arguments["query"])
	require.Equal(t, float64(5), arguments["limit"])

	functionCall, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.IsType(t, "", functionCall["arguments"], "function_call arguments must remain a JSON string")
}

func TestApplyCodexOAuthTransform_DropsNoneTypedTools(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.5",
		"tools": []any{
			map[string]any{"type": "None"},
			map[string]any{"type": nil},
			map[string]any{},
			map[string]any{"type": "function", "name": "run", "parameters": map[string]any{"type": "object"}},
		},
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "hi"},
		},
	}

	result := applyCodexOAuthTransform(reqBody, false, false)

	require.True(t, result.Modified)
	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function", tool["type"])
	require.Equal(t, "run", tool["name"])
}

func TestApplyCodexOAuthTransform_MapsWebSearchPreviewTools(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.5",
		"tools": []any{
			map[string]any{"type": "function", "name": "late_bound", "defer_loading": true, "parameters": map[string]any{"type": "object"}},
			map[string]any{"type": "web_search_preview"},
			map[string]any{"type": "web_search_preview_2025_03_11"},
		},
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "search docs"},
		},
	}

	result := applyCodexOAuthTransform(reqBody, false, false)

	require.True(t, result.Modified)
	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 3)
	deferredTool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function", deferredTool["type"])
	require.Equal(t, true, deferredTool["defer_loading"])
	for _, rawTool := range tools[1:] {
		tool, ok := rawTool.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "tool_search", tool["type"])
	}
}

func TestApplyCodexOAuthTransform_StripsAdditionalUnsupportedFields(t *testing.T) {
	reqBody := map[string]any{
		"model":          "gpt-5.5",
		"max_tokens":     float64(64),
		"max_tool_calls": float64(8),
		"web_search":     map[string]any{"enabled": true},
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "hi"},
		},
	}

	result := applyCodexOAuthTransform(reqBody, false, false)

	require.True(t, result.Modified)
	require.NotContains(t, reqBody, "max_tokens")
	require.NotContains(t, reqBody, "max_tool_calls")
	require.NotContains(t, reqBody, "web_search")
}

func TestApplyCodexOAuthTransform_TruncatesOversizedToolOutputs(t *testing.T) {
	oversized := strings.Repeat("a", codexToolOutputMaxChars+1)
	reqBody := map[string]any{
		"model": "gpt-5.2",
		"input": []any{
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": oversized},
			map[string]any{"type": "tool_search_output", "call_id": "call_2", "output": oversized},
			map[string]any{"type": "custom_tool_call_output", "call_id": "call_3", "output": oversized},
			map[string]any{"type": "mcp_tool_call_output", "call_id": "call_4", "output": oversized},
		},
	}

	result := applyCodexOAuthTransform(reqBody, false, false)

	require.True(t, result.Modified)
	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 4)
	for _, raw := range input {
		item, ok := raw.(map[string]any)
		require.True(t, ok)
		output, ok := item["output"].(string)
		require.True(t, ok)
		require.Len(t, output, codexToolOutputMaxChars)
		require.Equal(t, strings.Repeat("a", codexToolOutputMaxChars), output)
	}
}

func TestApplyCodexOAuthTransform_ImageAndWebSearchCallsDoNotGainCallID(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.2",
		"input": []any{
			map[string]any{"type": "image_generation_call", "id": "ig_123", "status": "completed"},
			map[string]any{"type": "web_search_call", "call_id": "call_bad", "status": "completed"},
		},
		"tool_choice": "auto",
	}

	applyCodexOAuthTransform(reqBody, false, false)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)

	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ig_123", first["id"])
	_, hasCallID := first["call_id"]
	require.False(t, hasCallID)

	second, ok := input[1].(map[string]any)
	require.True(t, ok)
	_, hasCallID = second["call_id"]
	require.False(t, hasCallID)
}

func TestApplyCodexOAuthTransform_ConvertsToolRoleMessageToFunctionCallOutput(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"input": []any{
			map[string]any{
				"type":         "message",
				"role":         "tool",
				"tool_call_id": "call_1",
				"content":      "ok",
			},
		},
	}

	applyCodexOAuthTransform(reqBody, true, false)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)

	item, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function_call_output", item["type"])
	require.Equal(t, "fc_1", item["call_id"])
	require.Equal(t, "ok", item["output"])
	_, hasRole := item["role"]
	require.False(t, hasRole)
}

func TestApplyCodexOAuthTransform_TruncatesToolRoleMessageOutput(t *testing.T) {
	oversized := strings.Repeat("b", codexToolOutputMaxChars+1)
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"input": []any{
			map[string]any{
				"type":         "message",
				"role":         "tool",
				"tool_call_id": "call_1",
				"content":      oversized,
			},
		},
	}

	result := applyCodexOAuthTransform(reqBody, true, false)

	require.True(t, result.Modified)
	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)

	item, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function_call_output", item["type"])
	output, ok := item["output"].(string)
	require.True(t, ok)
	require.Len(t, output, codexToolOutputMaxChars)
	require.Equal(t, strings.Repeat("b", codexToolOutputMaxChars), output)
}

func TestTruncateOpenAIResponsesToolOutputs_TruncatesWithoutCodexTransform(t *testing.T) {
	oversized := strings.Repeat("中", codexToolOutputMaxChars+1)
	reqBody := map[string]any{
		"input": []any{
			map[string]any{"type": "item_reference", "id": "call_1"},
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": oversized},
		},
	}

	require.True(t, truncateOpenAIResponsesToolOutputs(reqBody))

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	item, ok := input[1].(map[string]any)
	require.True(t, ok)
	output, ok := item["output"].(string)
	require.True(t, ok)
	require.Len(t, []rune(output), codexToolOutputMaxChars)
	require.Equal(t, strings.Repeat("中", codexToolOutputMaxChars), output)
}

func TestTruncateOpenAIResponsesToolOutputs_TruncatesOversizedMessageContentText(t *testing.T) {
	oversizedOutput := strings.Repeat("a", codexToolOutputMaxChars+1)
	oversizedText := strings.Repeat("中", codexToolOutputMaxChars+1)
	reqBody := map[string]any{
		"input": []any{
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": oversizedOutput},
			map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "short"},
					map[string]any{"type": "input_text", "text": oversizedText},
				},
			},
		},
	}

	require.True(t, truncateOpenAIResponsesToolOutputs(reqBody))

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)

	outputItem, ok := input[0].(map[string]any)
	require.True(t, ok)
	output, ok := outputItem["output"].(string)
	require.True(t, ok)
	require.Len(t, output, codexToolOutputMaxChars)

	message, ok := input[1].(map[string]any)
	require.True(t, ok)
	content, ok := message["content"].([]any)
	require.True(t, ok)
	part, ok := content[1].(map[string]any)
	require.True(t, ok)
	text, ok := part["text"].(string)
	require.True(t, ok)
	require.Len(t, []rune(text), codexToolOutputMaxChars)
	require.Equal(t, strings.Repeat("中", codexToolOutputMaxChars), text)
}

func TestNeedsReqBodyForToolOutputTruncation_DetectsOversizedMessageContentText(t *testing.T) {
	body := []byte(`{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"` + strings.Repeat("x", codexToolOutputMaxChars+1) + `"}]}]}`)

	require.True(t, needsReqBodyForToolOutputTruncation(body))
}

func TestApplyCodexOAuthTransform_StringifiesNonStringMessageContentText(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"input": []any{
			map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": []any{"a", "b"}},
				},
			},
		},
	}

	applyCodexOAuthTransform(reqBody, true, false)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	item, ok := input[0].(map[string]any)
	require.True(t, ok)
	content, ok := item["content"].([]any)
	require.True(t, ok)
	part, ok := content[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, `["a","b"]`, part["text"])
}

func TestApplyCodexOAuthTransform_DowngradesUnknownToolChoice(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"tools": []any{
			map[string]any{"type": "function", "name": "shell"},
		},
		"tool_choice": map[string]any{"type": "custom"},
	}

	applyCodexOAuthTransform(reqBody, true, false)

	require.Equal(t, "auto", reqBody["tool_choice"])
}

func TestApplyCodexOAuthTransform_PreservesKnownToolChoice(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"tools": []any{
			map[string]any{"type": "custom", "name": "shell"},
		},
		"tool_choice": map[string]any{"type": "custom"},
	}

	applyCodexOAuthTransform(reqBody, true, false)

	choice, ok := reqBody["tool_choice"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "custom", choice["type"])
}

func TestApplyCodexOAuthTransform_NormalizesLegacyFunctionToolChoice(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"tools": []any{
			map[string]any{"type": "function", "name": "shell"},
		},
		"tool_choice": map[string]any{
			"type":     "function",
			"function": map[string]any{"name": "shell"},
		},
	}

	applyCodexOAuthTransform(reqBody, true, false)

	choice, ok := reqBody["tool_choice"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function", choice["type"])
	require.Equal(t, "shell", choice["name"])
	require.NotContains(t, choice, "function")
}

func TestApplyCodexOAuthTransform_DowngradesMissingFunctionToolChoice(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"tools": []any{
			map[string]any{"type": "function", "name": "shell"},
		},
		"tool_choice": map[string]any{
			"type":     "function",
			"function": map[string]any{"name": "missing"},
		},
	}

	applyCodexOAuthTransform(reqBody, true, false)

	require.Equal(t, "auto", reqBody["tool_choice"])
}

func TestApplyCodexOAuthTransform_AddsFallbackNameForFunctionCallInput(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "run tool"},
			map[string]any{"type": "function_call", "call_id": "call_1", "arguments": "{}"},
		},
	}

	applyCodexOAuthTransform(reqBody, true, false)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)
	item, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function_call", item["type"])
	require.Equal(t, "tool", item["name"])
	require.Equal(t, "fc_1", item["call_id"])
}

func TestApplyCodexOAuthTransform_PreservesFunctionCallInputName(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"input": []any{
			map[string]any{"type": "custom_tool_call", "call_id": "call_1", "name": "shell", "input": "pwd"},
		},
	}

	applyCodexOAuthTransform(reqBody, true, false)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	item, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "shell", item["name"])
	require.Equal(t, "fc_1", item["call_id"])
}

func TestApplyCodexOAuthTransform_PreservesMCPToolCallIDAndName(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"input": []any{
			map[string]any{
				"type":      "mcp_tool_call",
				"call_id":   "call_abc",
				"name":      "remote_tool",
				"arguments": "{}",
			},
		},
	}

	applyCodexOAuthTransform(reqBody, true, false)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	item, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "mcp_tool_call", item["type"])
	require.Equal(t, "remote_tool", item["name"])
	require.Equal(t, "fc_abc", item["call_id"])
}

func TestCodexInputItemRequiresNameTypesAllowCallID(t *testing.T) {
	for _, typ := range []string{"function_call", "custom_tool_call", "mcp_tool_call"} {
		require.True(t, codexInputItemRequiresName(typ), typ)
		require.True(t, isCodexToolCallItemType(typ), typ)
	}
}

func TestSanitizeCodexToolName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"shell", "shell"},
		{"my_tool-1", "my_tool-1"},
		{"read.file", "read_file"},
		{"with space", "with_space"},
		{"net:fetch", "net_fetch"},
		{"namespace.sub.tool/v2", "namespace_sub_tool_v2"},
		{"工具", "__"},
		{"  trimmed  ", "trimmed"},
		{"", ""},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, sanitizeCodexToolName(tc.in), tc.in)
	}
}

func TestApplyCodexOAuthTransform_SanitizesInvalidToolNamesAcrossTooling(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.5",
		"tools": []any{
			map[string]any{
				"type": "function",
				"name": "read.file",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": "net:fetch",
					"parameters": map[string]any{
						"type":       "object",
						"properties": map[string]any{},
					},
				},
			},
		},
		"tool_choice": map[string]any{
			"type": "function",
			"name": "read.file",
		},
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "go"},
			map[string]any{
				"type":      "function_call",
				"call_id":   "call_1",
				"name":      "read.file",
				"arguments": "{}",
			},
			map[string]any{
				"type":      "mcp_tool_call",
				"call_id":   "call_2",
				"name":      "namespace.sub.tool/v2",
				"arguments": "{}",
			},
		},
	}

	applyCodexOAuthTransform(reqBody, true, false)

	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 2)
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		require.True(t, ok)
		name, _ := tool["name"].(string)
		require.Regexp(t, `^[a-zA-Z0-9_-]+$`, name)
		if fn, ok := tool["function"].(map[string]any); ok {
			fnName, _ := fn["name"].(string)
			require.Regexp(t, `^[a-zA-Z0-9_-]+$`, fnName)
		}
	}

	choice, ok := reqBody["tool_choice"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function", choice["type"])
	require.Equal(t, "read_file", choice["name"])

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 3)

	fnCall, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function_call", fnCall["type"])
	require.Equal(t, "read_file", fnCall["name"])

	mcpCall, ok := input[2].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "mcp_tool_call", mcpCall["type"])
	require.Equal(t, "namespace_sub_tool_v2", mcpCall["name"])
}

func TestApplyCodexOAuthTransform_DropsToolChoiceWhenSanitizedNameMissing(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.5",
		"tools": []any{
			map[string]any{"type": "function", "name": "shell"},
		},
		"tool_choice": map[string]any{
			"type": "function",
			"name": "工具",
		},
	}

	applyCodexOAuthTransform(reqBody, true, false)

	require.Equal(t, "auto", reqBody["tool_choice"])
}

func TestApplyCodexOAuthTransform_ExplicitStoreFalsePreserved(t *testing.T) {
	// 续链场景：显式 store=false 不再强制为 true，保持 false。

	reqBody := map[string]any{
		"model": "gpt-5.1",
		"store": false,
		"input": []any{
			map[string]any{"type": "function_call_output", "call_id": "call_1"},
		},
		"tool_choice": "auto",
	}

	applyCodexOAuthTransform(reqBody, false, false)

	store, ok := reqBody["store"].(bool)
	require.True(t, ok)
	require.False(t, store)
}

func TestApplyCodexOAuthTransform_ExplicitStoreTrueForcedFalse(t *testing.T) {
	// 显式 store=true 也会强制为 false。

	reqBody := map[string]any{
		"model": "gpt-5.1",
		"store": true,
		"input": []any{
			map[string]any{"type": "function_call_output", "call_id": "call_1"},
		},
		"tool_choice": "auto",
	}

	applyCodexOAuthTransform(reqBody, false, false)

	store, ok := reqBody["store"].(bool)
	require.True(t, ok)
	require.False(t, store)
}

func TestApplyCodexOAuthTransform_CompactForcesNonStreaming(t *testing.T) {
	reqBody := map[string]any{
		"model":  "gpt-5.1-codex",
		"store":  true,
		"stream": true,
	}

	result := applyCodexOAuthTransform(reqBody, true, true)

	_, hasStore := reqBody["store"]
	require.False(t, hasStore)
	_, hasStream := reqBody["stream"]
	require.False(t, hasStream)
	require.True(t, result.Modified)
}

func TestApplyCodexOAuthTransform_NonContinuationDefaultsStoreFalseAndStripsIDs(t *testing.T) {
	// 非续链场景：未设置 store 时默认 false，并移除 input 中的 id。

	reqBody := map[string]any{
		"model": "gpt-5.1",
		"input": []any{
			map[string]any{"type": "text", "id": "t1", "text": "hi"},
		},
	}

	applyCodexOAuthTransform(reqBody, false, false)

	store, ok := reqBody["store"].(bool)
	require.True(t, ok)
	require.False(t, store)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	// 校验 input[0] 为 map，避免类型不匹配触发 errcheck。
	item, ok := input[0].(map[string]any)
	require.True(t, ok)
	_, hasID := item["id"]
	require.False(t, hasID)
}

func TestFilterCodexInput_RemovesItemReferenceWhenNotPreserved(t *testing.T) {
	input := []any{
		map[string]any{"type": "item_reference", "id": "ref1"},
		map[string]any{"type": "text", "id": "t1", "text": "hi"},
	}

	filtered := filterCodexInput(input, false)
	require.Len(t, filtered, 1)
	// 校验 filtered[0] 为 map，确保字段检查可靠。
	item, ok := filtered[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "text", item["type"])
	_, hasID := item["id"]
	require.False(t, hasID)
}

func TestApplyCodexOAuthTransform_NormalizeCodexTools_PreservesResponsesFunctionTools(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.1",
		"tools": []any{
			map[string]any{
				"type":        "function",
				"name":        "bash",
				"description": "desc",
				"parameters":  map[string]any{"type": "object"},
			},
			map[string]any{
				"type":     "function",
				"function": nil,
			},
		},
	}

	applyCodexOAuthTransform(reqBody, false, false)

	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)

	first, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function", first["type"])
	require.Equal(t, "bash", first["name"])
}

func TestApplyCodexOAuthTransform_NormalizeCodexTools_FillsMissingParameters(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.1",
		"tools": []any{
			map[string]any{
				"type": "function",
				"name": "builtin_web_search",
			},
		},
	}

	applyCodexOAuthTransform(reqBody, false, false)

	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)

	first, ok := tools[0].(map[string]any)
	require.True(t, ok)
	params, ok := first["parameters"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "object", params["type"])
	_, hasProperties := params["properties"]
	require.True(t, hasProperties)
}

func TestNormalizeOpenAIResponsesImageGenerationTools_RewritesLegacyFields(t *testing.T) {
	reqBody := map[string]any{
		"tools": []any{
			map[string]any{
				"type":        "image_generation",
				"format":      "png",
				"compression": 60,
			},
		},
	}

	modified := normalizeOpenAIResponsesImageGenerationTools(reqBody)
	require.True(t, modified)

	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	first, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "png", first["output_format"])
	require.Equal(t, 60, first["output_compression"])
	_, hasFormat := first["format"]
	require.False(t, hasFormat)
	_, hasCompression := first["compression"]
	require.False(t, hasCompression)
}

func TestNormalizeOpenAIResponsesImageGenerationTools_RemovesUnsupportedImageField(t *testing.T) {
	reqBody := map[string]any{
		"tools": []any{
			map[string]any{
				"type":  "image_generation",
				"image": "data:image/png;base64,aGVsbG8=",
				"size":  "1024x1024",
			},
		},
	}

	modified := normalizeOpenAIResponsesImageGenerationTools(reqBody)
	require.True(t, modified)

	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	first, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "image_generation", first["type"])
	require.Equal(t, "1024x1024", first["size"])
	require.NotContains(t, first, "image")
}

func TestNormalizeOpenAIResponsesImageGenerationTools_AutoCorrectsInvalidSize(t *testing.T) {
	reqBody := map[string]any{
		"tools": []any{
			map[string]any{
				"type": "image_generation",
				"size": "2048x1153",
			},
		},
	}

	modified := normalizeOpenAIResponsesImageGenerationTools(reqBody)
	require.True(t, modified)

	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	first, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "2048x1152", first["size"])
}

func TestEnsureOpenAIResponsesImageGenerationTool_NoTools(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"input": "draw a cat",
	}

	modified := ensureOpenAIResponsesImageGenerationTool(reqBody)
	require.True(t, modified)

	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "image_generation", tool["type"])
	require.Equal(t, "png", tool["output_format"])
}

func TestEnsureOpenAIResponsesImageGenerationTool_SkipsSpark(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.3-codex-spark",
		"input": "draw a cat",
	}

	modified := ensureOpenAIResponsesImageGenerationTool(reqBody)
	require.False(t, modified)
	require.NotContains(t, reqBody, "tools")
}

func TestEnsureOpenAIResponsesImageGenerationTool_AppendsToExistingTools(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"tools": []any{
			map[string]any{"type": "web_search"},
		},
	}

	modified := ensureOpenAIResponsesImageGenerationTool(reqBody)
	require.True(t, modified)

	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 2)
	first, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "web_search", first["type"])
	second, ok := tools[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "image_generation", second["type"])
	require.Equal(t, "png", second["output_format"])
}

func TestEnsureOpenAIResponsesImageGenerationTool_PreservesExistingImageTool(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"tools": []any{
			map[string]any{"type": "image_generation", "output_format": "webp"},
			map[string]any{"type": "web_search"},
		},
	}

	modified := ensureOpenAIResponsesImageGenerationTool(reqBody)
	require.False(t, modified)

	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 2)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "webp", tool["output_format"])
}

func TestApplyCodexImageGenerationBridgeInstructions_AppendsBridgeOnce(t *testing.T) {
	reqBody := map[string]any{
		"model":        "gpt-5.4",
		"instructions": "existing instructions",
		"tools": []any{
			map[string]any{"type": "image_generation", "output_format": "png"},
		},
	}

	modified := applyCodexImageGenerationBridgeInstructions(reqBody)
	require.True(t, modified)

	instructions, ok := reqBody["instructions"].(string)
	require.True(t, ok)
	require.Contains(t, instructions, "existing instructions")
	require.Contains(t, instructions, codexImageGenerationBridgeMarker)
	require.Contains(t, instructions, "Responses native `image_generation` tool")

	modified = applyCodexImageGenerationBridgeInstructions(reqBody)
	require.False(t, modified)
}

func TestApplyCodexImageGenerationBridgeInstructions_SkipsSpark(t *testing.T) {
	reqBody := map[string]any{
		"model":        "gpt-5.3-codex-spark",
		"instructions": "existing instructions",
		"tools": []any{
			map[string]any{"type": "image_generation", "output_format": "png"},
		},
	}

	modified := applyCodexImageGenerationBridgeInstructions(reqBody)
	require.False(t, modified)
	require.Equal(t, "existing instructions", reqBody["instructions"])
}

func TestApplyCodexImageGenerationBridgeInstructions_SkipsWithoutImageTool(t *testing.T) {
	reqBody := map[string]any{
		"instructions": "existing instructions",
		"tools": []any{
			map[string]any{"type": "web_search"},
		},
	}

	modified := applyCodexImageGenerationBridgeInstructions(reqBody)
	require.False(t, modified)
	require.Equal(t, "existing instructions", reqBody["instructions"])
}

func TestValidateCodexSparkInputRejectsInputImage(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.3-codex-spark",
		"input": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "describe"},
					map[string]any{"type": "input_image", "image_url": "data:image/png;base64,aGVsbG8="},
				},
			},
		},
	}

	err := validateCodexSparkInput(reqBody, "gpt-5.3-codex-spark")
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not support image input")
}

func TestValidateCodexSparkInputRejectsChatImageURL(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.3-codex-spark",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "describe"},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,aGVsbG8="}},
				},
			},
		},
	}

	err := validateCodexSparkInput(reqBody, "gpt-5.3-codex-spark")
	require.Error(t, err)
}

func TestValidateCodexSparkInputAllowsTextOnly(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.3-codex-spark",
		"input": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "hello"},
				},
			},
		},
	}

	require.NoError(t, validateCodexSparkInput(reqBody, "gpt-5.3-codex-spark"))
}

func TestApplyCodexOAuthTransform_AddsSparkImageUnsupportedInstructions(t *testing.T) {
	reqBody := map[string]any{
		"model":        "gpt-5.3-codex-spark",
		"instructions": "existing instructions",
		"input":        "hello",
	}

	result := applyCodexOAuthTransform(reqBody, true, false)
	require.True(t, result.Modified)

	instructions, ok := reqBody["instructions"].(string)
	require.True(t, ok)
	require.Contains(t, instructions, "existing instructions")
	require.Contains(t, instructions, codexSparkImageUnsupportedMarker)
	require.Contains(t, instructions, "does not support image generation")
	require.Contains(t, instructions, "switch to a non-Spark Codex model")
	require.NotContains(t, instructions, codexImageGenerationBridgeMarker)
}

func TestApplyCodexOAuthTransform_DoesNotAddSparkImageUnsupportedForNonSpark(t *testing.T) {
	reqBody := map[string]any{
		"model":        "gpt-5.4",
		"instructions": "existing instructions",
		"input":        "hello",
	}

	applyCodexOAuthTransform(reqBody, true, false)
	instructions, ok := reqBody["instructions"].(string)
	require.True(t, ok)
	require.NotContains(t, instructions, codexSparkImageUnsupportedMarker)
}

// gpt-5.3-codex-spark rejects the image_generation tool upstream (HTTP 400
// invalid_request_error, param=tools). Codex CLI advertises that tool by default,
// so the OAuth transform must strip it for spark while keeping the rest.
func TestApplyCodexOAuthTransform_StripsImageGenerationToolForSpark(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.3-codex-spark",
		"input": "hello",
		"tools": []any{
			map[string]any{"type": "function", "name": "shell"},
			map[string]any{"type": "image_generation", "output_format": "png"},
		},
	}

	result := applyCodexOAuthTransform(reqBody, true, false)
	require.True(t, result.Modified)
	require.False(t, hasOpenAIImageGenerationTool(reqBody))

	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	first, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function", first["type"])
	require.Equal(t, "shell", first["name"])
}

// Spark reasoning-effort aliases (e.g. -low/-high) normalize to gpt-5.3-codex-spark,
// so they must be stripped too.
func TestApplyCodexOAuthTransform_StripsImageGenerationToolForSparkAlias(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.3-codex-spark-high",
		"input": "hello",
		"tools": []any{
			map[string]any{"type": "image_generation", "output_format": "png"},
		},
	}

	result := applyCodexOAuthTransform(reqBody, true, false)
	require.True(t, result.Modified)
	require.False(t, hasOpenAIImageGenerationTool(reqBody))
	// tools became empty after stripping the only entry; the key is dropped.
	_, hasTools := reqBody["tools"]
	require.False(t, hasTools)
}

// Non-spark Codex models support image_generation; the tool must be preserved.
func TestApplyCodexOAuthTransform_KeepsImageGenerationToolForNonSpark(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.3-codex",
		"input": "hello",
		"tools": []any{
			map[string]any{"type": "image_generation", "output_format": "png"},
		},
	}

	applyCodexOAuthTransform(reqBody, true, false)
	require.True(t, hasOpenAIImageGenerationTool(reqBody))
}

func TestNormalizeOpenAIResponsesImageOnlyModel_BuildsImageToolRequest(t *testing.T) {
	reqBody := map[string]any{
		"model":         "gpt-image-2",
		"prompt":        "draw a cat",
		"size":          "1024x1024",
		"output_format": "png",
		"style":         "vivid",
	}

	modified := normalizeOpenAIResponsesImageOnlyModel(reqBody, "")
	require.True(t, modified)
	require.Equal(t, openAIImagesResponsesMainModel, reqBody["model"])
	require.Equal(t, "draw a cat", reqBody["input"])
	_, hasPrompt := reqBody["prompt"]
	require.False(t, hasPrompt)
	_, hasTopLevelSize := reqBody["size"]
	require.False(t, hasTopLevelSize)

	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "image_generation", tool["type"])
	require.Equal(t, "gpt-image-2", tool["model"])
	require.Equal(t, "1024x1024", tool["size"])
	require.Equal(t, "png", tool["output_format"])
	_, hasStyle := tool["style"]
	require.False(t, hasStyle)
	_, hasTopLevelStyle := reqBody["style"]
	require.False(t, hasTopLevelStyle)

	choice, ok := reqBody["tool_choice"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "image_generation", choice["type"])
}

func TestNormalizeOpenAIResponsesImageOnlyModel_PreservesExistingImageTool(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-image-2",
		"input": "draw a cat",
		"tools": []any{
			map[string]any{
				"type":  "image_generation",
				"model": "gpt-image-1.5",
			},
		},
		"tool_choice": "auto",
	}

	modified := normalizeOpenAIResponsesImageOnlyModel(reqBody, "gpt-5.4")
	require.True(t, modified)
	require.Equal(t, "gpt-5.4", reqBody["model"])
	require.Equal(t, "auto", reqBody["tool_choice"])

	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "gpt-image-1.5", tool["model"])
}

func TestValidateOpenAIImageMainModel_RejectsSpark(t *testing.T) {
	err := ValidateOpenAIImageMainModel("gpt-5.3-codex-spark")
	require.ErrorContains(t, err, "does not support image_generation")
}

func TestValidateOpenAIResponsesImageModel_RejectsImageOnlyModel(t *testing.T) {
	err := validateOpenAIResponsesImageModel(map[string]any{
		"tools": []any{
			map[string]any{"type": "image_generation"},
		},
	}, "gpt-image-2")

	require.ErrorContains(t, err, `/v1/responses image_generation requests require a Responses-capable text model`)
}

func TestApplyCodexOAuthTransform_EmptyInput(t *testing.T) {
	// 空 input 应保持为空且不触发异常。

	reqBody := map[string]any{
		"model": "gpt-5.1",
		"input": []any{},
	}

	applyCodexOAuthTransform(reqBody, false, false)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 0)
}

func TestApplyCodexOAuthTransform_StripsCodexMessageNoiseFields(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.2",
		"input": []any{
			map[string]any{
				"id":                nil,
				"cwd":               nil,
				"name":              nil,
				"role":              "user",
				"type":              "message",
				"input":             nil,
				"action":            nil,
				"output":            nil,
				"stderr":            nil,
				"stdout":            nil,
				"call_id":           nil,
				"changes":           nil,
				"command":           nil,
				"content":           "health check",
				"thought":           nil,
				"arguments":         nil,
				"encrypted_content": nil,
				"reasoning_content": nil,
				"thought_signature": "[REDACTED]",
				"working_directory": nil,
			},
		},
	}

	result := applyCodexOAuthTransform(reqBody, true, false)

	require.True(t, result.Modified)
	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)

	msg, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "message", msg["type"])
	require.Equal(t, "user", msg["role"])
	require.Equal(t, "health check", msg["content"])
	require.NotContains(t, msg, "action")
	require.NotContains(t, msg, "cwd")
	require.NotContains(t, msg, "command")
	require.NotContains(t, msg, "stdout")
	require.NotContains(t, msg, "thought_signature")
}

func TestNormalizeCodexModel_Gpt53(t *testing.T) {
	cases := map[string]string{
		"gpt-5.4":                   "gpt-5.4",
		"gpt5.5":                    "gpt-5.5",
		"openai/gpt5.5":             "gpt-5.5",
		"gpt-5.5-pro":               "gpt-5.5-pro",
		"gpt5.5-pro":                "gpt-5.5-pro",
		"openai/gpt5.5-pro":         "gpt-5.5-pro",
		"gpt-5.5-pro-high":          "gpt-5.5-pro",
		"codex-auto-review":         "codex-auto-review",
		"gpt5.4":                    "gpt-5.4",
		"gpt-5.4-high":              "gpt-5.4",
		"gpt-5.4-chat-latest":       "gpt-5.4",
		"gpt 5.4":                   "gpt-5.4",
		"gpt-5.4-mini":              "gpt-5.4-mini",
		"gpt5.4-mini":               "gpt-5.4-mini",
		"gpt5.4mini":                "gpt-5.4-mini",
		"gpt 5.4 mini":              "gpt-5.4-mini",
		"gpt-5.3":                   "gpt-5.3-codex",
		"gpt5.3":                    "gpt-5.3-codex",
		"gpt-5.3-codex":             "gpt-5.3-codex",
		"gpt5.3-codex":              "gpt-5.3-codex",
		"gpt5.3codex":               "gpt-5.3-codex",
		"gpt-5.3-codex-xhigh":       "gpt-5.3-codex",
		"gpt-5.3-codex-spark":       "gpt-5.3-codex-spark",
		"gpt5.3-codex-spark":        "gpt-5.3-codex-spark",
		"gpt5.3codexspark":          "gpt-5.3-codex-spark",
		"gpt 5.3 codex spark":       "gpt-5.3-codex-spark",
		"gpt-5.3-codex-spark-high":  "gpt-5.3-codex-spark",
		"gpt-5.3-codex-spark-xhigh": "gpt-5.3-codex-spark",
		"gpt 5.3 codex":             "gpt-5.3-codex",
	}

	for input, expected := range cases {
		require.Equal(t, expected, normalizeCodexModel(input))
	}
}

func TestNormalizeCodexModel_RemovedModelsFallbackToSupportedTargets(t *testing.T) {
	cases := map[string]string{
		"":                   "gpt-5.4",
		"gpt-5":              "gpt-5.4",
		"gpt-5-mini":         "gpt-5.4",
		"gpt-5-nano":         "gpt-5.4",
		"gpt-5.1":            "gpt-5.4",
		"gpt-5.1-codex":      "gpt-5.3-codex",
		"gpt-5.1-codex-max":  "gpt-5.3-codex",
		"gpt-5.1-codex-mini": "gpt-5.3-codex",
		"gpt-5.2-codex":      "gpt-5.2",
		"codex-mini-latest":  "gpt-5.3-codex",
		"gpt-5-codex":        "gpt-5.3-codex",
	}

	for input, expected := range cases {
		require.Equal(t, expected, normalizeCodexModel(input))
	}
}

func TestApplyCodexOAuthTransform_PreservesBareSparkModel(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.3-codex-spark",
		"input": []any{},
	}

	result := applyCodexOAuthTransform(reqBody, false, false)

	require.Equal(t, "gpt-5.3-codex-spark", reqBody["model"])
	require.Equal(t, "gpt-5.3-codex-spark", result.NormalizedModel)
	store, ok := reqBody["store"].(bool)
	require.True(t, ok)
	require.False(t, store)
}

func TestApplyCodexOAuthTransform_TrimmedModelWithoutPolicyRewrite(t *testing.T) {
	reqBody := map[string]any{
		"model": "  gpt-5.3-codex-spark  ",
		"input": []any{},
	}

	result := applyCodexOAuthTransform(reqBody, false, false)

	require.Equal(t, "gpt-5.3-codex-spark", reqBody["model"])
	require.Equal(t, "gpt-5.3-codex-spark", result.NormalizedModel)
	require.True(t, result.Modified)
}

func TestApplyCodexOAuthTransform_CodexCLI_PreservesExistingInstructions(t *testing.T) {
	// Codex CLI 场景：已有 instructions 时不修改

	reqBody := map[string]any{
		"model":        "gpt-5.1",
		"instructions": "existing instructions",
	}

	result := applyCodexOAuthTransform(reqBody, true, false) // isCodexCLI=true

	instructions, ok := reqBody["instructions"].(string)
	require.True(t, ok)
	require.Equal(t, "existing instructions", instructions)
	// Modified 仍可能为 true（因为其他字段被修改），但 instructions 应保持不变
	_ = result
}

func TestApplyCodexOAuthTransform_CodexCLI_SuppliesDefaultWhenEmpty(t *testing.T) {
	// Codex CLI 场景：无 instructions 时补充默认值

	reqBody := map[string]any{
		"model": "gpt-5.1",
		// 没有 instructions 字段
	}

	result := applyCodexOAuthTransform(reqBody, true, false) // isCodexCLI=true

	instructions, ok := reqBody["instructions"].(string)
	require.True(t, ok)
	require.NotEmpty(t, instructions)
	require.True(t, result.Modified)
}

func TestApplyCodexOAuthTransform_GPT55SuppliesModelSpecificInstructions(t *testing.T) {
	reqBody := map[string]any{
		"model":        "gpt-5.5",
		"instructions": "   ",
	}

	result := applyCodexOAuthTransform(reqBody, true, false)

	instructions, ok := reqBody["instructions"].(string)
	require.True(t, ok)
	require.Contains(t, instructions, "You are Codex, based on GPT-5")
	require.NotContains(t, instructions, "You are GPT-5.1 running in the Codex CLI")
	require.True(t, result.Modified)
}

func TestApplyCodexOAuthTransform_NonCodexCLI_PreservesExistingInstructions(t *testing.T) {
	// 非 Codex CLI 场景：已有 instructions 时保留客户端的值，不再覆盖

	reqBody := map[string]any{
		"model":        "gpt-5.1",
		"instructions": "old instructions",
	}

	applyCodexOAuthTransform(reqBody, false, false) // isCodexCLI=false

	instructions, ok := reqBody["instructions"].(string)
	require.True(t, ok)
	require.Equal(t, "old instructions", instructions)
}

func TestApplyCodexOAuthTransform_StringInputConvertedToArray(t *testing.T) {
	reqBody := map[string]any{"model": "gpt-5.4", "input": "Hello, world!"}
	result := applyCodexOAuthTransform(reqBody, false, false)
	require.True(t, result.Modified)
	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	msg, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "message", msg["type"])
	require.Equal(t, "user", msg["role"])
	require.Equal(t, "Hello, world!", msg["content"])
}

func TestApplyCodexOAuthTransform_EmptyStringInputBecomesEmptyArray(t *testing.T) {
	reqBody := map[string]any{"model": "gpt-5.4", "input": ""}
	result := applyCodexOAuthTransform(reqBody, false, false)
	require.True(t, result.Modified)
	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 0)
}

func TestApplyCodexOAuthTransform_WhitespaceStringInputBecomesEmptyArray(t *testing.T) {
	reqBody := map[string]any{"model": "gpt-5.4", "input": "   "}
	result := applyCodexOAuthTransform(reqBody, false, false)
	require.True(t, result.Modified)
	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 0)
}

func TestApplyCodexOAuthTransform_StringInputWithToolsField(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"input": "Run the tests",
		"tools": []any{map[string]any{"type": "function", "name": "bash"}},
	}
	applyCodexOAuthTransform(reqBody, false, false)
	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
}

func TestExtractSystemMessagesFromInput(t *testing.T) {
	t.Run("no system messages", func(t *testing.T) {
		reqBody := map[string]any{
			"input": []any{
				map[string]any{"role": "user", "content": "hello"},
			},
		}
		result := extractSystemMessagesFromInput(reqBody)
		require.False(t, result)
		input, ok := reqBody["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 1)
		_, hasInstructions := reqBody["instructions"]
		require.False(t, hasInstructions)
	})

	t.Run("string content system message", func(t *testing.T) {
		reqBody := map[string]any{
			"input": []any{
				map[string]any{"role": "system", "content": "You are an assistant."},
				map[string]any{"role": "user", "content": "hello"},
			},
		}
		result := extractSystemMessagesFromInput(reqBody)
		require.True(t, result)
		input, ok := reqBody["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 2)
		msg, ok := input[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "developer", msg["role"])
		require.Equal(t, "You are an assistant.", msg["content"])
		user, ok := input[1].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "user", user["role"])
		require.Equal(t, "You are an assistant.", reqBody["instructions"])
	})

	t.Run("array content system message", func(t *testing.T) {
		reqBody := map[string]any{
			"input": []any{
				map[string]any{
					"role": "system",
					"content": []any{
						map[string]any{"type": "text", "text": "Be helpful."},
					},
				},
			},
		}
		result := extractSystemMessagesFromInput(reqBody)
		require.True(t, result)
		require.Equal(t, "Be helpful.", reqBody["instructions"])
		input, ok := reqBody["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 1)
		msg, ok := input[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "developer", msg["role"])
		require.Equal(t, []any{
			map[string]any{"type": "text", "text": "Be helpful."},
		}, msg["content"])
		require.Equal(t, "Be helpful.", reqBody["instructions"])
	})

	t.Run("multiple system messages concatenated", func(t *testing.T) {
		reqBody := map[string]any{
			"input": []any{
				map[string]any{"role": "system", "content": "First."},
				map[string]any{"role": "system", "content": "Second."},
				map[string]any{"role": "user", "content": "hi"},
			},
		}
		result := extractSystemMessagesFromInput(reqBody)
		require.True(t, result)
		require.Equal(t, "First.\n\nSecond.", reqBody["instructions"])
		input, ok := reqBody["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 3)
		first, ok := input[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "developer", first["role"])
		second, ok := input[1].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "developer", second["role"])
		user, ok := input[2].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "user", user["role"])
		require.Equal(t, "First.\n\nSecond.", reqBody["instructions"])
	})

	t.Run("mixed system and non-system preserves non-system", func(t *testing.T) {
		reqBody := map[string]any{
			"input": []any{
				map[string]any{"role": "user", "content": "hello"},
				map[string]any{"role": "system", "content": "Sys prompt."},
				map[string]any{"role": "assistant", "content": "Hi there"},
			},
		}
		result := extractSystemMessagesFromInput(reqBody)
		require.True(t, result)
		input, ok := reqBody["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 3)
		first, ok := input[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "user", first["role"])
		second, ok := input[1].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "developer", second["role"])
		third, ok := input[2].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "assistant", third["role"])
		require.Equal(t, "Sys prompt.", reqBody["instructions"])
	})

	t.Run("existing instructions prepended", func(t *testing.T) {
		reqBody := map[string]any{
			"input": []any{
				map[string]any{"role": "system", "content": "Extracted."},
				map[string]any{"role": "user", "content": "hi"},
			},
			"instructions": "Existing instructions.",
		}
		result := extractSystemMessagesFromInput(reqBody)
		require.True(t, result)
		require.Equal(t, "Extracted.\n\nExisting instructions.", reqBody["instructions"])
		input, ok := reqBody["input"].([]any)
		require.True(t, ok)
		msg, ok := input[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "developer", msg["role"])
	})
}

// TestApplyCodexOAuthTransform_StripsPromptCacheRetention is a regression
// test: some clients (e.g. Cursor cloud via the Responses-shape compat path)
// send prompt_cache_retention, but the ChatGPT internal Codex endpoint
// rejects it with "Unsupported parameter: prompt_cache_retention".
func TestApplyCodexOAuthTransform_StripsPromptCacheRetention(t *testing.T) {
	reqBody := map[string]any{
		"model":                  "gpt-5.1",
		"prompt_cache_retention": "24h",
		"input": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	}

	applyCodexOAuthTransform(reqBody, false, false)

	_, stillThere := reqBody["prompt_cache_retention"]
	require.False(t, stillThere,
		"prompt_cache_retention must be stripped before forwarding to Codex upstream")
}

func TestApplyCodexOAuthTransform_StripsChatGPTInternalUnsupportedFields(t *testing.T) {
	reqBody := map[string]any{
		"model":                  "gpt-5.4",
		"name":                   "request-name",
		"user":                   "user_123",
		"metadata":               map[string]any{"trace_id": "abc"},
		"prompt_cache_retention": "24h",
		"reasoningSummary":       "auto",
		"safety_identifier":      "sid",
		"stream_options":         map[string]any{"include_usage": true},
		"input": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	}

	result := applyCodexOAuthTransform(reqBody, true, false)

	require.True(t, result.Modified)
	for _, field := range openAIChatGPTInternalUnsupportedFields {
		require.NotContains(t, reqBody, field)
	}
}

func TestApplyCodexOAuthTransform_ExtractsSystemMessages(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.1",
		"input": []any{
			map[string]any{"role": "system", "content": "You are a coding assistant."},
			map[string]any{"role": "user", "content": "Write a function."},
		},
	}

	result := applyCodexOAuthTransform(reqBody, false, false)

	require.True(t, result.Modified)

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)
	system, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "developer", system["role"])
	require.Equal(t, "You are a coding assistant.", system["content"])
	user, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "user", user["role"])
	require.Equal(t, "You are a coding assistant.", reqBody["instructions"])
}

func TestApplyCodexOAuthTransform_JsonObjectKeepsJsonInstructionInInput(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"input": []any{
			map[string]any{
				"role":    "system",
				"content": "You are an assistant. Output JSON only.",
			},
			map[string]any{
				"role":    "user",
				"content": "symbol data without the keyword",
			},
		},
		"text": map[string]any{
			"format": map[string]any{
				"type": "json_object",
			},
		},
	}

	result := applyCodexOAuthTransform(reqBody, false, false)

	require.True(t, result.Modified)
	instructions, ok := reqBody["instructions"].(string)
	require.True(t, ok)
	require.Contains(t, instructions, "JSON")
	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)
	developer, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "developer", developer["role"])
	require.Contains(t, developer["content"], "JSON")
	user, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "user", user["role"])
}

func TestIsInstructionsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		reqBody  map[string]any
		expected bool
	}{
		{"missing field", map[string]any{}, true},
		{"nil value", map[string]any{"instructions": nil}, true},
		{"empty string", map[string]any{"instructions": ""}, true},
		{"whitespace only", map[string]any{"instructions": "   "}, true},
		{"non-string", map[string]any{"instructions": 123}, true},
		{"valid string", map[string]any{"instructions": "hello"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isInstructionsEmpty(tt.reqBody)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeOpenAIStrictFunctionToolSchemas(t *testing.T) {
	reqBody := map[string]any{
		"tools": []any{
			map[string]any{
				"name":   "read_file",
				"type":   "function",
				"strict": true,
				"parameters": map[string]any{
					"type":     "object",
					"required": []any{"filePath"},
					"properties": map[string]any{
						"limit":    map[string]any{"type": "number"},
						"offset":   map[string]any{"type": "number"},
						"filePath": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	modified := normalizeOpenAIStrictFunctionToolSchemas(reqBody)

	require.False(t, modified)
	required := requireFirstToolRequiredFields(t, reqBody)
	require.Equal(t, "filePath", firstNonEmptyString(required[0]))
	require.Len(t, required, 1)
}

func TestNormalizeOpenAIStrictFunctionToolSchemas_CleansInvalidRequiredEntries(t *testing.T) {
	reqBody := map[string]any{
		"tools": []any{
			map[string]any{
				"name":   "read_file",
				"type":   "function",
				"strict": true,
				"parameters": map[string]any{
					"type":     "object",
					"required": []any{"offset", "filePath", "offset", "missing", "  "},
					"properties": map[string]any{
						"limit":    map[string]any{"type": "number"},
						"offset":   map[string]any{"type": "number"},
						"filePath": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	modified := normalizeOpenAIStrictFunctionToolSchemas(reqBody)

	require.True(t, modified)
	required := requireFirstToolRequiredFields(t, reqBody)
	require.Equal(t, []any{"filePath", "offset"}, required)
}

func TestApplyCodexOAuthTransform_NormalizeCodexTools_CoercesNullRequiredToArray(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"tools": []any{
			map[string]any{
				"type": "function",
				"name": "ExitPlanMode",
				"parameters": map[string]any{
					"type":     "object",
					"required": nil,
					"properties": map[string]any{
						"plan": map[string]any{
							"type":     "object",
							"required": nil,
							"properties": map[string]any{
								"summary": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
		},
	}

	result := applyCodexOAuthTransform(reqBody, true, false)

	require.True(t, result.Modified)
	parameters := requireFirstToolParameters(t, reqBody)
	require.Equal(t, []any{}, parameters["required"])
	plan, ok := parameters["properties"].(map[string]any)["plan"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []any{}, plan["required"])
}

func TestApplyCodexOAuthTransform_NormalizeCodexTools_AddsMissingArrayItems(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"tools": []any{
			map[string]any{
				"type": "function",
				"name": "Workflow",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"steps": map[string]any{
							"type": "array",
						},
					},
				},
			},
		},
	}

	result := applyCodexOAuthTransform(reqBody, true, false)

	require.True(t, result.Modified)
	parameters := requireFirstToolParameters(t, reqBody)
	steps, ok := parameters["properties"].(map[string]any)["steps"].(map[string]any)
	require.True(t, ok)
	items, ok := steps["items"].(map[string]any)
	require.True(t, ok)
	require.Empty(t, items)
}

func TestNormalizeOpenAITextFormatSchema_RebuildsRequiredFromProperties(t *testing.T) {
	reqBody := map[string]any{
		"text": map[string]any{
			"format": map[string]any{
				"type": "json_schema",
				"name": "codex_output_schema",
				"schema": map[string]any{
					"type":     "object",
					"required": []any{"action_dispatch_maps"},
					"properties": map[string]any{
						"action_dispatch": map[string]any{"type": "string"},
						"summary":         map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	modified := normalizeOpenAIResponseFormatSchemas(reqBody)

	require.True(t, modified)
	text, ok := reqBody["text"].(map[string]any)
	require.True(t, ok)
	format, ok := text["format"].(map[string]any)
	require.True(t, ok)
	schema, ok := format["schema"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []any{"action_dispatch", "summary"}, schema["required"])
}

func TestNormalizeOpenAITextFormatSchema_AddsMissingArrayItems(t *testing.T) {
	reqBody := map[string]any{
		"text": map[string]any{
			"format": map[string]any{
				"type": "json_schema",
				"name": "semantic_discovery",
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"nodes": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"bbox": map[string]any{
										"type": "array",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	modified := normalizeOpenAIResponseFormatSchemas(reqBody)

	require.True(t, modified)
	text, ok := reqBody["text"].(map[string]any)
	require.True(t, ok)
	format, ok := text["format"].(map[string]any)
	require.True(t, ok)
	schema, ok := format["schema"].(map[string]any)
	require.True(t, ok)
	nodes, ok := schema["properties"].(map[string]any)["nodes"].(map[string]any)
	require.True(t, ok)
	nodeItems, ok := nodes["items"].(map[string]any)
	require.True(t, ok)
	bbox, ok := nodeItems["properties"].(map[string]any)["bbox"].(map[string]any)
	require.True(t, ok)
	bboxItems, ok := bbox["items"].(map[string]any)
	require.True(t, ok)
	require.Empty(t, bboxItems)
}

func requireFirstToolRequiredFields(t *testing.T, reqBody map[string]any) []any {
	t.Helper()

	parameters := requireFirstToolParameters(t, reqBody)

	required, ok := parameters["required"].([]any)
	require.True(t, ok)

	return required
}

func requireFirstToolParameters(t *testing.T, reqBody map[string]any) map[string]any {
	t.Helper()

	tools, ok := reqBody["tools"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, tools)

	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)

	parameters, ok := tool["parameters"].(map[string]any)
	require.True(t, ok)

	return parameters
}

func TestFilterCodexInput_DropsReasoningItemsRegardlessOfPreserveReferences(t *testing.T) {
	// Reasoning items in input[] reference rs_* IDs that were emitted by
	// chatgpt.com under store=false (forced by applyCodexOAuthTransform).
	// They are never persisted upstream, so forwarding them produces a
	// guaranteed 404 ("Item with id 'rs_...' not found"). Drop them
	// regardless of preserveReferences. See: Wei-Shaw/sub2api issue #1957.

	build := func() []any {
		return []any{
			map[string]any{
				"type":              "reasoning",
				"id":                "rs_0672f12450da0b9c0169f07220a6c08198b68c2455ced99344",
				"encrypted_content": "gAAAAAB-enc-payload",
				"summary":           []any{},
			},
		}
	}

	for _, preserve := range []bool{true, false} {
		preserve := preserve
		t.Run(fmt.Sprintf("preserveReferences=%v", preserve), func(t *testing.T) {
			filtered := filterCodexInput(build(), preserve)
			require.Len(t, filtered, 1)

			item, ok := filtered[0].(map[string]any)
			require.True(t, ok)
			// Contract 2: the reasoning item survives the filter.
			require.Equal(t, "reasoning", item["type"])
			// Contract 2: encrypted_content (cross-turn channel) preserved verbatim.
			require.Equal(t, "gAAAAAB-enc-payload", item["encrypted_content"])
			// Contract 1/3: rs_* id stripped unconditionally, even when
			// PreserveReferences=true (id lookup, not the item, triggers the 404).
			_, hasID := item["id"]
			require.False(t, hasID)
			// summary passed through untouched.
			summary, ok := item["summary"].([]any)
			require.True(t, ok)
			require.Len(t, summary, 0)
		})
	}
}

// TestFilterCodexInput_BareReasoningStripsIDBackfillsSummary covers contract 1
// plus 5: a reasoning item carrying only an rs_* id (no encrypted_content) is
// kept as an empty shell with the id stripped, and a missing summary is
// backfilled to [] so upstream does not reject it with 400 "Missing required
// parameter 'input[N].summary'". Verified against chatgpt.com codex (gpt-5.5).
func TestFilterCodexInput_BareReasoningStripsIDBackfillsSummary(t *testing.T) {
	input := []any{
		map[string]any{
			"type": "reasoning",
			"id":   "rs_0672f12450da0b9c0169f07220a6c08198b68c2455ced99344",
		},
	}

	filtered := filterCodexInput(input, false)
	require.Len(t, filtered, 1)

	item, ok := filtered[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "reasoning", item["type"])
	// Contract 1: id stripped.
	_, hasID := item["id"]
	require.False(t, hasID)
	// Contract 5: summary backfilled to an empty array.
	summary, ok := item["summary"].([]any)
	require.True(t, ok)
	require.Len(t, summary, 0)
}

// TestFilterCodexInput_ReasoningBackfillsMissingSummary isolates contract 5:
// even when a reasoning item carries other content (here encrypted_content),
// a missing summary field is always added as [] before forwarding upstream.
func TestFilterCodexInput_ReasoningBackfillsMissingSummary(t *testing.T) {
	input := []any{
		map[string]any{
			"type":              "reasoning",
			"id":                "rs_abc",
			"encrypted_content": "gAAAAAB-enc",
		},
	}

	filtered := filterCodexInput(input, false)
	require.Len(t, filtered, 1)

	item, ok := filtered[0].(map[string]any)
	require.True(t, ok)
	summary, ok := item["summary"].([]any)
	require.True(t, ok)
	require.Len(t, summary, 0)
	// encrypted_content still preserved alongside the backfilled summary.
	require.Equal(t, "gAAAAAB-enc", item["encrypted_content"])
}

// TestFilterCodexInput_PreservesReasoningSummaryAndContent verifies that a
// non-empty summary is not overwritten and that arbitrary reasoning fields
// (e.g. content) survive verbatim — only the id is removed.
func TestFilterCodexInput_PreservesReasoningSummaryAndContent(t *testing.T) {
	summary := []any{
		map[string]any{"type": "summary_text", "text": "Considered the options."},
	}
	content := []any{
		map[string]any{"type": "reasoning_text", "text": "internal chain"},
	}
	input := []any{
		map[string]any{
			"type":              "reasoning",
			"id":                "rs_abc",
			"summary":           summary,
			"content":           content,
			"encrypted_content": "gAAAAAB-enc",
		},
	}

	filtered := filterCodexInput(input, false)
	require.Len(t, filtered, 1)

	item, ok := filtered[0].(map[string]any)
	require.True(t, ok)
	// Non-empty summary preserved verbatim (not replaced with []).
	require.Equal(t, summary, item["summary"])
	// content preserved verbatim.
	require.Equal(t, content, item["content"])
	require.Equal(t, "gAAAAAB-enc", item["encrypted_content"])
	_, hasID := item["id"]
	require.False(t, hasID)
}

// TestFilterCodexInput_PreservesReasoningInMixedInput exercises contract 7:
// reasoning items are stripped of their rs_* ids but kept (with
// encrypted_content) while message / function_call / function_call_output
// items flow through unchanged, with tool-call pairing (call_id) intact.
func TestFilterCodexInput_PreservesReasoningInMixedInput(t *testing.T) {
	build := func() []any {
		return []any{
			map[string]any{"type": "message", "id": "msg_0", "role": "user", "content": "hi"},
			map[string]any{
				"type":              "reasoning",
				"id":                "rs_1",
				"encrypted_content": "gAAAAAB-enc-1",
				"summary":           []any{},
			},
			map[string]any{
				"type":    "reasoning",
				"id":      "rs_2",
				"summary": []any{},
			},
			// call_id already in fc_ form so the unrelated call_->fc_
			// normalization does not obscure the pairing assertion.
			map[string]any{"type": "function_call", "id": "fc_1", "call_id": "fc_1", "name": "tool", "arguments": "{}"},
			map[string]any{"type": "function_call_output", "call_id": "fc_1", "output": "{}"},
		}
	}

	for _, preserve := range []bool{true, false} {
		preserve := preserve
		t.Run(fmt.Sprintf("preserveReferences=%v", preserve), func(t *testing.T) {
			filtered := filterCodexInput(build(), preserve)
			// Nothing is dropped: both reasoning items are now preserved.
			require.Len(t, filtered, 5)

			byType := make(map[string][]map[string]any)
			for _, raw := range filtered {
				item, ok := raw.(map[string]any)
				require.True(t, ok)
				typ, _ := item["type"].(string)
				byType[typ] = append(byType[typ], item)
				// No surviving item may carry an rs_* id.
				if id, ok := item["id"].(string); ok {
					require.False(t, strings.HasPrefix(id, "rs_"),
						"no item carrying an rs_* id should survive the filter")
				}
			}

			// Both reasoning items kept, ids stripped, summary present.
			require.Len(t, byType["reasoning"], 2)
			for _, r := range byType["reasoning"] {
				_, hasID := r["id"]
				require.False(t, hasID)
				_, hasSummary := r["summary"]
				require.True(t, hasSummary)
			}
			require.Equal(t, "gAAAAAB-enc-1", byType["reasoning"][0]["encrypted_content"])

			// message / function_call(+output) untouched by reasoning handling.
			require.Len(t, byType["message"], 1)
			// Contract 7: tool-call pairing by call_id is unaffected.
			require.Len(t, byType["function_call"], 1)
			require.Equal(t, "fc_1", byType["function_call"][0]["call_id"])
			require.Len(t, byType["function_call_output"], 1)
			require.Equal(t, "fc_1", byType["function_call_output"][0]["call_id"])
		})
	}
}
