package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestParsePricingData_ParsesPriorityAndServiceTierFields(t *testing.T) {
	svc := &PricingService{}
	body := []byte(`{
		"gpt-5.4": {
			"input_cost_per_token": 0.0000025,
			"input_cost_per_token_priority": 0.000005,
			"output_cost_per_token": 0.000015,
			"output_cost_per_token_priority": 0.00003,
			"cache_creation_input_token_cost": 0.0000025,
			"cache_read_input_token_cost": 0.00000025,
			"cache_read_input_token_cost_priority": 0.0000005,
			"supports_service_tier": true,
			"supports_prompt_caching": true,
			"litellm_provider": "openai",
			"mode": "chat"
		}
	}`)

	data, err := svc.parsePricingData(body)
	require.NoError(t, err)
	pricing := data["gpt-5.4"]
	require.NotNil(t, pricing)
	require.InDelta(t, 5e-6, pricing.InputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 3e-5, pricing.OutputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 5e-7, pricing.CacheReadInputTokenCostPriority, 1e-12)
	require.True(t, pricing.SupportsServiceTier)
}

func TestGetModelPricing_Gpt53CodexSparkUsesExactPricingWhenAvailable(t *testing.T) {
	sparkPricing := &LiteLLMModelPricing{InputCostPerToken: 1}
	gpt51Pricing := &LiteLLMModelPricing{InputCostPerToken: 9}

	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.3-codex-spark": sparkPricing,
			"gpt-5.1-codex":       gpt51Pricing,
		},
	}

	got := svc.GetModelPricing("gpt-5.3-codex-spark")
	require.Same(t, sparkPricing, got)
}

func TestGetModelPricing_Gpt53CodexSparkUsesDedicatedStaticFallbackWhenRemoteMissing(t *testing.T) {
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.1-codex": {InputCostPerToken: 1.25e-6},
		},
	}

	got := svc.GetModelPricing("gpt-5.3-codex-spark")
	require.NotNil(t, got)
	require.InDelta(t, 1.75e-6, got.InputCostPerToken, 1e-12)
	require.InDelta(t, 1.4e-5, got.OutputCostPerToken, 1e-12)
	require.InDelta(t, 1.75e-7, got.CacheReadInputTokenCost, 1e-12)
	require.InDelta(t, 3.5e-6, got.InputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 2.8e-5, got.OutputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 3.5e-7, got.CacheReadInputTokenCostPriority, 1e-12)
}

func TestPricingValidateURLDisabledStillBlocksPrivateHosts(t *testing.T) {
	svc := &PricingService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{Enabled: false},
			},
		},
	}

	for _, raw := range []string{
		"https://localhost/pricing.json",
		"https://127.0.0.1/pricing.json",
		"https://169.254.169.254/latest/meta-data",
	} {
		if _, err := svc.validatePricingURL(raw); err == nil {
			t.Fatalf("expected %s to be rejected when allowlist is disabled but private hosts are not allowed", raw)
		}
	}

	svc.cfg.Security.URLAllowlist.AllowPrivateHosts = true
	if _, err := svc.validatePricingURL("https://127.0.0.1/pricing.json"); err != nil {
		t.Fatalf("expected private host to be allowed only when allow_private_hosts=true, got %v", err)
	}
}

func TestGetModelPricing_ClaudeFableUsesStaticFallbackWhenRemoteMissing(t *testing.T) {
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"claude-opus-4-7": {InputCostPerToken: 5e-6, OutputCostPerToken: 2.5e-5},
		},
	}

	for _, model := range []string{"claude-fable-5", "claude-fable-5-1m", "claude-fable-6"} {
		got := svc.GetModelPricing(model)
		require.NotNilf(t, got, "expected fallback pricing for %s", model)
		require.InDelta(t, 1e-5, got.InputCostPerToken, 1e-12, model)
		require.InDelta(t, 5e-5, got.OutputCostPerToken, 1e-12, model)
		require.InDelta(t, 1.25e-5, got.CacheCreationInputTokenCost, 1e-12, model)
		require.InDelta(t, 1e-6, got.CacheReadInputTokenCost, 1e-12, model)
		require.True(t, got.SupportsPromptCaching, model)
	}
}

func TestGetModelPricing_ClaudeFablePrefersRemoteWhenAvailable(t *testing.T) {
	remote := &LiteLLMModelPricing{InputCostPerToken: 7e-6, OutputCostPerToken: 3e-5}
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"claude-fable-5": remote,
		},
	}

	got := svc.GetModelPricing("claude-fable-5")
	require.Same(t, remote, got)
}

func TestGetModelPricing_Gpt53CodexFallbackStillUsesGpt52Codex(t *testing.T) {
	gpt52CodexPricing := &LiteLLMModelPricing{InputCostPerToken: 2}

	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.2-codex": gpt52CodexPricing,
		},
	}

	got := svc.GetModelPricing("gpt-5.3-codex")
	require.Same(t, gpt52CodexPricing, got)
}

func TestGetModelPricing_OpenAIFallbackMatchedLoggedAsInfo(t *testing.T) {
	logSink, restore := captureStructuredLog(t)
	defer restore()

	gpt52CodexPricing := &LiteLLMModelPricing{InputCostPerToken: 2}
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.2-codex": gpt52CodexPricing,
		},
	}

	got := svc.GetModelPricing("gpt-5.3-codex")
	require.Same(t, gpt52CodexPricing, got)

	require.True(t, logSink.ContainsMessageAtLevel("[Pricing] OpenAI fallback matched gpt-5.3-codex -> gpt-5.2-codex", "info"))
	require.False(t, logSink.ContainsMessageAtLevel("[Pricing] OpenAI fallback matched gpt-5.3-codex -> gpt-5.2-codex", "warn"))
}

func TestGetModelPricing_Gpt54UsesStaticFallbackWhenRemoteMissing(t *testing.T) {
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.1-codex": &LiteLLMModelPricing{InputCostPerToken: 1.25e-6},
		},
	}

	got := svc.GetModelPricing("gpt-5.4")
	require.NotNil(t, got)
	require.InDelta(t, 2.5e-6, got.InputCostPerToken, 1e-12)
	require.InDelta(t, 1.5e-5, got.OutputCostPerToken, 1e-12)
	require.InDelta(t, 2.5e-7, got.CacheReadInputTokenCost, 1e-12)
	require.Equal(t, 272000, got.LongContextInputTokenThreshold)
	require.InDelta(t, 2.0, got.LongContextInputCostMultiplier, 1e-12)
	require.InDelta(t, 1.5, got.LongContextOutputCostMultiplier, 1e-12)
}

func TestGetModelPricing_OpenAICompactAliasUsesStaticFallback(t *testing.T) {
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.1-codex": {InputCostPerToken: 1.25e-6},
		},
	}

	got := svc.GetModelPricing("openai/gpt5.5")
	require.NotNil(t, got)
	require.InDelta(t, 2.5e-6, got.InputCostPerToken, 1e-12)
	require.InDelta(t, 1.5e-5, got.OutputCostPerToken, 1e-12)
}

func TestDefaultPricingIncludesCodexAutoReview(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)

	svc := &PricingService{}
	pricingData, err := svc.parsePricingData(data)
	require.NoError(t, err)
	svc.pricingData = pricingData

	got := svc.GetModelPricing("codex-auto-review")
	require.NotNil(t, got)
	require.InDelta(t, 5e-6, got.InputCostPerToken, 1e-12)
	require.InDelta(t, 3e-5, got.OutputCostPerToken, 1e-12)
	require.InDelta(t, 5e-7, got.CacheReadInputTokenCost, 1e-12)
}

func TestDefaultPricingIncludesGrokTextModels(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)

	svc := &PricingService{}
	pricingData, err := svc.parsePricingData(data)
	require.NoError(t, err)
	svc.pricingData = pricingData

	tests := []struct {
		model     string
		in        float64
		out       float64
		cacheRead float64
	}{
		{model: "grok-4.5", in: 2e-6, out: 6e-6, cacheRead: 0.5e-6},
		{model: "grok-4.3", in: 1.25e-6, out: 2.5e-6, cacheRead: 0.2e-6},
		{model: "grok-4.20-0309-reasoning", in: 1.25e-6, out: 2.5e-6, cacheRead: 0.2e-6},
		{model: "grok-4.20-0309-non-reasoning", in: 1.25e-6, out: 2.5e-6, cacheRead: 0.2e-6},
		{model: "grok-4.20-multi-agent-0309", in: 1.25e-6, out: 2.5e-6, cacheRead: 0.2e-6},
		{model: "grok-build-0.1", in: 1e-6, out: 2e-6, cacheRead: 0.2e-6},
	}
	for _, tt := range tests {
		got := svc.GetModelPricing(tt.model)
		require.NotNilf(t, got, "expected default pricing for %s", tt.model)
		require.Equal(t, "xai", got.LiteLLMProvider)
		require.InDelta(t, tt.in, got.InputCostPerToken, 1e-12, tt.model)
		require.InDelta(t, tt.out, got.OutputCostPerToken, 1e-12, tt.model)
		require.InDelta(t, tt.cacheRead, got.CacheReadInputTokenCost, 1e-12, "cache_read for %s", tt.model)
		require.Greater(t, got.CacheReadInputTokenCost, 0.0, "cache_read must be non-zero for %s", tt.model)
		require.True(t, got.SupportsPromptCaching, "supports_prompt_caching for %s", tt.model)
	}
}

func TestDefaultPricingIncludesGrokImagineMediaModels(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)

	svc := &PricingService{}
	pricingData, err := svc.parsePricingData(data)
	require.NoError(t, err)
	svc.pricingData = pricingData

	quality := svc.GetModelPricing("grok-imagine-image-quality")
	require.NotNil(t, quality)
	require.Equal(t, "xai", quality.LiteLLMProvider)
	require.Equal(t, "image_generation", quality.Mode)
	require.InDelta(t, 0.05, quality.OutputCostPerImage, 1e-12)

	fast := svc.GetModelPricing("grok-imagine-image")
	require.NotNil(t, fast)
	require.Equal(t, "image_generation", fast.Mode)
	require.InDelta(t, 0.02, fast.OutputCostPerImage, 1e-12)

	video := svc.GetModelPricing("grok-imagine-video")
	require.NotNil(t, video)
	require.Equal(t, "video_generation", video.Mode)
	require.InDelta(t, 0.05, video.OutputCostPerSecond, 1e-12)

	video15 := svc.GetModelPricing("grok-imagine-video-1.5")
	require.NotNil(t, video15)
	require.Equal(t, "video_generation", video15.Mode)
	require.InDelta(t, 0.08, video15.OutputCostPerSecond, 1e-12)
}

func TestGetModelPricing_GrokUsesXAIProviderCandidate(t *testing.T) {
	remote := &LiteLLMModelPricing{InputCostPerToken: 1.25e-6, OutputCostPerToken: 2.5e-6, LiteLLMProvider: "xai"}
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"xai/grok-4.3": remote,
		},
	}

	got := svc.GetModelPricing("grok-4.3")
	require.Same(t, remote, got)
}

func TestGetModelPricing_Gpt54MiniUsesDedicatedStaticFallbackWhenRemoteMissing(t *testing.T) {
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.1-codex": {InputCostPerToken: 1.25e-6},
		},
	}

	got := svc.GetModelPricing("gpt-5.4-mini")
	require.NotNil(t, got)
	require.InDelta(t, 7.5e-7, got.InputCostPerToken, 1e-12)
	require.InDelta(t, 4.5e-6, got.OutputCostPerToken, 1e-12)
	require.InDelta(t, 7.5e-8, got.CacheReadInputTokenCost, 1e-12)
	require.Zero(t, got.LongContextInputTokenThreshold)
}

func TestGetModelPricing_Gpt54NanoUsesDedicatedStaticFallbackWhenRemoteMissing(t *testing.T) {
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.1-codex": {InputCostPerToken: 1.25e-6},
		},
	}

	got := svc.GetModelPricing("gpt-5.4-nano")
	require.NotNil(t, got)
	require.InDelta(t, 2e-7, got.InputCostPerToken, 1e-12)
	require.InDelta(t, 1.25e-6, got.OutputCostPerToken, 1e-12)
	require.InDelta(t, 2e-8, got.CacheReadInputTokenCost, 1e-12)
	require.Zero(t, got.LongContextInputTokenThreshold)
}

func TestGetModelPricing_ImageModelDoesNotFallbackToTextModel(t *testing.T) {
	imagePricing := &LiteLLMModelPricing{InputCostPerToken: 3}
	textPricing := &LiteLLMModelPricing{InputCostPerToken: 9}

	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-image-2": imagePricing,
			"gpt-5.4":     textPricing,
		},
	}

	got := svc.GetModelPricing("gpt-image-3")
	require.Same(t, imagePricing, got)
}

func TestParsePricingData_PreservesPriorityAndServiceTierFields(t *testing.T) {
	raw := map[string]any{
		"gpt-5.4": map[string]any{
			"input_cost_per_token":                 2.5e-6,
			"input_cost_per_token_priority":        5e-6,
			"output_cost_per_token":                15e-6,
			"output_cost_per_token_priority":       30e-6,
			"cache_read_input_token_cost":          0.25e-6,
			"cache_read_input_token_cost_priority": 0.5e-6,
			"supports_service_tier":                true,
			"supports_prompt_caching":              true,
			"litellm_provider":                     "openai",
			"mode":                                 "chat",
		},
	}
	body, err := json.Marshal(raw)
	require.NoError(t, err)

	svc := &PricingService{}
	pricingMap, err := svc.parsePricingData(body)
	require.NoError(t, err)

	pricing := pricingMap["gpt-5.4"]
	require.NotNil(t, pricing)
	require.InDelta(t, 2.5e-6, pricing.InputCostPerToken, 1e-12)
	require.InDelta(t, 5e-6, pricing.InputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 15e-6, pricing.OutputCostPerToken, 1e-12)
	require.InDelta(t, 30e-6, pricing.OutputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 0.25e-6, pricing.CacheReadInputTokenCost, 1e-12)
	require.InDelta(t, 0.5e-6, pricing.CacheReadInputTokenCostPriority, 1e-12)
	require.True(t, pricing.SupportsServiceTier)
}

func TestParsePricingData_PreservesServiceTierPriorityFields(t *testing.T) {
	svc := &PricingService{}
	pricingData, err := svc.parsePricingData([]byte(`{
		"gpt-5.4": {
			"input_cost_per_token": 0.0000025,
			"input_cost_per_token_priority": 0.000005,
			"output_cost_per_token": 0.000015,
			"output_cost_per_token_priority": 0.00003,
			"cache_read_input_token_cost": 0.00000025,
			"cache_read_input_token_cost_priority": 0.0000005,
			"supports_service_tier": true,
			"litellm_provider": "openai",
			"mode": "chat"
		}
	}`))
	require.NoError(t, err)

	pricing := pricingData["gpt-5.4"]
	require.NotNil(t, pricing)
	require.InDelta(t, 0.0000025, pricing.InputCostPerToken, 1e-12)
	require.InDelta(t, 0.000005, pricing.InputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 0.000015, pricing.OutputCostPerToken, 1e-12)
	require.InDelta(t, 0.00003, pricing.OutputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 0.00000025, pricing.CacheReadInputTokenCost, 1e-12)
	require.InDelta(t, 0.0000005, pricing.CacheReadInputTokenCostPriority, 1e-12)
	require.True(t, pricing.SupportsServiceTier)
}

// ---------------------------------------------------------------------------
// ListModelNamesByProvider
// ---------------------------------------------------------------------------

func TestListModelNamesByProvider_ReturnsMatchingModels(t *testing.T) {
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"claude-opus-4-5-20251101": {LiteLLMProvider: "anthropic", InputCostPerToken: 1.5e-5},
			"claude-sonnet-4-5":        {LiteLLMProvider: "anthropic", InputCostPerToken: 3e-6},
			"gpt-4o":                   {LiteLLMProvider: "openai", InputCostPerToken: 5e-6},
			"gemini-2.5-pro":           {LiteLLMProvider: "google", InputCostPerToken: 1.25e-6},
		},
	}

	got := svc.ListModelNamesByProvider("anthropic")
	require.ElementsMatch(t, []string{"claude-opus-4-5-20251101", "claude-sonnet-4-5"}, got)
	// Must be sorted
	require.Equal(t, "claude-opus-4-5-20251101", got[0])
	require.Equal(t, "claude-sonnet-4-5", got[1])
}

func TestListModelNamesByProvider_CaseInsensitive(t *testing.T) {
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-4o": {LiteLLMProvider: "OpenAI", InputCostPerToken: 5e-6},
		},
	}

	got := svc.ListModelNamesByProvider("openai")
	require.Equal(t, []string{"gpt-4o"}, got)

	got2 := svc.ListModelNamesByProvider("OPENAI")
	require.Equal(t, []string{"gpt-4o"}, got2)
}

func TestListModelNamesByProvider_NoMatch(t *testing.T) {
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-4o": {LiteLLMProvider: "openai", InputCostPerToken: 5e-6},
		},
	}

	got := svc.ListModelNamesByProvider("anthropic")
	require.NotNil(t, got)
	require.Empty(t, got)
}

func TestListModelNamesByProvider_EmptyCatalog(t *testing.T) {
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{},
	}

	got := svc.ListModelNamesByProvider("openai")
	require.NotNil(t, got)
	require.Empty(t, got)
}

func TestBillingGrokFallback_CacheReadNonZero(t *testing.T) {
	svc := NewBillingService(&config.Config{}, nil)

	tests := []struct {
		model     string
		input     float64
		output    float64
		cacheRead float64
	}{
		{model: "grok-4.5", input: 2e-6, output: 6e-6, cacheRead: 0.5e-6},
		{model: "grok", input: 2e-6, output: 6e-6, cacheRead: 0.5e-6},
		{model: "grok-latest", input: 2e-6, output: 6e-6, cacheRead: 0.5e-6},
		{model: "grok-4.3", input: 1.25e-6, output: 2.5e-6, cacheRead: 0.2e-6},
		{model: "grok-4.20-0309-reasoning", input: 1.25e-6, output: 2.5e-6, cacheRead: 0.2e-6},
		{model: "grok-build-0.1", input: 1e-6, output: 2e-6, cacheRead: 0.2e-6},
		{model: "grok-build", input: 1e-6, output: 2e-6, cacheRead: 0.2e-6},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			pricing, err := svc.GetModelPricing(tt.model)
			require.NoError(t, err)
			require.NotNil(t, pricing)
			require.InDelta(t, tt.input, pricing.InputPricePerToken, 1e-12)
			require.InDelta(t, tt.output, pricing.OutputPricePerToken, 1e-12)
			require.InDelta(t, tt.cacheRead, pricing.CacheReadPricePerToken, 1e-12)
			require.Greater(t, pricing.CacheReadPricePerToken, 0.0)
		})
	}
}

func TestBillingGrokFallback_ImagineDoesNotUseTextRates(t *testing.T) {
	svc := NewBillingService(&config.Config{}, nil)

	for _, model := range []string{
		"grok-imagine-image-quality",
		"grok-imagine-image",
		"grok-imagine-video",
		"grok-imagine-video-1.5",
		"grok-imagine",
		"grok-unknown-future-model",
	} {
		t.Run(model, func(t *testing.T) {
			require.Nil(t, svc.getFallbackPricing(model), "text token fallback must not apply to %s", model)
			_, err := svc.GetModelPricing(model)
			require.Error(t, err)
		})
	}
}

func TestCalculateImageCost_GrokImagineDefaultsWithoutGroupConfig(t *testing.T) {
	svc := NewBillingService(&config.Config{}, nil)

	// Quality: $0.05/image flat (no 2K/4K multiplier)
	cost := svc.CalculateImageCost("grok-imagine-image-quality", "1K", 2, nil, 1.0)
	require.InDelta(t, 0.10, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.10, cost.ActualCost, 1e-12)

	cost = svc.CalculateImageCost("grok-imagine-image-quality", "4K", 1, nil, 1.0)
	require.InDelta(t, 0.05, cost.TotalCost, 1e-12)

	// Fast: $0.02/image
	cost = svc.CalculateImageCost("grok-imagine-image", "2K", 3, nil, 1.0)
	require.InDelta(t, 0.06, cost.TotalCost, 1e-12)

	// Alias → quality default
	cost = svc.CalculateImageCost("grok-imagine", "1K", 1, nil, 1.0)
	require.InDelta(t, 0.05, cost.TotalCost, 1e-12)

	// Must not fall through to Gemini $0.134
	cost = svc.CalculateImageCost("grok-imagine-image-quality", "1K", 1, nil, 1.0)
	require.InDelta(t, 0.05, cost.TotalCost, 1e-12)
	require.NotEqual(t, 0.134, cost.TotalCost)
}

func TestCalculateVideoCost_GrokImagineDefaultsWithoutGroupConfig(t *testing.T) {
	svc := NewBillingService(&config.Config{}, nil)

	// Without model → still 0 (cannot invent price)
	cost := svc.CalculateVideoCost("720p", 8, 1, nil, 1.0)
	require.InDelta(t, 0, cost.TotalCost, 1e-12)

	// grok-imagine-video 720p: $0.07/sec
	cost = svc.CalculateVideoCost("720p", 8, 1, nil, 1.0, "grok-imagine-video")
	require.InDelta(t, 0.56, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.56, cost.ActualCost, 1e-12)
	require.Equal(t, string(BillingModeVideo), cost.BillingMode)

	// grok-imagine-video-1.5 1080p: $0.25/sec
	cost = svc.CalculateVideoCost("1080p", 10, 2, nil, 1.0, "grok-imagine-video-1.5")
	require.InDelta(t, 5.00, cost.TotalCost, 1e-12) // 0.25 * 10 * 2

	// Alias
	cost = svc.CalculateVideoCost("480p", 5, 1, nil, 1.0, "grok-video-1.5")
	require.InDelta(t, 0.40, cost.TotalCost, 1e-12)
}

func TestCalculateImageCost_GrokImagineUsesCatalogWhenPricingServicePresent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)

	ps := &PricingService{}
	pricingData, err := ps.parsePricingData(data)
	require.NoError(t, err)
	ps.pricingData = pricingData

	svc := NewBillingService(&config.Config{}, ps)

	cost := svc.CalculateImageCost("grok-imagine-image-quality", "1K", 1, nil, 1.0)
	require.InDelta(t, 0.05, cost.TotalCost, 1e-12)

	cost = svc.CalculateImageCost("grok-imagine-image", "1K", 1, nil, 1.0)
	require.InDelta(t, 0.02, cost.TotalCost, 1e-12)

	vCost := svc.CalculateVideoCost("720p", 6, 1, nil, 1.0, "grok-imagine-video")
	require.InDelta(t, 0.42, vCost.TotalCost, 1e-12)

	vCost = svc.CalculateVideoCost("720p", 6, 1, nil, 1.0, "grok-imagine-video-1.5")
	require.InDelta(t, 0.84, vCost.TotalCost, 1e-12)
}
