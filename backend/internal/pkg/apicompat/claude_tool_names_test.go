package apicompat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	require.Equal(t, "tool_use", AnthropicStopReasonString(anth.StopReason))
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

	require.Equal(t, "tool_use", AnthropicStopReasonString(anth.StopReason))
	require.Len(t, anth.Content, 1)
	require.Equal(t, "tool_use", anth.Content[0].Type)
	require.Equal(t, "call_search", anth.Content[0].ID)
	require.Equal(t, "tool_search", anth.Content[0].Name)
	require.JSONEq(t, `{"query":"docs","limit":3}`, string(anth.Content[0].Input))
}
