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
