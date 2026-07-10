//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayServiceCalculateOpenAIRecordUsageCost_AudioDoesNotReplaceTokenCost(t *testing.T) {
	t.Parallel()

	audioPricePerMin := 0.42
	svc := &OpenAIGatewayService{billingService: NewBillingService(nil, nil)}
	groupID := int64(123)
	usage := UsageTokens{
		InputTokens:  1000,
		OutputTokens: 200,
	}

	tokenOnlyCost := expectedOpenAICost(t, svc, "grok-4.3", OpenAIUsage{
		InputTokens:  1000,
		OutputTokens: 200,
	}, 1)
	audioCost := svc.billingService.CalculateAudioCost("realtime", 3.5, &audioPriceConfig{
		RealtimePerMin: &audioPricePerMin,
	}, 1)

	breakdown, err := svc.calculateOpenAIRecordUsageCost(
		context.Background(),
		&OpenAIForwardResult{
			Model: "grok-4.3",
			AudioUsage: &AudioUsage{
				Mode:            "realtime",
				DurationOrUnits: 3.5,
			},
		},
		&APIKey{GroupID: &groupID, Group: &Group{ID: groupID, AudioRealtimePricePerMin: &audioPricePerMin}},
		"grok-4.3",
		1,
		1,
		1,
		usage,
		"",
		RequestTypeUnknown,
	)
	require.NoError(t, err)
	require.NotNil(t, breakdown)
	require.Equal(t, string(BillingModeAudio), breakdown.BillingMode)
	require.Greater(t, breakdown.InputCost, 0.0)
	require.Greater(t, breakdown.OutputCost, 0.0)
	require.InDelta(t, tokenOnlyCost.TotalCost+audioCost.TotalCost, breakdown.TotalCost, 1e-12)
	require.InDelta(t, tokenOnlyCost.ActualCost+audioCost.ActualCost, breakdown.ActualCost, 1e-12)
}
