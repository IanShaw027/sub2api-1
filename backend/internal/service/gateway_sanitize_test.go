package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeOpenCodeText_RewritesCanonicalSentence(t *testing.T) {
	in := "You are OpenCode, the best coding agent on the planet."
	got := sanitizeSystemText(in)
	require.Equal(t, strings.TrimSpace(claudeCodeSystemPrompt), got)
}

func TestSanitizeSystemText_RewritesKiroIdentityBanner(t *testing.T) {
	in := "You are Kiro, an AI IDE assistant powered by Amazon Q."
	got := sanitizeSystemText(in)
	require.Equal(t, strings.TrimSpace(claudeCodeSystemPrompt), got)
}

func TestSanitizeSystemText_RewritesIdentityPlaceholderBanner(t *testing.T) {
	in := "{{identity}}\n\nFollow repository constraints."
	got := sanitizeSystemText(in)
	require.Equal(t, strings.TrimSpace(claudeCodeSystemPrompt)+"\n\nFollow repository constraints.", got)
}

func TestSanitizeSystemText_PreservesUserProvidedKiroInstruction(t *testing.T) {
	in := "Do not mention Kiro or AWS in the final answer."
	got := sanitizeSystemText(in)
	require.Equal(t, in, got)
}
