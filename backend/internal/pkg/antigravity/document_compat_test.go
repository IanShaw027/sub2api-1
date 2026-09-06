package antigravity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildPartsPreservesInlinePDF(t *testing.T) {
	parts, _, err := buildParts(json.RawMessage(`[{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"JVBERi0xLjQ="}}]`), map[string]string{}, true)
	require.NoError(t, err)
	require.Len(t, parts, 1)
	require.Equal(t, &GeminiInlineData{MimeType: "application/pdf", Data: "JVBERi0xLjQ="}, parts[0].InlineData)
}

func TestBuildPartsRejectsUnsupportedDocument(t *testing.T) {
	for _, content := range []string{`[{"type":"document"}]`, `[{"type":"document","source":{"type":"url","url":"https://example.test/file.pdf"}}]`} {
		_, _, err := buildParts(json.RawMessage(content), map[string]string{}, true)
		require.Error(t, err)
	}
}
