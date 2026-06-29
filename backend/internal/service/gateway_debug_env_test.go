package service

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDebugEnvBool(t *testing.T) {
	t.Run("empty is false", func(t *testing.T) {
		if parseDebugEnvBool("") {
			t.Fatalf("expected false for empty string")
		}
	})

	t.Run("true-like values", func(t *testing.T) {
		for _, value := range []string{"1", "true", "TRUE", "yes", "on"} {
			t.Run(value, func(t *testing.T) {
				if !parseDebugEnvBool(value) {
					t.Fatalf("expected true for %q", value)
				}
			})
		}
	})

	t.Run("false-like values", func(t *testing.T) {
		for _, value := range []string{"0", "false", "off", "debug"} {
			t.Run(value, func(t *testing.T) {
				if parseDebugEnvBool(value) {
					t.Fatalf("expected false for %q", value)
				}
			})
		}
	})
}

func TestDebugLogGatewaySnapshot_RedactsAndTruncatesBody(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "gateway_debug.log")

	svc := &GatewayService{}
	svc.initDebugGatewayBodyFile(path)
	f := svc.debugGatewayBodyFile.Load()
	if f == nil {
		t.Fatalf("expected debug file to be initialized")
	}
	defer f.Close()

	secretText := "USER-PII-CCH-MISMATCH-CONTENT"
	body := []byte(`{"messages":[{"role":"user","content":"` + secretText + strings.Repeat("x", 8192) + `"}]}`)
	svc.debugLogGatewaySnapshot("CLIENT_ORIGINAL", http.Header{"Authorization": {"Bearer secret-token"}}, body, map[string]string{"account": "123(test)"})

	contentBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read debug log: %v", err)
	}
	content := string(contentBytes)

	if strings.Contains(content, secretText) {
		t.Fatalf("debug dump leaked user text: %s", content)
	}
	if strings.Contains(content, string(body)) {
		t.Fatalf("debug dump contains the full request body")
	}
	if !strings.Contains(content, "body_sha256:") {
		t.Fatalf("debug dump should include body hash, got: %s", content)
	}
	if !strings.Contains(content, "body_truncated: true") {
		t.Fatalf("debug dump should mark large body as truncated, got: %s", content)
	}
	if len(contentBytes) >= len(body) {
		t.Fatalf("debug dump should be smaller than the original body after truncation: log=%d body=%d", len(contentBytes), len(body))
	}
}

func TestInitDebugGatewayBodyFile_CreatesPrivateFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "gateway_debug.log")

	svc := &GatewayService{}
	svc.initDebugGatewayBodyFile(path)
	f := svc.debugGatewayBodyFile.Load()
	if f == nil {
		t.Fatalf("expected debug file to be initialized")
	}
	defer f.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat debug log: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("debug log permissions = %04o, want 0600", got)
	}
}
