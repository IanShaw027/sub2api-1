//go:build unit

package service

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeadIdentityHelpersAreRemoved(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		require.NoError(t, err)
		text := string(src)
		require.NotContains(t, text, "func applyCodexClientMetadata(", name+": dead Codex helper must stay deleted")
		require.NotContains(t, text, "rewriteAnthropicUserIDFromProfile(", name+": dead Claude helper must stay deleted")
	}
}
