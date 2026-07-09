package kiro

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"rsc.io/pdf"
)

const (
	maxPDFInputBytes = 20 << 20
	maxPDFPages      = 100
)

// extractPDFText 从 PDF 字节中提取纯文本(纯 Go 实现,基于 rsc.io/pdf)。
//
// 适用于文本型 PDF;扫描件/图片型、使用自定义 CID 字体编码、或含 rsc.io/pdf
// 不支持的流过滤器(如 ASCII85Decode)的 PDF 会返回错误,由调用方回退到占位说明。
//
// 注意:rsc.io/pdf 对不支持的过滤器/编码会 panic,这里用 recover 兜底,
// 避免用户上传的畸形 PDF 导致网关 goroutine 崩溃。
func extractPDFText(data []byte) (text string, err error) {
	if len(data) == 0 {
		return "", errors.New("empty pdf data")
	}
	if len(data) > maxPDFInputBytes {
		return "", fmt.Errorf("pdf data exceeds size limit of %d bytes", maxPDFInputBytes)
	}
	defer func() {
		if r := recover(); r != nil {
			text = ""
			err = fmt.Errorf("pdf extraction panic (unsupported encoding/filter): %v", r)
		}
	}()

	reader, rerr := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if rerr != nil {
		return "", rerr
	}
	limiter := newDocumentTextLimiter(maxDocumentExtractedRunes)
	numPages := reader.NumPage()
	pagesToRead := numPages
	pageLimitReached := false
	if pagesToRead > maxPDFPages {
		pagesToRead = maxPDFPages
		pageLimitReached = true
	}
	for i := 1; i <= pagesToRead; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		content := page.Content()
		var lastY float64
		first := true
		for _, t := range content.Text {
			if t.S == "" {
				continue
			}
			if !first && t.Y != lastY {
				_ = limiter.WriteByte('\n')
			} else if !first {
				_ = limiter.WriteByte(' ')
			}
			limiter.WriteString(t.S)
			lastY = t.Y
			first = false
			if limiter.Truncated() {
				break
			}
		}
		if limiter.Truncated() {
			break
		}
		if i < pagesToRead {
			_ = limiter.WriteByte('\n')
		}
	}
	result := strings.TrimSpace(limiter.String())
	if result == "" {
		return "", errors.New("no extractable text (likely scanned/image-only pdf)")
	}
	// 自定义 CID 字体会把字形 ID 当字符输出,产生大量控制字符/替换符的乱码。
	// 若可打印字符占比过低,判定为提取失败,回退占位。
	if !isMostlyPrintable(result) {
		return "", errors.New("extracted text is garbled (custom font encoding not decodable)")
	}
	if limiter.Truncated() {
		result = appendDocumentLimitNote(result, documentTextTruncatedNote())
	}
	if pageLimitReached {
		result = appendDocumentLimitNote(result, fmt.Sprintf("(Note: PDF page limit reached at %d pages.)", maxPDFPages))
	}
	return result, nil
}

// isMostlyPrintable 判断字符串是否以可打印文本为主(阈值 80%)。
func isMostlyPrintable(s string) bool {
	if s == "" {
		return false
	}
	total := 0
	printable := 0
	for _, r := range s {
		if r == utf8.RuneError {
			total++
			continue
		}
		total++
		if r == '\n' || r == '\t' || r == ' ' || unicode.IsPrint(r) {
			printable++
		}
	}
	if total == 0 {
		return false
	}
	return float64(printable)/float64(total) >= 0.8
}
