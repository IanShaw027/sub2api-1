package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ChatCompletionsToResponses tests
// ---------------------------------------------------------------------------

func TestChatCompletionsToResponses_BasicText(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4o", resp.Model)
	assert.True(t, resp.Stream) // always forced true
	assert.False(t, *resp.Store)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)
	assert.Equal(t, "user", items[0].Role)
}

func TestChatCompletionsToResponses_SystemMessage(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "system", Content: json.RawMessage(`"You are helpful."`)},
			{Role: "user", Content: json.RawMessage(`"Hi"`)},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 2)
	assert.Equal(t, "system", items[0].Role)
	assert.Equal(t, "user", items[1].Role)
}

func TestChatCompletionsToResponses_ToolCalls(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Call the function"`)},
			{
				Role: "assistant",
				ToolCalls: []ChatToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: ChatFunctionCall{
							Name:      "ping",
							Arguments: `{"host":"example.com"}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "call_1",
				Content:    json.RawMessage(`"pong"`),
			},
		},
		Tools: []ChatTool{
			{
				Type: "function",
				Function: &ChatFunction{
					Name:        "ping",
					Description: "Ping a host",
					Parameters:  json.RawMessage(`{"type":"object"}`),
				},
			},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	// user + function_call + function_call_output = 3
	// (assistant message with empty content + tool_calls → only function_call items emitted)
	require.Len(t, items, 3)

	// Check function_call item
	assert.Equal(t, "function_call", items[1].Type)
	assert.Equal(t, "call_1", items[1].CallID)
	assert.Empty(t, items[1].ID)
	assert.Equal(t, "ping", items[1].Name)

	// Check function_call_output item
	assert.Equal(t, "function_call_output", items[2].Type)
	assert.Equal(t, "call_1", items[2].CallID)
	assert.Equal(t, "pong", items[2].Output)

	// Check tools
	require.Len(t, resp.Tools, 1)
	assert.Equal(t, "function", resp.Tools[0].Type)
	assert.Equal(t, "ping", resp.Tools[0].Name)
}

func TestChatCompletionsToResponses_FunctionToolParametersNormalized(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Call the function"`)},
		},
		Tools: []ChatTool{
			{
				Type: "function",
				Function: &ChatFunction{
					Name:        "builtin_web_search",
					Description: "Search the web",
				},
			},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	require.Len(t, resp.Tools, 1)
	assert.Equal(t, "function", resp.Tools[0].Type)
	assert.Equal(t, "builtin_web_search", resp.Tools[0].Name)
	assert.JSONEq(t, `{"type":"object","properties":{}}`, string(resp.Tools[0].Parameters))
}

func TestChatCompletionsToResponses_FunctionToolDefaultsStrictFalse(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Call the function"`)},
		},
		Tools: []ChatTool{
			{
				Type: "function",
				Function: &ChatFunction{
					Name:       "ping",
					Parameters: json.RawMessage(`{"type":"object"}`),
				},
			},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	require.Len(t, resp.Tools, 1)
	require.NotNil(t, resp.Tools[0].Strict)
	assert.False(t, *resp.Tools[0].Strict)
}

func TestChatCompletionsToResponses_MapsNativeServerTools(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Search the web"`)},
		},
		Tools: []ChatTool{
			{Type: "web_search"},
			{Type: "google_search"},
			{Type: "web_fetch"},
			{
				Type: "function",
				Function: &ChatFunction{
					Name:       "lookup",
					Parameters: json.RawMessage(`{"type":"object"}`),
				},
			},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	require.Len(t, resp.Tools, 4)
	assert.Equal(t, "web_search", resp.Tools[0].Type)
	assert.Equal(t, "google_search", resp.Tools[1].Type)
	assert.Equal(t, "web_fetch", resp.Tools[2].Type)
	assert.Equal(t, "function", resp.Tools[3].Type)
	assert.Equal(t, "lookup", resp.Tools[3].Name)
}

func TestChatCompletionsToResponses_MaxTokens(t *testing.T) {
	t.Run("max_tokens", func(t *testing.T) {
		maxTokens := 100
		req := &ChatCompletionsRequest{
			Model:     "gpt-4o",
			MaxTokens: &maxTokens,
			Messages:  []ChatMessage{{Role: "user", Content: json.RawMessage(`"Hi"`)}},
		}
		resp, err := ChatCompletionsToResponses(req)
		require.NoError(t, err)
		require.NotNil(t, resp.MaxOutputTokens)
		// Below minMaxOutputTokens (128), should be clamped
		assert.Equal(t, minMaxOutputTokens, *resp.MaxOutputTokens)
	})

	t.Run("max_completion_tokens_preferred", func(t *testing.T) {
		maxTokens := 100
		maxCompletion := 500
		req := &ChatCompletionsRequest{
			Model:               "gpt-4o",
			MaxTokens:           &maxTokens,
			MaxCompletionTokens: &maxCompletion,
			Messages:            []ChatMessage{{Role: "user", Content: json.RawMessage(`"Hi"`)}},
		}
		resp, err := ChatCompletionsToResponses(req)
		require.NoError(t, err)
		require.NotNil(t, resp.MaxOutputTokens)
		assert.Equal(t, 500, *resp.MaxOutputTokens)
	})
}

func TestChatCompletionsToResponses_ReasoningEffort(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model:           "gpt-4o",
		ReasoningEffort: "high",
		Messages:        []ChatMessage{{Role: "user", Content: json.RawMessage(`"Hi"`)}},
	}
	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Reasoning)
	assert.Equal(t, "high", resp.Reasoning.Effort)
	assert.Equal(t, "auto", resp.Reasoning.Summary)
}

func TestChatCompletionsToResponses_ResponseFormatJsonObject(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model:          "gpt-4o",
		Messages:       []ChatMessage{{Role: "user", Content: json.RawMessage(`"Return JSON"`)}},
		ResponseFormat: json.RawMessage(`{"type":"json_object"}`),
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Text)
	assert.JSONEq(t, `{"type":"json_object"}`, string(resp.Text.Format))

	payload, err := json.Marshal(resp)
	require.NoError(t, err)
	var serialized struct {
		Text ResponsesText `json:"text"`
	}
	require.NoError(t, json.Unmarshal(payload, &serialized))
	assert.JSONEq(t, `{"type":"json_object"}`, string(serialized.Text.Format))
}

func TestChatCompletionsToResponses_ResponseFormatJsonSchema(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model:    "gpt-4o",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"Return structured JSON"`)}},
		ResponseFormat: json.RawMessage(`{
			"type":"json_schema",
			"json_schema":{
				"name":"answer",
				"schema":{
					"type":"object",
					"properties":{"ok":{"type":"boolean"}},
					"required":["ok"],
					"additionalProperties":false
				},
				"strict":true
			}
		}`),
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Text)
	assert.JSONEq(t, `{
		"type":"json_schema",
		"name":"answer",
		"schema":{
			"type":"object",
			"properties":{"ok":{"type":"boolean"}},
			"required":["ok"],
			"additionalProperties":false
		},
		"strict":true
	}`, string(resp.Text.Format))
}

func TestChatCompletionsToResponses_ImageURL(t *testing.T) {
	content := `[{"type":"text","text":"Describe this"},{"type":"image_url","image_url":{"url":"data:image/png;base64,abc123"}}]`
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(content)},
		},
	}
	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)

	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[0].Content, &parts))
	require.Len(t, parts, 2)
	assert.Equal(t, "input_text", parts[0].Type)
	assert.Equal(t, "Describe this", parts[0].Text)
	assert.Equal(t, "input_image", parts[1].Type)
	assert.Equal(t, "data:image/png;base64,abc123", parts[1].ImageURL)
}

func TestChatCompletionsToResponses_EmptyBase64ImageURLSkipped(t *testing.T) {
	content := `[{"type":"text","text":"Describe this"},{"type":"image_url","image_url":{"url":"data:image/png;base64,"}}]`
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(content)},
		},
	}
	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)

	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[0].Content, &parts))
	require.Len(t, parts, 1)
	assert.Equal(t, "input_text", parts[0].Type)
	assert.Equal(t, "Describe this", parts[0].Text)
}

func TestChatCompletionsToResponses_WhitespaceOnlyBase64ImageURLSkipped(t *testing.T) {
	content := `[{"type":"text","text":"Describe this"},{"type":"image_url","image_url":{"url":"data:image/png;base64,   "}}]`
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(content)},
		},
	}
	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 1)

	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[0].Content, &parts))
	require.Len(t, parts, 1)
	assert.Equal(t, "input_text", parts[0].Type)
	assert.Equal(t, "Describe this", parts[0].Text)
}

func TestChatCompletionsToResponses_SystemArrayContent(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "system", Content: json.RawMessage(`[{"type":"text","text":"You are a careful visual assistant."}]`)},
			{Role: "user", Content: json.RawMessage(`[{"type":"text","text":"Describe this image"},{"type":"image_url","image_url":{"url":"data:image/png;base64,abc123"}}]`)},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 2)

	var systemParts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[0].Content, &systemParts))
	require.Len(t, systemParts, 1)
	assert.Equal(t, "input_text", systemParts[0].Type)
	assert.Equal(t, "You are a careful visual assistant.", systemParts[0].Text)

	var userParts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[1].Content, &userParts))
	require.Len(t, userParts, 2)
	assert.Equal(t, "input_image", userParts[1].Type)
	assert.Equal(t, "data:image/png;base64,abc123", userParts[1].ImageURL)
}

func TestChatCompletionsToResponses_LegacyFunctions(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hi"`)},
		},
		Functions: []ChatFunction{
			{
				Name:        "get_weather",
				Description: "Get weather",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
		FunctionCall: json.RawMessage(`{"name":"get_weather"}`),
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	require.Len(t, resp.Tools, 1)
	assert.Equal(t, "function", resp.Tools[0].Type)
	assert.Equal(t, "get_weather", resp.Tools[0].Name)

	// tool_choice should be converted
	require.NotNil(t, resp.ToolChoice)
	var tc map[string]any
	require.NoError(t, json.Unmarshal(resp.ToolChoice, &tc))
	assert.Equal(t, "function", tc["type"])
	assert.Equal(t, "get_weather", tc["name"])
	assert.NotContains(t, tc, "function")
}

func TestChatCompletionsToResponses_ServiceTier(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model:       "gpt-4o",
		ServiceTier: "flex",
		Messages:    []ChatMessage{{Role: "user", Content: json.RawMessage(`"Hi"`)}},
	}
	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	assert.Equal(t, "flex", resp.ServiceTier)
}

func TestChatCompletionsToResponses_AssistantWithTextAndToolCalls(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Do something"`)},
			{
				Role:    "assistant",
				Content: json.RawMessage(`"Let me call a function."`),
				ToolCalls: []ChatToolCall{
					{
						ID:   "call_abc",
						Type: "function",
						Function: ChatFunctionCall{
							Name:      "do_thing",
							Arguments: `{}`,
						},
					},
				},
			},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	// user + assistant message (with text) + function_call
	require.Len(t, items, 3)
	assert.Equal(t, "user", items[0].Role)
	assert.Equal(t, "assistant", items[1].Role)
	assert.Equal(t, "function_call", items[2].Type)
	assert.Empty(t, items[2].ID)
}

func TestChatCompletionsToResponses_AssistantArrayContentPreserved(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hi"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"text","text":"A"},{"type":"text","text":"B"}]`)},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 2)
	assert.Equal(t, "assistant", items[1].Role)

	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[1].Content, &parts))
	require.Len(t, parts, 1)
	assert.Equal(t, "output_text", parts[0].Type)
	assert.Equal(t, "AB", parts[0].Text)
}

func TestChatCompletionsToResponses_AssistantThinkingTagPreserved(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hi"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"thinking","thinking":"internal plan"},{"type":"text","text":"final answer"}]`)},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 2)

	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(items[1].Content, &parts))
	require.Len(t, parts, 1)
	assert.Equal(t, "output_text", parts[0].Type)
	assert.Contains(t, parts[0].Text, "<thinking>internal plan</thinking>")
	assert.Contains(t, parts[0].Text, "final answer")
}

func TestChatCompletionsToResponses_LegacyAssistantFunctionCallReplayable(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Call ping"`)},
			{
				Role: "assistant",
				FunctionCall: &ChatFunctionCall{
					Name:      "ping",
					Arguments: `{"host":"example.com"}`,
				},
			},
			{
				Role:    "function",
				Name:    "ping",
				Content: json.RawMessage(`"pong"`),
			},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 3)
	assert.Equal(t, "user", items[0].Role)
	assert.Equal(t, "function_call", items[1].Type)
	assert.Equal(t, "ping", items[1].Name)
	assert.Equal(t, `{"host":"example.com"}`, items[1].Arguments)
	assert.NotEmpty(t, items[1].CallID)
	assert.Equal(t, "function_call_output", items[2].Type)
	assert.Equal(t, items[1].CallID, items[2].CallID)
	assert.Equal(t, "pong", items[2].Output)
}

func TestChatCompletionsToResponses_LegacyAssistantFunctionCallSameNameSequentialCallsKeepDistinctState(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Run ping twice"`)},
			{
				Role: "assistant",
				FunctionCall: &ChatFunctionCall{
					Name:      "ping",
					Arguments: `{"host":"a.example.com"}`,
				},
			},
			{
				Role:    "function",
				Name:    "ping",
				Content: json.RawMessage(`"pong-a"`),
			},
			{
				Role: "assistant",
				FunctionCall: &ChatFunctionCall{
					Name:      "ping",
					Arguments: `{"host":"b.example.com"}`,
				},
			},
			{
				Role:    "function",
				Name:    "ping",
				Content: json.RawMessage(`"pong-b"`),
			},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 5)
	assert.Equal(t, "function_call", items[1].Type)
	assert.Equal(t, "function_call_output", items[2].Type)
	assert.Equal(t, "function_call", items[3].Type)
	assert.Equal(t, "function_call_output", items[4].Type)
	assert.NotEmpty(t, items[1].CallID)
	assert.NotEmpty(t, items[3].CallID)
	assert.NotEqual(t, items[1].CallID, items[3].CallID)
	assert.Equal(t, items[1].CallID, items[2].CallID)
	assert.Equal(t, items[3].CallID, items[4].CallID)
	assert.Equal(t, "pong-a", items[2].Output)
	assert.Equal(t, "pong-b", items[4].Output)
}

// ---------------------------------------------------------------------------
// ResponsesToChatCompletions tests
// ---------------------------------------------------------------------------

func TestResponsesToChatCompletions_BasicText(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_123",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type: "message",
				Content: []ResponsesContentPart{
					{Type: "output_text", Text: "Hello, world!"},
				},
			},
		},
		Usage: &ResponsesUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}

	chat := ResponsesToChatCompletions(resp, "gpt-4o")
	assert.Equal(t, "chat.completion", chat.Object)
	assert.Equal(t, "gpt-4o", chat.Model)
	require.Len(t, chat.Choices, 1)
	assert.Equal(t, "stop", chat.Choices[0].FinishReason)

	var content string
	require.NoError(t, json.Unmarshal(chat.Choices[0].Message.Content, &content))
	assert.Equal(t, "Hello, world!", content)

	require.NotNil(t, chat.Usage)
	assert.Equal(t, 10, chat.Usage.PromptTokens)
	assert.Equal(t, 5, chat.Usage.CompletionTokens)
	assert.Equal(t, 15, chat.Usage.TotalTokens)
}

func TestResponsesToChatCompletions_ToolCalls(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_456",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type:      "function_call",
				CallID:    "call_xyz",
				Name:      "get_weather",
				Arguments: `{"city":"NYC"}`,
			},
		},
	}

	chat := ResponsesToChatCompletions(resp, "gpt-4o")
	require.Len(t, chat.Choices, 1)
	assert.Equal(t, "tool_calls", chat.Choices[0].FinishReason)

	msg := chat.Choices[0].Message
	require.Len(t, msg.ToolCalls, 1)
	assert.Equal(t, "call_xyz", msg.ToolCalls[0].ID)
	assert.Equal(t, "function", msg.ToolCalls[0].Type)
	assert.Equal(t, "get_weather", msg.ToolCalls[0].Function.Name)
	assert.Equal(t, `{"city":"NYC"}`, msg.ToolCalls[0].Function.Arguments)
}

func TestResponsesToChatCompletions_CustomAndToolSearchCalls(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_custom_search",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type:   "custom_tool_call",
				CallID: "call_exec",
				Name:   "exec",
				Input:  "dir",
			},
			{
				Type:      "tool_search_call",
				CallID:    "call_search",
				Arguments: `{"query":"docs"}`,
			},
		},
	}

	chat := ResponsesToChatCompletions(resp, "gpt-5.2")
	require.Len(t, chat.Choices, 1)
	assert.Equal(t, "tool_calls", chat.Choices[0].FinishReason)

	msg := chat.Choices[0].Message
	require.Len(t, msg.ToolCalls, 2)
	assert.Equal(t, "call_exec", msg.ToolCalls[0].ID)
	assert.Equal(t, "exec", msg.ToolCalls[0].Function.Name)
	assert.JSONEq(t, `{"input":"dir"}`, msg.ToolCalls[0].Function.Arguments)
	assert.Equal(t, "call_search", msg.ToolCalls[1].ID)
	assert.Equal(t, toolSearchProxyName, msg.ToolCalls[1].Function.Name)
	assert.Equal(t, `{"query":"docs"}`, msg.ToolCalls[1].Function.Arguments)
}

func TestResponsesToChatCompletions_Reasoning(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_789",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type: "reasoning",
				Summary: []ResponsesSummary{
					{Type: "summary_text", Text: "I thought about it."},
				},
			},
			{
				Type: "message",
				Content: []ResponsesContentPart{
					{Type: "output_text", Text: "The answer is 42."},
				},
			},
		},
	}

	chat := ResponsesToChatCompletions(resp, "gpt-4o")
	require.Len(t, chat.Choices, 1)

	var content string
	require.NoError(t, json.Unmarshal(chat.Choices[0].Message.Content, &content))
	assert.Equal(t, "The answer is 42.", content)
	assert.Equal(t, "I thought about it.", chat.Choices[0].Message.ReasoningContent)
}

func TestChatCompletionsToResponses_ToolArrayContent(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Use the tool"`)},
			{
				Role: "assistant",
				ToolCalls: []ChatToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: ChatFunctionCall{
							Name:      "inspect_image",
							Arguments: `{}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "call_1",
				Content: json.RawMessage(
					`[{"type":"text","text":"image width: 100"},{"type":"image_url","image_url":{"url":"data:image/png;base64,ignored"}},{"type":"text","text":"; image height: 200"}]`,
				),
			},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(resp.Input, &items))
	require.Len(t, items, 3)
	assert.Equal(t, "function_call_output", items[2].Type)
	assert.Equal(t, "call_1", items[2].CallID)
	assert.Equal(t, "image width: 100; image height: 200", items[2].Output)
}

func TestResponsesToChatCompletions_Incomplete(t *testing.T) {
	resp := &ResponsesResponse{
		ID:                "resp_inc",
		Status:            "incomplete",
		IncompleteDetails: &ResponsesIncompleteDetails{Reason: "max_output_tokens"},
		Output: []ResponsesOutput{
			{
				Type: "message",
				Content: []ResponsesContentPart{
					{Type: "output_text", Text: "partial..."},
				},
			},
		},
	}

	chat := ResponsesToChatCompletions(resp, "gpt-4o")
	require.Len(t, chat.Choices, 1)
	assert.Equal(t, "length", chat.Choices[0].FinishReason)
}

func TestResponsesToChatCompletions_IncompleteContentFilterMapsToContentFilter(t *testing.T) {
	resp := &ResponsesResponse{
		ID:                "resp_inc_filter",
		Status:            "incomplete",
		IncompleteDetails: &ResponsesIncompleteDetails{Reason: "content_filter"},
		Output: []ResponsesOutput{
			{
				Type: "message",
				Content: []ResponsesContentPart{
					{Type: "output_text", Text: "blocked"},
				},
			},
		},
	}

	chat := ResponsesToChatCompletions(resp, "gpt-4o")
	require.Len(t, chat.Choices, 1)
	assert.Equal(t, "content_filter", chat.Choices[0].FinishReason)
}

func TestResponsesToChatCompletions_FailedMapsToStopFinishReason(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_failed",
		Status: "failed",
		Error:  &ResponsesError{Code: "server_error", Message: "upstream failed"},
	}

	chat := ResponsesToChatCompletions(resp, "gpt-4o")
	require.Len(t, chat.Choices, 1)
	assert.Equal(t, "stop", chat.Choices[0].FinishReason)
}

func TestResponsesToChatCompletions_FailedPolicyStillMapsToContentFilter(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_failed_policy",
		Status: "failed",
		Error:  &ResponsesError{Code: "content_policy_violation", Message: "Request not allowed by safety policy"},
	}

	chat := ResponsesToChatCompletions(resp, "gpt-4o")
	require.Len(t, chat.Choices, 1)
	assert.Equal(t, "content_filter", chat.Choices[0].FinishReason)
}

func TestChatCompletionsResponseToResponses_ContentFilterMapsToIncomplete(t *testing.T) {
	resp := &ChatCompletionsResponse{
		ID:    "chatcmpl_filter",
		Model: "gpt-4o",
		Choices: []ChatChoice{{
			Index: 0,
			Message: ChatMessage{
				Role:    "assistant",
				Content: json.RawMessage(`"blocked"`),
			},
			FinishReason: "content_filter",
		}},
	}

	out := ChatCompletionsResponseToResponses(resp, "gpt-4o")
	require.Equal(t, "incomplete", out.Status)
	require.NotNil(t, out.IncompleteDetails)
	assert.Equal(t, "content_filter", out.IncompleteDetails.Reason)
	require.Len(t, out.Output, 1)
	assert.Equal(t, "blocked", out.Output[0].Content[0].Text)
}

func TestResponsesToChatCompletionsRequest_MapsServerToolsToNativeTools(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-4o",
		Input: json.RawMessage(`"hello"`),
		Tools: []ResponsesTool{
			{Type: "web_search", Name: "search"},
			{Type: "web_fetch"},
			{Type: "google_search"},
		},
		ToolChoice: json.RawMessage(`{"type":"google_search"}`),
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Tools, 2)
	require.Equal(t, "web_search", out.Tools[0].Type)
	require.Nil(t, out.Tools[0].Function)
	require.Equal(t, "web_fetch", out.Tools[1].Type)
	require.Nil(t, out.Tools[1].Function)
	require.JSONEq(t, `{"type":"web_search"}`, string(out.ToolChoice))
}

func TestResponsesToChatCompletionsRequest_FlattensNamespaceTools(t *testing.T) {
	var req ResponsesRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "gpt-4o",
		"input": "hello",
		"tools": [
			{
				"type": "namespace",
				"name": "browser.tools",
				"description": "Browser tools",
				"tools": [
					{
						"type": "function",
						"name": "open.url",
						"description": "Open a URL",
						"parameters": {
							"type": "object",
							"properties": {
								"url": {"type": "string"}
							},
							"required": ["url"]
						}
					},
					{
						"type": "function",
						"name": "find",
						"description": "Find text",
						"parameters": {
							"type": "object",
							"properties": {
								"pattern": {"type": "string"}
							},
							"required": ["pattern"]
						}
					}
				]
			}
		],
		"tool_choice": {"type": "function", "name": "browser.tools.open.url"}
	}`), &req))

	out, err := ResponsesToChatCompletionsRequest(&req)
	require.NoError(t, err)
	require.Len(t, out.Tools, 2)
	require.Equal(t, "function", out.Tools[0].Type)
	require.NotNil(t, out.Tools[0].Function)
	require.Equal(t, "browser_tools_open_url", out.Tools[0].Function.Name)
	require.Equal(t, "Open a URL", out.Tools[0].Function.Description)
	require.JSONEq(t, `{"type":"object","properties":{"url":{"type":"string"}},"required":["url"]}`, string(out.Tools[0].Function.Parameters))
	require.Equal(t, "function", out.Tools[1].Type)
	require.NotNil(t, out.Tools[1].Function)
	require.Equal(t, "browser_tools_find", out.Tools[1].Function.Name)
	require.JSONEq(t, `{"type":"function","function":{"name":"browser_tools_open_url"}}`, string(out.ToolChoice))
}

func TestResponsesToChatCompletionsRequest_SkipsUnsupportedServerTools(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-4o",
		Input: json.RawMessage(`"hello"`),
		Tools: []ResponsesTool{
			{Type: "image_generation"},
		},
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Empty(t, out.Tools)
}

func TestResponsesToChatCompletionsRequest_DropsUnsupportedServerToolChoice(t *testing.T) {
	req := &ResponsesRequest{
		Model:      "gpt-4o",
		Input:      json.RawMessage(`"hello"`),
		ToolChoice: json.RawMessage(`{"type":"image_generation"}`),
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Empty(t, out.ToolChoice)
}

func TestResponsesToChatCompletionsRequest_MapsStringServerToolChoice(t *testing.T) {
	req := &ResponsesRequest{
		Model:      "gpt-4o",
		Input:      json.RawMessage(`"hello"`),
		Tools:      []ResponsesTool{{Type: "google_search"}},
		ToolChoice: json.RawMessage(`"google_search"`),
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"web_search"}`, string(out.ToolChoice))
}

func TestResponsesToChatCompletionsRequest_FallsBackUnsupportedStringToolChoice(t *testing.T) {
	req := &ResponsesRequest{
		Model:      "gpt-4o",
		Input:      json.RawMessage(`"hello"`),
		Tools:      []ResponsesTool{{Type: "function", Name: "wait"}},
		ToolChoice: json.RawMessage(`"image_generation"`),
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.JSONEq(t, `"auto"`, string(out.ToolChoice))
}

func TestResponsesToChatCompletions_CachedTokens(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_cache",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type:    "message",
				Content: []ResponsesContentPart{{Type: "output_text", Text: "cached"}},
			},
		},
		Usage: &ResponsesUsage{
			InputTokens:  100,
			OutputTokens: 10,
			TotalTokens:  110,
			InputTokensDetails: &ResponsesInputTokensDetails{
				CachedTokens: 80,
			},
		},
	}

	chat := ResponsesToChatCompletions(resp, "gpt-4o")
	require.NotNil(t, chat.Usage)
	require.NotNil(t, chat.Usage.PromptTokensDetails)
	assert.Equal(t, 80, chat.Usage.PromptTokensDetails.CachedTokens)
}

func TestResponsesToChatCompletions_ReasoningTokens(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_reasoning",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type:    "message",
				Content: []ResponsesContentPart{{Type: "output_text", Text: "ping"}},
			},
		},
		Usage: &ResponsesUsage{
			InputTokens:  24,
			OutputTokens: 33,
			TotalTokens:  57,
			OutputTokensDetails: &ResponsesOutputTokensDetails{
				ReasoningTokens: 32,
			},
		},
	}

	chat := ResponsesToChatCompletions(resp, "gpt-5.5")
	require.NotNil(t, chat.Usage)
	assert.Equal(t, 33, chat.Usage.CompletionTokens)
	require.NotNil(t, chat.Usage.CompletionTokensDetails)
	assert.Equal(t, 32, chat.Usage.CompletionTokensDetails.ReasoningTokens)
}

func TestResponsesToChatCompletions_AllTokenDetailsPassThrough(t *testing.T) {
	// Covers the full OpenAI CompletionUsage detail field set so future audio
	// and prediction-outputs responses propagate without further changes.
	resp := &ResponsesResponse{
		ID:     "resp_full_details",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type:    "message",
				Content: []ResponsesContentPart{{Type: "output_text", Text: "x"}},
			},
		},
		Usage: &ResponsesUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
			InputTokensDetails: &ResponsesInputTokensDetails{
				CachedTokens: 60,
				AudioTokens:  4,
			},
			OutputTokensDetails: &ResponsesOutputTokensDetails{
				ReasoningTokens:          30,
				AudioTokens:              2,
				AcceptedPredictionTokens: 10,
				RejectedPredictionTokens: 3,
			},
		},
	}

	chat := ResponsesToChatCompletions(resp, "gpt-5.5")
	require.NotNil(t, chat.Usage)
	require.NotNil(t, chat.Usage.PromptTokensDetails)
	assert.Equal(t, 60, chat.Usage.PromptTokensDetails.CachedTokens)
	assert.Equal(t, 4, chat.Usage.PromptTokensDetails.AudioTokens)

	require.NotNil(t, chat.Usage.CompletionTokensDetails)
	assert.Equal(t, 30, chat.Usage.CompletionTokensDetails.ReasoningTokens)
	assert.Equal(t, 2, chat.Usage.CompletionTokensDetails.AudioTokens)
	assert.Equal(t, 10, chat.Usage.CompletionTokensDetails.AcceptedPredictionTokens)
	assert.Equal(t, 3, chat.Usage.CompletionTokensDetails.RejectedPredictionTokens)

	raw, err := json.Marshal(chat.Usage)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"prompt_tokens_details"`)
	assert.Contains(t, string(raw), `"completion_tokens_details"`)
	assert.Contains(t, string(raw), `"reasoning_tokens":30`)
	assert.Contains(t, string(raw), `"accepted_prediction_tokens":10`)
}

func TestResponsesToChatCompletions_NoReasoningTokensWhenZero(t *testing.T) {
	// Non-reasoning models do not return reasoning_tokens. The mapping must
	// omit completion_tokens_details entirely rather than emitting a zero-valued
	// field, so non-reasoning responses stay clean.
	resp := &ResponsesResponse{
		ID:     "resp_no_reasoning",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type:    "message",
				Content: []ResponsesContentPart{{Type: "output_text", Text: "hi"}},
			},
		},
		Usage: &ResponsesUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
			OutputTokensDetails: &ResponsesOutputTokensDetails{
				ReasoningTokens: 0,
			},
		},
	}

	chat := ResponsesToChatCompletions(resp, "gpt-4o")
	require.NotNil(t, chat.Usage)
	assert.Nil(t, chat.Usage.CompletionTokensDetails)

	raw, err := json.Marshal(chat.Usage)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "completion_tokens_details")
	assert.NotContains(t, string(raw), "reasoning_tokens")
}

func TestResponsesToChatCompletions_WebSearch(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_ws",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type:   "web_search_call",
				Action: &WebSearchAction{Type: "search", Query: "test"},
			},
			{
				Type:    "message",
				Content: []ResponsesContentPart{{Type: "output_text", Text: "search results"}},
			},
		},
	}

	chat := ResponsesToChatCompletions(resp, "gpt-4o")
	require.Len(t, chat.Choices, 1)
	assert.Equal(t, "stop", chat.Choices[0].FinishReason)

	var content string
	require.NoError(t, json.Unmarshal(chat.Choices[0].Message.Content, &content))
	assert.Equal(t, "search results", content)
}

// ---------------------------------------------------------------------------
// Streaming: ResponsesEventToChatChunks tests
// ---------------------------------------------------------------------------

func TestResponsesEventToChatChunks_TextDelta(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"

	// response.created → role chunk
	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.created",
		Response: &ResponsesResponse{
			ID: "resp_stream",
		},
	}, state)
	require.Len(t, chunks, 1)
	assert.Equal(t, "assistant", chunks[0].Choices[0].Delta.Role)
	assert.True(t, state.SentRole)

	// response.output_text.delta → content chunk
	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:  "response.output_text.delta",
		Delta: "Hello",
	}, state)
	require.Len(t, chunks, 1)
	require.NotNil(t, chunks[0].Choices[0].Delta.Content)
	assert.Equal(t, "Hello", *chunks[0].Choices[0].Delta.Content)
}

func TestResponsesEventToChatChunks_ToolCallDelta(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"
	state.SentRole = true

	// response.output_item.added (function_call) — output_index=1 (e.g. after a message item at 0)
	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 1,
		Item: &ResponsesOutput{
			Type:   "function_call",
			CallID: "call_1",
			Name:   "get_weather",
		},
	}, state)
	require.Len(t, chunks, 1)
	require.Len(t, chunks[0].Choices[0].Delta.ToolCalls, 1)
	tc := chunks[0].Choices[0].Delta.ToolCalls[0]
	assert.Equal(t, "call_1", tc.ID)
	assert.Equal(t, "get_weather", tc.Function.Name)
	require.NotNil(t, tc.Index)
	assert.Equal(t, 0, *tc.Index)

	// response.function_call_arguments.delta — uses output_index (NOT call_id) to find tool
	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.delta",
		OutputIndex: 1, // matches the output_index from output_item.added above
		Delta:       `{"city":`,
	}, state)
	require.Len(t, chunks, 1)
	tc = chunks[0].Choices[0].Delta.ToolCalls[0]
	require.NotNil(t, tc.Index)
	assert.Equal(t, 0, *tc.Index, "argument delta must use same index as the tool call")
	assert.Equal(t, `{"city":`, tc.Function.Arguments)

	// Add a second function call at output_index=2
	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 2,
		Item: &ResponsesOutput{
			Type:   "function_call",
			CallID: "call_2",
			Name:   "get_time",
		},
	}, state)
	require.Len(t, chunks, 1)
	tc = chunks[0].Choices[0].Delta.ToolCalls[0]
	require.NotNil(t, tc.Index)
	assert.Equal(t, 1, *tc.Index, "second tool call should get index 1")

	// Argument delta for second tool call
	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.delta",
		OutputIndex: 2,
		Delta:       `{"tz":"UTC"}`,
	}, state)
	require.Len(t, chunks, 1)
	tc = chunks[0].Choices[0].Delta.ToolCalls[0]
	require.NotNil(t, tc.Index)
	assert.Equal(t, 1, *tc.Index, "second tool arg delta must use index 1")

	// Argument delta for first tool call (interleaved)
	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.delta",
		OutputIndex: 1,
		Delta:       `"Tokyo"}`,
	}, state)
	require.Len(t, chunks, 1)
	tc = chunks[0].Choices[0].Delta.ToolCalls[0]
	require.NotNil(t, tc.Index)
	assert.Equal(t, 0, *tc.Index, "first tool arg delta must still use index 0")
}

func TestResponsesEventToChatChunks_FunctionCallArgumentsDoneOnly(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"
	state.SentRole = true

	_ = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 1,
		Item: &ResponsesOutput{
			Type:   "function_call",
			CallID: "call_1",
			Name:   "get_weather",
		},
	}, state)

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.done",
		OutputIndex: 1,
		Arguments:   `{"city":"NYC"}`,
	}, state)
	require.Len(t, chunks, 1)
	require.Len(t, chunks[0].Choices[0].Delta.ToolCalls, 1)
	tc := chunks[0].Choices[0].Delta.ToolCalls[0]
	require.NotNil(t, tc.Index)
	assert.Equal(t, 0, *tc.Index)
	assert.Equal(t, `{"city":"NYC"}`, tc.Function.Arguments)
}

func TestResponsesEventToChatChunks_FunctionCallArgumentsDoneDoesNotDuplicateDelta(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"
	state.SentRole = true

	_ = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 1,
		Item: &ResponsesOutput{
			Type:   "function_call",
			CallID: "call_1",
			Name:   "get_weather",
		},
	}, state)

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.delta",
		OutputIndex: 1,
		Delta:       `{"city":"NYC"}`,
	}, state)
	require.Len(t, chunks, 1)

	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.done",
		OutputIndex: 1,
		Arguments:   `{"city":"NYC"}`,
	}, state)
	assert.Empty(t, chunks)
}

func TestChatCompletionsChunkToResponsesEvents_DoesNotDuplicateFirstToolArgumentDelta(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("gpt-4o")
	firstChunk := &ChatCompletionsChunk{
		ID:    "chatcmpl_tool_stream",
		Model: "gpt-4o",
		Choices: []ChatChunkChoice{{
			Delta: ChatDelta{
				ToolCalls: []ChatToolCall{{
					Index: intPtr(0),
					ID:    "call_1",
					Function: ChatFunctionCall{
						Name:      "get_weather",
						Arguments: `{"city":"`,
					},
				}},
			},
		}},
	}

	events := ChatCompletionsChunkToResponsesEvents(firstChunk, state)
	require.Len(t, events, 3)
	require.Equal(t, "response.function_call_arguments.delta", events[2].Type)
	require.Equal(t, `{"city":"`, events[2].Delta)
	require.Contains(t, state.ToolCalls, 0)
	require.Equal(t, `{"city":"`, state.ToolCalls[0].Function.Arguments)
}

func TestChatCompletionsChunkToResponsesEvents_FinalizesFunctionCallsWithStableItemIDs(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("gpt-4o")
	firstChunk := &ChatCompletionsChunk{
		ID:    "chatcmpl_tool_stream",
		Model: "gpt-4o",
		Choices: []ChatChunkChoice{{
			Delta: ChatDelta{
				ToolCalls: []ChatToolCall{{
					Index: intPtr(0),
					ID:    "call_1",
					Function: ChatFunctionCall{
						Name:      "get_weather",
						Arguments: `{"city":"`,
					},
				}},
			},
		}},
	}

	events := ChatCompletionsChunkToResponsesEvents(firstChunk, state)
	require.Len(t, events, 3)
	require.Equal(t, "response.output_item.added", events[1].Type)
	itemID := events[1].Item.ID
	require.NotEmpty(t, itemID)
	require.Equal(t, "call_1", events[1].Item.CallID)
	require.Equal(t, "response.function_call_arguments.delta", events[2].Type)
	require.Equal(t, `{"city":"`, events[2].Delta)

	finishReason := "tool_calls"
	secondChunk := &ChatCompletionsChunk{
		ID:    "chatcmpl_tool_stream",
		Model: "gpt-4o",
		Choices: []ChatChunkChoice{{
			Delta: ChatDelta{
				ToolCalls: []ChatToolCall{{
					Index: intPtr(0),
					Function: ChatFunctionCall{
						Arguments: `NYC"}`,
					},
				}},
			},
			FinishReason: &finishReason,
		}},
	}

	events = ChatCompletionsChunkToResponsesEvents(secondChunk, state)
	require.Len(t, events, 1)
	require.Equal(t, "response.function_call_arguments.delta", events[0].Type)
	require.Equal(t, `NYC"}`, events[0].Delta)

	finalEvents := FinalizeChatCompletionsResponsesStream(state)
	require.Len(t, finalEvents, 3)
	require.Equal(t, "response.function_call_arguments.done", finalEvents[0].Type)
	require.Equal(t, `{"city":"NYC"}`, finalEvents[0].Arguments)
	require.Equal(t, "response.output_item.done", finalEvents[1].Type)
	require.NotNil(t, finalEvents[1].Item)
	require.Equal(t, itemID, finalEvents[1].Item.ID)
	require.Equal(t, "call_1", finalEvents[1].Item.CallID)
	require.Equal(t, "response.completed", finalEvents[2].Type)
	require.NotNil(t, finalEvents[2].Response)
	require.Len(t, finalEvents[2].Response.Output, 1)
	require.Equal(t, "function_call", finalEvents[2].Response.Output[0].Type)
	assert.Equal(t, itemID, finalEvents[2].Response.Output[0].ID)
	assert.Equal(t, `{"city":"NYC"}`, finalEvents[2].Response.Output[0].Arguments)
}

func TestFinalizeChatCompletionsResponsesStream_ContentFilterMapsToIncomplete(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("gpt-4o")
	finishReason := "content_filter"

	events := ChatCompletionsChunkToResponsesEvents(&ChatCompletionsChunk{
		ID:    "chatcmpl_content_filter",
		Model: "gpt-4o",
		Choices: []ChatChunkChoice{{
			FinishReason: &finishReason,
		}},
	}, state)
	require.Len(t, events, 1)
	require.Equal(t, "response.created", events[0].Type)

	finalEvents := FinalizeChatCompletionsResponsesStream(state)
	require.Len(t, finalEvents, 1)
	require.Equal(t, "response.completed", finalEvents[0].Type)
	require.NotNil(t, finalEvents[0].Response)
	assert.Equal(t, "incomplete", finalEvents[0].Response.Status)
	require.NotNil(t, finalEvents[0].Response.IncompleteDetails)
	assert.Equal(t, "content_filter", finalEvents[0].Response.IncompleteDetails.Reason)
}

func TestResponsesEventToChatChunks_Completed(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"
	state.IncludeUsage = true

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
			Usage: &ResponsesUsage{
				InputTokens:  50,
				OutputTokens: 20,
				TotalTokens:  70,
				InputTokensDetails: &ResponsesInputTokensDetails{
					CachedTokens: 30,
				},
			},
		},
	}, state)
	// finish chunk + usage chunk
	require.Len(t, chunks, 2)

	// First chunk: finish_reason
	require.NotNil(t, chunks[0].Choices[0].FinishReason)
	assert.Equal(t, "stop", *chunks[0].Choices[0].FinishReason)

	// Second chunk: usage
	require.NotNil(t, chunks[1].Usage)
	assert.Equal(t, 50, chunks[1].Usage.PromptTokens)
	assert.Equal(t, 20, chunks[1].Usage.CompletionTokens)
	assert.Equal(t, 70, chunks[1].Usage.TotalTokens)
	require.NotNil(t, chunks[1].Usage.PromptTokensDetails)
	assert.Equal(t, 30, chunks[1].Usage.PromptTokensDetails.CachedTokens)
}

func TestResponsesEventToChatChunks_CompletedWithRefusal(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
			Output: []ResponsesOutput{{
				Type: "message",
				Role: "assistant",
				Content: []ResponsesContentPart{{
					Type:    "refusal",
					Refusal: "I can't help with that.",
				}},
			}},
		},
	}, state)

	require.Len(t, chunks, 2)
	require.NotNil(t, chunks[0].Choices[0].Delta.Refusal)
	assert.Equal(t, "I can't help with that.", *chunks[0].Choices[0].Delta.Refusal)
	require.NotNil(t, chunks[1].Choices[0].FinishReason)
	assert.Equal(t, "stop", *chunks[1].Choices[0].FinishReason)
}

func TestResponsesEventToChatChunks_CompletedWithReasoningTokens(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-5.5"
	state.IncludeUsage = true

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
			Usage: &ResponsesUsage{
				InputTokens:  24,
				OutputTokens: 33,
				TotalTokens:  57,
				OutputTokensDetails: &ResponsesOutputTokensDetails{
					ReasoningTokens: 32,
				},
			},
		},
	}, state)
	require.Len(t, chunks, 2)

	require.NotNil(t, chunks[1].Usage)
	require.NotNil(t, chunks[1].Usage.CompletionTokensDetails)
	assert.Equal(t, 32, chunks[1].Usage.CompletionTokensDetails.ReasoningTokens)
}

func TestResponsesEventToChatChunks_ResponseDone(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"
	state.IncludeUsage = true

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.done",
		Response: &ResponsesResponse{
			Status: "completed",
			Usage:  &ResponsesUsage{InputTokens: 13, OutputTokens: 7},
		},
	}, state)
	require.Len(t, chunks, 2)
	require.NotNil(t, chunks[0].Choices[0].FinishReason)
	assert.Equal(t, "stop", *chunks[0].Choices[0].FinishReason)
	require.NotNil(t, chunks[1].Usage)
	assert.Equal(t, 13, chunks[1].Usage.PromptTokens)
	assert.Equal(t, 7, chunks[1].Usage.CompletionTokens)
	assert.Nil(t, FinalizeResponsesChatStream(state))
}

func TestResponsesEventToChatChunks_TopLevelTerminalUsage(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"
	state.IncludeUsage = true

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
		},
		Usage: &ResponsesUsage{
			InputTokens:  21,
			OutputTokens: 9,
			InputTokensDetails: &ResponsesInputTokensDetails{
				CachedTokens: 4,
			},
		},
	}, state)

	require.Len(t, chunks, 2)
	require.NotNil(t, chunks[1].Usage)
	assert.Equal(t, 21, chunks[1].Usage.PromptTokens)
	assert.Equal(t, 9, chunks[1].Usage.CompletionTokens)
	require.NotNil(t, chunks[1].Usage.PromptTokensDetails)
	assert.Equal(t, 4, chunks[1].Usage.PromptTokensDetails.CachedTokens)
}

func TestResponsesEventToChatChunks_ResponseDoneIncomplete(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"
	state.IncludeUsage = true

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.done",
		Response: &ResponsesResponse{
			Status:            "incomplete",
			IncompleteDetails: &ResponsesIncompleteDetails{Reason: "max_output_tokens"},
			Usage:             &ResponsesUsage{InputTokens: 13, OutputTokens: 7},
		},
	}, state)
	require.Len(t, chunks, 2)
	require.NotNil(t, chunks[0].Choices[0].FinishReason)
	assert.Equal(t, "length", *chunks[0].Choices[0].FinishReason)
	require.NotNil(t, chunks[1].Usage)
	assert.Equal(t, 13, chunks[1].Usage.PromptTokens)
	assert.Equal(t, 7, chunks[1].Usage.CompletionTokens)
	assert.Nil(t, FinalizeResponsesChatStream(state))
}

func TestResponsesEventToChatChunks_ResponseDoneIncompleteContentFilter(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.done",
		Response: &ResponsesResponse{
			Status:            "incomplete",
			IncompleteDetails: &ResponsesIncompleteDetails{Reason: "content_filter"},
		},
	}, state)
	require.Len(t, chunks, 1)
	require.NotNil(t, chunks[0].Choices[0].FinishReason)
	assert.Equal(t, "content_filter", *chunks[0].Choices[0].FinishReason)
}

func TestResponsesEventToChatChunks_CompletedWithToolCalls(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"
	state.SawToolCall = true

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
		},
	}, state)
	require.Len(t, chunks, 1)
	require.NotNil(t, chunks[0].Choices[0].FinishReason)
	assert.Equal(t, "tool_calls", *chunks[0].Choices[0].FinishReason)
}

func TestResponsesEventToChatChunks_FailedUsesStopFinishReason(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.failed",
		Response: &ResponsesResponse{
			Status: "failed",
			Error:  &ResponsesError{Code: "server_error", Message: "upstream failed"},
		},
	}, state)
	require.Len(t, chunks, 1)
	require.NotNil(t, chunks[0].Choices[0].FinishReason)
	assert.Equal(t, "stop", *chunks[0].Choices[0].FinishReason)
}

func TestResponsesEventToChatChunks_FailedPolicyUsesContentFilterFinishReason(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.failed",
		Response: &ResponsesResponse{
			Status: "failed",
			Error:  &ResponsesError{Code: "content_policy_violation", Message: "Request blocked by policy"},
		},
	}, state)
	require.Len(t, chunks, 1)
	require.NotNil(t, chunks[0].Choices[0].FinishReason)
	assert.Equal(t, "content_filter", *chunks[0].Choices[0].FinishReason)
}

func TestResponsesEventToChatChunks_ReasoningDelta(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"
	state.SentRole = true

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:  "response.reasoning_summary_text.delta",
		Delta: "Thinking...",
	}, state)
	require.Len(t, chunks, 1)
	require.NotNil(t, chunks[0].Choices[0].Delta.ReasoningContent)
	assert.Equal(t, "Thinking...", *chunks[0].Choices[0].Delta.ReasoningContent)

	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.reasoning_summary_text.done",
	}, state)
	require.Len(t, chunks, 0)
}

func TestResponsesEventToChatChunks_ReasoningThenTextAutoCloseTag(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"
	state.SentRole = true

	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:  "response.reasoning_summary_text.delta",
		Delta: "plan",
	}, state)
	require.Len(t, chunks, 1)
	require.NotNil(t, chunks[0].Choices[0].Delta.ReasoningContent)
	assert.Equal(t, "plan", *chunks[0].Choices[0].Delta.ReasoningContent)

	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:  "response.output_text.delta",
		Delta: "answer",
	}, state)
	require.Len(t, chunks, 1)
	require.NotNil(t, chunks[0].Choices[0].Delta.Content)
	assert.Equal(t, "answer", *chunks[0].Choices[0].Delta.Content)
}

func TestFinalizeResponsesChatStream(t *testing.T) {
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"
	state.IncludeUsage = true
	state.Usage = &ChatUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	chunks := FinalizeResponsesChatStream(state)
	require.Len(t, chunks, 2)

	// Finish chunk
	require.NotNil(t, chunks[0].Choices[0].FinishReason)
	assert.Equal(t, "stop", *chunks[0].Choices[0].FinishReason)

	// Usage chunk
	require.NotNil(t, chunks[1].Usage)
	assert.Equal(t, 100, chunks[1].Usage.PromptTokens)

	// Idempotent: second call returns nil
	assert.Nil(t, FinalizeResponsesChatStream(state))
}

func TestFinalizeResponsesChatStream_AfterCompleted(t *testing.T) {
	// If response.completed already emitted the finish chunk, FinalizeResponsesChatStream
	// must be a no-op (prevents double finish_reason being sent to the client).
	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"
	state.IncludeUsage = true

	// Simulate response.completed
	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
			Usage: &ResponsesUsage{
				InputTokens:  10,
				OutputTokens: 5,
				TotalTokens:  15,
			},
		},
	}, state)
	require.NotEmpty(t, chunks) // finish + usage chunks

	// Now FinalizeResponsesChatStream should return nil — already finalized.
	assert.Nil(t, FinalizeResponsesChatStream(state))
}

func TestChatChunkToSSE(t *testing.T) {
	chunk := ChatCompletionsChunk{
		ID:      "chatcmpl-test",
		Object:  "chat.completion.chunk",
		Created: 1700000000,
		Model:   "gpt-4o",
		Choices: []ChatChunkChoice{
			{
				Index:        0,
				Delta:        ChatDelta{Role: "assistant"},
				FinishReason: nil,
			},
		},
	}

	sse, err := ChatChunkToSSE(chunk)
	require.NoError(t, err)
	assert.Contains(t, sse, "data: ")
	assert.Contains(t, sse, "chatcmpl-test")
	assert.Contains(t, sse, "assistant")
	assert.True(t, len(sse) > 10)
}

// ---------------------------------------------------------------------------
// Stream round-trip test
// ---------------------------------------------------------------------------

func TestChatCompletionsStreamRoundTrip(t *testing.T) {
	// Simulate: client sends chat completions request, upstream returns Responses SSE events.
	// Verify that the streaming state machine produces correct chat completions chunks.

	state := NewResponsesEventToChatState()
	state.Model = "gpt-4o"
	state.IncludeUsage = true

	var allChunks []ChatCompletionsChunk

	// 1. response.created
	chunks := ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_rt"},
	}, state)
	allChunks = append(allChunks, chunks...)

	// 2. text deltas
	for _, text := range []string{"Hello", ", ", "world", "!"} {
		chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
			Type:  "response.output_text.delta",
			Delta: text,
		}, state)
		allChunks = append(allChunks, chunks...)
	}

	// 3. response.completed
	chunks = ResponsesEventToChatChunks(&ResponsesStreamEvent{
		Type: "response.completed",
		Response: &ResponsesResponse{
			Status: "completed",
			Usage: &ResponsesUsage{
				InputTokens:  10,
				OutputTokens: 4,
				TotalTokens:  14,
			},
		},
	}, state)
	allChunks = append(allChunks, chunks...)

	// Verify: role chunk + 4 text chunks + finish chunk + usage chunk = 7
	require.Len(t, allChunks, 7)

	// First chunk has role
	assert.Equal(t, "assistant", allChunks[0].Choices[0].Delta.Role)

	// Text chunks
	var fullText string
	for i := 1; i <= 4; i++ {
		require.NotNil(t, allChunks[i].Choices[0].Delta.Content)
		fullText += *allChunks[i].Choices[0].Delta.Content
	}
	assert.Equal(t, "Hello, world!", fullText)

	// Finish chunk
	require.NotNil(t, allChunks[5].Choices[0].FinishReason)
	assert.Equal(t, "stop", *allChunks[5].Choices[0].FinishReason)

	// Usage chunk
	require.NotNil(t, allChunks[6].Usage)
	assert.Equal(t, 10, allChunks[6].Usage.PromptTokens)
	assert.Equal(t, 4, allChunks[6].Usage.CompletionTokens)

	// All chunks share the same ID
	for _, c := range allChunks {
		assert.Equal(t, "resp_rt", c.ID)
	}
}

// ---------------------------------------------------------------------------
// BufferedResponseAccumulator tests
// ---------------------------------------------------------------------------

func TestBufferedResponseAccumulator_TextOnly(t *testing.T) {
	acc := NewBufferedResponseAccumulator()

	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.output_text.delta", Delta: "Hello"})
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.output_text.delta", Delta: ", world!"})

	assert.True(t, acc.HasContent())

	output := acc.BuildOutput()
	require.Len(t, output, 1)
	assert.Equal(t, "message", output[0].Type)
	assert.Equal(t, "assistant", output[0].Role)
	require.Len(t, output[0].Content, 1)
	assert.Equal(t, "output_text", output[0].Content[0].Type)
	assert.Equal(t, "Hello, world!", output[0].Content[0].Text)
}

func TestBufferedResponseAccumulator_ToolCalls(t *testing.T) {
	acc := NewBufferedResponseAccumulator()

	// Add function call at output_index=1
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 1,
		Item: &ResponsesOutput{
			Type:   "function_call",
			CallID: "call_abc",
			Name:   "get_weather",
		},
	})
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.delta",
		OutputIndex: 1,
		Delta:       `{"city":`,
	})
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.delta",
		OutputIndex: 1,
		Delta:       `"NYC"}`,
	})

	assert.True(t, acc.HasContent())

	output := acc.BuildOutput()
	require.Len(t, output, 1)
	assert.Equal(t, "function_call", output[0].Type)
	assert.Equal(t, "call_abc", output[0].CallID)
	assert.Equal(t, "get_weather", output[0].Name)
	assert.Equal(t, `{"city":"NYC"}`, output[0].Arguments)
}

func TestBufferedResponseAccumulator_ToolCallArgumentsDoneOnly(t *testing.T) {
	acc := NewBufferedResponseAccumulator()

	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 1,
		Item: &ResponsesOutput{
			Type:   "function_call",
			CallID: "call_abc",
			Name:   "get_weather",
		},
	})
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.done",
		OutputIndex: 1,
		Arguments:   `{"city":"NYC"}`,
	})

	output := acc.BuildOutput()
	require.Len(t, output, 1)
	assert.Equal(t, `{"city":"NYC"}`, output[0].Arguments)
}

func TestBufferedResponseAccumulator_ToolCallArgumentsDoneDoesNotDuplicateDelta(t *testing.T) {
	acc := NewBufferedResponseAccumulator()

	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 1,
		Item: &ResponsesOutput{
			Type:   "function_call",
			CallID: "call_abc",
			Name:   "get_weather",
		},
	})
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.delta",
		OutputIndex: 1,
		Delta:       `{"city":"NYC"}`,
	})
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.done",
		OutputIndex: 1,
		Arguments:   `{"city":"NYC"}`,
	})

	output := acc.BuildOutput()
	require.Len(t, output, 1)
	assert.Equal(t, `{"city":"NYC"}`, output[0].Arguments)
}

func TestBufferedResponseAccumulator_SupplementFunctionCallArgumentsDoneOnly(t *testing.T) {
	acc := NewBufferedResponseAccumulator()
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 1,
		Item: &ResponsesOutput{
			Type:   "function_call",
			CallID: "call_abc",
			Name:   "get_weather",
		},
	})
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.done",
		OutputIndex: 1,
		Arguments:   `{"city":"NYC"}`,
	})

	resp := &ResponsesResponse{
		ID:     "resp_tools",
		Status: "completed",
		Output: []ResponsesOutput{{
			Type:   "function_call",
			CallID: "call_abc",
			Name:   "get_weather",
		}},
	}
	acc.SupplementResponseOutput(resp)

	require.Len(t, resp.Output, 1)
	assert.Equal(t, `{"city":"NYC"}`, resp.Output[0].Arguments)
}

func TestBufferedResponseAccumulator_Reasoning(t *testing.T) {
	acc := NewBufferedResponseAccumulator()

	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.reasoning_summary_text.delta", Delta: "Step 1: "})
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.reasoning_summary_text.delta", Delta: "think about it"})

	assert.True(t, acc.HasContent())

	output := acc.BuildOutput()
	require.Len(t, output, 1)
	assert.Equal(t, "reasoning", output[0].Type)
	require.Len(t, output[0].Summary, 1)
	assert.Equal(t, "summary_text", output[0].Summary[0].Type)
	assert.Equal(t, "Step 1: think about it", output[0].Summary[0].Text)
}

func TestBufferedResponseAccumulator_Mixed(t *testing.T) {
	acc := NewBufferedResponseAccumulator()

	// Reasoning first
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.reasoning_summary_text.delta", Delta: "I thought about it."})

	// Then text
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.output_text.delta", Delta: "The answer is 42."})

	// Then a tool call
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 2,
		Item: &ResponsesOutput{
			Type:   "function_call",
			CallID: "call_1",
			Name:   "verify",
		},
	})
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.delta",
		OutputIndex: 2,
		Delta:       `{}`,
	})

	assert.True(t, acc.HasContent())

	output := acc.BuildOutput()
	// Order: reasoning → message → function_calls
	require.Len(t, output, 3)
	assert.Equal(t, "reasoning", output[0].Type)
	assert.Equal(t, "message", output[1].Type)
	assert.Equal(t, "function_call", output[2].Type)
	assert.Equal(t, "The answer is 42.", output[1].Content[0].Text)
	assert.Equal(t, "verify", output[2].Name)
}

func TestBufferedResponseAccumulator_SupplementEmptyOutput(t *testing.T) {
	acc := NewBufferedResponseAccumulator()
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.output_text.delta", Delta: "Hello"})

	resp := &ResponsesResponse{
		ID:     "resp_1",
		Status: "completed",
		Output: nil, // empty output
		Usage:  &ResponsesUsage{InputTokens: 10, OutputTokens: 5},
	}

	acc.SupplementResponseOutput(resp)

	require.Len(t, resp.Output, 1)
	assert.Equal(t, "message", resp.Output[0].Type)
	assert.Equal(t, "Hello", resp.Output[0].Content[0].Text)
	// Usage should be untouched
	assert.Equal(t, 10, resp.Usage.InputTokens)
}

func TestBufferedResponseAccumulator_NoSupplementWhenOutputExists(t *testing.T) {
	acc := NewBufferedResponseAccumulator()
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.output_text.delta", Delta: "from deltas"})

	resp := &ResponsesResponse{
		ID:     "resp_2",
		Status: "completed",
		Output: []ResponsesOutput{
			{
				Type: "message",
				Content: []ResponsesContentPart{
					{Type: "output_text", Text: "from terminal event"},
				},
			},
		},
	}

	acc.SupplementResponseOutput(resp)

	// Output should NOT be overwritten
	require.Len(t, resp.Output, 1)
	assert.Equal(t, "from terminal event", resp.Output[0].Content[0].Text)
}

func TestBufferedResponseAccumulator_EmptyDeltas(t *testing.T) {
	acc := NewBufferedResponseAccumulator()

	// Process events with empty delta — should not accumulate
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.output_text.delta", Delta: ""})
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.created"})

	assert.False(t, acc.HasContent())

	resp := &ResponsesResponse{ID: "resp_3", Status: "completed"}
	acc.SupplementResponseOutput(resp)
	assert.Nil(t, resp.Output)
}

func TestBufferedResponseAccumulator_IgnoresNonFunctionCallItems(t *testing.T) {
	acc := NewBufferedResponseAccumulator()

	// output_item.added with type "message" should be ignored
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item:        &ResponsesOutput{Type: "message"},
	})

	assert.False(t, acc.HasContent())
}
