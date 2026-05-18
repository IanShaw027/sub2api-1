package kiro

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConvertAnthropicRequestWithModel_DoesNotInjectSyntheticAssistantOrPolicy(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":"Follow repository constraints.",
		"messages":[{"role":"user","content":"hello"}]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result.Body, &payload); err != nil {
		t.Fatalf("unmarshal converted payload: %v", err)
	}

	state, ok := payload["conversationState"].(map[string]any)
	if !ok {
		t.Fatalf("conversationState has type %T, want map[string]any", payload["conversationState"])
	}
	history, ok := state["history"].([]any)
	if !ok {
		t.Fatalf("history has type %T, want []any", state["history"])
	}
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}

	first, ok := history[0].(map[string]any)
	if !ok {
		t.Fatalf("first history item has type %T, want map[string]any", history[0])
	}
	if _, ok := first["assistantResponseMessage"]; ok {
		t.Fatalf("unexpected synthetic assistantResponseMessage in history")
	}
	userMsg, ok := first["userInputMessage"].(map[string]any)
	if !ok {
		t.Fatalf("userInputMessage has type %T, want map[string]any", first["userInputMessage"])
	}
	content, _ := userMsg["content"].(string)
	if strings.Contains(content, "When the Write or Edit tool has content size limits") {
		t.Fatalf("unexpected injected chunked policy in history content: %q", content)
	}
}

func TestConvertAnthropicRequestWithModel_FiltersBillingHeaderAndInjectsThinkingPrefix(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":[
			{"type":"text","text":"x-anthropic-billing-header: should-be-removed"},
			{"type":"text","text":"Follow repository constraints."}
		],
		"thinking":{"type":"enabled","budget_tokens":5000},
		"messages":[{"role":"user","content":"hello"}]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result.Body, &payload); err != nil {
		t.Fatalf("unmarshal converted payload: %v", err)
	}

	history := payload["conversationState"].(map[string]any)["history"].([]any)
	first := history[0].(map[string]any)["userInputMessage"].(map[string]any)
	content := first["content"].(string)
	if strings.Contains(content, "x-anthropic-billing-header") {
		t.Fatalf("billing header leaked into system history: %q", content)
	}
	if !strings.Contains(content, "<thinking_mode>enabled</thinking_mode><max_thinking_length>5000</max_thinking_length>") {
		t.Fatalf("missing thinking prefix in system history: %q", content)
	}
	if !strings.Contains(content, "Follow repository constraints.") {
		t.Fatalf("missing system prompt in system history: %q", content)
	}
}

func TestConvertAnthropicRequestWithModel_FiltersUnsupportedServerTools(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"hello"}],
		"tools":[
			{"type":"server_tool","name":"web_search","description":"search"},
			{"type":"web_search_20250305","name":"web_search_20250305","description":"search v2"},
			{"name":"local_tool","description":"local","input_schema":{"type":"object"}}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result.Body, &payload); err != nil {
		t.Fatalf("unmarshal converted payload: %v", err)
	}

	state := payload["conversationState"].(map[string]any)
	current := state["currentMessage"].(map[string]any)
	userMsg := current["userInputMessage"].(map[string]any)
	ctx := userMsg["userInputMessageContext"].(map[string]any)
	tools, ok := ctx["tools"].([]any)
	if !ok {
		t.Fatalf("tools has type %T, want []any", ctx["tools"])
	}
	if len(tools) != 1 {
		t.Fatalf("tools length = %d, want 1", len(tools))
	}
	spec := tools[0].(map[string]any)["toolSpecification"].(map[string]any)
	if got := spec["name"]; got != "local_tool" {
		t.Fatalf("tool name = %v, want local_tool", got)
	}
}

func TestConvertAnthropicRequestWithModel_TruncatesLongToolNamesAndPreservesMap(t *testing.T) {
	longName := "mcp__server_name_with_really_long_prefix_that_exceeds_limits__tool_name_with_really_long_suffix"
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{"role":"assistant","content":[{"type":"tool_use","id":"tool-1","name":"` + longName + `","input":{"k":"v"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-1","content":"done"}]}
		],
		"tools":[
			{"name":"` + longName + `","description":"local","input_schema":{"type":"object"}}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	if len(result.ToolNameMap) != 1 {
		t.Fatalf("tool name map length = %d, want 1", len(result.ToolNameMap))
	}

	var shortName string
	for k, v := range result.ToolNameMap {
		shortName = k
		if v != longName {
			t.Fatalf("tool name map value = %q, want %q", v, longName)
		}
	}
	if len(shortName) > 63 {
		t.Fatalf("short tool name length = %d, want <= 63", len(shortName))
	}

	var payload map[string]any
	if err := json.Unmarshal(result.Body, &payload); err != nil {
		t.Fatalf("unmarshal converted payload: %v", err)
	}

	current := payload["conversationState"].(map[string]any)["currentMessage"].(map[string]any)
	ctx := current["userInputMessage"].(map[string]any)["userInputMessageContext"].(map[string]any)
	tools := ctx["tools"].([]any)
	spec := tools[0].(map[string]any)["toolSpecification"].(map[string]any)
	if got := spec["name"].(string); got != shortName {
		t.Fatalf("tool spec name = %q, want %q", got, shortName)
	}

	history := payload["conversationState"].(map[string]any)["history"].([]any)
	assistant := history[0].(map[string]any)["assistantResponseMessage"].(map[string]any)
	if got := assistant["toolUses"].([]any)[0].(map[string]any)["name"].(string); got != shortName {
		t.Fatalf("history tool use name = %q, want %q", got, shortName)
	}
	if got := assistant["content"].(string); got == "" {
		t.Fatal("assistant tool-only history content should not be empty")
	}
}

func TestConvertAnthropicRequestWithModel_CleansOrphanToolPairsAndFillsToolOnlyContent(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{"role":"assistant","content":[{"type":"tool_use","id":"orphan-use","name":"local_tool","input":{}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"orphan-result","content":"orphan result"}]},
			{"role":"assistant","content":[{"type":"tool_use","id":"paired-id","name":"local_tool","input":{}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"paired-id","content":"paired result"}]},
			{"role":"assistant","content":[{"type":"tool_use","id":"current-id","name":"local_tool","input":{}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"current-id","content":"current result"}]}
		],
		"tools":[
			{"name":"local_tool","description":"local","input_schema":{"type":"object"}}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result.Body, &payload); err != nil {
		t.Fatalf("unmarshal converted payload: %v", err)
	}

	state := payload["conversationState"].(map[string]any)
	history := state["history"].([]any)
	if len(history) != 5 {
		t.Fatalf("history length = %d, want 5", len(history))
	}

	firstAssistant := history[0].(map[string]any)["assistantResponseMessage"].(map[string]any)
	if _, ok := firstAssistant["toolUses"]; ok {
		t.Fatal("orphan assistant tool_use should be removed")
	}
	if got := firstAssistant["content"].(string); got != "" {
		t.Fatalf("orphan assistant text content = %q, want empty", got)
	}

	firstUser := history[1].(map[string]any)["userInputMessage"].(map[string]any)
	if _, ok := firstUser["userInputMessageContext"]; ok {
		t.Fatal("orphan user tool_result should be removed")
	}
	if got := firstUser["content"].(string); got != "" {
		t.Fatalf("orphan user text content = %q, want empty", got)
	}

	thirdAssistant := history[4].(map[string]any)["assistantResponseMessage"].(map[string]any)
	toolUses := thirdAssistant["toolUses"].([]any)
	if len(toolUses) != 1 {
		t.Fatalf("current-paired assistant toolUses length = %d, want 1", len(toolUses))
	}
	if got := toolUses[0].(map[string]any)["toolUseId"].(string); got != "current-id" {
		t.Fatalf("current-paired assistant toolUseId = %q, want current-id", got)
	}
	if got := thirdAssistant["content"].(string); got != "I will call the requested tools." {
		t.Fatalf("current-paired assistant content = %q, want placeholder", got)
	}

	current := state["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	if got := current["content"].(string); got != "Here are the tool results." {
		t.Fatalf("current tool-only content = %q, want placeholder", got)
	}
}
