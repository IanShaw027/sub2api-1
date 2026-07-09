//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayServiceCalculateRecordUsageCost_AudioDoesNotReplaceTokenCost(t *testing.T) {
	t.Parallel()

	audioPricePerMin := 0.42
	svc := &GatewayService{billingService: NewBillingService(nil, nil)}
	apiKey := &APIKey{
		Group: &Group{
			ID:                       11,
			AudioRealtimePricePerMin: &audioPricePerMin,
		},
	}
	result := &ForwardResult{
		Model: "claude-sonnet-4",
		Usage: ClaudeUsage{
			InputTokens:  1000,
			OutputTokens: 200,
		},
		AudioUsage: &AudioUsage{
			Mode:            "realtime",
			DurationOrUnits: 3.5,
		},
	}

	tokenOnlyCost := svc.calculateTokenCost(context.Background(), result, apiKey, "claude-sonnet-4", 1, &recordUsageOpts{})
	audioCost := svc.billingService.CalculateAudioCost("realtime", 3.5, &audioPriceConfig{
		RealtimePerMin: &audioPricePerMin,
	}, 1)

	breakdown := svc.calculateRecordUsageCost(context.Background(), result, apiKey, "claude-sonnet-4", 1, 1, &recordUsageOpts{})
	require.NotNil(t, breakdown)
	require.Equal(t, string(BillingModeAudio), breakdown.BillingMode)
	require.Greater(t, breakdown.InputCost, 0.0)
	require.Greater(t, breakdown.OutputCost, 0.0)
	require.InDelta(t, tokenOnlyCost.TotalCost+audioCost.TotalCost, breakdown.TotalCost, 1e-12)
	require.InDelta(t, tokenOnlyCost.ActualCost+audioCost.ActualCost, breakdown.ActualCost, 1e-12)
}

func TestGatewayServiceCalculateRecordUsageCost_NilOptsDoesNotPanic(t *testing.T) {
	t.Parallel()

	svc := &GatewayService{billingService: NewBillingService(nil, nil)}
	result := &ForwardResult{
		Model: "claude-sonnet-4",
		Usage: ClaudeUsage{
			InputTokens:  1000,
			OutputTokens: 200,
		},
	}

	expected := svc.calculateRecordUsageCost(context.Background(), result, nil, "claude-sonnet-4", 1, 1, &recordUsageOpts{})

	var actual *CostBreakdown
	require.NotPanics(t, func() {
		actual = svc.calculateRecordUsageCost(context.Background(), result, nil, "claude-sonnet-4", 1, 1, nil)
	})
	require.NotNil(t, actual)
	require.Equal(t, expected.BillingMode, actual.BillingMode)
	require.InDelta(t, expected.TotalCost, actual.TotalCost, 1e-12)
	require.InDelta(t, expected.ActualCost, actual.ActualCost, 1e-12)
}

func TestGatewayServiceCalculateRecordUsageCost_ImageWithNilAPIKeyDoesNotPanicWhenResolverPresent(t *testing.T) {
	t.Parallel()

	groupID := int64(126)
	svc := &GatewayService{
		billingService: NewBillingService(nil, nil),
		resolver:       newOpenAIImageChannelPricingResolverForTest(t, groupID, "gemini-image", 0.25),
	}
	result := &ForwardResult{
		Model:      "gemini-image",
		ImageCount: 2,
		ImageSize:  "1K",
	}

	expected := svc.billingService.CalculateImageCost("gemini-image", "1K", 2, nil, 1)

	var actual *CostBreakdown
	require.NotPanics(t, func() {
		actual = svc.calculateRecordUsageCost(context.Background(), result, nil, "gemini-image", 1, 1, &recordUsageOpts{})
	})
	require.NotNil(t, actual)
	require.Equal(t, string(BillingModeImage), actual.BillingMode)
	require.InDelta(t, expected.TotalCost, actual.TotalCost, 1e-12)
	require.InDelta(t, expected.ActualCost, actual.ActualCost, 1e-12)
}
