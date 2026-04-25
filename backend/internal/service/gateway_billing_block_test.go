package service

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeClaudeCodeFingerprint_UsesRunePositions(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"你好世界abc"}]}`)
	version := "2.1.92"

	got := computeClaudeCodeFingerprint(body, version)

	// "你好世界abc" at rune index 4 is "a"; 7 and 20 are missing => "0".
	sum := sha256.Sum256([]byte(fingerprintSalt + "a00" + version))
	want := hex.EncodeToString(sum[:])[:3]
	if got != want {
		t.Fatalf("fingerprint mismatch: got=%q want=%q", got, want)
	}
}

func TestComposeClaudeCodeBillingVersion(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"hello world"}]}`)
	version := "2.1.92"

	got := composeClaudeCodeBillingVersion(body, version)

	require.Equal(t, version+"."+computeClaudeCodeFingerprint(body, version), got)
}
