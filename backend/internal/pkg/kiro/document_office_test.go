package kiro

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractDocxText_Valid(t *testing.T) {
	data, err := os.ReadFile("testdata/sample.docx")
	require.NoError(t, err)

	text, err := extractDocxText(data)
	require.NoError(t, err)
	require.Contains(t, text, "Hello Word document")
	require.Contains(t, text, "Second paragraph here")
}

func TestExtractXlsxText_InlineStrings(t *testing.T) {
	data, err := os.ReadFile("testdata/sample.xlsx")
	require.NoError(t, err)

	text, err := extractXlsxText(data)
	require.NoError(t, err)
	require.Contains(t, text, "Name")
	require.Contains(t, text, "Alice")
	require.Contains(t, text, "95")
}

func TestExtractXlsxText_SharedStrings(t *testing.T) {
	data, err := os.ReadFile("testdata/sample_shared.xlsx")
	require.NoError(t, err)

	text, err := extractXlsxText(data)
	require.NoError(t, err)
	require.Contains(t, text, "City")
	require.Contains(t, text, "Tokyo")
	require.Contains(t, text, "Paris")
}

func TestExtractDocxText_NotAZip(t *testing.T) {
	require.NotPanics(t, func() {
		_, _ = extractDocxText([]byte("not a zip file at all"))
	})
	_, err := extractDocxText([]byte("not a zip file at all"))
	require.Error(t, err)
}

func TestExtractXlsxText_Empty(t *testing.T) {
	_, err := extractXlsxText(nil)
	require.Error(t, err)
}

func TestExtractDocxText_ZipWithoutDocument(t *testing.T) {
	// 合法 zip 但没有 word/document.xml → 错误,不 panic。
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("random.txt")
	_, _ = w.Write([]byte("hello"))
	_ = zw.Close()

	require.NotPanics(t, func() {
		_, _ = extractDocxText(buf.Bytes())
	})
	_, err := extractDocxText(buf.Bytes())
	require.Error(t, err)
}

func TestReadZipEntryRejectsOversizedEntry(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("word/document.xml")
	_, _ = w.Write(bytes.Repeat([]byte("x"), officeMaxEntryBytes+1))
	_ = zw.Close()
	zr, err := openOfficeZip(buf.Bytes())
	require.NoError(t, err)

	_, err = readZipEntry(zr, "word/document.xml")

	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds")
}

func TestExtractXlsxTextLimitsWorksheetCount(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 1; i <= officeMaxSheets+1; i++ {
		w, _ := zw.Create(fmt.Sprintf("xl/worksheets/sheet%d.xml", i))
		_, _ = w.Write([]byte(fmt.Sprintf(`<worksheet><sheetData><row><c t="inlineStr"><is><t>sheet-%d</t></is></c></row></sheetData></worksheet>`, i)))
	}
	_ = zw.Close()

	text, err := extractXlsxText(buf.Bytes())

	require.NoError(t, err)
	require.Contains(t, text, "sheet-1")
	require.NotContains(t, text, fmt.Sprintf("sheet-%d", officeMaxSheets+1))
	require.Contains(t, text, "worksheet limit reached")
}

func TestExtractDocumentText_DocxBase64(t *testing.T) {
	data, err := os.ReadFile("testdata/sample.docx")
	require.NoError(t, err)
	block := map[string]any{
		"type":  "document",
		"title": "report.docx",
		"source": map[string]any{
			"type":       "base64",
			"media_type": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"data":       base64.StdEncoding.EncodeToString(data),
		},
	}
	out := extractDocumentText(block)
	require.Contains(t, out, "[Document: report.docx]")
	require.Contains(t, out, "Hello Word document")
	require.NotContains(t, out, "not extractable")
}

func TestExtractDocumentText_XlsxBase64(t *testing.T) {
	data, err := os.ReadFile("testdata/sample_shared.xlsx")
	require.NoError(t, err)
	block := map[string]any{
		"type":  "document",
		"title": "data.xlsx",
		"source": map[string]any{
			"type":       "base64",
			"media_type": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			"data":       base64.StdEncoding.EncodeToString(data),
		},
	}
	out := extractDocumentText(block)
	require.Contains(t, out, "[Document: data.xlsx]")
	require.Contains(t, out, "Tokyo")
	require.NotContains(t, out, "not extractable")
}

func TestExtractDocumentText_UnknownOfficeFallsBack(t *testing.T) {
	// 老 .doc(二进制)无法解析 → 回退占位。
	block := map[string]any{
		"type": "document",
		"source": map[string]any{
			"type":       "base64",
			"media_type": "application/msword",
			"data":       base64.StdEncoding.EncodeToString([]byte("\xd0\xcf\x11\xe0binary doc")),
		},
	}
	out := extractDocumentText(block)
	require.Contains(t, out, "not extractable")
}

func TestAtoiSafe(t *testing.T) {
	require.Equal(t, 0, atoiSafe("0"))
	require.Equal(t, 42, atoiSafe("42"))
	require.Equal(t, -1, atoiSafe(""))
	require.Equal(t, -1, atoiSafe("1.5"))
	require.Equal(t, -1, atoiSafe("abc"))
}

func TestParseXlsxSheet_TabSeparated(t *testing.T) {
	// 简单 sheet:两列一行,inlineStr。
	xml := `<worksheet><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>foo</t></is></c><c r="B1" t="inlineStr"><is><t>bar</t></is></c></row></sheetData></worksheet>`
	out := parseXlsxSheet([]byte(xml), nil)
	require.Equal(t, "foo\tbar\n", out)
}

func TestParseXlsxSheet_SharedIndex(t *testing.T) {
	shared := []string{"hello", "world"}
	xml := `<worksheet><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row></sheetData></worksheet>`
	out := parseXlsxSheet([]byte(xml), shared)
	require.Equal(t, "hello\tworld\n", strings.TrimRight(out, "\n")+"\n")
}
