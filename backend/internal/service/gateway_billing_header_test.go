package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncBillingHeaderVersion(t *testing.T) {
	t.Run("recomputes suffix when billing header already has suffix", func(t *testing.T) {
		body := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.81.df2; cc_entrypoint=cli; cch=00000;"},{"type":"text","text":"You are Claude Code.","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":"hello billing fingerprint"}]}`)

		result := syncBillingHeaderVersion(body, "claude-cli/2.1.22 (external, cli)")

		expectedVersion := composeClaudeCodeBillingVersion(body, "2.1.22")
		require.NotEmpty(t, expectedVersion)
		assert.Contains(t, string(result), "cc_version="+expectedVersion)
		assert.NotContains(t, string(result), "cc_version=2.1.81.df2")
	})

	t.Run("no suffix keeps semver-only replacement", func(t *testing.T) {
		body := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.81; cc_entrypoint=cli; cch=00000;"}],"messages":[]}`)

		result := syncBillingHeaderVersion(body, "claude-cli/2.1.22")

		assert.Contains(t, string(result), "cc_version=2.1.22;")
		assert.NotContains(t, string(result), "cc_version=2.1.22.")
	})

	t.Run("no billing header in system", func(t *testing.T) {
		body := `{"system":[{"type":"text","text":"You are Claude Code."}],"messages":[]}`
		result := syncBillingHeaderVersion([]byte(body), "claude-cli/2.1.22")
		assert.Equal(t, body, string(result))
	})

	t.Run("no system field", func(t *testing.T) {
		body := `{"messages":[]}`
		result := syncBillingHeaderVersion([]byte(body), "claude-cli/2.1.22")
		assert.Equal(t, body, string(result))
	})

	t.Run("user-agent without version", func(t *testing.T) {
		body := `{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.81; cc_entrypoint=cli; cch=00000;"}],"messages":[]}`
		result := syncBillingHeaderVersion([]byte(body), "Mozilla/5.0")
		assert.Equal(t, body, string(result))
	})
}
