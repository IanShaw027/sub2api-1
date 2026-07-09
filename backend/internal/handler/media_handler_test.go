package handler

import (
	"strings"
	"testing"
)

func TestBuildDownloadContentDisposition_EncodesUnsafeCharacters(t *testing.T) {
	header := buildDownloadContentDisposition("evil\"\r\nX-Test: injected.pdf")
	if strings.Contains(header, "\r") || strings.Contains(header, "\n") {
		t.Fatalf("header contains raw newlines: %q", header)
	}
	if strings.Contains(header, "X-Test: injected") {
		t.Fatalf("header contains unescaped injected content: %q", header)
	}
	if !strings.Contains(header, "filename*=") {
		t.Fatalf("header missing RFC 5987 filename*: %q", header)
	}
}

func TestBuildDownloadContentDisposition_EncodesUTF8FileName(t *testing.T) {
	header := buildDownloadContentDisposition("发票 2026.pdf")
	if !strings.Contains(header, "filename*=") {
		t.Fatalf("header missing encoded utf-8 filename*: %q", header)
	}
	if strings.Contains(header, "发票 2026.pdf") {
		t.Fatalf("header should not contain raw utf-8 filename: %q", header)
	}
}
