package kiro

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// OOXML(docx/xlsx)服务端文本提取,纯标准库实现(archive/zip + encoding/xml),无第三方依赖。
//
// 与 PDF 提取一致:失败/空/畸形一律返回错误,由调用方回退到占位说明,绝不 panic 崩溃网关。

// officeMaxEntryBytes 限制单个 zip entry 解压后累计字节,防 zip 炸弹 / OOM。
const (
	officeMaxEntryBytes      = 8 << 20  // 8MB per XML entry
	officeMaxDecompressBytes = 32 << 20 // 32MB cumulative XML budget across a workbook
	officeMaxSheets          = 64
)

var errOfficeZipEntryNotFound = errors.New("office zip entry not found")

// openOfficeZip 打开 OOXML 容器。
func openOfficeZip(data []byte) (*zip.Reader, error) {
	if len(data) == 0 {
		return nil, errors.New("empty office data")
	}
	return zip.NewReader(bytes.NewReader(data), int64(len(data)))
}

// readZipEntry 读取 zip 内指定条目,带解压上限保护。
func readZipEntry(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer func() { _ = rc.Close() }()
		raw, readErr := io.ReadAll(io.LimitReader(rc, officeMaxEntryBytes+1))
		if readErr != nil {
			return nil, readErr
		}
		if len(raw) > officeMaxEntryBytes {
			return nil, fmt.Errorf("zip entry %s exceeds decompressed size limit of %d bytes", name, officeMaxEntryBytes)
		}
		return raw, nil
	}
	return nil, fmt.Errorf("%w: %s", errOfficeZipEntryNotFound, name)
}

func readZipEntryWithBudget(zr *zip.Reader, name string, remaining *int) ([]byte, error) {
	raw, err := readZipEntry(zr, name)
	if err != nil {
		return nil, err
	}
	if remaining == nil {
		return raw, nil
	}
	if len(raw) > *remaining {
		return nil, fmt.Errorf("xlsx decompressed size exceeds cumulative limit of %d bytes", officeMaxDecompressBytes)
	}
	*remaining -= len(raw)
	return raw, nil
}

// extractDocxText 从 Word .docx 提取纯文本。
func extractDocxText(data []byte) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			text = ""
			err = fmt.Errorf("docx extraction panic: %v", r)
		}
	}()

	zr, zerr := openOfficeZip(data)
	if zerr != nil {
		return "", zerr
	}
	raw, rerr := readZipEntry(zr, "word/document.xml")
	if rerr != nil {
		return "", rerr
	}

	limiter := newDocumentTextLimiter(maxDocumentExtractedRunes)
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, terr := dec.Token()
		if terr == io.EOF {
			break
		}
		if terr != nil {
			return "", terr
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "tab":
				_ = limiter.WriteByte('\t')
			case "br", "cr":
				_ = limiter.WriteByte('\n')
			case "t":
				// <w:t> 文本节点:读取其字符数据
				var content string
				if derr := dec.DecodeElement(&content, &t); derr == nil {
					limiter.WriteString(content)
				}
			}
		case xml.EndElement:
			// 段落结束补换行
			if t.Name.Local == "p" {
				_ = limiter.WriteByte('\n')
			}
		}
		if limiter.Truncated() {
			break
		}
	}

	result := strings.TrimSpace(limiter.String())
	if result == "" {
		return "", errors.New("docx has no extractable text")
	}
	if !isMostlyPrintable(result) {
		return "", errors.New("docx text is garbled")
	}
	if limiter.Truncated() {
		result = appendDocumentLimitNote(result, documentTextTruncatedNote())
	}
	return result, nil
}

// --- xlsx ---

type xlsxSharedStrings struct {
	Items []xlsxSI `xml:"si"`
}

type xlsxSI struct {
	// 简单字符串 <si><t>,或富文本 <si><r><t>(多段)
	T string   `xml:"t"`
	R []xlsxRT `xml:"r"`
}

type xlsxRT struct {
	T string `xml:"t"`
}

func (si xlsxSI) text() string {
	if len(si.R) > 0 {
		var sb strings.Builder
		for _, r := range si.R {
			_, _ = sb.WriteString(r.T)
		}
		return sb.String()
	}
	return si.T
}

// extractXlsxText 从 Excel .xlsx 提取纯文本(制表分列,换行分行,多 sheet 分段)。
func extractXlsxText(data []byte) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			text = ""
			err = fmt.Errorf("xlsx extraction panic: %v", r)
		}
	}()

	zr, zerr := openOfficeZip(data)
	if zerr != nil {
		return "", zerr
	}

	// 1) 解析共享字符串表(可能不存在)
	remainingBytes := officeMaxDecompressBytes
	var shared []string
	if raw, rerr := readZipEntryWithBudget(zr, "xl/sharedStrings.xml", &remainingBytes); rerr == nil {
		var ss xlsxSharedStrings
		if xml.Unmarshal(raw, &ss) == nil {
			shared = make([]string, len(ss.Items))
			for i, si := range ss.Items {
				shared[i] = si.text()
			}
		}
	} else if !errors.Is(rerr, errOfficeZipEntryNotFound) {
		return "", rerr
	}

	// 2) 收集所有 sheet 文件并按名排序(sheet1, sheet2 ...)
	sheetNames := make([]string, 0, 4)
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			sheetNames = append(sheetNames, f.Name)
		}
	}
	if len(sheetNames) == 0 {
		return "", errors.New("xlsx has no worksheets")
	}
	sort.Slice(sheetNames, func(i, j int) bool {
		return xlsxSheetNameLess(sheetNames[i], sheetNames[j])
	})
	sheetLimitReached := false
	if len(sheetNames) > officeMaxSheets {
		sheetNames = sheetNames[:officeMaxSheets]
		sheetLimitReached = true
	}

	limiter := newDocumentTextLimiter(maxDocumentExtractedRunes)
	for idx, name := range sheetNames {
		raw, rerr := readZipEntryWithBudget(zr, name, &remainingBytes)
		if rerr != nil {
			if errors.Is(rerr, errOfficeZipEntryNotFound) {
				continue
			}
			return "", rerr
		}
		sheetText := parseXlsxSheet(raw, shared)
		if strings.TrimSpace(sheetText) == "" {
			continue
		}
		if len(sheetNames) > 1 {
			limiter.WriteString(fmt.Sprintf("=== Sheet %d ===\n", idx+1))
		}
		limiter.WriteString(sheetText)
		_ = limiter.WriteByte('\n')
		if limiter.Truncated() {
			break
		}
	}

	result := strings.TrimSpace(limiter.String())
	if result == "" {
		return "", errors.New("xlsx has no extractable text")
	}
	if !isMostlyPrintable(result) {
		return "", errors.New("xlsx text is garbled")
	}
	if limiter.Truncated() {
		result = appendDocumentLimitNote(result, documentTextTruncatedNote())
	}
	if sheetLimitReached {
		result = appendDocumentLimitNote(result, fmt.Sprintf("(Note: worksheet limit reached at %d sheets.)", officeMaxSheets))
	}
	return result, nil
}

// parseXlsxSheet 流式解析单个 sheet 的 sheetData,还原为制表/换行的文本表格。
func parseXlsxSheet(raw []byte, shared []string) string {
	return parseXlsxSheetLimited(raw, shared, maxDocumentExtractedRunes)
}

func xlsxSheetNameLess(a, b string) bool {
	ai := xlsxSheetIndex(a)
	bi := xlsxSheetIndex(b)
	if ai >= 0 && bi >= 0 && ai != bi {
		return ai < bi
	}
	return a < b
}

func xlsxSheetIndex(name string) int {
	base := strings.TrimPrefix(name, "xl/worksheets/sheet")
	base = strings.TrimSuffix(base, ".xml")
	return atoiSafe(base)
}

func parseXlsxSheetLimited(raw []byte, shared []string, maxRunes int) string {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	limiter := newDocumentTextLimiter(maxRunes)
	var (
		inRow    bool
		cellType string
		inValue  bool
		inline   bool
		cellVal  strings.Builder
		firstCol bool
	)
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				inRow = true
				firstCol = true
			case "c":
				cellType = ""
				for _, a := range t.Attr {
					if a.Name.Local == "t" {
						cellType = a.Value
					}
				}
				cellVal.Reset()
			case "v":
				inValue = true
			case "t":
				// inlineStr 的 <is><t> 或 sharedStrings 里(此处只在 cell 内)
				inline = true
			}
		case xml.CharData:
			if inValue || inline {
				_, _ = cellVal.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "v":
				inValue = false
			case "t":
				inline = false
			case "c":
				val := cellVal.String()
				if cellType == "s" {
					// 共享字符串索引
					if idx := atoiSafe(val); idx >= 0 && idx < len(shared) {
						val = shared[idx]
					} else {
						val = ""
					}
				}
				if inRow {
					if !firstCol {
						_ = limiter.WriteByte('\t')
					}
					limiter.WriteString(val)
					firstCol = false
				}
			case "row":
				_ = limiter.WriteByte('\n')
				inRow = false
			}
		}
		if limiter.Truncated() {
			break
		}
	}
	return limiter.String()
}

// atoiSafe 解析非负整数,失败返回 -1。
func atoiSafe(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return -1
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	return n
}
