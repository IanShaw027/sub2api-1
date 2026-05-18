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
