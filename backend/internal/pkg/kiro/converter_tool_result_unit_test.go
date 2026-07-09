//go:build unit

package kiro

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConverterToolResultContent_OmitsImageBlockData(t *testing.T) {
	imageData := strings.Repeat("aGk=", 1024)
	content := []any{
		map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": "image/png",
				"data":       imageData,
			},
		},
	}

	text := toolResultContent(content)

	require.Equal(t, "[image omitted]", text)
	require.NotContains(t, text, imageData)
}

func TestKiroEstimateInputTokens_ToolResultImageUsesPlaceholder(t *testing.T) {
	imageData := strings.Repeat("aGk=", 8192)
	imageBody := mustMarshalKiroTestBody(t, []any{
		map[string]any{
			"type":        "tool_result",
			"tool_use_id": "toolu_read_image",
			"content": []any{
				map[string]any{
					"type": "image",
					"source": map[string]any{
						"type":       "base64",
						"media_type": "image/png",
						"data":       imageData,
					},
				},
			},
		},
	})
	placeholderBody := mustMarshalKiroTestBody(t, []any{
		map[string]any{
			"type":        "tool_result",
			"tool_use_id": "toolu_read_image",
			"content": []any{
				map[string]any{
					"type": "text",
					"text": "[image omitted]",
				},
			},
		},
	})

	require.Equal(t, EstimateInputTokens(placeholderBody), EstimateInputTokens(imageBody))
}

func mustMarshalKiroTestBody(t *testing.T, content []any) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"model": "claude-sonnet-4-6",
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": content,
			},
		},
	})
	require.NoError(t, err)
	return body
}
