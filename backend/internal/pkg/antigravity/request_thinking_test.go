package antigravity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransformClaudeThinkingUsesMappedModelAndEffort(t *testing.T) {
	for _, tc := range []struct{ model, mode, effort, want string }{
		{"gemini-3-flash-preview", "adaptive", "minimal", `{"includeThoughts":true,"thinkingLevel":"minimal"}`},
		{"gemini-2.5-flash", "adaptive", "medium", `{"includeThoughts":true,"thinkingBudget":8192}`},
		{"gemini-2.5-flash", "disabled", "none", `{"includeThoughts":false,"thinkingBudget":0}`},
		{"claude-opus-4-6-thinking", "adaptive", "low", `{"includeThoughts":true,"thinkingBudget":24576}`},
	} {
		t.Run(tc.model+tc.mode, func(t *testing.T) {
			var req ClaudeRequest
			require.NoError(t, json.Unmarshal([]byte(`{"model":"public-alias","max_tokens":32000,"messages":[{"role":"user","content":"Hello"}],"thinking":{"type":"`+tc.mode+`"},"output_config":{"effort":"`+tc.effort+`"}}`), &req))
			body, err := TransformClaudeToGemini(&req, "test-project", tc.model)
			require.NoError(t, err)
			var payload struct {
				Request struct {
					GenerationConfig struct {
						Thinking json.RawMessage `json:"thinkingConfig"`
					} `json:"generationConfig"`
				} `json:"request"`
			}
			require.NoError(t, json.Unmarshal(body, &payload))
			require.JSONEq(t, tc.want, string(payload.Request.GenerationConfig.Thinking))
		})
	}
}
