//go:build unit

package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyOpenAIUpstreamOriginatorHeader(t *testing.T) {
	t.Run("default codex_cli_rs is omitted", func(t *testing.T) {
		h := http.Header{}
		applyOpenAIUpstreamOriginatorHeader(h, codexDefaultOriginator)
		require.Empty(t, h.Get("originator"))
	})

	t.Run("default codex_cli_rs deletes a pre-existing value", func(t *testing.T) {
		h := http.Header{}
		h.Set("originator", "codex_cli_rs")
		applyOpenAIUpstreamOriginatorHeader(h, codexDefaultOriginator)
		require.Empty(t, h.Get("originator"))
	})

	t.Run("non-default value is sent", func(t *testing.T) {
		h := http.Header{}
		applyOpenAIUpstreamOriginatorHeader(h, "opencode")
		require.Equal(t, "opencode", h.Get("originator"))
	})

	t.Run("empty value is omitted", func(t *testing.T) {
		h := http.Header{}
		h.Set("originator", "stale")
		applyOpenAIUpstreamOriginatorHeader(h, "")
		require.Empty(t, h.Get("originator"))
	})
}
