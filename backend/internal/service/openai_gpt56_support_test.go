package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCodexModel_GPT56FamilyAndReasoningSuffixes(t *testing.T) {
	for _, model := range []string{"gpt-5.6", "gpt-5.6-none", "gpt-5.6-low", "gpt-5.6-medium", "gpt-5.6-high", "gpt-5.6-xhigh", "gpt-5.6-extra-high", "gpt-5.6-max"} {
		require.Equal(t, "gpt-5.6-sol", normalizeCodexModel(model), model)
	}
	for _, model := range []string{"gpt-5.6-sol", "gpt-5.6-sol-low", "gpt-5.6-sol-medium", "gpt-5.6-sol-high", "gpt-5.6-sol-xhigh", "gpt-5.6-sol-max"} {
		require.Equal(t, "gpt-5.6-sol", normalizeCodexModel(model), model)
	}
	for _, model := range []string{"gpt-5.6-terra", "gpt-5.6-terra-low", "gpt-5.6-terra-medium", "gpt-5.6-terra-high", "gpt-5.6-terra-xhigh", "gpt-5.6-terra-max"} {
		require.Equal(t, "gpt-5.6-terra", normalizeCodexModel(model), model)
	}
	for _, model := range []string{"gpt-5.6-luna", "gpt-5.6-luna-low", "gpt-5.6-luna-medium", "gpt-5.6-luna-high", "gpt-5.6-luna-xhigh", "gpt-5.6-luna-max"} {
		require.Equal(t, "gpt-5.6-luna", normalizeCodexModel(model), model)
	}
}

func TestOpenAICompatModelNormalization_GPT56MaxOnlyForGPT56(t *testing.T) {
	req := &apicompat.AnthropicRequest{Model: "gpt-5.6-terra-max"}
	applyOpenAICompatModelNormalization(req)
	require.Equal(t, "gpt-5.6-terra", req.Model)
	require.NotNil(t, req.OutputConfig)
	require.Equal(t, "max", req.OutputConfig.Effort)

	require.Equal(t, "gpt-5.4-max", NormalizeOpenAICompatRequestedModel("gpt-5.4-max"))
	require.Equal(t, "gpt-5.1-codex-max", NormalizeOpenAICompatRequestedModel("gpt-5.1-codex-max"))
}

func TestOpenAICompatModelNormalization_GPT56UltraSuffix(t *testing.T) {
	// ultra suffix strips to the base gpt-5.6 model and maps to Claude's top
	// output effort ("max").
	req := &apicompat.AnthropicRequest{Model: "gpt-5.6-sol-ultra"}
	applyOpenAICompatModelNormalization(req)
	require.Equal(t, "gpt-5.6-sol", req.Model)
	require.NotNil(t, req.OutputConfig)
	require.Equal(t, "max", req.OutputConfig.Effort)

	// ultra is only valid on the gpt-5.6 family; other models keep the suffix
	// verbatim (no reasoning split).
	require.Equal(t, "gpt-5.4-ultra", NormalizeOpenAICompatRequestedModel("gpt-5.4-ultra"))
	require.Equal(t, "gpt-5.1-codex-ultra", NormalizeOpenAICompatRequestedModel("gpt-5.1-codex-ultra"))
}

func TestNormalizeCodexModel_GPT56UltraAndMaxSuffixes(t *testing.T) {
	for _, model := range []string{"gpt-5.6-sol-ultra", "gpt-5.6-sol-max"} {
		require.Equal(t, "gpt-5.6-sol", normalizeCodexModel(model), model)
	}
	for _, model := range []string{"gpt-5.6-terra-ultra", "gpt-5.6-terra-max"} {
		require.Equal(t, "gpt-5.6-terra", normalizeCodexModel(model), model)
	}
	for _, model := range []string{"gpt-5.6-luna-ultra", "gpt-5.6-luna-max"} {
		require.Equal(t, "gpt-5.6-luna", normalizeCodexModel(model), model)
	}
}

func TestNormalizeOpenAIReasoningEffort_MaxAndUltra(t *testing.T) {
	require.Equal(t, "xhigh", normalizeOpenAIReasoningEffort("xhigh"))
	require.Equal(t, "xhigh", normalizeOpenAIReasoningEffort("extra-high"))
	require.Equal(t, "max", normalizeOpenAIReasoningEffort("max"))
	require.Equal(t, "ultra", normalizeOpenAIReasoningEffort("ultra"))
	require.Equal(t, "ultra", normalizeOpenAIReasoningEffort(" Ultra "))
	require.Equal(t, "", normalizeOpenAIReasoningEffort("none"))
}

func TestOpenAIUsageExtractionReadsGPT56CacheWriteTokens(t *testing.T) {
	usage, ok := extractOpenAIUsageFromJSONBytes([]byte(`{"usage":{"input_tokens":100,"output_tokens":20,"cache_write_tokens":30,"input_tokens_details":{"cached_tokens":40}}}`))
	require.True(t, ok)
	require.Equal(t, 100, usage.InputTokens)
	require.Equal(t, 20, usage.OutputTokens)
	require.Equal(t, 30, usage.CacheCreationInputTokens)
	require.Equal(t, 40, usage.CacheReadInputTokens)

	usage, ok = extractOpenAIUsageFromSSEEventBytes([]byte(`{"type":"response.completed","response":{"usage":{"input_tokens":10,"output_tokens":2,"input_tokens_details":{"cache_write_tokens":3,"cached_tokens":4}}}}`))
	require.True(t, ok)
	require.Equal(t, 3, usage.CacheCreationInputTokens)
	require.Equal(t, 4, usage.CacheReadInputTokens)
}

func TestPopulateOpenAIUsageFromResponseJSONDoesNotZeroParsedCacheWriteTokens(t *testing.T) {
	usage := &OpenAIUsage{
		InputTokens:              10,
		OutputTokens:             2,
		CacheCreationInputTokens: 3,
		CacheReadInputTokens:     4,
	}

	populateOpenAIUsageFromResponseJSON([]byte(`{"usage":{"input_tokens":10,"output_tokens":2,"input_tokens_details":{"cached_tokens":4,"cache_write_tokens":0}}}`), usage)

	require.Equal(t, 10, usage.InputTokens)
	require.Equal(t, 2, usage.OutputTokens)
	require.Equal(t, 3, usage.CacheCreationInputTokens)
	require.Equal(t, 4, usage.CacheReadInputTokens)
}

func TestBillingServiceGPT56PricingIncludesCacheWrites(t *testing.T) {
	svc := NewBillingService(&config.Config{}, nil)

	cases := []struct {
		model       string
		inputPrice  float64
		outputPrice float64
		writePrice  float64
		readPrice   float64
	}{
		{"gpt-5.6-sol", 5e-6, 30e-6, 6.25e-6, 0.5e-6},
		{"gpt-5.6-terra", 2.5e-6, 15e-6, 3.125e-6, 0.25e-6},
		{"gpt-5.6-luna", 1e-6, 6e-6, 1.25e-6, 0.1e-6},
	}
	for _, tt := range cases {
		t.Run(tt.model, func(t *testing.T) {
			pricing, err := svc.GetModelPricing(tt.model + "-max")
			require.NoError(t, err)
			require.InDelta(t, tt.inputPrice, pricing.InputPricePerToken, 1e-12)
			require.InDelta(t, tt.outputPrice, pricing.OutputPricePerToken, 1e-12)
			require.InDelta(t, tt.writePrice, pricing.CacheCreationPricePerToken, 1e-12)
			require.InDelta(t, tt.readPrice, pricing.CacheReadPricePerToken, 1e-12)

			cost, err := svc.CalculateCost(tt.model, UsageTokens{InputTokens: 10, OutputTokens: 20, CacheCreationTokens: 30, CacheReadTokens: 40}, 1)
			require.NoError(t, err)
			require.InDelta(t, 30*tt.writePrice, cost.CacheCreationCost, 1e-12)
			require.InDelta(t, 40*tt.readPrice, cost.CacheReadCost, 1e-12)
		})
	}
}
