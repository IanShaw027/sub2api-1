package repository

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountSchedulingThresholdSnapshotExtraKeysPreserveOpenAIImageCodexFields(t *testing.T) {
	keys := map[string]bool{}
	for _, key := range accountSchedulingThresholdSnapshotExtraKeys {
		keys[key] = true
		require.False(t, strings.HasPrefix(key, "openai_image_codex_"), "must not clear image Codex quota fields: %s", key)
	}

	require.True(t, keys["session_window_utilization"])
	require.True(t, keys["passive_usage_7d_utilization"])
	require.True(t, keys["passive_usage_7d_reset"])
	require.True(t, keys["passive_usage_sampled_at"])
	require.True(t, keys["codex_5h_used_percent"])
	require.True(t, keys["codex_5h_reset_at"])
	require.True(t, keys["codex_7d_used_percent"])
	require.True(t, keys["codex_7d_reset_at"])
	require.True(t, keys["codex_usage_updated_at"])
}

func TestAccountSchedulingThresholdSnapshotCleanupQueryIsScopedToSupportedPlatforms(t *testing.T) {
	query := accountSchedulingThresholdSnapshotCleanupQuery()

	require.Contains(t, query, "WHERE id = $1")
	require.Contains(t, query, "platform IN ('openai', 'anthropic')")
	require.NotContains(t, query, "gemini")
	require.NotContains(t, query, "kiro")
	require.NotContains(t, query, "antigravity")
}
