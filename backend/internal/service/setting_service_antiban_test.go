package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAntiBanPlatformsKeepsKnownPlatformsOnly(t *testing.T) {
	got := normalizeAntiBanPlatforms(map[string]bool{
		"anthropic": true,
		"grok":      true,
		"unknown":   true,
	})

	require.True(t, got["anthropic"])
	require.True(t, got["grok"])
	require.False(t, got["openai"])
	require.False(t, got["gemini"])
	require.False(t, got["kiro"])
	require.False(t, got["antigravity"])
	require.NotContains(t, got, "unknown")
}

func TestCloneAntiBanPlatformsBreaksAliases(t *testing.T) {
	src := map[string]bool{"anthropic": true}
	got := cloneAntiBanPlatforms(src)
	src["anthropic"] = false

	require.True(t, got["anthropic"])
}
