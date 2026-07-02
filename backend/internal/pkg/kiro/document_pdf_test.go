package kiro

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractPDFText_ValidTextPDF(t *testing.T) {
	data, err := os.ReadFile("testdata/sample_text.pdf")
	require.NoError(t, err)

	text, err := extractPDFText(data)
	require.NoError(t, err)
	// rsc.io/pdf 会在字形间插空格,归一化后应包含预期词。
	normalized := strings.ReplaceAll(text, " ", "")
	require.Contains(t, normalized, "HelloKiroPDF")
}

func TestExtractPDFText_Empty(t *testing.T) {
	_, err := extractPDFText(nil)
	require.Error(t, err)
}

func TestExtractPDFText_RejectsOversizedInput(t *testing.T) {
	_, err := extractPDFText(bytes.Repeat([]byte("%"), maxPDFInputBytes+1))
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds")
}

func TestExtractPDFText_NotAPDF(t *testing.T) {
	_, err := extractPDFText([]byte("this is plain text, not a pdf"))
	require.Error(t, err)
}

func TestExtractPDFText_MalformedRecovers(t *testing.T) {
	// 畸形/截断的 PDF 头:不能 panic,必须返回错误。
	garbage := []byte("%PDF-1.4\n" + strings.Repeat("\x00\xff\x01", 200))
	require.NotPanics(t, func() {
		_, _ = extractPDFText(garbage)
	})
	_, err := extractPDFText(garbage)
	require.Error(t, err)
}

func TestExtractDocumentText_PDFBase64(t *testing.T) {
	data, err := os.ReadFile("testdata/sample_text.pdf")
	require.NoError(t, err)
	b64 := base64.StdEncoding.EncodeToString(data)

	block := map[string]any{
		"type":  "document",
		"title": "report",
		"source": map[string]any{
			"type":       "base64",
			"media_type": "application/pdf",
			"data":       b64,
		},
	}
	out := extractDocumentText(block)
	require.Contains(t, out, "[Document: report]")
	normalized := strings.ReplaceAll(out, " ", "")
	require.Contains(t, normalized, "HelloKiroPDF")
	require.NotContains(t, out, "not extractable")
}

func TestExtractDocumentText_PDFGarbledFallsBackToPlaceholder(t *testing.T) {
	// 无法提取文本的 PDF(此处用非 PDF 数据触发失败)应回退占位,而非空串。
	b64 := base64.StdEncoding.EncodeToString([]byte("not a real pdf"))
	block := map[string]any{
		"type": "document",
		"source": map[string]any{
			"type":       "base64",
			"media_type": "application/pdf",
			"data":       b64,
		},
	}
	out := extractDocumentText(block)
	require.Contains(t, out, "not extractable")
}

func TestExtractDocumentText_TextBase64TruncatesLargeDecodedText(t *testing.T) {
	hugeText := strings.Repeat("word ", maxDocumentExtractedRunes/5+1000)
	block := map[string]any{
		"type":  "document",
		"title": "large.txt",
		"source": map[string]any{
			"type":       "base64",
			"media_type": "text/plain",
			"data":       base64.StdEncoding.EncodeToString([]byte(hugeText)),
		},
	}

	out := extractDocumentText(block)

	require.Contains(t, out, "[Document: large.txt]")
	require.Contains(t, out, "truncated")
	require.LessOrEqual(t, len([]rune(out)), maxDocumentExtractedRunes+256)
}

func TestIsMostlyPrintable(t *testing.T) {
	require.True(t, isMostlyPrintable("Hello world, this is normal text."))
	require.True(t, isMostlyPrintable("中文文本也算可打印"))
	require.False(t, isMostlyPrintable(fmt.Sprintf("abc%s", strings.Repeat("\x00\x01\x02", 50))))
	require.False(t, isMostlyPrintable(""))
}
