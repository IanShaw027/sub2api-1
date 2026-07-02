package kiro

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxDocumentDecodedBytes   = 20 << 20 // 20MB decoded attachment cap
	maxDocumentExtractedRunes = 200_000
)

type documentTextLimiter struct {
	sb        strings.Builder
	written   int
	maxRunes  int
	truncated bool
}

func newDocumentTextLimiter(maxRunes int) *documentTextLimiter {
	return &documentTextLimiter{maxRunes: maxRunes}
}

func (l *documentTextLimiter) WriteString(s string) {
	if l == nil || s == "" || l.truncated {
		return
	}
	if l.maxRunes <= 0 {
		l.sb.WriteString(s)
		return
	}
	remaining := l.maxRunes - l.written
	if remaining <= 0 {
		l.truncated = true
		return
	}
	count := utf8.RuneCountInString(s)
	if count <= remaining {
		l.sb.WriteString(s)
		l.written += count
		return
	}
	runes := []rune(s)
	l.sb.WriteString(string(runes[:remaining]))
	l.written += remaining
	l.truncated = true
}

func (l *documentTextLimiter) WriteByte(b byte) {
	if l == nil || l.truncated {
		return
	}
	if l.maxRunes > 0 && l.written >= l.maxRunes {
		l.truncated = true
		return
	}
	l.sb.WriteByte(b)
	l.written++
}

func (l *documentTextLimiter) String() string {
	if l == nil {
		return ""
	}
	return l.sb.String()
}

func (l *documentTextLimiter) Truncated() bool {
	return l != nil && l.truncated
}

func limitDocumentText(text string) (string, bool) {
	if maxDocumentExtractedRunes <= 0 || utf8.RuneCountInString(text) <= maxDocumentExtractedRunes {
		return text, false
	}
	runes := []rune(text)
	return string(runes[:maxDocumentExtractedRunes]), true
}

func appendDocumentLimitNote(text, note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return strings.TrimSpace(text)
	}
	if strings.TrimSpace(text) == "" {
		return note
	}
	return strings.TrimSpace(text) + "\n\n" + note
}

func documentTextTruncatedNote() string {
	return fmt.Sprintf("(Note: document text truncated to %d characters.)", maxDocumentExtractedRunes)
}

func formatDocumentText(title, text string) string {
	text, truncated := limitDocumentText(text)
	if truncated {
		text = appendDocumentLimitNote(text, documentTextTruncatedNote())
	}
	if title != "" {
		return fmt.Sprintf("[Document: %s]\n%s", title, text)
	}
	return text
}
