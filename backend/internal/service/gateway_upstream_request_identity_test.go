//go:build unit

package service

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/stretchr/testify/require"
)

func TestApplyClaudeCodeMimicHeaders_DoesNotOverwriteFingerprintIdentity(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	require.NoError(t, err)
	setHeaderRaw(req.Header, "User-Agent", "claude-cli/2.1.221 (external, cli)")
	setHeaderRaw(req.Header, "X-Stainless-OS", "macOS")
	setHeaderRaw(req.Header, "X-Stainless-Arch", "x64")

	applyClaudeCodeMimicHeaders(req, true)

	require.Equal(t, "claude-cli/2.1.221 (external, cli)", getHeaderRaw(req.Header, "User-Agent"))
	require.Equal(t, "macOS", getHeaderRaw(req.Header, "X-Stainless-OS"))
	require.Equal(t, "x64", getHeaderRaw(req.Header, "X-Stainless-Arch"))
	require.NotEqual(t, claude.DefaultHeaders["User-Agent"], getHeaderRaw(req.Header, "User-Agent"))
	require.Equal(t, "stream", getHeaderRaw(req.Header, "x-stainless-helper-method"))
	require.NotEmpty(t, getHeaderRaw(req.Header, "x-client-request-id"))
}
