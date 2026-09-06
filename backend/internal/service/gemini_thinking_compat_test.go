package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertClaudeMessagesToGeminiThinkingUsesMappedModel(t *testing.T) {
	for _, tc := range []struct{ model, want string }{
		{"gemini-3-flash-preview", `{"includeThoughts":true,"thinkingLevel":"medium"}`},
		{"gemini-2.5-flash", `{"includeThoughts":true,"thinkingBudget":8192}`},
	} {
		t.Run(tc.model, func(t *testing.T) {
			body, err := convertClaudeMessagesToGeminiGenerateContent([]byte(`{"model":"public-alias","max_tokens":4096,"messages":[{"role":"user","content":"Hello"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"medium"}}`), tc.model)
			require.NoError(t, err)
			var payload struct {
				GenerationConfig struct {
					Thinking json.RawMessage `json:"thinkingConfig"`
				} `json:"generationConfig"`
			}
			require.NoError(t, json.Unmarshal(body, &payload))
			require.JSONEq(t, tc.want, string(payload.GenerationConfig.Thinking))
		})
	}
}
