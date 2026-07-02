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

	require.Equal(t, "gpt-5.4-high", cfg.OpusMappedModel)
	require.Equal(t, "gpt-5.3-codex", cfg.SonnetMappedModel)
	require.Equal(t, "gpt-5.4-mini-medium", cfg.HaikuMappedModel)
	require.Equal(t, map[string]string{
		"claude-sonnet-4-5-20250929": "gpt-5.2-high",
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
		"claude-haiku-4-5-20251001": "gpt-5.3-codex-spark-high",
	}, cfg.ExactModelMappings)
}

func TestResolveMessagesDispatchModelWithSource(t *testing.T) {
	t.Parallel()

	cfg := normalizeOpenAIMessagesDispatchModelConfig(OpenAIMessagesDispatchModelConfig{
		SonnetMappedModel: "gpt-5.4-medium",
		ExactModelMappings: map[string]string{
			"claude-opus-4-6": "gpt-5.5-high",
		},
	})

	mappedModel, explicit := resolveOpenAIMessagesDispatchModel(cfg, "claude-opus-4-6")
	require.Equal(t, "gpt-5.5-high", mappedModel)
	require.True(t, explicit)

	mappedModel, explicit = resolveOpenAIMessagesDispatchModel(cfg, "claude-sonnet-4-5-20250929")
	require.Equal(t, "gpt-5.4-medium", mappedModel)
	require.True(t, explicit)

	mappedModel, explicit = resolveOpenAIMessagesDispatchModel(OpenAIMessagesDispatchModelConfig{}, "claude-haiku-4-5-20251001")
	require.Equal(t, defaultOpenAIMessagesDispatchHaikuMappedModel, mappedModel)
	require.False(t, explicit)

	mappedModel, explicit = resolveOpenAIMessagesDispatchModel(OpenAIMessagesDispatchModelConfig{}, "gpt-5.4")
	require.Empty(t, mappedModel)
	require.False(t, explicit)
}

func TestGroupResolveMessagesDispatchModel_GrokMapsClaudeFamilyToGrok(t *testing.T) {
	t.Parallel()

	group := &Group{Platform: PlatformGrok}

	mappedModel, explicit := group.ResolveMessagesDispatchModelWithSource("claude-sonnet-4-5")
	require.Equal(t, "grok-4.3", mappedModel)
	require.False(t, explicit)
	require.Equal(t, "grok-4.3", group.ResolveMessagesDispatchModel("claude-sonnet-4-5"))
	require.Equal(t, "grok-4.3", group.ResolveMessagesDispatchModel("claude-opus-4-6"))
	require.Equal(t, "grok-4.3", group.ResolveMessagesDispatchModel("claude-haiku-4-5"))
	require.Empty(t, group.ResolveMessagesDispatchModel("grok"))
	require.Empty(t, group.ResolveMessagesDispatchModel("gpt-5.3-codex"))
}
