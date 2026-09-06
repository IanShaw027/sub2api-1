package gemini

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeThinkingConfig(t *testing.T) {
	for _, tc := range []struct {
		name, model, mode, effort, want string
		budget                          int
	}{
		{"flash minimal", "gemini-3-flash-preview", "adaptive", "minimal", `{"includeThoughts":true,"thinkingLevel":"minimal"}`, 0},
		{"pro medium", "gemini-3-pro-preview", "adaptive", "medium", `{"includeThoughts":true,"thinkingLevel":"high"}`, 0},
		{"pro 3.1 medium", "gemini-3.1-pro-preview", "adaptive", "medium", `{"includeThoughts":true,"thinkingLevel":"medium"}`, 0},
		{"pro cannot disable", "gemini-3.1-pro-preview", "disabled", "none", `{"includeThoughts":false,"thinkingLevel":"low"}`, 0},
		{"budget to level", "gemini-3-flash-preview", "enabled", "", `{"includeThoughts":true,"thinkingLevel":"medium"}`, 8192},
		{"flash effort budget", "gemini-2.5-flash", "adaptive", "low", `{"includeThoughts":true,"thinkingBudget":1024}`, 0},
		{"flash disabled zero", "gemini-2.5-flash", "disabled", "none", `{"includeThoughts":false,"thinkingBudget":0}`, 0},
		{"2.5 pro cannot disable", "gemini-2.5-pro", "disabled", "none", `{"includeThoughts":false,"thinkingBudget":128}`, 0},
		{"flash budget clamp", "gemini-2.5-flash", "enabled", "", `{"includeThoughts":true,"thinkingBudget":24576}`, 32768},
		{"lite minimum", "gemini-2.5-flash-lite", "enabled", "", `{"includeThoughts":true,"thinkingBudget":512}`, 1},
		{"dynamic", "gemini-2.5-pro", "adaptive", "", `{"includeThoughts":true,"thinkingBudget":-1}`, 0},
		{"unsupported", "gemini-2.0-flash", "adaptive", "high", `null`, 0},
		{"image unsupported", "gemini-2.5-flash-image", "adaptive", "high", `null`, 0},
		{"default unchanged", "gemini-3-flash-preview", "", "", `null`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(ClaudeThinkingConfig(tc.model, tc.mode, tc.budget, tc.effort))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(got))
		})
	}
}
