package service

import (
	"reflect"
	"testing"
)

func TestDropOrphanFunctionCallOutputs(t *testing.T) {
	t.Run("removes_orphan", func(t *testing.T) {
		input := []any{
			map[string]any{"type": "message", "role": "user", "content": "hi"},
			map[string]any{"type": "function_call_output", "call_id": "call_abc", "output": "result"},
		}
		got, dropped := dropOrphanFunctionCallOutputs(input)
		if !dropped {
			t.Fatal("expected dropped=true")
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 item kept, got %d", len(got))
		}
		gotItem, _ := got[0].(map[string]any)
		if gotType, _ := gotItem["type"].(string); gotType != "message" {
			t.Fatalf("expected message kept, got %#v", got[0])
		}
	})

	t.Run("keeps_matched_pair", func(t *testing.T) {
		input := []any{
			map[string]any{"type": "function_call", "call_id": "call_abc", "name": "list"},
			map[string]any{"type": "function_call_output", "call_id": "call_abc", "output": "result"},
		}
		got, dropped := dropOrphanFunctionCallOutputs(input)
		if dropped {
			t.Fatal("expected dropped=false when call_id matches")
		}
		if len(got) != 2 {
			t.Fatalf("expected pair preserved, got %d items", len(got))
		}
	})

	t.Run("function_call_output_does_not_seed_itself", func(t *testing.T) {
		// Two function_call_outputs sharing a call_id but no function_call source.
		// Both must be dropped — neither should "seed" the other.
		input := []any{
			map[string]any{"type": "function_call_output", "call_id": "call_abc", "output": "first"},
			map[string]any{"type": "function_call_output", "call_id": "call_abc", "output": "second"},
		}
		got, dropped := dropOrphanFunctionCallOutputs(input)
		if !dropped {
			t.Fatal("expected dropped=true")
		}
		if len(got) != 0 {
			t.Fatalf("expected both orphans removed, got %d items", len(got))
		}
	})

	t.Run("missing_call_id_dropped", func(t *testing.T) {
		input := []any{
			map[string]any{"type": "function_call_output", "output": "result"},
		}
		got, dropped := dropOrphanFunctionCallOutputs(input)
		if !dropped {
			t.Fatal("expected dropped=true for missing call_id")
		}
		if len(got) != 0 {
			t.Fatalf("expected output removed, got %d items", len(got))
		}
	})

	t.Run("non_function_call_output_untouched", func(t *testing.T) {
		input := []any{
			map[string]any{"type": "reasoning", "id": "rs_1", "summary": "..."},
			map[string]any{"type": "message", "role": "assistant", "content": "..."},
		}
		original := append([]any{}, input...)
		got, dropped := dropOrphanFunctionCallOutputs(input)
		if dropped {
			t.Fatal("expected no drop")
		}
		if !reflect.DeepEqual(got, original) {
			t.Fatalf("expected input unchanged")
		}
	})

	t.Run("call_id_via_other_call_source_types", func(t *testing.T) {
		input := []any{
			map[string]any{"type": "custom_tool_call", "call_id": "call_x", "name": "x"},
			map[string]any{"type": "function_call_output", "call_id": "call_x", "output": "ok"},
		}
		_, dropped := dropOrphanFunctionCallOutputs(input)
		if dropped {
			t.Fatal("expected custom_tool_call to satisfy the call source check")
		}
	})

	t.Run("id_only_tool_search_call_keeps_output", func(t *testing.T) {
		input := []any{
			map[string]any{"type": "tool_search_call", "id": "fc_search_1", "query": "docs"},
			map[string]any{"type": "tool_search_output", "call_id": "fc_search_1", "output": "ok"},
		}
		_, dropped := dropOrphanFunctionCallOutputs(input)
		if dropped {
			t.Fatal("expected tool_search_call id to satisfy the call source check")
		}
	})

	t.Run("drops_output_when_call_source_appears_later", func(t *testing.T) {
		input := []any{
			map[string]any{"type": "tool_search_output", "call_id": "call_late", "output": "ok"},
			map[string]any{"type": "tool_search_call", "call_id": "call_late", "query": "hello"},
		}
		got, dropped := dropOrphanFunctionCallOutputs(input)
		if !dropped {
			t.Fatal("expected dropped=true when output appears before call source")
		}
		if len(got) != 1 {
			t.Fatalf("expected only later call source kept, got %d items", len(got))
		}
		gotItem, _ := got[0].(map[string]any)
		if gotType, _ := gotItem["type"].(string); gotType != "tool_search_call" {
			t.Fatalf("expected call source kept, got %#v", got[0])
		}
	})

	t.Run("item_reference_keeps_later_output", func(t *testing.T) {
		input := []any{
			map[string]any{"type": "item_reference", "id": "call_ref"},
			map[string]any{"type": "mcp_tool_call_output", "call_id": "call_ref", "output": "ok"},
		}
		got, dropped := dropOrphanFunctionCallOutputs(input)
		if dropped {
			t.Fatal("expected item_reference to satisfy the call source check")
		}
		if len(got) != 2 {
			t.Fatalf("expected pair preserved, got %d items", len(got))
		}
	})

	t.Run("item_reference_after_output_drops_orphan", func(t *testing.T) {
		input := []any{
			map[string]any{"type": "custom_tool_call_output", "call_id": "call_ref", "output": "ok"},
			map[string]any{"type": "item_reference", "id": "call_ref"},
		}
		got, dropped := dropOrphanFunctionCallOutputs(input)
		if !dropped {
			t.Fatal("expected dropped=true when output appears before item_reference")
		}
		if len(got) != 1 {
			t.Fatalf("expected only later item_reference kept, got %d items", len(got))
		}
		gotItem, _ := got[0].(map[string]any)
		if gotType, _ := gotItem["type"].(string); gotType != "item_reference" {
			t.Fatalf("expected item_reference kept, got %#v", got[0])
		}
	})
}

func TestSanitizeOpenAIResponsesOrphanToolOutputs_KeepsIDOnlyToolSearchCallOutput(t *testing.T) {
	input := []any{
		map[string]any{"type": "tool_search_call", "id": "fc_search_1", "query": "docs"},
		map[string]any{"type": "tool_search_output", "call_id": "fc_search_1", "output": "ok"},
	}
	reqBody := map[string]any{"input": input}

	modified := sanitizeOpenAIResponsesOrphanToolOutputs(reqBody, input, false)

	if modified {
		t.Fatal("expected matched tool_search_output to be preserved")
	}
	gotInput, ok := reqBody["input"].([]any)
	if !ok {
		t.Fatalf("expected input array, got %#v", reqBody["input"])
	}
	if !reflect.DeepEqual(gotInput, input) {
		t.Fatalf("expected input unchanged, got %#v", gotInput)
	}
}

func TestFilterCodexInputWithOptions_DropOrphanFunctionCallOutputs(t *testing.T) {
	input := []any{
		map[string]any{"type": "message", "role": "user", "content": "hi"},
		map[string]any{"type": "function_call_output", "call_id": "call_orphan", "output": "stale"},
	}
	filtered, modified := filterCodexInputWithOptions(input, codexInputFilterOptions{
		dropOrphanFunctionCallOutputs: true,
	})
	if !modified {
		t.Fatal("expected modified=true")
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 item kept, got %d (items=%#v)", len(filtered), filtered)
	}
	filteredItem, _ := filtered[0].(map[string]any)
	if filteredType, _ := filteredItem["type"].(string); filteredType != "message" {
		t.Fatalf("expected message preserved, got %#v", filtered[0])
	}
}
