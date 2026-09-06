//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiMessagesPreservesInlineDocument(t *testing.T) {
	var messages any
	require.NoError(t, json.Unmarshal([]byte(`[{"role":"user","content":[{"type":"text","text":"Summarize this"},{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"JVBERi0xLjQ="}}]}]`), &messages))
	contents, err := convertClaudeMessagesToGeminiContents(messages, map[string]string{})
	require.NoError(t, err)
	parts := contents[0].(map[string]any)["parts"].([]any)
	require.Len(t, parts, 2)
	require.Equal(t, map[string]any{"inlineData": map[string]any{"mimeType": "application/pdf", "data": "JVBERi0xLjQ="}}, parts[1])
}

func TestGeminiMessagesRejectsUnsupportedDocumentsInsteadOfSendingEncodedText(t *testing.T) {
	for _, source := range []any{nil, map[string]any{"type": "url", "url": "https://example.test/file.pdf"}, map[string]any{"type": "base64", "media_type": "application/pdf", "data": ""}} {
		_, err := convertClaudeMessagesToGeminiContents([]any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "document", "source": source}}}}, map[string]string{})
		require.Error(t, err)
	}
}

func TestGeminiMessagesPreservesTextDocument(t *testing.T) {
	contents, err := convertClaudeMessagesToGeminiContents([]any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "document", "source": map[string]any{"type": "text", "media_type": "text/plain", "data": "report"}}}}}, map[string]string{})
	require.NoError(t, err)
	require.Equal(t, []any{map[string]any{"text": "report"}}, contents[0].(map[string]any)["parts"])
}
