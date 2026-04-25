package service

import "testing"

import "github.com/stretchr/testify/require"

func TestNormalizeOpenAIMessagesDispatchModelConfig(t *testing.T) {
	t.Parallel()

	cfg := normalizeOpenAIMessagesDispatchModelConfig(OpenAIMessagesDispatchModelConfig{
		OpusMappedModel:   " gpt-5.4-high ",
		SonnetMappedModel: "gpt-5.3-codex",
		HaikuMappedModel:  " gpt-5.4-mini-medium ",
		ExactModelMappings: map[string]string{
			" claude-sonnet-4-5-20250929 ": " gpt-5.2-high ",
			"":                             "gpt-5.4",
			"claude-opus-4-6":              " ",
		},
	})

	require.Equal(t, "gpt-5.4", cfg.OpusMappedModel)
	require.Equal(t, "gpt-5.3-codex", cfg.SonnetMappedModel)
	require.Equal(t, "gpt-5.4-mini", cfg.HaikuMappedModel)
	require.Equal(t, map[string]string{
		"claude-sonnet-4-5-20250929": "gpt-5.2",
	}, cfg.ExactModelMappings)
}

func TestNormalizeOpenAIMessagesDispatchModelConfig_PreservesSparkTargets(t *testing.T) {
	t.Parallel()

	cfg := normalizeOpenAIMessagesDispatchModelConfig(OpenAIMessagesDispatchModelConfig{
		HaikuMappedModel: " gpt-5.3-codex-spark ",
		ExactModelMappings: map[string]string{
			" claude-haiku-4-5-20251001 ": " gpt-5.3-codex-spark-high ",
		},
	})

	require.Equal(t, "gpt-5.3-codex-spark", cfg.HaikuMappedModel)
	require.Equal(t, map[string]string{
		"claude-haiku-4-5-20251001": "gpt-5.3-codex-spark",
	}, cfg.ExactModelMappings)
}
